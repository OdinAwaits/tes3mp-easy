package archive

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

type zipFixture struct {
	name string
	body string
	mode os.FileMode
	dir  bool
}

func makeZip(t *testing.T, entries []zipFixture) []byte {
	t.Helper()

	var raw bytes.Buffer
	writer := zip.NewWriter(&raw)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		header.SetMode(entry.mode)
		if entry.dir {
			header.SetMode(os.ModeDir | 0o755)
		}
		item, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatalf("CreateHeader(%q): %v", entry.name, err)
		}
		if entry.body != "" {
			if _, err := io.WriteString(item, entry.body); err != nil {
				t.Fatalf("Write(%q): %v", entry.name, err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return raw.Bytes()
}

func zipReader(t *testing.T, data []byte) *zip.Reader {
	t.Helper()

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	return reader
}

func TestExtractZip(t *testing.T) {
	data := makeZip(t, []zipFixture{
		{name: "tes3mp.exe", body: "exe", mode: 0o755},
		{name: "resources/", dir: true, mode: os.ModeDir | 0o755},
		{name: "resources/icon.png", body: "png", mode: 0o644},
	})
	dest := filepath.Join(t.TempDir(), "game")
	if err := ExtractZip(zipReader(t, data), dest, DefaultZipLimits()); err != nil {
		t.Fatalf("ExtractZip: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "resources", "icon.png"))
	if err != nil || string(got) != "png" {
		t.Fatalf("extracted icon = %q, %v", got, err)
	}
	info, err := os.Stat(filepath.Join(dest, "tes3mp.exe"))
	if err != nil {
		t.Fatalf("stat exe: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("extracted mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestExtractZipRejectsUnsafeAndInvalidEntries(t *testing.T) {
	tests := []struct {
		name    string
		entries []zipFixture
		want    error
	}{
		{
			name:    "traversal",
			entries: []zipFixture{{name: "../escape", body: "x", mode: 0o600}},
			want:    ErrUnsafePath,
		},
		{
			name:    "absolute",
			entries: []zipFixture{{name: "/etc/passwd", body: "x", mode: 0o600}},
			want:    ErrUnsafePath,
		},
		{
			name:    "windows drive",
			entries: []zipFixture{{name: "C:/Windows/file", body: "x", mode: 0o600}},
			want:    ErrUnsafePath,
		},
		{
			name:    "backslash",
			entries: []zipFixture{{name: `resources\file`, body: "x", mode: 0o600}},
			want:    ErrUnsafePath,
		},
		{
			name: "symlink",
			entries: []zipFixture{{
				name: "link", body: "/etc/passwd",
				mode: os.ModeSymlink | 0o777,
			}},
			want: ErrInvalid,
		},
		{
			name: "duplicate",
			entries: []zipFixture{
				{name: "file", body: "one", mode: 0o600},
				{name: "file", body: "two", mode: 0o600},
			},
			want: ErrInvalid,
		},
		{
			name: "file then child",
			entries: []zipFixture{
				{name: "file", body: "one", mode: 0o600},
				{name: "file/child", body: "two", mode: 0o600},
			},
			want: ErrInvalid,
		},
		{
			name: "child then file",
			entries: []zipFixture{
				{name: "dir/child", body: "one", mode: 0o600},
				{name: "dir", body: "two", mode: 0o600},
			},
			want: ErrInvalid,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := makeZip(t, test.entries)
			dest := filepath.Join(t.TempDir(), "dest")
			err := ExtractZip(zipReader(t, data), dest, DefaultZipLimits())
			if !errors.Is(err, test.want) {
				t.Fatalf("ExtractZip = %v, want %v", err, test.want)
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(dest), "escape")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("unsafe entry escaped destination")
			}
		})
	}
}

func TestExtractZipLimits(t *testing.T) {
	data := makeZip(t, []zipFixture{
		{name: "one", body: "1234", mode: 0o600},
		{name: "two", body: "5678", mode: 0o600},
	})
	tests := []struct {
		name   string
		limits ZipLimits
	}{
		{name: "entries", limits: ZipLimits{
			CompressedSize: 1 << 20, UncompressedSize: 1 << 20, Entries: 1,
		}},
		{name: "uncompressed", limits: ZipLimits{
			CompressedSize: 1 << 20, UncompressedSize: 7, Entries: 10,
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ExtractZip(zipReader(t, data), t.TempDir(), test.limits)
			if !errors.Is(err, ErrLimit) {
				t.Fatalf("ExtractZip = %v, want %v", err, ErrLimit)
			}
		})
	}

	path := filepath.Join(t.TempDir(), "game.zip")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	err := ExtractZipFileWithLimits(path, t.TempDir(), ZipLimits{
		CompressedSize: 1, UncompressedSize: 1 << 20, Entries: 10,
	})
	if !errors.Is(err, ErrLimit) {
		t.Fatalf("ExtractZipFileWithLimits = %v, want %v", err, ErrLimit)
	}
}

func TestExtractZipRejectsCorruptArchive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.zip")
	if err := os.WriteFile(path, []byte("not a zip"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ExtractZipFile(path, t.TempDir()); !errors.Is(err, ErrInvalid) {
		t.Fatalf("ExtractZipFile = %v, want %v", err, ErrInvalid)
	}
}
