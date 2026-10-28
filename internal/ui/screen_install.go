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
	"github.com/Saagilat/tes3mp-easy/internal/morrowind"
	"github.com/Saagilat/tes3mp-easy/internal/tes3mp"
	"github.com/Saagilat/tes3mp-easy/internal/ui/style"
)

// installStatus is the one-line verdict of an installation row.
type installStatus struct {
	summary    string
	detail     string
	importance widget.Importance
	// progress, when progressSet, adds a bar under the summary.
	progress    float64
	progressSet bool
}

// installScreen is the checklist that replaced the one-time setup wizard. Every
// row is built from an existing app method, so nothing is written to disk until
// the user presses a row action.
func (s *shell) installScreen(tr *i18n.Translator) fyne.CanvasObject {
	morrowindState := s.morrowindStatus(tr)
	tes3mpState := s.tes3mpStatus(tr)
	localizationState := s.localizationStatus(tr)
	uiVideoState := s.uiVideoStatus(tr)

	runWizard := style.PrimaryIconButton(tr.T(i18n.KeyInstallRunWizard),
		theme.DocumentCreateIcon(), s.showSetup)

	es3mpInstall := tr.T(i18n.KeyInstallInstall)
	if s.tes3mpInstalled() {
		es3mpInstall = tr.T(i18n.KeyInstallReinstall)
	}

	languageActions := []fyne.CanvasObject{
		style.SecondaryButton(tr.T(i18n.KeyLocaleInstall), s.showLanguageDialog),
	}
	if s.localizationInstalled() {
		languageActions = []fyne.CanvasObject{
			style.SecondaryButton(tr.T(i18n.KeyLocaleChange), s.showLanguageDialog),
			style.SecondaryButton(tr.T(i18n.KeyLanguageWizardRemove),
				s.removeLocalization),
		}
	}

	rows := []fyne.CanvasObject{
		installRow(tr.T(i18n.KeyInstallMorrowind), morrowindState,
			style.SecondaryButton(tr.T(i18n.KeySetupFind), s.findMorrowind),
			style.SecondaryButton(tr.T(i18n.KeyInstallChangeFolder), func() {
				s.openWizard(wizardMorrowind)
			}),
		),
		installRow(tr.Tf(i18n.KeySetupTES3MP, tes3mp.Version), tes3mpState,
			style.SecondaryButton(es3mpInstall, s.installTES3MPFromList),
		),
		s.fontsRow(tr),
		installRow(tr.T(i18n.KeyInstallLanguage), localizationState,
			languageActions...),
		installRow(tr.T(i18n.KeyInstallUIVideo), uiVideoState,
			style.SecondaryButton(tr.T(i18n.KeyInstallUIVideoConfigure),
				s.showUIVideoDialog)),
	}
	content := make([]fyne.CanvasObject, 0, len(rows)*2)
	for index, row := range rows {
		if index > 0 {
			content = append(content, widget.NewSeparator())
		}
		content = append(content, row)
	}

	header := container.NewVBox(
		style.Heading(tr.T(i18n.KeyInstallTitle)),
		style.MutedLabel(tr.T(i18n.KeyInstallIntro)),
		container.NewHBox(runWizard),
	)
	return container.NewVBox(
		header,
		style.Card(container.NewVBox(content...), false),
		s.installationAdvanced(tr),
		s.installationDangerZone(tr),
	)
}

// fontsRow reports the selected set and owns the inline download state. The
// background install started by the wizard lands here instead of opening a
// dialog over another screen.
func (s *shell) fontsRow(tr *i18n.Translator) fyne.CanvasObject {
	status := s.fontsStatus(tr)
	state := s.fontState()
	switch {
	case state.downloading:
		status.summary = tr.T(i18n.KeyFontsDownloading)
		status.detail = ""
		status.importance = widget.LowImportance
		status.progress = 0
		status.progressSet = true
		if state.hasProgress {
			status.summary = fontProgressLabel(tr, state.progress)
			status.progress = fontProgressFraction(state.progress)
		}
	case state.canceled:
		status.summary = tr.T(i18n.KeyFontsCanceled)
		status.detail = ""
		status.importance = widget.LowImportance
	case state.err != nil:
		status.summary = tr.T(i18n.KeyFontsDownloadFailed)
		status.detail = s.failureSummary(state.err)
		status.importance = widget.WarningImportance
	case len(state.warnings) > 0:
		status.summary = tr.Tf(i18n.KeyInstallFontsPartial,
			strings.Join(state.warnings, ", "))
		status.importance = widget.WarningImportance
	}

	var actions []fyne.CanvasObject
	if state.downloading {
		actions = append(actions, style.SecondaryButton(tr.T(i18n.KeyFontsCancel),
			s.cancelFontDownload))
	} else {
		actions = append(actions,
			style.SecondaryButton(tr.T(i18n.KeyFontsSetChange),
				s.showFontsDialog))
		switch {
		case state.err != nil, state.canceled:
			actions = append(actions, style.SecondaryButton(tr.T(i18n.KeyFontsRetry),
				s.startFontDownload))
		case !s.fontsInstalled():
			actions = append(actions, style.SecondaryButton(tr.T(i18n.KeyInstallInstall),
				s.startFontDownload))
		}
	}
	return installRow(tr.T(i18n.KeyInstallFonts), status, actions...)
}

