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

type failureCopy struct {
	what   string
	action string
}

var failureCopies = map[app.FailureKind]failureCopy{
	app.FailureMorrowindNotFound: {
		what:   i18n.KeyErrorMorrowindNotFound,
		action: i18n.KeyErrorMorrowindNotFoundAct,
	},
	app.FailureMorrowindIncomplete: {
		what:   i18n.KeyErrorMorrowindIncomplete,
		action: i18n.KeyErrorMorrowindIncompleteAct,
	},
	app.FailureMorrowindNotConfigured: {
		what:   i18n.KeyErrorMorrowindMissing,
		action: i18n.KeyErrorMorrowindMissingAct,
	},
	app.FailureNetwork: {
		what:   i18n.KeyErrorNetwork,
		action: i18n.KeyErrorNetworkAct,
	},
	app.FailureTimeout: {
		what:   i18n.KeyErrorTimeout,
		action: i18n.KeyErrorTimeoutAct,
	},
	app.FailureCertificate: {
		what:   i18n.KeyErrorCertificate,
		action: i18n.KeyErrorCertificateAct,
	},
	app.FailureNoMods: {
		what:   i18n.KeyErrorNoMods,
		action: i18n.KeyErrorNoModsAct,
	},
	app.FailureServerAPI: {
		what:   i18n.KeyErrorServerAPI,
		action: i18n.KeyErrorServerAPIAct,
	},
	app.FailureFileSystem: {
		what:   i18n.KeyErrorFileSystem,
		action: i18n.KeyErrorFileSystemAct,
	},
	app.FailureWorkDir: {
		what:   i18n.KeyErrorWorkDir,
		action: i18n.KeyErrorWorkDirAct,
	},
	app.FailureModsNotInstalled: {
		what:   i18n.KeyErrorModsMissing,
		action: i18n.KeyErrorModsMissingAct,
	},
	app.FailureLocalization: {
		what:   i18n.KeyErrorLocaleMissing,
		action: i18n.KeyErrorLocaleMissingAct,
	},
	app.FailureGameMissing: {
		what:   i18n.KeyErrorGameMissing,
		action: i18n.KeyErrorGameMissingAct,
	},
	app.FailureGameProton: {
		what:   i18n.KeyErrorGameProton,
		action: i18n.KeyErrorGameProtonAct,
	},
	app.FailureGameConfig: {
		what:   i18n.KeyErrorGameConfig,
		action: i18n.KeyErrorGameConfigAct,
	},
	app.FailureGameStart: {
		what:   i18n.KeyErrorGameStart,
		action: i18n.KeyErrorGameStartAct,
	},
	app.FailureGameExited: {
		what:   i18n.KeyErrorGameExited,
		action: i18n.KeyErrorGameExitedAct,
	},
	app.FailureGameStarting: {
		what:   i18n.KeyErrorGameStarting,
		action: i18n.KeyErrorGameStartingAct,
	},
	app.FailureGameRunning: {
		what:   i18n.KeyErrorGameRunning,
		action: i18n.KeyErrorGameRunningAct,
	},
	app.FailureUnknown: {
		what:   i18n.KeyErrorUnknown,
		action: i18n.KeyErrorUnknownAct,
	},
}

func failureCopyFor(kind app.FailureKind) failureCopy {
	if copy, ok := failureCopies[kind]; ok {
		return copy
	}
	return failureCopies[app.FailureUnknown]
}

// failureSummary is the one-line explanation used by inline background status.
func (s *shell) failureSummary(err error) string {
	failure := app.Explain(err)
	return s.service.Translator().T(failureCopyFor(failure.Kind).what)
}

// failureBody turns a classified error into the what/action/details/log text
// shown by compact inline surfaces. The dialog uses the richer layout below.
func (s *shell) failureBody(err error) string {
	tr := s.service.Translator()
	failure := app.Explain(err)
	copy := failureCopyFor(failure.Kind)
	lines := []string{
		tr.Tf(i18n.KeyErrorCode, failure.Code),
		tr.Tf(i18n.KeyErrorWhat, tr.T(copy.what)),
		tr.Tf(i18n.KeyErrorActionLine, tr.T(copy.action)),
	}
	if detail := strings.TrimSpace(failure.Detail); detail != "" {
		lines = append(lines, tr.Tf(i18n.KeyErrorDetail, detail))
	}
	if path := s.service.LogPath(); path != "" {
		lines = append(lines, tr.Tf(i18n.KeyErrorLogLine, path))
	}
	return strings.Join(lines, "\n\n")
}

// showError replaces the raw error dialog with a classified explanation.
func (s *shell) showError(err error) {
	s.showErrorWithRetry(err, nil)
}

