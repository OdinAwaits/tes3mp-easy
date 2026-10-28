package ui

import (
	"context"
	"path/filepath"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/ui/style"
)

// editorScreen is the OpenMW-CS section. The editor is not bundled with
// TES3MP, so a missing executable is an explained state instead of an error,
// and launching waits for the active server's mods because the editor opens
// the same data as the game.
func (s *shell) editorScreen(tr *i18n.Translator) fyne.CanvasObject {
	state, err := s.service.EditorState()
	profile, hasServer := activeProfile(s.service.Config())
	modsReady := hasServer && s.modsInstalled(profile.ID)

	status := installStatus{
		summary:    tr.T(i18n.KeyCSMissing),
		detail:     tr.Tf(i18n.KeyCSExpected, state.Path),
		importance: widget.WarningImportance,
	}
	switch {
	case err != nil:
		status.summary = tr.Tf(i18n.KeySetupMorrowindIncomplete,
			s.failureSummary(err))
	case state.Found:
		status = installStatus{
			summary:    tr.Tf(i18n.KeyCSFound, state.Path),
			importance: widget.SuccessImportance,
		}
	}

	launch := style.PrimaryIconButton(tr.T(i18n.KeyCSLaunch),
		theme.MediaPlayIcon(), func() {
			s.launchEditor(profile.ID)
		})
	checkAgain := style.SecondaryIconButton(tr.T(i18n.KeyCSCheckAgain),
		theme.ViewRefreshIcon(), func() {
			s.showScreen(screenCS)
		})
	openFolder := style.SecondaryIconButton(tr.T(i18n.KeyCSOpenFolder),
		theme.FolderOpenIcon(), func() {
			s.openFolder(filepath.Dir(state.Path))
		})

	hintText := ""
	switch {
	case err != nil:
		launch.Disable()
	case !state.Found:
		launch.Disable()
	case !hasServer:
		hintText = tr.T(i18n.KeyCSNoServer)
		launch.Disable()
	case !modsReady:
		hintText = tr.T(i18n.KeyCSModsRequired)
		launch.Disable()
	}

	lines := []fyne.CanvasObject{
		style.Heading(tr.T(i18n.KeyCSHeading)),
		style.MutedLabel(tr.T(i18n.KeyCSIntro)),
		installRow(tr.T(i18n.KeyCSEditor), status),
	}
	if hintText != "" {
		lines = append(lines, style.MutedLabel(hintText))
	}
	lines = append(lines, container.NewHBox(launch, checkAgain, openFolder))
	if runtime.GOOS == "linux" {
		lines = append(lines, style.MutedLabel(tr.T(i18n.KeyCSProtonNote)))
	}
	return container.NewVBox(lines...)
}

// launchEditor starts OpenMW-CS with the same profile preparation as Play.
func (s *shell) launchEditor(serverID string) {
	tr := s.service.Translator()
	go func() {
		result, err := s.service.OpenMWCS(context.Background(), serverID)
		fyne.Do(func() {
			if err != nil {
				if notice, refused := launchRefusal(tr, err); refused {
					s.showNotice(tr.T(i18n.KeyCSHeading), notice, nil)
					return
				}
				s.showErrorWithRetry(err, func() { s.launchEditor(serverID) })
				return
			}
			s.showNotice(tr.T(i18n.KeyCSHeading), tr.T(i18n.KeyCSStarted),
				result.Warnings)
		})
	}()
}
