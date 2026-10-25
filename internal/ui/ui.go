// Package ui renders the Fyne client shell.
//
// The package only marshals values from internal/app and forwards user actions
// back to it. Profile validation and persistence live outside Fyne so they can
// be tested without a display.
package ui

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"sync"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
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

type shell struct {
	service *app.Service
	window  fyne.Window

	statusMu sync.Mutex
	statuses map[string]serverStatus
	checking map[string]bool
}

func (s *shell) render() {
	tr := s.service.Translator()
	cfg := s.service.Config()
	s.window.SetTitle(tr.T(i18n.KeyAppTitle))
	s.applyTheme(cfg.Theme)

	header := container.NewBorder(nil, nil, s.brand(tr), s.headerActions(tr), nil)
	body := s.serverList(tr, cfg)
	footer := widget.NewLabelWithStyle(
		tr.Tf(i18n.KeyFooterPath, s.service.ConfigPath()),
		fyne.TextAlignLeading,
		fyne.TextStyle{Monospace: true},
	)
	footer.Importance = widget.LowImportance
	content := container.NewBorder(
		container.NewPadded(header),
		container.NewPadded(footer),
		nil,
		nil,
		container.NewStack(container.NewVScroll(container.NewPadded(body))),
	)
	s.window.SetContent(content)
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
	logs := widget.NewButtonWithIcon(tr.T(i18n.KeyLogsTitle), theme.ListIcon(), s.showLogs)
	settings := widget.NewButtonWithIcon(tr.T(i18n.KeyCommonSettings), theme.SettingsIcon(), s.showSettings)
	about := widget.NewButtonWithIcon(tr.T(i18n.KeyCommonAbout), theme.HelpIcon(), s.showAbout)
	return container.NewHBox(add, logs, settings, about)
}

func (s *shell) serverList(tr *i18n.Translator, cfg config.Config) fyne.CanvasObject {
	if len(cfg.Servers) == 0 {
		title := style.Heading(tr.T(i18n.KeyServerEmptyTitle))
		hint := style.MutedLabel(tr.T(i18n.KeyServerEmptyHint))
		add := widget.NewButtonWithIcon(tr.T(i18n.KeyCommonAdd), theme.ContentAddIcon(), func() {
			s.showServerDialog("", config.Server{Port: config.DefaultServerPort})
		})
		add.Importance = widget.HighImportance
		content := container.NewVBox(
			container.NewCenter(title),
			container.NewCenter(hint),
			container.NewCenter(add),
		)
		return container.NewCenter(style.Card(content, true))
	}

	rows := make([]fyne.CanvasObject, 0, len(cfg.Servers)+1)
	activeID := cfg.ActiveServer
	if activeID == "" && len(cfg.Servers) == 1 {
		activeID = cfg.Servers[0].ID
	}
	for _, profile := range cfg.Servers {
		rows = append(rows, s.serverCard(tr, activeID, profile))
	}
	rows = append(rows, container.NewCenter(widget.NewButtonWithIcon(
		tr.T(i18n.KeyCommonAdd), theme.ContentAddIcon(), func() {
			s.showServerDialog("", config.Server{Port: config.DefaultServerPort})
		})))
	return container.NewVBox(rows...)
}

