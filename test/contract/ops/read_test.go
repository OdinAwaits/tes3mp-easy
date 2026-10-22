package ops

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const (
	wantShowProperties = "--property=ActiveState,SubState," +
		"ActiveEnterTimestamp,LastTriggerUSec,LoadState,UnitFileState"
	serviceUnit = "tes3mp.service"
	timerUnit   = "tes3mp-easy-backup.timer"
)

type statusPayload struct {
	Unit      string  `json:"unit"`
	State     string  `json:"state"`
	Health    string  `json:"health"`
	StartedAt *string `json:"started_at"`
}

type logsPayload struct {
	Lines []string `json:"lines"`
}

func decodeStatus(t *testing.T, env envelope) statusPayload {
	t.Helper()

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(env.Data, &raw); err != nil {
		t.Fatalf("decode status data: %v; data=%s", err, env.Data)
	}
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	wantKeys := []string{"health", "started_at", "state", "unit"}
	if !reflect.DeepEqual(keys, wantKeys) {
		t.Fatalf("status keys = %v, want %v", keys, wantKeys)
	}

	var data statusPayload
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("decode status data: %v", err)
	}
	return data
}

func decodeLogs(t *testing.T, env envelope) logsPayload {
	t.Helper()

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(env.Data, &raw); err != nil {
		t.Fatalf("decode logs data: %v; data=%s", err, env.Data)
	}
	if len(raw) != 1 {
		t.Fatalf("logs keys = %d, want only lines: %s", len(raw), env.Data)
	}
	if _, ok := raw["lines"]; !ok {
		t.Fatalf("logs data has no lines: %s", env.Data)
	}

	var data logsPayload
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("decode logs data: %v", err)
	}
	return data
}

func requireCalls(t *testing.T, path string, want [][]string) {
	t.Helper()

	got := readFakeCalls(t, path)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("systemd calls = %#v, want %#v", got, want)
	}
}

func showCall(unit string) []string {
	return []string{"show", unit, wantShowProperties, "--timestamp=utc"}
}

func TestStatusRunningContract(t *testing.T) {
	callLog := filepath.Join(t.TempDir(), "calls.log")
	res := testHarness.RunEnv(t, map[string]string{
		"FAKE_SYSTEMD_LOG":          callLog,
		"FAKE_SYSTEMD_STATE":        "active",
		"FAKE_SYSTEMD_SUBSTATE":     "running",
		"FAKE_SYSTEMD_ACTIVE_ENTER": "Thu 2026-10-22 12:00:00 UTC",
	}, "--json", "status")

	res.requireExit(t, 0)
	if res.Stderr != "" {
		t.Fatalf("stderr = %q, want empty", res.Stderr)
	}
	env := res.decodeEnvelope(t)
	if !env.OK {
		t.Fatalf("ok = false, want true; stdout=%q", res.Stdout)
	}

	data := decodeStatus(t, env)
	want := statusPayload{
		Unit:      serviceUnit,
		State:     "running",
		Health:    "healthy",
		StartedAt: stringPtr("2026-10-22T12:00:00Z"),
	}
	if !reflect.DeepEqual(data, want) {
		t.Fatalf("status = %#v, want %#v", data, want)
	}

	requireCalls(t, callLog, [][]string{
		showCall(serviceUnit),
		{"is-active", serviceUnit},
	})
}

func TestStatusStoppedContract(t *testing.T) {
	callLog := filepath.Join(t.TempDir(), "calls.log")
	res := testHarness.RunEnv(t, map[string]string{
		"FAKE_SYSTEMD_LOG":          callLog,
		"FAKE_SYSTEMD_STATE":        "inactive",
		"FAKE_SYSTEMD_SUBSTATE":     "dead",
		"FAKE_SYSTEMD_ACTIVE_ENTER": "n/a",
	}, "--json", "status")

	res.requireExit(t, 0)
	data := decodeStatus(t, res.decodeEnvelope(t))
	if data.State != "stopped" || data.Health != "healthy" ||
		data.StartedAt != nil {
		t.Fatalf("status = %#v, want stopped/healthy/null", data)
	}
}

func TestStatusFailedContract(t *testing.T) {
	callLog := filepath.Join(t.TempDir(), "calls.log")
	res := testHarness.RunEnv(t, map[string]string{
		"FAKE_SYSTEMD_LOG":          callLog,
		"FAKE_SYSTEMD_STATE":        "failed",
		"FAKE_SYSTEMD_SUBSTATE":     "failed",
		"FAKE_SYSTEMD_ACTIVE_ENTER": "Thu 2026-10-22 12:00:00 UTC",
	}, "--json", "status")

	res.requireExit(t, 0)
	data := decodeStatus(t, res.decodeEnvelope(t))
	if data.State != "unhealthy" || data.Health != "unhealthy" {
		t.Fatalf("status = %#v, want unhealthy/unhealthy", data)
	}
}

