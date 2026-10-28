// Package ui renders the Fyne client shell.
//
// The package only marshals values from internal/app and forwards user actions
// back to it. Profile validation and persistence live outside Fyne so they can
// be tested without a display.
package ui

import (
	"fmt"
	"sync"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Saagilat/tes3mp-easy/internal/app"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/ui/style"
)

const appID = "io.github.saagilat.tes3mp-easy"

// Run starts the desktop shell and blocks until the last window is closed.
func Run(service *app.Service) error {
	if service == nil {
		return fmt.Errorf("ui: app service is nil")
	}
	fyneApp := fyneapp.NewWithID(appID)
	fyneApp.Settings().SetTheme(style.ThemeFor(service.Config().Theme))
	window := fyneApp.NewWindow(service.Translator().T(i18n.KeyAppTitle))
	window.Resize(fyne.NewSize(1000, 700))
	shell := &shell{
		service:  service,
		window:   window,
		statuses: make(map[string]serverStatus),
		checking: make(map[string]bool),
	}
	shell.render()
	window.CenterOnScreen()
	window.Show()
	if service.NeedsSetup() {
		shell.showSetup()
	}
	fyneApp.Run()
	return nil
}

// screenID names one section of the navigation rail.
type screenID string

const (
	screenPlay        screenID = "play"
	screenInstall     screenID = "installation"
	screenCS          screenID = "openmw-cs"
	screenDiagnostics screenID = "diagnostics"
	screenAbout       screenID = "about"
)

type shell struct {
	service *app.Service
	window  fyne.Window

	statusMu sync.Mutex
	statuses map[string]serverStatus
	checking map[string]bool

	// playMu guards playing: a profile can only download mods and launch once at
	// a time, so a repeated click cannot start a duplicate TES3MP instance.
	playMu  sync.Mutex
	playing map[string]bool

	// screen is the section shown in the body; the rail switches it.
	screen screenID
	// body holds the current screen; render replaces its only child.
	body *fyne.Container
	// rail maps a section to its button so the active state can be synced.
	rail map[screenID]*widget.Button
	// fonts tracks the background download started after the wizard.
	fonts fontDownload
}

func (s *shell) render() {
	tr := s.service.Translator()
	cfg := s.service.Config()
	s.window.SetTitle(tr.T(i18n.KeyAppTitle))
	s.applyTheme(cfg.Theme)

	s.body = container.NewStack()
	header := container.NewBorder(nil, nil, s.brand(tr), s.headerActions(tr), nil)
	// The footer answers "which build am I running?" without opening About:
	// configuration path on the left, client and TES3MP versions on the right.
	footer := container.NewBorder(nil, nil,
		footerLabel(tr.Tf(i18n.KeyFooterPath, s.service.ConfigPath()),
			fyne.TextAlignLeading),
		footerLabel(footerVersion(tr), fyne.TextAlignTrailing),
		nil,
	)
	content := container.NewBorder(
		container.NewPadded(header),
		container.NewPadded(footer),
		container.NewPadded(s.railContent(tr)),
		nil,
		s.body,
	)
	s.window.SetContent(content)
	s.showScreen(s.screen)
}

// footerLabel is one monospace line of the footer. Both footer fields stay
// low-importance so the path and the versions never compete with the content.
func footerLabel(text string, alignment fyne.TextAlign) *widget.Label {
	label := widget.NewLabelWithStyle(text, alignment, fyne.TextStyle{Monospace: true})
	label.Importance = widget.LowImportance
	return label
}

// footerVersion renders the client and bundled TES3MP releases.
func footerVersion(tr *i18n.Translator) string {
	return tr.Tf(i18n.KeyFooterVersion, app.Version, app.TES3MPVersion)
}

