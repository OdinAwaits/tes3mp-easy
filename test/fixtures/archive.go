// Package fixtures builds deterministic test data for archives, configuration
// files and public API responses.
//
// The package deliberately contains only synthetic data. It never reads or
// copies Morrowind game files, TES3MP runtime files or user data.
package fixtures

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// DefaultTime is the fixed timestamp used by all default fixture names.
var DefaultTime = time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)

// Entry is one tar entry. A zero Type means a regular file.
type Entry struct {
	Name     string
	Body     string
	Type     byte
	Mode     int64
	Linkname string
	ModTime  time.Time
}

// Tar returns an uncompressed tar archive built from entries.
func Tar(t testing.TB, entries []Entry) []byte {
	t.Helper()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, entry := range entries {
		typ := entry.Type
		if typ == 0 {
			typ = tar.TypeReg
		}
		mode := entry.Mode
		if mode == 0 {
			mode = 0o600
			if typ == tar.TypeDir {
				mode = 0o755
			}
		}
		modTime := entry.ModTime
		if modTime.IsZero() {
			modTime = DefaultTime
		}

		size := int64(0)
		if typ == tar.TypeReg {
			size = int64(len(entry.Body))
		}
		header := &tar.Header{
			Name:     entry.Name,
			Typeflag: typ,
			Mode:     mode,
			Size:     size,
			Linkname: entry.Linkname,
			ModTime:  modTime,
		}
		if err := tw.WriteHeader(header); err != nil {
			t.Fatalf("fixtures: write tar header %q: %v", entry.Name, err)
		}
		if size > 0 {
			if _, err := io.WriteString(tw, entry.Body); err != nil {
				t.Fatalf("fixtures: write tar body %q: %v", entry.Name, err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("fixtures: close tar: %v", err)
	}
	return buf.Bytes()
}

// TarGz returns a deterministic gzip-compressed tar archive.
func TarGz(t testing.TB, entries []Entry) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(Tar(t, entries)); err != nil {
		t.Fatalf("fixtures: gzip archive: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("fixtures: close gzip archive: %v", err)
	}
	return buf.Bytes()
}

// WriteFile writes body to path, creating parent directories as needed.
func WriteFile(t testing.TB, path, body string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("fixtures: create directory for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("fixtures: write %s: %v", path, err)
	}
}

// WriteTarGz writes a generated archive to path.
func WriteTarGz(t testing.TB, path string, entries []Entry) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("fixtures: create directory for %s: %v", path, err)
	}
	if err := os.WriteFile(path, TarGz(t, entries), 0o600); err != nil {
		t.Fatalf("fixtures: write archive %s: %v", path, err)
	}
}

// Name builds an archive name in the UTC format from docs/v2/03-contracts.md
// §6.6. A collision greater than zero appends -01, -02, ...
func Name(prefix string, at time.Time, collision int) string {
	base := prefix + at.UTC().Format("2006-01-02_15-04-05")
	if collision > 0 {
		base += fmt.Sprintf("-%02d", collision)
	}
	return base + ".tar.gz"
}

// ModsName returns mods-YYYY-MM-DD_HH-MM-SS.tar.gz.
func ModsName(at time.Time) string { return Name("mods-", at, 0) }

// ModsInitName returns mods-init-YYYY-MM-DD_HH-MM-SS.tar.gz.
func ModsInitName(at time.Time) string { return Name("mods-init-", at, 0) }

// StateName returns state-YYYY-MM-DD_HH-MM-SS.tar.gz.
func StateName(at time.Time) string { return Name("state-", at, 0) }

// StateManualName returns state-manual-YYYY-MM-DD_HH-MM-SS.tar.gz.
func StateManualName(at time.Time) string { return Name("state-manual-", at, 0) }

// StatePredeployName returns state-predeploy-YYYY-MM-DD_HH-MM-SS.tar.gz.
func StatePredeployName(at time.Time) string { return Name("state-predeploy-", at, 0) }
