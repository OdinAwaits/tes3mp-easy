package archive

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseArchiveNameAcceptsAllKinds(t *testing.T) {
	ts := "2026-10-22_12-00-00"
	tests := []struct {
		name      string
		kind      ArchiveKind
		collision int
	}{
		{"mods-" + ts + ".tar.gz", KindMods, 0},
		{"mods-init-" + ts + ".tar.gz", KindModsInit, 0},
		{"state-" + ts + ".tar.gz", KindStateAuto, 0},
		{"state-manual-" + ts + ".tar.gz", KindStateManual, 0},
		{"state-predeploy-" + ts + ".tar.gz", KindStatePredeploy, 0},
		{"mods-" + ts + "-01.tar.gz", KindMods, 1},
		{"state-manual-" + ts + "-42.tar.gz", KindStateManual, 42},
		{"state-predeploy-" + ts + "-100.tar.gz", KindStatePredeploy, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ParseArchiveName(tt.name)
			if err != nil {
				t.Fatalf("ParseArchiveName: %v", err)
			}
			if parsed.Kind != tt.kind || parsed.Collision != tt.collision {
				t.Fatalf("parsed = %+v, want kind %s collision %d",
					parsed, tt.kind, tt.collision)
			}
			if parsed.Time.Location() != time.UTC {
				t.Fatalf("parsed time location = %v, want UTC", parsed.Time.Location())
			}
			if got := parsed.String(); got != tt.name {
				t.Fatalf("String() = %q, want %q", got, tt.name)
			}
		})
	}
}

func TestParseArchiveNameRejectsInvalidNames(t *testing.T) {
	tests := []string{
		"",
		"archive.tar.gz",
		"mods.tar.gz",
		"mods-2026-10-22_12-00-00",
		"mods-2026-10-22_12-00-00.tar",
		"mods-2026-02-30_12-00-00.tar.gz",
		"mods-2026-10-22_24-00-00.tar.gz",
		"mods-2026-10-22_12-00-00-.tar.gz",
		"mods-2026-10-22_12-00-00-00.tar.gz",
		"mods-2026-10-22_12-00-00-1.tar.gz",
		"mods-2026-10-22_12-00-00-001.tar.gz",
		"mods-2026-10-22_12-00-00-010.tar.gz",
		"mods-2026-10-22_12-00-00-xx.tar.gz",
		"mods-2026-10-22_12-00-00_01.tar.gz",
		"mods-2026-10-22_12-00-00-01-extra.tar.gz",
		"../mods-2026-10-22_12-00-00.tar.gz",
		"mods-2026-10-22_12-00-00.tar.gz/../evil",
		"state-manual-.tar.gz",
		"state-predeploy-2026-10-22_12-00-00.tar.gz.bak",
	}
	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseArchiveName(name); !errors.Is(err, ErrInvalid) {
				t.Fatalf("ParseArchiveName(%q) error = %v, want ErrInvalid", name, err)
			}
		})
	}
}

func TestArchiveNameValidatorsCheckKind(t *testing.T) {
	if err := ValidateModsArchiveName("mods-2026-10-22_12-00-00.tar.gz"); err != nil {
		t.Fatalf("ValidateModsArchiveName: %v", err)
	}
	if err := ValidateStateArchiveName("state-manual-2026-10-22_12-00-00-01.tar.gz"); err != nil {
		t.Fatalf("ValidateStateArchiveName: %v", err)
	}
	if err := ValidateModsArchiveName("state-2026-10-22_12-00-00.tar.gz"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("mods validator accepted a state archive: %v", err)
	}
	if err := ValidateStateArchiveName("mods-init-2026-10-22_12-00-00.tar.gz"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("state validator accepted a mods archive: %v", err)
	}
}

func TestNextArchiveNameAddsCollisionSuffix(t *testing.T) {
	at := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	used := map[string]bool{
		"state-2026-10-22_12-00-00.tar.gz":    true,
		"state-2026-10-22_12-00-00-01.tar.gz": true,
	}
	name, err := NextArchiveName(KindStateAuto, at, func(name string) bool {
		return used[name]
	})
	if err != nil {
		t.Fatalf("NextArchiveName: %v", err)
	}
	if want := "state-2026-10-22_12-00-00-02.tar.gz"; name != want {
		t.Fatalf("NextArchiveName = %q, want %q", name, want)
	}
}

func TestNextArchiveNameInDir(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	first := filepath.Join(dir, "mods-2026-10-22_12-00-00.tar.gz")
	if err := os.WriteFile(first, []byte("archive"), 0o600); err != nil {
		t.Fatal(err)
	}

	name, err := NextArchiveNameInDir(dir, KindMods, at)
	if err != nil {
		t.Fatalf("NextArchiveNameInDir: %v", err)
	}
	if want := "mods-2026-10-22_12-00-00-01.tar.gz"; name != want {
		t.Fatalf("NextArchiveNameInDir = %q, want %q", name, want)
	}
	if _, err := NextArchiveNameInDir("", KindMods, at); err == nil {
		t.Fatal("NextArchiveNameInDir accepted an empty directory")
	}
}
