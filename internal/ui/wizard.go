package ui

import (
	"context"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/tes3mp"
)

type wizardStep int

const (
	wizardMorrowind wizardStep = iota
	wizardWorkDir
	wizardTES3MP
	wizardFonts
	wizardReady
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

	morrowind          *widget.Entry
	morrowindStatus    *widget.Label
	morrowindReady     bool
	morrowindAttempted bool

	workDir *widget.Entry

	tes3mpStatus   *widget.Label
	tes3mpProgress *widget.ProgressBar
	tes3mpInstall  *widget.Button
	tes3mpReady    bool

	fontsStatus    *widget.Label
	fontsEnabled   *widget.Check
	fontsAttempted bool
	fontsReady     bool
}

// showSetup opens the wizard at the folder that most often needs attention.
// The launcher UI is English by default, so a fresh install does not ask for a
// language first.
func (s *shell) showSetup() {
	s.openWizard(wizardMorrowind)
}

func (s *shell) openWizard(step wizardStep) {
	tr := s.service.Translator()
	cfg := s.service.Config()
	w := &setupWizard{shell: s, step: step}

	w.morrowind = widget.NewEntry()
	w.morrowind.SetText(cfg.MorrowindPath)
	w.morrowind.SetPlaceHolder(".../Morrowind")

	w.workDir = widget.NewEntry()
	w.workDir.SetText(cfg.WorkDir)
	w.workDir.SetPlaceHolder(s.service.Paths().DataDir)

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
	content := container.NewBorder(
		container.NewVBox(w.title, w.stepNo),
		container.NewVBox(problemRow, footer),
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
	case wizardWorkDir:
		return i18n.KeyWizardWorkDirTitle
	case wizardTES3MP:
		return i18n.KeyWizardTES3MPTitle
	case wizardFonts:
		return i18n.KeyWizardFontsTitle
	default:
		return i18n.KeyWizardReadyTitle
	}
}

func (w *setupWizard) renderStep() {
	tr := w.shell.service.Translator()
	w.title.SetText(tr.T(wizardTitleKey(w.step)))
	w.stepNo.SetText(tr.Tf(i18n.KeyWizardStep, int(w.step)+1,
		int(wizardStepCount)))
	w.body.Objects = w.stepObjects(tr)
	w.body.Refresh()
	if w.step == wizardReady {
		w.popup.SetDismissText(tr.T(i18n.KeyCommonClose))
	} else {
		w.popup.SetDismissText(tr.T(i18n.KeyWizardCancel))
	}
	w.updateButtons()
}

