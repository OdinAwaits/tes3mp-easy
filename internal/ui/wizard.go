package ui

import (
	"context"
	"errors"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/Saagilat/tes3mp-easy/internal/app"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/morrowind"
	"github.com/Saagilat/tes3mp-easy/internal/openmwcfg"
	"github.com/Saagilat/tes3mp-easy/internal/tes3mp"
	"github.com/Saagilat/tes3mp-easy/internal/ui/style"
)

type wizardStep int

const (
	wizardMorrowind wizardStep = iota
	wizardTES3MP
	wizardFonts
	wizardUIVideo
	wizardStepCount
)

type setupWizard struct {
	shell   *shell
	step    wizardStep
	popup   dialog.Dialog
	title   *widget.Label
	stepNo  *widget.Label
	body    *fyne.Container
	problem *widget.Label
	retry   *widget.Button
	back    *widget.Button
	next    *widget.Button

	busy        bool
	locked      bool
	retryAction func()

	// spinner runs while a check or a download is in flight: the disk scans
	// take seconds and the screen used to look frozen.
	spinner *widget.ProgressBarInfinite
	// morrowindActions are the three folder buttons of the first step. They are
	// disabled while a search runs, so a second click cannot queue another scan.
	morrowindActions []fyne.Disableable

	morrowind          *widget.Entry
	morrowindStatus    *widget.Label
	morrowindReady     bool
	morrowindMissing   bool
	morrowindAttempted bool

	tes3mpStatus   *widget.Label
	tes3mpProgress *widget.ProgressBar
	tes3mpInstall  *widget.Button
	tes3mpReady    bool

	fontsStatus    *widget.Label
	fontsAttempted bool
	fontSetID      string

	uiVideoForm   *uiVideoForm
	uiVideoStatus *widget.Label
	uiVideoState  openmwcfg.State
}

// showSetup opens the wizard at the folder that most often needs attention.
// The launcher UI is English by default, so a fresh install does not ask for a
// language first; a configured Morrowind folder skips straight to TES3MP.
func (s *shell) showSetup() {
	step := wizardMorrowind
	if strings.TrimSpace(s.service.Config().MorrowindPath) != "" {
		step = wizardTES3MP
	}
	s.openWizard(step)
}

func (s *shell) openWizard(step wizardStep) {
	tr := s.service.Translator()
	cfg := s.service.Config()
	w := &setupWizard{shell: s, step: step}

	w.morrowind = widget.NewEntry()
	w.morrowind.SetText(cfg.MorrowindPath)
	w.morrowind.SetPlaceHolder(".../Morrowind")

	w.title = widget.NewLabelWithStyle("", fyne.TextAlignLeading,
		fyne.TextStyle{Bold: true})
	w.stepNo = widget.NewLabel("")
	w.body = container.NewVBox()
	w.problem = widget.NewLabel("")
	w.problem.Wrapping = fyne.TextWrapWord
	w.retry = widget.NewButton(tr.T(i18n.KeyWizardRetry), func() {
		if w.retryAction == nil {
			return
		}
		action := w.retryAction
		w.clearProblem()
		action()
	})
	w.retry.Importance = widget.HighImportance
	w.problem.Hide()
	w.retry.Hide()

	w.back = widget.NewButton(tr.T(i18n.KeyWizardBack), w.previous)
	w.next = widget.NewButton(tr.T(i18n.KeyWizardNext), w.advance)
	w.next.Importance = widget.HighImportance

	problemRow := container.NewBorder(nil, nil, nil, w.retry, w.problem)
	footer := container.NewBorder(nil, nil, w.back, w.next)
	w.spinner = widget.NewProgressBarInfinite()
	w.spinner.Hide()
	content := container.NewBorder(
		container.NewVBox(w.title, w.stepNo),
		container.NewVBox(w.spinner, problemRow, footer),
		nil, nil,
		container.NewVScroll(w.body),
	)
	w.popup = dialog.NewCustom(tr.T(i18n.KeyWizardTitle),
		tr.T(i18n.KeyWizardCancel), content, s.window)
	w.popup.Resize(fyne.NewSize(780, 580))
	w.renderStep()
	w.popup.Show()
	w.enterStep()
}

