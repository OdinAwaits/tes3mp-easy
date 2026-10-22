package systemd_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/systemd"
	"github.com/Saagilat/tes3mp-easy/internal/systemd/systemdtest"
)

func showArgs(unit string) []string {
	return []string{
		"show", unit,
		"--property=" + systemd.ShowProperties,
		"--timestamp=utc",
	}
}

func commandError(name string, args []string, exitCode int, stderr string) error {
	return &systemd.CommandError{
		Name:     name,
		Args:     append([]string(nil), args...),
		ExitCode: exitCode,
		Stderr:   stderr,
		Err:      fmt.Errorf("exit status %d", exitCode),
	}
}

func assertCalls(t *testing.T, got, want []systemdtest.Call) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %#v, want %#v", got, want)
	}
}

func TestClientShow(t *testing.T) {
	runner := systemdtest.New()
	runner.Respond("systemctl", showArgs(systemd.UnitTES3MPService),
		"ActiveState=active\n"+
			"SubState=running\n"+
			"ActiveEnterTimestamp=Thu 2026-10-22 12:00:00 UTC\n"+
			"LastTriggerUSec=n/a\n"+
			"LoadState=loaded\n"+
			"UnitFileState=enabled\n", nil)

	client := systemd.New(runner)
	properties, err := client.Show(context.Background(), systemd.UnitTES3MPService)
	if err != nil {
		t.Fatalf("Show: %v", err)
	}

	want := systemd.Properties{
		"ActiveState":          "active",
		"SubState":             "running",
		"ActiveEnterTimestamp": "Thu 2026-10-22 12:00:00 UTC",
		"LastTriggerUSec":      "n/a",
		"LoadState":            "loaded",
		"UnitFileState":        "enabled",
	}
	if !reflect.DeepEqual(properties, want) {
		t.Fatalf("properties = %#v, want %#v", properties, want)
	}
	if got := properties.Get("ActiveState"); got != "active" {
		t.Fatalf("Get(ActiveState) = %q, want active", got)
	}
	if got := properties.Get("Missing"); got != "" {
		t.Fatalf("Get(Missing) = %q, want empty", got)
	}

	assertCalls(t, runner.Calls(), []systemdtest.Call{{
		Name: "systemctl",
		Args: showArgs(systemd.UnitTES3MPService),
	}})
}

func TestClientShowMalformedOutput(t *testing.T) {
	runner := systemdtest.New()
	runner.Respond("systemctl", showArgs(systemd.UnitBackupTimer),
		"ActiveState=active\nbroken\n", nil)

	_, err := systemd.New(runner).Show(context.Background(), systemd.UnitBackupTimer)
	if err == nil || !strings.Contains(err.Error(), "line 2 is not KEY=VALUE") {
		t.Fatalf("Show error = %v, want malformed property line", err)
	}
}

func TestClientRejectsInvalidUnit(t *testing.T) {
	units := []string{
		"",
		"--help",
		"../tes3mp.service",
		".hidden.service",
		"tes3mp/service",
		"tes3mp service",
		strings.Repeat("a", 257),
	}

	for _, unit := range units {
		t.Run(unit, func(t *testing.T) {
			runner := systemdtest.New()
			client := systemd.New(runner)

			if _, err := client.Show(context.Background(), unit); !errors.Is(err, systemd.ErrInvalidUnit) {
				t.Fatalf("Show error = %v, want ErrInvalidUnit", err)
			}
			if _, err := client.IsActive(context.Background(), unit); !errors.Is(err, systemd.ErrInvalidUnit) {
				t.Fatalf("IsActive error = %v, want ErrInvalidUnit", err)
			}
			if _, err := client.Logs(context.Background(), unit, 10); !errors.Is(err, systemd.ErrInvalidUnit) {
				t.Fatalf("Logs error = %v, want ErrInvalidUnit", err)
			}
			if calls := runner.Calls(); len(calls) != 0 {
				t.Fatalf("invalid unit reached runner: %+v", calls)
			}
		})
	}
}

