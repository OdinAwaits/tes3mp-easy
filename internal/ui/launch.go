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
)

func (s *shell) showLaunch(profile config.Server) {
	tr := s.service.Translator()
	status := widget.NewLabel(tr.T(i18n.KeyServerLaunching))
	status.Wrapping = fyne.TextWrapWord
	progress := widget.NewProgressBarInfinite()
	progress.Start()
	popup := dialog.NewCustom(tr.T(i18n.KeyCommonPlay),
		tr.T(i18n.KeyCommonClose), container.NewVBox(status, progress),
		s.window)
	popup.Show()

	go func() {
		result, err := s.service.Launch(context.Background(), profile.ID)
		fyne.Do(func() {
			progress.Stop()
			popup.Hide()
			s.render()
			if err != nil {
				s.showErrorWithRetry(err, func() { s.showLaunch(profile) })
				return
			}
			s.showNotice(tr.T(i18n.KeyCommonPlay),
				tr.T(i18n.KeyServerLaunched), result.Warnings)
		})
		if err != nil || result.Process == nil {
			return
		}
		waitErr := result.Process.Wait()
		fyne.Do(func() {
			if waitErr != nil {
				s.showError(fmt.Errorf("%s: %w",
					tr.T(i18n.KeyServerExited), waitErr))
				return
			}
			s.showGameExited()
		})
	}()
}

func (s *shell) showGameExited() {
	tr := s.service.Translator()
	hint := widget.NewLabel(tr.T(i18n.KeyLogsExitHint))
	hint.Wrapping = fyne.TextWrapWord
	open := widget.NewButton(tr.T(i18n.KeyLogsTitle), s.showLogs)
	content := container.NewVBox(hint, container.NewHBox(open))
	dialog.NewCustom(tr.T(i18n.KeyCommonPlay), tr.T(i18n.KeyCommonClose),
		content, s.window).Show()
}

func (s *shell) showAdvanced(profile config.Server) {
	tr := s.service.Translator()
	hint := widget.NewLabel(tr.T(i18n.KeyAdvancedCSHint))
	hint.Wrapping = fyne.TextWrapWord
	var openCS *widget.Button
	openCS = widget.NewButton(tr.T(i18n.KeyAdvancedOpenCS), func() {
		openCS.Disable()
		go func() {
			result, err := s.service.OpenMWCS(context.Background(), profile.ID)
			fyne.Do(func() {
				openCS.Enable()
				if err != nil {
					s.showError(fmt.Errorf("%s: %w",
						tr.T(i18n.KeyAdvancedFailed), err))
					return
				}
				s.showNotice(tr.T(i18n.KeyAdvancedTitle),
					tr.T(i18n.KeyAdvancedStarted), result.Warnings)
			})
		}()
	})
	content := container.NewVBox(hint, openCS)
	dialog.NewCustom(tr.T(i18n.KeyAdvancedTitle), tr.T(i18n.KeyCommonClose),
		content, s.window).Show()
}
