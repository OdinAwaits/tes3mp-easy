package ops

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/Saagilat/tes3mp-easy/test/fixtures"
)

type doctorCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type doctorPayload struct {
	Status string        `json:"status"`
	Checks []doctorCheck `json:"checks"`
}

func doctorRoot(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	for _, name := range []string{
		"configs", "players", "world", "mods", "backups", "panel-data",
	} {
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}
	return root
}

func doctorHealthyRoot(t *testing.T) (string, map[string]string) {
	t.Helper()
	return fixtures.DoctorRoot(t)
}

func doctorOSRelease(t *testing.T, content string) string {
	t.Helper()
	return fixtures.OSReleaseFile(t, content)
}

func cgroupCall(unit string) []string {
	return []string{"show", unit, "--property=MemoryHigh,MemoryMax"}
}

func decodeDoctor(t *testing.T, env envelope) doctorPayload {
	t.Helper()

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(env.Data, &raw); err != nil {
		t.Fatalf("decode doctor data: %v; data=%s", err, env.Data)
	}
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if !reflect.DeepEqual(keys, []string{"checks", "status"}) {
		t.Fatalf("doctor keys = %v, want [checks status]", keys)
	}

	var data doctorPayload
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("decode doctor data: %v", err)
	}
	if len(data.Checks) == 0 {
		t.Fatalf("doctor has no checks: %s", env.Data)
	}
	for _, check := range data.Checks {
		if check.Name == "" || check.Status == "" {
			t.Fatalf("incomplete check: %#v", check)
		}
	}
	return data
}

func doctorCheckByName(t *testing.T, data doctorPayload, name string) doctorCheck {
	t.Helper()

	for _, check := range data.Checks {
		if check.Name == name {
			return check
		}
	}
	t.Fatalf("doctor has no %q check: %#v", name, data.Checks)
	return doctorCheck{}
}

func TestDoctorOKContract(t *testing.T) {
	root, env := doctorHealthyRoot(t)
	callLog := filepath.Join(t.TempDir(), "calls.log")
	journalLog := filepath.Join(t.TempDir(), "journal.log")
	env["FAKE_SYSTEMD_LOG"] = callLog
	env["FAKE_JOURNALD_LOG"] = journalLog
	res := testHarness.RunRootEnv(t, root, env, "--json", "doctor")

	res.requireExit(t, 0)
	if res.Stderr != "" {
		t.Fatalf("stderr = %q, want empty", res.Stderr)
	}
	payload := res.decodeEnvelope(t)
	if !payload.OK {
		t.Fatalf("ok = false, want true; stdout=%q", res.Stdout)
	}
	data := decodeDoctor(t, payload)
	if data.Status != "ok" && data.Status != "warn" {
		t.Fatalf("status = %q, want ok or warn", data.Status)
	}
	if len(data.Checks) != 19 {
		t.Fatalf("checks = %d, want 19", len(data.Checks))
	}
	for _, name := range []string{
		"root", "os", "arch", "systemd", "units",
		"ownership", "journald", "runtime", "memory", "cgroup",
		"filesystems", "budgets", "current", "datafiles", "archives",
		"configs", "cleanup", "journald-config",
	} {
		check := doctorCheckByName(t, data, name)
		if check.Status != "ok" {
			t.Errorf("%s = %q (%s), want ok", name, check.Status, check.Message)
		}
	}

	requireCalls(t, callLog, [][]string{
		{"--version"},
		showCall(serviceUnit),
		showCall("tes3mp-panel.service"),
		showCall(timerUnit),
		cgroupCall(serviceUnit),
		cgroupCall("tes3mp-panel.service"),
		cgroupCall("tes3mp-easy-backup.service"),
	})
	requireCalls(t, journalLog, [][]string{{
		"-u", serviceUnit, "-n", "1", "--no-pager", "-o", "cat",
	}})
}

func TestDoctorUnknownOSContract(t *testing.T) {
	root, env := doctorHealthyRoot(t)
	env["TES3MP_EASY_OS_RELEASE"] = doctorOSRelease(t,
		"ID=cachyos\nVERSION_ID=rolling\n")
	res := testHarness.RunRootEnv(t, root, env, "--json", "doctor")

	res.requireExit(t, 0)
	data := decodeDoctor(t, res.decodeEnvelope(t))
	if data.Status != "warn" {
		t.Fatalf("status = %q, want warn", data.Status)
	}
	check := doctorCheckByName(t, data, "os")
	if check.Status != "warn" || !strings.Contains(check.Message, "unsupported") {
		t.Fatalf("os check = %#v, want unsupported warning", check)
	}
}

func TestDoctorNonX86Contract(t *testing.T) {
	root, extra := doctorHealthyRoot(t)
	extra["FAKE_UNAME_M"] = "aarch64"
	res := testHarness.RunRootEnv(t, root, extra, "--json", "doctor")

	res.requireExit(t, 10)
	env := res.decodeEnvelope(t)
	if !env.OK {
		t.Fatal("doctor error envelope must stay ok=true; diagnostics live in data.checks")
	}
	if env.Error != nil {
		t.Fatalf("doctor returned error object: %#v", env.Error)
	}
	data := decodeDoctor(t, env)
	if data.Status != "error" {
		t.Fatalf("status = %q, want error", data.Status)
	}
	check := doctorCheckByName(t, data, "arch")
	if check.Status != "error" || !strings.Contains(check.Message, "x86_64") {
		t.Fatalf("arch check = %#v, want non-x86_64 error", check)
	}
}

func TestDoctorSystemdUnavailableContract(t *testing.T) {
	root, extra := doctorHealthyRoot(t)
	callLog := filepath.Join(t.TempDir(), "calls.log")
	extra["FAKE_SYSTEMD_LOG"] = callLog
	extra["FAKE_SYSTEMD_MODE"] = "unavailable"
	res := testHarness.RunRootEnv(t, root, extra, "--json", "doctor")

	res.requireExit(t, 10)
	env := res.decodeEnvelope(t)
	if !env.OK || env.Error != nil {
		t.Fatalf("doctor failure must be data-only; stdout=%q", res.Stdout)
	}
	data := decodeDoctor(t, env)
	if data.Status != "error" {
		t.Fatalf("status = %q, want error", data.Status)
	}
	for _, name := range []string{"systemd", "units"} {
		check := doctorCheckByName(t, data, name)
		if check.Status != "error" {
			t.Errorf("%s = %#v, want error", name, check)
		}
	}
	requireCalls(t, callLog, [][]string{{"--version"}})
}

func TestDoctorInvalidArgumentContract(t *testing.T) {
	callLog := filepath.Join(t.TempDir(), "calls.log")
	res := testHarness.RunRootEnv(t, doctorRoot(t), map[string]string{
		"FAKE_SYSTEMD_LOG": callLog,
	}, "--json", "doctor", "--unknown")

	res.requireJSONError(t, 2, "invalid_argument")
	if calls := readFakeCalls(t, callLog); len(calls) != 0 {
		t.Fatalf("doctor called systemd for invalid arguments: %#v", calls)
	}
}

func TestDoctorDoesNotModifyRootContract(t *testing.T) {
	root, env := doctorHealthyRoot(t)
	before := snapshotTree(t, root)

	res := testHarness.RunRootEnv(t, root, env, "--json", "doctor")
	res.requireExit(t, 0)

	after := snapshotTree(t, root)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("doctor changed temp root\nbefore: %#v\nafter:  %#v", before, after)
	}
}
