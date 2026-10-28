package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/Saagilat/tes3mp-easy/internal/app"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/morrowind"
	"github.com/Saagilat/tes3mp-easy/internal/openmwcfg"
	"github.com/Saagilat/tes3mp-easy/internal/tes3mp"
)

// newTestWizard builds the parts of the wizard the first step touches, without
// the popup: the step logic is what the tests check.
func newTestWizard(t *testing.T, s *shell) *setupWizard {
	t.Helper()
	w := &setupWizard{shell: s, step: wizardMorrowind}
	w.morrowind = widget.NewEntry()
	w.morrowindStatus = widget.NewLabel("")
	w.problem = widget.NewLabel("")
	w.retry = widget.NewButton("retry", nil)
	w.retry.Hide()
	w.spinner, _ = newWizardSpinner()
	return w
}

// TestWizardMorrowindStepIsNotBlocking keeps the wizard finishable on a machine
// without the game: the folder is a warning, not a gate.
func TestWizardMorrowindStepIsNotBlocking(t *testing.T) {
	s := testShell(t)
	defer s.window.Close()
	w := newTestWizard(t, s)

	if w.morrowindReady {
		t.Fatal("the test wizard starts ready")
	}
	if !w.canAdvance() {
		t.Fatal("the Morrowind step blocks Next without the game")
	}
}

// TestWizardBusyDisablesFolderActions covers the dead-looking buttons: while a
// search runs the three folder actions are disabled and the spinner is visible.
func TestWizardBusyDisablesFolderActions(t *testing.T) {
	s := testShell(t)
	defer s.window.Close()
	w := newTestWizard(t, s)

	find := widget.NewButton("find", nil)
	choose := widget.NewButton("choose", nil)
	check := widget.NewButton("check", nil)
	w.morrowindActions = []fyne.Disableable{find, choose, check}

	if !w.beginBusy(true) {
		t.Fatal("beginBusy refused the first call")
	}
	for index, action := range w.morrowindActions {
		if !action.Disabled() {
			t.Fatalf("action %d is still enabled during a search", index)
		}
	}
	if !w.spinner.Visible() {
		t.Fatal("the spinner is hidden during a search")
	}
	if w.beginBusy(true) {
		t.Fatal("beginBusy accepted a second search")
	}

	w.endBusy()
	for index, action := range w.morrowindActions {
		if action.Disabled() {
			t.Fatalf("action %d stayed disabled after the search", index)
		}
	}
	if w.spinner.Visible() {
		t.Fatal("the spinner stayed visible after the search")
	}
}

// TestWizardInstallHasOneProgressBar is the BL-019 rule: the TES3MP step draws
// its own bar, so a second endless one must not appear next to it.
func TestWizardInstallHasOneProgressBar(t *testing.T) {
	s := testShell(t)
	defer s.window.Close()
	w := newTestWizard(t, s)

	progress := widget.NewProgressBar()
	w.tes3mpProgress = progress

	if !w.beginBusy(false) {
		t.Fatal("beginBusy(false) refused the first call")
	}
	if w.spinner.Visible() {
		t.Fatal("the endless bar is shown next to the install progress")
	}
	if !progress.Visible() {
		t.Fatal("the install progress bar is hidden")
	}
	w.endBusy()
	if w.spinner.Visible() {
		t.Fatal("the endless bar stayed visible after the install")
	}
}

// TestWizardFooterKeepsSpinnerSpace freezes the height of the busy row: a
// hidden bar must not pull the footer buttons up.
func TestWizardFooterKeepsSpinnerSpace(t *testing.T) {
	spinner, row := newWizardSpinner()
	if spinner.Visible() {
		t.Fatal("a fresh busy row starts visible")
	}
	if got, want := row.MinSize().Height, spinner.MinSize().Height; got != want {
		t.Fatalf("busy row height = %.0f, want %.0f", got, want)
	}
	spinner.Show()
	if got, want := row.MinSize().Height, spinner.MinSize().Height; got != want {
		t.Fatalf("busy row height grew to %.0f from %.0f", got, want)
	}
}

// TestFontsStepPutsSampleBesideTheForm is the BL-020 rule: the settings and the
// sample never share one vertical container, because the dropdown of the set
// list opens downwards and covered the sample while the user compared sets.
func TestFontsStepPutsSampleBesideTheForm(t *testing.T) {
	settings := widget.NewLabel("settings")
	sample := widget.NewLabel("sample")
	leading, trailing := fontsColumns(settings, sample)

	split, ok := fontsStepLayout(leading, trailing).(*container.Split)
	if !ok {
		t.Fatal("the fonts step is not a horizontal split")
	}
	if !labelTexts(split.Leading)["settings"] || labelTexts(split.Leading)["sample"] {
		t.Fatalf("left column = %v, want only the settings", labelTexts(split.Leading))
	}
	if !labelTexts(split.Trailing)["sample"] || labelTexts(split.Trailing)["settings"] {
		t.Fatalf("right column = %v, want only the sample", labelTexts(split.Trailing))
	}
}

