package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/ops"
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

func showOutput(loadState, activeState, subState, activeEnter, lastTrigger string) string {
	return fmt.Sprintf(
		"ActiveState=%s\nSubState=%s\nActiveEnterTimestamp=%s\n"+
			"LastTriggerUSec=%s\nLoadState=%s\nUnitFileState=enabled\n",
		activeState, subState, activeEnter, lastTrigger, loadState)
}

func decodeJSONData(t *testing.T, output string, target any) {
	t.Helper()

	var envelope struct {
		OK   bool            `json:"ok"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		t.Fatalf("decode envelope: %v; stdout=%q", err, output)
	}
	if !envelope.OK {
		t.Fatalf("envelope.ok = false; stdout=%q", output)
	}
	if err := json.Unmarshal(envelope.Data, target); err != nil {
		t.Fatalf("decode data: %v; data=%q", err, envelope.Data)
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

func TestRunStatusJSON(t *testing.T) {
	runner := systemdtest.New()
	runner.Respond("systemctl", showArgs(systemd.UnitTES3MPService),
		showOutput("loaded", "active", "running",
			"Thu 2026-10-22 12:00:00 UTC", "n/a"), nil)
	runner.Respond("systemctl", []string{"is-active", systemd.UnitTES3MPService},
		"active\n", nil)

	var stdout, stderr bytes.Buffer
	code := runWithClient([]string{"--json", "status"}, &stdout, &stderr,
		systemd.New(runner))
	if code != int(ops.ExitOK) {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q",
			code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	var data statusData
	decodeJSONData(t, stdout.String(), &data)
	if data.Unit != systemd.UnitTES3MPService {
		t.Fatalf("unit = %q, want %q", data.Unit, systemd.UnitTES3MPService)
	}
	if data.State != stateRunning || data.Health != healthHealthy {
		t.Fatalf("state/health = %q/%q, want running/healthy",
			data.State, data.Health)
	}
	if data.StartedAt == nil || *data.StartedAt != "2026-10-22T12:00:00Z" {
		t.Fatalf("started_at = %v, want 2026-10-22T12:00:00Z", data.StartedAt)
	}

	wantCalls := []systemdtest.Call{
		{Name: "systemctl", Args: showArgs(systemd.UnitTES3MPService)},
		{Name: "systemctl", Args: []string{"is-active", systemd.UnitTES3MPService}},
	}
	if calls := runner.Calls(); !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", calls, wantCalls)
	}
}

func TestRunStatusTimerJSON(t *testing.T) {
	runner := systemdtest.New()
	runner.Respond("systemctl", showArgs(systemd.UnitBackupTimer),
		showOutput("loaded", "active", "waiting", "n/a",
			"Thu 2026-10-22 11:55:00 UTC"), nil)
	runner.Respond("systemctl", []string{"is-active", systemd.UnitBackupTimer},
		"active\n", nil)

	var stdout, stderr bytes.Buffer
	code := runWithClient([]string{"--json", "status",
		"--unit", systemd.UnitBackupTimer}, &stdout, &stderr, systemd.New(runner))
	if code != int(ops.ExitOK) {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q",
			code, stdout.String(), stderr.String())
	}

	var data statusData
	decodeJSONData(t, stdout.String(), &data)
	if data.Unit != systemd.UnitBackupTimer || data.State != stateRunning {
		t.Fatalf("unit/state = %q/%q, want timer/running", data.Unit, data.State)
	}
	if data.StartedAt == nil || *data.StartedAt != "2026-10-22T11:55:00Z" {
		t.Fatalf("started_at = %v, want 2026-10-22T11:55:00Z", data.StartedAt)
	}
}

func TestRunStatusHuman(t *testing.T) {
	runner := systemdtest.New()
	runner.Respond("systemctl", showArgs(systemd.UnitTES3MPService),
		showOutput("loaded", "inactive", "dead", "n/a", "n/a"), nil)
	runner.Respond("systemctl", []string{"is-active", systemd.UnitTES3MPService},
		"inactive\n", commandError("systemctl",
			[]string{"is-active", systemd.UnitTES3MPService}, 3, ""))

	var stdout, stderr bytes.Buffer
	code := runWithClient([]string{"status"}, &stdout, &stderr,
		systemd.New(runner))
	if code != int(ops.ExitOK) {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q",
			code, stdout.String(), stderr.String())
	}
	want := "unit: tes3mp.service\nstate: stopped\nhealth: healthy\nstarted_at: -\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestClassifyStatus(t *testing.T) {
	service := systemd.UnitTES3MPService
	timer := systemd.UnitBackupTimer
	activeErr := errors.New("is-active failed")

	tests := []struct {
		name       string
		unit       string
		loadState  string
		active     string
		subState   string
		isActive   bool
		activeErr  error
		wantState  string
		wantHealth string
		wantErr    bool
	}{
		{name: "running", unit: service, loadState: "loaded", active: "active",
			subState: "running", isActive: true, wantState: stateRunning, wantHealth: healthHealthy},
		{name: "stopped", unit: service, loadState: "loaded", active: "inactive",
			subState: "dead", wantState: stateStopped, wantHealth: healthHealthy},
		{name: "starting", unit: service, loadState: "loaded", active: "activating",
			subState: "start-pre", wantState: stateStarting, wantHealth: healthHealthy},
		{name: "reloading", unit: service, loadState: "loaded", active: "reloading",
			subState: "running", wantState: stateStarting, wantHealth: healthHealthy},
		{name: "failed", unit: service, loadState: "loaded", active: "failed",
			subState: "failed", wantState: stateUnhealthy, wantHealth: healthUnhealthy},
		{name: "unknown state", unit: service, loadState: "loaded", active: "unknown",
			subState: "dead", wantState: stateUnknown, wantHealth: healthUnknown},
		{name: "load error", unit: service, loadState: "error", active: "inactive",
			subState: "dead", wantState: stateUnhealthy, wantHealth: healthUnhealthy},
		{name: "masked", unit: service, loadState: "masked", active: "inactive",
			subState: "dead", wantState: stateStopped, wantHealth: healthUnhealthy},
		{name: "not found", unit: service, loadState: "not-found",
			active: "inactive", subState: "dead", wantErr: true},
		{name: "is-active error", unit: service, loadState: "loaded", active: "active",
			subState: "running", activeErr: activeErr, wantState: stateRunning,
			wantHealth: healthUnhealthy},
		{name: "is-active mismatch", unit: service, loadState: "loaded", active: "active",
			subState: "running", wantState: stateStopped, wantHealth: healthHealthy},
		{name: "active race", unit: service, loadState: "loaded", active: "inactive",
			subState: "dead", isActive: true, wantState: stateRunning, wantHealth: healthHealthy},
		{name: "deactivating", unit: service, loadState: "loaded", active: "deactivating",
			subState: "stop", wantState: stateStopped, wantHealth: healthHealthy},
		{name: "timer running", unit: timer, loadState: "loaded", active: "active",
			subState: "waiting", isActive: true, wantState: stateRunning, wantHealth: healthHealthy},
		{name: "timer stopped", unit: timer, loadState: "loaded", active: "inactive",
			subState: "dead", wantState: stateStopped, wantHealth: healthHealthy},
		{name: "timer activating is stopped", unit: timer, loadState: "loaded",
			active: "activating", subState: "dead", wantState: stateStopped,
			wantHealth: healthHealthy},
		{name: "timer failed", unit: timer, loadState: "loaded", active: "failed",
			subState: "failed", wantState: stateUnhealthy, wantHealth: healthUnhealthy},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			properties := systemd.Properties{
				"LoadState":   tt.loadState,
				"ActiveState": tt.active,
				"SubState":    tt.subState,
			}
			state, health, err := classifyStatus(tt.unit, properties,
				tt.isActive, tt.activeErr)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("classifyStatus error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("classifyStatus error: %v", err)
			}
			if state != tt.wantState || health != tt.wantHealth {
				t.Fatalf("state/health = %q/%q, want %q/%q",
					state, health, tt.wantState, tt.wantHealth)
			}
		})
	}
}

func TestParseSystemdTimestamp(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "utc", raw: "Thu 2026-10-22 12:00:00 UTC",
			want: "2026-10-22T12:00:00Z"},
		{name: "fraction", raw: "Thu 2026-10-22 12:00:00.123456 UTC",
			want: "2026-10-22T12:00:00Z"},
		{name: "unix", raw: "@1792674049", want: "2026-10-22T13:00:49Z"},
		{name: "rfc3339", raw: "2026-10-22T12:00:00+03:00",
			want: "2026-10-22T09:00:00Z"},
		{name: "n/a", raw: "n/a"},
		{name: "empty", raw: ""},
		{name: "garbage", raw: "not a timestamp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSystemdTimestamp(tt.raw)
			if tt.want == "" {
				if got != nil {
					t.Fatalf("parseSystemdTimestamp(%q) = %q, want nil",
						tt.raw, *got)
				}
				return
			}
			if got == nil || *got != tt.want {
				t.Fatalf("parseSystemdTimestamp(%q) = %v, want %q",
					tt.raw, got, tt.want)
			}
		})
	}
}

func TestRunStatusErrors(t *testing.T) {
	t.Run("systemd unavailable", func(t *testing.T) {
		runner := systemdtest.New()
		runner.Respond("systemctl", showArgs(systemd.UnitTES3MPService), "",
			commandError("systemctl", showArgs(systemd.UnitTES3MPService),
				1, "Failed to connect to bus: No such file or directory"))

		var stdout, stderr bytes.Buffer
		code := runWithClient([]string{"--json", "status"}, &stdout, &stderr,
			systemd.New(runner))
		if code != int(ops.ExitServiceUnavailable) {
			t.Fatalf("exit = %d, want %d", code, ops.ExitServiceUnavailable)
		}
		assertJSONError(t, stdout.String(), ops.ExitServiceUnavailable,
			"Failed to connect to bus")
		if stderr.Len() != 0 {
			t.Fatalf("stderr = %q, want empty in JSON mode", stderr.String())
		}
	})

	t.Run("unit not installed", func(t *testing.T) {
		runner := systemdtest.New()
		runner.Respond("systemctl", showArgs(systemd.UnitTES3MPService),
			showOutput("not-found", "inactive", "dead", "n/a", "n/a"), nil)
		runner.Respond("systemctl", []string{"is-active", systemd.UnitTES3MPService},
			"inactive\n", commandError("systemctl",
				[]string{"is-active", systemd.UnitTES3MPService}, 3, ""))

		var stdout, stderr bytes.Buffer
		code := runWithClient([]string{"--json", "status"}, &stdout, &stderr,
			systemd.New(runner))
		if code != int(ops.ExitServiceUnavailable) {
			t.Fatalf("exit = %d, want %d", code, ops.ExitServiceUnavailable)
		}
		assertJSONError(t, stdout.String(), ops.ExitServiceUnavailable,
			"not installed")
	})

	t.Run("is-active failure marks unhealthy", func(t *testing.T) {
		runner := systemdtest.New()
		runner.Respond("systemctl", showArgs(systemd.UnitTES3MPService),
			showOutput("loaded", "active", "running",
				"Thu 2026-10-22 12:00:00 UTC", "n/a"), nil)
		runner.Respond("systemctl", []string{"is-active", systemd.UnitTES3MPService},
			"active\n", commandError("systemctl",
				[]string{"is-active", systemd.UnitTES3MPService},
				1, "transient failure"))

		var stdout, stderr bytes.Buffer
		code := runWithClient([]string{"--json", "status"}, &stdout, &stderr,
			systemd.New(runner))
		if code != int(ops.ExitOK) {
			t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q",
				code, stdout.String(), stderr.String())
		}
		var data statusData
		decodeJSONData(t, stdout.String(), &data)
		if data.State != stateRunning || data.Health != healthUnhealthy {
			t.Fatalf("state/health = %q/%q, want running/unhealthy",
				data.State, data.Health)
		}
	})
}

func TestParseStatusArgs(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		unit, err := parseStatusArgs(nil)
		if err != nil || unit != systemd.UnitTES3MPService {
			t.Fatalf("parseStatusArgs(nil) = %q, %v", unit, err)
		}
	})

	tests := []struct {
		name string
		args []string
	}{
		{name: "unsupported unit", args: []string{"--unit", "tes3mp-panel.service"}},
		{name: "empty unit", args: []string{"--unit="}},
		{name: "unexpected argument", args: []string{"extra"}},
		{name: "unknown flag", args: []string{"--bogus"}},
		{name: "missing unit value", args: []string{"--unit"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseStatusArgs(tt.args)
			if err == nil || err.Exit != ops.ExitInvalidArgument {
				t.Fatalf("parseStatusArgs(%v) error = %v, want invalid_argument",
					tt.args, err)
			}
		})
	}

	unit, err := parseStatusArgs([]string{"--unit=" + systemd.UnitBackupTimer})
	if err != nil {
		t.Fatalf("parseStatusArgs error: %v", err)
	}
	if unit != systemd.UnitBackupTimer {
		t.Fatalf("unit = %q, want %q", unit, systemd.UnitBackupTimer)
	}
}

func TestStatusJSONHasNoBackendField(t *testing.T) {
	runner := systemdtest.New()
	runner.Respond("systemctl", showArgs(systemd.UnitTES3MPService),
		showOutput("loaded", "active", "running",
			"Thu 2026-10-22 12:00:00 UTC", "n/a"), nil)
	runner.Respond("systemctl", []string{"is-active", systemd.UnitTES3MPService},
		"active\n", nil)

	var stdout, stderr bytes.Buffer
	runWithClient([]string{"--json", "status"}, &stdout, &stderr,
		systemd.New(runner))
	if strings.Contains(stdout.String(), "backend") || stderr.Len() != 0 {
		t.Fatalf("unexpected backend/stderr: stdout=%q stderr=%q",
			stdout.String(), stderr.String())
	}
}
