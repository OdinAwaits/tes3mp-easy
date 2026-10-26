package ui

import (
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Saagilat/tes3mp-easy/internal/app"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/ui/style"
)

// aboutScreen holds application information and the only interface language
// selector. The theme stays in the header and must not be duplicated here.
func (s *shell) aboutScreen(tr *i18n.Translator) fyne.CanvasObject {
	header := container.NewVBox(
		style.Heading(tr.T(i18n.KeyAboutTitle)),
		style.MutedLabel(tr.T(i18n.KeyAboutIntro)),
	)

	language := widget.NewSelect(interfaceLanguageLabels(),
		func(label string) {
			chosen, ok := interfaceLanguageByLabel(label)
			if !ok || chosen == s.service.Language() {
				return
			}
			settings := s.settings()
			settings.Language = string(chosen)
			if err := s.service.UpdateSettings(settings); err != nil {
				s.showError(err)
				return
			}
			s.render()
		})
	language.SetSelected(i18n.DisplayName(s.service.Language()))
	languageBox := container.NewGridWrap(fyne.NewSize(160, language.MinSize().Height),
		language)

	project, _ := url.Parse(app.ProjectURL)
	openPage := widget.NewButtonWithIcon(tr.T(i18n.KeyAboutOpenPage),
		theme.ComputerIcon(), func() {
			if fyne.CurrentApp() == nil {
				return
			}
			if err := fyne.CurrentApp().OpenURL(project); err != nil {
				s.showError(err)
			}
		})

	application := style.Card(container.NewVBox(
		style.Heading(tr.Tf(i18n.KeyAboutVersion, app.Version)),
		style.MutedLabel(tr.T(i18n.KeyAboutHint)),
		aboutLine(tr.T(i18n.KeyAboutInterfaceLanguage),
			tr.T(i18n.KeyAboutInterfaceLanguageHint), languageBox),
		aboutLine(tr.T(i18n.KeyAboutLicense),
			tr.T(i18n.KeyAboutLicenseValue), aboutBadge(tr.T(i18n.KeyAboutOpenSource),
				widget.LowImportance)),
		aboutLine(tr.T(i18n.KeyAboutProject), app.ProjectURL, openPage),
	), false)

	tes3mpState, err := s.service.TES3MPState()
	tes3mpBadge := aboutBadge(tr.T(i18n.KeyAboutNotInstalled),
		widget.WarningImportance)
	if err == nil && tes3mpState.Installed {
		tes3mpBadge = aboutBadge(tr.T(i18n.KeyAboutReady),
			widget.SuccessImportance)
	}
	components := style.Card(container.NewVBox(
		style.Subheading(tr.T(i18n.KeyAboutComponents)),
		aboutLine(tr.Tf(i18n.KeyAboutTES3MP, app.TES3MPVersion),
			tr.T(i18n.KeyAboutTES3MPHint), tes3mpBadge),
		aboutLine(tr.T(i18n.KeyAboutOpenMW), tr.T(i18n.KeyAboutOpenMWHint),
			aboutBadge(tr.T(i18n.KeyAboutReady), widget.SuccessImportance)),
		aboutLine(tr.T(i18n.KeyAboutFyne), tr.T(i18n.KeyAboutFyneHint),
			aboutBadge(tr.T(i18n.KeyAboutSystem), widget.LowImportance)),
		style.MutedLabel(tr.T(i18n.KeyAboutLegal)),
	), false)

	return container.NewVBox(header,
		container.NewGridWithColumns(2, application, components))
}

// aboutLine is a name/detail pair with an optional trailing control.
func aboutLine(name, detail string, trailing fyne.CanvasObject) fyne.CanvasObject {
	text := container.NewVBox(widget.NewLabel(name), style.MutedLabel(detail))
	if trailing == nil {
		return text
	}
	return container.NewBorder(nil, nil, nil, trailing, text)
}

func aboutBadge(text string, importance widget.Importance) *widget.Label {
	badge := widget.NewLabel(text)
	badge.Importance = importance
	return badge
}

// interfaceLanguageLabels returns language names in their own language, in the
// same order as i18n.Supported.
func interfaceLanguageLabels() []string {
	languages := i18n.Supported()
	labels := make([]string, 0, len(languages))
	for _, language := range languages {
		labels = append(labels, i18n.DisplayName(language))
	}
	return labels
}

// interfaceLanguageByLabel maps a selector label back to its identifier without
// relying on the current translator.
func interfaceLanguageByLabel(label string) (i18n.Language, bool) {
	for _, language := range i18n.Supported() {
		if i18n.DisplayName(language) == label {
			return language, true
		}
	}
	return "", false
}
