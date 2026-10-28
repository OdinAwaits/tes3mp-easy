package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/Saagilat/tes3mp-easy/internal/app"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/morrowind"
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
	w.spinner = widget.NewProgressBarInfinite()
	w.spinner.Hide()
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

	if !w.beginBusy() {
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
	if w.beginBusy() {
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
