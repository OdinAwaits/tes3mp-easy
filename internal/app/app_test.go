package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/clientlog"
	"github.com/Saagilat/tes3mp-easy/internal/config"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/morrowind"
)

func testPaths(t *testing.T) config.Paths {
	t.Helper()
	root := t.TempDir()
	return config.Paths{
		ConfigDir:  filepath.Join(root, "config"),
		ConfigFile: filepath.Join(root, "config", "config.json"),
		DataDir:    filepath.Join(root, "data"),
		ServersDir: filepath.Join(root, "data", "servers"),
		CacheDir:   filepath.Join(root, "data", "cache"),
		LogsDir:    filepath.Join(root, "data", "logs"),
	}
}

func envMap(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func testServer() config.Server {
	return config.Server{
		Name:     "Friends",
		Address:  "203.0.113.10",
		Port:     25565,
		Password: "secret",
		PanelURL: "https://203.0.113.10",
	}
}

func TestOpenDefaultsAndLanguage(t *testing.T) {
	paths := testPaths(t)
	service, err := OpenWithEnv(paths, envMap(map[string]string{"LANG": "ru_RU.UTF-8"}))
	if err != nil {
		t.Fatalf("OpenWithEnv: %v", err)
	}
	t.Cleanup(func() { service.Close() })
	if !service.NeedsSetup() {
		t.Fatal("fresh service does not need setup")
	}
	if got := service.Config(); got.Version != config.CurrentVersion || len(got.Servers) != 0 {
		t.Fatalf("config = %+v", got)
	}
	if got := service.WorkDir(); got != paths.DataDir {
		t.Fatalf("WorkDir = %q, want %q", got, paths.DataDir)
	}
	if got, want := service.LogPath(), filepath.Join(paths.LogsDir,
		clientlog.FileName); got != want {
		t.Fatalf("LogPath = %q, want %q", got, want)
	}
	if got := service.Language(); got != i18n.English {
		t.Fatalf("Language = %q", got)
	}
	if got := service.Translator().T(i18n.KeyCommonPlay); got != "Play" {
		t.Fatalf("translation = %q", got)
	}
	if service.ConfigPath() != paths.ConfigFile {
		t.Fatalf("ConfigPath = %q", service.ConfigPath())
	}
}

func TestSaveServerAddsAndPersists(t *testing.T) {
	paths := testPaths(t)
	service, err := OpenWithEnv(paths, nil)
	if err != nil {
		t.Fatalf("OpenWithEnv: %v", err)
	}
	id, err := service.SaveServer("", testServer())
	if err != nil {
		t.Fatalf("SaveServer: %v", err)
	}
	if id != "203.0.113.10_25565" {
		t.Fatalf("id = %q", id)
	}
	cfg := service.Config()
	if len(cfg.Servers) != 1 || cfg.ActiveServer != id {
		t.Fatalf("config = %+v", cfg)
	}
	if cfg.Servers[0].Password != "secret" {
		t.Fatalf("password = %q", cfg.Servers[0].Password)
	}

	reloaded, err := config.Load(paths.ConfigFile)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(reloaded.Servers) != 1 || reloaded.Servers[0].ID != id {
		t.Fatalf("reloaded = %+v", reloaded)
	}
}

func TestSaveServerEditsProfile(t *testing.T) {
	paths := testPaths(t)
	service, err := OpenWithEnv(paths, nil)
	if err != nil {
		t.Fatalf("OpenWithEnv: %v", err)
	}
	id, err := service.SaveServer("", testServer())
	if err != nil {
		t.Fatalf("SaveServer add: %v", err)
	}
	profile := testServer()
	profile.Name = "Renamed"
	if newID, err := service.SaveServer(id, profile); err != nil || newID != id {
		t.Fatalf("SaveServer edit = %q, %v", newID, err)
	}
	cfg := service.Config()
	if len(cfg.Servers) != 1 || cfg.Servers[0].Name != "Renamed" {
		t.Fatalf("config = %+v", cfg)
	}
}

func TestSaveServerRenamesIDAndActiveProfile(t *testing.T) {
	paths := testPaths(t)
	service, err := OpenWithEnv(paths, nil)
	if err != nil {
		t.Fatalf("OpenWithEnv: %v", err)
	}
	oldID, err := service.SaveServer("", testServer())
	if err != nil {
		t.Fatalf("SaveServer add: %v", err)
	}
	profile := testServer()
	profile.Port = 25566
	newID, err := service.SaveServer(oldID, profile)
	if err != nil {
		t.Fatalf("SaveServer move: %v", err)
	}
	if newID == oldID {
		t.Fatal("id did not change with the port")
	}
	cfg := service.Config()
	if len(cfg.Servers) != 1 || cfg.Servers[0].ID != newID {
		t.Fatalf("servers = %+v", cfg.Servers)
	}
	if cfg.ActiveServer != newID {
		t.Fatalf("ActiveServer = %q, want %q", cfg.ActiveServer, newID)
	}
}

func TestClientLogRecordsErrorsAndTail(t *testing.T) {
	paths := testPaths(t)
	service, err := OpenWithEnv(paths, nil)
	if err != nil {
		t.Fatalf("OpenWithEnv: %v", err)
	}
	defer service.Close()
	service.LogInfof("setup step %d", 1)
	service.LogWarnf("optional voice archive skipped")
	service.LogError("check server", errors.New("connection refused"))
	lines, err := service.ReadLogTail(20)
	if err != nil {
		t.Fatalf("ReadLogTail: %v", err)
	}
	joined := strings.Join(lines, "\n")
	for _, want := range []string{
		"client started", "setup step 1", "optional voice archive skipped",
		"ERROR", "check server: connection refused", "code=T3E-APP-99",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("log does not contain %q:\n%s", want, joined)
		}
	}
	if info, err := os.Stat(service.LogPath()); err != nil ||
		info.Size() == 0 {
		t.Fatalf("log stat = %+v, %v", info, err)
	}
}

