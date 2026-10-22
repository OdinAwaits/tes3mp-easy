package archive

import (
	"archive/tar"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testModsName = "mods-2026-10-22_12-00-00.tar.gz"

func modsDirWith(t *testing.T, body string) (string, Current) {
	t.Helper()

	dir := t.TempDir()
	current := Current{SHA256: validSHA(body), Name: testModsName}
	if err := os.WriteFile(
		filepath.Join(dir, current.Name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	writeCurrentRecord(t, dir, current)
	return dir, current
}

func writeCurrentRecord(t *testing.T, dir string, current Current) {
	t.Helper()

	if err := os.WriteFile(
		filepath.Join(dir, currentFileName),
		[]byte(currentLine(current.SHA256, current.Name)),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
}

func stateEntries(current Current) []fixtureEntry {
	entries := []fixtureEntry{
		{name: "players/", typ: tar.TypeDir},
		{name: "world/", typ: tar.TypeDir},
	}
	for _, name := range stateWorldDirs {
		entries = append(entries, fixtureEntry{
			name: "world/" + name + "/",
			typ:  tar.TypeDir,
		})
	}
	return append(entries,
		fixtureEntry{
			name: requiredDataFilesName,
			body: requiredJSON(baseRequiredEntries()...),
		},
		fixtureEntry{
			name: currentFileName,
			body: currentLine(current.SHA256, current.Name),
		},
	)
}

func cloneEntries(entries []fixtureEntry) []fixtureEntry {
	return append([]fixtureEntry(nil), entries...)
}

func withoutEntry(entries []fixtureEntry, name string) []fixtureEntry {
	filtered := make([]fixtureEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.name != name {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func replaceEntry(entries []fixtureEntry, name, body string) []fixtureEntry {
	replaced := cloneEntries(entries)
	for i := range replaced {
		if replaced[i].name == name {
			replaced[i].body = body
		}
	}
	return replaced
}

func validateStateEntries(t *testing.T, entries []fixtureEntry, modsDir string) (StateInfo, error) {
	t.Helper()
	return ValidateStateReader(bytes.NewReader(makeTarGz(t, entries)), modsDir)
}

func TestValidateStateAcceptsEmptyArchive(t *testing.T) {
	modsDir, current := modsDirWith(t, "mods")
	info, err := validateStateEntries(t, stateEntries(current), modsDir)
	if err != nil {
		t.Fatalf("ValidateStateReader: %v", err)
	}
	if info.Current != current {
		t.Fatalf("current = %+v, want %+v", info.Current, current)
	}
	if info.Players != 0 || info.WorldFiles != 0 {
		t.Fatalf("info = %+v, want no state files", info)
	}
	if len(info.RequiredDataFiles) != len(baseRequiredEntries()) {
		t.Fatalf("required entries = %d, want %d",
			len(info.RequiredDataFiles), len(baseRequiredEntries()))
	}
}

func TestValidateStateAcceptsFullArchive(t *testing.T) {
	modsDir, current := modsDirWith(t, "mods")
	entries := append(cloneEntries(stateEntries(current)),
		fixtureEntry{name: "players/alice.json", body: "{}"},
		fixtureEntry{name: "players/bob.json", body: "{}"},
		fixtureEntry{name: "world/cell/cell.json", body: "{}"},
		fixtureEntry{name: "world/custom/custom.json", body: "{}"},
	)

	info, err := validateStateEntries(t, entries, modsDir)
	if err != nil {
		t.Fatalf("ValidateStateReader: %v", err)
	}
	if info.Players != 2 || info.WorldFiles != 2 {
		t.Fatalf("info = %+v, want 2 players and 2 world files", info)
	}
}

func TestValidateStateAcceptsImplicitDirectories(t *testing.T) {
	modsDir, current := modsDirWith(t, "mods")
	entries := []fixtureEntry{
		{name: "players/alice.json", body: "{}"},
		{name: "world/cell/cell.json", body: "{}"},
		{name: "world/world/world.json", body: "{}"},
		{name: "world/map/map.json", body: "{}"},
		{name: "world/recordstore/recordstore.json", body: "{}"},
		{name: "world/custom/custom.json", body: "{}"},
		{name: requiredDataFilesName, body: requiredJSON(baseRequiredEntries()...)},
		{name: currentFileName, body: currentLine(current.SHA256, current.Name)},
	}

	if _, err := validateStateEntries(t, entries, modsDir); err != nil {
		t.Fatalf("ValidateStateReader: %v", err)
	}
}

func TestValidateStateRejectsInvalidLayout(t *testing.T) {
	modsDir, current := modsDirWith(t, "mods")
	base := stateEntries(current)
	sha := strings.Repeat("a", 64)
	worldless := withoutEntry(base, "world/")
	for _, name := range stateWorldDirs {
		worldless = withoutEntry(worldless, "world/"+name+"/")
	}

	tests := []struct {
		name    string
		entries []fixtureEntry
		want    string
	}{
		{
			name: "flat cell",
			entries: append(cloneEntries(base), fixtureEntry{
				name: "cell/", typ: tar.TypeDir,
			}),
			want: "unexpected top-level directory",
		},
		{
			name:    "missing current",
			entries: withoutEntry(base, currentFileName),
			want:    currentFileName,
		},
		{
			name:    "missing required JSON",
			entries: withoutEntry(base, requiredDataFilesName),
			want:    requiredDataFilesName,
		},
		{
			name:    "missing players",
			entries: withoutEntry(base, "players/"),
			want:    "players",
		},
		{
			name:    "missing world",
			entries: worldless,
			want:    "world",
		},
		{
			name:    "missing world custom",
			entries: withoutEntry(base, "world/custom/"),
			want:    "world/custom",
		},
		{
			name: "nested players directory",
			entries: append(cloneEntries(base), fixtureEntry{
				name: "players/sub/", typ: tar.TypeDir,
			}),
			want: "nested directories",
		},
		{
			name: "nested world directory",
			entries: append(cloneEntries(base), fixtureEntry{
				name: "world/cell/sub/", typ: tar.TypeDir,
			}),
			want: "nested directories",
		},
		{
			name: "unknown world directory",
			entries: append(cloneEntries(base), fixtureEntry{
				name: "world/extra/", typ: tar.TypeDir,
			}),
			want: "unexpected world directory",
		},
		{
			name: "world file without subdirectory",
			entries: append(cloneEntries(base), fixtureEntry{
				name: "world/foo", body: "{}",
			}),
			want: "expected world/<name>/<file>",
		},
		{
			name: "nested world file",
			entries: append(cloneEntries(base), fixtureEntry{
				name: "world/cell/sub/foo", body: "{}",
			}),
			want: "nested files",
		},
		{
			name: "nested player file",
			entries: append(cloneEntries(base), fixtureEntry{
				name: "players/sub/foo", body: "{}",
			}),
			want: "nested files",
		},
		{
			name: "extra top-level file",
			entries: append(cloneEntries(base), fixtureEntry{
				name: "extra.txt", body: "{}",
			}),
			want: "unexpected top-level file",
		},
		{
			name: "unexpected top-level directory",
			entries: append(cloneEntries(base), fixtureEntry{
				name: "plugins/", typ: tar.TypeDir,
			}),
			want: "unexpected top-level directory",
		},
		{
			name:    "old current without filename",
			entries: replaceEntry(base, currentFileName, sha+"\n"),
			want:    "expected <sha256> <filename>",
		},
		{
			name:    "current CRLF",
			entries: replaceEntry(base, currentFileName, sha+" "+current.Name+"\r\n"),
			want:    "LF-terminated",
		},
		{
			name:    "current extra space",
			entries: replaceEntry(base, currentFileName, sha+"  "+current.Name+"\n"),
			want:    "exactly one space",
		},
		{
			name:    "current invalid archive name",
			entries: replaceEntry(base, currentFileName, sha+" state-2026-10-22_12-00-00.tar.gz\n"),
			want:    "must start with mods-",
		},
		{
			name:    "invalid required JSON",
			entries: replaceEntry(base, requiredDataFilesName, "{}"),
			want:    "top-level value must be an array",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateStateEntries(t, tt.entries, modsDir)
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("error = %v, want ErrInvalid", err)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want substring %q", err, tt.want)
			}
		})
	}
}

func TestValidateStateVerifiesCurrentMods(t *testing.T) {
	current := Current{
		SHA256: validSHA("other"),
		Name:   "mods-2026-10-22_12-00-01.tar.gz",
	}

	t.Run("missing current metadata", func(t *testing.T) {
		modsDir := t.TempDir()
		if _, err := validateStateEntries(t, stateEntries(current), modsDir); !errors.Is(err, ErrInvalid) {
			t.Fatalf("error = %v, want ErrInvalid", err)
		}
	})

	t.Run("foreign current", func(t *testing.T) {
		modsDir, _ := modsDirWith(t, "mods")
		if _, err := validateStateEntries(t, stateEntries(current), modsDir); !errors.Is(err, ErrInvalid) {
			t.Fatalf("error = %v, want ErrInvalid", err)
		}
	})

	t.Run("missing target archive", func(t *testing.T) {
		modsDir := t.TempDir()
		writeCurrentRecord(t, modsDir, current)
		if _, err := validateStateEntries(t, stateEntries(current), modsDir); !errors.Is(err, ErrInvalid) {
			t.Fatalf("error = %v, want ErrInvalid", err)
		}
	})

	t.Run("sha256 mismatch", func(t *testing.T) {
		modsDir := t.TempDir()
		current := Current{
			SHA256: strings.Repeat("0", 64),
			Name:   testModsName,
		}
		if err := os.WriteFile(
			filepath.Join(modsDir, current.Name), []byte("mods"), 0o600); err != nil {
			t.Fatal(err)
		}
		writeCurrentRecord(t, modsDir, current)
		if _, err := validateStateEntries(t, stateEntries(current), modsDir); !errors.Is(err, ErrInvalid) {
			t.Fatalf("error = %v, want ErrInvalid", err)
		}
	})

	t.Run("invalid current metadata", func(t *testing.T) {
		modsDir := t.TempDir()
		if err := os.WriteFile(
			filepath.Join(modsDir, currentFileName), []byte("bad"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := validateStateEntries(t, stateEntries(current), modsDir); !errors.Is(err, ErrInvalid) {
			t.Fatalf("error = %v, want ErrInvalid", err)
		}
	})
}

func TestValidateStateFile(t *testing.T) {
	modsDir, current := modsDirWith(t, "mods")
	file := filepath.Join(t.TempDir(), "state.tar.gz")
	if err := os.WriteFile(file, makeTarGz(t, stateEntries(current)), 0o600); err != nil {
		t.Fatal(err)
	}

	info, err := ValidateStateFile(file, modsDir)
	if err != nil {
		t.Fatalf("ValidateStateFile: %v", err)
	}
	if info.Current != current {
		t.Fatalf("current = %+v, want %+v", info.Current, current)
	}
}

func TestValidateStateRejectsNilReader(t *testing.T) {
	if _, err := ValidateState(nil); err == nil {
		t.Fatal("ValidateState(nil) succeeded, want error")
	}
}