// showScreen swaps the body and keeps the rail in sync with the selection.
func (s *shell) showScreen(id screenID) {
	switch id {
	case screenInstall, screenCS, screenDiagnostics, screenAbout:
	default:
		id = screenPlay
	}
	s.screen = id
	tr := s.service.Translator()
	var screen fyne.CanvasObject
	switch id {
	case screenInstall:
		screen = s.installScreen(tr)
	case screenCS:
		screen = s.editorScreen(tr)
	case screenDiagnostics:
		screen = s.diagnosticsScreen(tr)
	case screenAbout:
		screen = s.aboutScreen(tr)
	default:
		screen = s.playScreen(tr)
	}
	if s.body != nil {
		s.body.Objects = []fyne.CanvasObject{
			container.NewVScroll(container.NewPadded(screen)),
		}
		s.body.Refresh()
	}
	s.syncRail()
}

func (s *shell) railContent(tr *i18n.Translator) fyne.CanvasObject {
	s.rail = map[screenID]*widget.Button{
		screenPlay: s.railButton(tr.T(i18n.KeyRailPlay),
			theme.MediaPlayIcon(), screenPlay),
		screenInstall: s.railButton(tr.T(i18n.KeyRailInstall),
			theme.DownloadIcon(), screenInstall),
		screenCS: s.railButton(tr.T(i18n.KeyRailOpenMWCS),
			theme.DocumentCreateIcon(), screenCS),
		screenDiagnostics: s.railButton(tr.T(i18n.KeyRailDiagnostics),
			theme.ListIcon(), screenDiagnostics),
		screenAbout: s.railButton(tr.T(i18n.KeyRailAbout),
			theme.InfoIcon(), screenAbout),
	}
	buttons := container.NewVBox(s.rail[screenPlay], s.rail[screenInstall],
		s.rail[screenCS], s.rail[screenDiagnostics], s.rail[screenAbout])
	return container.NewBorder(nil, nil, nil, nil, buttons)
}

func (s *shell) railButton(label string, icon fyne.Resource,
	id screenID,
) *widget.Button {
	button := widget.NewButtonWithIcon(label, icon, func() {
		s.showScreen(id)
	})
	button.Alignment = widget.ButtonAlignLeading
	return button
}

func (s *shell) syncRail() {
	for id, button := range s.rail {
		if id == s.screen {
			button.Importance = widget.HighImportance
		} else {
			button.Importance = widget.MediumImportance
		}
		button.Refresh()
	}
}

func (s *shell) applyTheme(mode string) {
	if fyne.CurrentApp() == nil {
		return
	}
	fyne.CurrentApp().Settings().SetTheme(style.ThemeFor(mode))
}

func (s *shell) brand(tr *i18n.Translator) fyne.CanvasObject {
	title := style.Heading(tr.T(i18n.KeyAppTitle))
	tagline := style.MutedLabel(tr.T(i18n.KeyAppTagline))
	return container.NewVBox(title, tagline)
}

func (s *shell) headerActions(tr *i18n.Translator) fyne.CanvasObject {
	return container.NewHBox(s.languageSwitch(tr), s.themeSwitch(tr))
}

// headerControlWidth is shared by both header controls, so the group keeps its
// size when a longer translation or another language arrives. It fits the
// "Как в системе" mode next to the theme caption.
const headerControlWidth float32 = 190

// themeMode pairs a catalog key with the mode it selects; the order is the
// order shown in the list.
type themeMode struct {
	key  string
	mode style.Mode
}

var themeModes = []themeMode{
	{key: i18n.KeyThemeSystem, mode: style.ModeSystem},
	{key: i18n.KeyThemeDark, mode: style.ModeDark},
	{key: i18n.KeyThemeLight, mode: style.ModeLight},
}

// themeModeLabels returns the localized mode names in themeModes order.
func themeModeLabels(tr *i18n.Translator) []string {
	labels := make([]string, 0, len(themeModes))
	for _, option := range themeModes {
		labels = append(labels, tr.T(option.key))
	}
	return labels
}

// themeModeLabel returns the caption of one mode. An unknown mode falls back to
// the system default, the same way style.ParseMode does.
func themeModeLabel(tr *i18n.Translator, mode style.Mode) string {
	mode = style.ParseMode(string(mode))
	for _, option := range themeModes {
		if option.mode == mode {
			return tr.T(option.key)
		}
	}
	return tr.T(i18n.KeyThemeSystem)
}