func wizardTitleKey(step wizardStep) string {
	switch step {
	case wizardMorrowind:
		return i18n.KeyWizardMorrowindTitle
	case wizardTES3MP:
		return i18n.KeyWizardTES3MPTitle
	case wizardFonts:
		return i18n.KeyWizardFontsTitle
	case wizardUIVideo:
		return i18n.KeyWizardUIVideoTitle
	default:
		return i18n.KeyWizardUIVideoTitle
	}
}

func (w *setupWizard) renderStep() {
	tr := w.shell.service.Translator()
	w.title.SetText(tr.T(wizardTitleKey(w.step)))
	w.stepNo.SetText(tr.Tf(i18n.KeyWizardStep, int(w.step)+1,
		int(wizardStepCount)))
	w.body.Objects = w.stepObjects(tr)
	w.body.Refresh()
	w.popup.SetDismissText(tr.T(i18n.KeyWizardCancel))
	w.updateButtons()
}

func (w *setupWizard) stepObjects(tr *i18n.Translator) []fyne.CanvasObject {
	switch w.step {
	case wizardMorrowind:
		return w.morrowindObjects(tr)
	case wizardTES3MP:
		return w.tes3mpObjects(tr)
	case wizardFonts:
		return w.fontsObjects(tr)
	case wizardUIVideo:
		return w.uiVideoObjects(tr)
	default:
		return w.uiVideoObjects(tr)
	}
}

func wizardHint(tr *i18n.Translator, key string) *widget.Label {
	label := widget.NewLabel(tr.T(key))
	label.Wrapping = fyne.TextWrapWord
	return label
}

func (w *setupWizard) morrowindObjects(tr *i18n.Translator) []fyne.CanvasObject {
	w.morrowindStatus = widget.NewLabel("")
	w.morrowindStatus.Wrapping = fyne.TextWrapWord
	switch {
	case w.morrowindReady:
		w.morrowindStatus.SetText(tr.Tf(i18n.KeySetupMorrowindReady,
			w.morrowind.Text))
	case w.morrowindMissing:
		w.morrowindStatus.Importance = widget.WarningImportance
		w.morrowindStatus.SetText(tr.T(i18n.KeySetupMorrowindNotFound))
	}
	find := widget.NewButton(tr.T(i18n.KeySetupFind), w.detectMorrowind)
	choose := widget.NewButton(tr.T(i18n.KeySetupChoose), w.chooseMorrowind)
	check := widget.NewButton(tr.T(i18n.KeySetupCheck), func() {
		w.checkMorrowind(w.morrowind.Text)
	})
	w.morrowindActions = []fyne.Disableable{find, choose, check}
	return []fyne.CanvasObject{
		wizardHint(tr, i18n.KeyWizardMorrowindHint),
		w.morrowind,
		container.NewHBox(find, choose, check),
		w.morrowindStatus,
	}
}

func (w *setupWizard) fontsObjects(tr *i18n.Translator) []fyne.CanvasObject {
	w.fontsStatus = widget.NewLabel("")
	w.fontsStatus.Wrapping = fyne.TextWrapWord
	w.fontSetID = w.shell.service.FontSet()
	chooser := newFontSetChooser(tr, w.fontSetID, w.selectFontSet)
	w.renderFontsState(tr)
	return []fyne.CanvasObject{
		wizardHint(tr, i18n.KeyWizardFontsHint),
		widget.NewLabel(tr.T(i18n.KeyFontsSet)),
		chooser.group,
		style.MutedLabel(tr.T(i18n.KeyFontsSetLicense)),
		w.fontsStatus,
	}
}

