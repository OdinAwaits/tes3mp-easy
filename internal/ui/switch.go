package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// The switch is a 44×24 pill with a round knob. A box with a tick reads as
// "pick any number of these", which is why the theme toggle looked like a form
// field; a pill reads as "this setting is on" and matches the 32 px list that
// stands next to it in the header.
const (
	switchTrackWidth  float32 = 44
	switchTrackHeight float32 = 24
	switchKnobGap     float32 = 3
)

// switchToggle is the two-position control of the header. It is drawn by hand
// because Fyne ships a checkbox and no switch; the state, the callback and the
// keyboard handling stay as small as a checkbox's.
type switchToggle struct {
	widget.BaseWidget

	on       bool
	onChange func(bool)
	focused  bool
}

// newSwitchToggle builds the control. onChange may be nil; it receives the new
// position after a tap or a key press, never after SetChecked.
func newSwitchToggle(on bool, onChange func(bool)) *switchToggle {
	toggle := &switchToggle{on: on, onChange: onChange}
	toggle.ExtendBaseWidget(toggle)
	return toggle
}

// Checked reports the position the pill is drawn in.
func (t *switchToggle) Checked() bool {
	return t.on
}

// SetChecked moves the pill without calling onChange: the caller already knows
// and a rejected change must be undone without a second callback.
func (t *switchToggle) SetChecked(on bool) {
	if t.on == on {
		return
	}
	t.on = on
	t.Refresh()
}

// Toggle flips the pill and reports the new position.
func (t *switchToggle) Toggle() {
	t.SetChecked(!t.on)
	if t.onChange != nil {
		t.onChange(t.on)
	}
}

func (t *switchToggle) Tapped(*fyne.PointEvent) {
	t.Toggle()
}

func (t *switchToggle) FocusGained() {
	t.focused = true
	t.Refresh()
}

func (t *switchToggle) FocusLost() {
	t.focused = false
	t.Refresh()
}

func (t *switchToggle) AcceptsTab() bool {
	return true
}

// TypedRune is part of fyne.Focusable: the switch takes no text, so a typed
// character is ignored instead of being swallowed by the header.
func (t *switchToggle) TypedRune(rune) {}

// TypedKey keeps the switch usable without a pointer, the same way a checkbox
// does: Space and Enter flip it, every other key is left to the window.
func (t *switchToggle) TypedKey(event *fyne.KeyEvent) {
	switch event.Name {
	case fyne.KeySpace, fyne.KeyReturn, fyne.KeyEnter:
		t.Toggle()
	}
}

func (t *switchToggle) CreateRenderer() fyne.WidgetRenderer {
	return &switchRenderer{
		toggle: t,
		track:  canvas.NewRectangle(nil),
		knob:   canvas.NewCircle(nil),
	}
}

type switchRenderer struct {
	toggle *switchToggle
	track  *canvas.Rectangle
	knob   *canvas.Circle
}

func (r *switchRenderer) MinSize() fyne.Size {
	return fyne.NewSize(switchTrackWidth, switchTrackHeight)
}

func (r *switchRenderer) Layout(fyne.Size) {
	height := switchTrackHeight
	r.track.CornerRadius = height / 2
	r.track.Resize(fyne.NewSize(switchTrackWidth, height))

	knobSize := height - 2*switchKnobGap
	knobX := switchKnobGap
	if r.toggle.on {
		knobX = switchTrackWidth - knobSize - switchKnobGap
	}
	r.knob.Resize(fyne.NewSize(knobSize, knobSize))
	r.knob.Move(fyne.NewPos(knobX, switchKnobGap))
}

// Refresh recolours the pill for the current position: the amber primary when
// on, the quiet input surface when off, and a focus ring instead of the border
// while the keyboard is on the control.
func (r *switchRenderer) Refresh() {
	if r.toggle.on {
		r.track.FillColor = theme.Color(theme.ColorNamePrimary)
		r.track.StrokeColor = color.Transparent
		r.track.StrokeWidth = 0
		r.knob.FillColor = theme.Color(theme.ColorNameForegroundOnPrimary)
	} else {
		r.track.FillColor = theme.Color(theme.ColorNameInputBackground)
		r.track.StrokeColor = theme.Color(theme.ColorNameInputBorder)
		r.track.StrokeWidth = 1
		r.knob.FillColor = theme.Color(theme.ColorNameForeground)
	}
	if r.toggle.focused {
		r.track.StrokeColor = theme.Color(theme.ColorNameFocus)
		r.track.StrokeWidth = 2
	}
	r.Layout(r.toggle.Size())
	r.track.Refresh()
	r.knob.Refresh()
}

func (r *switchRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.track, r.knob}
}

func (r *switchRenderer) Destroy() {}
