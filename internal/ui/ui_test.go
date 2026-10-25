package ui

import (
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/config"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/locale"
	"github.com/Saagilat/tes3mp-easy/internal/mods"
	"github.com/Saagilat/tes3mp-easy/internal/tes3mp"
)

func TestRunRejectsNilService(t *testing.T) {
	if err := Run(nil); err == nil {
		t.Fatal("Run(nil) returned nil")
	}
}

func TestEndpointText(t *testing.T) {
	tests := []struct {
		profile config.Server
		want    string
	}{
		{profile: config.Server{}, want: ""},
		{profile: config.Server{Address: "example.com", Port: 25565},
			want: "example.com"},
		{profile: config.Server{Address: "example.com", Port: 25570},
			want: "example.com:25570"},
		{profile: config.Server{Address: "2001:db8::1", Port: 25570},
			want: "[2001:db8::1]:25570"},
		{profile: config.Server{Address: "[2001:db8::1]", Port: 25565},
			want: "2001:db8::1"},
	}
	for _, test := range tests {
		if got := endpointText(test.profile); got != test.want {
			t.Errorf("endpointText(%+v) = %q, want %q",
				test.profile, got, test.want)
		}
	}
}

func TestServerEndpoint(t *testing.T) {
	tests := []struct {
		profile config.Server
		want    string
	}{
		{profile: config.Server{}, want: ""},
		{profile: config.Server{Address: "example.com", Port: 25565},
			want: "example.com:25565"},
		{profile: config.Server{Address: "2001:db8::1", Port: 25567},
			want: "[2001:db8::1]:25567"},
		{profile: config.Server{Address: "[2001:db8::1]", Port: 25567},
			want: "[2001:db8::1]:25567"},
		{profile: config.Server{Address: "example.com"}, want: "example.com"},
	}
	for _, test := range tests {
		if got := serverEndpoint(test.profile); got != test.want {
			t.Errorf("serverEndpoint(%+v) = %q, want %q",
				test.profile, got, test.want)
		}
	}
}

func TestPlayButtonLabel(t *testing.T) {
	translator := i18n.New(i18n.English)
	tests := []struct {
		installed   bool
		needsUpdate bool
		want        string
	}{
		{installed: false, want: "Download mods and play"},
		{installed: true, needsUpdate: true, want: "Update and play"},
		{installed: true, want: "Play"},
	}
	for _, test := range tests {
		if got := playButtonLabel(translator, test.installed, test.needsUpdate); got != test.want {
			t.Errorf("playButtonLabel(%t, %t) = %q, want %q",
				test.installed, test.needsUpdate, got, test.want)
		}
	}
}

func TestThemeMode(t *testing.T) {
	translator := i18n.New(i18n.English)
	for _, mode := range []string{config.ThemeSystem, config.ThemeDark, config.ThemeLight} {
		if got := themeMode(translator, themeLabel(translator, mode)); got != mode {
			t.Errorf("themeMode(%q) = %q", mode, got)
		}
	}
	if got := themeMode(translator, "unknown"); got != config.ThemeSystem {
		t.Errorf("themeMode(unknown) = %q", got)
	}
}

func TestLocalizationLabels(t *testing.T) {
	translator := i18n.New(i18n.English)
	for _, pack := range locale.Packages() {
		label := localizationLabel(translator, pack.Language)
		if got := localizationValue(translator, label); got != pack.Language {
			t.Fatalf("localizationValue(%q) = %q, want %q",
				label, got, pack.Language)
		}
	}
	if got := localizationValue(translator, "unknown"); got != "" {
		t.Fatalf("localizationValue(unknown) = %q", got)
	}
}

func TestLocalizationNeedsInstall(t *testing.T) {
	tests := []struct {
		name     string
		state    locale.State
		language string
		want     bool
	}{
		{name: "not installed", language: locale.LanguageRussian, want: true},
		{name: "installed with voiceover", language: locale.LanguageRussian,
			state: locale.State{Installed: true, Language: locale.LanguageRussian,
				Voiceover: true}, want: false},
		{name: "installed without voiceover", language: locale.LanguageRussian,
			state: locale.State{Installed: true, Language: locale.LanguageRussian},
			want:  true},
		{name: "different package", language: locale.LanguageRussian,
			state: locale.State{Installed: true, Language: "other",
				Voiceover: true}, want: true},
	}
	for _, test := range tests {
		if got := localizationNeedsInstall(test.state, test.language); got != test.want {
			t.Errorf("%s: localizationNeedsInstall() = %t, want %t",
				test.name, got, test.want)
		}
	}
}