// selectFontSet stores the chosen set. The files are downloaded after Finish,
// so moving through the wizard never stalls on the network.
func (w *setupWizard) selectFontSet(id string) {
	if id == "" || id == w.fontSetID {
		return
	}
	if err := w.shell.service.SetFontSet(id); err != nil {
		w.setProblem(err, func() { w.selectFontSet(id) })
		return
	}
	w.fontSetID = id
	w.clearProblem()
	w.renderFontsState(w.shell.service.Translator())
}

func (w *setupWizard) tes3mpObjects(tr *i18n.Translator) []fyne.CanvasObject {
	w.tes3mpStatus = widget.NewLabel("")
	w.tes3mpStatus.Wrapping = fyne.TextWrapWord
	w.tes3mpProgress = widget.NewProgressBar()
	w.tes3mpProgress.Min = 0
	w.tes3mpProgress.Max = 1
	install := widget.NewButton(tr.T(i18n.KeySetupInstall), w.installTES3MP)
	install.Importance = widget.HighImportance
	w.tes3mpInstall = install
	if w.tes3mpReady {
		install.Disable()
		w.tes3mpProgress.SetValue(1)
		w.tes3mpStatus.SetText(tr.Tf(i18n.KeySetupTES3MPReady,
			tes3mp.Version))
	}
	return []fyne.CanvasObject{
		wizardHint(tr, i18n.KeyWizardTES3MPHint),
		w.tes3mpStatus,
		container.NewBorder(nil, nil, nil, install, w.tes3mpProgress),
	}
}

func (w *setupWizard) uiVideoObjects(tr *i18n.Translator) []fyne.CanvasObject {
	w.uiVideoStatus = widget.NewLabel("")
	w.uiVideoStatus.Wrapping = fyne.TextWrapWord
	w.uiVideoStatus.SetText(uiVideoStateLabel(tr, w.uiVideoState))
	w.uiVideoForm = newUIVideoForm(openmwcfg.Defaults())
	objects := []fyne.CanvasObject{
		wizardHint(tr, i18n.KeyWizardUIVideoHint),
		w.uiVideoStatus,
	}
	return append(objects, w.uiVideoForm.objects(tr)...)
}

// loadUIVideo fills the step from the current settings.cfg. The file appears
// after the first game launch, so the defaults are shown until then.
func (w *setupWizard) loadUIVideo() {
	state, err := w.shell.service.ClientSettings()
	if err != nil {
		w.setProblem(err, w.loadUIVideo)
		return
	}
	w.uiVideoState = state
	if w.uiVideoForm != nil {
		w.uiVideoForm.set(state.Values)
	}
	if w.uiVideoStatus != nil {
		w.uiVideoStatus.SetText(
			uiVideoStateLabel(w.shell.service.Translator(), state))
	}
	w.updateButtons()
}

// saveUIVideo writes the step only when the values differ or settings.cfg
// already exists: the wizard must not create the file just to hold defaults.
func (w *setupWizard) saveUIVideo() bool {
	values, err := w.uiVideoForm.values()
	if err != nil {
		return false
	}
	if !w.uiVideoState.Exists && values == w.uiVideoState.Values {
		return true
	}
	if _, err := w.shell.service.UpdateClientSettings(values); err != nil {
		w.setProblem(err, w.advance)
		return false
	}
	return true
}

func (w *setupWizard) updateButtons() {
	if w.next == nil || w.back == nil {
		return
	}
	tr := w.shell.service.Translator()
	if w.step == wizardUIVideo {
		w.next.SetText(tr.T(i18n.KeyWizardFinish))
	} else {
		w.next.SetText(tr.T(i18n.KeyWizardNext))
	}
	if w.busy || !w.canAdvance() {
		w.next.Disable()
	} else {
		w.next.Enable()
	}
	if !w.canGoBack() {
		w.back.Hide()
		return
	}
	w.back.Show()
	if w.busy {
		w.back.Disable()
	} else {
		w.back.Enable()
	}
}

