package restorepoint

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

var testNow = time.Date(2026, 10, 27, 16, 40, 0, 0, time.UTC)

func TestParseNameKinds(t *testing.T) {
	tests := []struct {
		name  string
		kind  Kind
		valid bool
	}{
		{"snapshot-2026-10-27_16-40-00.tar.gz", KindSnapshot, true},
		{"checkpoint-2026-10-27_16-40-00.tar.gz", KindCheckpoint, true},
		{"checkpoint-predeploy-2026-10-27_16-40-00.tar.gz", KindPredeploy, true},
		{"snapshot-2026-10-27_16-40-00-01.tar.gz", KindSnapshot, true},
		{"snapshot-2026-10-27_16-40-00.zip", KindSnapshot, false},
		{"snapshot-2026-10-27_16-40-00-00.tar.gz", KindSnapshot, false},
		{"snapshot-../16-40-00.tar.gz", KindSnapshot, false},
		{"state-2026-10-27_16-40-00.tar.gz", KindSnapshot, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ParseName(tt.name)
			if !tt.valid {
				if err == nil {
					t.Fatalf("ParseName(%q) succeeded, want error", tt.name)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseName(%q): %v", tt.name, err)
			}
			if parsed.Kind != tt.kind {
				t.Fatalf("kind = %s, want %s", parsed.Kind, tt.kind)
			}
			if parsed.String() != tt.name {
				t.Fatalf("round trip = %q, want %q", parsed.String(), tt.name)
			}
		})
	}
}

func TestValidateNameRejectsPredeployForCheckpoint(t *testing.T) {
	if err := ValidateName(KindCheckpoint,
		"checkpoint-predeploy-2026-10-27_16-40-00.tar.gz"); err == nil {
		t.Fatal("ValidateName accepted a predeploy archive as a checkpoint")
	}
}

func TestNextNameAddsCollision(t *testing.T) {
	dir := t.TempDir()
	name, err := NextName(dir, KindSnapshot, testNow)
	if err != nil {
		t.Fatalf("NextName: %v", err)
	}
	if name != "snapshot-2026-10-27_16-40-00.tar.gz" {
		t.Fatalf("name = %q", name)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	name, err = NextName(dir, KindSnapshot, testNow)
	if err != nil {
		t.Fatalf("NextName collision: %v", err)
	}
	if name != "snapshot-2026-10-27_16-40-00-01.tar.gz" {
		t.Fatalf("collision name = %q", name)
	}
}

func TestDigestIsOrderIndependent(t *testing.T) {
	files := []File{
		{Path: "players/b.json", Size: 2, SHA256: "bb"},
		{Path: "players/a.json", Size: 1, SHA256: "aa"},
	}
	reversed := slices.Clone(files)
	slices.Reverse(reversed)
	first := Digest("state", files)
	if first != Digest("state", reversed) {
		t.Fatal("Digest depends on file order")
	}
	if first == Digest("mods", files) {
		t.Fatal("Digest does not include the layer name")
	}
}