func TestNeedsSetupIgnoresProfilesUntilComplete(t *testing.T) {
	service, err := OpenWithEnv(testPaths(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if _, err := service.CheckMorrowind(""); !errors.Is(err,
		ErrMorrowindNotConfigured) {
		t.Fatalf("CheckMorrowind(empty) = %v", err)
	}
	if _, err := service.SaveServer("", testServer()); err != nil {
		t.Fatal(err)
	}
	if !service.NeedsSetup() {
		t.Fatal("NeedsSetup is false before setup is completed")
	}
	if err := service.CompleteSetup(); err != nil {
		t.Fatal(err)
	}
	if service.NeedsSetup() {
		t.Fatal("NeedsSetup is true after setup is completed")
	}
	root := makeMorrowind(t, "Morrowind.esm")
	if _, err := service.CheckMorrowind(root); !errors.Is(err,
		morrowind.ErrIncomplete) {
		t.Fatalf("CheckMorrowind(incomplete) = %v", err)
	}
}

func TestSaveServerRejectsInvalidAndDuplicate(t *testing.T) {
	paths := testPaths(t)
	service, err := OpenWithEnv(paths, nil)
	if err != nil {
		t.Fatalf("OpenWithEnv: %v", err)
	}
	if _, err := service.SaveServer("", testServer()); err != nil {
		t.Fatalf("SaveServer: %v", err)
	}
	if _, err := service.SaveServer("", testServer()); err == nil ||
		!strings.Contains(err.Error(), "already exists") {
		t.Fatalf("duplicate error = %v", err)
	}
	bad := testServer()
	bad.Address = "bad host"
	if _, err := service.SaveServer("", bad); err == nil {
		t.Fatal("invalid profile accepted")
	}
	if got := service.Config(); len(got.Servers) != 1 {
		t.Fatalf("failed saves changed config: %+v", got)
	}
	if _, err := service.SaveServer("missing", testServer()); err == nil ||
		!strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing profile error = %v", err)
	}
}
