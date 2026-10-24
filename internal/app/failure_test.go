package app

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/launcher"
	"github.com/Saagilat/tes3mp-easy/internal/morrowind"
	"github.com/Saagilat/tes3mp-easy/internal/serverapi"
)

func TestExplainGameFailures(t *testing.T) {
	tests := []struct {
		err  error
		kind FailureKind
		code FailureCode
	}{
		{err: launcher.ErrTES3MPMissing, kind: FailureGameMissing, code: CodeGameMissing},
		{err: launcher.ErrProton, kind: FailureGameProton, code: CodeGameProton},
		{err: launcher.ErrConfig, kind: FailureGameConfig, code: CodeGameConfig},
		{err: launcher.ErrStart, kind: FailureGameStart, code: CodeGameStart},
		{err: ErrGameExited, kind: FailureGameExited, code: CodeGameExited},
	}
	for _, test := range tests {
		failure := Explain(test.err)
		if failure.Kind != test.kind || failure.Code != test.code {
			t.Errorf("Explain(%v) = %q/%q, want %q/%q", test.err,
				failure.Kind, failure.Code, test.kind, test.code)
		}
	}
}

func TestExplainClassifiesErrors(t *testing.T) {
	missingFile := filepath.Join(t.TempDir(), "missing")
	_, missingErr := os.Stat(missingFile)
	if missingErr == nil {
		t.Fatal("os.Stat unexpectedly succeeded")
	}
	_, missingMorrowind := morrowind.Inspect(filepath.Join(t.TempDir(), "Morrowind"))
	if missingMorrowind == nil {
		t.Fatal("morrowind.Inspect unexpectedly succeeded")
	}

	tests := []struct {
		name string
		err  error
		want FailureKind
	}{
		{name: "morrowind not found",
			err:  fmt.Errorf("discover: %w", morrowind.ErrNotFound),
			want: FailureMorrowindNotFound},
		{name: "morrowind incomplete",
			err: &morrowind.IncompleteError{Root: "C:/Games/Morrowind",
				Missing: []string{"Data Files/Bloodmoon.esm"}},
			want: FailureMorrowindIncomplete},
		{name: "morrowind not configured",
			err:  ErrMorrowindNotConfigured,
			want: FailureMorrowindNotConfigured},
		{name: "no mods",
			err:  &serverapi.APIError{Status: 404, Code: "no_mods"},
			want: FailureNoMods},
		{name: "api error",
			err:  &serverapi.APIError{Status: 503, Code: "service_unavailable"},
			want: FailureServerAPI},
		{name: "network",
			err: &url.Error{Op: "Get", URL: "https://example.test",
				Err: &net.DNSError{Err: "no such host", Name: "example.test"}},
			want: FailureNetwork},
		{name: "timeout",
			err:  fmt.Errorf("request: %w", context.DeadlineExceeded),
			want: FailureTimeout},
		{name: "certificate",
			err: &url.Error{Op: "Get", URL: "https://example.test",
				Err: &tls.CertificateVerificationError{
					Err: x509.UnknownAuthorityError{},
				}},
			want: FailureCertificate},
		{name: "filesystem",
			err: &os.PathError{Op: "open", Path: "/locked/file",
				Err: fs.ErrPermission},
			want: FailureFileSystem},
		{name: "missing path is not a network error",
			err:  missingErr,
			want: FailureFileSystem},
		{name: "missing configured Morrowind folder",
			err:  missingMorrowind,
			want: FailureMorrowindNotFound},
		{name: "invalid panel response",
			err:  fmt.Errorf("panel API: %w: empty name", serverapi.ErrInvalidResponse),
			want: FailureServerAPI},
		{name: "work dir",
			err:  WrapWorkDir(errors.New("client work directory is empty")),
			want: FailureWorkDir},
		{name: "mods not installed",
			err:  fmt.Errorf("%w; update mods", ErrModsNotInstalled),
			want: FailureModsNotInstalled},
		{name: "localization not installed",
			err:  fmt.Errorf("%w; open Settings", ErrLocalizationNotInstalled),
			want: FailureLocalization},
		{name: "unknown",
			err:  errors.New("something odd"),
			want: FailureUnknown},
		{name: "nil", err: nil, want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := Explain(test.err)
			if got.Kind != test.want {
				t.Fatalf("Explain(%v) = %q, want %q (%s)",
					test.err, got.Kind, test.want, got.Detail)
			}
			if test.err != nil && got.Detail != test.err.Error() {
				t.Fatalf("Detail = %q, want %q", got.Detail, test.err.Error())
			}
		})
	}
}

func TestWrapWorkDirIsStable(t *testing.T) {
	err := WrapWorkDir(errors.New("not absolute"))
	if !errors.Is(err, ErrWorkDir) ||
		!strings.Contains(err.Error(), "not absolute") {
		t.Fatalf("WrapWorkDir = %v", err)
	}
	if again := WrapWorkDir(err); again != err {
		t.Fatalf("WrapWorkDir wrapped twice: %v", again)
	}
	if WrapWorkDir(nil) != nil {
		t.Fatal("WrapWorkDir(nil) is not nil")
	}
}
