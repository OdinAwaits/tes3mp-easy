package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/systemd"
	"github.com/Saagilat/tes3mp-easy/internal/systemd/systemdtest"
)

func TestRunControlJSON(t *testing.T) {
	for _, action := range []string{"start", "stop", "restart"} {
		t.Run(action, func(t *testing.T) {
			root := t.TempDir()
			runner := systemdtest.New()
			runner.Respond("sudo", []string{"-n", systemd.RootctlPath, action},
				"", nil)

			var stdout, stderr bytes.Buffer
			code := runWithClient([]string{"--root", root, "--json", action},
				&stdout, &stderr, systemd.New(runner))
			if code != int(ops.ExitOK) {
				t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q",
					code, stdout.String(), stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}

			var data controlData
			decodeJSONData(t, stdout.String(), &data)
			if data.Unit != systemd.UnitTES3MPService ||
				data.Action != action || data.DryRun {
				t.Fatalf("data = %+v, want unit/action without dry_run", data)
			}

			want := []systemdtest.Call{{
				Name: "sudo",
				Args: []string{"-n", systemd.RootctlPath, action},
			}}
			if calls := runner.Calls(); !reflect.DeepEqual(calls, want) {
				t.Fatalf("calls = %#v, want %#v", calls, want)
			}
		})
	}
}

func TestRunControlDryRunDoesNotCallRootctl(t *testing.T) {
	root := t.TempDir()
	runner := systemdtest.New()

	var stdout, stderr bytes.Buffer
	code := runWithClient([]string{"--root", root, "--json", "--dry-run", "stop"},
		&stdout, &stderr, systemd.New(runner))
	if code != int(ops.ExitOK) {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q",
			code, stdout.String(), stderr.String())
	}

	var data controlData
	decodeJSONData(t, stdout.String(), &data)
	if data.Unit != systemd.UnitTES3MPService ||
		data.Action != "stop" || !data.DryRun {
		t.Fatalf("data = %+v, want dry_run stop", data)
	}
	if calls := runner.Calls(); len(calls) != 0 {
		t.Fatalf("calls = %#v, want none", calls)
	}
}

func TestRunControlFailureIsServiceUnavailable(t *testing.T) {
	root := t.TempDir()
	runner := systemdtest.New()
	args := []string{"-n", systemd.RootctlPath, "start"}
	runner.Respond("sudo", args, "",
		commandError("sudo", args, 1, "rootctl: systemd is not running"))

	var stdout, stderr bytes.Buffer
	code := runWithClient([]string{"--root", root, "--json", "start"},
		&stdout, &stderr, systemd.New(runner))
	if code != int(ops.ExitServiceUnavailable) {
		t.Fatalf("exit = %d, want %d; stdout=%q stderr=%q",
			code, ops.ExitServiceUnavailable, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty in JSON mode", stderr.String())
	}

	var envelope struct {
		OK    bool `json:"ok"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode envelope: %v; stdout=%q", err, stdout.String())
	}
	if envelope.OK || envelope.Error.Code != ops.CodeServiceUnavailable {
		t.Fatalf("envelope = %+v, want service_unavailable", envelope)
	}
	if !strings.Contains(envelope.Error.Message, "start:") ||
		!strings.Contains(envelope.Error.Message, "systemd is not running") {
		t.Fatalf("message = %q, want command and rootctl stderr", envelope.Error.Message)
	}
}

func TestRunControlRejectsArguments(t *testing.T) {
	root := t.TempDir()
	runner := systemdtest.New()

	var stdout, stderr bytes.Buffer
	code := runWithClient([]string{"--root", root, "--json", "restart", "extra"},
		&stdout, &stderr, systemd.New(runner))
	if code != int(ops.ExitInvalidArgument) {
		t.Fatalf("exit = %d, want %d; stdout=%q stderr=%q",
			code, ops.ExitInvalidArgument, stdout.String(), stderr.String())
	}
	if calls := runner.Calls(); len(calls) != 0 {
		t.Fatalf("calls = %#v, want none", calls)
	}
}

func TestRunControlRespectsHeldLock(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".locks")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("create .locks: %v", err)
	}
	// A shared lock held by this test blocks the exclusive mutation lock.
	path := filepath.Join(dir, "ops.lock")
	lockFile, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("open lock: %v", err)
	}
	defer lockFile.Close()
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatalf("flock: %v", err)
	}

	runner := systemdtest.New()
	var stdout, stderr bytes.Buffer
	code := runWithClient([]string{
		"--root", root, "--json", "--lock-timeout", "10ms", "start",
	}, &stdout, &stderr, systemd.New(runner))
	if code != int(ops.ExitLocked) {
		t.Fatalf("exit = %d, want %d; stdout=%q stderr=%q",
			code, ops.ExitLocked, stdout.String(), stderr.String())
	}
	if calls := runner.Calls(); len(calls) != 0 {
		t.Fatalf("calls = %#v, want none", calls)
	}
}

func TestRunControlHuman(t *testing.T) {
	root := t.TempDir()
	runner := systemdtest.New()
	runner.Respond("sudo", []string{"-n", systemd.RootctlPath, "start"}, "", nil)

	var stdout, stderr bytes.Buffer
	code := runWithClient([]string{"--root", root, "--dry-run", "start"},
		&stdout, &stderr, systemd.New(runner))
	if code != int(ops.ExitOK) {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q",
			code, stdout.String(), stderr.String())
	}
	want := fmt.Sprintf("unit: %s\naction: start\ndry_run: true\nresult: planned\n",
		systemd.UnitTES3MPService)
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}
