package ui

import (
	"context"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/Saagilat/tes3mp-easy/internal/config"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/locale"
)

func (s *shell) showLocalization(profile config.Server, install, voiceover bool) {
	tr := s.service.Translator()
	status := widget.NewLabel(tr.T(i18n.KeyLocaleChecking))
	status.Wrapping = fyne.TextWrapWord
	progress := widget.NewProgressBar()
	progress.Min = 0
	progress.Max = 1
	content := container.NewVBox(status, progress)
	popup := dialog.NewCustom(tr.T(i18n.KeyLocaleTitle),
		tr.T(i18n.KeyCommonClose), content, s.window)
	popup.Show()

	go func() {
		if !install {
			warnings, err := s.service.RemoveLocalization(profile.ID)
			fyne.Do(func() {
				popup.Hide()
				s.render()
				if err != nil {
					s.showError(err)
					return
				}
				s.showNotice(tr.T(i18n.KeyLocaleRemoved),
					tr.T(i18n.KeyLocaleRemoved), warnings)
			})
			return
		}
		result, err := s.service.InstallLocalization(context.Background(),
			profile.ID, voiceover, func(update locale.Progress) {
				fyne.Do(func() {
					progress.SetValue(localeProgressFraction(update))
					status.SetText(localeProgressLabel(tr, update))
				})
			})
		fyne.Do(func() {
			popup.Hide()
			s.render()
			if err != nil {
				s.showErrorWithRetry(err, func() {
					s.showLocalization(profile, install, voiceover)
				})
				return
			}
			progress.SetValue(1)
			s.showNotice(tr.T(i18n.KeyLocaleDone), tr.T(i18n.KeyLocaleDone),
				result.Warnings)
		})
	}()
}

func (s *shell) showNotice(title, body string, warnings []string) {
	if len(warnings) > 0 {
		body += "\n\n" + strings.Join(warnings, "\n")
	}
	dialog.ShowInformation(title, body, s.window)
}

func localeProgressFraction(update locale.Progress) float64 {
	if update.Stage != locale.StageDownloading || update.Total <= 0 {
		return 1
	}
	value := float64(update.Downloaded) / float64(update.Total)
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func localeProgressLabel(tr *i18n.Translator, update locale.Progress) string {
	switch update.Stage {
	case locale.StageDownloading:
		if update.Total > 0 {
			return tr.Tf(i18n.KeyLocaleDownloading,
				formatBytes(update.Downloaded), formatBytes(update.Total))
		}
		return tr.Tf(i18n.KeyLocaleDownloading, formatBytes(update.Downloaded), "?")
	case locale.StageExtracting:
		return tr.T(i18n.KeyLocaleExtracting)
	case locale.StageInstalling:
		return tr.T(i18n.KeyLocaleInstalling)
	default:
		return tr.T(i18n.KeyLocaleChecking)
	}
}
