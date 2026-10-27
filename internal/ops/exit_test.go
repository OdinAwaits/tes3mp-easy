package ops

import "testing"

func TestExitCodes(t *testing.T) {
	tests := []struct {
		name string
		exit ExitCode
		want int
		code string
	}{
		{name: "ok", exit: ExitOK, want: 0, code: ""},
		{name: "internal", exit: ExitInternal, want: 1, code: CodeInternal},
		{name: "invalid argument", exit: ExitInvalidArgument, want: 2, code: CodeInvalidArgument},
		{name: "not found", exit: ExitNotFound, want: 3, code: CodeNotFound},
		{name: "locked", exit: ExitLocked, want: 4, code: CodeLocked},
		{name: "no space", exit: ExitNoSpace, want: 5, code: CodeNoSpace},
		{name: "service unavailable", exit: ExitServiceUnavailable, want: 6, code: CodeServiceUnavailable},
		{name: "archive invalid", exit: ExitArchiveInvalid, want: 7, code: CodeArchiveInvalid},
		{name: "unsafe path", exit: ExitUnsafePath, want: 8, code: CodeUnsafePath},
		{name: "config invalid", exit: ExitConfigInvalid, want: 9, code: CodeConfigInvalid},
		{name: "checks failed", exit: ExitChecksFailed, want: 10, code: CodeChecksFailed},
		{name: "plugin in use", exit: ExitPluginInUse, want: 11, code: CodePluginInUse},
		{name: "archive in use", exit: ExitArchiveInUse, want: 12, code: CodeArchiveInUse},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if int(tt.exit) != tt.want {
				t.Fatalf("exit = %d, want %d", tt.exit, tt.want)
			}
			if got := tt.exit.ErrorCode(); got != tt.code {
				t.Fatalf("ErrorCode() = %q, want %q", got, tt.code)
			}
			if !tt.exit.Valid() {
				t.Fatalf("Valid() = false for %d", tt.exit)
			}
		})
	}
}

func TestUnknownExitCode(t *testing.T) {
	const unknown ExitCode = 99

	if unknown.Valid() {
		t.Fatal("Valid() = true, want false")
	}
	if got := unknown.ErrorCode(); got != CodeInternal {
		t.Fatalf("ErrorCode() = %q, want %q", got, CodeInternal)
	}
}

func TestNewError(t *testing.T) {
	err := NewError(ExitArchiveInvalid, "bad archive")
	if err.Exit != ExitArchiveInvalid {
		t.Fatalf("Exit = %d, want %d", err.Exit, ExitArchiveInvalid)
	}
	if err.Code != CodeArchiveInvalid {
		t.Fatalf("Code = %q, want %q", err.Code, CodeArchiveInvalid)
	}
	if err.Error() != "bad archive" {
		t.Fatalf("Error() = %q, want %q", err.Error(), "bad archive")
	}
}

func TestErrorEnvelope(t *testing.T) {
	err := NewError(ExitLocked, "another mutation is running")
	err.Details = map[string]any{"lock": "ops.lock"}

	envelope := err.Envelope()
	if envelope.OK {
		t.Fatal("envelope.OK = true, want false")
	}
	if envelope.Data != nil {
		t.Fatalf("envelope.Data = %#v, want nil", envelope.Data)
	}
	if envelope.Error == nil {
		t.Fatal("envelope.Error is nil")
	}
	if envelope.Error.Code != CodeLocked {
		t.Fatalf("error.code = %q, want %q", envelope.Error.Code, CodeLocked)
	}
	if envelope.Error.Message != "another mutation is running" {
		t.Fatalf("error.message = %q", envelope.Error.Message)
	}
	if got := envelope.Error.Details["lock"]; got != "ops.lock" {
		t.Fatalf("error.details.lock = %#v, want ops.lock", got)
	}
}