func (w *setupWizard) canAdvance() bool {
	switch w.step {
	case wizardMorrowind:
		// Not blocking: a machine without the game still gets a working client,
		// and the missing folder is reported as a warning on Installation.
		return true
	case wizardTES3MP:
		return w.tes3mpReady
	case wizardUIVideo:
		// A saved step must not carry an out-of-range value to settings.cfg.
		if w.uiVideoForm == nil {
			return false
		}
		_, err := w.uiVideoForm.values()
		return err == nil
	default:
		// Fonts never block the game: a failed download is a warning and the
		// next Play retries.
		return true
	}
}

func (w *setupWizard) canGoBack() bool {
	return !w.locked && w.step > wizardMorrowind &&
		w.step < wizardStepCount
}

func (w *setupWizard) advance() {
	if w.busy || !w.canAdvance() {
		return
	}
	switch w.step {
	case wizardMorrowind:
		w.keepMorrowindPath()
		w.gotoStep(wizardTES3MP)
	case wizardTES3MP:
		w.gotoStep(wizardFonts)
	case wizardFonts:
		w.gotoStep(wizardUIVideo)
	case wizardUIVideo:
		if w.saveUIVideo() {
			w.finishWizard()
		}
	}
}

func (w *setupWizard) previous() {
	if w.busy || !w.canGoBack() {
		return
	}
	switch w.step {
	case wizardTES3MP:
		w.gotoStep(wizardMorrowind)
	case wizardFonts:
		w.gotoStep(wizardTES3MP)
	case wizardUIVideo:
		w.gotoStep(wizardFonts)
	}
}

func (w *setupWizard) gotoStep(step wizardStep) {
	w.step = step
	w.clearProblem()
	w.renderStep()
	w.enterStep()
}

func (w *setupWizard) finishWizard() {
	if err := w.shell.service.CompleteSetup(); err != nil {
		w.setProblem(err, w.finishWizard)
		return
	}
	w.shell.service.LogInfof("first-run setup completed")
	needFonts := !w.shell.fontsInstalled()
	w.popup.Hide()
	w.shell.render()
	if needFonts {
		w.shell.startFontDownload()
	}
}

func (w *setupWizard) beginBusy() bool {
	if w.busy {
		return false
	}
	w.busy = true
	w.setActionsEnabled(false)
	if w.spinner != nil {
		w.spinner.Show()
	}
	w.updateButtons()
	return true
}

func (w *setupWizard) endBusy() {
	w.busy = false
	w.setActionsEnabled(true)
	if w.spinner != nil {
		w.spinner.Hide()
	}
	w.updateButtons()
}

// setActionsEnabled moves the step buttons with the busy flag. Without it a
// second click on "Find" was dropped silently, which looked like a dead button.
func (w *setupWizard) setActionsEnabled(enabled bool) {
	for _, action := range w.morrowindActions {
		if enabled {
			action.Enable()
			continue
		}
		action.Disable()
	}
}

// setMorrowindStatus writes the one-line progress or verdict of the step and
// drops the warning colour, so a retried search does not look like a failure.
func (w *setupWizard) setMorrowindStatus(text string, importance widget.Importance) {
	if w.morrowindStatus == nil {
		return
	}
	w.morrowindStatus.Importance = importance
	w.morrowindStatus.SetText(text)
}

// morrowindFailure splits an empty search from a real problem. A machine
// without the game is not broken: the status asks for the folder and the
// details (which paths were checked) stay below without a code or a Retry.
// A folder the user pointed at deserves the full T3E-ENV-01 block.
func (w *setupWizard) morrowindFailure(err error, retry func()) {
	w.morrowindReady = false
	w.morrowindMissing = errors.Is(err, morrowind.ErrNotFound) ||
		errors.Is(err, app.ErrMorrowindNotConfigured)
	if w.morrowindMissing {
		w.setMorrowindStatus(w.shell.service.Translator().T(
			i18n.KeySetupMorrowindNotFound), widget.WarningImportance)
		w.setProblemDetail(err)
		return
	}
	w.setProblem(err, retry)
}

