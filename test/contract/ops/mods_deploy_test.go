package ops

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
	"github.com/Saagilat/tes3mp-easy/internal/systemd"
	"github.com/Saagilat/tes3mp-easy/test/fixtures"
)

type modsDeployPayload struct {
	Archive     string  `json:"archive"`
	SHA256      string  `json:"sha256"`
	Previous    *string `json:"previous"`
	StateBackup string  `json:"state_backup"`
	Restarted   bool    `json:"restarted"`
}

func modsDeployContractRoot(t *testing.T) (string, fixtures.ModsManifest,
	fixtures.ModsManifest,
) {
	t.Helper()

	root, previous := createContractRoot(t)
	target := fixtures.ScriptedMods()
	target.Name = "mods-2026-10-22_13-00-00.tar.gz"
	path := filepath.Join(root, "backups", "mods", target.Name)
	target.Write(t, path)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read deploy archive: %v", err)
	}
	return root, target.Manifest(body), previous
}

func decodeModsDeploy(t *testing.T, res result) (envelope, modsDeployPayload) {
	t.Helper()

	env := requireReadSuccess(t, res,
		[]string{"archive", "previous", "restarted", "sha256", "state_backup"})
	var data modsDeployPayload
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("decode deploy payload: %v", err)
	}
	return env, data
}

func assertModsDeployLeftovers(t *testing.T, root string) {
	t.Helper()

	for _, pattern := range []string{
		filepath.Join(root, "mods", "*.old-*"),
		filepath.Join(root, ".staging", "mods-*"),
	} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("glob %s: %v", pattern, err)
		}
		if len(matches) != 0 {
			t.Fatalf("leftovers for %s: %v", pattern, matches)
		}
	}
}

func TestModsDeployContract(t *testing.T) {
	root, target, previous := modsDeployContractRoot(t)
	systemdLog := filepath.Join(t.TempDir(), "systemctl.log")
	sudoLog := filepath.Join(t.TempDir(), "sudo.log")

	res := testHarness.RunRootEnv(t, root, map[string]string{
		"FAKE_SYSTEMD_STATE": "active",
		"FAKE_SYSTEMD_LOG":   systemdLog,
		"FAKE_SUDO_LOG":      sudoLog,
	}, "--json", "mods", "deploy", "--latest")
	_, data := decodeModsDeploy(t, res)

	if data.Archive != target.Name || data.SHA256 != target.SHA256 {
		t.Fatalf("archive/sha = %q/%q, want %q/%q",
			data.Archive, data.SHA256, target.Name, target.SHA256)
	}
	if data.Previous == nil || *data.Previous != previous.Name {
		t.Fatalf("previous = %v, want %q", data.Previous, previous.Name)
	}
	if !data.Restarted {
		t.Fatal("restarted = false, want true")
	}
	parsed, err := archive.ParseArchiveName(data.StateBackup)
	if err != nil {
		t.Fatalf("state_backup name: %v", err)
	}
	if parsed.Kind != archive.KindStatePredeploy {
		t.Fatalf("state_backup kind = %s, want %s",
			parsed.Kind, archive.KindStatePredeploy)
	}

	plugin, err := os.ReadFile(filepath.Join(root, "mods", "plugins", "mod.esp"))
	if err != nil || string(plugin) != "synthetic mod data\n" {
		t.Fatalf("installed plugin = %q, %v", plugin, err)
	}
	if _, err := os.Stat(filepath.Join(root, "mods", "scripts", "test.lua")); err != nil {
		t.Fatalf("installed script: %v", err)
	}
	custom, err := os.ReadFile(filepath.Join(root, "configs", "customScripts.lua"))
	if err != nil {
		t.Fatalf("read customScripts.lua: %v", err)
	}
	wantCustom := "require(\"custom.other_1-Lua\")\nrequire(\"custom.test\")\n"
	if string(custom) != wantCustom {
		t.Fatalf("customScripts.lua = %q, want %q", custom, wantCustom)
	}

	modsDir := filepath.Join(root, "backups", "mods")
	wantCurrent := fixtures.CurrentLine(target.SHA256, target.Name)
	if got := fixtures.ReadCurrent(t, modsDir); got != wantCurrent {
		t.Fatalf("current.txt = %q, want %q", got, wantCurrent)
	}

	statePath := filepath.Join(root, "backups", "state", data.StateBackup)
	reader, err := archive.Open(statePath)
	if err != nil {
		t.Fatalf("open predeploy state: %v", err)
	}
	info, validateErr := archive.ValidateState(reader)
	closeErr := reader.Close()
	if validateErr != nil || closeErr != nil {
		t.Fatalf("validate predeploy state: %v (close: %v)",
			validateErr, closeErr)
	}
	if info.Current.Name != previous.Name ||
		info.Current.SHA256 != previous.SHA256 {
		t.Fatalf("predeploy current = %+v, want %q/%q",
			info.Current, previous.Name, previous.SHA256)
	}
	assertModsDeployLeftovers(t, root)

	wantSystemd := [][]string{{"is-active", "tes3mp.service"}}
	if calls := readFakeCalls(t, systemdLog); !reflect.DeepEqual(calls, wantSystemd) {
		t.Fatalf("systemctl calls = %#v, want %#v", calls, wantSystemd)
	}
	wantSudo := [][]string{
		{"-n", systemd.RootctlPath, "stop"},
		{"-n", systemd.RootctlPath, "start"},
	}
	if calls := readFakeCalls(t, sudoLog); !reflect.DeepEqual(calls, wantSudo) {
		t.Fatalf("sudo calls = %#v, want %#v", calls, wantSudo)
	}
}

