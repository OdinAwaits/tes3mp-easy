package ui

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Saagilat/tes3mp-easy/internal/app"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/tes3mp"
	"github.com/Saagilat/tes3mp-easy/internal/ui/style"
)

// installStatus is the one-line verdict of an installation row.
type installStatus struct {
	summary    string
	detail     string
	importance widget.Importance
	ready      bool
}

// installScreen is the checklist that replaced the one-time setup wizard. Every
// row is built from an existing app method, so nothing is written to disk until
// the user presses a row action.
func (s *shell) installScreen(tr *i18n.Translator) fyne.CanvasObject {
	morrowindState := s.morrowindStatus(tr)
	tes3mpState := s.tes3mpStatus(tr)
	fontsState := s.fontsStatus(tr)
	localizationState := s.localizationStatus(tr)
	uiVideoState := s.uiVideoStatus(tr)
	s.issues = countIssues(morrowindState, tes3mpState, fontsState,
		localizationState, uiVideoState)

	runWizard := widget.NewButtonWithIcon(tr.T(i18n.KeyInstallRunWizard),
		theme.DocumentCreateIcon(), s.showSetup)
	runWizard.Importance = widget.HighImportance

	tes3mpInstall := tr.T(i18n.KeyInstallInstall)
	if s.tes3mpInstalled() {
		tes3mpInstall = tr.T(i18n.KeyInstallReinstall)
	}

	localizationActions := []fyne.CanvasObject{
		widget.NewButton(tr.T(i18n.KeyInstallInstall), s.installLocalization),
	}
	if localizationState.ready {
		localizationActions = []fyne.CanvasObject{
			widget.NewButton(tr.T(i18n.KeyLanguageWizardRemove),
				s.removeLocalization),
		}
	}

	rows := container.NewVBox(
		installRow(tr.T(i18n.KeyInstallMorrowind), morrowindState,
			widget.NewButton(tr.T(i18n.KeyInstallCheck), func() {
				s.showScreen(screenInstall)
			}),
			widget.NewButton(tr.T(i18n.KeyInstallChangeFolder), func() {
				s.openWizard(wizardMorrowind)
			}),
		),
		installRow(tr.Tf(i18n.KeySetupTES3MP, tes3mp.Version), tes3mpState,
			widget.NewButtonWithIcon(tr.T(i18n.KeyInstallOpenFolder),
				theme.FolderOpenIcon(), func() {
					s.openFolder(s.tes3mpFolder())
				}),
			widget.NewButton(tes3mpInstall, s.installTES3MPFromList),
		),
		installRow(tr.T(i18n.KeyInstallFonts), fontsState,
			widget.NewButton(tr.T(i18n.KeyFontsSetChange),
				s.showFontsDialog),
			widget.NewButton(tr.T(i18n.KeyInstallInstall), s.installFonts),
		),
		installRow(tr.T(i18n.KeyInstallLocalization), localizationState,
			localizationActions...),
		installRow(tr.T(i18n.KeyInstallUIVideo), uiVideoState,
			widget.NewButton(tr.T(i18n.KeyInstallUIVideoConfigure),
				s.showUIVideoDialog)),
	)

	header := container.NewVBox(
		style.Heading(tr.T(i18n.KeyInstallTitle)),
		style.MutedLabel(tr.T(i18n.KeyInstallIntro)),
		container.NewHBox(runWizard),
	)
	return container.NewVBox(
		header,
		rows,
		s.installationAdvanced(tr),
		s.installationDiagnostics(tr),
		s.installationDangerZone(tr),
	)
}

