package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if got := run([]string{"--help"}, &stdout, &stderr); got != 0 {
		t.Fatalf("run(--help) = %d, want 0", got)
	}
	if !strings.Contains(stdout.String(), "Usage: tes3mp-easy-ops") {
		t.Fatalf("stdout = %q, want usage", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunRequiresCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if got := run(nil, &stdout, &stderr); got != 2 {
		t.Fatalf("run(nil) = %d, want 2", got)
	}
	if !strings.Contains(stderr.String(), "command is required") {
		t.Fatalf("stderr = %q, want missing-command error", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestRunRejectsUnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if got := run([]string{"--unknown"}, &stdout, &stderr); got != 2 {
		t.Fatalf("run(--unknown) = %d, want 2", got)
	}
	if stderr.Len() == 0 {
		t.Fatal("stderr is empty, want flag error")
	}
}

func TestRunUnimplementedCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if got := run([]string{"status"}, &stdout, &stderr); got != 1 {
		t.Fatalf("run(status) = %d, want 1", got)
	}
	if !strings.Contains(stderr.String(), `command "status" is not implemented`) {
		t.Fatalf("stderr = %q, want not-implemented error", stderr.String())
	}
}
