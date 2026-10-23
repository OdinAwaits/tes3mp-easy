// Package ui renders the Fyne client shell.
//
// The package only marshals values from internal/app and forwards user actions
// back to it. Profile validation and persistence live outside Fyne so they can
// be tested without a display.
package ui

import (
	"fmt"
	"strconv"

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
)

const appID = "io.github.saagilat.tes3mp-easy"

// Run starts the desktop shell and blocks until the last window is closed.
func Run(service *app.Service) error {
	if service == nil {
		return fmt.Errorf("ui: app service is nil")
	}
	fyneApp := fyneapp.NewWithID(appID)
	window := fyneApp.NewWindow(service.Translator().T(i18n.KeyAppTitle))
	window.Resize(fyne.NewSize(920, 620))
	shell := &shell{service: service, window: window}
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
}

func (s *shell) render() {
	tr := s.service.Translator()
	cfg := s.service.Config()
	s.window.SetTitle(tr.T(i18n.KeyAppTitle))

	header := container.NewBorder(nil, nil, s.brand(tr), s.headerActions(tr), nil)
	body := s.serverList(tr, cfg)
	footer := widget.NewLabelWithStyle(
		tr.Tf(i18n.KeyFooterPath, s.service.ConfigPath()),
		fyne.TextAlignLeading,
		fyne.TextStyle{Monospace: true},
	)
	content := container.NewBorder(
		container.NewPadded(header),
		container.NewPadded(footer),
		nil,
		nil,
		container.NewStack(container.NewVScroll(container.NewPadded(body))),
	)
	s.window.SetContent(content)
}

func (s *shell) brand(tr *i18n.Translator) fyne.CanvasObject {
	title := widget.NewLabelWithStyle(
		tr.T(i18n.KeyAppTitle),
		fyne.TextAlignLeading,
		fyne.TextStyle{Bold: true},
	)
	tagline := widget.NewLabel(tr.T(i18n.KeyAppTagline))
	return container.NewVBox(title, tagline)
}

func (s *shell) headerActions(tr *i18n.Translator) fyne.CanvasObject {
	add := widget.NewButtonWithIcon(tr.T(i18n.KeyCommonAdd), theme.ContentAddIcon(), func() {
		s.showServerDialog("", config.Server{Port: 25565})
	})
	language := s.languageSelect(tr)
	logs := widget.NewButton(tr.T(i18n.KeyLogsTitle), s.showLogs)
	setup := widget.NewButton(tr.T(i18n.KeyCommonSetup), s.showSetup)
	settings := widget.NewButtonWithIcon(tr.T(i18n.KeyCommonSettings), theme.SettingsIcon(), s.showSettings)
	about := widget.NewButton(tr.T(i18n.KeyCommonAbout), s.showAbout)
	return container.NewHBox(add, language, logs, setup, settings, about)
}

func (s *shell) languageSelect(tr *i18n.Translator) *widget.Select {
	languages := i18n.Supported()
	names := make([]string, len(languages))
	for index, language := range languages {
		names[index] = i18n.DisplayName(language)
	}
	selectWidget := widget.NewSelect(names, func(selected string) {
		for index, name := range names {
			if name != selected {
				continue
			}
			language := languages[index]
			if language == s.service.Language() {
				return
			}
			settings := s.settings()
			settings.Language = string(language)
			if err := s.service.UpdateSettings(settings); err != nil {
				s.showError(err)
				return
			}
			s.render()
			return
		}
	})
	selectWidget.SetSelected(i18n.DisplayName(s.service.Language()))
	return selectWidget
}

func (s *shell) serverList(tr *i18n.Translator, cfg config.Config) fyne.CanvasObject {
	if len(cfg.Servers) == 0 {
		title := widget.NewLabelWithStyle(
			tr.T(i18n.KeyServerEmptyTitle),
			fyne.TextAlignCenter,
			fyne.TextStyle{Bold: true},
		)
		hint := widget.NewLabelWithStyle(
			tr.T(i18n.KeyServerEmptyHint),
			fyne.TextAlignCenter,
			fyne.TextStyle{},
		)
		add := widget.NewButtonWithIcon(tr.T(i18n.KeyCommonAdd), theme.ContentAddIcon(), func() {
			s.showServerDialog("", config.Server{Port: 25565})
		})
		return container.NewCenter(container.NewVBox(title, hint, add))
	}

	rows := make([]fyne.CanvasObject, 0, len(cfg.Servers))
	for _, profile := range cfg.Servers {
		rows = append(rows, s.serverCard(tr, cfg.ActiveServer, profile))
	}
	return container.NewVBox(rows...)
}

func (s *shell) serverCard(tr *i18n.Translator, activeID string,
	profile config.Server,
) fyne.CanvasObject {
	subtitle := fmt.Sprintf("%s:%d", profile.Address, profile.Port)
	if profile.ID == activeID {
		subtitle += " - " + tr.T(i18n.KeyServerActive)
	}
	panelLabel := widget.NewLabel(tr.T(i18n.KeyServerPanel) + ": " + profile.PanelURL)
	panelLabel.Wrapping = fyne.TextWrapWord

	play := widget.NewButtonWithIcon(tr.T(i18n.KeyCommonPlay), theme.MediaPlayIcon(), func() {
		s.showLaunch(profile)
	})
	check := widget.NewButton(tr.T(i18n.KeyServerCheck), func() {
		s.showCheckServer(profile)
	})
	updateMods := widget.NewButton(tr.T(i18n.KeyServerUpdateMods), func() {
		s.showModsUpdate(profile)
	})
	advanced := widget.NewButton(tr.T(i18n.KeyCommonAdvanced), func() {
		s.showAdvanced(profile)
	})
	edit := widget.NewButton(tr.T(i18n.KeyCommonEdit), func() {
		s.showServerDialog(profile.ID, profile)
	})
	remove := widget.NewButton(tr.T(i18n.KeyCommonDelete), func() {
		s.confirmDelete(tr, profile)
	})
	actions := container.NewHBox(play, check, updateMods, advanced, edit, remove)
	if profile.ID != activeID {
		actions.Add(widget.NewButton(tr.T(i18n.KeyServerMakeActive), func() {
			if err := s.service.SetActiveServer(profile.ID); err != nil {
				s.showError(err)
				return
			}
			s.render()
		}))
	}
	return widget.NewCard(
		profile.Name,
		subtitle,
		container.NewVBox(panelLabel, container.NewHBox(layout.NewSpacer(), actions)),
	)
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
		MorrowindPath: cfg.MorrowindPath,
		WorkDir:       cfg.WorkDir,
		ProtonPath:    cfg.ProtonPath,
		Localization:  cfg.Localization,
	}
}

func portText(port int) string {
	if port <= 0 {
		return ""
	}
	return strconv.Itoa(port)
}
