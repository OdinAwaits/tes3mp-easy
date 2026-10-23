package ops

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

type readCommandCase struct {
	name     string
	args     []string
	dataKeys []string
}

func readCommandCases() []readCommandCase {
	return []readCommandCase{
		{
			name: "status",
			args: []string{"status"},
			dataKeys: []string{
				"health", "started_at", "state", "unit",
			},
		},
		{
			name:     "logs",
			args:     []string{"logs", "--tail", "2"},
			dataKeys: []string{"lines"},
		},
		{
			name: "backups list",
			args: []string{
				"backups", "list", "--type", "mods", "--sha256",
			},
			dataKeys: []string{
				"current", "items", "next_before", "type",
			},
		},
		{
			name:     "doctor",
			args:     []string{"doctor"},
			dataKeys: []string{"checks", "status"},
		},
	}
}

func exactJSONKeys(t *testing.T, raw []byte, want []string) {
	t.Helper()

	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatalf("decode JSON object: %v; body=%s", err, raw)
	}
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	sortedWant := append([]string(nil), want...)
	sort.Strings(sortedWant)
	if !reflect.DeepEqual(keys, sortedWant) {
		t.Fatalf("JSON keys = %v, want %v", keys, sortedWant)
	}
}

func requireReadSuccess(t *testing.T, res result, dataKeys []string) envelope {
	t.Helper()

	res.requireExit(t, 0)
	if res.Stderr != "" {
		t.Fatalf("stderr = %q, want empty", res.Stderr)
	}
	env := res.decodeEnvelope(t)
	if !env.OK {
		t.Fatalf("ok = false, want true; stdout=%q", res.Stdout)
	}
	if env.Error != nil {
		t.Fatalf("success envelope contains error: %#v", env.Error)
	}
	exactJSONKeys(t, []byte(res.Stdout), []string{"data", "ok"})
	exactJSONKeys(t, env.Data, dataKeys)
	return env
}

func TestReadCommandsSuccessContract(t *testing.T) {
	root, env := doctorHealthyRoot(t)
	env["FAKE_JOURNALD_OUTPUT"] = "line one\nline two\n"
	env["FAKE_SYSTEMD_LOG"] = filepath.Join(t.TempDir(), "systemd.log")
	env["FAKE_JOURNALD_LOG"] = filepath.Join(t.TempDir(), "journald.log")

	before := snapshotTree(t, root)
	for _, tt := range readCommandCases() {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"--json"}, tt.args...)
			res := testHarness.RunRootEnv(t, root, env, args...)
			payload := requireReadSuccess(t, res, tt.dataKeys)

			switch tt.name {
			case "logs":
				data := decodeLogs(t, payload)
				want := []string{"line one", "line two"}
				if !reflect.DeepEqual(data.Lines, want) {
					t.Fatalf("lines = %#v, want %#v", data.Lines, want)
				}
			case "backups list":
				data := decodeBackups(t, payload, true)
				if len(data.Items) != 1 {
					t.Fatalf("items = %#v, want one archive", data.Items)
				}
			case "doctor":
				data := decodeDoctor(t, payload)
				if len(data.Checks) != 19 {
					t.Fatalf("checks = %d, want 19", len(data.Checks))
				}
			}
		})
	}

	after := snapshotTree(t, root)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("read commands changed temp root\nbefore: %#v\nafter:  %#v",
			before, after)
	}
	if _, err := os.Stat(filepath.Join(root, ".locks", "ops.lock")); !os.IsNotExist(err) {
		t.Fatalf("read commands created .locks/ops.lock: %v", err)
	}
}

func TestReadCommandsDoNotTakeLockContract(t *testing.T) {
	root, env := doctorHealthyRoot(t)
	env["FAKE_JOURNALD_OUTPUT"] = "line one\n"
	holdOpsLock(t, root)

	for _, tt := range readCommandCases() {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{
				"--json", "--lock-timeout=1ns",
			}, tt.args...)
			res := testHarness.RunRootEnv(t, root, env, args...)
			requireReadSuccess(t, res, tt.dataKeys)
		})
	}
}

