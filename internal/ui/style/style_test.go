package style

import (
	"image/color"
	"testing"

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
		background, ok := palette[theme.ColorNameBackground].(color.NRGBA)
		if !ok {
			t.Fatalf("%s background is not NRGBA", name)
		}
		foreground, ok := palette[theme.ColorNameForeground].(color.NRGBA)
		if !ok {
			t.Fatalf("%s foreground is not NRGBA", name)
		}
		if ratio := contrastRatio(background, foreground); ratio < 7 {
			t.Errorf("%s foreground contrast = %.2f, want >= 7", name, ratio)
		}
		primary, ok := palette[theme.ColorNamePrimary].(color.NRGBA)
		if !ok {
			t.Fatalf("%s primary is not NRGBA", name)
		}
		onPrimary, ok := palette[theme.ColorNameForegroundOnPrimary].(color.NRGBA)
		if !ok {
			t.Fatalf("%s on-primary is not NRGBA", name)
		}
		if ratio := contrastRatio(primary, onPrimary); ratio < 4.5 {
			t.Errorf("%s primary text contrast = %.2f, want >= 4.5", name, ratio)
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
