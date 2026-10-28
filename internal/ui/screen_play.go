package ui

import (
	"net"
	"net/url"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Saagilat/tes3mp-easy/internal/config"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/ui/style"
)

// playScreen lists server profiles. It is the default section of the rail.
func (s *shell) playScreen(tr *i18n.Translator) fyne.CanvasObject {
	header := container.NewVBox(
		style.Heading(tr.T(i18n.KeyServerListTitle)),
		style.MutedLabel(tr.T(i18n.KeyServerListHint)),
	)
	if len(s.service.Config().Servers) == 0 {
		return container.NewVBox(header, s.serverList(tr, s.service.Config()))
	}
	add := s.addServerButton(tr)
	headerRow := container.NewBorder(nil, nil, nil,
		container.NewCenter(add), header)
	return container.NewVBox(headerRow,
		s.serverList(tr, s.service.Config()))
}

func (s *shell) addServerButton(tr *i18n.Translator) *widget.Button {
	add := widget.NewButtonWithIcon(tr.T(i18n.KeyCommonAdd),
		theme.ContentAddIcon(), func() {
			s.showServerDialog("", config.Server{Port: config.DefaultServerPort})
		})
	add.Importance = widget.HighImportance
	return add
}

func (s *shell) serverList(tr *i18n.Translator, cfg config.Config) fyne.CanvasObject {
	if len(cfg.Servers) == 0 {
		title := style.CenteredHeading(tr.T(i18n.KeyServerEmptyTitle))
		hint := style.CenteredMutedLabel(tr.T(i18n.KeyServerEmptyHint))
		content := container.NewVBox(
			title,
			hint,
			container.NewCenter(s.addServerButton(tr)),
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
	if _, running := s.service.GameRunning(); running {
		play.Disable()
	}
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
	content := container.NewVBox(
		style.Subheading(profile.Name),
		style.MutedLabel(serverEndpoint(profile)),
		widget.NewButton(tr.T(i18n.KeyServerOpenPanel), func() {
			s.openPanel(profile)
		}),
		widget.NewButton(tr.T(i18n.KeyServerUpdateMods), func() {
			s.showModsUpdate(profile)
		}),
		widget.NewButton(tr.T(i18n.KeyCommonEdit), func() {
			s.showServerDialog(profile.ID, profile)
		}),
		widget.NewButton(tr.T(i18n.KeyCommonDelete), func() {
			s.confirmDelete(tr, profile)
		}),
	)
	popup := dialog.NewCustom(tr.T(i18n.KeyServerActionsTitle),
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