// showErrorWithRetry adds an in-place Retry action for operations that can be
// repeated without changing their inputs.
func (s *shell) showErrorWithRetry(err error, retry func()) {
	if err == nil {
		return
	}
	s.service.LogError("error dialog", err)
	tr := s.service.Translator()
	failure := app.Explain(err)
	copy := failureCopyFor(failure.Kind)

	code := widget.NewLabelWithStyle(string(failure.Code),
		fyne.TextAlignLeading, fyne.TextStyle{Bold: true, Monospace: true})
	code.Importance = widget.DangerImportance
	what := widget.NewLabel(tr.T(copy.what))
	what.Wrapping = fyne.TextWrapWord
	what.TextStyle = fyne.TextStyle{Bold: true}
	action := style.MutedLabel(tr.Tf(i18n.KeyErrorActionLine, tr.T(copy.action)))

	detailText := strings.TrimSpace(failure.Detail)
	if detailText == "" {
		detailText = tr.T(i18n.KeyErrorNoDetails)
	}
	details := widget.NewLabelWithStyle(detailText, fyne.TextAlignLeading,
		fyne.TextStyle{Monospace: true})
	details.Wrapping = fyne.TextWrapWord
	details.Selectable = true
	detailsScroll := container.NewVScroll(details)
	detailsScroll.SetMinSize(fyne.NewSize(600, 180))
	accordion := widget.NewAccordion(widget.NewAccordionItem(
		tr.T(i18n.KeyErrorShowDetails), detailsScroll))

	copyStatus := widget.NewLabel("")
	copyStatus.Wrapping = fyne.TextWrapWord
	copyStatus.Importance = widget.SuccessImportance
	copyStatus.Hide()

	content := container.NewVBox(
		code,
		what,
		action,
		accordion,
		copyStatus,
	)

	popup := dialog.NewCustomWithoutButtons(tr.T(i18n.KeyCommonError),
		content, s.window)
	buttons := []fyne.CanvasObject{}
	if retry != nil {
		retryButton := widget.NewButton(tr.T(i18n.KeyErrorRetry), func() {
			popup.Hide()
			retry()
		})
		retryButton.Importance = widget.HighImportance
		buttons = append(buttons, retryButton)
	}
	buttons = append(buttons,
		widget.NewButton(tr.T(i18n.KeyErrorOpenLog), func() {
			popup.Hide()
			s.showLogs()
		}),
		widget.NewButton(tr.T(i18n.KeyErrorCopyReport), func() {
			lines, _ := s.service.ReadLogTail(app.TailLines)
			report := s.service.DiagnosticReport(err, lines)
			fyne.CurrentApp().Clipboard().SetContent(report)
			copyStatus.SetText(tr.T(i18n.KeyErrorReportCopied))
			copyStatus.Show()
		}),
	)
	if profile, ok := activeProfile(s.service.Config()); ok {
		if failureNeedsPanel(failure.Kind) {
			buttons = append(buttons, widget.NewButton(tr.T(i18n.KeyServerOpenPanel), func() {
				popup.Hide()
				s.openPanel(profile)
			}))
		}
		if failureNeedsMods(failure.Kind) {
			buttons = append(buttons, widget.NewButton(tr.T(i18n.KeyServerUpdateMods), func() {
				popup.Hide()
				s.showModsUpdate(profile)
			}))
		}
	}
	if failureNeedsSettings(failure.Kind) {
		buttons = append(buttons, widget.NewButton(tr.T(i18n.KeyRailInstall), func() {
			popup.Hide()
			s.showScreen(screenInstall)
		}))
	}
	if failure.Kind == app.FailureGameMissing {
		buttons = append(buttons, widget.NewButton(tr.T(i18n.KeyCommonSetup), func() {
			popup.Hide()
			s.showSetup()
		}))
	}
	closeButton := widget.NewButton(tr.T(i18n.KeyCommonClose), popup.Hide)
	footer := container.NewBorder(nil, nil, nil, closeButton,
		container.NewHBox(buttons...))
	popup.SetButtons([]fyne.CanvasObject{footer})
	popup.Resize(fyne.NewSize(700, 500))
	popup.Show()
}

func failureNeedsSettings(kind app.FailureKind) bool {
	switch kind {
	case app.FailureMorrowindNotFound, app.FailureMorrowindIncomplete,
		app.FailureMorrowindNotConfigured, app.FailureWorkDir,
		app.FailureLocalization, app.FailureGameProton:
		return true
	default:
		return false
	}
}

func failureNeedsPanel(kind app.FailureKind) bool {
	switch kind {
	case app.FailureNetwork, app.FailureTimeout, app.FailureCertificate,
		app.FailureServerAPI, app.FailureNoMods, app.FailureModsNotInstalled:
		return true
	default:
		return false
	}
}

func failureNeedsMods(kind app.FailureKind) bool {
	return kind == app.FailureNoMods || kind == app.FailureModsNotInstalled
}
