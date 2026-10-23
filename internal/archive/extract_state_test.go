package archive

import (
	"archive/tar"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func stateDeployEntries(t *testing.T, current Current) []fixtureEntry {
	t.Helper()

	entries := stateEntries(current)
	return append(entries,
		fixtureEntry{name: "players/alice.json", body: "{\"restored\":true}\n"},
		fixtureEntry{name: "world/cell/cell.json", body: "cell\n"},
		fixtureEntry{name: "world/custom/custom.json", body: "custom\n"},
	)
}

func stateEntriesSize(entries []fixtureEntry) int64 {
	var size int64
	for _, entry := range entries {
		size += int64(len(entry.body))
	}
	return size
}

func TestExtractStateAndValidateSize(t *testing.T) {
	modsDir, current := modsDirWith(t, "mods")
	entries := stateDeployEntries(t, current)
	path := filepath.Join(t.TempDir(), "state.tar.gz")
	if err := os.WriteFile(path, makeTarGz(t, entries), 0o600); err != nil {
		t.Fatal(err)
	}

	info, size, err := ValidateStateFileWithSize(path, modsDir)
	if err != nil {
		t.Fatalf("ValidateStateFileWithSize: %v", err)
	}
	if info.Players != 1 || info.WorldFiles != 2 {
		t.Fatalf("info = %+v, want one player and two world files", info)
	}
	if want := stateEntriesSize(entries); size != want {
		t.Fatalf("uncompressed = %d, want %d", size, want)
	}

	dest := filepath.Join(t.TempDir(), "stage")
	if err := ExtractStateFile(path, dest); err != nil {
		t.Fatalf("ExtractStateFile: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "players", "alice.json"))
	if err != nil || string(got) != "{\"restored\":true}\n" {
		t.Fatalf("extracted player = %q, %v", got, err)
	}
	got, err = os.ReadFile(filepath.Join(dest, "world", "cell", "cell.json"))
	if err != nil || string(got) != "cell\n" {
		t.Fatalf("extracted world file = %q, %v", got, err)
	}
	for _, dir := range stateRequiredDirs() {
		if _, err := os.Stat(filepath.Join(dest, dir)); err != nil {
			t.Fatalf("extracted %s: %v", dir, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dest, currentFileName)); err != nil {
		t.Fatalf("extracted %s: %v", currentFileName, err)
	}
}

func TestExtractStateReadsStreamingReader(t *testing.T) {
	_, current := modsDirWith(t, "mods")
	reader, err := NewReader(bytes.NewReader(
		makeTarGz(t, stateDeployEntries(t, current))))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	dest := t.TempDir()
	if err := ExtractState(reader, dest); err != nil {
		t.Fatalf("ExtractState: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "players", "alice.json")); err != nil {
		t.Fatalf("extracted player: %v", err)
	}
}

func TestExtractStateRejectsInvalidLayout(t *testing.T) {
	_, current := modsDirWith(t, "mods")
	base := stateEntries(current)

	tests := []struct {
		name    string
		entries []fixtureEntry
		want    string
	}{
		{
			name: "flat world",
			entries: append(withoutEntry(base, "world/cell/"),
				fixtureEntry{name: "cell/", typ: tar.TypeDir},
				fixtureEntry{name: "cell/cell.json", body: "{}"},
			),
			want: "unexpected top-level",
		},
		{
			name:    "missing current",
			entries: withoutEntry(base, currentFileName),
			want:    currentFileName,
		},
		{
			name: "missing world custom",
			entries: append(withoutEntry(base, "world/custom/"),
				fixtureEntry{name: "players/alice.json", body: "{}"},
			),
			want: "world/custom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := NewReader(bytes.NewReader(makeTarGz(t, tt.entries)))
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()

			err = ExtractState(reader, filepath.Join(t.TempDir(), "stage"))
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("error = %v, want ErrInvalid", err)
			}
			if !bytes.Contains([]byte(err.Error()), []byte(tt.want)) {
				t.Fatalf("error = %q, want substring %q", err, tt.want)
			}
		})
	}
}

func TestExtractStateRejectsNilInputs(t *testing.T) {
	if err := ExtractState(nil, t.TempDir()); err == nil {
		t.Fatal("ExtractState(nil) succeeded")
	}
	reader, err := NewReader(bytes.NewReader(makeTarGz(t, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if err := ExtractState(reader, ""); err == nil {
		t.Fatal("ExtractState with empty destination succeeded")
	}
}
