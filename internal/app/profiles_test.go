package app

import (
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/config"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
)

func TestDeleteServerClearsActive(t *testing.T) {
	paths := testPaths(t)
	service, err := OpenWithEnv(paths, nil)
	if err != nil {
		t.Fatalf("OpenWithEnv: %v", err)
	}
	id, err := service.SaveServer("", testServer())
	if err != nil {
		t.Fatalf("SaveServer: %v", err)
	}
	if err := service.DeleteServer(id); err != nil {
		t.Fatalf("DeleteServer: %v", err)
	}
	cfg := service.Config()
	if len(cfg.Servers) != 0 || cfg.ActiveServer != "" {
		t.Fatalf("config after delete = %+v", cfg)
	}
	if err := service.DeleteServer(id); err == nil {
		t.Fatal("deleting a missing server succeeded")
	}
}

func TestSetActiveServer(t *testing.T) {
	paths := testPaths(t)
	service, err := OpenWithEnv(paths, nil)
	if err != nil {
		t.Fatalf("OpenWithEnv: %v", err)
	}
	first := testServer()
	if _, err := service.SaveServer("", first); err != nil {
		t.Fatalf("SaveServer first: %v", err)
	}
	second := testServer()
	second.Port = 25566
	secondID, err := service.SaveServer("", second)
	if err != nil {
		t.Fatalf("SaveServer second: %v", err)
	}
	if err := service.SetActiveServer(secondID); err != nil {
		t.Fatalf("SetActiveServer: %v", err)
	}
	if got := service.Config().ActiveServer; got != secondID {
		t.Fatalf("ActiveServer = %q, want %q", got, secondID)
	}
	if err := service.SetActiveServer("missing"); err == nil {
		t.Fatal("missing active server accepted")
	}
	if got := service.Config().ActiveServer; got != secondID {
		t.Fatalf("failed update changed active server to %q", got)
	}
}

func TestUpdateSettings(t *testing.T) {
	paths := testPaths(t)
	service, err := OpenWithEnv(paths, envMap(map[string]string{"LANG": "en_US.UTF-8"}))
	if err != nil {
		t.Fatalf("OpenWithEnv: %v", err)
	}
	if err := service.UpdateSettings(Settings{
		Language:      "ru",
		MorrowindPath: " /games/Morrowind ",
		WorkDir:       " /work ",
		ProtonPath:    " /proton ",
	}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	cfg := service.Config()
	if cfg.Language != "ru" || cfg.MorrowindPath != "/games/Morrowind" ||
		cfg.WorkDir != "/work" || cfg.ProtonPath != "/proton" {
		t.Fatalf("config = %+v", cfg)
	}
	if got := service.WorkDir(); got != "/work" {
		t.Fatalf("WorkDir = %q", got)
	}
	if got := service.Language(); got != i18n.Russian {
		t.Fatalf("Language = %q", got)
	}
	if err := service.UpdateSettings(Settings{Language: "de"}); err == nil {
		t.Fatal("unsupported language accepted")
	}
	if got := service.Config().Language; got != "ru" {
		t.Fatalf("failed settings update changed language to %q", got)
	}
}

func TestOpenRejectsEmptyConfigPath(t *testing.T) {
	if _, err := OpenWithEnv(config.Paths{}, nil); err == nil {
		t.Fatal("empty config path accepted")
	}
}