// installRow renders one checklist line: title and status on the left, actions
// on the right.
func installRow(title string, status installStatus,
	actions ...fyne.CanvasObject,
) fyne.CanvasObject {
	summary := widget.NewLabel(status.summary)
	summary.Wrapping = fyne.TextWrapWord
	summary.Importance = status.importance

	lines := []fyne.CanvasObject{style.Subheading(title), summary}
	if status.detail != "" {
		detail := widget.NewLabelWithStyle(status.detail, fyne.TextAlignLeading,
			fyne.TextStyle{Monospace: true})
		detail.Wrapping = fyne.TextWrapWord
		detail.Importance = widget.LowImportance
		lines = append(lines, detail)
	}
	text := container.NewVBox(lines...)
	if len(actions) == 0 {
		return style.Card(text, false)
	}
	return style.Card(container.NewBorder(nil, nil, nil,
		container.NewHBox(actions...), text), false)
}

func countIssues(statuses ...installStatus) int {
	issues := 0
	for _, status := range statuses {
		if !status.ready {
			issues++
		}
	}
	return issues
}

// refreshInstallationIssues keeps the rail badge current without blocking the
// window on filesystem checks.
func (s *shell) refreshInstallationIssues() {
	go func() {
		tr := s.service.Translator()
		issues := countIssues(
			s.morrowindStatus(tr),
			s.tes3mpStatus(tr),
			s.fontsStatus(tr),
			s.localizationStatus(tr),
			s.uiVideoStatus(tr),
		)
		fyne.Do(func() {
			s.issues = issues
			if button, ok := s.rail[screenInstall]; ok {
				button.SetText(railInstallLabel(tr, issues))
			}
		})
	}()
}

func (s *shell) morrowindStatus(tr *i18n.Translator) installStatus {
	installation, err := s.service.CheckMorrowind("")
	switch {
	case err == nil:
		return installStatus{
			summary:    tr.Tf(i18n.KeySetupMorrowindReady, installation.Root),
			importance: widget.SuccessImportance,
			ready:      true,
		}
	case errors.Is(err, app.ErrMorrowindNotConfigured):
		return installStatus{
			summary:    tr.T(i18n.KeyInstallMorrowindMissing),
			importance: widget.WarningImportance,
		}
	default:
		return installStatus{
			summary: tr.Tf(i18n.KeySetupMorrowindIncomplete,
				s.failureSummary(err)),
			importance: widget.WarningImportance,
		}
	}
}

func (s *shell) tes3mpStatus(tr *i18n.Translator) installStatus {
	state, err := s.service.TES3MPState()
	if err != nil {
		return installStatus{
			summary: tr.Tf(i18n.KeySetupMorrowindIncomplete,
				s.failureSummary(err)),
			importance: widget.WarningImportance,
		}
	}
	switch {
	case state.Current:
		return installStatus{
			summary:    tr.Tf(i18n.KeySetupTES3MPReady, state.Version),
			detail:     state.Dir,
			importance: widget.SuccessImportance,
			ready:      true,
		}
	case state.Installed:
		return installStatus{
			summary: tr.Tf(i18n.KeySetupTES3MPOutdated, state.Version,
				tes3mp.Version),
			detail:     state.Dir,
			importance: widget.WarningImportance,
		}
	default:
		return installStatus{
			summary:    tr.T(i18n.KeySetupTES3MPMissing),
			detail:     state.Dir,
			importance: widget.WarningImportance,
		}
	}
}

func (s *shell) fontsStatus(tr *i18n.Translator) installStatus {
	state, err := s.service.FontsState()
	if err != nil {
		return installStatus{
			summary: tr.Tf(i18n.KeyInstallFontsPartial,
				s.failureSummary(err)),
			importance: widget.WarningImportance,
		}
	}
	label := fontSetLabel(tr, state.Set)
	if s.service.Config().SkipFonts {
		return installStatus{
			summary:    tr.T(i18n.KeyInstallFontsSkipped),
			detail:     state.FontsDir,
			importance: widget.WarningImportance,
		}
	}
	switch {
	case state.Installed:
		return installStatus{
			summary:    tr.Tf(i18n.KeyInstallFontsReady, label),
			detail:     state.FontsDir,
			importance: widget.SuccessImportance,
			ready:      true,
		}
	case len(state.Missing) > 0 || len(state.Invalid) > 0:
		broken := append([]string(nil), state.Missing...)
		broken = append(broken, state.Invalid...)
		return installStatus{
			summary:    tr.Tf(i18n.KeyInstallFontsPartial, strings.Join(broken, ", ")),
			detail:     state.FontsDir,
			importance: widget.WarningImportance,
		}
	default:
		return installStatus{
			summary:    tr.Tf(i18n.KeyInstallFontsMissingSet, label),
			detail:     state.FontsDir,
			importance: widget.WarningImportance,
		}
	}
}

