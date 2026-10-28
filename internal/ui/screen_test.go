package ui

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
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

// TestInstallationAdvancedEditsPathsWithPicker keeps folder editing on the
// native picker: the rows must not contain text fields, and every row exposes
// both the picker button and the typed-entry fallback.
func TestInstallationAdvancedEditsPathsWithPicker(t *testing.T) {
	s := testShell(t)
	defer s.window.Close()
	tr := s.service.Translator()

	entries := 0
	buttons := 0
	walkObjects(s.installationAdvanced(tr), func(item fyne.CanvasObject) {
		switch item.(type) {
		case *widget.Entry:
			entries++
		case *widget.Button:
			buttons++
		}
	})
	if entries != 0 {
		t.Fatalf("Advanced shows %d text fields, want none: paths are picked", entries)
	}
	rows := 1 + boolToInt(runtime.GOOS == "linux")
	if want := 2 * rows; buttons < want {
		t.Fatalf("Advanced has %d buttons, want at least %d (picker and typed "+
			"entry per row)", buttons, want)
	}
}

// TestPathButtonsOfferPickerAndTypedEntry keeps the fallback visible next to the
// native picker, so a desktop without a portal can still set a folder.
func TestPathButtonsOfferPickerAndTypedEntry(t *testing.T) {
	s := testShell(t)
	defer s.window.Close()
	tr := s.service.Translator()

	var captions []string
	walkObjects(s.pathButtons(tr, "Title", tr.T(i18n.KeyInstallChange), t.TempDir(),
		func(string) {}), func(item fyne.CanvasObject) {
		if button, ok := item.(*widget.Button); ok {
			captions = append(captions, button.Text)
		}
	})
	want := []string{tr.T(i18n.KeyInstallChange), tr.T(i18n.KeyCommonEnterPath)}
	if len(captions) != len(want) {
		t.Fatalf("path buttons = %q, want %q", captions, want)
	}
	for index, caption := range want {
		if captions[index] != caption {
			t.Fatalf("path button %d = %q, want %q", index, captions[index], caption)
		}
	}
}

// TestPathPickerOpensFolderDialog proves the primary action is the native folder
// dialog: after a click the top overlay is a folder list, not a text prompt.
func TestPathPickerOpensFolderDialog(t *testing.T) {
	s := testShell(t)
	defer s.window.Close()
	tr := s.service.Translator()

	picker := pathButton(t, s, tr, tr.T(i18n.KeyInstallChange))
	test.Tap(picker)
	overlay := s.window.Canvas().Overlays().Top()
	if overlay == nil {
		t.Fatal("the picker button opened no dialog")
	}
	lists, entries := 0, 0
	walkObjects(overlay, func(item fyne.CanvasObject) {
		switch item.(type) {
		case *widget.List:
			lists++
		case *widget.Entry:
			entries++
		}
	})
	if lists == 0 {
		t.Fatal("the folder dialog shows no folder list")
	}
	if entries != 0 {
		t.Fatalf("the folder dialog shows %d text fields, want none", entries)
	}
}

// TestTypedPathFallbackOpensEntryDialog keeps the second action working: on a
// desktop without a portal the user must still be able to type the folder.
func TestTypedPathFallbackOpensEntryDialog(t *testing.T) {
	s := testShell(t)
	defer s.window.Close()
	tr := s.service.Translator()

	manual := pathButton(t, s, tr, tr.T(i18n.KeyCommonEnterPath))
	test.Tap(manual)
	overlay := s.window.Canvas().Overlays().Top()
	if overlay == nil {
		t.Fatal("the typed-entry button opened no dialog")
	}
	entries := 0
	walkObjects(overlay, func(item fyne.CanvasObject) {
		if _, ok := item.(*widget.Entry); ok {
			entries++
		}
	})
	if entries != 1 {
		t.Fatalf("the typed-entry dialog has %d fields, want 1", entries)
	}
}

// pathButton builds one path row and returns the button with the given caption.
func pathButton(t *testing.T, s *shell, tr *i18n.Translator,
	caption string,
) *widget.Button {
	t.Helper()
	button := (*widget.Button)(nil)
	walkObjects(s.pathButtons(tr, "Folder", tr.T(i18n.KeyInstallChange),
		t.TempDir(), func(string) {}), func(item fyne.CanvasObject) {
		if typed, ok := item.(*widget.Button); ok && typed.Text == caption {
			button = typed
		}
	})
	if button == nil {
		t.Fatalf("the path row has no %q button", caption)
	}
	return button
}

// TestFolderURI guards the picker's start location: only an existing directory
// positions the dialog, anything else keeps the platform default.
func TestFolderURI(t *testing.T) {
	dir := t.TempDir()
	if folderURI(dir) == nil {
		t.Fatalf("folderURI(%q) = nil, want the directory", dir)
	}
	if got := folderURI(filepath.Join(dir, "missing")); got != nil {
		t.Fatalf("folderURI(missing) = %q, want nil", got)
	}
	if got := folderURI("   "); got != nil {
		t.Fatalf("folderURI(empty) = %q, want nil", got)
	}
}

