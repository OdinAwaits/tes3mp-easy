package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/Saagilat/tes3mp-easy/internal/app"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
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
	app.FailureUnknown: {
		what:   i18n.KeyErrorUnknown,
		action: i18n.KeyErrorUnknownAct,
	},
}

// failureBody turns a classified error into the what/action/details/log text
// shown by every error surface. The log path is included even when the log
// could not be opened, because it still tells the user where to look.
func (s *shell) failureBody(err error) string {
	tr := s.service.Translator()
	failure := app.Explain(err)
	copy, ok := failureCopies[failure.Kind]
	if !ok {
		copy = failureCopies[app.FailureUnknown]
	}
	lines := []string{
		tr.Tf(i18n.KeyErrorWhat, tr.T(copy.what)),
		tr.Tf(i18n.KeyErrorActionLine, tr.T(copy.action)),
	}
	if detail := strings.TrimSpace(failure.Detail); detail != "" {
		const limit = 700
		if len(detail) > limit {
			detail = detail[:limit] + "…"
		}
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
	body := widget.NewLabel(s.failureBody(err))
	body.Wrapping = fyne.TextWrapWord

	content := container.NewVBox(body)
	var popup dialog.Dialog
	if retry != nil {
		content.Add(container.NewHBox(widget.NewButton(tr.T(i18n.KeyErrorRetry), func() {
			popup.Hide()
			retry()
		})))
	}
	popup = dialog.NewCustom(tr.T(i18n.KeyCommonError), tr.T(i18n.KeyCommonClose),
		content, s.window)
	popup.Resize(fyne.NewSize(640, 340))
	popup.Show()
}
