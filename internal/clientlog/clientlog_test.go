package clientlog

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestOpenWritesPrivateLog(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	logger, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	logger.Infof("hello %d", 1)
	logger.Warnf("warning")
	logger.Errorf("line one\nline two")
	path := logger.Path()
	if path != filepath.Join(dir, FileName) {
		t.Fatalf("Path = %q", path)
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != FileMode {
			t.Fatalf("mode = %o, want %o", got, FileMode)
		}
		info, err = os.Stat(dir)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != DirMode {
			t.Fatalf("dir mode = %o, want %o", got, DirMode)
		}
	}
	lines, err := Tail(path, 10)
	if err != nil {
		t.Fatalf("Tail: %v", err)
	}
	if len(lines) != 4 {
		t.Fatalf("lines = %#v", lines)
	}
	for _, want := range []string{
		"INFO", "hello 1", "WARN", "warning", "ERROR", "line one\\nline two",
	} {
		joined := strings.Join(lines, "\n")
		if !strings.Contains(joined, want) {
			t.Fatalf("tail %q does not contain %q", joined, want)
		}
	}
}

func TestTailKeepsLastLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\n"), FileMode); err != nil {
		t.Fatal(err)
	}
	lines, err := Tail(path, 2)
	if err != nil {
		t.Fatalf("Tail: %v", err)
	}
	if strings.Join(lines, ",") != "two,three" {
		t.Fatalf("lines = %#v", lines)
	}
	if lines, err := Tail(filepath.Join(t.TempDir(), "missing.log"), 2); err != nil ||
		len(lines) != 0 {
		t.Fatalf("missing tail = %#v, %v", lines, err)
	}
	if _, err := Tail(path, 0); err == nil {
		t.Fatal("zero line limit accepted")
	}
}

func TestRotationKeepsPreviousLog(t *testing.T) {
	dir := t.TempDir()
	logger, err := open(dir, 120)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	for index := 0; index < 10; index++ {
		logger.Infof("entry %d with enough text to rotate the file", index)
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	rotated := filepath.Join(dir, RotatedName)
	if _, err := os.Stat(rotated); err != nil {
		t.Fatalf("rotated log is missing: %v", err)
	}
	lines, err := Tail(rotated, 100)
	if err != nil {
		t.Fatalf("Tail rotated: %v", err)
	}
	if len(lines) == 0 {
		t.Fatal("rotated log is empty")
	}
	lines, err = Tail(filepath.Join(dir, FileName), 100)
	if err != nil {
		t.Fatalf("Tail current: %v", err)
	}
	if len(lines) == 0 || len(lines) >= 10 {
		t.Fatalf("current log lines = %d", len(lines))
	}
}

func TestOpenRejectsInvalidDirectory(t *testing.T) {
	if _, err := Open(""); err == nil {
		t.Fatal("empty directory accepted")
	}
	if _, err := Open("relative"); err == nil {
		t.Fatal("relative directory accepted")
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("x"), FileMode); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(file); err == nil {
		t.Fatal("regular file accepted as log directory")
	}
}
