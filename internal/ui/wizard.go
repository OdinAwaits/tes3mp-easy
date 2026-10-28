package ui

import (
	"context"
	"errors"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
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
	fontPreview   *fontPreview
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

// openWizard builds the setup dialog at step, shows it and starts the check of
// that step.
func (s *shell) openWizard(step wizardStep) *setupWizard {
	w := s.newWizard(step)
	w.popup.Show()
	w.enterStep()
	return w
}

// newWizard builds the setup dialog at step without showing it and without
// starting its background check, so callers (and tests) can inspect the step
// that is on screen before any scan or download runs.
func (s *shell) newWizard(step wizardStep) *setupWizard {
	tr := s.service.Translator()
	cfg := s.service.Config()
	w := &setupWizard{shell: s, step: step}

	w.morrowind = widget.NewEntry()
	w.morrowind.SetText(cfg.MorrowindPath)
	w.morrowind.SetPlaceHolder(".../Morrowind")

	w.title = widget.NewLabelWithStyle("", fyne.TextAlignLeading,
		fyne.TextStyle{Bold: true})
	w.stepNo = widget.NewLabel("")
	// The body is a stack, not a VBox: a VBox gives its child the child's
	// MinSize, which squeezed a scrollable step into the 32 px floor of a
	// scroll and left the dialog mostly empty.
	w.body = container.NewStack()
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
	var spinnerRow fyne.CanvasObject
	w.spinner, spinnerRow = newWizardSpinner()
	body := container.NewBorder(
		container.NewVBox(w.title, w.stepNo),
		container.NewVBox(spinnerRow, problemRow, footer),
		nil, nil,
		w.body,
	)
	size := wizardSize(s.window)
	content := container.New(cappedMinLayout{cap: wizardContentCap(size)}, body)
	w.popup = dialog.NewCustom(tr.T(i18n.KeyWizardTitle),
		tr.T(i18n.KeyWizardCancel), content, s.window)
	w.popup.Resize(size)
	w.renderStep()
	return w
}

// wizardSize is the dialog size of the wizard: the launcher window size, shrunk
// for a smaller window but never below the size the last step needs. Step 3
// keeps a sample column beside a form of four values. The padding keeps the
// modal off the window edges.
func wizardSize(window fyne.Window) fyne.Size {
	size := launcherSize
	if window != nil {
		if canvas := window.Canvas().Size(); canvas.Width > 0 && canvas.Height > 0 {
			size.Width = max(min(size.Width, canvas.Width), wizardMinWidth)
			size.Height = max(min(size.Height, canvas.Height), wizardMinHeight)
		}
	}
	padding := theme.Padding() * 2
	return fyne.NewSize(size.Width-padding, size.Height-padding)
}

// wizardMinWidth and wizardMinHeight keep the wizard usable in a small window:
// below them the two columns of step 3 would be clipped instead of scrolled.
const (
	wizardMinWidth  float32 = 780
	wizardMinHeight float32 = 580
)

// dialogChromeWidth and dialogChromeHeight are what a Fyne dialog adds around
// its content: 32 px of horizontal padding, plus the title row, the button row
// and the padding between them. The cap subtracts them so the dialog keeps the
// size it was given.
const (
	dialogChromeWidth  float32 = 56
	dialogChromeHeight float32 = 150
)

// wizardContentCap is the largest MinSize the dialog content may report. Without
// it step 3 asked for 803 px of height in a 700 px window: Fyne sizes a dialog by
// the MinSize of its content, a vertical scroll reports the full height of what
// it scrolls, and the window edge cut the form instead of the step scrolling.
func wizardContentCap(size fyne.Size) fyne.Size {
	return fyne.NewSize(max(size.Width-dialogChromeWidth, 0),
		max(size.Height-dialogChromeHeight, 0))
}

// cappedMinLayout stretches one object to the whole area and reports a MinSize
// no larger than cap, which is how a scrollable step stays inside the dialog.
type cappedMinLayout struct {
	cap fyne.Size
}

func (l cappedMinLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, object := range objects {
		object.Move(fyne.NewPos(0, 0))
		object.Resize(size)
	}
}

func (l cappedMinLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	size := fyne.NewSize(0, 0)
	for _, object := range objects {
		min := object.MinSize()
		size.Width = max(size.Width, min.Width)
		size.Height = max(size.Height, min.Height)
	}
	return fyne.NewSize(min(size.Width, l.cap.Width), min(size.Height, l.cap.Height))
}

func wizardTitleKey(step wizardStep) string {
	switch step {
	case wizardMorrowind:
		return i18n.KeyWizardMorrowindTitle
	case wizardTES3MP:
		return i18n.KeyWizardTES3MPTitle
	default:
		// The last step holds both the font set and the settings.cfg values:
		// showing them apart made the wizard four screens of one topic.
		return i18n.KeyWizardFontsTitle
	}
}

func (w *setupWizard) renderStep() {
	tr := w.shell.service.Translator()
	w.title.SetText(tr.T(wizardTitleKey(w.step)))
	w.stepNo.SetText(tr.Tf(i18n.KeyWizardStep, int(w.step)+1,
		int(wizardStepCount)))
	w.body.Objects = []fyne.CanvasObject{w.stepContent(tr)}
	w.body.Refresh()
	w.popup.SetDismissText(tr.T(i18n.KeyWizardCancel))
	w.updateButtons()
}

// stepContent builds the body of the current step as one object. The first two
// steps scroll as a whole; the fonts step splits the body into two scrolls, so
// the sample stays on screen while the form of four values is scrolled.
func (w *setupWizard) stepContent(tr *i18n.Translator) fyne.CanvasObject {
	switch w.step {
	case wizardMorrowind:
		return container.NewVScroll(container.NewVBox(w.morrowindObjects(tr)...))
	case wizardTES3MP:
		return container.NewVScroll(container.NewVBox(w.tes3mpObjects(tr)...))
	default:
		return w.fontsStepContent(tr)
	}
}

// wizardHint is a step caption: it wraps instead of being cut by the wizard
// width, and it is built from the finished text so a formatted string never
// reaches the window with a placeholder in it.
func wizardHint(text string) *widget.Label {
	label := widget.NewLabel(text)
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
		wizardHint(tr.T(i18n.KeyWizardMorrowindHint)),
		w.morrowind,
		container.NewHBox(find, choose, check),
		w.morrowindStatus,
	}
}