func TestStatusStartingContract(t *testing.T) {
	callLog := filepath.Join(t.TempDir(), "calls.log")
	res := testHarness.RunEnv(t, map[string]string{
		"FAKE_SYSTEMD_LOG":          callLog,
		"FAKE_SYSTEMD_STATE":        "activating",
		"FAKE_SYSTEMD_SUBSTATE":     "start-pre",
		"FAKE_SYSTEMD_ACTIVE_ENTER": "n/a",
	}, "--json", "status")

	res.requireExit(t, 0)
	data := decodeStatus(t, res.decodeEnvelope(t))
	if data.State != "starting" || data.Health != "healthy" {
		t.Fatalf("status = %#v, want starting/healthy", data)
	}
}

func TestStatusTimerContract(t *testing.T) {
	callLog := filepath.Join(t.TempDir(), "calls.log")
	res := testHarness.RunEnv(t, map[string]string{
		"FAKE_SYSTEMD_LOG":             callLog,
		"FAKE_SYSTEMD_STATE":           "active",
		"FAKE_SYSTEMD_SUBSTATE":        "waiting",
		"FAKE_SYSTEMD_LAST_TRIGGER":    "Thu 2026-10-22 11:55:00 UTC",
		"FAKE_SYSTEMD_ACTIVE_ENTER":    "n/a",
		"FAKE_SYSTEMD_UNIT_FILE_STATE": "enabled",
	}, "--json", "status", "--unit", timerUnit)

	res.requireExit(t, 0)
	data := decodeStatus(t, res.decodeEnvelope(t))
	if data.Unit != timerUnit || data.State != "running" ||
		data.Health != "healthy" ||
		data.StartedAt == nil || *data.StartedAt != "2026-10-22T11:55:00Z" {
		t.Fatalf("status = %#v, want timer running with last trigger", data)
	}
	requireCalls(t, callLog, [][]string{
		showCall(timerUnit),
		{"is-active", timerUnit},
	})
}

func stringPtr(value string) *string {
	return &value
}

func TestStatusUnavailableContract(t *testing.T) {
	res := testHarness.RunEnv(t, map[string]string{
		"FAKE_SYSTEMD_MODE": "unavailable",
	}, "--json", "status")

	env := res.requireJSONError(t, 6, "service_unavailable")
	if env.Error.Details != nil {
		if _, ok := env.Error.Details["backend"]; ok {
			t.Fatal("error.details contains forbidden backend field")
		}
	}
}

func TestStatusIsActiveFailureContract(t *testing.T) {
	res := testHarness.RunEnv(t, map[string]string{
		"FAKE_SYSTEMD_MODE":         "is-active-unavailable",
		"FAKE_SYSTEMD_STATE":        "active",
		"FAKE_SYSTEMD_SUBSTATE":     "running",
		"FAKE_SYSTEMD_ACTIVE_ENTER": "Thu 2026-10-22 12:00:00 UTC",
	}, "--json", "status")

	res.requireExit(t, 0)
	data := decodeStatus(t, res.decodeEnvelope(t))
	if data.State != "running" || data.Health != "unhealthy" {
		t.Fatalf("status = %#v, want running/unhealthy", data)
	}
}

func TestStatusNotInstalledContract(t *testing.T) {
	res := testHarness.RunEnv(t, map[string]string{
		"FAKE_SYSTEMD_LOADSTATE": "not-found",
		"FAKE_SYSTEMD_STATE":     "inactive",
		"FAKE_SYSTEMD_SUBSTATE":  "dead",
	}, "--json", "status")

	env := res.requireJSONError(t, 6, "service_unavailable")
	if !strings.Contains(env.Error.Message, "not installed") {
		t.Fatalf("message = %q, want not installed", env.Error.Message)
	}
}

func TestStatusHumanContract(t *testing.T) {
	res := testHarness.RunEnv(t, map[string]string{
		"FAKE_SYSTEMD_STATE":        "active",
		"FAKE_SYSTEMD_SUBSTATE":     "running",
		"FAKE_SYSTEMD_ACTIVE_ENTER": "Thu 2026-10-22 12:00:00 UTC",
	}, "status")

	res.requireExit(t, 0)
	if res.Stderr != "" {
		t.Fatalf("stderr = %q, want empty", res.Stderr)
	}
	want := "unit: tes3mp.service\n" +
		"state: running\n" +
		"health: healthy\n" +
		"started_at: 2026-10-22T12:00:00Z\n"
	if res.Stdout != want {
		t.Fatalf("stdout = %q, want %q", res.Stdout, want)
	}
}

func TestStatusRejectsInvalidArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "unsupported unit", args: []string{"status", "--unit", "tes3mp-panel.service"}},
		{name: "empty unit", args: []string{"status", "--unit="}},
		{name: "missing unit value", args: []string{"status", "--unit"}},
		{name: "unexpected argument", args: []string{"status", "extra"}},
		{name: "unknown flag", args: []string{"status", "--bogus"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callLog := filepath.Join(t.TempDir(), "calls.log")
			args := append([]string{"--json"}, tt.args...)
			res := testHarness.RunEnv(t, map[string]string{
				"FAKE_SYSTEMD_LOG": callLog,
			}, args...)

			res.requireJSONError(t, 2, "invalid_argument")
			if calls := readFakeCalls(t, callLog); calls != nil {
				t.Fatalf("systemd was called for invalid args: %#v", calls)
			}
		})
	}
}

func TestStatusDoesNotRequireRoot(t *testing.T) {
	res := testHarness.RunArgs(t, "--json", "status")
	res.requireExit(t, 0)
	decodeStatus(t, res.decodeEnvelope(t))
}

func journalArgs(tail string) []string {
	return []string{"-u", serviceUnit, "-n", tail, "--no-pager", "-o", "cat"}
}

func TestLogsJSONContract(t *testing.T) {
	callLog := filepath.Join(t.TempDir(), "calls.log")
	res := testHarness.RunEnv(t, map[string]string{
		"FAKE_JOURNALD_LOG":    callLog,
		"FAKE_JOURNALD_OUTPUT": "line one\nline two\nline three\n",
	}, "--json", "logs", "--tail", "3")

	res.requireExit(t, 0)
	if res.Stderr != "" {
		t.Fatalf("stderr = %q, want empty", res.Stderr)
	}
	data := decodeLogs(t, res.decodeEnvelope(t))
	want := []string{"line one", "line two", "line three"}
	if !reflect.DeepEqual(data.Lines, want) {
		t.Fatalf("lines = %#v, want %#v", data.Lines, want)
	}
	if got := readFakeCalls(t, callLog); !reflect.DeepEqual(got,
		[][]string{journalArgs("3")}) {
		t.Fatalf("journal calls = %#v, want exact argv", got)
	}
}

func TestLogsHumanContract(t *testing.T) {
	res := testHarness.RunEnv(t, map[string]string{
		"FAKE_JOURNALD_OUTPUT": "first\nsecond\n",
	}, "logs", "--tail=2")

	res.requireExit(t, 0)
	if res.Stderr != "" {
		t.Fatalf("stderr = %q, want empty", res.Stderr)
	}
	if res.Stdout != "first\nsecond\n" {
		t.Fatalf("stdout = %q, want plain lines", res.Stdout)
	}
}

func TestLogsEmptyContract(t *testing.T) {
	res := testHarness.RunEnv(t, map[string]string{
		"FAKE_JOURNALD_OUTPUT": "",
	}, "--json", "logs", "--tail", "5")

	res.requireExit(t, 0)
	data := decodeLogs(t, res.decodeEnvelope(t))
	if data.Lines == nil {
		t.Fatal("lines = nil, want []")
	}
	if len(data.Lines) != 0 {
		t.Fatalf("lines = %#v, want empty", data.Lines)
	}
}

func TestLogsUnavailableContract(t *testing.T) {
	res := testHarness.RunEnv(t, map[string]string{
		"FAKE_JOURNALD_MODE": "unavailable",
	}, "--json", "logs", "--tail", "10")

	res.requireJSONError(t, 6, "service_unavailable")
}

func TestLogsRejectsInvalidArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing tail", args: []string{"logs"}},
		{name: "zero tail", args: []string{"logs", "--tail=0"}},
		{name: "negative tail", args: []string{"logs", "--tail=-1"}},
		{name: "too large", args: []string{"logs", "--tail=10001"}},
		{name: "not a number", args: []string{"logs", "--tail=many"}},
		{name: "missing value", args: []string{"logs", "--tail"}},
		{name: "unexpected argument", args: []string{"logs", "--tail", "10", "extra"}},
		{name: "unknown flag", args: []string{"logs", "--bogus"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callLog := filepath.Join(t.TempDir(), "calls.log")
			args := append([]string{"--json"}, tt.args...)
			res := testHarness.RunEnv(t, map[string]string{
				"FAKE_JOURNALD_LOG": callLog,
			}, args...)

			res.requireJSONError(t, 2, "invalid_argument")
			if calls := readFakeCalls(t, callLog); calls != nil {
				t.Fatalf("journalctl was called for invalid args: %#v", calls)
			}
		})
	}
}

func TestLogsWaitsForGlobalFlagsAfterCommand(t *testing.T) {
	res := testHarness.RunEnv(t, map[string]string{
		"FAKE_JOURNALD_OUTPUT": "one\n",
	}, "logs", "--tail", "1", "--json")

	res.requireExit(t, 0)
	data := decodeLogs(t, res.decodeEnvelope(t))
	if !reflect.DeepEqual(data.Lines, []string{"one"}) {
		t.Fatalf("lines = %#v, want [one]", data.Lines)
	}
}
