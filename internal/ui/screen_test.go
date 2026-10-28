package ui

import (
	"path/filepath"
	"runtime"
	"strings"
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

// TestScreensHaveNoFormatArtifacts catches a translated key that is formatted
// with more arguments than it has verbs: fmt then prints "%!(EXTRA …)" into the
// screen instead of the value.
func TestScreensHaveNoFormatArtifacts(t *testing.T) {
	s := testShell(t)
	defer s.window.Close()

	screens := []struct {
		name  string
		build func(*i18n.Translator) fyne.CanvasObject
	}{
		{name: "play", build: s.playScreen},
		{name: "installation", build: s.installScreen},
		{name: "about", build: s.aboutScreen},
		{name: "openmw-cs", build: s.editorScreen},
		{name: "diagnostics", build: s.diagnosticsScreen},
	}
	for _, language := range i18n.Supported() {
		tr := i18n.New(language)
		for _, screen := range screens {
			content := screen.build(tr)
			if content == nil {
				t.Fatalf("%s screen built nil content", screen.name)
			}
			texts := 0
			walkTexts(content, func(text string) {
				texts++
				if strings.Contains(text, "%!") {
					t.Errorf("%s screen (%s) shows a format artifact: %q",
						screen.name, language, text)
				}
			})
			if texts == 0 {
				t.Errorf("%s screen (%s) exposes no text to check",
					screen.name, language)
			}
		}
	}
}

// walkTexts visits label texts, rich text segments and button captions in an
// object tree.
func walkTexts(object fyne.CanvasObject, visit func(string)) {
	walkObjects(object, func(item fyne.CanvasObject) {
		switch typed := item.(type) {
		case *widget.Label:
			visit(typed.Text)
		case *widget.RichText:
			for _, segment := range typed.Segments {
				if text, ok := segment.(*widget.TextSegment); ok {
					visit(text.Text)
				}
			}
		case *widget.Button:
			visit(typed.Text)
		}
	})
}

// walkObjects visits every object in a tree. Widgets are walked through their
// renderers because containers hide children such as accordion details and
// form rows from struct fields.
func walkObjects(object fyne.CanvasObject, visit func(fyne.CanvasObject)) {
	if object == nil {
		return
	}
	visit(object)
	switch typed := object.(type) {
	case *fyne.Container:
		for _, child := range typed.Objects {
			walkObjects(child, visit)
		}
		return
	}
	item, ok := object.(fyne.Widget)
	if !ok {
		return
	}
	renderer := test.WidgetRenderer(item)
	if renderer == nil {
		return
	}
	for _, child := range renderer.Objects() {
		walkObjects(child, visit)
	}
}

// TestInstallationAdvancedKeepsOnlyPathAccordions guards against help-only rows
// such as the removed "Client profile" item: Advanced must list exactly the
// settings it edits.
func TestInstallationAdvancedKeepsOnlyPathAccordions(t *testing.T) {
	s := testShell(t)
	defer s.window.Close()
	tr := s.service.Translator()

	var accordions []*widget.Accordion
	walkObjects(s.installationAdvanced(tr), func(item fyne.CanvasObject) {
		if accordion, ok := item.(*widget.Accordion); ok {
			accordions = append(accordions, accordion)
		}
	})
	if len(accordions) != 1 {
		t.Fatalf("Advanced has %d accordions, want 1", len(accordions))
	}
	want := []string{tr.T(i18n.KeySettingsWorkDir)}
	if runtime.GOOS == "linux" {
		want = append(want, tr.T(i18n.KeySettingsProton))
	}
	items := accordions[0].Items
	if len(items) != len(want) {
		t.Fatalf("Advanced accordion has %d items, want %d", len(items), len(want))
	}
	for index, title := range want {
		if items[index].Title != title {
			t.Fatalf("Advanced item %d = %q, want %q", index, items[index].Title,
				title)
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
