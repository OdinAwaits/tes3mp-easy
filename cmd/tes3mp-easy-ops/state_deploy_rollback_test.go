package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/systemd"
	"github.com/Saagilat/tes3mp-easy/internal/systemd/systemdtest"
	"github.com/Saagilat/tes3mp-easy/test/fixtures"
)

func assertStateDeployOldState(t *testing.T, root string) {
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

func TestRunStateDeployRollbackOnExtractionFailure(t *testing.T) {
	fixStateDeployHooks(t)
	root, _, _, _ := stateDeployRoot(t)
	runner := activeRunner(t)
	stateDeployExtractFunc = func(string, string) error {
		return &os.PathError{Op: "write", Path: "staging",
			Err: syscall.ENOSPC}
	}

	code, _, stdout, stderr := runStateDeployCommand(t, root,
		systemd.New(runner), "--latest")
	if code != int(ops.ExitNoSpace) {
		t.Fatalf("exit = %d, want %d; stdout=%q stderr=%q",
			code, ops.ExitNoSpace, stdout, stderr)
	}
	assertJSONError(t, stdout, ops.ExitNoSpace, "extract archive")
	assertStateDeployOldState(t, root)
	if _, err := os.Stat(filepath.Join(root, "backups", "state",
		fixtures.StatePredeployName(stateDeployTestNow))); err != nil {
		t.Fatalf("predeploy archive: %v", err)
	}
	assertNoStateDeployLeftovers(t, root)

	wantCalls := []systemdtest.Call{
		{Name: "systemctl", Args: []string{"is-active", systemd.UnitTES3MPService}},
		{Name: "sudo", Args: []string{"-n", systemd.RootctlPath, "stop"}},
		{Name: "sudo", Args: []string{"-n", systemd.RootctlPath, "start"}},
	}
	if calls := runner.Calls(); !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %#v, want a rollback restart", calls)
	}
}

func TestRunStateDeployRollbackOnSwitchFailure(t *testing.T) {
	fixStateDeployHooks(t)
	root, _, _, _ := stateDeployRoot(t)
	runner := activeRunner(t)
	stateDeployRenameFunc = func(oldPath, newPath string) error {
		if strings.Contains(oldPath, string(filepath.Separator)+".staging"+
			string(filepath.Separator)) &&
			strings.HasSuffix(newPath, string(filepath.Separator)+"players") {
			return syscall.ENOSPC
		}
		return os.Rename(oldPath, newPath)
	}

	code, _, stdout, stderr := runStateDeployCommand(t, root,
		systemd.New(runner), "--latest")
	if code != int(ops.ExitNoSpace) {
		t.Fatalf("exit = %d, want %d; stdout=%q stderr=%q",
			code, ops.ExitNoSpace, stdout, stderr)
	}
	assertJSONError(t, stdout, ops.ExitNoSpace, "install state")
	assertStateDeployOldState(t, root)
	assertNoStateDeployLeftovers(t, root)

	wantCalls := []systemdtest.Call{
		{Name: "systemctl", Args: []string{"is-active", systemd.UnitTES3MPService}},
		{Name: "sudo", Args: []string{"-n", systemd.RootctlPath, "stop"}},
		{Name: "sudo", Args: []string{"-n", systemd.RootctlPath, "start"}},
	}
	if calls := runner.Calls(); !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %#v, want a rollback restart", calls)
	}
}

func TestRunStateDeployRollbackOnStartFailure(t *testing.T) {
	fixStateDeployHooks(t)
	root, _, _, _ := stateDeployRoot(t)
	runner := systemdtest.New()
	runner.Respond("systemctl", []string{"is-active", systemd.UnitTES3MPService},
		"active\n", nil)
	runner.Respond("sudo", []string{"-n", systemd.RootctlPath, "stop"}, "", nil)
	runner.Respond("sudo", []string{"-n", systemd.RootctlPath, "start"},
		"", commandError("sudo", []string{"-n", systemd.RootctlPath, "start"}, 1,
			"start failed"))

	code, _, stdout, stderr := runStateDeployCommand(t, root,
		systemd.New(runner), "--latest")
	if code != int(ops.ExitServiceUnavailable) {
		t.Fatalf("exit = %d, want %d; stdout=%q stderr=%q",
			code, ops.ExitServiceUnavailable, stdout, stderr)
	}
	assertJSONError(t, stdout, ops.ExitServiceUnavailable, "start")
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	assertStateDeployOldState(t, root)
	assertNoStateDeployLeftovers(t, root)

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
