package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Usage: tes3mp-easy-pack") {
		t.Fatalf("help = %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestInvalidArguments(t *testing.T) {
	for _, args := range [][]string{
		{"--version="},
		{"--unknown"},
		{"positional"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 2 {
			t.Fatalf("%v: exit = %d, stderr = %q", args, code, stderr.String())
		}
		if !strings.Contains(stderr.String(), "tes3mp-easy-pack:") {
			t.Fatalf("%v: stderr = %q", args, stderr.String())
		}
	}
}

func TestPackRejectsInvalidVersionBeforeBuild(t *testing.T) {
	source := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := run([]string{"--version", "latest", "--source", source},
		&stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q",
			code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "invalid release version") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestPackRejectsMissingSource(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	var stdout, stderr bytes.Buffer
	code := run([]string{"--version", "1.2.3", "--source", missing},
		&stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q",
			code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "source directory") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if _, err := os.Stat(missing); err == nil {
		t.Fatal("pack created the missing source directory")
	}
}
