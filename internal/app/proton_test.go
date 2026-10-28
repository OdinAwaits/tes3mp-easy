package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/launcher"
)

// protonHome builds a fake home with one Steam library, the layout
// DiscoverProton searches on a real Linux desktop.
func protonHome(t *testing.T) (string, string) {
	t.Helper()
	home := t.TempDir()
	proton := filepath.Join(home, ".steam", "steam", "steamapps", "common",
		"Proton 9.0")
	writeProtonFolder(t, proton)
	return home, proton
}

func writeProtonFolder(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("create %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "proton"), []byte("#!/bin/sh\n"),
		0o755); err != nil {
		t.Fatalf("write proton: %v", err)
	}
}

func openProtonService(t *testing.T, home string) *Service {
	t.Helper()
	service, err := OpenWithEnv(testPaths(t), envMap(nil))
	if err != nil {
		t.Fatalf("OpenWithEnv: %v", err)
	}
	t.Cleanup(func() { service.Close() })
	service.goos = "linux"
	service.homeDir = home
	return service
}

func TestProtonStatusUsesDiscoveredFolder(t *testing.T) {
	home, proton := protonHome(t)
	status := openProtonService(t, home).ProtonStatus()
	if status.Source != ProtonDiscovered || status.Path != proton ||
		status.Err != nil {
		t.Fatalf("ProtonStatus() = %+v, want the discovered %q", status, proton)
	}
}

func TestProtonStatusPrefersConfiguredFolder(t *testing.T) {
	home, _ := protonHome(t)
	manual := filepath.Join(home, "Tools", "GE-Proton9")
	writeProtonFolder(t, manual)
	service := openProtonService(t, home)
	if err := service.UpdateSettings(Settings{ProtonPath: manual}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	status := service.ProtonStatus()
	if status.Source != ProtonFromConfig || status.Path != manual ||
		status.Err != nil {
		t.Fatalf("ProtonStatus() = %+v, want the configured %q", status, manual)
	}
}

func TestProtonStatusReportsUnusableConfiguredFolder(t *testing.T) {
	home, _ := protonHome(t)
	broken := filepath.Join(home, "Proton 8.0")
	if err := os.MkdirAll(broken, 0o700); err != nil {
		t.Fatalf("create folder: %v", err)
	}
	service := openProtonService(t, home)
	if err := service.UpdateSettings(Settings{ProtonPath: broken}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	status := service.ProtonStatus()
	if status.Source != ProtonFromConfig || status.Path != broken ||
		!errors.Is(status.Err, launcher.ErrProton) {
		t.Fatalf("ProtonStatus() = %+v, want the rejected %q", status, broken)
	}
}

func TestProtonStatusReportsMissingProton(t *testing.T) {
	status := openProtonService(t, t.TempDir()).ProtonStatus()
	if status.Source != ProtonMissing || status.Path != "" ||
		!errors.Is(status.Err, launcher.ErrProton) {
		t.Fatalf("ProtonStatus() = %+v, want a missing Proton", status)
	}
}

func TestProtonStatusIgnoresOtherPlatforms(t *testing.T) {
	home, _ := protonHome(t)
	service := openProtonService(t, home)
	service.goos = "windows"
	if status := service.ProtonStatus(); status.Source != ProtonMissing ||
		status.Path != "" {
		t.Fatalf("Windows ProtonStatus() = %+v, want empty", status)
	}
}

func TestProtonStatusHandlesNilService(t *testing.T) {
	var service *Service
	if status := service.ProtonStatus(); status.Source != ProtonMissing {
		t.Fatalf("nil service status = %+v, want missing", status)
	}
}
