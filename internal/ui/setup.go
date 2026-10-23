package ui

import (
	"fmt"

	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/tes3mp"
)

func progressFraction(update tes3mp.Progress) float64 {
	if update.Stage != tes3mp.StageDownloading || update.Total <= 0 {
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

func progressLabel(tr *i18n.Translator, update tes3mp.Progress) string {
	switch update.Stage {
	case tes3mp.StageDownloading:
		if update.Total > 0 {
			return tr.Tf(i18n.KeySetupDownloading,
				formatBytes(update.Downloaded), formatBytes(update.Total))
		}
		return tr.Tf(i18n.KeySetupDownloading, formatBytes(update.Downloaded),
			"?")
	case tes3mp.StageExtracting:
		return tr.T(i18n.KeySetupExtracting)
	case tes3mp.StageInstalling:
		return tr.T(i18n.KeySetupInstalling)
	default:
		return tr.T(i18n.KeySetupVerifying)
	}
}

func formatBytes(value int64) string {
	const (
		kib = 1024
		mib = 1024 * kib
		gib = 1024 * mib
	)
	switch {
	case value >= gib:
		return fmt.Sprintf("%.1f GiB", float64(value)/gib)
	case value >= mib:
		return fmt.Sprintf("%.1f MiB", float64(value)/mib)
	case value >= kib:
		return fmt.Sprintf("%.1f KiB", float64(value)/kib)
	default:
		return fmt.Sprintf("%d B", value)
	}
}
