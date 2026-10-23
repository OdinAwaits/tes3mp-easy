package ui

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/Saagilat/tes3mp-easy/internal/app"
	"github.com/Saagilat/tes3mp-easy/internal/config"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
)

type serverForm struct {
	name     *widget.Entry
	address  *widget.Entry
	port     *widget.Entry
	password *widget.Entry
	panelURL *widget.Entry
}

func newServerForm(profile config.Server) *serverForm {
	name := widget.NewEntry()
	name.SetText(profile.Name)
	name.SetPlaceHolder("Morrowind with friends")

	address := widget.NewEntry()
	address.SetText(profile.Address)
	address.SetPlaceHolder("203.0.113.10")

	port := widget.NewEntry()
	port.SetText(portText(profile.Port))
	port.SetPlaceHolder("25565")

	password := widget.NewPasswordEntry()
	password.SetText(profile.Password)

	panelURL := widget.NewEntry()
	panelURL.SetText(profile.PanelURL)
	panelURL.SetPlaceHolder("https://203.0.113.10")

	return &serverForm{
		name:     name,
		address:  address,
		port:     port,
		password: password,
		panelURL: panelURL,
	}
}

func (f *serverForm) items(tr *i18n.Translator) []*widget.FormItem {
	return []*widget.FormItem{
		widget.NewFormItem(tr.T(i18n.KeyServerFieldName), f.name),
		widget.NewFormItem(tr.T(i18n.KeyServerFieldAddr), f.address),
		widget.NewFormItem(tr.T(i18n.KeyServerFieldPort), f.port),
		widget.NewFormItem(tr.T(i18n.KeyServerFieldPass), f.password),
		widget.NewFormItem(tr.T(i18n.KeyServerFieldPanel), f.panelURL),
	}
}

func (f *serverForm) profile() (config.Server, error) {
	port, err := strconv.Atoi(strings.TrimSpace(f.port.Text))
	if err != nil {
		return config.Server{}, fmt.Errorf("port must be a number")
	}
	return config.Server{
		Name:     f.name.Text,
		Address:  f.address.Text,
		Port:     port,
		Password: f.password.Text,
		PanelURL: f.panelURL.Text,
	}, nil
}

func (s *shell) showServerDialog(originalID string, profile config.Server) {
	tr := s.service.Translator()
	form := newServerForm(profile)
	title := tr.T(i18n.KeyServerAddTitle)
	if originalID != "" {
		title = tr.T(i18n.KeyServerEditTitle)
	}
	formDialog := dialog.NewForm(
		title,
		tr.T(i18n.KeyCommonSave),
		tr.T(i18n.KeyCommonCancel),
		form.items(tr),
		func(ok bool) {
			if !ok {
				return
			}
			value, err := form.profile()
			if err != nil {
				s.showError(err)
				return
			}
			if _, err := s.service.SaveServer(originalID, value); err != nil {
				s.showError(err)
				return
			}
			s.render()
		},
		s.window,
	)
	formDialog.Show()
}

func (s *shell) showSettings() {
	tr := s.service.Translator()
	cfg := s.service.Config()
	paths := s.service.Paths()

	morrowind := widget.NewEntry()
	morrowind.SetText(cfg.MorrowindPath)
	morrowind.SetPlaceHolder(".../Morrowind")

	workDir := widget.NewEntry()
	workDir.SetText(cfg.WorkDir)
	workDir.SetPlaceHolder(paths.DataDir)

	proton := widget.NewEntry()
	proton.SetText(cfg.ProtonPath)
	proton.SetPlaceHolder(".../Proton")

	hint := widget.NewLabel(tr.T(i18n.KeySettingsHint))
	hint.Wrapping = fyne.TextWrapWord
	items := []*widget.FormItem{
		widget.NewFormItem(tr.T(i18n.KeySettingsMorrowind), morrowind),
		widget.NewFormItem(tr.T(i18n.KeySettingsWorkDir), workDir),
	}
	if runtime.GOOS == "linux" {
		items = append(items, widget.NewFormItem(tr.T(i18n.KeySettingsProton), proton))
	}
	items = append(items, widget.NewFormItem("", hint))
	dialog.NewForm(
		tr.T(i18n.KeySettingsTitle),
		tr.T(i18n.KeyCommonSave),
		tr.T(i18n.KeyCommonCancel),
		items,
		func(ok bool) {
			if !ok {
				return
			}
			protonPath := cfg.ProtonPath
			if runtime.GOOS == "linux" {
				protonPath = proton.Text
			}
			settings := app.Settings{
				Language:      s.service.Config().Language,
				MorrowindPath: morrowind.Text,
				WorkDir:       workDir.Text,
				ProtonPath:    protonPath,
			}
			if err := s.service.UpdateSettings(settings); err != nil {
				s.showError(err)
				return
			}
			s.render()
		},
		s.window,
	).Show()
}
