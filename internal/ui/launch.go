package ui

import (
	"context"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/Saagilat/tes3mp-easy/internal/app"
	"github.com/Saagilat/tes3mp-easy/internal/config"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/mods"
)

// playServer is the single primary action of a server card. It downloads or
// updates the mod pack when needed, writes the TES3MP endpoint and starts the
// game. Progress stays in one dialog; success is silent unless there are
// warnings.
func (s *shell) playServer(profile config.Server) {
	tr := s.service.Translator()
	if err := s.service.SetActiveServer(profile.ID); err != nil {
		s.showErrorWithRetry(err, func() { s.playServer(profile) })
		return
	}

	status := widget.NewLabel(tr.T(i18n.KeyServerPreparing))
	status.Wrapping = fyne.TextWrapWord
	progress := widget.NewProgressBar()
	progress.Min = 0
	progress.Max = 1
	content := container.NewVBox(status, progress)
	popup := dialog.NewCustom(tr.T(i18n.KeyCommonPlay),
		tr.T(i18n.KeyCommonClose), content, s.window)
	popup.Resize(fyne.NewSize(560, 180))
	popup.Show()

	go func() {
		ctx := context.Background()
		warnings := []string{}
		needsUpdate := !s.modsInstalled(profile.ID) ||
			s.statusFor(profile.ID).NeedsUpdate
		if needsUpdate {
			fyne.Do(func() {
				status.SetText(tr.T(i18n.KeyModsChecking))
				progress.SetValue(0)
			})
			result, err := s.service.UpdateMods(ctx, profile.ID,
				func(update mods.Progress) {
					fyne.Do(func() {
						progress.SetValue(modsProgressFraction(update))
						status.SetText(modsProgressLabel(tr, update))
					})
				})
			if err != nil {
				fyne.Do(func() {
					popup.Hide()
					s.showErrorWithRetry(err, func() { s.playServer(profile) })
				})
				return
			}
			warnings = append(warnings, result.Warnings...)
		}

		fyne.Do(func() {
			status.SetText(tr.T(i18n.KeyServerLaunching))
			progress.SetValue(0)
		})
		result, err := s.service.Launch(ctx, profile.ID)
		if err != nil {
			fyne.Do(func() {
				popup.Hide()
				s.showErrorWithRetry(err, func() { s.playServer(profile) })
			})
			return
		}
		warnings = append(warnings, result.Warnings...)
		fyne.Do(func() {
			popup.Hide()
			s.render()
			if len(warnings) > 0 {
				s.showNotice(tr.T(i18n.KeyCommonPlay), tr.T(i18n.KeyServerLaunched), warnings)
			}
		})
		if result.Process == nil {
			return
		}
		waitErr := result.Process.Wait()
		if waitErr == nil {
			return
		}
		fyne.Do(func() {
			s.showError(fmt.Errorf("%w: %v", app.ErrGameExited, waitErr))
		})
	}()
}

func (s *shell) modsInstalled(serverID string) bool {
	state, err := s.service.ModsState(serverID)
	return err == nil && state.Installed
}
