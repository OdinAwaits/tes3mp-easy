package atomicfile

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func readFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func requireMissing(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("%s exists or stat failed: %v", path, err)
	}
}

func TestWriteFileContract(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "config.lua")
	part := target + ".part"
	if err := os.WriteFile(target, []byte("old\n"), 0o600); err != nil {
		t.Fatalf("write old target: %v", err)
	}

	if err := WriteFile(target, []byte("new\n"), 0o640); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if got := readFile(t, target); got != "new\n" {
		t.Fatalf("target = %q, want new", got)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat target: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o640 {
		t.Fatalf("target permissions = %o, want 640", perm)
	}
	requireMissing(t, part)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "config.lua" {
		t.Fatalf("directory entries = %v, want only config.lua", entries)
	}
}

func TestCommitKeepsTargetInvisibleUntilRename(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "current.txt")
	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatalf("write old target: %v", err)
	}

	writer, err := New(target, 0o600)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := readFile(t, target); got != "old" {
		t.Fatalf("target before commit = %q, want old", got)
	}
	if _, err := writer.Write([]byte("new")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if got := readFile(t, target); got != "old" {
		t.Fatalf("target during write = %q, want old", got)
	}
	if got := readFile(t, target+".part"); got != "new" {
		t.Fatalf(".part = %q, want new", got)
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if got := readFile(t, target); got != "new" {
		t.Fatalf("target after commit = %q, want new", got)
	}
	requireMissing(t, target+".part")

	if err := writer.Commit(); !errors.Is(err, ErrClosed) {
		t.Fatalf("second Commit = %v, want ErrClosed", err)
	}
	if _, err := writer.Write([]byte("late")); !errors.Is(err, ErrClosed) {
		t.Fatalf("Write after Commit = %v, want ErrClosed", err)
	}
	if err := writer.Abort(); err != nil {
		t.Fatalf("Abort after Commit: %v", err)
	}
}

func TestAbortLeavesTargetIntact(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "banlist.json")
	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatalf("write old target: %v", err)
	}

	writer, err := New(target, 0o600)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := writer.Write([]byte("partial")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := writer.Abort(); err != nil {
		t.Fatalf("Abort: %v", err)
	}
	if err := writer.Abort(); err != nil {
		t.Fatalf("second Abort: %v", err)
	}
	if got := readFile(t, target); got != "old" {
		t.Fatalf("target after Abort = %q, want old", got)
	}
	requireMissing(t, target+".part")
	if err := writer.Commit(); !errors.Is(err, ErrClosed) {
		t.Fatalf("Commit after Abort = %v, want ErrClosed", err)
	}
}

func TestCommitSyncFailureKeepsTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "current.txt")
	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatalf("write old target: %v", err)
	}
	sentinel := errors.New("sync failed")

	writer, err := New(target, 0o600)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := writer.Write([]byte("new")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	writer.hooks.syncFile = func(*os.File) error { return sentinel }

	if err := writer.Commit(); !errors.Is(err, sentinel) {
		t.Fatalf("Commit = %v, want sync failure", err)
	}
	if got := readFile(t, target); got != "old" {
		t.Fatalf("target after sync failure = %q, want old", got)
	}
	requireMissing(t, target+".part")
}

func TestCommitRenameFailureKeepsTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "config.lua")
	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatalf("write old target: %v", err)
	}
	sentinel := errors.New("rename failed")

	writer, err := New(target, 0o600)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := writer.Write([]byte("new")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	writer.hooks.rename = func(string, string) error { return sentinel }

	if err := writer.Commit(); !errors.Is(err, sentinel) {
		t.Fatalf("Commit = %v, want rename failure", err)
	}
	if got := readFile(t, target); got != "old" {
		t.Fatalf("target after rename failure = %q, want old", got)
	}
	requireMissing(t, target+".part")
}

func TestCommitDirectorySyncFailureIsReported(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "current.txt")
	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatalf("write old target: %v", err)
	}
	sentinel := errors.New("directory sync failed")

	writer, err := New(target, 0o600)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := writer.Write([]byte("new")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	writer.hooks.syncDir = func(string) error { return sentinel }

	if err := writer.Commit(); !errors.Is(err, sentinel) {
		t.Fatalf("Commit = %v, want directory sync failure", err)
	}
	if got := readFile(t, target); got != "new" {
		t.Fatalf("target after rename = %q, want new", got)
	}
	requireMissing(t, target+".part")
}

func TestNewValidationAndDefaultMode(t *testing.T) {
	dir := t.TempDir()

	if _, err := New("", 0o600); err == nil {
		t.Fatal("New with empty target succeeded")
	}
	if _, err := New(filepath.Join(dir, "missing", "target"), 0o600); err == nil {
		t.Fatal("New with missing directory succeeded")
	}
	if _, err := New(filepath.Join(dir, "target"), os.ModeDir|0o700); err == nil {
		t.Fatal("New with non-permission mode succeeded")
	}

	target := filepath.Join(dir, "default")
	if err := WriteFile(target, []byte("x"), 0); err != nil {
		t.Fatalf("WriteFile with default mode: %v", err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat target: %v", err)
	}
	if perm := info.Mode().Perm(); perm != DefaultMode {
		t.Fatalf("default permissions = %o, want %o", perm, DefaultMode)
	}
}
