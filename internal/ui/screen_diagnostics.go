package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Saagilat/tes3mp-easy/internal/app"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/ui/style"
)

// diagnosticsScreen is the single home for the log, the support report and the
// paths users may need to inspect. Installation links here instead of keeping
// a copy of each action.
func (s *shell) diagnosticsScreen(tr *i18n.Translator) fyne.CanvasObject {
	header := container.NewVBox(
		style.Heading(tr.T(i18n.KeyDiagnosticsTitle)),
		style.MutedLabel(tr.T(i18n.KeyDiagnosticsIntro)),
	)

	checks := container.NewGridWithColumns(2,
		diagnosticLine(tr.T(i18n.KeyInstallMorrowind), s.morrowindStatus(tr)),
		diagnosticLine(tr.T(i18n.KeyInstallTES3MP), s.tes3mpStatus(tr)),
		diagnosticLine(tr.T(i18n.KeyInstallFonts), s.fontsStatus(tr)),
		diagnosticLine(tr.T(i18n.KeyInstallLanguage), s.localizationStatus(tr)),
	)
	copyReport := style.PrimaryIconButton(tr.T(i18n.KeyDiagnosticsCopyReport),
		theme.ContentCopyIcon(), s.copyReport)
	report := style.Card(container.NewVBox(
		style.Subheading(tr.T(i18n.KeyDiagnosticsReport)),
		style.MutedLabel(tr.T(i18n.KeyDiagnosticsReportHint)),
		container.NewHBox(
			aboutBadge(tr.T(i18n.KeyDiagnosticsReportPasswords),
				widget.LowImportance),
			aboutBadge(tr.T(i18n.KeyDiagnosticsReportLocal),
				widget.SuccessImportance),
		),
		checks,
		container.NewHBox(copyReport),
	), false)

	return container.NewVBox(header, report, s.diagnosticsLog(tr),
		s.diagnosticsFolders(tr))
}

func diagnosticLine(label string, status installStatus) fyne.CanvasObject {
	summary := widget.NewLabel(status.summary)
	summary.Wrapping = fyne.TextWrapWord
	summary.Importance = status.importance
	return container.NewVBox(style.MutedLabel(label), summary)
}

// diagnosticsLog shows the bounded tail inline; no dialog is needed and the
// Refresh button simply rebuilds this screen.
func (s *shell) diagnosticsLog(tr *i18n.Translator) fyne.CanvasObject {
	logText := widget.NewLabelWithStyle("", fyne.TextAlignLeading,
		fyne.TextStyle{Monospace: true})
	logText.Wrapping = fyne.TextWrapWord
	lines, err := s.service.ReadLogTail(app.TailLines)
	switch {
	case err != nil:
		logText.SetText(s.failureBody(err))
	case len(lines) == 0:
		logText.SetText(tr.T(i18n.KeyLogsEmpty))
	default:
		logText.SetText(strings.Join(lines, "\n"))
	}
	scroll := container.NewVScroll(logText)
	scroll.SetMinSize(fyne.NewSize(760, 220))

	refresh := widget.NewButtonWithIcon(tr.T(i18n.KeyLogsRefresh),
		theme.ViewRefreshIcon(), func() {
			s.showScreen(screenDiagnostics)
		})
	open := widget.NewButtonWithIcon(tr.T(i18n.KeyLogsOpenFolder),
		theme.FolderOpenIcon(), s.openLogFolder)
	hint := style.MutedLabel(tr.Tf(i18n.KeyLogsHint, app.TailLines))

	return style.Card(container.NewBorder(
		container.NewVBox(style.Subheading(tr.T(i18n.KeyLogsTitle)), hint),
		container.NewHBox(open, refresh),
		nil, nil, scroll,
	), false)
}

func (s *shell) diagnosticsFolders(tr *i18n.Translator) fyne.CanvasObject {
	paths := s.service.Paths()
	workDir := paths.DataDir
	if strings.TrimSpace(s.service.Config().WorkDir) != "" {
		workDir = s.service.Config().WorkDir
	}
	rows := container.NewGridWithColumns(2,
		s.diagnosticFolderRow(tr.T(i18n.KeyDiagnosticsFolderWorkDir), workDir),
		s.diagnosticFolderRow(tr.T(i18n.KeySettingsProton),
			s.service.Config().ProtonPath),
		s.diagnosticFolderRow(tr.T(i18n.KeyDiagnosticsFolderTES3MP),
			s.tes3mpFolder()),
		s.diagnosticFolderRow(tr.T(i18n.KeyDiagnosticsFolderOpenMW),
			s.openMWConfigFolder()),
		s.diagnosticFolderRow(tr.T(i18n.KeyDiagnosticsFolderLog),
			s.logFolder()),
	)
	return style.Card(container.NewVBox(
		style.Subheading(tr.T(i18n.KeyDiagnosticsFolders)),
		style.MutedLabel(tr.T(i18n.KeyDiagnosticsFoldersHint)),
		rows,
	), false)
}

func (s *shell) diagnosticFolderRow(title, path string) fyne.CanvasObject {
	open := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
		s.openFolder(path)
	})
	open.Importance = widget.LowImportance
	return container.NewBorder(nil, nil, nil, open,
		container.NewVBox(widget.NewLabel(title), pathLabel(path)))
}
