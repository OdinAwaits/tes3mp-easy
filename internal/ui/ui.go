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
	"github.com/Saagilat/tes3mp-easy/internal/config"
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
	screenPlay    screenID = "play"
	screenInstall screenID = "installation"
	screenCS      screenID = "openmw-cs"
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
	// addButton is only relevant on the Play screen.
	addButton *widget.Button
	// issues counts installation rows that still need an action.
	issues int
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
	s.refreshInstallationIssues()
}

// showScreen swaps the body and keeps the rail in sync with the selection.
func (s *shell) showScreen(id screenID) {
	if id != screenInstall && id != screenCS {
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
	default:
		screen = s.playScreen(tr)
	}
	if s.body != nil {
		s.body.Objects = []fyne.CanvasObject{
			container.NewVScroll(container.NewPadded(screen)),
		}
		s.body.Refresh()
	}
	if s.addButton != nil {
		if id == screenPlay {
			s.addButton.Show()
		} else {
			s.addButton.Hide()
		}
	}
	s.syncRail()
}

func (s *shell) railContent(tr *i18n.Translator) fyne.CanvasObject {
	s.rail = map[screenID]*widget.Button{
		screenPlay: s.railButton(tr.T(i18n.KeyRailPlay),
			theme.MediaPlayIcon(), screenPlay),
		screenInstall: s.railButton(railInstallLabel(tr, s.issues),
			theme.DownloadIcon(), screenInstall),
		screenCS: s.railButton(tr.T(i18n.KeyRailOpenMWCS),
			theme.DocumentCreateIcon(), screenCS),
	}
	buttons := container.NewVBox(s.rail[screenPlay], s.rail[screenInstall],
		s.rail[screenCS])
	note := style.MutedLabel(tr.T(i18n.KeyRailMenuNote))
	return container.NewBorder(nil, note, nil, nil, buttons)
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
	tr := s.service.Translator()
	for id, button := range s.rail {
		if id == screenInstall {
			button.SetText(railInstallLabel(tr, s.issues))
		}
		if id == s.screen {
			button.Importance = widget.HighImportance
		} else {
			button.Importance = widget.MediumImportance
		}
		button.Refresh()
	}
}

// railInstallLabel shows how many installation rows still need attention.
func railInstallLabel(tr *i18n.Translator, issues int) string {
	label := tr.T(i18n.KeyRailInstall)
	if issues <= 0 {
		return label
	}
	return tr.Tf(i18n.KeyRailInstallBadge, label, issues)
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
	add := widget.NewButtonWithIcon(tr.T(i18n.KeyCommonAdd), theme.ContentAddIcon(), func() {
		s.showServerDialog("", config.Server{Port: config.DefaultServerPort})
	})
	add.Importance = widget.HighImportance
	s.addButton = add
	menu := widget.NewButtonWithIcon(tr.T(i18n.KeyCommonMenu), theme.MenuIcon(), nil)
	menu.OnTapped = func() {
		widget.ShowPopUpMenuAtRelativePosition(s.appMenu(tr), s.window.Canvas(),
			fyne.NewPos(0, menu.Size().Height), menu)
	}
	return container.NewHBox(add, menu)
}

// appMenu contains global actions. None of them belong to a server profile.
func (s *shell) appMenu(tr *i18n.Translator) *fyne.Menu {
	themeItem := fyne.NewMenuItem(tr.T(i18n.KeySettingsTheme), nil)
	themeItem.ChildMenu = fyne.NewMenu("",
		s.themeMenuItem(tr, i18n.KeyThemeSystem, config.ThemeSystem),
		s.themeMenuItem(tr, i18n.KeyThemeDark, config.ThemeDark),
		s.themeMenuItem(tr, i18n.KeyThemeLight, config.ThemeLight),
	)
	return fyne.NewMenu("",
		fyne.NewMenuItem(tr.T(i18n.KeyCommonLanguage), s.showLanguageWizard),
		themeItem,
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem(tr.T(i18n.KeyLogsTitle), s.showLogs),
		fyne.NewMenuItem(tr.T(i18n.KeyMenuCopyReport), s.copyReport),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem(tr.T(i18n.KeyCommonAbout), s.showAbout),
	)
}

func (s *shell) themeMenuItem(tr *i18n.Translator, labelKey, mode string) *fyne.MenuItem {
	return newThemeMenuItem(tr, labelKey, mode, s.service.Config().Theme,
		s.setTheme)
}

// newThemeMenuItem keeps the theme menu and its checked state testable without
// a running service.
func newThemeMenuItem(tr *i18n.Translator, labelKey, mode, current string,
	apply func(string),
) *fyne.MenuItem {
	item := fyne.NewMenuItem(tr.T(labelKey), func() {
		apply(mode)
	})
	item.Checked = style.ParseMode(current) == style.ParseMode(mode)
	return item
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
	dialog.ShowInformation(tr.T(i18n.KeyMenuCopyReport),
		tr.T(i18n.KeyMenuReportCopied), s.window)
}

func (s *shell) showAbout() {
	tr := s.service.Translator()
	dialog.ShowInformation(
		tr.T(i18n.KeyAboutTitle),
		tr.Tf(i18n.KeyAboutBody, app.Version, app.TES3MPVersion, app.ProjectURL),
		s.window,
	)
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