// themeModeByLabel maps a list entry back to its mode without relying on the
// index, so a reordered catalog cannot apply the wrong theme.
func themeModeByLabel(tr *i18n.Translator, label string) (style.Mode, bool) {
	for _, option := range themeModes {
		if tr.T(option.key) == label {
			return option.mode, true
		}
	}
	return "", false
}

// themeSwitch is the only theme control. A list shows all three modes at once,
// including the system one, and its fixed width keeps the header stable.
func (s *shell) themeSwitch(tr *i18n.Translator) fyne.CanvasObject {
	current := style.ParseMode(s.service.Config().Theme)
	selector := widget.NewSelect(themeModeLabels(tr), nil)
	selector.SetSelected(themeModeLabel(tr, current))
	selector.OnChanged = func(label string) {
		mode, ok := themeModeByLabel(tr, label)
		if !ok || mode == style.ParseMode(s.service.Config().Theme) {
			return
		}
		if !s.setTheme(string(mode)) {
			selector.SetSelected(themeModeLabel(tr, current))
		}
	}
	group := container.NewBorder(nil, nil,
		style.MutedLabel(tr.T(i18n.KeySettingsTheme)), nil, selector)
	return container.NewGridWrap(
		fyne.NewSize(headerControlWidth, selector.MinSize().Height), group)
}

// languageSwitch toggles the interface language. With two languages the caption
// names the language the click switches to, so the control needs no new key and
// no hint; a third language would turn it into the same list as the theme.
func (s *shell) languageSwitch(tr *i18n.Translator) fyne.CanvasObject {
	target, ok := nextLanguage(s.service.Language())
	button := widget.NewButton(i18n.DisplayName(target), func() {
		s.setLanguage(target)
	})
	// The caption starts at the same offset as the theme list text above it.
	button.Alignment = widget.ButtonAlignLeading
	if !ok {
		button.Disable()
	}
	return container.NewGridWrap(
		fyne.NewSize(headerControlWidth, button.MinSize().Height), button)
}

// nextLanguage returns the language the header button offers: the entry after
// the current one in i18n.Supported, so two languages toggle each other. An
// unknown current language falls back to the first supported one.
func nextLanguage(current i18n.Language) (i18n.Language, bool) {
	languages := i18n.Supported()
	if len(languages) < 2 {
		return "", false
	}
	for index, language := range languages {
		if language == current {
			return languages[(index+1)%len(languages)], true
		}
	}
	return languages[0], true
}

// setTheme persists the theme and applies it in place. The window is not
// rebuilt, so the scroll position and the focus survive the switch; the list
// already shows the new choice.
func (s *shell) setTheme(mode string) bool {
	settings := s.settings()
	settings.Theme = mode
	if err := s.service.UpdateSettings(settings); err != nil {
		s.showError(err)
		return false
	}
	s.applyTheme(mode)
	return true
}

// setLanguage persists the interface language and rebuilds the window: every
// string changes, including the captions of the header controls.
func (s *shell) setLanguage(language i18n.Language) {
	settings := s.settings()
	settings.Language = string(language)
	if err := s.service.UpdateSettings(settings); err != nil {
		s.showError(err)
		return
	}
	s.render()
}

// copyReport copies the diagnostic report with the latest log lines.
func (s *shell) copyReport() {
	tr := s.service.Translator()
	lines, _ := s.service.ReadLogTail(app.TailLines)
	report := s.service.DiagnosticReport(nil, lines)
	fyne.CurrentApp().Clipboard().SetContent(report)
	dialog.ShowInformation(tr.T(i18n.KeyDiagnosticsCopyReport),
		tr.T(i18n.KeyDiagnosticsReportCopied), s.window)
}

func (s *shell) settings() app.Settings {
	cfg := s.service.Config()
	return app.Settings{
		Language:      cfg.Language,
		Theme:         cfg.Theme,
		MorrowindPath: cfg.MorrowindPath,
		WorkDir:       cfg.WorkDir,
		ProtonPath:    cfg.ProtonPath,
	}
}