func TestModsDeployServiceUnavailableContract(t *testing.T) {
	root, target, previous := modsDeployContractRoot(t)

	res := testHarness.RunRootEnv(t, root, map[string]string{
		"FAKE_SYSTEMD_MODE": "unavailable",
	}, "--json", "mods", "deploy", "--archive", target.Name)
	res.requireJSONError(t, 6, "service_unavailable")

	modsDir := filepath.Join(root, "backups", "mods")
	if got := fixtures.ReadCurrent(t, modsDir); got != fixtures.CurrentLine(
		previous.SHA256, previous.Name) {
		t.Fatalf("current.txt changed after service failure: %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, "mods", "plugins",
		"requiredDataFiles.json")); err != nil {
		t.Fatalf("old requiredDataFiles.json was not restored: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "mods", "plugins",
		"mod.esp")); !os.IsNotExist(err) {
		t.Fatalf("new plugin was installed after service failure: %v", err)
	}

	entries, err := os.ReadDir(filepath.Join(root, "backups", "state"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("predeploy archives = %d, want one", len(entries))
	}
	assertModsDeployLeftovers(t, root)
}

func TestModsDeployCurrentWithoutMetadataContract(t *testing.T) {
	root, previous := createContractRoot(t)
	if err := os.Remove(filepath.Join(root, "backups", "mods",
		fixtures.CurrentName)); err != nil {
		t.Fatal(err)
	}

	res := testHarness.RunRoot(t, root, "--json", "mods", "deploy", "--current")
	res.requireJSONError(t, 7, "archive_invalid")

	if _, err := os.Stat(filepath.Join(root, "backups", "mods",
		previous.Name)); err != nil {
		t.Fatalf("previous mods archive was removed: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(root, "backups", "state"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("state archives = %d, want none", len(entries))
	}
}

func TestModsDeployInvalidTargetsContract(t *testing.T) {
	tests := []struct {
		name string
		args []string
		exit int
		code string
	}{
		{
			name: "traversal",
			args: []string{"--archive", "../mods.tar.gz"},
			exit: 8,
			code: "unsafe_path",
		},
		{
			name: "not an archive",
			args: []string{"--archive", "not-a-mods-archive.tar.gz"},
			exit: 7,
			code: "archive_invalid",
		},
		{
			name: "missing archive",
			args: []string{"--archive", "mods-2026-10-23_00-00-00.tar.gz"},
			exit: 3,
			code: "not_found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, _, previous := modsDeployContractRoot(t)
			args := append([]string{"--json", "mods", "deploy"}, tt.args...)
			res := testHarness.RunRoot(t, root, args...)
			res.requireJSONError(t, tt.exit, tt.code)

			if got := fixtures.ReadCurrent(t, filepath.Join(root, "backups", "mods")); got != fixtures.CurrentLine(previous.SHA256, previous.Name) {
				t.Fatalf("current.txt changed after failed deploy: %q", got)
			}
			assertModsDeployLeftovers(t, root)
		})
	}
}

func TestModsDeployNoArchivesForLatestContract(t *testing.T) {
	root, _, _ := modsDeployContractRoot(t)
	modsDir := filepath.Join(root, "backups", "mods")
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if err := os.Remove(filepath.Join(modsDir, entry.Name())); err != nil {
			t.Fatal(err)
		}
	}

	res := testHarness.RunRoot(t, root, "--json", "mods", "deploy", "--latest")
	res.requireJSONError(t, 3, "not_found")
}

func TestModsDeployLockedContract(t *testing.T) {
	root, target, previous := modsDeployContractRoot(t)
	holdOpsLock(t, root)

	res := testHarness.RunRoot(t, root, "--lock-timeout=0",
		"--json", "mods", "deploy", "--archive", target.Name)
	res.requireJSONError(t, 4, "locked")

	if got := fixtures.ReadCurrent(t, filepath.Join(root, "backups", "mods")); got != fixtures.CurrentLine(previous.SHA256, previous.Name) {
		t.Fatalf("current.txt changed after lock failure: %q", got)
	}
	entries, err := os.ReadDir(filepath.Join(root, "backups", "state"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("state archives = %d, want none", len(entries))
	}
}
