package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/systemd"
	"github.com/Saagilat/tes3mp-easy/internal/systemd/systemdtest"
	"github.com/Saagilat/tes3mp-easy/test/fixtures"
)

var modsDeployTestNow = time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)

func fixModsDeployHooks(t *testing.T) {
	t.Helper()

	oldNow := modsDeployNowFunc
	oldFree := modsDeployDiskFreeFunc
	oldBackupFree := backupDiskFreeFunc
	oldWrite := modsDeployWriteFileFunc
	modsDeployNowFunc = func() time.Time { return modsDeployTestNow }
	modsDeployDiskFreeFunc = func(string) (int64, error) { return 10 << 30, nil }
	backupDiskFreeFunc = func(string) (int64, error) { return 10 << 30, nil }
	modsDeployWriteFileFunc = atomicfile.WriteFile
	t.Cleanup(func() {
		modsDeployNowFunc = oldNow
		modsDeployDiskFreeFunc = oldFree
		backupDiskFreeFunc = oldBackupFree
		modsDeployWriteFileFunc = oldWrite
	})
}

func deployTestRoot(t *testing.T) (string, fixtures.ModsArchive,
	fixtures.ModsManifest, string,
) {
	t.Helper()

	root, current := backupCreateRoot(t)
	mods := fixtures.ScriptedMods()
	mods.Name = "mods-2026-10-22_13-00-00.tar.gz"
	path := filepath.Join(root, "backups", "mods", mods.Name)
	mods.Write(t, path)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read deploy archive: %v", err)
	}
	return root, mods, current, fixtures.SHA256Hex(body)
}

func activeRunner(t *testing.T) *systemdtest.Runner {
	t.Helper()

	runner := systemdtest.New()
	runner.Respond("systemctl", []string{"is-active", systemd.UnitTES3MPService},
		"active\n", nil)
	runner.Respond("sudo", []string{"-n", systemd.RootctlPath, "stop"}, "", nil)
	runner.Respond("sudo", []string{"-n", systemd.RootctlPath, "start"}, "", nil)
	return runner
}

func runDeployCommand(t *testing.T, root string, client *systemd.Client,
	args ...string,
) (int, modsDeployData, string, string) {
	t.Helper()

	full := append([]string{"--root", root, "--json", "mods", "deploy"}, args...)
	var stdout, stderr bytes.Buffer
	code := runWithClient(full, &stdout, &stderr, client)

	var data modsDeployData
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

func assertNoDeployLeftovers(t *testing.T, root string) {
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

func TestParseModsDeployArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		exit      ops.ExitCode
		archive   string
		latest    bool
		current   bool
		noRestart bool
	}{
		{
			name: "archive", args: []string{"--archive", "mods-a.tar.gz"},
			exit: ops.ExitOK, archive: "mods-a.tar.gz",
		},
		{
			name: "current", args: []string{"--current", "--no-restart"},
			exit: ops.ExitOK, current: true, noRestart: true,
		},
		{
			name: "latest", args: []string{"--latest"},
			exit: ops.ExitOK, latest: true,
		},
		{name: "no selector", exit: ops.ExitInvalidArgument},
		{
			name: "two selectors",
			args: []string{"--latest", "--current"},
			exit: ops.ExitInvalidArgument,
		},
		{
			name: "empty archive",
			args: []string{"--archive="},
			exit: ops.ExitInvalidArgument,
		},
		{
			name: "extra argument",
			args: []string{"--latest", "extra"},
			exit: ops.ExitInvalidArgument,
		},
		{
			name: "unknown flag",
			args: []string{"--bogus"},
			exit: ops.ExitInvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseModsDeployArgs(tt.args)
			if err != nil {
				if tt.exit == ops.ExitOK {
					t.Fatalf("parse error: %v", err)
				}
				if err.Exit != tt.exit {
					t.Fatalf("exit = %d, want %d", err.Exit, tt.exit)
				}
				if !strings.Contains(err.Message, "mods deploy") {
					t.Fatalf("message = %q, want command prefix", err.Message)
				}
				return
			}
			if tt.exit != ops.ExitOK {
				t.Fatal("parse succeeded, want error")
			}
			if got.archive != tt.archive || got.latest != tt.latest ||
				got.current != tt.current || got.noRestart != tt.noRestart {
				t.Fatalf("args = %+v, want archive=%q latest=%v current=%v no-restart=%v",
					got, tt.archive, tt.latest, tt.current, tt.noRestart)
			}
		})
	}
}

