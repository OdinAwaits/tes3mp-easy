package ui

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/Saagilat/tes3mp-easy/internal/config"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/locale"
	"github.com/Saagilat/tes3mp-easy/internal/ui/style"
)

type serverForm struct {
	endpoint *widget.Entry
	password *widget.Entry
	name     *widget.Entry
	panelURL *widget.Entry
}

func newServerForm(profile config.Server) *serverForm {
	endpoint := widget.NewEntry()
	endpoint.SetText(endpointText(profile))
	endpoint.SetPlaceHolder("play.example.com or play.example.com:25565")

	password := widget.NewPasswordEntry()
	password.SetText(profile.Password)
	password.SetPlaceHolder("Optional server password")

	name := widget.NewEntry()
	name.SetText(profile.Name)
	name.SetPlaceHolder("Morrowind with friends")

	panelURL := widget.NewEntry()
	panelURL.SetText(profile.PanelURL)
	panelURL.SetPlaceHolder("https://panel.example.com")

	return &serverForm{
		endpoint: endpoint,
		password: password,
		name:     name,
		panelURL: panelURL,
	}
}

func endpointText(profile config.Server) string {
	if strings.TrimSpace(profile.Address) == "" {
		return ""
	}
	if profile.Port <= 0 || profile.Port == config.DefaultServerPort {
		return config.NormalizeAddress(profile.Address)
	}
	return net.JoinHostPort(config.NormalizeAddress(profile.Address),
		strconv.Itoa(profile.Port))
}

func (f *serverForm) profile() (config.Server, error) {
	endpoint, err := config.ParseServerEndpoint(f.endpoint.Text)
	if err != nil {
		return config.Server{}, err
	}
	name := strings.TrimSpace(f.name.Text)
	if name == "" {
		name = endpoint.Address
		if endpoint.Port != config.DefaultServerPort {
			name = net.JoinHostPort(endpoint.Address, strconv.Itoa(endpoint.Port))
		}
	}
	panelURL := endpoint.PanelURL
	if raw := strings.TrimSpace(f.panelURL.Text); raw != "" {
		normalized, err := config.NormalizePanelURL(raw)
		if err != nil {
			return config.Server{}, fmt.Errorf("panel URL: %w", err)
		}
		panelURL = normalized
	}
	return config.Server{
		Name:     name,
		Address:  endpoint.Address,
		Port:     endpoint.Port,
		Password: f.password.Text,
		PanelURL: panelURL,
	}, nil
}

func (s *shell) showServerDialog(originalID string, profile config.Server) {
	tr := s.service.Translator()
	form := newServerForm(profile)
	title := tr.T(i18n.KeyServerAddTitle)
	if originalID != "" {
		title = tr.T(i18n.KeyServerEditTitle)
	}

	problem := widget.NewLabel("")
	problem.Wrapping = fyne.TextWrapWord
	problem.Importance = widget.DangerImportance
	problem.Hide()

	advanced := container.NewVBox(
		widget.NewLabel(tr.T(i18n.KeyServerFieldName)),
		form.name,
		widget.NewLabel(tr.T(i18n.KeyServerFieldPanel)),
		form.panelURL,
		widget.NewLabel(tr.T(i18n.KeyServerPanelHint)),
	)
	advanced.Hide()
	toggle := widget.NewButton(tr.T(i18n.KeyServerAdvancedOptions), func() {
		if advanced.Visible() {
			advanced.Hide()
		} else {
			advanced.Show()
		}
	})

	content := container.NewVBox(
		widget.NewLabel(tr.T(i18n.KeyServerFieldAddr)),
		form.endpoint,
		style.MutedLabel(tr.T(i18n.KeyServerEndpointHint)),
		widget.NewLabel(tr.T(i18n.KeyServerFieldPass)),
		form.password,
		toggle,
		advanced,
		problem,
	)

	popup := dialog.NewCustomWithoutButtons(title, content, s.window)
	cancel := widget.NewButton(tr.T(i18n.KeyCommonCancel), popup.Hide)
	save := widget.NewButton(tr.T(i18n.KeyCommonSave), func() {
		value, err := form.profile()
		if err != nil {
			problem.SetText(s.failureBody(err))
			problem.Show()
			return
		}
		if _, err := s.service.SaveServer(originalID, value); err != nil {
			problem.SetText(s.failureBody(err))
			problem.Show()
			return
		}
		popup.Hide()
		s.render()
	})
	save.Importance = widget.HighImportance
	popup.SetButtons([]fyne.CanvasObject{cancel, save})
	popup.Resize(fyne.NewSize(560, 430))
	popup.Show()
}
func localizationLabel(tr *i18n.Translator, language string) string {
	if language == locale.LanguageRussian {
		return tr.T(i18n.KeyLanguageRussian)
	}
	return language
}

func activeProfile(cfg config.Config) (config.Server, bool) {
	for _, profile := range cfg.Servers {
		if profile.ID == cfg.ActiveServer {
			return profile, true
		}
	}
	if len(cfg.Servers) == 0 {
		return config.Server{}, false
	}
	return cfg.Servers[0], true
}