func (w *setupWizard) stepObjects(tr *i18n.Translator) []fyne.CanvasObject {
	switch w.step {
	case wizardMorrowind:
		return w.morrowindObjects(tr)
	case wizardWorkDir:
		return w.workDirObjects(tr)
	case wizardTES3MP:
		return w.tes3mpObjects(tr)
	case wizardFonts:
		return w.fontsObjects(tr)
	default:
		return w.readyObjects(tr)
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
	if w.morrowindReady {
		w.morrowindStatus.SetText(tr.Tf(i18n.KeySetupMorrowindReady,
			w.morrowind.Text))
	}
	find := widget.NewButton(tr.T(i18n.KeySetupFind), w.detectMorrowind)
	choose := widget.NewButton(tr.T(i18n.KeySetupChoose), w.chooseMorrowind)
	check := widget.NewButton(tr.T(i18n.KeySetupCheck), func() {
		w.checkMorrowind(w.morrowind.Text)
	})
	return []fyne.CanvasObject{
		wizardHint(tr, i18n.KeyWizardMorrowindHint),
		w.morrowind,
		container.NewHBox(find, choose, check),
		w.morrowindStatus,
	}
}

func (w *setupWizard) workDirObjects(tr *i18n.Translator) []fyne.CanvasObject {
	return []fyne.CanvasObject{
		wizardHint(tr, i18n.KeyWizardWorkDirHint),
		w.workDir,
	}
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

func (w *setupWizard) fontsObjects(tr *i18n.Translator) []fyne.CanvasObject {
	cfg := w.shell.service.Config()
	w.fontsEnabled = widget.NewCheck(tr.T(i18n.KeySettingsFonts), nil)
	w.fontsEnabled.SetChecked(!cfg.SkipFonts || cfg.Localization)
	w.fontsStatus = widget.NewLabel(tr.T(i18n.KeyWizardFontsChecking))
	w.fontsStatus.Wrapping = fyne.TextWrapWord
	if !w.fontsEnabled.Checked {
		w.fontsReady = true
		w.fontsAttempted = true
		w.fontsStatus.SetText(tr.T(i18n.KeyWizardFontsSkipped))
	} else if w.fontsReady {
		w.fontsStatus.SetText(tr.T(i18n.KeyWizardFontsReady))
	}
	w.fontsEnabled.OnChanged = func(enabled bool) {
		if w.busy {
			return
		}
		if enabled {
			w.fontsReady = false
			w.fontsAttempted = false
			w.clearProblem()
			w.fontsStatus.SetText(tr.T(i18n.KeyWizardFontsChecking))
			w.installFonts()
			return
		}
		w.fontsReady = true
		w.fontsAttempted = true
		w.clearProblem()
		w.fontsStatus.SetText(tr.T(i18n.KeyWizardFontsSkipped))
		w.updateButtons()
	}
	return []fyne.CanvasObject{
		wizardHint(tr, i18n.KeyWizardFontsHint),
		w.fontsEnabled,
		w.fontsStatus,
	}
}

func (w *setupWizard) readyObjects(tr *i18n.Translator) []fyne.CanvasObject {
	cfg := w.shell.service.Config()
	summary := widget.NewLabel(tr.Tf(i18n.KeyWizardSummary,
		cfg.MorrowindPath, w.shell.service.WorkDir(), tes3mp.Version,
		w.shell.service.LogPath()))
	summary.Wrapping = fyne.TextWrapWord
	return []fyne.CanvasObject{
		wizardHint(tr, i18n.KeyWizardReadyHint),
		summary,
	}
}

func (w *setupWizard) updateButtons() {
	if w.next == nil || w.back == nil {
		return
	}
	tr := w.shell.service.Translator()
	if w.step == wizardReady {
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
		return w.morrowindReady
	case wizardTES3MP:
		return w.tes3mpReady
	case wizardFonts:
		return w.fontsReady || (w.fontsEnabled != nil && !w.fontsEnabled.Checked)
	default:
		return true
	}
}

func (w *setupWizard) canGoBack() bool {
	return !w.locked && w.step > wizardMorrowind && w.step < wizardFonts
}

func (w *setupWizard) advance() {
	if w.busy || !w.canAdvance() {
		return
	}
	switch w.step {
	case wizardMorrowind:
		w.gotoStep(wizardWorkDir)
	case wizardWorkDir:
		w.saveWorkDir()
	case wizardTES3MP:
		w.gotoStep(wizardFonts)
	case wizardFonts:
		w.gotoStep(wizardReady)
	default:
		w.finishWizard()
	}
}

func (w *setupWizard) previous() {
	if w.busy || !w.canGoBack() {
		return
	}
	switch w.step {
	case wizardWorkDir:
		w.gotoStep(wizardMorrowind)
	case wizardTES3MP:
		w.gotoStep(wizardWorkDir)
	}
}

func (w *setupWizard) gotoStep(step wizardStep) {
	w.step = step
	w.clearProblem()
	w.renderStep()
	w.enterStep()
}

func (w *setupWizard) finishWizard() {
	if w.fontsEnabled != nil {
		settings := w.shell.settings()
		settings.SkipFonts = !w.fontsEnabled.Checked
		if w.shell.service.Config().Localization {
			settings.SkipFonts = false
		}
		if err := w.shell.service.UpdateSettings(settings); err != nil {
			w.setProblem(err, w.finishWizard)
			return
		}
	}
	if err := w.shell.service.CompleteSetup(); err != nil {
		w.setProblem(err, w.finishWizard)
		return
	}
	w.shell.service.LogInfof("first-run setup completed")
	w.popup.Hide()
	w.shell.render()
}

func (w *setupWizard) beginBusy() bool {
	if w.busy {
		return false
	}
	w.busy = true
	w.updateButtons()
	return true
}

func (w *setupWizard) endBusy() {
	w.busy = false
	w.updateButtons()
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
		if !w.fontsAttempted {
			w.fontsAttempted = true
			if w.fontsEnabled != nil && !w.fontsEnabled.Checked {
				w.fontsReady = true
				return
			}
			w.installFonts()
		}
	}
}

func (w *setupWizard) detectMorrowind() {
	if !w.beginBusy() {
		return
	}
	tr := w.shell.service.Translator()
	status := w.morrowindStatus
	go func() {
		installation, err := w.shell.service.DetectMorrowind()
		fyne.Do(func() {
			w.endBusy()
			if err != nil {
				w.morrowindReady = false
				w.shell.service.LogError("detect Morrowind", err)
				w.setProblem(err, w.detectMorrowind)
				return
			}
			w.morrowind.SetText(installation.Root)
			w.morrowindReady = true
			w.clearProblem()
			if status != nil {
				status.SetText(tr.Tf(i18n.KeySetupMorrowindReady,
					installation.Root))
			}
			w.updateButtons()
		})
	}()
}

func (w *setupWizard) chooseMorrowind() {
	dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil || uri == nil {
			return
		}
		w.checkMorrowind(uri.Path())
	}, w.shell.window).Show()
}

