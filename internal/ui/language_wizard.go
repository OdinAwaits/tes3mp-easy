package ui

import (
	"context"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/locale"
)

type languageWizardStep int

const (
	languageWizardChoose languageWizardStep = iota
	languageWizardInstall
	languageWizardStepCount
)

type languageWizard struct {
	shell   *shell
	step    languageWizardStep
	popup   *dialog.CustomDialog
	title   *widget.Label
	stepNo  *widget.Label
	body    *fyne.Container
	problem *widget.Label
	retry   *widget.Button
	back    *widget.Button
	next    *widget.Button
	cancel  *widget.Button

	choice     *widget.RadioGroup
	choiceHint *widget.Label
	status     *widget.Label
	progress   *widget.ProgressBar

	current     locale.State
	busy        bool
	finished    bool
	changed     bool
	retryAction func()
}

// showLanguageWizard installs one optional localization package for every
// server profile. It is not a per-profile setting.
func (s *shell) showLanguageWizard() {
	tr := s.service.Translator()
	state, err := s.service.LocalizationState()
	if err != nil {
		s.showError(err)
		return
	}
	w := &languageWizard{shell: s, current: state}

	w.title = widget.NewLabelWithStyle("", fyne.TextAlignLeading,
		fyne.TextStyle{Bold: true})
	w.stepNo = widget.NewLabel("")
	w.body = container.NewVBox()
	w.problem = widget.NewLabel("")
	w.problem.Wrapping = fyne.TextWrapWord
	w.problem.Importance = widget.DangerImportance
	w.problem.Hide()

	w.retry = widget.NewButton(tr.T(i18n.KeyWizardRetry), func() {
		if w.retryAction == nil || w.busy {
			return
		}
		action := w.retryAction
		w.clearProblem()
		action()
	})
	w.retry.Importance = widget.HighImportance
	w.retry.Hide()

	w.choice = widget.NewRadioGroup(localizationLabels(tr), nil)
	w.choice.Horizontal = false
	if state.Installed {
		w.choice.SetSelected(localizationLabel(tr, state.Language))
	}
	if w.choice.Selected == "" && len(w.choice.Options) > 0 {
		w.choice.SetSelected(w.choice.Options[0])
	}
	w.choice.OnChanged = func(string) {
		w.syncChoiceHint()
		w.updateButtons()
	}

	w.back = widget.NewButton(tr.T(i18n.KeyWizardBack), w.previous)
	w.next = widget.NewButton(tr.T(i18n.KeyWizardNext), w.advance)
	w.next.Importance = widget.HighImportance
	w.cancel = widget.NewButton(tr.T(i18n.KeyCommonCancel), w.close)

	problemRow := container.NewBorder(nil, nil, nil, w.retry, w.problem)
	footer := container.NewBorder(nil, nil, w.back,
		container.NewHBox(w.cancel, w.next))
	content := container.NewBorder(
		container.NewVBox(w.title, w.stepNo),
		container.NewVBox(problemRow, footer),
		nil, nil,
		container.NewVScroll(w.body),
	)
	w.popup = dialog.NewCustomWithoutButtons(tr.T(i18n.KeyLocaleInstallTitle),
		content, s.window)
	w.popup.Resize(fyne.NewSize(640, 480))
	w.renderStep()
	w.popup.Show()
}

func languageWizardTitleKey(step languageWizardStep) string {
	if step == languageWizardInstall {
		return i18n.KeyLanguageWizardInstallTitle
	}
	return i18n.KeyLanguageWizardChooseTitle
}

func (w *languageWizard) renderStep() {
	tr := w.shell.service.Translator()
	w.title.SetText(tr.T(languageWizardTitleKey(w.step)))
	w.stepNo.SetText(tr.Tf(i18n.KeyWizardStep, int(w.step)+1,
		int(languageWizardStepCount)))
	w.body.Objects = w.stepObjects(tr)
	w.body.Refresh()
	w.updateButtons()
}

func (w *languageWizard) stepObjects(tr *i18n.Translator) []fyne.CanvasObject {
	if w.step == languageWizardInstall {
		return w.installObjects(tr)
	}
	return w.chooseObjects(tr)
}

func (w *languageWizard) chooseObjects(tr *i18n.Translator) []fyne.CanvasObject {
	w.choiceHint = widget.NewLabel("")
	w.choiceHint.Wrapping = fyne.TextWrapWord
	w.syncChoiceHint()
	return []fyne.CanvasObject{
		wizardHint(tr, i18n.KeyLanguageWizardChooseHint),
		widget.NewLabel(tr.Tf(i18n.KeyLanguageWizardCurrent,
			localizationStateLabel(tr, w.current))),
		w.choice,
		w.choiceHint,
	}
}

func (w *languageWizard) installObjects(tr *i18n.Translator) []fyne.CanvasObject {
	w.status = widget.NewLabel(tr.T(i18n.KeyLanguageWizardApplying))
	w.status.Wrapping = fyne.TextWrapWord
	w.progress = widget.NewProgressBar()
	w.progress.Min = 0
	w.progress.Max = 1
	return []fyne.CanvasObject{
		wizardHint(tr, i18n.KeyLanguageWizardInstallHint),
		w.status,
		w.progress,
	}
}