func TestLocalizationStateLabel(t *testing.T) {
	translator := i18n.New(i18n.English)
	russian := localizationLabel(translator, locale.LanguageRussian)
	tests := []struct {
		name  string
		state locale.State
		want  string
	}{
		{name: "missing", want: "not installed"},
		{name: "installed",
			state: locale.State{Installed: true, Language: locale.LanguageRussian},
			want:  russian + ", installed"},
		{name: "voiceover",
			state: locale.State{Installed: true, Language: locale.LanguageRussian,
				Voiceover: true},
			want: russian + ", installed with voiceover"},
	}
	for _, test := range tests {
		if got := localizationStateLabel(translator, test.state); got != test.want {
			t.Errorf("%s: localizationStateLabel() = %q, want %q",
				test.name, got, test.want)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	tests := map[int64]string{
		0:       "0 B",
		1023:    "1023 B",
		1024:    "1.0 KiB",
		1536:    "1.5 KiB",
		2 << 20: "2.0 MiB",
		3 << 30: "3.0 GiB",
	}
	for value, want := range tests {
		if got := formatBytes(value); got != want {
			t.Fatalf("formatBytes(%d) = %q, want %q", value, got, want)
		}
	}
}

func TestProgressFraction(t *testing.T) {
	tests := []struct {
		name   string
		update tes3mp.Progress
		want   float64
	}{
		{name: "other stage", update: tes3mp.Progress{
			Stage: tes3mp.StageExtracting}, want: 1,
		},
		{name: "unknown total", update: tes3mp.Progress{
			Stage: tes3mp.StageDownloading, Downloaded: 10}, want: 1,
		},
		{name: "half", update: tes3mp.Progress{
			Stage: tes3mp.StageDownloading, Downloaded: 5, Total: 10}, want: 0.5,
		},
		{name: "clamped", update: tes3mp.Progress{
			Stage: tes3mp.StageDownloading, Downloaded: 20, Total: 10}, want: 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := progressFraction(test.update); got != test.want {
				t.Fatalf("progressFraction() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestProgressLabel(t *testing.T) {
	translator := i18n.New(i18n.English)
	got := progressLabel(translator, tes3mp.Progress{
		Stage: tes3mp.StageDownloading, Downloaded: 1 << 20, Total: 2 << 20,
	})
	if got != "Downloading TES3MP: 1.0 MiB of 2.0 MiB" {
		t.Fatalf("download label = %q", got)
	}
	for stage, want := range map[tes3mp.Stage]string{
		tes3mp.StageVerifying:  "Checking the TES3MP archive...",
		tes3mp.StageExtracting: "Extracting TES3MP...",
		tes3mp.StageInstalling: "Finishing the TES3MP installation...",
	} {
		if got := progressLabel(translator, tes3mp.Progress{Stage: stage}); got != want {
			t.Fatalf("progressLabel(%q) = %q, want %q", stage, got, want)
		}
	}
}

func TestModsProgress(t *testing.T) {
	translator := i18n.New(i18n.English)
	update := mods.Progress{
		Stage: mods.StageDownloading, Downloaded: 1 << 20, Total: 2 << 20,
	}
	if got := modsProgressLabel(translator, update); got !=
		"Downloading mods: 1.0 MiB of 2.0 MiB" {
		t.Fatalf("mods progress label = %q", got)
	}
	if got := modsProgressFraction(update); got != 0.5 {
		t.Fatalf("mods progress fraction = %v", got)
	}
	if got := modsProgressFraction(mods.Progress{
		Stage: mods.StageExtracting}); got != 1 {
		t.Fatalf("extracting fraction = %v", got)
	}
	for stage, want := range map[mods.Stage]string{
		mods.StageFetching:   "Checking server mods...",
		mods.StageVerifying:  "Checking the mods archive...",
		mods.StageExtracting: "Extracting mods...",
		mods.StageInstalling: "Installing mods...",
	} {
		if got := modsProgressLabel(translator, mods.Progress{Stage: stage}); got != want {
			t.Fatalf("modsProgressLabel(%q) = %q, want %q", stage, got, want)
		}
	}
}

func TestShortSHA(t *testing.T) {
	if got := shortSHA("abcdef"); got != "abcdef" {
		t.Fatalf("shortSHA = %q", got)
	}
	if got := shortSHA("abcdefghijklmnop"); got != "abcdefghijkl…" {
		t.Fatalf("shortSHA = %q", got)
	}
}

func TestLocaleProgress(t *testing.T) {
	translator := i18n.New(i18n.English)
	update := locale.Progress{
		Stage: locale.StageDownloading, Downloaded: 1 << 20, Total: 2 << 20,
	}
	if got := localeProgressLabel(translator, update); got !=
		"Downloading language files: 1.0 MiB of 2.0 MiB" {
		t.Fatalf("locale progress label = %q", got)
	}
	if got := localeProgressFraction(update); got != 0.5 {
		t.Fatalf("locale progress fraction = %v", got)
	}
	if got := localeProgressFraction(locale.Progress{
		Stage: locale.StageExtracting}); got != 1 {
		t.Fatalf("extracting fraction = %v", got)
	}
	if got := localeProgressLabel(translator, locale.Progress{
		Stage: locale.StageDownloading, Downloaded: 10}); got !=
		"Downloading language files: 10 B of ?" {
		t.Fatalf("unknown total label = %q", got)
	}
	for stage, want := range map[locale.Stage]string{
		locale.StageExtracting: "Extracting language files...",
		locale.StageInstalling: "Applying language files...",
	} {
		if got := localeProgressLabel(translator,
			locale.Progress{Stage: stage}); got != want {
			t.Fatalf("localeProgressLabel(%q) = %q, want %q", stage, got, want)
		}
	}
}
