package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/systemd"
	"github.com/Saagilat/tes3mp-easy/internal/systemd/systemdtest"
	"github.com/Saagilat/tes3mp-easy/test/fixtures"
)

var stateDeployTestNow = time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)

func fixStateDeployHooks(t *testing.T) {
	t.Helper()

	oldNow := stateDeployNowFunc
	oldFree := modsDeployDiskFreeFunc
	oldBackupFree := backupDiskFreeFunc
	oldExtract := stateDeployExtractFunc
	oldRename := stateDeployRenameFunc
	stateDeployNowFunc = func() time.Time { return stateDeployTestNow }
	modsDeployDiskFreeFunc = func(string) (int64, error) { return 10 << 30, nil }
	backupDiskFreeFunc = func(string) (int64, error) { return 10 << 30, nil }
	stateDeployExtractFunc = archive.ExtractStateFile
	stateDeployRenameFunc = os.Rename
	t.Cleanup(func() {
		stateDeployNowFunc = oldNow
		modsDeployDiskFreeFunc = oldFree
		backupDiskFreeFunc = oldBackupFree
		stateDeployExtractFunc = oldExtract
		stateDeployRenameFunc = oldRename
	})
}

func stateDeployRoot(t *testing.T) (string, fixtures.ModsManifest,
	fixtures.StateArchive, string,
) {
	t.Helper()

	root, manifest := backupCreateRoot(t)
	state := fixtures.FullState(manifest)
	state.Name = "state-manual-2026-10-22_13-00-00.tar.gz"
	state.Players = []fixtures.StateFile{{
		Name: "player.json", Body: "{\"restored\":true}\n",
	}}
	state.World["cell"] = []fixtures.StateFile{{
		Name: "cell.json", Body: "restored cell\n",
	}}
	state.Write(t, filepath.Join(root, "backups", "state", state.Name))
	return root, manifest, state, state.Name
}

