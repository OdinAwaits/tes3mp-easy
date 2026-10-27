package ops

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
	"github.com/Saagilat/tes3mp-easy/internal/systemd"
	"github.com/Saagilat/tes3mp-easy/test/fixtures"
)

type stateDeployPayload struct {
	Archive         string `json:"archive"`
	ModsArchive     string `json:"mods_archive"`
	PredeployBackup string `json:"predeploy_backup"`
	Restarted       bool   `json:"restarted"`
}

func stateDeployContractRoot(t *testing.T) (string, fixtures.ModsManifest,
	fixtures.StateArchive, string,
) {
	t.Helper()

	root, manifest := createContractRoot(t)
	state := fixtures.FullState(manifest)
	state.Name = "state-manual-2026-10-22_13-00-00.tar.gz"
	state.Configs = []fixtures.StateFile{
		{Name: "config.lua", Body: "config = {}\n"},
		{Name: "tes3mp-server-default.cfg", Body: "[General]\n"},
		{Name: "banlist.json", Body: "{\"playerNames\":[],\"ipAddresses\":[]}\n"},
	}
	state.Players = []fixtures.StateFile{{
		Name: "player.json", Body: "{\"restored\":true}\n",
	}}
	state.World["cell"] = []fixtures.StateFile{{
		Name: "cell.json", Body: "restored cell\n",
	}}
	state.Write(t, filepath.Join(root, "backups", "state", state.Name))
	return root, manifest, state, state.Name
}

func decodeStateDeploy(t *testing.T, res result) (envelope, stateDeployPayload) {
	t.Helper()

	env := requireReadSuccess(t, res,
		[]string{"archive", "mods_archive", "predeploy_backup", "restarted"})
	var data stateDeployPayload
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("decode state deploy payload: %v", err)
	}
	return env, data
}

func assertStateDeployContractLeftovers(t *testing.T, root string) {
	t.Helper()

	for _, pattern := range []string{
		filepath.Join(root, "*.old-*"),
		filepath.Join(root, ".staging", "state-*"),
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

func assertStateDeployContractOld(t *testing.T, root string) {
	t.Helper()

	player, err := os.ReadFile(filepath.Join(root, "players", "player.json"))
	if err != nil || string(player) != "{}\n" {
		t.Fatalf("players/player.json = %q, %v; want old contents", player, err)
	}
	cell, err := os.ReadFile(filepath.Join(root, "world", "cell", "cell.json"))
	if err != nil || string(cell) != "{}\n" {
		t.Fatalf("world/cell/cell.json = %q, %v; want old contents", cell, err)
	}
}

func assertStateDeployPredeploy(t *testing.T, root, name string) {
	t.Helper()

	parsed, err := archive.ParseArchiveName(name)
	if err != nil {
		t.Fatalf("predeploy name: %v", err)
	}
	if parsed.Kind != archive.KindStatePredeploy {
		t.Fatalf("predeploy kind = %s, want %s", parsed.Kind,
			archive.KindStatePredeploy)
	}
	path := filepath.Join(root, "backups", "state", name)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("predeploy archive %s: %v", path, err)
	}
}

func TestStateDeployContract(t *testing.T) {
	root, manifest, _, stateName := stateDeployContractRoot(t)
	predeploy := fixtures.FullState(manifest)
	predeploy.Name = fixtures.StatePredeployName(
		time.Date(2026, 10, 22, 14, 0, 0, 0, time.UTC))
	predeploy.Players = []fixtures.StateFile{{
		Name: "player.json", Body: "{\"predeploy\":true}\n",
	}}
	predeploy.Write(t, filepath.Join(root, "backups", "state", predeploy.Name))
	currentBefore := fixtures.ReadCurrent(t,
		filepath.Join(root, "backups", "mods"))

	systemdLog := filepath.Join(t.TempDir(), "systemctl.log")
	sudoLog := filepath.Join(t.TempDir(), "sudo.log")
	res := testHarness.RunRootEnv(t, root, map[string]string{
		"FAKE_SYSTEMD_STATE": "active",
		"FAKE_SYSTEMD_LOG":   systemdLog,
		"FAKE_SUDO_LOG":      sudoLog,
	}, "--json", "state", "deploy", "--latest")
	_, data := decodeStateDeploy(t, res)

	if data.Archive != stateName {
		t.Fatalf("archive = %q, want manual %q (predeploy must be skipped)",
			data.Archive, stateName)
	}
	if data.ModsArchive != manifest.Name {
		t.Fatalf("mods_archive = %q, want %q", data.ModsArchive, manifest.Name)
	}
	assertStateDeployPredeploy(t, root, data.PredeployBackup)
	if !data.Restarted {
		t.Fatal("restarted = false, want true")
	}

	player, err := os.ReadFile(filepath.Join(root, "players", "player.json"))
	if err != nil || string(player) != "{\"restored\":true}\n" {
		t.Fatalf("restored player = %q, %v", player, err)
	}
	cell, err := os.ReadFile(filepath.Join(root, "world", "cell", "cell.json"))
	if err != nil || string(cell) != "restored cell\n" {
		t.Fatalf("restored cell = %q, %v", cell, err)
	}

	predeployPath := filepath.Join(root, "backups", "state", data.PredeployBackup)
	info, err := archive.ValidateStateFile(predeployPath,
		filepath.Join(root, "backups", "mods"))
	if err != nil {
		t.Fatalf("ValidateStateFile(predeploy): %v", err)
	}
	if info.Current.SHA256 != manifest.SHA256 ||
		info.Current.Name != manifest.Name {
		t.Fatalf("predeploy current = %+v, want %s/%s",
			info.Current, manifest.SHA256, manifest.Name)
	}
	if got := fixtures.ReadCurrent(t,
		filepath.Join(root, "backups", "mods")); got != currentBefore {
		t.Fatalf("mods current.txt = %q, want unchanged %q", got, currentBefore)
	}
	assertStateDeployContractLeftovers(t, root)

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

func TestStateDeployNoRestartContract(t *testing.T) {
	root, _, _, stateName := stateDeployContractRoot(t)
	sudoLog := filepath.Join(t.TempDir(), "sudo.log")

	res := testHarness.RunRootEnv(t, root, map[string]string{
		"FAKE_SYSTEMD_STATE": "active",
		"FAKE_SUDO_LOG":      sudoLog,
	}, "--json", "state", "deploy", "--archive", stateName, "--no-restart")
	_, data := decodeStateDeploy(t, res)

	if data.Archive != stateName || data.Restarted {
		t.Fatalf("data = %+v, want archive without restart", data)
	}
	wantSudo := [][]string{{"-n", systemd.RootctlPath, "stop"}}
	if calls := readFakeCalls(t, sudoLog); !reflect.DeepEqual(calls, wantSudo) {
		t.Fatalf("sudo calls = %#v, want %#v", calls, wantSudo)
	}
	player, err := os.ReadFile(filepath.Join(root, "players", "player.json"))
	if err != nil || string(player) != "{\"restored\":true}\n" {
		t.Fatalf("restored player = %q, %v", player, err)
	}
	assertStateDeployContractLeftovers(t, root)
}