func TestRunModsDeployRestartsServer(t *testing.T) {
	fixModsDeployHooks(t)
	root, mods, current, sha := deployTestRoot(t)
	runner := activeRunner(t)

	code, data, stdout, stderr := runDeployCommand(t, root,
		systemd.New(runner), "--archive", mods.Name)
	if code != int(ops.ExitOK) {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q", code, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if data.Archive != mods.Name || data.SHA256 != sha {
		t.Fatalf("archive/sha = %q/%q, want %q/%q",
			data.Archive, data.SHA256, mods.Name, sha)
	}
	if data.Previous == nil || *data.Previous != current.Name {
		t.Fatalf("previous = %v, want %q", data.Previous, current.Name)
	}
	if data.StateBackup != fixtures.StatePredeployName(modsDeployTestNow) {
		t.Fatalf("state_backup = %q, want %q", data.StateBackup,
			fixtures.StatePredeployName(modsDeployTestNow))
	}
	if !data.Restarted {
		t.Fatal("restarted = false, want true")
	}

	pluginsDir := filepath.Join(root, "mods", "plugins")
	body, err := os.ReadFile(filepath.Join(pluginsDir, "mod.esp"))
	if err != nil || string(body) != "synthetic mod data\n" {
		t.Fatalf("installed plugin = %q, %v", body, err)
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
	if got := fixtures.ReadCurrent(t, modsDir); got != fixtures.CurrentLine(sha, mods.Name) {
		t.Fatalf("current.txt = %q, want %q", got,
			fixtures.CurrentLine(sha, mods.Name))
	}
	if _, err := os.Stat(filepath.Join(root, "backups", "state", data.StateBackup)); err != nil {
		t.Fatalf("predeploy archive: %v", err)
	}
	assertNoDeployLeftovers(t, root)

	wantCalls := []systemdtest.Call{
		{Name: "systemctl", Args: []string{"is-active", systemd.UnitTES3MPService}},
		{Name: "sudo", Args: []string{"-n", systemd.RootctlPath, "stop"}},
		{Name: "sudo", Args: []string{"-n", systemd.RootctlPath, "start"}},
	}
	if calls := runner.Calls(); !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", calls, wantCalls)
	}
}

func TestRunModsDeployInactiveServer(t *testing.T) {
	fixModsDeployHooks(t)
	root, mods, _, _ := deployTestRoot(t)
	runner := systemdtest.New()
	runner.Respond("systemctl", []string{"is-active", systemd.UnitTES3MPService},
		"inactive\n", commandError("systemctl",
			[]string{"is-active", systemd.UnitTES3MPService}, 3, ""))

	code, data, stdout, stderr := runDeployCommand(t, root,
		systemd.New(runner), "--latest")
	if code != int(ops.ExitOK) {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q", code, stdout, stderr)
	}
	if data.Archive != mods.Name || data.Restarted {
		t.Fatalf("data = %+v, want archive %s and restarted=false",
			data, mods.Name)
	}
	if _, err := os.Stat(filepath.Join(root, "mods", "plugins", "mod.esp")); err != nil {
		t.Fatalf("installed plugin: %v", err)
	}
	wantCalls := []systemdtest.Call{{
		Name: "systemctl",
		Args: []string{"is-active", systemd.UnitTES3MPService},
	}}
	if calls := runner.Calls(); !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", calls, wantCalls)
	}
}

