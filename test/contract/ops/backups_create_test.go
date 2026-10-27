package ops

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
	"github.com/Saagilat/tes3mp-easy/test/fixtures"
)

type createPayload struct {
	Archive  string   `json:"archive"`
	Size     *int64   `json:"size"`
	SHA256   *string  `json:"sha256"`
	Skipped  bool     `json:"skipped"`
	Reason   string   `json:"reason"`
	Pruned   []string `json:"pruned"`
	Warnings []string `json:"warnings"`
}

var createWorldDirs = []string{"cell", "world", "map", "recordstore", "custom"}

func createContractRoot(t *testing.T) (string, fixtures.ModsManifest) {
	t.Helper()

	root := t.TempDir()
	for _, dir := range []string{
		"players",
		"world",
		"configs",
		"mods/plugins",
		"backups/mods",
		"backups/state",
		".locks",
	} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o750); err != nil {
			t.Fatalf("create %s: %v", dir, err)
		}
	}
	for _, name := range createWorldDirs {
		if err := os.MkdirAll(filepath.Join(root, "world", name), 0o750); err != nil {
			t.Fatalf("create world/%s: %v", name, err)
		}
	}
	fixtures.WriteConfigs(t, filepath.Join(root, "configs"))

	mods := fixtures.DefaultModsInit()
	manifest := mods.Install(t, filepath.Join(root, "backups", "mods"))
	fixtures.WriteFile(t,
		filepath.Join(root, "mods", "plugins", "requiredDataFiles.json"),
		string(mods.RequiredJSON()))
	fixtures.WriteFile(t, filepath.Join(root, "players", "player.json"), "{}\n")
	for _, name := range createWorldDirs {
		fixtures.WriteFile(t,
			filepath.Join(root, "world", name, name+".json"), "{}\n")
	}
	return root, manifest
}

func createFileSHA256(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return fixtures.SHA256Hex(data)
}

func decodeCreate(t *testing.T, env envelope, skipped bool) createPayload {
	t.Helper()

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(env.Data, &raw); err != nil {
		t.Fatalf("decode create data: %v; data=%s", err, env.Data)
	}
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	want := []string{"archive", "pruned", "sha256", "size", "skipped", "warnings"}
	if skipped {
		want = []string{"pruned", "reason", "skipped", "warnings"}
	}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("create keys = %v, want %v", keys, want)
	}

	var data createPayload
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("decode create payload: %v", err)
	}
	return data
}

func TestBackupsCreateContract(t *testing.T) {
	tests := []struct {
		name string
		args []string
		kind archive.ArchiveKind
	}{
		{name: "automatic", args: []string{"--type", "state"},
			kind: archive.KindStateAuto},
		{name: "manual", args: []string{"--type", "state", "--manual"},
			kind: archive.KindStateManual},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, _ := createContractRoot(t)
			args := append([]string{"--json", "backups", "create"}, tt.args...)
			res := testHarness.RunRoot(t, root, args...)
			env := requireReadSuccess(t, res,
				[]string{"archive", "pruned", "sha256", "size", "skipped", "warnings"})
			data := decodeCreate(t, env, false)
			if data.Skipped || data.Size == nil || data.SHA256 == nil {
				t.Fatalf("data = %+v, want a created archive", data)
			}

			parsed, err := archive.ParseArchiveName(data.Archive)
			if err != nil {
				t.Fatalf("archive name: %v", err)
			}
			if parsed.Kind != tt.kind {
				t.Fatalf("kind = %s, want %s", parsed.Kind, tt.kind)
			}

			path := filepath.Join(root, "backups", "state", data.Archive)
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("stat archive: %v", err)
			}
			if info.Size() != *data.Size {
				t.Fatalf("size = %d, want %d", info.Size(), *data.Size)
			}
			sum := createFileSHA256(t, path)
			if sum != *data.SHA256 {
				t.Fatalf("sha256 = %s, want %s", sum, *data.SHA256)
			}
			if _, err := archive.ValidateStateFile(path,
				filepath.Join(root, "backups", "mods")); err != nil {
				t.Fatalf("ValidateStateFile: %v", err)
			}
			if _, err := os.Stat(path + ".part"); !os.IsNotExist(err) {
				t.Fatalf(".part exists after create: %v", err)
			}
			if _, err := os.Stat(filepath.Join(root, ".locks", "ops.lock")); err != nil {
				t.Fatalf("ops lock was not created: %v", err)
			}
		})
	}
}

