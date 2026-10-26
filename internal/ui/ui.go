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
	footer := widget.NewLabelWithStyle(
		tr.Tf(i18n.KeyFooterPath, s.service.ConfigPath()),
		fyne.TextAlignLeading,
		fyne.TextStyle{Monospace: true},
	)
	footer.Importance = widget.LowImportance
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
	return s.themeSwitch(tr)
}

// themeSwitch is the only theme control: the active mode keeps a high
// importance, so the header shows the current choice without a menu.
func (s *shell) themeSwitch(tr *i18n.Translator) fyne.CanvasObject {
	current := style.ParseMode(s.service.Config().Theme)
	options := []struct {
		key  string
		mode style.Mode
	}{
		{key: i18n.KeyThemeSystem, mode: style.ModeSystem},
		{key: i18n.KeyThemeDark, mode: style.ModeDark},
		{key: i18n.KeyThemeLight, mode: style.ModeLight},
	}
	buttons := make([]fyne.CanvasObject, 0, len(options))
	for _, option := range options {
		mode := option.mode
		button := widget.NewButton(tr.T(option.key), func() {
			s.setTheme(string(mode))
		})
		button.Importance = themeOptionImportance(current, mode)
		buttons = append(buttons, button)
	}
	return container.NewHBox(buttons...)
}

// themeOptionImportance keeps the active mode visible and is testable without
// a running Fyne application.
func themeOptionImportance(current, mode style.Mode) widget.Importance {
	if current == mode {
		return widget.HighImportance
	}
	return widget.MediumImportance
}

// setTheme persists the theme and re-renders the window with it.
func (s *shell) setTheme(mode string) {
	settings := s.settings()
	settings.Theme = mode
	if err := s.service.UpdateSettings(settings); err != nil {
		s.showError(err)
		return
	}
	s.applyTheme(mode)
	s.render()
}

// copyReport copies the diagnostic report with the latest log lines.
func (s *shell) copyReport() {
	tr := s.service.Translator()
	lines, _ := s.service.ReadLogTail(app.TailLines)
	report := s.service.DiagnosticReport(nil, lines)
	s.window.Clipboard().SetContent(report)
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
		SkipFonts:     cfg.SkipFonts,
	}
}