// tes3mpHint names the version the step installs. It used to be rendered with
// T(), so the caption showed a literal %s instead of the version.
func tes3mpHint(tr *i18n.Translator) string {
	return tr.Tf(i18n.KeyWizardTES3MPHint, tes3mp.Version)
}

// fontsStepContent is the last step: everything the user edits is in the left
// column, the sample keeps the right one. The step body is shorter than both
// columns, so each column scrolls on its own and the sample stays on screen
// while a settings.cfg value is edited.
func (w *setupWizard) fontsStepContent(tr *i18n.Translator) fyne.CanvasObject {
	w.fontsStatus = widget.NewLabel("")
	w.fontsStatus.Wrapping = fyne.TextWrapWord
	w.fontSetID = w.shell.service.FontSet()
	chooser := newFontSetChooser(tr, w.fontSetID, w.selectFontSet)
	w.fontPreview = newFontPreview(tr, w.shell.fontPreviewFiles(), w.fontSetID)
	w.renderFontsState(tr)

	// settings.cfg belongs to the same screen: the font set and the four
	// values it renders are one decision, and the wizard does not need a
	// second click on Next for it.
	w.uiVideoStatus = widget.NewLabel("")
	w.uiVideoStatus.Wrapping = fyne.TextWrapWord
	w.uiVideoStatus.Hide()
	w.uiVideoForm = newUIVideoForm(openmwcfg.Defaults())
	w.uiVideoForm.onChanged(w.uiVideoChanged)
	w.uiVideoChanged()

	settings := container.NewVBox(
		widget.NewLabelWithStyle(tr.T(i18n.KeyFontsSet),
			fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		chooser.picker,
		w.fontsStatus,
		w.uiVideoStatus,
		uiVideoFormCard(tr, w.uiVideoForm),
	)
	leading, trailing := fontsColumns(settings, w.fontPreview.object())
	return container.NewBorder(
		wizardHint(tr.T(i18n.KeyWizardFontsHint)),
		nil, nil, nil,
		fontsStepLayout(leading, trailing),
	)
}

// fontsColumns wraps the two halves of the fonts step in their own scrolls. The
// sample used to be pinned above the form in the same column, where it was
// taller than the step body and painted over the footer buttons; a column of its
// own keeps it visible while the form scrolls, and a scroll clips it instead.
func fontsColumns(settings, sample fyne.CanvasObject) (fyne.CanvasObject, fyne.CanvasObject) {
	return container.NewVScroll(settings), container.NewVScroll(sample)
}

// fontsStepLayout puts the editable half beside the sample: widget.Select opens
// its popup downwards, so a sample under the list was covered exactly while the
// user compared sets. The splitter keeps the step usable in a smaller window
// instead of clipping one of the columns.
func fontsStepLayout(settings, sample fyne.CanvasObject) fyne.CanvasObject {
	split := container.NewHSplit(settings, sample)
	split.SetOffset(0.55)
	return split
}

// uiVideoFormCard is the settings.cfg half of the fonts step: a heading, the
// four managed values and the one-line explanation of what the form touches.
func uiVideoFormCard(tr *i18n.Translator, form *uiVideoForm) fyne.CanvasObject {
	hint := style.MutedLabel(tr.T(i18n.KeyInstallUIVideoHint))
	hint.Wrapping = fyne.TextWrapWord
	objects := append([]fyne.CanvasObject{
		widget.NewLabelWithStyle(tr.T(i18n.KeyInstallUIVideoTitle),
			fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	}, form.objects(tr)...)
	return container.NewVBox(append(objects, hint)...)
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
	if w.fontPreview != nil {
		w.fontPreview.selectSet(id)
	}
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
		wizardHint(tes3mpHint(tr)),
		w.tes3mpStatus,
		container.NewBorder(nil, nil, nil, install, w.tes3mpProgress),
	}
}

// uiVideoChanged redraws the font sample with the size the form describes right
// now. An invalid entry keeps the previous size: the form reports it on save.
func (w *setupWizard) uiVideoChanged() {
	if w.fontPreview == nil || w.uiVideoForm == nil {
		return
	}
	values, err := w.uiVideoForm.values()
	if err != nil {
		return
	}
	w.fontPreview.setScaling(values.FontSize, values.ScalingFactor)
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
	w.uiVideoChanged()
	// The fields already show the values of an existing settings.cfg; the
	// status line is only needed while the file is missing, where it explains
	// that the launcher would create it on the first save.
	if w.uiVideoStatus != nil {
		if state.Exists {
			w.uiVideoStatus.SetText("")
			w.uiVideoStatus.Hide()
		} else {
			w.uiVideoStatus.SetText(uiVideoStateLabel(
				w.shell.service.Translator(), state))
			w.uiVideoStatus.Show()
		}
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
	if w.step == wizardFonts {
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
	default:
		// The font download never blocks the game, but a saved step must not
		// carry an out-of-range value to settings.cfg.
		if w.uiVideoForm == nil {
			return false
		}
		_, err := w.uiVideoForm.values()
		return err == nil
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
	default:
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

// newWizardSpinner builds the busy row of the footer. The transparent rectangle
// keeps the row at the height of the bar, so a check that starts or ends moves
// nothing but its own bar; the buttons stay where the user last saw them.
func newWizardSpinner() (*widget.ProgressBarInfinite, fyne.CanvasObject) {
	spinner := widget.NewProgressBarInfinite()
	space := canvas.NewRectangle(color.Transparent)
	space.SetMinSize(spinner.MinSize())
	spinner.Hide()
	return spinner, container.NewStack(space, spinner)
}

// beginBusy locks the step while a background action runs. showSpinner is false
// for actions that already draw their own progress: two bars on one screen made
// the install look like two jobs.
func (w *setupWizard) beginBusy(showSpinner bool) bool {
	if w.busy {
		return false
	}
	w.busy = true
	w.setActionsEnabled(false)
	if showSpinner && w.spinner != nil {
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
		if !w.fontsAttempted {
			w.fontsAttempted = true
			w.renderFontsState(w.shell.service.Translator())
		}
		w.loadUIVideo()
	}
}

func (w *setupWizard) detectMorrowind() {
	if !w.beginBusy(true) {
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
	if !w.beginBusy(true) {
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
	if !w.beginBusy(true) {
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
	if !w.beginBusy(false) {
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