// keepMorrowindPath stores what the user typed before leaving the step: the
// field is only saved after a successful check, and the first step must not
// silently drop a path just because it was not checked.
func (w *setupWizard) keepMorrowindPath() {
	value := strings.TrimSpace(w.morrowind.Text)
	if value == "" || value == strings.TrimSpace(w.shell.service.Config().MorrowindPath) {
		return
	}
	settings := w.shell.settings()
	settings.MorrowindPath = value
	if err := w.shell.service.UpdateSettings(settings); err != nil {
		w.shell.service.LogError("save Morrowind folder", err)
	}
}

func (w *setupWizard) setProblem(err error, retry func()) {
	if err == nil {
		return
	}
	w.problem.SetText(w.shell.failureBody(err))
	w.problem.Show()
	w.retryAction = retry
	if retry != nil {
		w.retry.Show()
	} else {
		w.retry.Hide()
	}
	w.updateButtons()
}

// setProblemDetail shows only the checked paths. The status line already says
// what to do, an empty search has nothing to retry, and a T3E code would claim
// a failure where the user only has to pick a folder.
func (w *setupWizard) setProblemDetail(err error) {
	if w.problem == nil {
		return
	}
	w.retryAction = nil
	w.problem.SetText(err.Error())
	w.problem.Show()
	if w.retry != nil {
		w.retry.Hide()
	}
	w.updateButtons()
}

func (w *setupWizard) clearProblem() {
	w.retryAction = nil
	if w.problem != nil {
		w.problem.Hide()
	}
	if w.retry != nil {
		w.retry.Hide()
	}
}

func (w *setupWizard) enterStep() {
	switch w.step {
	case wizardMorrowind:
		if w.morrowindAttempted {
			return
		}
		w.morrowindAttempted = true
		if strings.TrimSpace(w.morrowind.Text) == "" {
			w.detectMorrowind()
			return
		}
		w.checkMorrowind(w.morrowind.Text)
	case wizardTES3MP:
		w.checkTES3MP()
	case wizardFonts:
		if w.fontsAttempted {
			return
		}
		w.fontsAttempted = true
		w.renderFontsState(w.shell.service.Translator())
	case wizardUIVideo:
		w.loadUIVideo()
	}
}

func (w *setupWizard) detectMorrowind() {
	if !w.beginBusy() {
		return
	}
	tr := w.shell.service.Translator()
	status := w.morrowindStatus
	if status != nil {
		status.SetText(tr.T(i18n.KeySetupSearching))
	}
	go func() {
		installation, err := w.shell.service.DetectMorrowind()
		fyne.Do(func() {
			w.endBusy()
			if err != nil {
				w.shell.service.LogError("detect Morrowind", err)
				w.morrowindFailure(err, w.detectMorrowind)
				return
			}
			w.morrowind.SetText(installation.Root)
			w.morrowindReady = true
			w.morrowindMissing = false
			w.clearProblem()
			w.setMorrowindStatus(tr.Tf(i18n.KeySetupMorrowindReady,
				installation.Root), widget.MediumImportance)
			w.updateButtons()
		})
	}()
}

func (w *setupWizard) chooseMorrowind() {
	tr := w.shell.service.Translator()
	// The same picker as Installation → Advanced: the current value positions
	// the dialog, and a missing dialog still leaves the editable field.
	w.shell.chooseFolder(tr.T(i18n.KeyWizardMorrowindTitle),
		w.morrowind.Text, w.checkMorrowind)
}

func (w *setupWizard) checkMorrowind(path string) {
	if !w.beginBusy() {
		return
	}
	tr := w.shell.service.Translator()
	status := w.morrowindStatus
	value := strings.TrimSpace(path)
	if status != nil {
		status.SetText(tr.T(i18n.KeySetupChecking))
	}
	go func() {
		installation, err := w.shell.service.CheckMorrowind(value)
		fyne.Do(func() {
			w.endBusy()
			if err != nil {
				w.shell.service.LogError("check Morrowind", err)
				w.morrowindFailure(err, func() { w.checkMorrowind(value) })
				return
			}
			settings := w.shell.settings()
			settings.MorrowindPath = installation.Root
			if saveErr := w.shell.service.UpdateSettings(settings); saveErr != nil {
				w.morrowindReady = false
				w.setProblem(saveErr, func() { w.checkMorrowind(value) })
				return
			}
			w.morrowind.SetText(installation.Root)
			w.morrowindReady = true
			w.morrowindMissing = false
			w.clearProblem()
			if status != nil {
				status.Importance = widget.MediumImportance
				status.SetText(tr.Tf(i18n.KeySetupMorrowindReady,
					installation.Root))
			}
			w.updateButtons()
		})
	}()
}

