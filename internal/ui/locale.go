package ui

import (
	"strings"

	"fyne.io/fyne/v2/dialog"

	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/locale"
)

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
