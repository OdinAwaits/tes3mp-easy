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

// launcherSize is the window the client opens with. The setup wizard fills it:
// its last step holds a sample above a form of four values, which does not fit
// a small dialog.
var launcherSize = fyne.NewSize(1000, 700)

// Run starts the desktop shell and blocks until the last window is closed.
func Run(service *app.Service) error {
	if service == nil {
		return fmt.Errorf("ui: app service is nil")
	}
	fyneApp := fyneapp.NewWithID(appID)
	fyneApp.Settings().SetTheme(style.ThemeFor(service.Config().Theme))
	window := fyneApp.NewWindow(service.Translator().T(i18n.KeyAppTitle))
	window.Resize(launcherSize)
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

// headerControlWidth and headerControlHeight are shared by both header
// controls, so the group keeps its size when a longer translation or another
// language arrives and the two captions stay on one line.
const (
	headerControlWidth  float32 = 190
	headerControlHeight float32 = 32
)

// headerSlot packs one control into a fixed cell: the language list fills the
// width, the 44 px switch keeps its own size. Both start on the same left edge,
// so the captions above them line up and neither control stretches the pill.
type headerSlot struct {
	stretch bool
}

func (l headerSlot) MinSize([]fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(headerControlWidth, headerControlHeight)
}

func (l headerSlot) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, object := range objects {
		natural := object.MinSize()
		width := natural.Width
		if l.stretch {
			width = size.Width
		}
		object.Resize(fyne.NewSize(width, natural.Height))
		object.Move(fyne.NewPos(0, (size.Height-natural.Height)/2))
	}
}

// headerControl stacks a caption above its control. A caption placed sideways in
// a Border reads as part of the value ("ТемаСистемная"); above it, it names the
// setting. Both header groups use the same structure, so they keep one height.
func headerControl(caption string, control fyne.CanvasObject,
	stretch bool,
) fyne.CanvasObject {
	return container.NewVBox(
		style.MutedLabel(caption),
		container.New(headerSlot{stretch: stretch}, control),
	)
}

// languageSwitch lists every supported language. A list shows the current
// language at a glance and does not need a new control when a third one arrives.
func (s *shell) languageSwitch(tr *i18n.Translator) fyne.CanvasObject {
	languages := i18n.Supported()
	labels := make([]string, 0, len(languages))
	for _, language := range languages {
		labels = append(labels, i18n.DisplayName(language))
	}
	selector := widget.NewSelect(labels, nil)
	selector.SetSelected(i18n.DisplayName(s.service.Language()))
	selector.OnChanged = func(label string) {
		language, ok := languageByLabel(label)
		if !ok || language == s.service.Language() {
			return
		}
		s.setLanguage(language)
	}
	if len(languages) < 2 {
		selector.Disable()
	}
	return headerControl(tr.T(i18n.KeySettingsLanguage), selector, true)
}

// languageByLabel maps a list entry back to its language without relying on the
// index, so a reordered catalog cannot apply the wrong interface language.
func languageByLabel(label string) (i18n.Language, bool) {
	for _, language := range i18n.Supported() {
		if i18n.DisplayName(language) == label {
			return language, true
		}
	}
	return "", false
}

// themeSwitch is a two-position toggle: on means the amber dark theme, off the
// light one. A config that still says "system" shows the variant the desktop
// actually paints, and the first click writes an explicit dark or light mode.
func (s *shell) themeSwitch(tr *i18n.Translator) fyne.CanvasObject {
	toggle := newSwitchToggle(style.IsDark(s.service.Config().Theme), nil)
	toggle.onChange = func(checked bool) {
		mode := style.ModeLight
		if checked {
			mode = style.ModeDark
		}
		if style.ParseMode(s.service.Config().Theme) == mode {
			return
		}
		if !s.setTheme(string(mode)) {
			toggle.SetChecked(!checked)
		}
	}
	return headerControl(tr.T(i18n.KeySettingsDarkTheme), toggle, false)
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
