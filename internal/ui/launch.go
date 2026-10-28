package ui

import (
	"context"
	"errors"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/Saagilat/tes3mp-easy/internal/app"
	"github.com/Saagilat/tes3mp-easy/internal/config"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/mods"
	"github.com/Saagilat/tes3mp-easy/internal/morrowind"
)

// playServer is the single primary action of a server card. It downloads or
// updates the mod pack when needed, writes the TES3MP endpoint and starts the
// game. Progress stays in one dialog; success is silent unless there are
// warnings. A second click while the game starts or runs is answered with a
// notice instead of a second TES3MP instance.
func (s *shell) playServer(profile config.Server) {
	tr := s.service.Translator()
	if _, running := s.service.GameRunning(); running {
		s.showNotice(tr.T(i18n.KeyCommonPlay),
			tr.T(i18n.KeyServerAlreadyRunning), nil)
		return
	}
	// The game folder is checked before mods are downloaded: a machine without
	// Morrowind must not spend a gigabyte of traffic to learn that it cannot
	// start, and the fix lives on the Installation screen.
	if _, err := s.service.CheckMorrowind(""); err != nil {
		s.showInstallFix(err)
		return
	}
	if err := s.service.SetActiveServer(profile.ID); err != nil {
		s.showErrorWithRetry(err, func() { s.playServer(profile) })
		return
	}
	if !s.beginPlay(profile.ID) {
		s.showNotice(tr.T(i18n.KeyCommonPlay),
			tr.T(i18n.KeyServerAlreadyStarting), nil)
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
		defer s.endPlay(profile.ID)
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
				if s.showInstallFix(err) {
					return
				}
				if notice, refused := launchRefusal(tr, err); refused {
					s.showNotice(tr.T(i18n.KeyCommonPlay), notice, nil)
					s.render()
					return
				}
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
		s.service.GameStopped()
		fyne.Do(func() {
			s.render()
			if waitErr != nil {
				s.showError(fmt.Errorf("%w: %v", app.ErrGameExited, waitErr))
			}
		})
	}()
}

// launchRefusal turns a rejected duplicate launch into a plain notice: nothing
// is broken, so there is no error dialog and nothing to retry.
func launchRefusal(tr *i18n.Translator, err error) (string, bool) {
	switch {
	case errors.Is(err, app.ErrLaunchInFlight):
		return tr.T(i18n.KeyServerAlreadyStarting), true
	case errors.Is(err, app.ErrGameRunning):
		return tr.T(i18n.KeyServerAlreadyRunning), true
	default:
		return "", false
	}
}

// showInstallFix sends a launch that lacks the game folder to the screen where
// it is fixed. It reports true when the error was this case, so the caller can
// skip an error dialog that would only repeat the same launch.
func (s *shell) showInstallFix(err error) bool {
	if !errors.Is(err, app.ErrMorrowindNotConfigured) &&
		!errors.Is(err, morrowind.ErrNotFound) &&
		!errors.Is(err, morrowind.ErrIncomplete) {
		return false
	}
	tr := s.service.Translator()
	s.showScreen(screenInstall)
	s.showNotice(tr.T(i18n.KeyInstallTitle),
		tr.T(i18n.KeyServerMorrowindRequired), nil)
	return true
}

// beginPlay marks a profile as launching. It reports false when the profile is
// already downloading mods or starting the game.
func (s *shell) beginPlay(serverID string) bool {
	s.playMu.Lock()
	defer s.playMu.Unlock()
	if s.playing == nil {
		s.playing = make(map[string]bool)
	}
	if s.playing[serverID] {
		return false
	}
	s.playing[serverID] = true
	return true
}

func (s *shell) endPlay(serverID string) {
	s.playMu.Lock()
	defer s.playMu.Unlock()
	delete(s.playing, serverID)
}

func (s *shell) modsInstalled(serverID string) bool {
	state, err := s.service.ModsState(serverID)
	return err == nil && state.Installed
}
