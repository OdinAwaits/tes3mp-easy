package ui

import (
	"fmt"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// TestSwitchToggleFlipsOnTap covers the header control: a tap flips the pill and
// reports the new position exactly once.
func TestSwitchToggleFlipsOnTap(t *testing.T) {
	test.NewApp()
	var changes []bool
	toggle := newSwitchToggle(false, func(on bool) {
		changes = append(changes, on)
	})

	if toggle.Checked() {
		t.Fatal("a fresh switch is on")
	}
	toggle.Tapped(&fyne.PointEvent{})
	if !toggle.Checked() {
		t.Fatal("the first tap did not switch the theme on")
	}
	toggle.Tapped(&fyne.PointEvent{})
	if toggle.Checked() {
		t.Fatal("the second tap did not switch the theme off")
	}
	if got, want := fmt.Sprint(changes), "[true false]"; got != want {
		t.Fatalf("reported changes = %s, want %s", got, want)
	}
}

// TestSwitchToggleSetCheckedIsSilent keeps the undo path quiet: a rejected
// change puts the pill back without firing the callback a second time.
func TestSwitchToggleSetCheckedIsSilent(t *testing.T) {
	test.NewApp()
	calls := 0
	toggle := newSwitchToggle(false, func(bool) { calls++ })

	toggle.SetChecked(true)
	toggle.SetChecked(true)
	if !toggle.Checked() {
		t.Fatal("SetChecked(true) did not move the switch")
	}
	if calls != 0 {
		t.Fatalf("SetChecked reported %d changes, want none", calls)
	}
}

// TestSwitchToggleKeyboard keeps the switch usable without a pointer, the same
// way the checkbox it replaced was.
func TestSwitchToggleKeyboard(t *testing.T) {
	test.NewApp()
	toggle := newSwitchToggle(false, nil)

	toggle.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
	if !toggle.Checked() {
		t.Fatal("Space did not switch the theme on")
	}
	toggle.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
	if !toggle.Checked() {
		t.Fatal("Escape changed the switch")
	}
	toggle.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if toggle.Checked() {
		t.Fatal("Enter did not switch the theme off")
	}
	if !toggle.AcceptsTab() {
		t.Fatal("the switch cannot be reached with Tab")
	}
	var focusable fyne.Focusable = toggle
	focusable.TypedRune('x')
	focusable.FocusGained()
	focusable.FocusLost()
}

// TestSwitchToggleMovesTheKnob pins the two positions: the knob sits at the left
// edge when off, at the right edge when on, and the track stays a pill.
func TestSwitchToggleMovesTheKnob(t *testing.T) {
	test.NewApp()
	toggle := newSwitchToggle(false, nil)
	toggle.Resize(fyne.NewSize(switchTrackWidth, switchTrackHeight))

	renderer, ok := test.WidgetRenderer(toggle).(*switchRenderer)
	if !ok {
		t.Fatal("the switch has no pill renderer")
	}
	renderer.Refresh()
	if got, want := renderer.track.CornerRadius,
		switchTrackHeight/2; got != want {
		t.Fatalf("track corner radius = %.0f, want %.0f", got, want)
	}
	off := renderer.knob.Position().X

	toggle.SetChecked(true)
	on := renderer.knob.Position().X
	if on <= off {
		t.Fatalf("knob stays at %.0f px when on, was %.0f px when off", on, off)
	}
	toggle.SetChecked(false)
	if got := renderer.knob.Position().X; got != off {
		t.Fatalf("knob came back to %.0f px, want %.0f", got, off)
	}
}