func runStateDeployCommand(t *testing.T, root string, client *systemd.Client,
	args ...string,
) (int, stateDeployData, string, string) {
	t.Helper()

	full := append([]string{"--root", root, "--json", "state", "deploy"}, args...)
	var stdout, stderr bytes.Buffer
	code := runWithClient(full, &stdout, &stderr, client)

	var data stateDeployData
	if stdout.Len() > 0 {
		var envelope struct {
			OK   bool            `json:"ok"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
			t.Fatalf("decode envelope: %v; stdout=%q", err, stdout.String())
		}
		if envelope.OK {
			if err := json.Unmarshal(envelope.Data, &data); err != nil {
				t.Fatalf("decode data: %v; data=%s", err, envelope.Data)
			}
		}
	}
	return code, data, stdout.String(), stderr.String()
}

func assertNoStateDeployLeftovers(t *testing.T, root string) {
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

func TestParseStateDeployArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		exit      ops.ExitCode
		archive   string
		latest    bool
		noRestart bool
	}{
		{name: "archive", args: []string{"--archive", "state-manual-x.tar.gz"},
			exit: ops.ExitOK, archive: "state-manual-x.tar.gz"},
		{name: "latest", args: []string{"--latest"}, exit: ops.ExitOK,
			latest: true},
		{name: "no restart", args: []string{"--latest", "--no-restart"},
			exit: ops.ExitOK, latest: true, noRestart: true},
		{name: "missing selector", exit: ops.ExitInvalidArgument},
		{name: "empty archive", args: []string{"--archive="},
			exit: ops.ExitInvalidArgument},
		{name: "both selectors", args: []string{"--latest", "--archive", "a"},
			exit: ops.ExitInvalidArgument},
		{name: "unknown flag", args: []string{"--bogus"},
			exit: ops.ExitInvalidArgument},
		{name: "extra argument", args: []string{"--latest", "extra"},
			exit: ops.ExitInvalidArgument},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseStateDeployArgs(tt.args)
			if err != nil {
				if tt.exit == ops.ExitOK {
					t.Fatalf("parse error: %v", err)
				}
				if err.Exit != tt.exit {
					t.Fatalf("exit = %d, want %d", err.Exit, tt.exit)
				}
				if !strings.Contains(err.Message, "state deploy") {
					t.Fatalf("message = %q, want command prefix", err.Message)
				}
				return
			}
			if tt.exit != ops.ExitOK {
				t.Fatal("parse succeeded, want error")
			}
			if got.archive != tt.archive || got.latest != tt.latest ||
				got.noRestart != tt.noRestart {
				t.Fatalf("args = %+v", got)
			}
		})
	}
}

func TestRunStateDeployRestartsServer(t *testing.T) {
	fixStateDeployHooks(t)
	root, manifest, _, stateName := stateDeployRoot(t)
	runner := activeRunner(t)

	code, data, stdout, stderr := runStateDeployCommand(t, root,
		systemd.New(runner), "--latest")
	if code != int(ops.ExitOK) {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q", code, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if data.Archive != stateName || data.ModsArchive != manifest.Name {
		t.Fatalf("archive/mods = %q/%q, want %q/%q",
			data.Archive, data.ModsArchive, stateName, manifest.Name)
	}
	if data.PredeployBackup != fixtures.StatePredeployName(stateDeployTestNow) {
		t.Fatalf("predeploy = %q", data.PredeployBackup)
	}
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
	if _, err := os.Stat(filepath.Join(root, "backups", "state",
		data.PredeployBackup)); err != nil {
		t.Fatalf("predeploy archive: %v", err)
	}
	if got := fixtures.ReadCurrent(t, filepath.Join(root, "backups", "mods")); got !=
		fixtures.CurrentLine(manifest.SHA256, manifest.Name) {
		t.Fatalf("current.txt changed: %q", got)
	}
	assertNoStateDeployLeftovers(t, root)

	wantCalls := []systemdtest.Call{
		{Name: "systemctl", Args: []string{"is-active", systemd.UnitTES3MPService}},
		{Name: "sudo", Args: []string{"-n", systemd.RootctlPath, "stop"}},
		{Name: "sudo", Args: []string{"-n", systemd.RootctlPath, "start"}},
	}
	if calls := runner.Calls(); !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %#v, want stop and start", calls)
	}
}

func TestRunStateDeployWorksWithoutRootWrite(t *testing.T) {
	fixStateDeployHooks(t)
	root, _, _, _ := stateDeployRoot(t)
	runner := activeRunner(t)
	root = filepath.Clean(root)

	var rootRenames []string
	stateDeployRenameFunc = func(oldPath, newPath string) error {
		if filepath.Dir(filepath.Clean(oldPath)) == root ||
			filepath.Dir(filepath.Clean(newPath)) == root {
			rootRenames = append(rootRenames, oldPath+" -> "+newPath)
			return &os.PathError{Op: "rename", Path: oldPath,
				Err: os.ErrPermission}
		}
		return os.Rename(oldPath, newPath)
	}

	code, _, stdout, stderr := runStateDeployCommand(t, root,
		systemd.New(runner), "--latest")
	if code != int(ops.ExitOK) {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q", code, stdout, stderr)
	}
	if len(rootRenames) != 0 {
		t.Fatalf("state deploy renamed entries in the data root: %v", rootRenames)
	}
	player, err := os.ReadFile(filepath.Join(root, "players", "player.json"))
	if err != nil || string(player) != "{\"restored\":true}\n" {
		t.Fatalf("restored player = %q, %v", player, err)
	}
	cell, err := os.ReadFile(filepath.Join(root, "world", "cell", "cell.json"))
	if err != nil || string(cell) != "restored cell\n" {
		t.Fatalf("restored cell = %q, %v", cell, err)
	}
	assertNoStateDeployLeftovers(t, root)
}

func TestRunStateDeployWithoutRootWritePermission(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses permission checks")
	}
	fixStateDeployHooks(t)
	root, _, _, _ := stateDeployRoot(t)
	runner := activeRunner(t)
	if err := os.Mkdir(filepath.Join(root, ".staging"), 0o700); err != nil {
		t.Fatalf("create staging parent: %v", err)
	}
	if err := os.Chmod(root, 0o555); err != nil {
		t.Fatalf("make root read-only: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(root, 0o755)
	})

	code, _, stdout, stderr := runStateDeployCommand(t, root,
		systemd.New(runner), "--latest")
	if code != int(ops.ExitOK) {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q", code, stdout, stderr)
	}
	player, err := os.ReadFile(filepath.Join(root, "players", "player.json"))
	if err != nil || string(player) != "{\"restored\":true}\n" {
		t.Fatalf("restored player = %q, %v", player, err)
	}
	assertNoStateDeployLeftovers(t, root)
}

func TestRunStateDeployNoRestart(t *testing.T) {
	fixStateDeployHooks(t)
	root, _, _, _ := stateDeployRoot(t)
	runner := activeRunner(t)

	code, data, stdout, stderr := runStateDeployCommand(t, root,
		systemd.New(runner), "--latest", "--no-restart")
	if code != int(ops.ExitOK) {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q", code, stdout, stderr)
	}
	if data.Restarted {
		t.Fatal("restarted = true, want false")
	}
	wantCalls := []systemdtest.Call{
		{Name: "systemctl", Args: []string{"is-active", systemd.UnitTES3MPService}},
		{Name: "sudo", Args: []string{"-n", systemd.RootctlPath, "stop"}},
	}
	if calls := runner.Calls(); !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %#v, want no start", calls)
	}
}

func TestRunStateDeployInactiveServer(t *testing.T) {
	fixStateDeployHooks(t)
	root, _, _, _ := stateDeployRoot(t)
	runner := systemdtest.New()
	runner.Respond("systemctl", []string{"is-active", systemd.UnitTES3MPService},
		"inactive\n", nil)

	code, data, stdout, stderr := runStateDeployCommand(t, root,
		systemd.New(runner), "--latest")
	if code != int(ops.ExitOK) {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q", code, stdout, stderr)
	}
	if data.Restarted {
		t.Fatal("restarted = true, want false")
	}
	wantCalls := []systemdtest.Call{
		{Name: "systemctl", Args: []string{"is-active", systemd.UnitTES3MPService}},
	}
	if calls := runner.Calls(); !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %#v, want only is-active", calls)
	}
}

func TestLatestStateArchiveSkipsPredeploy(t *testing.T) {
	dir := t.TempDir()
	now := stateDeployTestNow
	manual := fixtures.StateManualName(now)
	auto := fixtures.StateName(now.Add(-time.Hour))
	predeploy := fixtures.StatePredeployName(now.Add(time.Hour))
	collision := strings.TrimSuffix(manual, ".tar.gz") + "-03.tar.gz"
	for _, name := range []string{
		manual, auto, predeploy, collision, "current.txt",
		"state-not-a-date.tar.gz",
	} {
		fixtures.WriteFile(t, filepath.Join(dir, name), "")
	}

	got, err := latestStateArchive(dir)
	if err != nil {
		t.Fatalf("latestStateArchive: %v", err)
	}
	if got != collision {
		t.Fatalf("latest = %q, want %q", got, collision)
	}

	_, err = latestStateArchive(t.TempDir())
	if err == nil || err.Exit != ops.ExitNotFound {
		t.Fatalf("empty latest error = %v, want not_found", err)
	}
}

func TestWriteStateDeployHuman(t *testing.T) {
	var out bytes.Buffer
	data := stateDeployData{
		Archive:         "state-manual-2026-10-22_12-00-00.tar.gz",
		ModsArchive:     "mods-2026-10-22_12-00-00.tar.gz",
		PredeployBackup: "state-predeploy-2026-10-22_12-00-00.tar.gz",
		Restarted:       true,
	}
	if err := writeStateDeployHuman(&out, data); err != nil {
		t.Fatalf("writeStateDeployHuman: %v", err)
	}
	want := "archive: state-manual-2026-10-22_12-00-00.tar.gz\n" +
		"mods_archive: mods-2026-10-22_12-00-00.tar.gz\n" +
		"predeploy_backup: state-predeploy-2026-10-22_12-00-00.tar.gz\n" +
		"restarted: true\n"
	if out.String() != want {
		t.Fatalf("human output = %q, want %q", out.String(), want)
	}
}
