package archive

import (
	"archive/tar"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func deployModsEntries(body, script string) []fixtureEntry {
	metadata := requiredJSON(append(
		baseRequiredEntries(), requiredEntry("mod.esp", modCRC(body)))...)
	return []fixtureEntry{
		{name: "plugins/", typ: tar.TypeDir},
		{name: "plugins/mod.esp", body: body},
		{name: "plugins/requiredDataFiles.json", body: metadata},
		{name: "scripts/", typ: tar.TypeDir},
		{name: "scripts/test.lua", body: script},
	}
}

func deployModsArchiveFile(t *testing.T, entries []fixtureEntry) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mods-2026-10-22_14-00-00.tar.gz")
	if err := os.WriteFile(path, makeTarGz(t, entries), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExtractModsAndValidateModsDir(t *testing.T) {
	const (
		body   = "mod data"
		script = "print(\"synthetic\")\n"
	)
	entries := deployModsEntries(body, script)
	archivePath := deployModsArchiveFile(t, entries)
	metadata := requiredJSON(append(
		baseRequiredEntries(), requiredEntry("mod.esp", modCRC(body)))...)

	info, size, err := ValidateModsFileWithSize(archivePath)
	if err != nil {
		t.Fatalf("ValidateModsFileWithSize: %v", err)
	}
	if info.Plugins != 1 || info.Scripts != 1 {
		t.Fatalf("info = %+v, want one plugin and one script", info)
	}
	if want := int64(len(body) + len(metadata) + len(script)); size != want {
		t.Fatalf("uncompressed = %d, want %d", size, want)
	}

	dest := filepath.Join(t.TempDir(), "stage")
	if err := ExtractModsFile(archivePath, dest); err != nil {
		t.Fatalf("ExtractModsFile: %v", err)
	}
	live, err := ValidateModsDir(dest)
	if err != nil {
		t.Fatalf("ValidateModsDir: %v", err)
	}
	if live != info {
		t.Fatalf("live info = %+v, want %+v", live, info)
	}
	if got, err := os.ReadFile(filepath.Join(dest, "scripts", "test.lua")); err != nil ||
		string(got) != script {
		t.Fatalf("extracted script = %q, %v", got, err)
	}
}

func TestValidateModsDirRejectsBadTrees(t *testing.T) {
	const body = "mod data"
	tests := []struct {
		name   string
		mutate func(t *testing.T, dir string)
	}{
		{
			name: "missing metadata",
			mutate: func(t *testing.T, dir string) {
				if err := os.Remove(filepath.Join(
					dir, "plugins", requiredDataFilesName)); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "base ESM",
			mutate: func(t *testing.T, dir string) {
				if err := os.WriteFile(filepath.Join(
					dir, "plugins", "Morrowind.esm"), []byte("esm"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "nested directory",
			mutate: func(t *testing.T, dir string) {
				if err := os.Mkdir(filepath.Join(
					dir, "plugins", "nested.esp"), 0o700); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "symlink",
			mutate: func(t *testing.T, dir string) {
				if err := os.Symlink(filepath.Join(dir, "plugins", "mod.esp"),
					filepath.Join(dir, "plugins", "link.esp")); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "metadata symlink",
			mutate: func(t *testing.T, dir string) {
				metadata := filepath.Join(dir, "plugins", requiredDataFilesName)
				data, err := os.ReadFile(metadata)
				if err != nil {
					t.Fatal(err)
				}
				target := filepath.Join(dir, "metadata.json")
				if err := os.WriteFile(target, data, 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(metadata); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, metadata); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "non-lua script",
			mutate: func(t *testing.T, dir string) {
				if err := os.WriteFile(filepath.Join(
					dir, "scripts", "readme.txt"), []byte("text"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "missing required plugin",
			mutate: func(t *testing.T, dir string) {
				if err := os.Remove(filepath.Join(
					dir, "plugins", "mod.esp")); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "plugin CRC mismatch",
			mutate: func(t *testing.T, dir string) {
				if err := os.WriteFile(filepath.Join(
					dir, "plugins", "mod.esp"), []byte("changed"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dest := filepath.Join(t.TempDir(), "stage")
			if err := ExtractModsFile(
				deployModsArchiveFile(t, deployModsEntries(body, "")), dest); err != nil {
				t.Fatalf("ExtractModsFile: %v", err)
			}
			tt.mutate(t, dest)
			if _, err := ValidateModsDir(dest); !errors.Is(err, ErrInvalid) {
				t.Fatalf("ValidateModsDir error = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestExtractModsReadsStreamingReader(t *testing.T) {
	entries := deployModsEntries("mod data", "")
	reader, err := NewReader(bytes.NewReader(makeTarGz(t, entries)))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	dest := t.TempDir()
	if err := ExtractMods(reader, dest); err != nil {
		t.Fatalf("ExtractMods: %v", err)
	}
	if _, err := ValidateModsDir(dest); err != nil {
		t.Fatalf("ValidateModsDir: %v", err)
	}
}

func TestExtractModsEnforcesUncompressedLimit(t *testing.T) {
	data := makeTarGz(t, deployModsEntries("mod data", ""))
	limits := Limits{
		CompressedSize:   int64(len(data)),
		UncompressedSize: 512,
		Entries:          10,
	}
	reader, err := NewReaderWithLimits(bytes.NewReader(data), limits)
	if err != nil {
		t.Fatalf("NewReaderWithLimits: %v", err)
	}
	defer reader.Close()

	dest := filepath.Join(t.TempDir(), "stage")
	if err := ExtractMods(reader, dest); !errors.Is(err, ErrLimit) {
		t.Fatalf("ExtractMods error = %v, want ErrLimit", err)
	}
}
