package launcher

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestOpenMWConfigDir(t *testing.T) {
	home := t.TempDir()
	windows, err := OpenMWConfigDir(Options{GOOS: "windows", HomeDir: home})
	if err != nil {
		t.Fatalf("windows: %v", err)
	}
	if windows != filepath.Join(home, "Documents", "My Games", "OpenMW") {
		t.Fatalf("windows config = %q", windows)
	}
	workDir := t.TempDir()
	linux, err := OpenMWConfigDir(Options{
		GOOS: "linux", WorkDir: workDir, HomeDir: home,
	})
	if err != nil {
		t.Fatalf("linux: %v", err)
	}
	if !strings.HasPrefix(linux, filepath.Join(workDir, "prefix")) {
		t.Fatalf("linux config = %q", linux)
	}
	override := filepath.Join(t.TempDir(), "OpenMW")
	overridden, err := OpenMWConfigDir(Options{
		GOOS: "linux", WorkDir: workDir, ConfigDir: override, HomeDir: home,
	})
	if err != nil || overridden != override {
		t.Fatalf("override = %q, %v", overridden, err)
	}
	if _, err := OpenMWConfigDir(Options{
		GOOS: "linux", WorkDir: "relative", HomeDir: home,
	}); err == nil {
		t.Fatal("relative work directory accepted")
	}
	if _, err := OpenMWConfigDir(Options{GOOS: "darwin", HomeDir: home}); err == nil {
		t.Fatal("unsupported OS accepted")
	}
}

func TestDiscoverProton(t *testing.T) {
	home := t.TempDir()
	writeExecutable(t, filepath.Join(home, ".steam", "steam", "steamapps",
		"common", "Proton 8.0", "proton"))
	newest := filepath.Join(home, ".local", "share", "Steam", "steamapps",
		"common", "Proton 9.0")
	writeExecutable(t, filepath.Join(newest, "proton"))
	found, err := DiscoverProton(home)
	if err != nil {
		t.Fatalf("DiscoverProton: %v", err)
	}
	if found != newest {
		t.Fatalf("DiscoverProton = %q, want %q", found, newest)
	}
	if _, err := DiscoverProton(t.TempDir()); err == nil ||
		!strings.Contains(err.Error(), "install Proton") {
		t.Fatalf("missing Proton error = %v", err)
	}
}

func TestProtonDir(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Tools", "GE-Proton9")
	writeExecutable(t, filepath.Join(dir, "proton"))
	found, err := protonDir("~/Tools/GE-Proton9", home)
	if err != nil {
		t.Fatalf("protonDir: %v", err)
	}
	if found != dir {
		t.Fatalf("protonDir = %q, want %q", found, dir)
	}
	if _, err := protonDir(filepath.Join(home, "missing"), home); err == nil {
		t.Fatal("missing Proton folder accepted")
	}
	noProton := filepath.Join(home, "no-proton")
	if err := os.MkdirAll(noProton, 0o700); err != nil {
		t.Fatalf("create folder: %v", err)
	}
	if _, err := protonDir(noProton, home); err == nil {
		t.Fatal("Proton folder without proton accepted")
	}
}

// TestProtonErrorsAreClassified keeps the UI contract: a Proton problem found
// while discovering or validating the folder must report T3E-GAME-02 with the
// "choose a folder" action instead of an unknown failure.
func TestProtonErrorsAreClassified(t *testing.T) {
	home := t.TempDir()
	if _, err := FindProton(home); !errors.Is(err, ErrProton) {
		t.Fatalf("FindProton error = %v, want ErrProton", err)
	}
	noProton := filepath.Join(home, "Proton 9.0")
	if err := os.MkdirAll(noProton, 0o700); err != nil {
		t.Fatalf("create folder: %v", err)
	}
	if _, err := ProtonFolder(noProton, home); !errors.Is(err, ErrProton) {
		t.Fatalf("ProtonFolder error = %v, want ErrProton", err)
	}
	valid := filepath.Join(home, "Tools", "GE-Proton9")
	writeExecutable(t, filepath.Join(valid, "proton"))
	found, err := ProtonFolder(valid, home)
	if err != nil || found != valid {
		t.Fatalf("ProtonFolder(%q) = %q, %v; want %q", valid, found, err, valid)
	}
}