func (w *setupWizard) checkMorrowind(path string) {
	if !w.beginBusy() {
		return
	}
	tr := w.shell.service.Translator()
	status := w.morrowindStatus
	value := strings.TrimSpace(path)
	go func() {
		installation, err := w.shell.service.CheckMorrowind(value)
		fyne.Do(func() {
			w.endBusy()
			if err != nil {
				w.morrowindReady = false
				w.shell.service.LogError("check Morrowind", err)
				w.setProblem(err, func() { w.checkMorrowind(value) })
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
			w.clearProblem()
			if status != nil {
				status.SetText(tr.Tf(i18n.KeySetupMorrowindReady,
					installation.Root))
			}
			w.updateButtons()
		})
	}()
}

func (w *setupWizard) saveWorkDir() {
	value := strings.TrimSpace(w.workDir.Text)
	if _, err := w.shell.service.CheckWorkDir(value); err != nil {
		w.setProblem(err, nil)
		return
	}
	settings := w.shell.settings()
	settings.WorkDir = value
	if err := w.shell.service.UpdateSettings(settings); err != nil {
		w.setProblem(err, w.saveWorkDir)
		return
	}
	w.gotoStep(wizardTES3MP)
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
		status.SetText(tr.T(i18n.KeySetupVerifying))
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

func (w *setupWizard) installFonts() {
	if !w.beginBusy() {
		return
	}
	tr := w.shell.service.Translator()
	status := w.fontsStatus
	go func() {
		result, err := w.shell.service.InstallFonts(context.Background())
		fyne.Do(func() {
			w.endBusy()
			if err != nil {
				w.fontsReady = false
				w.fontsAttempted = false
				w.setProblem(err, w.installFonts)
				return
			}
			w.fontsReady = true
			w.locked = true
			w.clearProblem()
			if status != nil {
				text := tr.T(i18n.KeyWizardFontsReady)
				if len(result.Warnings) > 0 {
					text = tr.Tf(i18n.KeyWizardFontsWarning,
						strings.Join(result.Warnings, "; "))
				}
				status.SetText(text)
			}
			w.updateButtons()
		})
	}()
}