// installRow renders one line of the checklist: the actions sit next to the
// title, the status and the details stay under it. Keeping the buttons in the
// right slot of the whole row left a field of empty space between the text and
// the buttons on a wide window. The rows share one card; installScreen adds the
// separators.
func installRow(title string, status installStatus,
	actions ...fyne.CanvasObject,
) fyne.CanvasObject {
	summary := widget.NewLabel(status.summary)
	summary.Wrapping = fyne.TextWrapWord
	summary.Importance = status.importance

	titleRow := fyne.CanvasObject(style.Subheading(title))
	if len(actions) > 0 {
		titleRow = container.NewBorder(nil, nil, nil,
			container.NewHBox(actions...), titleRow)
	}
	lines := []fyne.CanvasObject{titleRow, summary}
	if status.progressSet {
		bar := widget.NewProgressBar()
		bar.Min = 0
		bar.Max = 1
		bar.SetValue(status.progress)
		lines = append(lines, bar)
	}
	if status.detail != "" {
		detail := widget.NewLabelWithStyle(status.detail, fyne.TextAlignLeading,
			fyne.TextStyle{Monospace: true})
		detail.Wrapping = fyne.TextWrapWord
		detail.Importance = widget.LowImportance
		lines = append(lines, detail)
	}
	return container.NewVBox(lines...)
}

func (s *shell) morrowindStatus(tr *i18n.Translator) installStatus {
	installation, err := s.service.CheckMorrowind("")
	switch {
	case err == nil:
		return installStatus{
			summary:    tr.Tf(i18n.KeySetupMorrowindReady, installation.Root),
			importance: widget.SuccessImportance,
		}
	case errors.Is(err, app.ErrMorrowindNotConfigured),
		errors.Is(err, morrowind.ErrNotFound):
		// Nothing is broken: the row only waits for a folder.
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
	switch {
	case state.Installed:
		return installStatus{
			summary:    tr.Tf(i18n.KeyInstallFontsReady, label),
			detail:     state.FontsDir,
			importance: widget.SuccessImportance,
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
			summary:    tr.T(i18n.KeyInstallLanguageOriginal),
			importance: widget.LowImportance,
		}
	}
	return installStatus{
		summary:    localizationInstallLabel(tr, state),
		detail:     state.DataFiles,
		importance: widget.SuccessImportance,
	}
}

// localizationInstalled reports whether the overlay marker is present so the
// row can choose between Install and Change.
func (s *shell) localizationInstalled() bool {
	state, err := s.service.LocalizationState()
	return err == nil && state.Installed
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
	case !state.Exists:
		status.importance = widget.LowImportance
	}
	return status
}

