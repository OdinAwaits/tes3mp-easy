package ops

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"syscall"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/systemd"
)

type controlData struct {
	Unit   string `json:"unit"`
	Action string `json:"action"`
	DryRun bool   `json:"dry_run"`
}

func TestControlSuccessContract(t *testing.T) {
	for _, action := range []string{"start", "stop", "restart"} {
		t.Run(action, func(t *testing.T) {
			root := t.TempDir()
			sudoLog := filepath.Join(t.TempDir(), "sudo.log")
			res := testHarness.RunRootEnv(t, root, map[string]string{
				"FAKE_SUDO_LOG":  sudoLog,
				"FAKE_SUDO_MODE": "ok",
			}, "--json", action)
			res.requireExit(t, 0)
			if res.Stderr != "" {
				t.Fatalf("stderr = %q, want empty", res.Stderr)
			}

			env := res.decodeEnvelope(t)
			var data controlData
			if err := json.Unmarshal(env.Data, &data); err != nil {
				t.Fatalf("decode data: %v; data=%s", err, env.Data)
			}
			if data.Unit != systemd.UnitTES3MPService || data.Action != action ||
				data.DryRun {
				t.Fatalf("data = %+v, want unit/action without dry_run", data)
			}

			var object map[string]any
			if err := json.Unmarshal(env.Data, &object); err != nil {
				t.Fatalf("decode object: %v", err)
			}
			wantKeys := []string{"action", "unit"}
			if keys := sortedKeys(object); !reflect.DeepEqual(keys, wantKeys) {
				t.Fatalf("data keys = %v, want %v", keys, wantKeys)
			}

			wantCalls := [][]string{{"-n", systemd.RootctlPath, action}}
			if calls := readFakeCalls(t, sudoLog); !reflect.DeepEqual(calls, wantCalls) {
				t.Fatalf("sudo calls = %#v, want %#v", calls, wantCalls)
			}
		})
	}
}

func TestControlDryRunContract(t *testing.T) {
	root := t.TempDir()
	sudoLog := filepath.Join(t.TempDir(), "sudo.log")
	res := testHarness.RunRootEnv(t, root, map[string]string{
		"FAKE_SUDO_LOG": sudoLog,
	}, "--json", "--dry-run", "restart")
	res.requireExit(t, 0)

	env := res.decodeEnvelope(t)
	var data controlData
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("decode data: %v; data=%s", err, env.Data)
	}
	if data.Unit != systemd.UnitTES3MPService ||
		data.Action != "restart" || !data.DryRun {
		t.Fatalf("data = %+v, want dry_run restart", data)
	}
	if calls := readFakeCalls(t, sudoLog); len(calls) != 0 {
		t.Fatalf("sudo calls = %#v, want none", calls)
	}
}

func TestControlFailureContract(t *testing.T) {
	root := t.TempDir()
	res := testHarness.RunRootEnv(t, root, map[string]string{
		"FAKE_SUDO_MODE": "fail",
	}, "--json", "stop")
	env := res.requireJSONError(t, 6, "service_unavailable")
	if !strings.Contains(env.Error.Message, "rootctl failed") {
		t.Fatalf("error.message = %q, want rootctl stderr", env.Error.Message)
	}
}

func TestControlArgumentContract(t *testing.T) {
	res := testHarness.Run(t, "--json", "start", "extra")
	res.requireJSONError(t, 2, "invalid_argument")
}

func TestControlHumanContract(t *testing.T) {
	res := testHarness.Run(t, "--dry-run", "start")
	res.requireExit(t, 0)
	want := "unit: tes3mp.service\naction: start\ndry_run: true\nresult: planned\n"
	if res.Stdout != want {
		t.Fatalf("stdout = %q, want %q", res.Stdout, want)
	}
}

func TestControlRespectsOpsLock(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".locks")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("create .locks: %v", err)
	}
	lockFile, err := os.OpenFile(filepath.Join(dir, "ops.lock"),
		os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("open ops.lock: %v", err)
	}
	defer lockFile.Close()
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatalf("flock ops.lock: %v", err)
	}

	sudoLog := filepath.Join(t.TempDir(), "sudo.log")
	res := testHarness.RunRootEnv(t, root, map[string]string{
		"FAKE_SUDO_LOG": sudoLog,
	}, "--json", "--lock-timeout", "10ms", "restart")
	res.requireJSONError(t, 4, "locked")
	if calls := readFakeCalls(t, sudoLog); len(calls) != 0 {
		t.Fatalf("sudo calls = %#v, want none", calls)
	}
}

func sortedKeys(object map[string]any) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
