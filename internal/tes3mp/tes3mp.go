// Package tes3mp installs the fixed TES3MP Win64 client used by the launcher.
// The archive is always verified against the SHA256 embedded in the build.
package tes3mp

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// Version is the TES3MP client release expected by the launcher.
	Version = "0.8.1"

	// URL is the official Win64 release archive.
	URL = "https://github.com/TES3MP/TES3MP/releases/download/tes3mp-0.8.1/tes3mp.Win64.release.0.8.1.zip"

	// SHA256 is the checksum of URL.
	SHA256 = "72d35d574689e698196855c80e27bf79a86a591c2a7d42dddd03de0c597e2bad"

	// ArchiveName is the cached archive filename.
	ArchiveName = "tes3mp-0.8.1-win64.zip"

	// Executable is the main TES3MP binary inside the installation.
	Executable = "tes3mp.exe"

	// VersionFile records the installed release inside tes3mp/.
	VersionFile = ".tes3mp-version"

	// MaxDownloadSize bounds the compressed archive accepted from the network.
	MaxDownloadSize int64 = 256 << 20
)

// State describes a possibly installed TES3MP directory.
type State struct {
	Dir        string
	Executable string
	Version    string
	Installed  bool
	Current    bool
}

// Dir returns the TES3MP directory inside a client work directory.
func Dir(workDir string) string {
	return filepath.Join(workDir, "tes3mp")
}

// Inspect checks dir without modifying it. A missing directory is a normal
// not-installed state; an unreadable one is an error.
func Inspect(dir string) (State, error) {
	return inspectVersion(dir, Version)
}

func inspectVersion(dir, expected string) (State, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return State{}, errors.New("TES3MP directory is empty")
	}
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return State{}, fmt.Errorf("TES3MP directory %q: %w", dir, err)
	}
	state := State{Dir: absolute}
	info, err := os.Lstat(absolute)
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("inspect TES3MP directory %q: %w", absolute, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return State{}, fmt.Errorf("TES3MP path %q is not a directory", absolute)
	}

	executable := filepath.Join(absolute, Executable)
	info, err = os.Stat(executable)
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("inspect %s: %w", executable, err)
	}
	if !info.Mode().IsRegular() {
		return State{}, fmt.Errorf("%s is not a regular file", executable)
	}
	state.Executable = executable
	state.Installed = true

	versionPath := filepath.Join(absolute, VersionFile)
	data, err := os.ReadFile(versionPath)
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("read %s: %w", versionPath, err)
	}
	state.Version = strings.TrimSpace(string(data))
	state.Current = state.Version == expected
	return state, nil
}
