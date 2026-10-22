package archive

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validSHA(body string) string {
	sum := sha256.Sum256([]byte(body))
	return fmt.Sprintf("%x", sum)
}

func currentLine(sha, name string) string {
	return sha + " " + name + "\n"
}

func TestParseCurrentAcceptsCanonicalFormats(t *testing.T) {
	sha := strings.Repeat("a", 64)
	names := []string{
		"mods-2026-10-22_12-00-00.tar.gz",
		"mods-init-2026-10-22_12-00-00.tar.gz",
		"mods-2026-10-22_12-00-00-01.tar.gz",
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			current, err := ParseCurrent([]byte(currentLine(sha, name)))
			if err != nil {
				t.Fatalf("ParseCurrent: %v", err)
			}
			if current.SHA256 != sha || current.Name != name {
				t.Fatalf("current = %+v, want %s %s", current, sha, name)
			}
		})
	}
}

func TestParseCurrentRejectsInvalidFormats(t *testing.T) {
	sha := strings.Repeat("a", 64)
	name := "mods-2026-10-22_12-00-00.tar.gz"

	tests := []struct {
		name string
		data string
	}{
		{name: "empty", data: ""},
		{name: "missing newline", data: sha + " " + name},
		{name: "crlf", data: sha + " " + name + "\r\n"},
		{name: "sha only", data: sha + "\n"},
		{name: "missing separator", data: strings.Repeat("a", 64) + name + "\n"},
		{name: "two spaces", data: sha + "  " + name + "\n"},
		{name: "tab separator", data: sha + "\t" + name + "\n"},
		{name: "trailing space", data: sha + " " + name + " \n"},
		{name: "empty filename", data: sha + " \n"},
		{name: "uppercase sha", data: strings.Repeat("A", 64) + " " + name + "\n"},
		{name: "short sha", data: strings.Repeat("a", 63) + " " + name + "\n"},
		{name: "non-hex sha", data: strings.Repeat("g", 64) + " " + name + "\n"},
		{name: "state archive", data: sha + " state-2026-10-22_12-00-00.tar.gz\n"},
		{name: "path in filename", data: sha + " ../" + name + "\n"},
		{name: "invalid timestamp", data: sha + " mods-2026-13-22_12-00-00.tar.gz\n"},
		{name: "invalid suffix", data: sha + " mods-2026-10-22_12-00-00-1.tar.gz\n"},
		{name: "extra line", data: sha + " " + name + "\nextra\n"},
		{name: "blank line", data: "\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseCurrent([]byte(tt.data)); !errors.Is(err, ErrInvalid) {
				t.Fatalf("ParseCurrent error = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestValidateModsArchiveName(t *testing.T) {
	valid := []string{
		"mods-2026-10-22_12-00-00.tar.gz",
		"mods-init-2026-10-22_12-00-00.tar.gz",
		"mods-2026-10-22_12-00-00-01.tar.gz",
		"mods-init-2026-10-22_12-00-00-42.tar.gz",
	}
	for _, name := range valid {
		if err := ValidateModsArchiveName(name); err != nil {
			t.Errorf("ValidateModsArchiveName(%q): %v", name, err)
		}
	}

	invalid := []string{
		"",
		"mods.tar.gz",
		"mods-2026-10-22_12-00-00",
		"mods-2026-10-22_12-00-00.tar",
		"mods-2026-02-30_12-00-00.tar.gz",
		"mods-2026-10-22_24-00-00.tar.gz",
		"mods-2026-10-22_12-00-00-.tar.gz",
		"mods-2026-10-22_12-00-00-00.tar.gz",
		"mods-2026-10-22_12-00-00-1.tar.gz",
		"mods-2026-10-22_12-00-00-xx.tar.gz",
		"state-2026-10-22_12-00-00.tar.gz",
		"../mods-2026-10-22_12-00-00.tar.gz",
		"mods-2026-10-22_12-00-00.tar.gz/../evil",
	}
	for _, name := range invalid {
		if err := ValidateModsArchiveName(name); !errors.Is(err, ErrInvalid) {
			t.Errorf("ValidateModsArchiveName(%q) = %v, want ErrInvalid", name, err)
		}
	}
}

func TestReadCurrentFile(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, currentFileName)
	sha := validSHA("mods")
	want := Current{SHA256: sha, Name: "mods-2026-10-22_12-00-00.tar.gz"}

	if err := os.WriteFile(name, []byte(currentLine(want.SHA256, want.Name)), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadCurrentFile(name)
	if err != nil {
		t.Fatalf("ReadCurrentFile: %v", err)
	}
	if got != want {
		t.Fatalf("current = %+v, want %+v", got, want)
	}

	if _, err := ReadCurrentFile(filepath.Join(dir, "missing.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing ReadCurrentFile error = %v, want os.ErrNotExist", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "bad.txt"), []byte("bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadCurrentFile(filepath.Join(dir, "bad.txt")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("bad ReadCurrentFile error = %v, want ErrInvalid", err)
	}

	oversized := filepath.Join(dir, "oversized.txt")
	if err := os.WriteFile(oversized, bytes.Repeat([]byte("a"), int(MaxCurrentFileSize)+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadCurrentFile(oversized); !errors.Is(err, ErrInvalid) {
		t.Fatalf("oversized ReadCurrentFile error = %v, want ErrInvalid", err)
	}

	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(name, link); err == nil {
		if _, err := ReadCurrentFile(link); !errors.Is(err, ErrInvalid) {
			t.Fatalf("symlink ReadCurrentFile error = %v, want ErrInvalid", err)
		}
	}
}