func (s *shell) localizationStatus(tr *i18n.Translator) installStatus {
	state, err := s.service.LocalizationState()
	if err != nil {
		return installStatus{
			summary: tr.Tf(i18n.KeySetupMorrowindIncomplete,
				s.failureSummary(err)),
			importance: widget.WarningImportance,
		}
	}
	if !state.Installed {
		return installStatus{
			summary:    tr.T(i18n.KeyInstallLocalizationMissing),
			importance: widget.WarningImportance,
		}
	}
	language := localizationInstallLabel(tr, state)
	return installStatus{
		summary:    tr.Tf(i18n.KeySetupLocalizationReady, language),
		detail:     state.DataFiles,
		importance: widget.SuccessImportance,
		ready:      true,
	}
}

// uiVideoStatus reports whether settings.cfg already holds the launcher
// values. A missing file is normal before the first game launch, so it does not
// count as an issue.
func (s *shell) uiVideoStatus(tr *i18n.Translator) installStatus {
	state, err := s.service.ClientSettings()
	if err != nil {
		return installStatus{
			summary: tr.Tf(i18n.KeyInstallUIVideoUnreadable,
				s.failureSummary(err)),
			importance: widget.WarningImportance,
		}
	}
	status := installStatus{
		summary:    uiVideoStateLabel(tr, state),
		detail:     state.Path,
		importance: widget.WarningImportance,
	}
	switch {
	case state.Configured:
		status.importance = widget.SuccessImportance
		status.ready = true
	case !state.Exists:
		status.importance = widget.LowImportance
		status.ready = true
	}
	return status
}

// installationAdvanced keeps the paths that used to live in the settings
// dialog. The work directory stays platform-default unless changed here.
func (s *shell) installationAdvanced(tr *i18n.Translator) fyne.CanvasObject {
	cfg := s.service.Config()
	paths := s.service.Paths()

	workDir := paths.DataDir
	if strings.TrimSpace(cfg.WorkDir) != "" {
		workDir = cfg.WorkDir
	}
	workDirRow := container.NewBorder(nil, nil, nil,
		widget.NewButton(tr.T(i18n.KeyInstallChange), func() {
			s.changePath(tr.T(i18n.KeySettingsWorkDir), workDir,
				func(value string) {
					settings := s.settings()
					settings.WorkDir = value
					s.saveSettings(settings)
				})
		}),
		container.NewVBox(pathLabel(workDir),
			style.MutedLabel(tr.T(i18n.KeyInstallWorkDirHint))))

	items := []*widget.AccordionItem{
		widget.NewAccordionItem(tr.T(i18n.KeySettingsWorkDir),
			container.NewVBox(workDirRow)),
	}
	if runtime.GOOS == "linux" {
		protonRow := container.NewBorder(nil, nil, nil,
			widget.NewButton(tr.T(i18n.KeyInstallChange), func() {
				s.changePath(tr.T(i18n.KeySettingsProton), cfg.ProtonPath,
					func(value string) {
						settings := s.settings()
						settings.ProtonPath = value
						s.saveSettings(settings)
					})
			}),
			container.NewVBox(pathLabel(cfg.ProtonPath),
				style.MutedLabel(tr.T(i18n.KeyInstallProtonHint))))
		items = append(items, widget.NewAccordionItem(
			tr.T(i18n.KeySettingsProton), container.NewVBox(protonRow)))
	}

	folders := container.NewVBox(
		s.folderRow(tr, tr.T(i18n.KeyInstallFolderTES3MP), s.tes3mpFolder()),
		s.folderRow(tr, tr.T(i18n.KeyInstallFolderConfig), s.openMWConfigFolder()),
		s.folderRow(tr, tr.T(i18n.KeyLogsTitle), s.logFolder()),
	)
	items = append(items, widget.NewAccordionItem(tr.T(i18n.KeyInstallFolders),
		folders))

	accordion := widget.NewAccordion(items...)
	return container.NewVBox(
		style.Subheading(tr.T(i18n.KeyCommonAdvanced)),
		accordion,
		style.MutedLabel(tr.T(i18n.KeyInstallProfileAuto)),
	)
}