func TestClientIsActive(t *testing.T) {
	args := []string{"is-active", systemd.UnitTES3MPService}
	tests := []struct {
		name    string
		stdout  string
		err     error
		want    bool
		wantErr bool
	}{
		{name: "active", stdout: "active\n", want: true},
		{name: "inactive", stdout: "inactive\n",
			err: commandError("systemctl", args, 4, ""), want: false},
		{name: "failed", stdout: "failed\n",
			err: commandError("systemctl", args, 3, ""), want: false},
		{name: "unknown state", stdout: "unknown\n",
			err: commandError("systemctl", args, 4, ""), want: false},
		{name: "systemd failure", stdout: "unknown\n",
			err: commandError("systemctl", args, 1,
				"Failed to connect to bus: No such file or directory"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := systemdtest.New()
			runner.Respond("systemctl", args, tt.stdout, tt.err)

			active, err := systemd.New(runner).IsActive(
				context.Background(), systemd.UnitTES3MPService)
			if (err != nil) != tt.wantErr {
				t.Fatalf("IsActive error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				var commandErr *systemd.CommandError
				if !errors.As(err, &commandErr) {
					t.Fatalf("IsActive error type = %T, want *CommandError", err)
				}
			}
			if active != tt.want {
				t.Fatalf("IsActive = %v, want %v", active, tt.want)
			}
		})
	}
}

func TestClientLogs(t *testing.T) {
	args := []string{
		"-u", systemd.UnitTES3MPService,
		"-n", "200",
		"--no-pager",
		"-o", "cat",
	}
	runner := systemdtest.New()
	runner.Respond("journalctl", args,
		"2026-10-22 12:00:00 | server started\n"+
			"2026-10-22 12:01:00 | player joined\r\n", nil)

	lines, err := systemd.New(runner).Logs(
		context.Background(), systemd.UnitTES3MPService, 200)
	if err != nil {
		t.Fatalf("Logs: %v", err)
	}

	want := []string{
		"2026-10-22 12:00:00 | server started",
		"2026-10-22 12:01:00 | player joined",
	}
	if !reflect.DeepEqual(lines, want) {
		t.Fatalf("lines = %#v, want %#v", lines, want)
	}
	assertCalls(t, runner.Calls(), []systemdtest.Call{{
		Name: "journalctl",
		Args: args,
	}})
}

func TestClientLogsEdgeCases(t *testing.T) {
	runner := systemdtest.New()
	runner.Respond("journalctl", []string{
		"-u", systemd.UnitTES3MPService,
		"-n", "1",
		"--no-pager",
		"-o", "cat",
	}, "", nil)

	lines, err := systemd.New(runner).Logs(
		context.Background(), systemd.UnitTES3MPService, 1)
	if err != nil {
		t.Fatalf("Logs: %v", err)
	}
	if lines == nil || len(lines) != 0 {
		t.Fatalf("empty logs = %#v, want non-nil empty slice", lines)
	}

	for _, tail := range []int{0, -1} {
		runner.Reset()
		if _, err := systemd.New(runner).Logs(
			context.Background(), systemd.UnitTES3MPService, tail); err == nil {
			t.Fatalf("Logs accepted tail %d", tail)
		}
		if calls := runner.Calls(); len(calls) != 0 {
			t.Fatalf("invalid tail %d reached runner: %+v", tail, calls)
		}
	}

	args := []string{
		"-u", systemd.UnitTES3MPService,
		"-n", "10",
		"--no-pager",
		"-o", "cat",
	}
	runner.Respond("journalctl", args, "", commandError(
		"journalctl", args, 1, "Failed to open journal"))
	if _, err := systemd.New(runner).Logs(
		context.Background(), systemd.UnitTES3MPService, 10); err == nil {
		t.Fatal("Logs swallowed a journalctl failure")
	}
}
func TestClientControl(t *testing.T) {
	tests := []struct {
		action systemd.ControlAction
	}{
		{action: systemd.ActionStart},
		{action: systemd.ActionStop},
		{action: systemd.ActionRestart},
	}

	for _, tt := range tests {
		t.Run(string(tt.action), func(t *testing.T) {
			args := []string{"-n", systemd.RootctlPath, string(tt.action)}
			runner := systemdtest.New()
			runner.Respond("sudo", args, "", nil)

			if err := systemd.New(runner).Control(
				context.Background(), tt.action); err != nil {
				t.Fatalf("Control: %v", err)
			}
			assertCalls(t, runner.Calls(), []systemdtest.Call{{
				Name: "sudo",
				Args: args,
			}})
		})
	}
}

func TestClientControlRejectsUnknownAction(t *testing.T) {
	for _, action := range []systemd.ControlAction{"", "reload", "start --now"} {
		runner := systemdtest.New()
		err := systemd.New(runner).Control(context.Background(), action)
		if !errors.Is(err, systemd.ErrInvalidAction) {
			t.Fatalf("Control(%q) error = %v, want ErrInvalidAction", action, err)
		}
		if calls := runner.Calls(); len(calls) != 0 {
			t.Fatalf("Control(%q) reached runner: %+v", action, calls)
		}
	}
}

func TestClientControlPropagatesFailure(t *testing.T) {
	args := []string{"-n", systemd.RootctlPath, string(systemd.ActionStart)}
	runner := systemdtest.New()
	runner.Respond("sudo", args, "", commandError("sudo", args, 1,
		"sudo: a password is required"))

	err := systemd.New(runner).Control(context.Background(), systemd.ActionStart)
	var commandErr *systemd.CommandError
	if !errors.As(err, &commandErr) {
		t.Fatalf("Control error type = %T, want *CommandError", err)
	}
	if commandErr.ExitCode != 1 {
		t.Fatalf("ExitCode = %d, want 1", commandErr.ExitCode)
	}
}

func TestClientWithoutRunner(t *testing.T) {
	_, err := systemd.New(nil).Show(
		context.Background(), systemd.UnitTES3MPService)
	if err == nil || !strings.Contains(err.Error(), "runner is not configured") {
		t.Fatalf("Show error = %v, want missing runner", err)
	}
}

func TestValidateUnit(t *testing.T) {
	valid := []string{
		systemd.UnitTES3MPService,
		systemd.UnitPanelService,
		systemd.UnitBackupService,
		systemd.UnitBackupTimer,
		"tes3mp@slot-1.service",
		"a_b-c:d.service",
	}
	for _, unit := range valid {
		if err := systemd.ValidateUnit(unit); err != nil {
			t.Fatalf("ValidateUnit(%q): %v", unit, err)
		}
	}
}

func TestCommandError(t *testing.T) {
	cause := errors.New("exit status 1")
	err := &systemd.CommandError{
		Name:     "systemctl",
		Args:     []string{"status", systemd.UnitTES3MPService},
		ExitCode: 1,
		Stderr:   "Unit not found",
		Err:      cause,
	}

	message := err.Error()
	for _, want := range []string{"systemctl status tes3mp.service", "exit status 1", "Unit not found"} {
		if !strings.Contains(message, want) {
			t.Fatalf("Error() = %q, want substring %q", message, want)
		}
	}
	if !errors.Is(err, cause) {
		t.Fatal("CommandError does not unwrap to its cause")
	}
}

func TestExecRunnerHelper(t *testing.T) {
	if os.Getenv("TES3MP_SYSTEMD_HELPER") != "1" {
		return
	}
	if delay := os.Getenv("TES3MP_SYSTEMD_HELPER_SLEEP"); delay != "" {
		duration, err := time.ParseDuration(delay)
		if err != nil {
			fmt.Fprintf(os.Stderr, "bad helper sleep %q: %v", delay, err)
			os.Exit(99)
		}
		time.Sleep(duration)
	}

	fmt.Fprint(os.Stdout, os.Getenv("TES3MP_SYSTEMD_HELPER_STDOUT"))
	fmt.Fprint(os.Stderr, os.Getenv("TES3MP_SYSTEMD_HELPER_STDERR"))

	code, err := strconv.Atoi(os.Getenv("TES3MP_SYSTEMD_HELPER_EXIT"))
	if err != nil {
		os.Exit(99)
	}
	os.Exit(code)
}

func setExecHelper(t *testing.T, stdout, stderr string, exitCode int) []string {
	t.Helper()
	t.Setenv("TES3MP_SYSTEMD_HELPER", "1")
	t.Setenv("TES3MP_SYSTEMD_HELPER_SLEEP", "")
	t.Setenv("TES3MP_SYSTEMD_HELPER_STDOUT", stdout)
	t.Setenv("TES3MP_SYSTEMD_HELPER_STDERR", stderr)
	t.Setenv("TES3MP_SYSTEMD_HELPER_EXIT", strconv.Itoa(exitCode))
	return []string{"-test.run=^TestExecRunnerHelper$"}
}

func TestExecRunnerCapturesStdout(t *testing.T) {
	args := setExecHelper(t, "hello\n", "", 0)

	out, err := (systemd.ExecRunner{}).Run(context.Background(), os.Args[0], args...)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if string(out) != "hello\n" {
		t.Fatalf("stdout = %q, want hello", out)
	}
}

func TestExecRunnerReportsExitCodeAndStderr(t *testing.T) {
	args := setExecHelper(t, "partial\n", "boom\n", 7)

	out, err := (systemd.ExecRunner{}).Run(context.Background(), os.Args[0], args...)
	if err == nil {
		t.Fatal("Run returned nil error for exit 7")
	}
	if string(out) != "partial\n" {
		t.Fatalf("stdout = %q, want partial", out)
	}

	var commandErr *systemd.CommandError
	if !errors.As(err, &commandErr) {
		t.Fatalf("error type = %T, want *CommandError", err)
	}
	if commandErr.ExitCode != 7 {
		t.Fatalf("ExitCode = %d, want 7", commandErr.ExitCode)
	}
	if !strings.Contains(commandErr.Stderr, "boom") {
		t.Fatalf("Stderr = %q, want boom", commandErr.Stderr)
	}
	if !strings.Contains(commandErr.Error(), "boom") {
		t.Fatalf("Error() = %q, want stderr", commandErr.Error())
	}
}

func TestExecRunnerReportsMissingCommand(t *testing.T) {
	_, err := (systemd.ExecRunner{}).Run(
		context.Background(), "/definitely/not/a/tes3mp-command")
	if err == nil {
		t.Fatal("Run accepted a missing executable")
	}

	var commandErr *systemd.CommandError
	if !errors.As(err, &commandErr) {
		t.Fatalf("error type = %T, want *CommandError", err)
	}
	if commandErr.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0 for exec failure", commandErr.ExitCode)
	}
}

func TestExecRunnerHonorsContext(t *testing.T) {
	args := setExecHelper(t, "", "", 0)
	t.Setenv("TES3MP_SYSTEMD_HELPER_SLEEP", "10s")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := (systemd.ExecRunner{}).Run(ctx, os.Args[0], args...)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run error = %v, want deadline exceeded", err)
	}
}

func TestExecRunnerDoesNotInvokeShell(t *testing.T) {
	_, err := (systemd.ExecRunner{}).Run(
		context.Background(), "printf hello")
	if err == nil {
		t.Fatal("Run accepted a command name containing shell syntax")
	}
}