func (s *shell) serverCard(tr *i18n.Translator, activeID string,
	profile config.Server,
) fyne.CanvasObject {
	active := profile.ID == activeID
	titleText := profile.Name
	if active {
		titleText += "  ·  " + tr.T(i18n.KeyServerActive)
	}
	title := style.Heading(titleText)
	address := widget.NewLabelWithStyle(
		serverEndpoint(profile),
		fyne.TextAlignLeading,
		fyne.TextStyle{Monospace: true},
	)
	address.Importance = widget.LowImportance

	statusLabel := widget.NewLabel(s.statusText(profile.ID))
	statusLabel.Wrapping = fyne.TextWrapWord
	statusLabel.Importance = s.statusImportance(profile.ID)

	modsInstalled := false
	if state, err := s.service.ModsState(profile.ID); err == nil {
		modsInstalled = state.Installed
	}
	play := widget.NewButtonWithIcon(
		playButtonLabel(tr, modsInstalled, s.statusFor(profile.ID).NeedsUpdate),
		theme.MediaPlayIcon(),
		func() { s.playServer(profile) },
	)
	play.Importance = widget.HighImportance
	more := widget.NewButton(tr.T(i18n.KeyServerMore), func() {
		s.showServerActions(profile)
	})

	var panel fyne.CanvasObject = layout.NewSpacer()
	if effective := profile.EffectivePanelURL(); effective != "" {
		if parsed, err := url.Parse(effective); err == nil {
			link := widget.NewHyperlink(tr.T(i18n.KeyServerPanel)+" · "+parsed.Host, parsed)
			panel = link
		}
	}

	details := container.NewBorder(nil, nil, nil, container.NewHBox(panel, play, more),
		container.NewVBox(address, statusLabel))
	content := container.NewVBox(title, details)
	s.checkServer(profile, statusLabel, play, modsInstalled)
	return style.Card(content, active)
}

func playButtonLabel(tr *i18n.Translator, modsInstalled, needsUpdate bool) string {
	switch {
	case !modsInstalled:
		return tr.T(i18n.KeyServerPlayDownload)
	case needsUpdate:
		return tr.T(i18n.KeyServerPlayUpdate)
	default:
		return tr.T(i18n.KeyCommonPlay)
	}
}

func serverEndpoint(profile config.Server) string {
	address := config.NormalizeAddress(profile.Address)
	if address == "" || profile.Port <= 0 {
		return address
	}
	return net.JoinHostPort(address, strconv.Itoa(profile.Port))
}

func (s *shell) showServerActions(profile config.Server) {
	tr := s.service.Translator()
	var popup *dialog.CustomDialog
	content := container.NewVBox(
		style.Subheading(profile.Name),
		style.MutedLabel(serverEndpoint(profile)),
		widget.NewButton(tr.T(i18n.KeyServerOpenPanel), func() {
			s.openPanel(profile)
		}),
		widget.NewButton(tr.T(i18n.KeyServerUpdateMods), func() {
			s.showModsUpdate(profile)
		}),
		widget.NewButton(tr.T(i18n.KeyLocaleInstallTitle), func() {
			if popup != nil {
				popup.Hide()
			}
			s.showLanguageWizard()
		}),
		widget.NewButton(tr.T(i18n.KeyCommonAdvanced), func() {
			s.showAdvanced(profile)
		}),
		widget.NewButton(tr.T(i18n.KeyCommonEdit), func() {
			s.showServerDialog(profile.ID, profile)
		}),
		widget.NewButton(tr.T(i18n.KeyCommonDelete), func() {
			s.confirmDelete(tr, profile)
		}),
	)
	popup = dialog.NewCustom(tr.T(i18n.KeyServerActionsTitle),
		tr.T(i18n.KeyCommonClose), content, s.window)
	popup.Resize(fyne.NewSize(460, 420))
	popup.Show()
}

func (s *shell) openPanel(profile config.Server) {
	parsed, err := url.Parse(profile.EffectivePanelURL())
	if err != nil {
		s.showError(err)
		return
	}
	if fyne.CurrentApp() == nil {
		return
	}
	if err := fyne.CurrentApp().OpenURL(parsed); err != nil {
		s.showError(err)
	}
}

func (s *shell) confirmDelete(tr *i18n.Translator, profile config.Server) {
	content := widget.NewLabel(tr.Tf(i18n.KeyServerDeleteBody, profile.Name))
	content.Wrapping = fyne.TextWrapWord
	confirm := dialog.NewCustomConfirm(
		tr.T(i18n.KeyServerDeleteTitle),
		tr.T(i18n.KeyCommonDelete),
		tr.T(i18n.KeyCommonCancel),
		content,
		func(ok bool) {
			if !ok {
				return
			}
			if err := s.service.DeleteServer(profile.ID); err != nil {
				s.showError(err)
				return
			}
			s.statusMu.Lock()
			delete(s.statuses, profile.ID)
			delete(s.checking, profile.ID)
			s.statusMu.Unlock()
			s.render()
		},
		s.window,
	)
	confirm.Show()
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
