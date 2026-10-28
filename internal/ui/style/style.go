// Package style contains the amber/dark visual language of the Fyne client.
//
// It is deliberately separate from internal/config and internal/ui so palette
// changes and contrast checks can be unit-tested without a display.
package style

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Mode is a persisted theme preference.
type Mode string

const (
	ModeSystem Mode = "system"
	ModeDark   Mode = "dark"
	ModeLight  Mode = "light"
)

// ParseMode accepts the persisted config values and falls back to system.
func ParseMode(value string) Mode {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(ModeDark):
		return ModeDark
	case string(ModeLight):
		return ModeLight
	default:
		return ModeSystem
	}
}

// ThemeFor returns the amber theme for a persisted mode.
func ThemeFor(mode string) fyne.Theme {
	return amberTheme{mode: ParseMode(mode)}
}

type amberTheme struct {
	mode Mode
}

func (t amberTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch t.mode {
	case ModeDark:
		variant = theme.VariantDark
	case ModeLight:
		variant = theme.VariantLight
	}
	palette := darkPalette
	if variant == theme.VariantLight {
		palette = lightPalette
	}
	if value, ok := palette[name]; ok {
		return value
	}
	return theme.DefaultTheme().Color(name, variant)
}

func (t amberTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (t amberTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (t amberTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameButtonRadius, theme.SizeNameInputRadius:
		// One scale for buttons, fields and cards: square corners read as
		// bricks and do not match the 14 px card.
		return 8
	case theme.SizeNameInnerPadding:
		// 6 instead of the default 8 keeps a button 32 px high.
		return 6
	case theme.SizeNameInlineIcon:
		return 18
	}
	return theme.DefaultTheme().Size(name)
}

type palette map[fyne.ThemeColorName]color.Color

func rgba(value uint32) color.Color {
	return color.NRGBA{
		R: uint8(value >> 24),
		G: uint8(value >> 16),
		B: uint8(value >> 8),
		A: uint8(value),
	}
}

// darkPalette keeps the resting button fill one visible step above the card
// surface and raises borders and separators to 3:1 (WCAG 1.4.11). A 3:1 *fill*
// would turn every secondary control into a light tile: the boundary comes from
// the input border and the label instead. Hover and focus are translucent
// blends over the resting fill, so one step works on quiet and primary buttons.
var darkPalette = palette{
	theme.ColorNameBackground:                rgba(0x14110eff),
	theme.ColorNameButton:                    rgba(0x514433ff),
	theme.ColorNameDisabledButton:            rgba(0x332c22ff),
	theme.ColorNameForeground:                rgba(0xf2e7d5ff),
	theme.ColorNameDisabled:                  rgba(0xa8987fff),
	theme.ColorNamePlaceHolder:               rgba(0x9b8b77ff),
	theme.ColorNamePrimary:                   rgba(0xd8a548ff),
	theme.ColorNameForegroundOnPrimary:       rgba(0x1a140eff),
	theme.ColorNameHover:                     rgba(0xf2e7d533),
	theme.ColorNameFocus:                     rgba(0xd8a54899),
	theme.ColorNamePressed:                   rgba(0xd8a54833),
	theme.ColorNameSelection:                 rgba(0xd8a54855),
	theme.ColorNameInputBackground:           rgba(0x1d1813ff),
	theme.ColorNameInputBorder:               rgba(0x87785bff),
	theme.ColorNameMenuBackground:            rgba(0x1b1611ff),
	theme.ColorNameOverlayBackground:         rgba(0x1b1611ff),
	theme.ColorNameHeaderBackground:          rgba(0x1a1510ff),
	theme.ColorNameSeparator:                 rgba(0x635a4aff),
	theme.ColorNameShadow:                    rgba(0x00000099),
	theme.ColorNameScrollBar:                 rgba(0x6a5335ff),
	theme.ColorNameScrollBarBackground:       rgba(0x1a1510ff),
	theme.ColorNameHyperlink:                 rgba(0xe3b866ff),
	theme.ColorNameError:                     rgba(0xe06c5aff),
	theme.ColorNameForegroundOnError:         rgba(0x1a140eff),
	theme.ColorNameSuccess:                   rgba(0x7fae6aff),
	theme.ColorNameForegroundOnSuccess:       rgba(0x14200eff),
	theme.ColorNameWarning:                   rgba(0xe0b45aff),
	theme.ColorNameForegroundOnWarning:       rgba(0x1a140eff),
	theme.ColorNameInnerWindowBorder:         rgba(0x4a3a26ff),
	theme.ColorNameInnerWindowBorderInactive: rgba(0x2e251bff),
}

var lightPalette = palette{
	theme.ColorNameBackground:                rgba(0xf4efe6ff),
	theme.ColorNameButton:                    rgba(0xcbbda6ff),
	theme.ColorNameDisabledButton:            rgba(0xdfd6c6ff),
	theme.ColorNameForeground:                rgba(0x211a12ff),
	theme.ColorNameDisabled:                  rgba(0x625a4eff),
	theme.ColorNamePlaceHolder:               rgba(0x76685a6f),
	theme.ColorNamePrimary:                   rgba(0x8f5d12ff),
	theme.ColorNameForegroundOnPrimary:       rgba(0xfff8ecff),
	theme.ColorNameHover:                     rgba(0x211a1233),
	theme.ColorNameFocus:                     rgba(0x8f5d1266),
	theme.ColorNamePressed:                   rgba(0x8f5d1222),
	theme.ColorNameSelection:                 rgba(0x8f5d1233),
	theme.ColorNameInputBackground:           rgba(0xfffaf1ff),
	theme.ColorNameInputBorder:               rgba(0x9d9280ff),
	theme.ColorNameMenuBackground:            rgba(0xfbf7efff),
	theme.ColorNameOverlayBackground:         rgba(0xfbf7efff),
	theme.ColorNameHeaderBackground:          rgba(0xeee5d7ff),
	theme.ColorNameSeparator:                 rgba(0xab9c84ff),
	theme.ColorNameShadow:                    rgba(0x3a2e2044),
	theme.ColorNameScrollBar:                 rgba(0xb29a77ff),
	theme.ColorNameScrollBarBackground:       rgba(0xeee5d7ff),
	theme.ColorNameHyperlink:                 rgba(0x8f5d12ff),
	theme.ColorNameError:                     rgba(0xa83224ff),
	theme.ColorNameForegroundOnError:         rgba(0xfff8ecff),
	theme.ColorNameSuccess:                   rgba(0x3f6f2aff),
	theme.ColorNameForegroundOnSuccess:       rgba(0xfff8ecff),
	theme.ColorNameWarning:                   rgba(0x8a5a00ff),
	theme.ColorNameForegroundOnWarning:       rgba(0xfff8ecff),
	theme.ColorNameInnerWindowBorder:         rgba(0xc8b89fff),
	theme.ColorNameInnerWindowBorderInactive: rgba(0xd8cab4ff),
}

// Card wraps content in a rounded surface with a subtle border. The optional
// accent flag is used for the active server card.
func Card(content fyne.CanvasObject, accent bool) fyne.CanvasObject {
	surface := canvas.NewRectangle(theme.Color(theme.ColorNameInputBackground))
	// Wrapping text has a zero minimum width. A centered card would otherwise
	// collapse to the width of its widest button and let labels overflow.
	surface.SetMinSize(fyne.NewSize(480, 0))
	surface.CornerRadius = 14
	surface.StrokeColor = theme.Color(theme.ColorNameSeparator)
	surface.StrokeWidth = 1
	if accent {
		surface.StrokeColor = theme.Color(theme.ColorNamePrimary)
		surface.StrokeWidth = 1.5
	}
	return container.NewStack(surface, container.NewPadded(content))
}

// Heading renders a prominent title with theme-aware typography.
func Heading(text string) *widget.RichText {
	textWidget := widget.NewRichText(&widget.TextSegment{
		Style: widget.RichTextStyleHeading,
		Text:  text,
	})
	textWidget.Wrapping = fyne.TextWrapWord
	return textWidget
}

// CenteredHeading renders a heading centered within the available width.
func CenteredHeading(text string) *widget.RichText {
	textWidget := Heading(text)
	textWidget.Segments[0].(*widget.TextSegment).Style.Alignment = fyne.TextAlignCenter
	return textWidget
}

// Subheading renders a smaller section title.
func Subheading(text string) *widget.RichText {
	textWidget := widget.NewRichText(&widget.TextSegment{
		Style: widget.RichTextStyleSubHeading,
		Text:  text,
	})
	textWidget.Wrapping = fyne.TextWrapWord
	return textWidget
}

// MutedLabel returns a wrapping secondary label.
func MutedLabel(text string) *widget.Label {
	label := widget.NewLabel(text)
	label.Wrapping = fyne.TextWrapWord
	label.Importance = widget.LowImportance
	return label
}

// CenteredMutedLabel returns a wrapping secondary label centered within the
// available width.
func CenteredMutedLabel(text string) *widget.Label {
	label := MutedLabel(text)
	label.Alignment = fyne.TextAlignCenter
	return label
}
