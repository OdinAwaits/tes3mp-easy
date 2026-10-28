package ui

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/Saagilat/tes3mp-easy/internal/app"
	"github.com/Saagilat/tes3mp-easy/internal/config"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/ui/style"
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

// TestHeaderHasLanguageAndThemeSwitches pins the header contract: one language
// toggle and one theme list instead of three theme buttons and no language.
func TestHeaderHasLanguageAndThemeSwitches(t *testing.T) {
	s := testShell(t)
	defer s.window.Close()
	tr := s.service.Translator()

	var buttons []*widget.Button
	var selects []*widget.Select
	walkObjects(s.headerActions(tr), func(item fyne.CanvasObject) {
		switch typed := item.(type) {
		case *widget.Button:
			buttons = append(buttons, typed)
		case *widget.Select:
			selects = append(selects, typed)
		}
	})
	if len(selects) != 1 {
		t.Fatalf("header has %d lists, want 1 theme list", len(selects))
	}
	if got, want := strings.Join(selects[0].Options, "|"),
		strings.Join(themeModeLabels(tr), "|"); got != want {
		t.Fatalf("theme list options = %q, want %q", got, want)
	}
	if want := themeModeLabel(tr, style.ParseMode(s.service.Config().Theme)); selects[0].Selected != want {
		t.Fatalf("theme list selection = %q, want %q", selects[0].Selected, want)
	}
	target, ok := nextLanguage(s.service.Language())
	if !ok {
		t.Fatal("the test configuration has no language target")
	}
	if len(buttons) != 1 {
		t.Fatalf("header has %d buttons, want 1 language toggle", len(buttons))
	}
	if want := i18n.DisplayName(target); buttons[0].Text != want {
		t.Fatalf("language toggle = %q, want %q", buttons[0].Text, want)
	}
}

// TestHeaderFitsSupportedWindow keeps the header on one line at the smallest
// window we ship: the brand and the two fixed-width switches must fit 900 px in
// both languages.
func TestHeaderFitsSupportedWindow(t *testing.T) {
	s := testShell(t)
	defer s.window.Close()
	const narrowest float32 = 900
	for _, language := range i18n.Supported() {
		tr := i18n.New(language)
		header := container.NewBorder(nil, nil, s.brand(tr), s.headerActions(tr), nil)
		if got := header.MinSize().Width; got > narrowest {
			t.Fatalf("%s header needs %.0f px, want at most %.0f",
				language, got, narrowest)
		}
	}
}

// TestAboutHasNoLanguageSelector keeps settings in the header: About only
// reports versions and links now.
func TestAboutHasNoLanguageSelector(t *testing.T) {
	s := testShell(t)
	defer s.window.Close()

	selects := 0
	walkObjects(s.aboutScreen(s.service.Translator()), func(item fyne.CanvasObject) {
		if _, ok := item.(*widget.Select); ok {
			selects++
		}
	})
	if selects != 0 {
		t.Fatalf("About has %d selectors, want none: settings live in the header",
			selects)
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