// renderFontsState prints the current set state into the step. A missing set is
// not an error here: the background download starts after Finish.
func (w *setupWizard) renderFontsState(tr *i18n.Translator) {
	state, err := w.shell.service.FontsState()
	if err != nil {
		if w.fontsStatus != nil {
			w.fontsStatus.SetText(w.shell.failureBody(err))
		}
		w.updateButtons()
		return
	}
	if w.fontsStatus == nil {
		return
	}
	label := fontSetLabel(tr, state.Set)
	switch {
	case state.Installed:
		w.fontsStatus.SetText(tr.Tf(i18n.KeyInstallFontsReadyDetail,
			label, state.FontsDir))
	case len(state.Missing) > 0 || len(state.Invalid) > 0:
		broken := append([]string(nil), state.Missing...)
		broken = append(broken, state.Invalid...)
		w.fontsStatus.SetText(tr.Tf(i18n.KeyWizardFontsPartial,
			strings.Join(broken, ", ")))
	default:
		w.fontsStatus.SetText(tr.Tf(i18n.KeyWizardFontsPending, label))
	}
	w.updateButtons()
}

func (w *setupWizard) checkTES3MP() {
	if !w.beginBusy() {
		return
	}
	tr := w.shell.service.Translator()
	status := w.tes3mpStatus
	go func() {
		state, err := w.shell.service.TES3MPState()
		fyne.Do(func() {
			w.endBusy()
			if err != nil {
				w.tes3mpReady = false
				w.setProblem(err, w.checkTES3MP)
				return
			}
			w.tes3mpReady = state.Current
			if w.tes3mpReady && w.tes3mpInstall != nil {
				w.tes3mpInstall.Disable()
			}
			if status == nil {
				w.updateButtons()
				return
			}
			switch {
			case state.Current:
				status.SetText(tr.Tf(i18n.KeySetupTES3MPReady, state.Version))
			case state.Installed:
				status.SetText(tr.Tf(i18n.KeySetupTES3MPOutdated,
					state.Version, tes3mp.Version))
			default:
				status.SetText(tr.T(i18n.KeySetupTES3MPMissing))
			}
			w.updateButtons()
		})
	}()
}

func (w *setupWizard) installTES3MP() {
	if !w.beginBusy() {
		return
	}
	tr := w.shell.service.Translator()
	status := w.tes3mpStatus
	progress := w.tes3mpProgress
	if status != nil {
		status.SetText(tr.T(i18n.KeySetupStarting))
	}
	if progress != nil {
		progress.SetValue(0)
	}
	go func() {
		_, err := w.shell.service.InstallTES3MP(context.Background(),
			func(update tes3mp.Progress) {
				fyne.Do(func() {
					if progress != nil {
						progress.SetValue(progressFraction(update))
					}
					if status != nil {
						status.SetText(progressLabel(tr, update))
					}
				})
			})
		fyne.Do(func() {
			w.endBusy()
			if err != nil {
				w.tes3mpReady = false
				w.setProblem(err, w.installTES3MP)
				return
			}
			w.tes3mpReady = true
			w.locked = true
			w.clearProblem()
			if progress != nil {
				progress.SetValue(1)
			}
			if status != nil {
				status.SetText(tr.T(i18n.KeySetupDone))
			}
			if w.tes3mpInstall != nil {
				w.tes3mpInstall.Disable()
			}
			w.updateButtons()
		})
	}()
}