// folderRow shows one path with an Open folder action.
func (s *shell) folderRow(tr *i18n.Translator, title, path string,
) fyne.CanvasObject {
	return container.NewBorder(nil, nil, style.MutedLabel(title), nil,
		container.NewHBox(
			pathLabel(path),
			widget.NewButtonWithIcon(tr.T(i18n.KeyInstallOpenFolder),
				theme.FolderOpenIcon(), func() {
					s.openFolder(path)
				}),
		),
	)
}

func (s *shell) installationDiagnostics(tr *i18n.Translator) fyne.CanvasObject {
	copyReport := widget.NewButtonWithIcon(tr.T(i18n.KeyMenuCopyReport),
		theme.ContentCopyIcon(), s.copyReport)
	openLog := widget.NewButtonWithIcon(tr.T(i18n.KeyLogsOpenFolder),
		theme.FolderOpenIcon(), s.openLogFolder)
	return container.NewVBox(
		style.Subheading(tr.T(i18n.KeyInstallDiagnostics)),
		container.NewHBox(copyReport, openLog),
	)
}

// installationDangerZone removes client-owned folders. It is the only place in
// the shell that deletes user data, so it stays visibly separate.
func (s *shell) installationDangerZone(tr *i18n.Translator) fyne.CanvasObject {
	button := widget.NewButton(tr.T(i18n.KeyUninstall), s.showUninstallDialog)
	button.Importance = widget.DangerImportance
	return container.NewVBox(
		style.Subheading(tr.T(i18n.KeyUninstallDangerZone)),
		button,
		style.MutedLabel(tr.T(i18n.KeyUninstallKeepMorrowind)),
	)
}

// changePath asks for a path and stores it through the supplied setter.
func (s *shell) changePath(title, current string, apply func(string)) {
	tr := s.service.Translator()
	entry := widget.NewEntry()
	entry.SetText(current)
	content := container.NewVBox(widget.NewLabel(title), entry)
	popup := dialog.NewCustomConfirm(title, tr.T(i18n.KeyCommonSave),
		tr.T(i18n.KeyCommonCancel), content, func(ok bool) {
			if !ok {
				return
			}
			apply(strings.TrimSpace(entry.Text))
		}, s.window)
	popup.Resize(fyne.NewSize(560, 200))
	popup.Show()
}

func (s *shell) saveSettings(settings app.Settings) {
	if err := s.service.UpdateSettings(settings); err != nil {
		s.showError(err)
		return
	}
	s.showScreen(screenInstall)
}

func (s *shell) tes3mpInstalled() bool {
	state, err := s.service.TES3MPState()
	return err == nil && state.Installed
}

func (s *shell) tes3mpFolder() string {
	state, err := s.service.TES3MPState()
	if err == nil && state.Dir != "" {
		return state.Dir
	}
	return s.service.WorkDir()
}

func (s *shell) openMWConfigFolder() string {
	dir, err := s.service.OpenMWConfigDir()
	if err != nil {
		return ""
	}
	return dir
}

func (s *shell) logFolder() string {
	path := s.service.LogPath()
	if path == "" {
		return ""
	}
	return filepath.Dir(path)
}

