package main

import (
	"bytes"
	"reflect"
	"strconv"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/systemd"
	"github.com/Saagilat/tes3mp-easy/internal/systemd/systemdtest"
)

func logArgs(tail int) []string {
	return []string{
		"-u", systemd.UnitTES3MPService,
		"-n", strconv.Itoa(tail),
		"--no-pager",
		"-o", "cat",
	}
}

func TestRunLogsJSON(t *testing.T) {
	runner := systemdtest.New()
	runner.Respond("journalctl", logArgs(3),
		"line one\nline two\nline three\n", nil)

	var stdout, stderr bytes.Buffer
	code := runWithClient([]string{"--json", "logs", "--tail", "3"},
		&stdout, &stderr, systemd.New(runner))
	if code != int(ops.ExitOK) {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q",
			code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	var data logsData
	decodeJSONData(t, stdout.String(), &data)
	want := []string{"line one", "line two", "line three"}
	if !reflect.DeepEqual(data.Lines, want) {
		t.Fatalf("lines = %#v, want %#v", data.Lines, want)
	}
	wantCalls := []systemdtest.Call{{Name: "journalctl", Args: logArgs(3)}}
	if calls := runner.Calls(); !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", calls, wantCalls)
	}
}

func TestRunLogsHuman(t *testing.T) {
	runner := systemdtest.New()
	runner.Respond("journalctl", logArgs(2),
		"2026-10-22 12:00:00 | server started\nplayer joined\n", nil)

	var stdout, stderr bytes.Buffer
	code := runWithClient([]string{"logs", "--tail=2"}, &stdout, &stderr,
		systemd.New(runner))
	if code != int(ops.ExitOK) {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q",
			code, stdout.String(), stderr.String())
	}
	want := "2026-10-22 12:00:00 | server started\nplayer joined\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunLogsEmptyJSON(t *testing.T) {
	runner := systemdtest.New()
	runner.Respond("journalctl", logArgs(5), "", nil)

	var stdout, stderr bytes.Buffer
	code := runWithClient([]string{"--json", "logs", "--tail", "5"},
		&stdout, &stderr, systemd.New(runner))
	if code != int(ops.ExitOK) {
		t.Fatalf("exit = %d, want 0", code)
	}
	var data logsData
	decodeJSONData(t, stdout.String(), &data)
	if data.Lines == nil {
		t.Fatal("lines = nil, want empty array")
	}
	if len(data.Lines) != 0 {
		t.Fatalf("lines = %#v, want empty", data.Lines)
	}
}

func TestRunLogsUnavailable(t *testing.T) {
	runner := systemdtest.New()
	runner.Respond("journalctl", logArgs(10), "",
		commandError("journalctl", logArgs(10), 1,
			"Failed to open journal: No such file or directory"))

	var stdout, stderr bytes.Buffer
	code := runWithClient([]string{"--json", "logs", "--tail", "10"},
		&stdout, &stderr, systemd.New(runner))
	if code != int(ops.ExitServiceUnavailable) {
		t.Fatalf("exit = %d, want %d", code, ops.ExitServiceUnavailable)
	}
	assertJSONError(t, stdout.String(), ops.ExitServiceUnavailable,
		"Failed to open journal")
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty in JSON mode", stderr.String())
	}
}

func TestParseLogsArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing tail", args: nil},
		{name: "zero", args: []string{"--tail=0"}},
		{name: "negative", args: []string{"--tail=-1"}},
		{name: "too large", args: []string{"--tail=10001"}},
		{name: "not a number", args: []string{"--tail=many"}},
		{name: "missing value", args: []string{"--tail"}},
		{name: "unexpected argument", args: []string{"--tail", "10", "extra"}},
		{name: "unknown flag", args: []string{"--bogus"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tail, err := parseLogsArgs(tt.args)
			if err == nil || err.Exit != ops.ExitInvalidArgument {
				t.Fatalf("parseLogsArgs(%v) = %d, %v; want invalid_argument",
					tt.args, tail, err)
			}
		})
	}

	tail, err := parseLogsArgs([]string{"--tail", "1"})
	if err != nil || tail != 1 {
		t.Fatalf("parseLogsArgs(--tail 1) = %d, %v", tail, err)
	}
	if tail, err := parseLogsArgs([]string{"--tail=10000"}); err != nil ||
		tail != maxLogTail {
		t.Fatalf("parseLogsArgs(--tail=10000) = %d, %v", tail, err)
	}
}