// TestFontsColumnsScrollOnTheirOwn freezes the fix of the clipped step 3: the
// sample used to be pinned above the form in the same column, where it was taller
// than the step body and painted over the footer buttons. Each half now has its
// own scroll, which clips it to its column.
func TestFontsColumnsScrollOnTheirOwn(t *testing.T) {
	leading, trailing := fontsColumns(widget.NewLabel("form"), widget.NewLabel("sample"))

	leftScroll, ok := leading.(*container.Scroll)
	if !ok {
		t.Fatal("the form column is not scrollable")
	}
	if !labelTexts(leftScroll.Content)["form"] {
		t.Fatal("the form is not inside its scroll")
	}
	rightScroll, ok := trailing.(*container.Scroll)
	if !ok {
		t.Fatal("the sample column is not scrollable")
	}
	if !labelTexts(rightScroll.Content)["sample"] {
		t.Fatal("the sample is not inside its scroll")
	}
}

// TestWizardHintsUseTheRealValues covers two complaints about steps 2 and 3: the
// TES3MP caption went through T(), so a literal %s reached the window, and both
// captions explained in a paragraph what one sentence says.
func TestWizardHintsUseTheRealValues(t *testing.T) {
	for _, language := range i18n.Supported() {
		tr := i18n.New(language)
		hint := tes3mpHint(tr)
		if strings.Contains(hint, "%") {
			t.Fatalf("%s TES3MP hint still has a placeholder: %q", language, hint)
		}
		if !strings.Contains(hint, tes3mp.Version) {
			t.Fatalf("%s TES3MP hint does not name the version: %q",
				language, hint)
		}
		lower := strings.ToLower(hint)
		if strings.Contains(lower, "server") || strings.Contains(lower, "сервер") {
			t.Fatalf("%s TES3MP hint talks about server details: %q",
				language, hint)
		}
		if got := tr.T(i18n.KeyWizardFontsHint); len([]rune(got)) > 140 {
			t.Fatalf("%s fonts hint is %d characters, want one short line",
				language, len([]rune(got)))
		}
	}
}

// TestWizardFillsTheLauncherWindow keeps step 3 readable: the dialog used to be
// 780×580, where the form of four values had to be scrolled under the sample.
func TestWizardFillsTheLauncherWindow(t *testing.T) {
	s := testShell(t)
	defer s.window.Close()
	s.window.Resize(launcherSize)

	size := wizardSize(s.window)
	if size.Width < launcherSize.Width-16 || size.Height < launcherSize.Height-16 {
		t.Fatalf("wizard size = %.0f×%.0f, want the launcher window %.0f×%.0f",
			size.Width, size.Height, launcherSize.Width, launcherSize.Height)
	}
	if size.Width > launcherSize.Width || size.Height > launcherSize.Height {
		t.Fatalf("wizard size = %.0f×%.0f is larger than the window %.0f×%.0f",
			size.Width, size.Height, launcherSize.Width, launcherSize.Height)
	}
	// A window smaller than the wizard still gets the two columns of step 3.
	s.window.Resize(fyne.NewSize(600, 400))
	small := wizardSize(s.window)
	if small.Width < wizardMinWidth-16 || small.Height < wizardMinHeight-16 {
		t.Fatalf("wizard size in a 600×400 window = %.0f×%.0f, want at least %.0f×%.0f",
			small.Width, small.Height, wizardMinWidth, wizardMinHeight)
	}
}

// TestWizardContentFitsTheDialog is the "content runs past the window" rule.
// Fyne sizes a dialog by the MinSize of its content and a vertical scroll reports
// the full height of what it scrolls, so step 3 asked for a taller dialog than
// the launcher window and its form was cut by the window edge instead of
// scrolling inside the step.
func TestWizardContentFitsTheDialog(t *testing.T) {
	s := testShell(t)
	defer s.window.Close()
	s.window.Resize(launcherSize)

	size := wizardSize(s.window)
	// newWizard builds the steps without starting a background check, and a
	// check finishes with a fyne.Do update that would race with renderStep.
	w := s.newWizard(wizardTES3MP)
	defer w.popup.Hide()

	for _, step := range []wizardStep{wizardMorrowind, wizardTES3MP, wizardFonts} {
		w.step = step
		w.renderStep()
		min := w.popup.MinSize()
		if min.Width > size.Width || min.Height > size.Height {
			t.Fatalf("step %d asks for a %.0f×%.0f dialog, window dialog is %.0f×%.0f",
				int(step), min.Width, min.Height, size.Width, size.Height)
		}
	}
}