func TestRunModsDeployNoRestart(t *testing.T) {
	fixModsDeployHooks(t)
	root, mods, _, _ := deployTestRoot(t)
	runner := activeRunner(t)

	code, data, stdout, stderr := runDeployCommand(t, root,
		systemd.New(runner), "--archive", mods.Name, "--no-restart")
	if code != int(ops.ExitOK) {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q", code, stdout, stderr)
	}
	if data.Restarted {
		t.Fatal("restarted = true, want false")
	}
	if _, err := os.Stat(filepath.Join(root, "mods", "plugins", "mod.esp")); err != nil {
		t.Fatalf("installed plugin: %v", err)
	}
	wantCalls := []systemdtest.Call{
		{Name: "systemctl", Args: []string{"is-active", systemd.UnitTES3MPService}},
		{Name: "sudo", Args: []string{"-n", systemd.RootctlPath, "stop"}},
	}
	if calls := runner.Calls(); !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %#v, want no start call", calls)
	}
}

func TestRunModsDeployRollbackOnWriteFailure(t *testing.T) {
	fixModsDeployHooks(t)
	root, mods, current, _ := deployTestRoot(t)
	runner := activeRunner(t)
	modsDeployWriteFileFunc = func(path string, data []byte, mode os.FileMode) error {
		if filepath.Base(path) == backupsCurrentFile {
			return syscall.ENOSPC
		}
		return atomicfile.WriteFile(path, data, mode)
	}

	code, _, stdout, stderr := runDeployCommand(t, root,
		systemd.New(runner), "--archive", mods.Name)
	if code != int(ops.ExitNoSpace) {
		t.Fatalf("exit = %d, want %d; stdout=%q stderr=%q",
			code, ops.ExitNoSpace, stdout, stderr)
	}
	assertJSONError(t, stdout, ops.ExitNoSpace, "write current.txt")

	modsDir := filepath.Join(root, "backups", "mods")
	if got := fixtures.ReadCurrent(t, modsDir); got != fixtures.CurrentLine(
		current.SHA256, current.Name) {
		t.Fatalf("current.txt = %q, want the original %q", got,
			fixtures.CurrentLine(current.SHA256, current.Name))
	}
	if _, err := os.Stat(filepath.Join(root, "mods", "plugins",
		"requiredDataFiles.json")); err != nil {
		t.Fatalf("old requiredDataFiles.json was not restored: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "mods", "plugins", "mod.esp")); !os.IsNotExist(err) {
		t.Fatalf("installed plugin survived rollback: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "mods", "scripts")); !os.IsNotExist(err) {
		t.Fatalf("installed scripts survived rollback: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "configs", "customScripts.lua")); !os.IsNotExist(err) {
		t.Fatalf("generated customScripts.lua survived rollback: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "backups", "state",
		fixtures.StatePredeployName(modsDeployTestNow))); err != nil {
		t.Fatalf("predeploy archive: %v", err)
	}
	assertNoDeployLeftovers(t, root)

	wantCalls := []systemdtest.Call{
		{Name: "systemctl", Args: []string{"is-active", systemd.UnitTES3MPService}},
		{Name: "sudo", Args: []string{"-n", systemd.RootctlPath, "stop"}},
		{Name: "sudo", Args: []string{"-n", systemd.RootctlPath, "start"}},
	}
	if calls := runner.Calls(); !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %#v, want stop and rollback restart", calls)
	}
}

func TestRunModsDeployServiceUnavailable(t *testing.T) {
	fixModsDeployHooks(t)
	root, mods, current, _ := deployTestRoot(t)
	runner := systemdtest.New()
	runner.Respond("systemctl", []string{"is-active", systemd.UnitTES3MPService},
		"", commandError("systemctl",
			[]string{"is-active", systemd.UnitTES3MPService}, 1,
			"Failed to connect to bus"))

	code, _, stdout, stderr := runDeployCommand(t, root,
		systemd.New(runner), "--archive", mods.Name)
	if code != int(ops.ExitServiceUnavailable) {
		t.Fatalf("exit = %d, want %d; stdout=%q stderr=%q",
			code, ops.ExitServiceUnavailable, stdout, stderr)
	}
	assertJSONError(t, stdout, ops.ExitServiceUnavailable, "systemctl")
	if _, err := os.Stat(filepath.Join(root, "mods", "plugins", "mod.esp")); !os.IsNotExist(err) {
		t.Fatalf("mods changed without a running service check: %v", err)
	}
	modsDir := filepath.Join(root, "backups", "mods")
	if got := fixtures.ReadCurrent(t, modsDir); got != fixtures.CurrentLine(
		current.SHA256, current.Name) {
		t.Fatalf("current.txt changed: %q", got)
	}
	assertNoDeployLeftovers(t, root)
}

func TestRunModsDeployDryRunChangesNothing(t *testing.T) {
	fixModsDeployHooks(t)
	root, mods, current, sha := deployTestRoot(t)
	runner := systemdtest.New()

	full := []string{"--root", root, "--json", "--dry-run",
		"mods", "deploy", "--archive", mods.Name}
	var stdout, stderr bytes.Buffer
	code := runWithClient(full, &stdout, &stderr, systemd.New(runner))
	if code != int(ops.ExitOK) {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q",
			code, stdout.String(), stderr.String())
	}
	var envelope struct {
		OK   bool            `json:"ok"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode envelope: %v; stdout=%q", err, stdout.String())
	}
	var data modsDeployData
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	if !envelope.OK || data.Archive != mods.Name || data.SHA256 != sha {
		t.Fatalf("data = %+v", data)
	}
	if data.StateBackup != fixtures.StatePredeployName(modsDeployTestNow) {
		t.Fatalf("state_backup = %q", data.StateBackup)
	}
	if calls := runner.Calls(); len(calls) != 0 {
		t.Fatalf("dry run called systemd: %#v", calls)
	}
	if _, err := os.Stat(filepath.Join(root, "mods", "plugins", "mod.esp")); !os.IsNotExist(err) {
		t.Fatalf("dry run installed a plugin: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "backups", "state",
		data.StateBackup)); !os.IsNotExist(err) {
		t.Fatalf("dry run created a state archive: %v", err)
	}
	modsDir := filepath.Join(root, "backups", "mods")
	if got := fixtures.ReadCurrent(t, modsDir); got != fixtures.CurrentLine(
		current.SHA256, current.Name) {
		t.Fatalf("current.txt = %q, want unchanged", got)
	}
	assertNoDeployLeftovers(t, root)
}

func TestRunModsDeployStopFailureChangesNothing(t *testing.T) {
	fixModsDeployHooks(t)
	root, mods, current, _ := deployTestRoot(t)
	runner := systemdtest.New()
	runner.Respond("systemctl", []string{"is-active", systemd.UnitTES3MPService},
		"active\n", nil)
	runner.Respond("sudo", []string{"-n", systemd.RootctlPath, "stop"},
		"", commandError("sudo", []string{"-n", systemd.RootctlPath, "stop"}, 1,
			"stop failed"))

	code, _, stdout, stderr := runDeployCommand(t, root,
		systemd.New(runner), "--archive", mods.Name)
	if code != int(ops.ExitServiceUnavailable) {
		t.Fatalf("exit = %d, want %d; stdout=%q stderr=%q",
			code, ops.ExitServiceUnavailable, stdout, stderr)
	}
	assertJSONError(t, stdout, ops.ExitServiceUnavailable, "stop")
	if _, err := os.Stat(filepath.Join(root, "mods", "plugins", "mod.esp")); !os.IsNotExist(err) {
		t.Fatalf("mods changed after a failed stop: %v", err)
	}
	modsDir := filepath.Join(root, "backups", "mods")
	if got := fixtures.ReadCurrent(t, modsDir); got != fixtures.CurrentLine(
		current.SHA256, current.Name) {
		t.Fatalf("current.txt changed: %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, "backups", "state",
		fixtures.StatePredeployName(modsDeployTestNow))); err != nil {
		t.Fatalf("predeploy archive: %v", err)
	}
	assertNoDeployLeftovers(t, root)

	wantCalls := []systemdtest.Call{
		{Name: "systemctl", Args: []string{"is-active", systemd.UnitTES3MPService}},
		{Name: "sudo", Args: []string{"-n", systemd.RootctlPath, "stop"}},
	}
	if calls := runner.Calls(); !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %#v, want only check and stop", calls)
	}
}

func TestRunModsDeployRollbackOnStartFailure(t *testing.T) {
	fixModsDeployHooks(t)
	root, mods, current, _ := deployTestRoot(t)
	customPath := filepath.Join(root, "configs", "customScripts.lua")
	fixtures.WriteFile(t, customPath, "old custom scripts\n")
	runner := systemdtest.New()
	runner.Respond("systemctl", []string{"is-active", systemd.UnitTES3MPService},
		"active\n", nil)
	runner.Respond("sudo", []string{"-n", systemd.RootctlPath, "stop"}, "", nil)
	runner.Respond("sudo", []string{"-n", systemd.RootctlPath, "start"},
		"", commandError("sudo", []string{"-n", systemd.RootctlPath, "start"}, 1,
			"start failed"))

	code, _, stdout, stderr := runDeployCommand(t, root,
		systemd.New(runner), "--archive", mods.Name)
	if code != int(ops.ExitServiceUnavailable) {
		t.Fatalf("exit = %d, want %d; stdout=%q stderr=%q",
			code, ops.ExitServiceUnavailable, stdout, stderr)
	}
	assertJSONError(t, stdout, ops.ExitServiceUnavailable, "start")
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}

	modsDir := filepath.Join(root, "backups", "mods")
	if got := fixtures.ReadCurrent(t, modsDir); got != fixtures.CurrentLine(
		current.SHA256, current.Name) {
		t.Fatalf("current.txt = %q, want the original", got)
	}
	if _, err := os.Stat(filepath.Join(root, "mods", "plugins",
		"requiredDataFiles.json")); err != nil {
		t.Fatalf("old requiredDataFiles.json was not restored: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "mods", "plugins", "mod.esp")); !os.IsNotExist(err) {
		t.Fatalf("installed plugin survived rollback: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "mods", "scripts")); !os.IsNotExist(err) {
		t.Fatalf("installed scripts survived rollback: %v", err)
	}
	custom, err := os.ReadFile(customPath)
	if err != nil || string(custom) != "old custom scripts\n" {
		t.Fatalf("customScripts.lua = %q, %v; want old contents", custom, err)
	}
	assertNoDeployLeftovers(t, root)

	wantCalls := []systemdtest.Call{
		{Name: "systemctl", Args: []string{"is-active", systemd.UnitTES3MPService}},
		{Name: "sudo", Args: []string{"-n", systemd.RootctlPath, "stop"}},
		{Name: "sudo", Args: []string{"-n", systemd.RootctlPath, "start"}},
		{Name: "sudo", Args: []string{"-n", systemd.RootctlPath, "start"}},
	}
	if calls := runner.Calls(); !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %#v, want a rollback restart", calls)
	}
}

func TestRunModsDeployRollbackOnCustomScriptsFailure(t *testing.T) {
	fixModsDeployHooks(t)
	root, mods, current, _ := deployTestRoot(t)
	customPath := filepath.Join(root, "configs", "customScripts.lua")
	fixtures.WriteFile(t, customPath, "old custom scripts\n")
	runner := activeRunner(t)
	modsDeployWriteFileFunc = func(path string, data []byte, mode os.FileMode) error {
		if filepath.Base(path) == "customScripts.lua" {
			return syscall.ENOSPC
		}
		return atomicfile.WriteFile(path, data, mode)
	}

	code, _, stdout, stderr := runDeployCommand(t, root,
		systemd.New(runner), "--archive", mods.Name)
	if code != int(ops.ExitNoSpace) {
		t.Fatalf("exit = %d, want %d; stdout=%q stderr=%q",
			code, ops.ExitNoSpace, stdout, stderr)
	}
	assertJSONError(t, stdout, ops.ExitNoSpace, "customScripts.lua")
	modsDir := filepath.Join(root, "backups", "mods")
	if got := fixtures.ReadCurrent(t, modsDir); got != fixtures.CurrentLine(
		current.SHA256, current.Name) {
		t.Fatalf("current.txt = %q, want unchanged", got)
	}
	custom, err := os.ReadFile(customPath)
	if err != nil || string(custom) != "old custom scripts\n" {
		t.Fatalf("customScripts.lua = %q, %v; want old contents", custom, err)
	}
	assertNoDeployLeftovers(t, root)
}

func TestRunModsDeployTargetErrorsDoNotTouchService(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		exit   ops.ExitCode
		want   string
		before func(t *testing.T, root string)
	}{
		{
			name: "missing archive", args: []string{"--archive",
				"mods-2026-10-22_14-00-00.tar.gz"},
			exit: ops.ExitNotFound, want: "does not exist",
		},
		{
			name: "invalid archive", args: []string{"--archive",
				"mods-2026-10-22_14-00-00.tar.gz"},
			exit: ops.ExitArchiveInvalid, want: "validate archive",
			before: func(t *testing.T, root string) {
				fixtures.WriteFile(t, filepath.Join(root, "backups", "mods",
					"mods-2026-10-22_14-00-00.tar.gz"), "not a tar.gz")
			},
		},
		{
			name: "unsafe archive name",
			args: []string{"--archive", "../mods-2026-10-22_14-00-00.tar.gz"},
			exit: ops.ExitUnsafePath, want: "unsafe",
		},
		{
			name: "no archives for latest", args: []string{"--latest"},
			exit: ops.ExitNotFound, want: "no mods archive",
			before: func(t *testing.T, root string) {
				dir := filepath.Join(root, "backups", "mods")
				entries, err := os.ReadDir(dir)
				if err != nil {
					t.Fatal(err)
				}
				for _, entry := range entries {
					if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil {
						t.Fatal(err)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixModsDeployHooks(t)
			root, _, current, _ := deployTestRoot(t)
			if tt.before != nil {
				tt.before(t, root)
			}
			runner := activeRunner(t)

			code, _, stdout, stderr := runDeployCommand(t, root,
				systemd.New(runner), tt.args...)
			if code != int(tt.exit) {
				t.Fatalf("exit = %d, want %d; stdout=%q stderr=%q",
					code, tt.exit, stdout, stderr)
			}
			assertJSONError(t, stdout, tt.exit, tt.want)
			if calls := runner.Calls(); len(calls) != 0 {
				t.Fatalf("target error called systemd: %#v", calls)
			}
			entries, err := os.ReadDir(filepath.Join(root, "backups", "state"))
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("state archives = %d, want none", len(entries))
			}
			if tt.name != "no archives for latest" {
				modsDir := filepath.Join(root, "backups", "mods")
				if got := fixtures.ReadCurrent(t, modsDir); got !=
					fixtures.CurrentLine(current.SHA256, current.Name) {
					t.Fatalf("current.txt changed: %q", got)
				}
			}
			assertNoDeployLeftovers(t, root)
		})
	}
}

func TestRunModsDeployCurrentWithoutMetadata(t *testing.T) {
	fixModsDeployHooks(t)
	root, _, current, _ := deployTestRoot(t)
	modsDir := filepath.Join(root, "backups", "mods")
	if err := os.Remove(filepath.Join(modsDir, fixtures.CurrentName)); err != nil {
		t.Fatal(err)
	}
	runner := activeRunner(t)

	code, _, stdout, stderr := runDeployCommand(t, root,
		systemd.New(runner), "--current")
	if code != int(ops.ExitArchiveInvalid) {
		t.Fatalf("exit = %d, want %d; stdout=%q stderr=%q",
			code, ops.ExitArchiveInvalid, stdout, stderr)
	}
	assertJSONError(t, stdout, ops.ExitArchiveInvalid, "current")
	if calls := runner.Calls(); len(calls) != 0 {
		t.Fatalf("missing current.txt called systemd: %#v", calls)
	}
	if _, err := os.Stat(filepath.Join(modsDir, current.Name)); err != nil {
		t.Fatalf("current archive disappeared: %v", err)
	}
	assertNoDeployLeftovers(t, root)
}

func TestRunModsDeployNoSpace(t *testing.T) {
	fixModsDeployHooks(t)
	root, mods, current, _ := deployTestRoot(t)
	runner := activeRunner(t)
	modsDeployDiskFreeFunc = func(string) (int64, error) { return 100, nil }

	code, _, stdout, stderr := runDeployCommand(t, root,
		systemd.New(runner), "--archive", mods.Name)
	if code != int(ops.ExitNoSpace) {
		t.Fatalf("exit = %d, want %d; stdout=%q stderr=%q",
			code, ops.ExitNoSpace, stdout, stderr)
	}
	assertJSONError(t, stdout, ops.ExitNoSpace, "free space")
	if calls := runner.Calls(); len(calls) != 0 {
		t.Fatalf("mods deploy called systemd without enough disk: %#v", calls)
	}
	entries, err := os.ReadDir(filepath.Join(root, "backups", "state"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("state archives = %d, want none", len(entries))
	}
	modsDir := filepath.Join(root, "backups", "mods")
	if got := fixtures.ReadCurrent(t, modsDir); got != fixtures.CurrentLine(
		current.SHA256, current.Name) {
		t.Fatalf("current.txt = %q, want unchanged", got)
	}
	assertNoDeployLeftovers(t, root)
}

func TestLatestModsArchive(t *testing.T) {
	dir := t.TempDir()
	now := modsDeployTestNow
	old := fixtures.ModsName(now.Add(-24 * time.Hour))
	newer := fixtures.ModsName(now)
	collision := strings.TrimSuffix(newer, ".tar.gz") + "-02.tar.gz"
	for _, name := range []string{
		old,
		newer,
		collision,
		fixtures.ModsInitName(now.Add(-48 * time.Hour)),
		fixtures.StateName(now),
		"current.txt",
		"mods-not-a-date.tar.gz",
	} {
		fixtures.WriteFile(t, filepath.Join(dir, name), "")
	}

	got, err := latestModsArchive(dir)
	if err != nil {
		t.Fatalf("latestModsArchive: %v", err)
	}
	if got != collision {
		t.Fatalf("latest = %q, want %q", got, collision)
	}

	empty := t.TempDir()
	_, err = latestModsArchive(empty)
	if err == nil || err.Exit != ops.ExitNotFound {
		t.Fatalf("empty latest error = %v, want not_found", err)
	}
}

func TestWriteModsDeployHuman(t *testing.T) {
	previous := "mods-old.tar.gz"
	var out bytes.Buffer
	data := modsDeployData{
		Archive:     "mods-new.tar.gz",
		SHA256:      strings.Repeat("a", 64),
		Previous:    &previous,
		StateBackup: "state-predeploy-2026-10-22_12-00-00.tar.gz",
		Restarted:   true,
	}
	if err := writeModsDeployHuman(&out, data); err != nil {
		t.Fatalf("writeModsDeployHuman: %v", err)
	}
	want := "archive: mods-new.tar.gz\n" +
		"sha256: " + strings.Repeat("a", 64) + "\n" +
		"previous: mods-old.tar.gz\n" +
		"state_backup: state-predeploy-2026-10-22_12-00-00.tar.gz\n" +
		"restarted: true\n"
	if out.String() != want {
		t.Fatalf("human output = %q, want %q", out.String(), want)
	}

	out.Reset()
	data.Previous = nil
	data.Restarted = false
	if err := writeModsDeployHuman(&out, data); err != nil {
		t.Fatalf("writeModsDeployHuman without previous: %v", err)
	}
	if !strings.Contains(out.String(), "previous: -\n") ||
		!strings.Contains(out.String(), "restarted: false\n") {
		t.Fatalf("human output = %q", out.String())
	}
}
