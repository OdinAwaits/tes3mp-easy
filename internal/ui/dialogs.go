package ui

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/Saagilat/tes3mp-easy/internal/config"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/locale"
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

	profile, hasProfile := activeProfile(cfg)
	localizationState := locale.State{}
	if hasProfile {
		if state, err := s.service.LocalizationState(profile.ID); err == nil {
			localizationState = state
		}
	}

	localization := widget.NewCheck(tr.T(i18n.KeySettingsLocalization), nil)
	localization.SetChecked(cfg.Localization)
	voiceover := widget.NewCheck(tr.T(i18n.KeySettingsVoiceover), nil)
	voiceover.SetChecked(localizationState.Voiceover)
	localizationStatus := widget.NewLabel("")
	localizationStatus.Wrapping = fyne.TextWrapWord
	if !hasProfile {
		localization.Disable()
		voiceover.Disable()
		localizationStatus.SetText(tr.T(i18n.KeySettingsNoServer))
	} else {
		localizationStatus.SetText(tr.T(i18n.KeySettingsLocalizationHint))
	}
	syncLocalization := func(enabled bool) {
		if enabled && hasProfile {
			voiceover.Enable()
			return
		}
		voiceover.Disable()
	}
	localization.OnChanged = syncLocalization
	syncLocalization(localization.Checked)

	fontsHint := widget.NewLabel(tr.T(i18n.KeySettingsFontsHint))
	fontsHint.Wrapping = fyne.TextWrapWord
	logHint := widget.NewLabel(tr.T(i18n.KeySettingsLogHint))
	logHint.Wrapping = fyne.TextWrapWord
	openLog := widget.NewButton(tr.T(i18n.KeySettingsOpenLog), s.showLogs)
	hint := widget.NewLabel(tr.T(i18n.KeySettingsHint))
	hint.Wrapping = fyne.TextWrapWord
	items := []*widget.FormItem{
		widget.NewFormItem(tr.T(i18n.KeySettingsMorrowind), morrowind),
		widget.NewFormItem(tr.T(i18n.KeySettingsWorkDir), workDir),
	}
	if runtime.GOOS == "linux" {
		items = append(items, widget.NewFormItem(tr.T(i18n.KeySettingsProton), proton))
	}
	items = append(items,
		widget.NewFormItem(tr.T(i18n.KeySettingsLocalization), localization),
		widget.NewFormItem(tr.T(i18n.KeySettingsVoiceover), voiceover),
		widget.NewFormItem("", localizationStatus),
		widget.NewFormItem("", fontsHint),
		widget.NewFormItem(tr.T(i18n.KeySettingsOpenLog), openLog),
		widget.NewFormItem("", logHint),
		widget.NewFormItem("", hint),
	)
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
			settings := s.settings()
			settings.MorrowindPath = morrowind.Text
			settings.WorkDir = workDir.Text
			settings.ProtonPath = protonPath
			if err := s.service.UpdateSettings(settings); err != nil {
				s.showError(err)
				return
			}
			s.render()
			if !hasProfile {
				return
			}
			enabled := localization.Checked
			withVoices := enabled && voiceover.Checked
			switch {
			case !enabled && (cfg.Localization || localizationState.Installed):
				s.showLocalization(profile, false, false)
			case enabled && (!localizationState.Installed ||
				localizationState.Voiceover != withVoices || !cfg.Localization):
				s.showLocalization(profile, true, withVoices)
			}
		},
		s.window,
	).Show()
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