func TestBackupsCreateIfRunningSkippedContract(t *testing.T) {
	root, _ := createContractRoot(t)
	callsPath := filepath.Join(t.TempDir(), "systemctl.log")
	res := testHarness.RunRootEnv(t, root, map[string]string{
		"FAKE_SYSTEMD_STATE": "inactive",
		"FAKE_SYSTEMD_LOG":   callsPath,
	}, "--json", "backups", "create", "--type", "state", "--if-running")

	env := requireReadSuccess(t, res,
		[]string{"pruned", "reason", "skipped", "warnings"})
	data := decodeCreate(t, env, true)
	if !data.Skipped || data.Reason != "not_running" {
		t.Fatalf("data = %+v, want skipped not_running", data)
	}
	entries, err := os.ReadDir(filepath.Join(root, "backups", "state"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("state archives = %d, want none", len(entries))
	}

	wantCalls := [][]string{{"is-active", "tes3mp.service"}}
	if calls := readFakeCalls(t, callsPath); !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("systemctl calls = %#v, want %#v", calls, wantCalls)
	}
}

func TestBackupsCreateServiceUnavailableContract(t *testing.T) {
	root, _ := createContractRoot(t)
	res := testHarness.RunRootEnv(t, root, map[string]string{
		"FAKE_SYSTEMD_MODE": "unavailable",
	}, "--json", "backups", "create", "--type", "state", "--if-running")
	res.requireJSONError(t, 6, "service_unavailable")

	entries, err := os.ReadDir(filepath.Join(root, "backups", "state"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("state archives = %d, want none", len(entries))
	}
}

func TestBackupsCreateLockedContract(t *testing.T) {
	root, _ := createContractRoot(t)
	holdOpsLock(t, root)

	res := testHarness.RunRoot(t, root, "--lock-timeout=0",
		"--json", "backups", "create", "--type", "state")
	res.requireJSONError(t, 4, "locked")

	entries, err := os.ReadDir(filepath.Join(root, "backups", "state"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("state archives = %d, want none", len(entries))
	}
}

func TestBackupsCreateDryRunContract(t *testing.T) {
	root, _ := createContractRoot(t)
	before := snapshotTree(t, root)

	res := testHarness.RunRoot(t, root, "--dry-run",
		"--json", "backups", "create", "--type", "state")
	env := requireReadSuccess(t, res,
		[]string{"archive", "pruned", "sha256", "size", "skipped", "warnings"})
	data := decodeCreate(t, env, false)
	if data.Skipped || data.Archive == "" {
		t.Fatalf("data = %+v, want a dry-run plan", data)
	}

	after := snapshotTree(t, root)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("dry-run changed temp root\nbefore: %#v\nafter:  %#v",
			before, after)
	}
}

func TestBackupsCreateInvalidCurrentContract(t *testing.T) {
	root, _ := createContractRoot(t)
	if err := os.Remove(filepath.Join(root, "backups", "mods", "current.txt")); err != nil {
		t.Fatal(err)
	}
	res := testHarness.RunRoot(t, root,
		"--json", "backups", "create", "--type", "state")
	res.requireJSONError(t, 7, "archive_invalid")
}

func TestBackupsCreateRejectsInvalidArgumentsContract(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing type", args: []string{"backups", "create"}},
		{name: "mods type", args: []string{"backups", "create", "--type", "mods"}},
		{name: "unknown flag", args: []string{
			"backups", "create", "--type", "state", "--bogus",
		}},
		{name: "extra argument", args: []string{
			"backups", "create", "--type", "state", "extra",
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, _ := createContractRoot(t)
			args := append([]string{"--json"}, tt.args...)
			res := testHarness.RunRoot(t, root, args...)
			res.requireJSONError(t, 2, "invalid_argument")
		})
	}
}
