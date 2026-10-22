package archive

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

const (
	// MaxCurrentFileSize bounds current.txt. The canonical format is at most
	// 64 + 1 + filename + 1 bytes, so 4 KiB leaves ample room for long names
	// while still rejecting accidental oversized files.
	MaxCurrentFileSize int64 = 4 << 10

	currentFileName = "current.txt"
)

// Current is the canonical content of current.txt from
// docs/v2/03-contracts.md §6.4.
type Current struct {
	SHA256 string
	Name   string
}

// ParseCurrent parses and validates the only accepted current.txt format:
// "<sha256> <filename>\n". It does not check that the referenced archive exists.
func ParseCurrent(data []byte) (Current, error) {
	if len(data) == 0 {
		return Current{}, archiveInvalidf("%s: empty file", currentFileName)
	}
	if data[len(data)-1] != '\n' {
		return Current{}, archiveInvalidf(
			"%s: missing trailing newline", currentFileName)
	}

	line := data[:len(data)-1]
	if bytes.IndexByte(line, '\n') >= 0 || bytes.IndexByte(line, '\r') >= 0 {
		return Current{}, archiveInvalidf(
			"%s: must contain exactly one LF-terminated line", currentFileName)
	}

	space := bytes.IndexByte(line, ' ')
	if space < 0 {
		return Current{}, archiveInvalidf(
			"%s: expected <sha256> <filename>", currentFileName)
	}
	if bytes.IndexByte(line[space+1:], ' ') >= 0 {
		return Current{}, archiveInvalidf(
			"%s: expected exactly one space between fields", currentFileName)
	}

	sha := string(line[:space])
	if !isSHA256Hex(sha) {
		return Current{}, archiveInvalidf(
			"%s: sha256 must be 64 lowercase hex characters", currentFileName)
	}

	name := string(line[space+1:])
	if name == "" {
		return Current{}, archiveInvalidf(
			"%s: archive filename is missing", currentFileName)
	}
	if err := ValidateModsArchiveName(name); err != nil {
		return Current{}, fmt.Errorf("%s: %w", currentFileName, err)
	}

	return Current{SHA256: sha, Name: name}, nil
}

// ReadCurrentFile reads and parses a current.txt file. It rejects symlinks and
// non-regular files, and wraps format errors with ErrInvalid.
func ReadCurrentFile(name string) (Current, error) {
	info, err := os.Lstat(name)
	if err != nil {
		return Current{}, err
	}
	if !info.Mode().IsRegular() {
		return Current{}, archiveInvalidf("%s: not a regular file", name)
	}
	if info.Size() > MaxCurrentFileSize {
		return Current{}, archiveInvalidf(
			"%s: file exceeds %d bytes", name, MaxCurrentFileSize)
	}

	f, err := os.Open(name)
	if err != nil {
		return Current{}, err
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, MaxCurrentFileSize+1))
	if err != nil {
		return Current{}, err
	}
	if int64(len(data)) > MaxCurrentFileSize {
		return Current{}, archiveInvalidf(
			"%s: file exceeds %d bytes", name, MaxCurrentFileSize)
	}

	current, err := ParseCurrent(data)
	if err != nil {
		return Current{}, fmt.Errorf("%s: %w", name, err)
	}
	return current, nil
}

// VerifyStateMods checks that the state metadata points at the current mods
// archive and that the archive exists with the recorded sha256. A state archive
// never replaces mods by itself, so any mismatch is archive_invalid.
func VerifyStateMods(info StateInfo, modsDir string) error {
	if modsDir == "" {
		return errors.New("archive: mods directory is required")
	}
	if err := validateCurrent(info.Current); err != nil {
		return err
	}

	currentPath := filepath.Join(modsDir, currentFileName)
	modsCurrent, err := ReadCurrentFile(currentPath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return archiveInvalidf(
			"%s: current mods archive metadata is missing", currentPath)
	case err != nil:
		return err
	}

	if modsCurrent != info.Current {
		return archiveInvalidf(
			"%s: current mods archive is %q (%s), state expects %q (%s)",
			currentPath, modsCurrent.Name, modsCurrent.SHA256,
			info.Current.Name, info.Current.SHA256)
	}

	return verifyArchiveSHA256(
		filepath.Join(modsDir, info.Current.Name), info.Current.SHA256)
}

func validateCurrent(current Current) error {
	if current.Name == "" {
		return archiveInvalidf("%s: archive filename is missing", currentFileName)
	}
	if err := ValidateModsArchiveName(current.Name); err != nil {
		return err
	}
	if !isSHA256Hex(current.SHA256) {
		return archiveInvalidf(
			"%s: sha256 must be 64 lowercase hex characters", currentFileName)
	}
	return nil
}

func isSHA256Hex(s string) bool {
	if len(s) != sha256.Size*2 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' || c >= 'a' && c <= 'f' {
			continue
		}
		return false
	}
	return true
}

func verifyArchiveSHA256(name, want string) error {
	info, err := os.Lstat(name)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return archiveInvalidf(
			"%s: archive referenced by %s does not exist", name, currentFileName)
	case err != nil:
		return err
	}
	if !info.Mode().IsRegular() {
		return archiveInvalidf("%s: not a regular file", name)
	}
	if info.Size() > MaxCompressedSize {
		return fmt.Errorf(
			"%w: %s exceeds %d bytes", ErrLimit, name, MaxCompressedSize)
	}

	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()

	sum := sha256.New()
	if _, err := io.CopyBuffer(sum, f, make([]byte, 32*1024)); err != nil {
		return err
	}
	got := hex.EncodeToString(sum.Sum(nil))
	if got != want {
		return archiveInvalidf(
			"%s: sha256 mismatch (expected %s, got %s)", name, want, got)
	}
	return nil
}
