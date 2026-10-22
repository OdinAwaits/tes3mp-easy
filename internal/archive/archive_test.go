package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/pathguard"
)

type fixtureEntry struct {
	name string
	typ  byte
	body string
	link string
}

func makeTar(t *testing.T, entries []fixtureEntry) []byte {
	t.Helper()

	var raw bytes.Buffer
	tw := tar.NewWriter(&raw)
	for _, entry := range entries {
		typ := entry.typ
		if typ == 0 {
			typ = tar.TypeReg
		}
		hdr := &tar.Header{
			Name:     entry.name,
			Typeflag: typ,
			Mode:     0o600,
			Size:     int64(len(entry.body)),
			Linkname: entry.link,
			ModTime:  time.Unix(0, 0),
		}
		if typ == tar.TypeDir {
			hdr.Mode = 0o755
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("WriteHeader(%q): %v", entry.name, err)
		}
		if entry.body != "" {
			if _, err := io.WriteString(tw, entry.body); err != nil {
				t.Fatalf("Write(%q): %v", entry.name, err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	return raw.Bytes()
}

func gzipData(t *testing.T, raw []byte) []byte {
	t.Helper()

	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	if _, err := gz.Write(raw); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return compressed.Bytes()
}

func makeTarGz(t *testing.T, entries []fixtureEntry) []byte {
	t.Helper()
	return gzipData(t, makeTar(t, entries))
}

func readEntries(t *testing.T, data []byte) ([]Entry, map[string]string, error) {
	t.Helper()

	reader, err := NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, nil, err
	}
	defer reader.Close()

	var entries []Entry
	contents := make(map[string]string)
	for {
		entry, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return entries, contents, nil
		}
		if err != nil {
			return entries, contents, err
		}
		body, err := io.ReadAll(reader)
		if err != nil {
			return entries, contents, err
		}
		entries = append(entries, *entry)
		contents[entry.Name] = string(body)
	}
}

func TestReadEntries(t *testing.T) {
	data := makeTarGz(t, []fixtureEntry{
		{name: "plugins/", typ: tar.TypeDir},
		{name: "plugins/mod.esp", body: "esp"},
		{name: "scripts/", typ: tar.TypeDir},
		{name: "scripts/test.lua", body: "print(1)"},
	})

	entries, contents, err := readEntries(t, data)
	if err != nil {
		t.Fatalf("readEntries: %v", err)
	}
	if len(entries) != 4 {
		t.Fatalf("len(entries) = %d, want 4", len(entries))
	}
	if entries[0].Name != "plugins" || entries[0].Type != TypeDir {
		t.Fatalf("entries[0] = %+v, want plugin directory", entries[0])
	}
	if entries[1].Name != "plugins/mod.esp" || entries[1].Type != TypeFile {
		t.Fatalf("entries[1] = %+v, want plugin file", entries[1])
	}
	if contents["plugins/mod.esp"] != "esp" {
		t.Fatalf("plugin body = %q", contents["plugins/mod.esp"])
	}
	if contents["scripts/test.lua"] != "print(1)" {
		t.Fatalf("script body = %q", contents["scripts/test.lua"])
	}
}

func TestEmptyTar(t *testing.T) {
	entries, contents, err := readEntries(t, makeTarGz(t, nil))
	if err != nil {
		t.Fatalf("readEntries: %v", err)
	}
	if len(entries) != 0 || len(contents) != 0 {
		t.Fatalf("empty archive returned %d entries, %d files", len(entries), len(contents))
	}
}

func TestRejectsEntryTypes(t *testing.T) {
	tests := []struct {
		name string
		typ  byte
		link string
	}{
		{name: "symlink", typ: tar.TypeSymlink, link: "target"},
		{name: "hardlink", typ: tar.TypeLink, link: "target"},
		{name: "char device", typ: tar.TypeChar},
		{name: "block device", typ: tar.TypeBlock},
		{name: "fifo", typ: tar.TypeFifo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := readEntries(t, makeTarGz(t, []fixtureEntry{{
				name: "plugins/evil",
				typ:  tt.typ,
				link: tt.link,
			}}))
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("error = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestRejectsUnsafePaths(t *testing.T) {
	tests := []string{
		"../evil",
		"plugins/../../evil",
		"..",
		"/etc/passwd",
		"//server/share",
		`plugins\evil`,
		"C:/evil",
	}
	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			_, _, err := readEntries(t, makeTarGz(t, []fixtureEntry{{
				name: name,
				body: "evil",
			}}))
			if !errors.Is(err, pathguard.ErrUnsafePath) {
				t.Fatalf("error = %v, want pathguard.ErrUnsafePath", err)
			}
		})
	}
}

func TestRejectsDuplicates(t *testing.T) {
	tests := []struct {
		name    string
		entries []fixtureEntry
	}{
		{
			name: "same name",
			entries: []fixtureEntry{
				{name: "plugins/mod.esp", body: "one"},
				{name: "plugins/mod.esp", body: "two"},
			},
		},
		{
			name: "case-insensitive name",
			entries: []fixtureEntry{
				{name: "plugins/Mod.esp", body: "one"},
				{name: "plugins/mod.esp", body: "two"},
			},
		},
		{
			name: "normalized name",
			entries: []fixtureEntry{
				{name: "plugins/mod.esp", body: "one"},
				{name: "./plugins/mod.esp", body: "two"},
			},
		},
		{
			name: "file before child",
			entries: []fixtureEntry{
				{name: "plugins", body: "one"},
				{name: "plugins/mod.esp", body: "two"},
			},
		},
		{
			name: "child before file",
			entries: []fixtureEntry{
				{name: "plugins/mod.esp", body: "one"},
				{name: "plugins", body: "two"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := readEntries(t, makeTarGz(t, tt.entries))
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("error = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestTrailingDataAfterTar(t *testing.T) {
	raw := append(makeTar(t, []fixtureEntry{{name: "plugins/mod.esp", body: "esp"}}), []byte("garbage")...)
	_, _, err := readEntries(t, gzipData(t, raw))
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("error = %v, want ErrInvalid", err)
	}
}

func TestZeroPaddingAfterTar(t *testing.T) {
	raw := append(makeTar(t, []fixtureEntry{{name: "plugins/mod.esp", body: "esp"}}), make([]byte, 1024)...)
	entries, _, err := readEntries(t, gzipData(t, raw))
	if err != nil {
		t.Fatalf("readEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
}

func TestTrailingDataAfterGzip(t *testing.T) {
	data := append(makeTarGz(t, []fixtureEntry{{name: "plugins/mod.esp", body: "esp"}}), []byte("garbage")...)
	_, _, err := readEntries(t, data)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("error = %v, want ErrInvalid", err)
	}
}

func TestRejectsCorruptGzip(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{name: "not gzip", data: []byte("not a gzip stream")},
		{name: "empty", data: nil},
		{name: "truncated", data: makeTarGz(t, []fixtureEntry{{name: "plugins/mod.esp", body: "esp"}})[:20]},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := readEntries(t, tt.data)
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("error = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestTruncatedTar(t *testing.T) {
	raw := makeTar(t, []fixtureEntry{{name: "plugins/mod.esp", body: "esp"}})
	raw = raw[:512+2]
	_, _, err := readEntries(t, gzipData(t, raw))
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("error = %v, want ErrInvalid", err)
	}
}

func readEntriesWithLimits(t *testing.T, data []byte, limits Limits) error {
	t.Helper()

	reader, err := NewReaderWithLimits(bytes.NewReader(data), limits)
	if err != nil {
		return err
	}
	defer reader.Close()

	for {
		_, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if _, err := io.Copy(io.Discard, reader); err != nil {
			return err
		}
	}
}

func TestLimits(t *testing.T) {
	raw := makeTar(t, []fixtureEntry{{name: "plugins/mod.esp", body: "esp"}})
	data := gzipData(t, raw)
	exact := Limits{
		CompressedSize:   int64(len(data)),
		UncompressedSize: int64(len(raw)),
		Entries:          1,
	}

	tests := []struct {
		name    string
		data    []byte
		limits  Limits
		wantErr error
	}{
		{name: "exact limits", data: data, limits: exact},
		{
			name:    "compressed over limit",
			data:    data,
			limits:  Limits{CompressedSize: int64(len(data)) - 1, UncompressedSize: exact.UncompressedSize, Entries: 1},
			wantErr: ErrLimit,
		},
		{
			name:    "uncompressed over limit",
			data:    data,
			limits:  Limits{CompressedSize: exact.CompressedSize, UncompressedSize: int64(len(raw)) - 1, Entries: 1},
			wantErr: ErrLimit,
		},
		{
			name:    "entries over limit",
			data:    data,
			limits:  Limits{CompressedSize: exact.CompressedSize, UncompressedSize: exact.UncompressedSize, Entries: 0},
			wantErr: ErrLimit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := readEntriesWithLimits(t, tt.data, tt.limits)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("error = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestEntryLimitWithMultipleEntries(t *testing.T) {
	data := makeTarGz(t, []fixtureEntry{
		{name: "plugins/one.esp", body: "one"},
		{name: "plugins/two.esp", body: "two"},
	})
	limits := Limits{
		CompressedSize:   MaxCompressedSize,
		UncompressedSize: MaxUncompressedSize,
		Entries:          1,
	}
	if err := readEntriesWithLimits(t, data, limits); !errors.Is(err, ErrLimit) {
		t.Fatalf("error = %v, want ErrLimit", err)
	}
}

func TestNegativeLimits(t *testing.T) {
	_, err := NewReaderWithLimits(bytes.NewReader(nil), Limits{CompressedSize: -1})
	if err == nil {
		t.Fatal("NewReaderWithLimits accepted negative limits")
	}
}

func TestOpenWithLimits(t *testing.T) {
	raw := makeTar(t, []fixtureEntry{{name: "plugins/mod.esp", body: "esp"}})
	data := gzipData(t, raw)
	file := filepath.Join(t.TempDir(), "mods.tar.gz")
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	limits := Limits{
		CompressedSize:   int64(len(data)) - 1,
		UncompressedSize: int64(len(raw)),
		Entries:          1,
	}
	if _, err := OpenWithLimits(file, limits); !errors.Is(err, ErrLimit) {
		t.Fatalf("OpenWithLimits error = %v, want ErrLimit", err)
	}

	limits.CompressedSize++
	reader, err := OpenWithLimits(file, limits)
	if err != nil {
		t.Fatalf("OpenWithLimits: %v", err)
	}
	defer reader.Close()

	entry, err := reader.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if entry.Name != "plugins/mod.esp" {
		t.Fatalf("entry.Name = %q", entry.Name)
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(body) != "esp" {
		t.Fatalf("body = %q", body)
	}
	if _, err := reader.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("final Next = %v, want io.EOF", err)
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

func makeLargeTarGz(t *testing.T, size int64) []byte {
	t.Helper()

	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{
		Name:     "plugins/big.bin",
		Typeflag: tar.TypeReg,
		Mode:     0o600,
		Size:     size,
		ModTime:  time.Unix(0, 0),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("WriteHeader: %v", err)
	}
	if _, err := io.CopyN(tw, zeroReader{}, size); err != nil {
		t.Fatalf("CopyN: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return compressed.Bytes()
}

func TestLargeEntryStreams(t *testing.T) {
	const size = int64(32 << 20)

	reader, err := NewReader(bytes.NewReader(makeLargeTarGz(t, size)))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer reader.Close()

	entry, err := reader.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if entry.Size != size {
		t.Fatalf("entry.Size = %d, want %d", entry.Size, size)
	}

	n, err := io.CopyBuffer(io.Discard, reader, make([]byte, 32*1024))
	if err != nil {
		t.Fatalf("CopyBuffer: %v", err)
	}
	if n != size {
		t.Fatalf("copied %d bytes, want %d", n, size)
	}
	if _, err := reader.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("final Next = %v, want io.EOF", err)
	}
}
