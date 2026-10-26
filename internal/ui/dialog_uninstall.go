package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/Saagilat/tes3mp-easy/internal/app"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/ui/style"
)

// showUninstallDialog is the Installation danger zone. Game files and the
// download cache are pre-selected; client settings stay unless the user opts
// in, because removing them also removes every server profile.
func (s *shell) showUninstallDialog() {
	tr := s.service.Translator()
	gameFiles := widget.NewCheck(tr.T(i18n.KeyUninstallGameFiles), nil)
	gameFiles.SetChecked(true)
	cache := widget.NewCheck(tr.T(i18n.KeyUninstallCache), nil)
	cache.SetChecked(true)
	settings := widget.NewCheck(tr.T(i18n.KeyUninstallSettings), nil)

	content := container.NewVBox(
		style.MutedLabel(tr.T(i18n.KeyUninstallHint)),
		gameFiles,
		cache,
		settings,
		style.MutedLabel(tr.T(i18n.KeyUninstallKeepMorrowind)),
	)
	popup := dialog.NewCustomConfirm(tr.T(i18n.KeyUninstallTitle),
		tr.T(i18n.KeyUninstallRemove), tr.T(i18n.KeyCommonCancel), content,
		func(ok bool) {
			if !ok {
				return
			}
			scope := app.UninstallScope{
				GameFiles: gameFiles.Checked,
				Cache:     cache.Checked,
				Settings:  settings.Checked,
			}
			if scope.Empty() {
				dialog.ShowInformation(tr.T(i18n.KeyUninstallTitle),
					tr.T(i18n.KeyUninstallEmpty), s.window)
				return
			}
			s.runUninstall(scope)
		}, s.window)
	popup.Resize(fyne.NewSize(640, 360))
	popup.Show()
}

// runUninstall deletes the selected folders in the background: the work
// directory can hold several gigabytes of game files.
func (s *shell) runUninstall(scope app.UninstallScope) {
	tr := s.service.Translator()
	progress := widget.NewProgressBarInfinite()
	progress.Start()
	busy := dialog.NewCustomWithoutButtons(tr.T(i18n.KeyUninstallRunning),
		container.NewVBox(progress), s.window)
	busy.Show()

	go func() {
		result, err := s.service.Uninstall(scope)
		fyne.Do(func() {
			progress.Stop()
			busy.Hide()
			if err != nil {
				s.showErrorWithRetry(err, func() { s.runUninstall(scope) })
				return
			}
			s.finishUninstall(result)
		})
	}()
}

// finishUninstall reports the removed folders. Removing the settings leaves the
// running client without a configuration, so it closes after the notice.
func (s *shell) finishUninstall(result app.UninstallResult) {
	tr := s.service.Translator()
	if result.ConfigRemoved {
		notice := dialog.NewInformation(tr.T(i18n.KeyUninstallTitle),
			tr.T(i18n.KeyUninstallRestart), s.window)
		notice.SetOnClosed(func() { fyne.CurrentApp().Quit() })
		notice.Show()
		return
	}
	if len(result.Removed) > 0 {
		dialog.ShowInformation(tr.T(i18n.KeyUninstallTitle),
			tr.Tf(i18n.KeyUninstallDone, strings.Join(result.Removed, "\n")),
			s.window)
	}
	s.showScreen(screenInstall)
}
