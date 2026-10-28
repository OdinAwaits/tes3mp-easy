package style

import (
	"image/color"
	"math"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

func TestThemeForForcesVariant(t *testing.T) {
	dark := ThemeFor(string(ModeDark))
	if got, want := dark.Color(theme.ColorNameBackground, theme.VariantLight),
		darkPalette[theme.ColorNameBackground]; got != want {
		t.Fatalf("dark background = %v, want %v", got, want)
	}
	light := ThemeFor(string(ModeLight))
	if got, want := light.Color(theme.ColorNameBackground, theme.VariantDark),
		lightPalette[theme.ColorNameBackground]; got != want {
		t.Fatalf("light background = %v, want %v", got, want)
	}
}

func TestPaletteContrast(t *testing.T) {
	for name, palette := range map[string]palette{
		"dark": darkPalette, "light": lightPalette,
	} {
		background := paletteColor(t, name, palette, theme.ColorNameBackground)
		card := paletteColor(t, name, palette, theme.ColorNameInputBackground)
		foreground := paletteColor(t, name, palette, theme.ColorNameForeground)
		button := paletteColor(t, name, palette, theme.ColorNameButton)

		if ratio := contrastRatio(background, foreground); ratio < 7 {
			t.Errorf("%s foreground contrast = %.2f, want >= 7", name, ratio)
		}
		primary := paletteColor(t, name, palette, theme.ColorNamePrimary)
		onPrimary := paletteColor(t, name, palette, theme.ColorNameForegroundOnPrimary)
		if ratio := contrastRatio(primary, onPrimary); ratio < 4.5 {
			t.Errorf("%s primary text contrast = %.2f, want >= 4.5", name, ratio)
		}
		if ratio := contrastRatio(foreground, button); ratio < 4.5 {
			t.Errorf("%s button text contrast = %.2f, want >= 4.5", name, ratio)
		}
		// The resting fill only has to read as a surface instead of a caption.
		// A WCAG 3:1 fill on a near-black window is #7b7262 — every secondary
		// control turns into a light tile, which the screenshots showed and the
		// density work cannot fix. The component boundary comes from the border
		// below and from the label, so the fill stays one visible step lighter.
		for surface, value := range map[string]color.NRGBA{
			"window": background, "card": card,
		} {
			if ratio := contrastRatio(button, value); ratio < 1.5 {
				t.Errorf("%s button/%s contrast = %.2f, want >= 1.5",
					name, surface, ratio)
			}
		}
		// An input is the case WCAG 1.4.11 really targets: without a border an
		// entry is invisible on the background.
		border := paletteColor(t, name, palette, theme.ColorNameInputBorder)
		if ratio := contrastRatio(border, card); ratio < 3 {
			t.Errorf("%s input border contrast = %.2f, want >= 3", name, ratio)
		}
		separator := paletteColor(t, name, palette, theme.ColorNameSeparator)
		if ratio := contrastRatio(separator, background); ratio < 2 {
			t.Errorf("%s card border contrast = %.2f, want >= 2", name, ratio)
		}
		// A disabled control must neither vanish nor pass for an active one.
		disabledFill := paletteColor(t, name, palette, theme.ColorNameDisabledButton)
		disabledText := paletteColor(t, name, palette, theme.ColorNameDisabled)
		if ratio := contrastRatio(disabledText, disabledFill); ratio < 4.5 {
			t.Errorf("%s disabled text contrast = %.2f, want >= 4.5", name, ratio)
		}
		if ratio := contrastRatio(disabledFill, button); ratio < 1.3 {
			t.Errorf("%s disabled/active button contrast = %.2f, want >= 1.3",
				name, ratio)
		}
		// Hover and focus are translucent blends over the resting fill; they
		// are state cues, so a visible delta is enough.
		for _, state := range []fyne.ThemeColorName{
			theme.ColorNameHover, theme.ColorNameFocus,
		} {
			blended := blendOver(paletteColor(t, name, palette, state), button)
			if ratio := contrastRatio(blended, button); ratio < 1.2 {
				t.Errorf("%s %s contrast = %.2f, want >= 1.2",
					name, state, ratio)
			}
		}
	}
}

func paletteColor(t *testing.T, themeName string, palette palette,
	name fyne.ThemeColorName,
) color.NRGBA {
	t.Helper()
	value, ok := palette[name].(color.NRGBA)
	if !ok {
		t.Fatalf("%s %s is not NRGBA", themeName, name)
	}
	return value
}

// blendOver composites an alpha overlay over an opaque surface, the way Fyne
// paints ColorNameHover and ColorNameFocus.
func blendOver(overlay, surface color.NRGBA) color.NRGBA {
	alpha := float64(overlay.A) / 255
	channel := func(top, bottom uint8) uint8 {
		return uint8(math.Round(float64(top)*alpha + float64(bottom)*(1-alpha)))
	}
	return color.NRGBA{
		R: channel(overlay.R, surface.R),
		G: channel(overlay.G, surface.G),
		B: channel(overlay.B, surface.B),
		A: 0xff,
	}
}

// TestThemeControlMetrics keeps the widget scale in one place: the button and
// the field share a radius, and a button stays 32 px high instead of the 36 px
// of the Fyne defaults.
func TestThemeControlMetrics(t *testing.T) {
	for _, mode := range []Mode{ModeDark, ModeLight} {
		th := ThemeFor(string(mode))
		radius := th.Size(theme.SizeNameButtonRadius)
		if radius < 8 {
			t.Errorf("%s button radius = %.0f, want >= 8", mode, radius)
		}
		if got := th.Size(theme.SizeNameInputRadius); got != radius {
			t.Errorf("%s input radius = %.0f, want the button radius %.0f",
				mode, got, radius)
		}
		if padding := th.Size(theme.SizeNameInnerPadding); padding > 6 {
			t.Errorf("%s inner padding = %.0f, want <= 6", mode, padding)
		}
		if icon := th.Size(theme.SizeNameInlineIcon); icon > 18 {
			t.Errorf("%s inline icon = %.0f, want <= 18", mode, icon)
		}
	}
}

func TestParseModeFallsBackToSystem(t *testing.T) {
	for value, want := range map[string]Mode{
		"dark": ModeDark, " LIGHT ": ModeLight, "": ModeSystem,
		"system": ModeSystem, "weird": ModeSystem,
	} {
		if got := ParseMode(value); got != want {
			t.Errorf("ParseMode(%q) = %q, want %q", value, got, want)
		}
	}
}

func contrastRatio(left, right color.NRGBA) float64 {
	lightLeft, lightRight := luminance(left), luminance(right)
	if lightLeft < lightRight {
		lightLeft, lightRight = lightRight, lightLeft
	}
	return (lightLeft + 0.05) / (lightRight + 0.05)
}

func luminance(value color.NRGBA) float64 {
	channel := func(raw uint8) float64 {
		value := float64(raw) / 255
		if value <= 0.03928 {
			return value / 12.92
		}
		return ((value + 0.055) / 1.055) * ((value + 0.055) / 1.055) * ((value + 0.055) / 1.055)
	}
	return 0.2126*channel(value.R) + 0.7152*channel(value.G) + 0.0722*channel(value.B)
}
