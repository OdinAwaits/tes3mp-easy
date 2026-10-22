// Package ops contains process-level contracts for tes3mp-easy-ops.
package ops

import "github.com/Saagilat/tes3mp-easy/internal/contract"

// ExitCode is the process exit status defined by docs/v2/03-contracts.md.
type ExitCode int

const (
	ExitOK ExitCode = iota
	ExitInternal
	ExitInvalidArgument
	ExitNotFound
	ExitLocked
	ExitNoSpace
	ExitServiceUnavailable
	ExitArchiveInvalid
	ExitUnsafePath
	ExitConfigInvalid
	ExitChecksFailed
)

// Error code strings exposed through the JSON envelope.
const (
	CodeInternal           = "internal"
	CodeInvalidArgument    = "invalid_argument"
	CodeNotFound           = "not_found"
	CodeLocked             = "locked"
	CodeNoSpace            = "no_space"
	CodeServiceUnavailable = "service_unavailable"
	CodeArchiveInvalid     = "archive_invalid"
	CodeUnsafePath         = "unsafe_path"
	CodeConfigInvalid      = "config_invalid"
	CodeChecksFailed       = "checks_failed"
)

// ErrorCode returns the stable error.code value for an exit status.
func (c ExitCode) ErrorCode() string {
	switch c {
	case ExitOK:
		return ""
	case ExitInternal:
		return CodeInternal
	case ExitInvalidArgument:
		return CodeInvalidArgument
	case ExitNotFound:
		return CodeNotFound
	case ExitLocked:
		return CodeLocked
	case ExitNoSpace:
		return CodeNoSpace
	case ExitServiceUnavailable:
		return CodeServiceUnavailable
	case ExitArchiveInvalid:
		return CodeArchiveInvalid
	case ExitUnsafePath:
		return CodeUnsafePath
	case ExitConfigInvalid:
		return CodeConfigInvalid
	case ExitChecksFailed:
		return CodeChecksFailed
	default:
		return CodeInternal
	}
}

// Valid reports whether c is one of the defined exit statuses.
func (c ExitCode) Valid() bool {
	return c >= ExitOK && c <= ExitChecksFailed
}

// Error is an ops failure with both a process exit status and a JSON error.
type Error struct {
	Exit    ExitCode
	Code    string
	Message string
	Details map[string]any
}

// NewError builds a failure with the error code matching its exit status.
func NewError(exit ExitCode, message string) *Error {
	return &Error{
		Exit:    exit,
		Code:    exit.ErrorCode(),
		Message: message,
	}
}

// Error implements error.
func (e *Error) Error() string {
	return e.Message
}

// Envelope converts the failure to the public JSON shape.
func (e *Error) Envelope() contract.Envelope {
	return contract.Failure(contract.Error{
		Code:    e.Code,
		Message: e.Message,
		Details: e.Details,
	})
}
