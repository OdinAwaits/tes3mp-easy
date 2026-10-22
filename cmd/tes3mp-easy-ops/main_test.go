package main

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/contract"
	"github.com/Saagilat/tes3mp-easy/internal/ops"
)

func TestRunHelp(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "long", args: []string{"--help"}},
		{name: "short", args: []string{"-h"}},
		{name: "single dash", args: []string{"-help"}},
		{name: "after command", args: []string{"status", "--help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			if got := run(tt.args, &stdout, &stderr); got != 0 {
				t.Fatalf("run(%v) = %d, want 0", tt.args, got)
			}
			if !strings.Contains(stdout.String(), "Usage: tes3mp-easy-ops") {
				t.Fatalf("stdout = %q, want usage", stdout.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestRunRequiresCommand(t *testing.T) {
	t.Run("human", func(t *testing.T) {
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
	})

	t.Run("json", func(t *testing.T) {
		var stdout, stderr bytes.Buffer

		if got := run([]string{"--json"}, &stdout, &stderr); got != 2 {
			t.Fatalf("run(--json) = %d, want 2", got)
		}
		assertJSONError(t, stdout.String(), ops.ExitInvalidArgument,
			"command is required")
		if stderr.Len() != 0 {
			t.Fatalf("stderr = %q, want empty in JSON mode", stderr.String())
		}
	})
}

func TestRunRejectsUnknownFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "human", args: []string{"--unknown"}},
		{name: "json before", args: []string{"--json", "--unknown"}},
		{name: "json after", args: []string{"--unknown", "--json"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			if got := run(tt.args, &stdout, &stderr); got != 2 {
				t.Fatalf("run(%v) = %d, want 2", tt.args, got)
			}
			if strings.Contains(strings.Join(tt.args, " "), "--json") {
				assertJSONError(t, stdout.String(), ops.ExitInvalidArgument,
					"unknown flag")
				if stderr.Len() != 0 {
					t.Fatalf("stderr = %q, want empty in JSON mode", stderr.String())
				}
				return
			}
			if !strings.Contains(stderr.String(), "unknown flag") {
				t.Fatalf("stderr = %q, want unknown-flag error", stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	t.Run("human", func(t *testing.T) {
		var stdout, stderr bytes.Buffer

		if got := run([]string{"bogus"}, &stdout, &stderr); got != 2 {
			t.Fatalf("run(bogus) = %d, want 2", got)
		}
		if !strings.Contains(stderr.String(), `unknown command "bogus"`) {
			t.Fatalf("stderr = %q, want unknown-command error", stderr.String())
		}
		if stdout.Len() != 0 {
			t.Fatalf("stdout = %q, want empty", stdout.String())
		}
	})

	t.Run("json", func(t *testing.T) {
		var stdout, stderr bytes.Buffer

		if got := run([]string{"bogus", "--json"}, &stdout, &stderr); got != 2 {
			t.Fatalf("run(bogus --json) = %d, want 2", got)
		}
		assertJSONError(t, stdout.String(), ops.ExitInvalidArgument,
			`unknown command "bogus"`)
		if stderr.Len() != 0 {
			t.Fatalf("stderr = %q, want empty in JSON mode", stderr.String())
		}
	})
}

func TestRunUnimplementedCommand(t *testing.T) {
	t.Run("human", func(t *testing.T) {
		var stdout, stderr bytes.Buffer

		if got := run([]string{"doctor"}, &stdout, &stderr); got != 1 {
			t.Fatalf("run(doctor) = %d, want 1", got)
		}
		if !strings.Contains(stderr.String(), `command "doctor" is not implemented`) {
			t.Fatalf("stderr = %q, want not-implemented error", stderr.String())
		}
		if stdout.Len() != 0 {
			t.Fatalf("stdout = %q, want empty", stdout.String())
		}
	})

	t.Run("json", func(t *testing.T) {
		var stdout, stderr bytes.Buffer

		if got := run([]string{"doctor", "--json"}, &stdout, &stderr); got != 1 {
			t.Fatalf("run(doctor --json) = %d, want 1", got)
		}
		assertJSONError(t, stdout.String(), ops.ExitInternal,
			`command "doctor" is not implemented`)
		if stderr.Len() != 0 {
			t.Fatalf("stderr = %q, want empty in JSON mode", stderr.String())
		}
	})
}

func TestRunAcceptsGlobalFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "before command",
			args: []string{
				"--root", "/tmp/root", "--dry-run", "--lock-timeout", "1s",
				"--command-timeout", "2s", "doctor",
			},
		},
		{
			name: "after command",
			args: []string{
				"doctor", "--root=/tmp/root", "--dry-run=true",
				"--lock-timeout=1s", "--command-timeout=2s",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			if got := run(tt.args, &stdout, &stderr); got != 1 {
				t.Fatalf("run(%v) = %d, want 1 (unimplemented)", tt.args, got)
			}
			if !strings.Contains(stderr.String(), "not implemented") {
				t.Fatalf("stderr = %q, want not-implemented error", stderr.String())
			}
		})
	}
}

func TestRunRejectsInvalidGlobalValues(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "empty root", args: []string{"--root=", "status"}, want: "--root"},
		{name: "missing root", args: []string{"--root"}, want: "root"},
		{name: "zero lock", args: []string{"--lock-timeout=0s", "status"}, want: "lock-timeout"},
		{name: "negative lock", args: []string{"--lock-timeout=-1s", "status"}, want: "lock-timeout"},
		{name: "bad lock", args: []string{"--lock-timeout=bad", "status"}, want: "lock-timeout"},
		{name: "zero command", args: []string{"--command-timeout=0s", "status"}, want: "command-timeout"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			if got := run(tt.args, &stdout, &stderr); got != 2 {
				t.Fatalf("run(%v) = %d, want 2", tt.args, got)
			}
			if !strings.Contains(stderr.String(), tt.want) {
				t.Fatalf("stderr = %q, want %q", stderr.String(), tt.want)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestRunJSONModeAfterParseError(t *testing.T) {
	var stdout, stderr bytes.Buffer

	args := []string{"--lock-timeout", "bad", "--json", "status"}
	if got := run(args, &stdout, &stderr); got != 2 {
		t.Fatalf("run(%v) = %d, want 2", args, got)
	}
	assertJSONError(t, stdout.String(), ops.ExitInvalidArgument, "lock-timeout")
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty in JSON mode", stderr.String())
	}
}

func TestRunJSONFalseUsesHumanOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if got := run([]string{"--json=false", "doctor"}, &stdout, &stderr); got != 1 {
		t.Fatalf("run(--json=false doctor) = %d, want 1", got)
	}
	if !strings.Contains(stderr.String(), "not implemented") {
		t.Fatalf("stderr = %q, want not-implemented error", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func assertJSONError(t *testing.T, output string, exit ops.ExitCode, message string) {
	t.Helper()

	envelope := decodeOneEnvelope(t, output)
	if envelope.OK {
		t.Fatal("envelope.ok = true, want false")
	}
	if envelope.Data != nil {
		t.Fatalf("envelope.data = %#v, want nil", envelope.Data)
	}
	if envelope.Error == nil {
		t.Fatal("envelope.error is nil")
	}
	if envelope.Error.Code != exit.ErrorCode() {
		t.Fatalf("error.code = %q, want %q", envelope.Error.Code, exit.ErrorCode())
	}
	if !strings.Contains(envelope.Error.Message, message) {
		t.Fatalf("error.message = %q, want substring %q",
			envelope.Error.Message, message)
	}
}

func decodeOneEnvelope(t *testing.T, output string) contract.Envelope {
	t.Helper()

	decoder := json.NewDecoder(strings.NewReader(output))
	var envelope contract.Envelope
	if err := decoder.Decode(&envelope); err != nil {
		t.Fatalf("stdout = %q, decode error: %v", output, err)
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("stdout contains more than one JSON value: %v", err)
	}
	return envelope
}