// installTES3MPFromList downloads the pinned release with the same progress
// wording as the wizard.
func (s *shell) installTES3MPFromList() {
	tr := s.service.Translator()
	status := widget.NewLabel(tr.T(i18n.KeySetupVerifying))
	progress := widget.NewProgressBar()
	progress.Min = 0
	progress.Max = 1
	popup := dialog.NewCustom(tr.T(i18n.KeySetupTES3MP),
		tr.T(i18n.KeyCommonClose), container.NewVBox(status, progress),
		s.window)
	popup.Resize(fyne.NewSize(560, 180))
	popup.Show()

	go func() {
		_, err := s.service.InstallTES3MP(context.Background(),
			func(update tes3mp.Progress) {
				fyne.Do(func() {
					progress.SetValue(progressFraction(update))
					status.SetText(progressLabel(tr, update))
				})
			})
		fyne.Do(func() {
			popup.Hide()
			if err != nil {
				s.showErrorWithRetry(err, s.installTES3MPFromList)
			}
			s.showScreen(screenInstall)
		})
	}()
}

// installFonts re-runs the fixed font set. The download has no progress
// callback, so the row reports the result instead of a percentage.
func (s *shell) installFonts() {
	tr := s.service.Translator()
	status := widget.NewLabel(tr.T(i18n.KeyLocaleChecking))
	status.Wrapping = fyne.TextWrapWord
	popup := dialog.NewCustomWithoutButtons(tr.T(i18n.KeyInstallFonts),
		container.NewVBox(status), s.window)
	popup.Resize(fyne.NewSize(560, 160))
	popup.Show()

	go func() {
		result, err := s.service.InstallFonts(context.Background())
		fyne.Do(func() {
			popup.Hide()
			switch {
			case err != nil:
				s.showErrorWithRetry(err, s.installFonts)
			case len(result.Warnings) > 0:
				s.showNotice(tr.T(i18n.KeyInstallFonts),
					tr.T(i18n.KeyInstallFontsMissing), result.Warnings)
			}
			s.showScreen(screenInstall)
		})
	}()
}

// installLocalization reuses the wizard step so the language choice, download
// progress and font warning stay in one place.
func (s *shell) installLocalization() {
	s.openWizard(wizardLocalization)
}

func (s *shell) removeLocalization() {
	tr := s.service.Translator()
	content := widget.NewLabel(tr.T(i18n.KeyLanguageWizardRemoveHint))
	content.Wrapping = fyne.TextWrapWord
	dialog.NewCustomConfirm(tr.T(i18n.KeyLanguageWizardRemove),
		tr.T(i18n.KeyLanguageWizardRemove), tr.T(i18n.KeyCommonCancel),
		content, func(ok bool) {
			if !ok {
				return
			}
			if err := s.service.RemoveLocalization(); err != nil {
				s.showErrorWithRetry(err, s.removeLocalization)
				return
			}
			s.showNotice(tr.T(i18n.KeyLanguageWizardRemove),
				tr.T(i18n.KeyLocaleRemoved), nil)
			s.showScreen(screenInstall)
		}, s.window).Show()
}

// openFolder opens a directory with the platform file manager.
func (s *shell) openFolder(folder string) {
	folder = strings.TrimSpace(folder)
	if folder == "" {
		return
	}
	name, args := openFolderCommand(runtime.GOOS, folder)
	command := exec.Command(name, args...)
	if err := command.Start(); err != nil {
		s.showErrorWithRetry(err, func() { s.openFolder(folder) })
		return
	}
	go func() {
		_ = command.Wait()
	}()
}

func pathLabel(path string) *widget.Label {
	if strings.TrimSpace(path) == "" {
		path = "—"
	}
	label := widget.NewLabelWithStyle(path, fyne.TextAlignLeading,
		fyne.TextStyle{Monospace: true})
	label.Wrapping = fyne.TextWrapWord
	label.Importance = widget.LowImportance
	return label
}
