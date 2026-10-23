package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/systemd"
	"github.com/Saagilat/tes3mp-easy/internal/systemd/systemdtest"
	"github.com/Saagilat/tes3mp-easy/test/fixtures"
)

func TestRunStateDeployDryRunChangesNothing(t *testing.T) {
	fixStateDeployHooks(t)
	root, manifest, _, stateName := stateDeployRoot(t)
	runner := systemdtest.New()

	full := []string{"--root", root, "--json", "--dry-run",
		"state", "deploy", "--latest"}
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
	var data stateDeployData
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	if !envelope.OK || data.Archive != stateName ||
		data.ModsArchive != manifest.Name || data.Restarted {
		t.Fatalf("data = %+v", data)
	}
	if data.PredeployBackup != fixtures.StatePredeployName(stateDeployTestNow) {
		t.Fatalf("predeploy_backup = %q", data.PredeployBackup)
	}
	if calls := runner.Calls(); len(calls) != 0 {
		t.Fatalf("dry run called systemd: %#v", calls)
	}
	if _, err := os.Stat(filepath.Join(root, "backups", "state",
		data.PredeployBackup)); !os.IsNotExist(err) {
		t.Fatalf("dry run created a predeploy archive: %v", err)
	}
	assertStateDeployOldState(t, root)
	assertNoStateDeployLeftovers(t, root)
}

func TestRunStateDeployNoSpace(t *testing.T) {
	fixStateDeployHooks(t)
	root, _, _, _ := stateDeployRoot(t)
	runner := activeRunner(t)
	modsDeployDiskFreeFunc = func(string) (int64, error) { return 100, nil }

	code, _, stdout, stderr := runStateDeployCommand(t, root,
		systemd.New(runner), "--latest")
	if code != int(ops.ExitNoSpace) {
		t.Fatalf("exit = %d, want %d; stdout=%q stderr=%q",
			code, ops.ExitNoSpace, stdout, stderr)
	}
	assertJSONError(t, stdout, ops.ExitNoSpace, "free space")
	if calls := runner.Calls(); len(calls) != 0 {
		t.Fatalf("no-space deploy called systemd: %#v", calls)
	}
	if _, err := os.Stat(filepath.Join(root, "backups", "state",
		fixtures.StatePredeployName(stateDeployTestNow))); !os.IsNotExist(err) {
		t.Fatalf("no-space deploy created a predeploy archive: %v", err)
	}
	assertStateDeployOldState(t, root)
	assertNoStateDeployLeftovers(t, root)
}

func TestRunStateDeployStopFailureChangesNothing(t *testing.T) {
	fixStateDeployHooks(t)
	root, _, _, _ := stateDeployRoot(t)
	runner := systemdtest.New()
	runner.Respond("systemctl", []string{"is-active", systemd.UnitTES3MPService},
		"active\n", nil)
	runner.Respond("sudo", []string{"-n", systemd.RootctlPath, "stop"},
		"", commandError("sudo", []string{"-n", systemd.RootctlPath, "stop"}, 1,
			"stop failed"))

	code, _, stdout, stderr := runStateDeployCommand(t, root,
		systemd.New(runner), "--latest")
	if code != int(ops.ExitServiceUnavailable) {
		t.Fatalf("exit = %d, want %d; stdout=%q stderr=%q",
			code, ops.ExitServiceUnavailable, stdout, stderr)
	}
	assertJSONError(t, stdout, ops.ExitServiceUnavailable, "stop")
	assertStateDeployOldState(t, root)
	if _, err := os.Stat(filepath.Join(root, "backups", "state",
		fixtures.StatePredeployName(stateDeployTestNow))); err != nil {
		t.Fatalf("predeploy archive: %v", err)
	}
	assertNoStateDeployLeftovers(t, root)

	wantCalls := []systemdtest.Call{
		{Name: "systemctl", Args: []string{"is-active", systemd.UnitTES3MPService}},
		{Name: "sudo", Args: []string{"-n", systemd.RootctlPath, "stop"}},
	}
	if calls := runner.Calls(); !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %#v, want only check and stop", calls)
	}
}
