package contract

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestSuccessJSON(t *testing.T) {
	got, err := json.Marshal(Success(map[string]string{"state": "running"}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	want := `{"ok":true,"data":{"state":"running"}}`
	if string(got) != want {
		t.Fatalf("json = %s, want %s", got, want)
	}
}

func TestFailureJSON(t *testing.T) {
	got, err := json.Marshal(Failure(Error{
		Code:    "archive_invalid",
		Message: "CRC mismatch",
		Details: map[string]any{"file": "test.esp"},
	}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	want := `{"ok":false,"error":{"code":"archive_invalid",` +
		`"message":"CRC mismatch","details":{"file":"test.esp"}}}`
	if string(got) != want {
		t.Fatalf("json = %s, want %s", got, want)
	}
}

func TestEnvelopeOmitsEmptyFields(t *testing.T) {
	tests := []struct {
		name     string
		envelope Envelope
		want     string
	}{
		{
			name:     "success without data",
			envelope: Success(nil),
			want:     `{"ok":true}`,
		},
		{
			name: "failure without details",
			envelope: Failure(Error{
				Code:    "locked",
				Message: "busy",
			}),
			want: `{"ok":false,"error":{"code":"locked","message":"busy"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.envelope)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(got) != tt.want {
				t.Fatalf("json = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestWriteOneObject(t *testing.T) {
	var buf bytes.Buffer

	if err := Write(&buf, Success(map[string]string{"unit": "tes3mp.service"})); err != nil {
		t.Fatalf("Write: %v", err)
	}

	decoder := json.NewDecoder(&buf)
	var envelope Envelope
	if err := decoder.Decode(&envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("Write produced more than one JSON value: %v", err)
	}
	if !envelope.OK || envelope.Error != nil {
		t.Fatalf("envelope = %#v, want success", envelope)
	}
}

func TestWriteDoesNotEscapeHTML(t *testing.T) {
	var buf bytes.Buffer

	err := Write(&buf, Failure(Error{
		Code:    "internal",
		Message: "<bad>",
	}))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if strings.Contains(buf.String(), `\u003c`) {
		t.Fatalf("json = %s, want unescaped <", buf.String())
	}
	if !strings.Contains(buf.String(), "<bad>") {
		t.Fatalf("json = %s, want message", buf.String())
	}
}