// TestProtonHintNamesTheSource keeps the Proton row honest: a discovered folder
// is labelled as such, a configured one as manual, and a missing one asks for a
// folder instead of printing a dash.
func TestProtonHintNamesTheSource(t *testing.T) {
	tr := i18n.New(i18n.English)
	tests := []struct {
		name   string
		status app.ProtonStatus
		want   string
	}{
		{name: "discovered", status: app.ProtonStatus{Path: "/steam/Proton 9.0",
			Source: app.ProtonDiscovered}, want: tr.T(i18n.KeyInstallProtonFound)},
		{name: "manual", status: app.ProtonStatus{Path: "/opt/GE-Proton9",
			Source: app.ProtonFromConfig}, want: tr.T(i18n.KeyInstallProtonManual)},
		{name: "missing", status: app.ProtonStatus{Source: app.ProtonMissing,
			Err: errors.New("Proton was not found")},
			want: tr.T(i18n.KeyInstallProtonMissing)},
		{name: "unusable manual path", status: app.ProtonStatus{
			Path: "/opt/empty", Source: app.ProtonFromConfig,
			Err: errors.New("no proton binary")},
			want: tr.T(i18n.KeyInstallProtonMissing)},
	}
	for _, test := range tests {
		if got := protonHint(tr, test.status); got != test.want {
			t.Errorf("%s hint = %q, want %q", test.name, got, test.want)
		}
	}
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
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

// TestInstallationMorrowindRowOffersFind keeps the search where the user looks
// for a problem: the row that says "no folder" owns the action that finds one.
func TestInstallationMorrowindRowOffersFind(t *testing.T) {
	s := testShell(t)
	defer s.window.Close()
	tr := s.service.Translator()

	buttons := []string{}
	walkObjects(s.installScreen(tr), func(item fyne.CanvasObject) {
		if button, ok := item.(*widget.Button); ok {
			buttons = append(buttons, button.Text)
		}
	})
	for _, want := range []string{tr.T(i18n.KeySetupFind),
		tr.T(i18n.KeyInstallChangeFolder)} {
		found := false
		for _, text := range buttons {
			if text == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("the Morrowind row has no %q button (has %v)", want, buttons)
		}
	}
}

// TestPlayWithoutMorrowindSendsToInstallation replaces the old error dialog:
// Play without the game opens the checklist and starts nothing.
func TestPlayWithoutMorrowindSendsToInstallation(t *testing.T) {
	s := testShell(t)
	defer s.window.Close()

	s.playServer(config.Server{ID: "example.com_25565", Address: "example.com",
		Port: 25565})

	if s.screen != screenInstall {
		t.Fatalf("Play without Morrowind opened %q, want %q", s.screen, screenInstall)
	}
	if len(s.playing) != 0 {
		t.Fatalf("Play without Morrowind started work for %v", s.playing)
	}
}

// TestScreenAccentBudget keeps one accent action per zone: the screen action
// (Run wizard, Copy report) and one Play per server card. Row actions are
// secondary, so a checklist does not look like ten primary buttons.
func TestScreenAccentBudget(t *testing.T) {
	s := testShell(t)
	defer s.window.Close()
	tr := s.service.Translator()
	for _, profile := range []config.Server{
		{Address: "203.0.113.10", Port: 25565, Name: "One"},
		{Address: "play.example.org", Port: 25570, Name: "Two"},
	} {
		if _, err := s.service.SaveServer("", profile); err != nil {
			t.Fatalf("SaveServer: %v", err)
		}
	}

	for _, screen := range []struct {
		name  string
		want  int
		build func() fyne.CanvasObject
	}{
		{name: "installation", want: 1, build: func() fyne.CanvasObject {
			return s.installScreen(tr)
		}},
		{name: "play", want: 2, build: func() fyne.CanvasObject {
			return s.playScreen(tr)
		}},
		{name: "diagnostics", want: 1, build: func() fyne.CanvasObject {
			return s.diagnosticsScreen(tr)
		}},
		{name: "about", want: 0, build: func() fyne.CanvasObject {
			return s.aboutScreen(tr)
		}},
	} {
		accent := 0
		walkObjects(screen.build(), func(item fyne.CanvasObject) {
			button, ok := item.(*widget.Button)
			if ok && button.Importance == widget.HighImportance {
				accent++
			}
		})
		if accent != screen.want {
			t.Errorf("%s has %d accent buttons, want %d", screen.name, accent,
				screen.want)
		}
	}
}

// TestInstallationIsOneCard keeps the checklist as one surface with separators:
// five separate cards pushed the screen past one window height and read as five
// unrelated blocks.
func TestInstallationIsOneCard(t *testing.T) {
	s := testShell(t)
	defer s.window.Close()

	cards, separators := 0, 0
	walkObjects(s.installScreen(s.service.Translator()), func(item fyne.CanvasObject) {
		if rect, ok := item.(*canvas.Rectangle); ok && rect.CornerRadius >= 12 {
			cards++
		}
		if _, ok := item.(*widget.Separator); ok {
			separators++
		}
	})
	if cards != 1 {
		t.Fatalf("installation has %d cards, want 1", cards)
	}
	// Five rows need four separators; the Advanced accordion adds its own.
	if separators < 4 {
		t.Fatalf("installation has %d separators, want at least 4 between rows",
			separators)
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
