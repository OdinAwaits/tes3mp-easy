// Package contract contains the stable JSON shapes used by the ops CLI.
package contract

import (
	"encoding/json"
	"io"
)

// Envelope is the single object written to stdout in --json mode.
type Envelope struct {
	OK    bool   `json:"ok"`
	Data  any    `json:"data,omitempty"`
	Error *Error `json:"error,omitempty"`
}

// Error is the machine-readable error part of a failed envelope.
type Error struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// Success returns an envelope with a successful result.
func Success(data any) Envelope {
	return Envelope{OK: true, Data: data}
}

// Failure returns an envelope with an error.
func Failure(err Error) Envelope {
	return Envelope{OK: false, Error: &err}
}

// Write encodes exactly one JSON object to w.
func Write(w io.Writer, envelope Envelope) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(envelope)
}
