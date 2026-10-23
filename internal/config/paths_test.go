package config

import (
	"path/filepath"
	"testing"
)

func envMap(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestPathsForLinux(t *testing.T) {
	paths, err := PathsFor("linux", envMap(map[string]string{
		"HOME":            "/home/player",
		"XDG_CONFIG_HOME": "/home/player/.cfg",
		"XDG_DATA_HOME":   "/home/player/.data",
	}))
	if err != nil {
		t.Fatalf("PathsFor: %v", err)
	}
	if want := filepath.Join("/home/player/.cfg", "tes3mp-easy"); paths.ConfigDir != want {
		t.Errorf("ConfigDir = %q, want %q", paths.ConfigDir, want)
	}
	if want := filepath.Join(paths.ConfigDir, "config.json"); paths.ConfigFile != want {
		t.Errorf("ConfigFile = %q, want %q", paths.ConfigFile, want)
	}
	if want := filepath.Join("/home/player/.data", "tes3mp-easy"); paths.DataDir != want {
		t.Errorf("DataDir = %q, want %q", paths.DataDir, want)
	}
	if want := filepath.Join(paths.DataDir, "servers"); paths.ServersDir != want {
		t.Errorf("ServersDir = %q, want %q", paths.ServersDir, want)
	}
	if want := filepath.Join(paths.DataDir, "cache"); paths.CacheDir != want {
		t.Errorf("CacheDir = %q, want %q", paths.CacheDir, want)
	}
	if want := filepath.Join(paths.DataDir, "logs"); paths.LogsDir != want {
		t.Errorf("LogsDir = %q, want %q", paths.LogsDir, want)
	}
}

func TestPathsForLinuxHomeFallback(t *testing.T) {
	paths, err := PathsFor("linux", envMap(map[string]string{"HOME": "/home/player"}))
	if err != nil {
		t.Fatalf("PathsFor: %v", err)
	}
	if want := filepath.Join("/home/player", ".config", "tes3mp-easy"); paths.ConfigDir != want {
		t.Errorf("ConfigDir = %q, want %q", paths.ConfigDir, want)
	}
	if want := filepath.Join("/home/player", ".local", "share", "tes3mp-easy"); paths.DataDir != want {
		t.Errorf("DataDir = %q, want %q", paths.DataDir, want)
	}
}

func TestPathsForWindows(t *testing.T) {
	paths, err := PathsFor("windows", envMap(map[string]string{
		"USERPROFILE":  `C:\Users\Player`,
		"APPDATA":      `C:\Users\Player\AppData\Roaming`,
		"LOCALAPPDATA": `C:\Users\Player\AppData\Local`,
		"HOME":         `C:\ignored`,
	}))
	if err != nil {
		t.Fatalf("PathsFor: %v", err)
	}
	if want := filepath.Join(`C:\Users\Player\AppData\Roaming`, "tes3mp-easy"); paths.ConfigDir != want {
		t.Errorf("ConfigDir = %q, want %q", paths.ConfigDir, want)
	}
	if want := filepath.Join(`C:\Users\Player\AppData\Local`, "tes3mp-easy"); paths.DataDir != want {
		t.Errorf("DataDir = %q, want %q", paths.DataDir, want)
	}
}

func TestPathsForWindowsFallsBackToUserProfile(t *testing.T) {
	paths, err := PathsFor("windows", envMap(map[string]string{
		"USERPROFILE": `C:\Users\Player`,
	}))
	if err != nil {
		t.Fatalf("PathsFor: %v", err)
	}
	if want := filepath.Join(`C:\Users\Player`, "AppData", "Roaming", "tes3mp-easy"); paths.ConfigDir != want {
		t.Errorf("ConfigDir = %q, want %q", paths.ConfigDir, want)
	}
	if want := filepath.Join(`C:\Users\Player`, "AppData", "Local", "tes3mp-easy"); paths.DataDir != want {
		t.Errorf("DataDir = %q, want %q", paths.DataDir, want)
	}
}

func TestPathsRequireHome(t *testing.T) {
	if _, err := PathsFor("linux", envMap(map[string]string{})); err == nil {
		t.Fatal("PathsFor without HOME returned nil")
	}
}

func TestPathsValidate(t *testing.T) {
	root := t.TempDir()
	paths := Paths{
		ConfigDir:  filepath.Join(root, "config"),
		ConfigFile: filepath.Join(root, "config", "config.json"),
		DataDir:    filepath.Join(root, "data"),
		ServersDir: filepath.Join(root, "data", "servers"),
		CacheDir:   filepath.Join(root, "data", "cache"),
		LogsDir:    filepath.Join(root, "data", "logs"),
	}
	if err := paths.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	paths.ConfigFile = "relative.json"
	if err := paths.Validate(); err == nil {
		t.Fatal("relative config path accepted")
	}
	if got := paths.ServerDir("example.com_25565"); got != filepath.Join(paths.ServersDir, "example.com_25565") {
		t.Fatalf("ServerDir = %q", got)
	}
}
