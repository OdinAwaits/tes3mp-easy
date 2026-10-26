package ui

import (
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/Saagilat/tes3mp-easy/internal/app"
	"github.com/Saagilat/tes3mp-easy/internal/config"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
)

// testShell builds a shell on a throw-away configuration so screens can be
// built by the Fyne test driver without touching the real client.
func testShell(t *testing.T) *shell {
	t.Helper()
	test.NewApp()
	root := t.TempDir()
	service, err := app.OpenWithEnv(config.Paths{
		ConfigDir:  filepath.Join(root, "config"),
		ConfigFile: filepath.Join(root, "config", "config.json"),
		DataDir:    filepath.Join(root, "data"),
		ServersDir: filepath.Join(root, "data", "servers"),
		CacheDir:   filepath.Join(root, "data", "cache"),
		LogsDir:    filepath.Join(root, "data", "logs"),
	}, func(string) string { return "" })
	if err != nil {
		t.Fatalf("OpenWithEnv: %v", err)
	}
	return &shell{
		service:  service,
		window:   test.NewWindow(nil),
		statuses: map[string]serverStatus{},
		checking: map[string]bool{},
		screen:   screenPlay,
	}
}

func TestScreensBuild(t *testing.T) {
	s := testShell(t)
	defer s.window.Close()
	tr := s.service.Translator()

	for _, screen := range []struct {
		name  string
		build func() fyne.CanvasObject
	}{
		{name: "play", build: func() fyne.CanvasObject {
			return s.playScreen(tr)
		}},
		{name: "installation", build: func() fyne.CanvasObject {
			return s.installScreen(tr)
		}},
		{name: "openmw-cs", build: func() fyne.CanvasObject {
			return s.editorScreen(tr)
		}},
	} {
		if content := screen.build(); content == nil {
			t.Fatalf("%s screen built nil content", screen.name)
		}
	}
}

func TestMissingLanguagePackIsNeutral(t *testing.T) {
	s := testShell(t)
	defer s.window.Close()

	tr := s.service.Translator()
	status := s.localizationStatus(tr)
	if status.importance != widget.LowImportance {
		t.Fatalf("missing language pack importance = %v, want low",
			status.importance)
	}
	if want := tr.T(i18n.KeyInstallLanguageOriginal); status.summary != want {
		t.Fatalf("missing language pack summary = %q, want %q",
			status.summary, want)
	}
	if s.localizationInstalled() {
		t.Fatal("a fresh work directory reports an installed language pack")
	}
}

func TestShowScreenFallsBackToPlay(t *testing.T) {
	s := testShell(t)
	defer s.window.Close()

	s.showScreen(screenID("unknown"))
	if s.screen != screenPlay {
		t.Fatalf("unknown screen selected %q", s.screen)
	}
	s.showScreen(screenCS)
	if s.screen != screenCS {
		t.Fatalf("showScreen(%q) left screen %q", screenCS, s.screen)
	}
}
