package ui

import (
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/config"
	"github.com/Saagilat/tes3mp-easy/internal/fonts"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/locale"
	"github.com/Saagilat/tes3mp-easy/internal/mods"
	"github.com/Saagilat/tes3mp-easy/internal/openmwcfg"
	"github.com/Saagilat/tes3mp-easy/internal/tes3mp"
)

func TestRunRejectsNilService(t *testing.T) {
	if err := Run(nil); err == nil {
		t.Fatal("Run(nil) returned nil")
	}
}

func TestAppMenuHasGlobalActions(t *testing.T) {
	menu := (&shell{}).appMenu(i18n.New(i18n.English))
	want := []string{"Language", "Theme", "Client log", "Copy report", "About"}
	var got []string
	for _, item := range menu.Items {
		if item.IsSeparator {
			continue
		}
		if item.Action == nil && item.ChildMenu == nil {
			t.Errorf("menu item %q has no action and no submenu", item.Label)
		}
		got = append(got, item.Label)
	}
	if len(got) != len(want) {
		t.Fatalf("appMenu labels = %q, want %q", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("appMenu labels = %q, want %q", got, want)
		}
	}
	theme := menu.Items[1]
	if theme.ChildMenu == nil || len(theme.ChildMenu.Items) != 3 {
		t.Fatalf("theme submenu has %v items, want 3", theme.ChildMenu)
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

func TestThemeMenuItems(t *testing.T) {
	translator := i18n.New(i18n.English)
	applied := ""
	dark := newThemeMenuItem(translator, i18n.KeyThemeDark, config.ThemeDark,
		config.ThemeLight, func(mode string) { applied = mode })
	if dark.Label != "Amber dark" {
		t.Fatalf("dark label = %q", dark.Label)
	}
	if dark.Checked {
		t.Fatal("dark item is checked while the current theme is light")
	}
	if dark.Action == nil {
		t.Fatal("dark item has no action")
	}
	dark.Action()
	if applied != config.ThemeDark {
		t.Fatalf("dark action applied %q", applied)
	}

	system := newThemeMenuItem(translator, i18n.KeyThemeSystem,
		config.ThemeSystem, config.ThemeSystem, func(string) {})
	if !system.Checked {
		t.Fatal("system item is not checked while the current theme is system")
	}
}

func TestRailInstallLabel(t *testing.T) {
	translator := i18n.New(i18n.English)
	if got := railInstallLabel(translator, 0); got != "Installation" {
		t.Fatalf("railInstallLabel(0) = %q", got)
	}
	if got := railInstallLabel(translator, 2); got != "Installation · 2" {
		t.Fatalf("railInstallLabel(2) = %q", got)
	}
}

func TestCountIssues(t *testing.T) {
	ready := installStatus{ready: true}
	pending := installStatus{}
	tests := []struct {
		name     string
		statuses []installStatus
		want     int
	}{
		{name: "empty", want: 0},
		{name: "all ready", statuses: []installStatus{ready, ready}, want: 0},
		{name: "mixed", statuses: []installStatus{ready, pending, pending}, want: 2},
	}
	for _, test := range tests {
		if got := countIssues(test.statuses...); got != test.want {
			t.Errorf("%s: countIssues() = %d, want %d", test.name, got, test.want)
		}
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
		voices   bool
		want     bool
	}{
		{name: "not installed", language: locale.LanguageRussian, voices: true,
			want: true},
		{name: "installed with voiceover", language: locale.LanguageRussian,
			voices: true,
			state: locale.State{Installed: true, Language: locale.LanguageRussian,
				Voiceover: true}, want: false},
		{name: "installed without voiceover", language: locale.LanguageRussian,
			voices: true,
			state:  locale.State{Installed: true, Language: locale.LanguageRussian},
			want:   true},
		{name: "text only matches request", language: locale.LanguageRussian,
			state: locale.State{Installed: true, Language: locale.LanguageRussian},
			want:  false},
		{name: "voiceover must be removed", language: locale.LanguageRussian,
			state: locale.State{Installed: true, Language: locale.LanguageRussian,
				Voiceover: true}, want: true},
		{name: "different package", language: locale.LanguageRussian,
			voices: true,
			state: locale.State{Installed: true, Language: "other",
				Voiceover: true}, want: true},
	}
	for _, test := range tests {
		if got := localizationNeedsInstall(test.state, test.language,
			test.voices); got != test.want {
			t.Errorf("%s: localizationNeedsInstall() = %t, want %t",
				test.name, got, test.want)
		}
	}
}

func TestLocalizationInstallLabel(t *testing.T) {
	translator := i18n.New(i18n.English)
	russian := localizationLabel(translator, locale.LanguageRussian)
	tests := []struct {
		name  string
		state locale.State
		want  string
	}{
		{name: "text",
			state: locale.State{Installed: true, Language: locale.LanguageRussian},
			want:  russian},
		{name: "voiceover",
			state: locale.State{Installed: true, Language: locale.LanguageRussian,
				Voiceover: true},
			want: russian + " + voiceover"},
	}
	for _, test := range tests {
		if got := localizationInstallLabel(translator, test.state); got != test.want {
			t.Errorf("%s: localizationInstallLabel() = %q, want %q",
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

func TestWizardStepsAreTranslatedAndOrdered(t *testing.T) {
	translator := i18n.New(i18n.English)
	want := []wizardStep{
		wizardMorrowind, wizardTES3MP, wizardFonts, wizardLocalization,
		wizardUIVideo, wizardReady,
	}
	if int(wizardStepCount) != len(want) {
		t.Fatalf("wizard has %d steps, want %d", wizardStepCount, len(want))
	}
	seen := make(map[string]wizardStep, len(want))
	for _, step := range want {
		key := wizardTitleKey(step)
		if key == "" {
			t.Fatalf("step %d has no title key", step)
		}
		if got := translator.T(key); got == key {
			t.Fatalf("step %d title %q is not translated", step, key)
		}
		if other, ok := seen[key]; ok {
			t.Fatalf("steps %d and %d share title %q", other, step, key)
		}
		seen[key] = step
	}
}

func TestFontSetChooserMapsLabelsToIDs(t *testing.T) {
	tr := i18n.New(i18n.English)
	var changed []string
	chooser := newFontSetChooser(tr, fonts.SetPlex, func(id string) {
		changed = append(changed, id)
	})
	if got := chooser.selected(); got != fonts.SetPlex {
		t.Fatalf("selected() = %q, want %q", got, fonts.SetPlex)
	}
	if len(changed) != 0 {
		t.Fatalf("preselect fired the callback: %q", changed)
	}
	chooser.group.SetSelected(fontSetLabel(tr, fonts.SetInter))
	if got := chooser.selected(); got != fonts.SetInter {
		t.Fatalf("selected() = %q, want %q", got, fonts.SetInter)
	}
	if len(changed) != 1 || changed[0] != fonts.SetInter {
		t.Fatalf("callback = %q, want [%s]", changed, fonts.SetInter)
	}
	for _, id := range fonts.SetIDs() {
		if _, ok := fontSetLabelKeys[id]; !ok {
			t.Fatalf("set %s has no translation", id)
		}
		if label := fontSetLabel(tr, id); label == "" || label == id {
			t.Fatalf("fontSetLabel(%s) = %q", id, label)
		}
	}
	if got := fontSetLabel(tr, "bogus"); got != fonts.DefaultSet {
		t.Fatalf("fontSetLabel(bogus) = %q, want the default id", got)
	}
}

func TestUIVideoFormParsesAndValidates(t *testing.T) {
	form := newUIVideoForm(openmwcfg.Settings{
		TTFResolution:  20,
		FontSize:       18,
		ScalingFactor:  1.25,
		FramerateLimit: 120,
	})
	values, err := form.values()
	if err != nil {
		t.Fatalf("values: %v", err)
	}
	if values.TTFResolution != 20 || values.FontSize != 18 ||
		values.ScalingFactor != 1.25 || values.FramerateLimit != 120 {
		t.Fatalf("values = %+v", values)
	}

	form.framerate.SetText("unlimited")
	if _, err := form.values(); err == nil {
		t.Fatal("a non-numeric framerate was accepted")
	}
	form.framerate.SetText("1000")
	if _, err := form.values(); err == nil {
		t.Fatal("a framerate above the limit was accepted")
	}
	form.framerate.SetText("60")
	form.fontSize.SetText("16")
	if _, err := form.values(); err != nil {
		t.Fatalf("values: %v", err)
	}
}