func TestReadCommandsGlobalFlagsAfterCommandContract(t *testing.T) {
	root, env := doctorHealthyRoot(t)
	env["FAKE_JOURNALD_OUTPUT"] = "line one\n"

	tests := []readCommandCase{
		{
			name: "status",
			args: []string{
				"status", "--root=" + root, "--json",
			},
			dataKeys: []string{
				"health", "started_at", "state", "unit",
			},
		},
		{
			name: "logs",
			args: []string{
				"logs", "--tail", "1", "--root=" + root, "--json",
			},
			dataKeys: []string{"lines"},
		},
		{
			name: "backups list",
			args: []string{
				"backups", "list", "--type", "mods",
				"--root=" + root, "--json",
			},
			dataKeys: []string{
				"current", "items", "next_before", "type",
			},
		},
		{
			name: "doctor",
			args: []string{
				"doctor", "--root=" + root, "--json",
			},
			dataKeys: []string{"checks", "status"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := testHarness.RunArgsEnv(t, env, tt.args...)
			payload := requireReadSuccess(t, res, tt.dataKeys)

			switch tt.name {
			case "backups list":
				data := decodeBackups(t, payload, false)
				if len(data.Items) != 1 {
					t.Fatalf("items = %#v, want archive from --root", data.Items)
				}
			case "doctor":
				data := decodeDoctor(t, payload)
				check := doctorCheckByName(t, data, "root")
				if check.Status != "ok" {
					t.Fatalf("root check = %#v, want ok from --root", check)
				}
			}
		})
	}
}

func TestStatusAndLogsIgnoreRootContract(t *testing.T) {
	notDir := filepath.Join(t.TempDir(), "not-a-root")
	if err := os.WriteFile(notDir, []byte("x"), 0o600); err != nil {
		t.Fatalf("write non-directory root: %v", err)
	}
	env := map[string]string{"FAKE_JOURNALD_OUTPUT": "line one\n"}

	tests := []readCommandCase{
		{
			name:     "status",
			args:     []string{"status", "--root=" + notDir, "--json"},
			dataKeys: []string{"health", "started_at", "state", "unit"},
		},
		{
			name:     "logs",
			args:     []string{"logs", "--tail", "1", "--root=" + notDir, "--json"},
			dataKeys: []string{"lines"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := testHarness.RunArgsEnv(t, env, tt.args...)
			requireReadSuccess(t, res, tt.dataKeys)
		})
	}
}

func TestReadCommandServiceAvailabilityContract(t *testing.T) {
	t.Run("status", func(t *testing.T) {
		res := testHarness.RunEnv(t, map[string]string{
			"FAKE_SYSTEMD_MODE": "unavailable",
		}, "--json", "status")
		res.requireJSONError(t, 6, "service_unavailable")
	})

	t.Run("logs", func(t *testing.T) {
		res := testHarness.RunEnv(t, map[string]string{
			"FAKE_JOURNALD_MODE": "unavailable",
		}, "--json", "logs", "--tail", "10")
		res.requireJSONError(t, 6, "service_unavailable")
	})

	t.Run("backups list", func(t *testing.T) {
		root := backupsContractRoot(t, "mods")
		dir := filepath.Join(root, "backups", "mods")
		name := "mods-2026-10-22_12-00-00.tar.gz"
		writeContractBackup(t, dir, name, "archive", backupsContractBase)
		writeContractCurrent(t, dir, name)

		systemdLog := filepath.Join(t.TempDir(), "systemd.log")
		journalLog := filepath.Join(t.TempDir(), "journald.log")
		res := testHarness.RunRootEnv(t, root, map[string]string{
			"FAKE_SYSTEMD_MODE":  "unavailable",
			"FAKE_JOURNALD_MODE": "unavailable",
			"FAKE_SYSTEMD_LOG":   systemdLog,
			"FAKE_JOURNALD_LOG":  journalLog,
		}, "--json", "backups", "list", "--type", "mods")

		payload := requireReadSuccess(t, res,
			[]string{"current", "items", "next_before", "type"})
		data := decodeBackups(t, payload, false)
		if len(data.Items) != 1 {
			t.Fatalf("items = %#v, want one archive without systemd", data.Items)
		}
		if calls := readFakeCalls(t, systemdLog); calls != nil {
			t.Fatalf("backups list called systemctl: %#v", calls)
		}
		if calls := readFakeCalls(t, journalLog); calls != nil {
			t.Fatalf("backups list called journalctl: %#v", calls)
		}
	})

	t.Run("doctor remains diagnostic", func(t *testing.T) {
		root, env := doctorHealthyRoot(t)
		env["FAKE_SYSTEMD_MODE"] = "unavailable"
		res := testHarness.RunRootEnv(t, root, env, "--json", "doctor")

		res.requireExit(t, 10)
		if res.Stderr != "" {
			t.Fatalf("stderr = %q, want empty", res.Stderr)
		}
		payload := res.decodeEnvelope(t)
		if !payload.OK || payload.Error != nil {
			t.Fatalf("doctor failure must stay data-only; stdout=%q", res.Stdout)
		}
		data := decodeDoctor(t, payload)
		if data.Status != "error" {
			t.Fatalf("doctor status = %q, want error", data.Status)
		}
	})
}