func (w *languageWizard) updateButtons() {
	if w.next == nil {
		return
	}
	tr := w.shell.service.Translator()
	w.next.Show()
	w.next.Enable()
	w.back.Hide()
	w.retry.Hide()
	w.cancel.Show()
	w.cancel.Enable()

	if w.busy {
		w.next.Disable()
		w.back.Hide()
		w.retry.Hide()
		w.cancel.Disable()
		return
	}

	if w.step == languageWizardChoose {
		w.next.SetText(tr.T(i18n.KeyWizardNext))
		return
	}
	w.next.Hide()
	w.back.Show()
	if w.finished {
		w.back.Hide()
		w.next.Show()
		w.next.Enable()
		w.next.SetText(tr.T(i18n.KeyCommonClose))
		w.cancel.Hide()
		return
	}
	if w.retryAction != nil {
		w.retry.Show()
	}
}

func (w *languageWizard) advance() {
	if w.busy {
		return
	}
	if w.step == languageWizardChoose {
		tr := w.shell.service.Translator()
		chosen := localizationValue(tr, w.choice.Selected)
		if chosen == "" || !localizationNeedsInstall(w.current, chosen) {
			w.close()
			return
		}
		w.gotoStep(languageWizardInstall)
		w.startInstall()
		return
	}
	if w.finished {
		w.close()
	}
}

func (w *languageWizard) previous() {
	if w.busy || w.finished {
		return
	}
	if w.step == languageWizardInstall {
		w.gotoStep(languageWizardChoose)
	}
}

func (w *languageWizard) gotoStep(step languageWizardStep) {
	w.step = step
	w.clearProblem()
	w.renderStep()
}

func (w *languageWizard) close() {
	if w.busy {
		return
	}
	w.popup.Hide()
	if w.changed {
		w.changed = false
		w.shell.render()
	}
}

func (w *languageWizard) startInstall() {
	if w.busy {
		return
	}
	tr := w.shell.service.Translator()
	chosen := localizationValue(tr, w.choice.Selected)
	if chosen == "" {
		return
	}
	status := w.status
	progress := w.progress
	w.clearProblem()
	w.busy = true
	w.finished = false
	w.updateButtons()
	if status != nil {
		status.SetText(tr.T(i18n.KeyLanguageWizardApplying))
	}
	if progress != nil {
		progress.SetValue(0)
	}

	go func() {
		result, err := w.shell.service.InstallLocalization(
			context.Background(), chosen, func(update locale.Progress) {
				fyne.Do(func() {
					if progress != nil {
						progress.SetValue(localeProgressFraction(update))
					}
					if status != nil {
						status.SetText(localeProgressLabel(tr, update))
					}
				})
			})
		fyne.Do(func() {
			w.finishInstall(result, err)
		})
	}()
}

func (w *languageWizard) finishInstall(result locale.Result, err error) {
	w.busy = false
	if err != nil {
		w.finished = false
		w.setProblem(err, w.startInstall)
		return
	}
	w.finished = true
	w.changed = true
	w.clearProblem()
	tr := w.shell.service.Translator()
	if w.progress != nil {
		w.progress.SetValue(1)
	}
	if w.status != nil {
		text := tr.T(i18n.KeyLanguageWizardDone)
		if len(result.Warnings) > 0 {
			text += "\n\n" + strings.Join(result.Warnings, "\n")
		}
		w.status.SetText(text)
	}
	w.choice.Disable()
	w.updateButtons()
}

func (w *languageWizard) syncChoiceHint() {
	if w.choiceHint == nil || w.choice == nil {
		return
	}
	tr := w.shell.service.Translator()
	key := i18n.KeyLanguageWizardRussianHint
	if localizationValue(tr, w.choice.Selected) != locale.LanguageRussian {
		key = i18n.KeyLanguageWizardChooseHint
	}
	w.choiceHint.SetText(tr.T(key))
}

func (w *languageWizard) setProblem(err error, retry func()) {
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

func (w *languageWizard) clearProblem() {
	w.retryAction = nil
	if w.problem != nil {
		w.problem.Hide()
	}
	if w.retry != nil {
		w.retry.Hide()
	}
}

func localizationLabels(tr *i18n.Translator) []string {
	packs := locale.Packages()
	labels := make([]string, 0, len(packs))
	for _, pack := range packs {
		labels = append(labels, localizationLabel(tr, pack.Language))
	}
	return labels
}

func localizationValue(tr *i18n.Translator, label string) string {
	for _, pack := range locale.Packages() {
		if label == localizationLabel(tr, pack.Language) {
			return pack.Language
		}
	}
	return ""
}

func localizationNeedsInstall(state locale.State, language string) bool {
	if !state.Installed || !strings.EqualFold(state.Language, language) {
		return true
	}
	return !state.Voiceover
}

func localizationStateLabel(tr *i18n.Translator, state locale.State) string {
	if !state.Installed {
		return tr.T(i18n.KeyLanguageWizardStateMissing)
	}
	label := localizationLabel(tr, state.Language)
	if state.Voiceover {
		return tr.Tf(i18n.KeyLanguageWizardStateInstalledVoice, label)
	}
	return tr.Tf(i18n.KeyLanguageWizardStateInstalled, label)
}
