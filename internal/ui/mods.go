package ui

import (
	"context"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/Saagilat/tes3mp-easy/internal/config"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/mods"
)

func (s *shell) showCheckServer(profile config.Server) {
	tr := s.service.Translator()
	go func() {
		info, err := s.service.CheckServer(context.Background(), profile.ID)
		fyne.Do(func() {
			if err != nil {
				s.showError(err)
				return
			}
			modsSHA := "-"
			if info.ModsSHA256 != nil {
				modsSHA = shortSHA(*info.ModsSHA256)
			}
			dialog.ShowInformation(
				tr.T(i18n.KeyServerCheckTitle),
				tr.Tf(i18n.KeyServerCheckBody, info.Name, info.Address,
					info.Port, info.Status, modsSHA),
				s.window,
			)
		})
	}()
}

func (s *shell) showModsUpdate(profile config.Server) {
	tr := s.service.Translator()
	progress := widget.NewProgressBar()
	progress.Min = 0
	progress.Max = 1
	status := widget.NewLabel(tr.T(i18n.KeyModsChecking))
	status.Wrapping = fyne.TextWrapWord
	content := container.NewVBox(status, progress)
	dialog.NewCustom(tr.T(i18n.KeyModsTitle), tr.T(i18n.KeyCommonClose),
		content, s.window).Show()

	go func() {
		result, err := s.service.UpdateMods(context.Background(), profile.ID,
			func(update mods.Progress) {
				fyne.Do(func() {
					progress.SetValue(modsProgressFraction(update))
					status.SetText(modsProgressLabel(tr, update))
				})
			})
		fyne.Do(func() {
			if err != nil {
				progress.SetValue(0)
				status.SetText(s.failureBody(err))
				return
			}
			progress.SetValue(1)
			if result.Skipped {
				status.SetText(tr.T(i18n.KeyModsUpToDate))
				return
			}
			status.SetText(tr.T(i18n.KeyModsDone))
		})
	}()
}

func modsProgressFraction(update mods.Progress) float64 {
	if update.Stage != mods.StageDownloading || update.Total <= 0 {
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

func modsProgressLabel(tr *i18n.Translator, update mods.Progress) string {
	switch update.Stage {
	case mods.StageDownloading:
		if update.Total > 0 {
			return tr.Tf(i18n.KeyModsDownloading, formatBytes(update.Downloaded),
				formatBytes(update.Total))
		}
		return tr.Tf(i18n.KeyModsDownloading, formatBytes(update.Downloaded), "?")
	case mods.StageVerifying:
		return tr.T(i18n.KeyModsVerifying)
	case mods.StageExtracting:
		return tr.T(i18n.KeyModsExtracting)
	case mods.StageInstalling:
		return tr.T(i18n.KeyModsInstalling)
	default:
		return tr.T(i18n.KeyModsChecking)
	}
}

func shortSHA(value string) string {
	if len(value) <= 12 {
		return value
	}
	return fmt.Sprintf("%s…", value[:12])
}