// installationAdvanced keeps the paths that used to live in the settings
// dialog. The work directory stays platform-default unless changed here, and the
// Proton row shows the folder the launcher would actually use, not only the
// persisted override.
func (s *shell) installationAdvanced(tr *i18n.Translator) fyne.CanvasObject {
	cfg := s.service.Config()
	paths := s.service.Paths()

	workDir := paths.DataDir
	if strings.TrimSpace(cfg.WorkDir) != "" {
		workDir = cfg.WorkDir
	}
	workDirRow := container.NewVBox(
		container.NewBorder(nil, nil, nil,
			s.pathButtons(tr, tr.T(i18n.KeySettingsWorkDir),
				tr.T(i18n.KeyInstallChange), workDir, func(value string) {
					settings := s.settings()
					settings.WorkDir = value
					s.saveSettings(settings)
				}),
			pathLabel(workDir)),
		style.MutedLabel(tr.T(i18n.KeyInstallWorkDirHint)))

	items := []*widget.AccordionItem{
		widget.NewAccordionItem(tr.T(i18n.KeySettingsWorkDir),
			container.NewVBox(workDirRow)),
	}
	if runtime.GOOS == "linux" {
		proton := s.service.ProtonStatus()
		protonRow := container.NewVBox(
			container.NewBorder(nil, nil, nil,
				s.pathButtons(tr, tr.T(i18n.KeySettingsProton),
					tr.T(i18n.KeyInstallBrowse), proton.Path,
					func(value string) {
						settings := s.settings()
						settings.ProtonPath = value
						s.saveSettings(settings)
					}),
				pathLabel(proton.Path)),
			style.MutedLabel(protonHint(tr, proton)))
		items = append(items, widget.NewAccordionItem(
			tr.T(i18n.KeySettingsProton), container.NewVBox(protonRow)))
	}

	accordion := widget.NewAccordion(items...)
	return container.NewVBox(
		style.Subheading(tr.T(i18n.KeyCommonAdvanced)),
		accordion,
	)
}

// protonHint explains where the Proton folder comes from, or asks for one when
// nothing usable was found.
func protonHint(tr *i18n.Translator, status app.ProtonStatus) string {
	if status.Err != nil || strings.TrimSpace(status.Path) == "" {
		return tr.T(i18n.KeyInstallProtonMissing)
	}
	if status.Source == app.ProtonFromConfig {
		return tr.T(i18n.KeyInstallProtonManual)
	}
	return tr.T(i18n.KeyInstallProtonFound)
}

// installationDangerZone removes client-owned folders. It is the only place in
// the shell that deletes user data, so it stays visibly separate.
func (s *shell) installationDangerZone(tr *i18n.Translator) fyne.CanvasObject {
	button := style.DangerButton(tr.T(i18n.KeyUninstall), s.showUninstallDialog)
	return container.NewVBox(
		style.Subheading(tr.T(i18n.KeyUninstallDangerZone)),
		button,
		style.MutedLabel(tr.T(i18n.KeyUninstallKeepMorrowind)),
	)
}

func (s *shell) saveSettings(settings app.Settings) {
	if err := s.service.UpdateSettings(settings); err != nil {
		s.showError(err)
		return
	}
	s.showScreen(screenInstall)
}

// findMorrowind runs the same search as the wizard, but from the checklist:
// the row stays visible, so the result is reported in place.
func (s *shell) findMorrowind() {
	tr := s.service.Translator()
	status := widget.NewLabel(tr.T(i18n.KeySetupSearching))
	status.Wrapping = fyne.TextWrapWord
	spinner := widget.NewProgressBarInfinite()
	popup := dialog.NewCustom(tr.T(i18n.KeySetupFind),
		tr.T(i18n.KeyCommonClose), container.NewVBox(status, spinner), s.window)
	popup.Resize(fyne.NewSize(520, 160))
	popup.Show()

	go func() {
		installation, err := s.service.DetectMorrowind()
		fyne.Do(func() {
			popup.Hide()
			s.showScreen(screenInstall)
			if err != nil {
				s.service.LogError("detect Morrowind", err)
				s.showNotice(tr.T(i18n.KeySetupFind),
					tr.T(i18n.KeySetupMorrowindNotFound), nil)
				return
			}
			s.showNotice(tr.T(i18n.KeySetupFind),
				tr.Tf(i18n.KeySetupMorrowindReady, installation.Root), nil)
		})
	}()
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
	status := widget.NewLabel(tr.T(i18n.KeySetupStarting))
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

// fontsInstalled reports whether the selected set is complete on disk.
func (s *shell) fontsInstalled() bool {
	state, err := s.service.FontsState()
	return err == nil && state.Installed
}

func (s *shell) removeLocalization() {
	tr := s.service.Translator()
	content := widget.NewLabel(tr.T(i18n.KeyLanguageWizardRemoveHint))
	content.Wrapping = fyne.TextWrapWord
	dialog.NewCustomConfirm(tr.T(i18n.KeyLocaleTitle),
		tr.T(i18n.KeyLanguageWizardRemove), tr.T(i18n.KeyCommonCancel),
		content, func(ok bool) {
			if !ok {
				return
			}
			if err := s.service.RemoveLocalization(); err != nil {
				s.showErrorWithRetry(err, s.removeLocalization)
				return
			}
			s.showNotice(tr.T(i18n.KeyLocaleTitle),
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
