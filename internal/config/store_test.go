package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLoadMissingReturnsDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Version != CurrentVersion || cfg.Servers == nil || len(cfg.Servers) != 0 {
		t.Fatalf("default config = %+v", cfg)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	server := validServer()
	server.Normalize()
	cfg := Default()
	cfg.Language = "ru"
	cfg.MorrowindPath = "/games/Morrowind"
	cfg.WorkDir = "/home/player/.local/share/tes3mp-easy"
	cfg.ProtonPath = "/steam/Proton"
	cfg.Localization = true
	cfg.SkipFonts = true
	cfg.Servers = []Server{server}
	cfg.ActiveServer = server.ID

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != FileMode {
		t.Fatalf("mode = %o, want %o", info.Mode().Perm(), FileMode)
	}
	if _, err := os.Stat(path + ".part"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary file left behind: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Fatal("config has no trailing newline")
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Language != "ru" || loaded.ActiveServer != server.ID ||
		loaded.ProtonPath != cfg.ProtonPath || loaded.Localization != cfg.Localization ||
		loaded.SkipFonts != cfg.SkipFonts ||
		loaded.MorrowindPath != cfg.MorrowindPath || loaded.WorkDir != cfg.WorkDir {
		t.Fatalf("loaded config = %+v", loaded)
	}
	if len(loaded.Servers) != 1 || loaded.Servers[0] != server {
		t.Fatalf("loaded servers = %+v", loaded.Servers)
	}
}

func TestSaveOverwritesPermissiveMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not use POSIX permission bits")
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := Save(path, Default()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if got := info.Mode().Perm(); got != FileMode {
		t.Fatalf("mode = %o, want %o", got, FileMode)
	}
}

func TestSaveRejectsInvalidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := Default()
	server := validServer()
	server.Normalize()
	cfg.Servers = []Server{server, server}
	if err := Save(path, cfg); err == nil {
		t.Fatal("Save accepted duplicate servers")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid config was written: %v", err)
	}
}

func TestLoadRejectsCorruptConfig(t *testing.T) {
	tests := map[string]string{
		"empty":     "",
		"invalid":   "{",
		"trailing":  `{"version":1,"servers":[]} {}`,
		"version":   `{"version":2,"servers":[]}`,
		"language":  `{"version":1,"language":"de","servers":[]}`,
		"server":    `{"version":1,"servers":[{"id":"x","name":"","address":"x","port":1,"panel_url":"https://x"}]}`,
		"duplicate": `{"version":1,"servers":[{"id":"x_1","name":"a","address":"x","port":1,"panel_url":"https://x"},{"id":"x_1","name":"b","address":"x","port":1,"panel_url":"https://x"}]}`,
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
			if _, err := Load(path); err == nil {
				t.Fatal("Load accepted corrupt config")
			}
		})
	}
}

func TestStoreUpdatePersistsAndRollsBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if store.Path() != path {
		t.Fatalf("Path = %q", store.Path())
	}

	server := validServer()
	server.Normalize()
	if err := store.Update(func(cfg *Config) error {
		cfg.Servers = append(cfg.Servers, server)
		cfg.ActiveServer = server.ID
		return nil
	}); err != nil {
		t.Fatalf("Update add: %v", err)
	}
	if got := store.Snapshot(); len(got.Servers) != 1 || got.ActiveServer != server.ID {
		t.Fatalf("snapshot after add = %+v", got)
	}

	before := store.Snapshot()
	if err := store.Update(func(cfg *Config) error {
		cfg.Servers[0].Port = 0
		return nil
	}); err == nil {
		t.Fatal("invalid update succeeded")
	}
	after := store.Snapshot()
	if after.Servers[0] != before.Servers[0] {
		t.Fatalf("failed update changed memory: %+v", after)
	}
	onDisk, err := Load(path)
	if err != nil {
		t.Fatalf("Load after failed update: %v", err)
	}
	if onDisk.Servers[0] != before.Servers[0] {
		t.Fatalf("failed update changed disk: %+v", onDisk)
	}

	sentinel := errors.New("stop")
	if err := store.Update(func(*Config) error { return sentinel }); !errors.Is(err, sentinel) {
		t.Fatalf("Update change error = %v", err)
	}
	if err := store.Update(nil); err == nil {
		t.Fatal("nil change accepted")
	}
}

func TestStoreSnapshotIsIndependent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	server := validServer()
	server.Normalize()
	if err := store.Update(func(cfg *Config) error {
		cfg.Servers = append(cfg.Servers, server)
		return nil
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	snapshot := store.Snapshot()
	snapshot.Servers[0].Name = "changed"
	snapshot.Servers = append(snapshot.Servers, validServer())
	if got := store.Snapshot(); len(got.Servers) != 1 || got.Servers[0].Name == "changed" {
		t.Fatalf("store was mutated through snapshot: %+v", got)
	}
}

func TestOpenStoreRejectsEmptyPath(t *testing.T) {
	if _, err := OpenStore(""); err == nil {
		t.Fatal("empty path accepted")
	}
}