// TestWizardSizeShrinksWithTheWindow keeps the modal inside a window the user
// made smaller than the launcher default, and off the window edges otherwise.
func TestWizardSizeShrinksWithTheWindow(t *testing.T) {
	s := testShell(t)
	defer s.window.Close()

	s.window.Resize(fyne.NewSize(900, 620))
	size := wizardSize(s.window)
	if size.Width >= launcherSize.Width || size.Height >= launcherSize.Height {
		t.Fatalf("wizard size = %.0f×%.0f, want less than the launcher %.0f×%.0f",
			size.Width, size.Height, launcherSize.Width, launcherSize.Height)
	}

	s.window.Resize(launcherSize)
	size = wizardSize(s.window)
	if size.Width > launcherSize.Width || size.Height > launcherSize.Height {
		t.Fatalf("wizard size = %.0f×%.0f does not fit the window %.0f×%.0f",
			size.Width, size.Height, launcherSize.Width, launcherSize.Height)
	}
}

// TestUIVideoFormCardHoldsTheWholeForm keeps the settings.cfg fields inside one
// block, so the step can place them beside the sample instead of below it.
func TestUIVideoFormCardHoldsTheWholeForm(t *testing.T) {
	form := newUIVideoForm(openmwcfg.Defaults())
	card := uiVideoFormCard(i18n.New(i18n.English), form)

	entries := 0
	walkObjects(card, func(item fyne.CanvasObject) {
		if _, ok := item.(*widget.Entry); ok {
			entries++
		}
	})
	if entries != 4 {
		t.Fatalf("the form card has %d entries, want 4", entries)
	}
	texts := labelTexts(card)
	if !texts[i18n.New(i18n.English).T(i18n.KeyInstallUIVideoTitle)] {
		t.Fatalf("the form card has no heading: %v", texts)
	}
}

// labelTexts collects the visible captions of an object tree.
func labelTexts(object fyne.CanvasObject) map[string]bool {
	texts := make(map[string]bool)
	walkTexts(object, func(text string) {
		texts[text] = true
	})
	return texts
}

// TestWizardMorrowindFailureClassification is the T3E-ENV-01 rule: a folder
// that is not there is a warning plus the checked paths, while a folder the
// user pointed at keeps the code and Retry.
func TestWizardMorrowindFailureClassification(t *testing.T) {
	s := testShell(t)
	defer s.window.Close()
	tr := s.service.Translator()
	w := newTestWizard(t, s)

	w.morrowindFailure(fmt.Errorf("discover: %w", morrowind.ErrNotFound),
		func() { t.Error("a missing game must not offer Retry") })
	if got := w.morrowindStatus.Text; got != tr.T(i18n.KeySetupMorrowindNotFound) {
		t.Fatalf("status = %q, want the short missing line", got)
	}
	if w.morrowindStatus.Importance != widget.WarningImportance {
		t.Fatal("the missing line is not a warning")
	}
	if w.retry.Visible() {
		t.Fatal("Retry is visible for a missing game")
	}
	if !w.problem.Visible() || !strings.Contains(w.problem.Text, "discover") {
		t.Fatalf("the checked paths are not shown: %q", w.problem.Text)
	}
	if strings.Contains(w.problem.Text, "T3E-") {
		t.Fatal("a missing game carries an error code")
	}
	if !w.canAdvance() {
		t.Fatal("the step blocked after a missing game")
	}

	incomplete := &morrowind.IncompleteError{Root: "/games/Morrowind",
		Missing: []string{"Tribunal.esm"}}
	w.morrowindFailure(incomplete, func() {})
	if w.retryAction == nil {
		t.Fatal("an incomplete folder lost its Retry action")
	}
	if !strings.Contains(w.problem.Text, string(app.CodeMorrowindIncomplete)) {
		t.Fatalf("an incomplete folder lost its code: %q", w.problem.Text)
	}
	if !w.retry.Visible() {
		t.Fatal("an incomplete folder hides Retry")
	}
}

// TestWizardKeepsTypedMorrowindPath: the first step is skippable, so a typed
// path must survive "Next" even without a successful check.
func TestWizardKeepsTypedMorrowindPath(t *testing.T) {
	s := testShell(t)
	defer s.window.Close()
	w := newTestWizard(t, s)

	root := t.TempDir()
	w.morrowind.SetText(" " + root + " ")
	w.keepMorrowindPath()

	if got := s.service.Config().MorrowindPath; got != root {
		t.Fatalf("config MorrowindPath = %q, want %q", got, root)
	}
}

// TestWizardSearchStatusIsNotAnError keeps the auto-search on entry readable:
// the status names the running scan instead of staying empty.
func TestWizardSearchStatusIsNotAnError(t *testing.T) {
	s := testShell(t)
	defer s.window.Close()
	tr := s.service.Translator()
	w := newTestWizard(t, s)

	w.setMorrowindStatus(tr.T(i18n.KeySetupSearching), widget.MediumImportance)
	if w.morrowindStatus.Text != tr.T(i18n.KeySetupSearching) {
		t.Fatalf("status = %q, want the searching line", w.morrowindStatus.Text)
	}
	if w.morrowindStatus.Importance != widget.MediumImportance {
		t.Fatal("the searching line kept a warning colour")
	}
	if _, err := s.service.CheckMorrowind(""); !errors.Is(err,
		app.ErrMorrowindNotConfigured) {
		t.Fatalf("CheckMorrowind on an empty config = %v", err)
	}
}
