package ui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/app"
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

func TestThemeToggleLabels(t *testing.T) {
	for _, language := range i18n.Supported() {
		translator := i18n.New(language)
		if got := translator.T(i18n.KeySettingsTheme); got == "" {
			t.Fatalf("%s has no theme caption", language)
		}
		if got := translator.T(i18n.KeySettingsDarkTheme); got == "" {
			t.Fatalf("%s has no dark theme toggle caption", language)
		}
		if got := translator.T(i18n.KeySettingsLanguage); got == "" {
			t.Fatalf("%s has no language caption", language)
		}
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

func TestThemeLabels(t *testing.T) {
	translator := i18n.New(i18n.English)
	for key, want := range map[string]string{
		i18n.KeySettingsTheme:     "Theme",
		i18n.KeySettingsLanguage:  "Language",
		i18n.KeySettingsDarkTheme: "Dark theme",
	} {
		if got := translator.T(key); got != want {
			t.Fatalf("%q = %q, want %q", key, got, want)
		}
	}
}

func TestLocalizationLabels(t *testing.T) {
	translator := i18n.New(i18n.English)
	for _, pack := range locale.Packages() {
		label := localizationLabel(translator, pack.Language)
		got, ok := localizationPack(translator, label)
		if !ok || got.Language != pack.Language {
			t.Fatalf("localizationPack(%q) = %q, %t; want %q",
				label, got.Language, ok, pack.Language)
		}
	}
	if _, ok := localizationPack(translator, "unknown"); ok {
		t.Fatal("localizationPack(unknown) matched a pack")
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
			want:  russian + " · text"},
		{name: "voiceover",
			state: locale.State{Installed: true, Language: locale.LanguageRussian,
				Voiceover: true},
			want: russian + " · text + voice"},
	}
	for _, test := range tests {
		if got := localizationInstallLabel(translator, test.state); got != test.want {
			t.Errorf("%s: localizationInstallLabel() = %q, want %q",
				test.name, got, test.want)
		}
	}
}

func TestLocalizationStatusText(t *testing.T) {
	translator := i18n.New(i18n.English)
	russian := localizationLabel(translator, locale.LanguageRussian)
	tests := []struct {
		name  string
		state locale.State
		want  string
	}{
		{name: "missing",
			want: "No pack installed — the game keeps the original Morrowind language."},
		{name: "installed",
			state: locale.State{Installed: true, Language: locale.LanguageRussian},
			want:  "Installed: " + russian + " · text"},
		{name: "voiceover",
			state: locale.State{Installed: true, Language: locale.LanguageRussian,
				Voiceover: true},
			want: "Installed: " + russian + " · text + voice"},
	}
	for _, test := range tests {
		if got := localizationStatusText(translator, test.state); got != test.want {
			t.Errorf("%s: localizationStatusText() = %q, want %q",
				test.name, got, test.want)
		}
	}
}

func TestLocaleVariantHintUsesPackSizes(t *testing.T) {
	translator := i18n.New(i18n.English)
	pack, ok := locale.PackFor(locale.LanguageRussian)
	if !ok {
		t.Fatal("the Russian pack is missing")
	}
	text := localeVariantHint(translator, pack, false)
	if !strings.Contains(text, formatBytes(pack.TextSize)) ||
		!strings.Contains(text, pack.Encoding) {
		t.Fatalf("text hint = %q", text)
	}
	voices := localeVariantHint(translator, pack, true)
	if !strings.Contains(voices, formatBytes(pack.TextSize+pack.VoicesSize)) {
		t.Fatalf("voices hint = %q", voices)
	}
	quiet := locale.Pack{TextSize: 1 << 20, Encoding: "utf-8"}
	if got := localeVariantHint(translator, quiet, true); !strings.Contains(
		got, "utf-8") {
		t.Fatalf("pack without voices hint = %q", got)
	}
}

func TestUIVideoFieldsUseOpenMWRanges(t *testing.T) {
	form := newUIVideoForm(openmwcfg.Defaults())
	fields := form.fields()
	if len(fields) != 4 {
		t.Fatalf("fields() returned %d entries, want 4", len(fields))
	}
	want := []string{
		"16–48", "8–32", "0.5–4", "0–360",
	}
	translator := i18n.New(i18n.English)
	for index, field := range fields {
		if field.rangeText != want[index] {
			t.Errorf("field %d range = %q, want %q",
				index, field.rangeText, want[index])
		}
		if got := translator.T(field.hintKey); got == field.hintKey {
			t.Errorf("field %d hint %q is not translated", index, field.hintKey)
		}
		if field.entry == nil {
			t.Errorf("field %d has no entry", index)
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
			Stage: tes3mp.StageDownloading, Downloaded: 10}, want: 0,
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

func TestFontProgress(t *testing.T) {
	translator := i18n.New(i18n.English)
	update := fonts.Progress{
		Stage: fonts.StageDownloading, Asset: "NotoSans-Regular.ttf",
		Downloaded: 1 << 20, Total: 2 << 20,
	}
	if got := fontProgressLabel(translator, update); got !=
		"Downloading NotoSans-Regular.ttf… 1.0 MiB of 2.0 MiB" {
		t.Fatalf("font progress label = %q", got)
	}
	if got := fontProgressFraction(update); got != 0.5 {
		t.Fatalf("font progress fraction = %v", got)
	}
	if got := fontProgressFraction(fonts.Progress{
		Stage: fonts.StageVerifying}); got != 1 {
		t.Fatalf("verifying fraction = %v", got)
	}
	if got := fontProgressLabel(translator, fonts.Progress{
		Stage: fonts.StageDownloading, Asset: "NotoSans-Regular.ttf",
		Downloaded: 10}); got != "Downloading NotoSans-Regular.ttf…" {
		t.Fatalf("unknown total label = %q", got)
	}
	if got := fontProgressLabel(translator, fonts.Progress{
		Stage: fonts.StageInstalling, Asset: "NotoSans-Regular.ttf"}); got !=
		"Installing NotoSans-Regular.ttf…" {
		t.Fatalf("installing label = %q", got)
	}
	if got := fontProgressLabel(translator, fonts.Progress{
		Stage: fonts.StageVerifying}); got != "Verifying the font set…" {
		t.Fatalf("verifying label = %q", got)
	}
}

func TestFontProgressRefreshDue(t *testing.T) {
	base := fonts.Progress{
		Stage: fonts.StageDownloading, Asset: "NotoSans-Regular.ttf",
		Downloaded: 1 << 20, Total: 20 << 20,
	}
	if !fontProgressRefreshDue(fonts.Progress{}, base) {
		t.Fatal("first update must redraw")
	}
	small := base
	small.Downloaded = base.Downloaded + base.Total/100
	if fontProgressRefreshDue(base, small) {
		t.Fatal("1% update must be throttled")
	}
	five := base
	five.Downloaded = base.Downloaded + base.Total/20
	if !fontProgressRefreshDue(base, five) {
		t.Fatal("5% update must redraw")
	}
	if !fontProgressRefreshDue(base, fonts.Progress{
		Stage: fonts.StageDownloading, Asset: "NotoSerif-Regular.ttf",
		Downloaded: 1 << 20, Total: 20 << 20}) {
		t.Fatal("next file must redraw")
	}
	if fontProgressRefreshDue(fonts.Progress{Stage: fonts.StageVerifying},
		fonts.Progress{Stage: fonts.StageVerifying}) {
		t.Fatal("repeated verifying update must be throttled")
	}
}

func TestCancelFontDownloadClearsProgress(t *testing.T) {
	s := &shell{}
	var canceled bool
	s.fonts.downloading = true
	s.fonts.cancel = func() { canceled = true }
	s.fonts.progress = fonts.Progress{
		Stage: fonts.StageDownloading, Asset: "NotoSans-Regular.ttf",
		Downloaded: 1 << 20, Total: 2 << 20,
	}
	s.fonts.shown = s.fonts.progress
	s.fonts.warnings = []string{"font NotoSerif.ttf: timeout"}

	s.cancelFontDownload()

	snapshot := s.fontState()
	if !canceled {
		t.Fatal("cancel function was not called")
	}
	if snapshot.downloading {
		t.Fatal("download is still marked as running")
	}
	if !snapshot.canceled {
		t.Fatal("canceled run is not marked as canceled")
	}
	if snapshot.hasProgress {
		t.Fatal("progress was not cleared")
	}
	if len(snapshot.warnings) != 0 {
		t.Fatalf("warnings = %v, want none", snapshot.warnings)
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
		wizardMorrowind, wizardTES3MP, wizardFonts,
	}
	if int(wizardStepCount) != len(want) {
		t.Fatalf("wizard has %d steps, want %d", wizardStepCount, len(want))
	}
	if got := wizardTitleKey(wizardStepCount - 1); got !=
		i18n.KeyWizardFontsTitle {
		t.Fatalf("last step title key = %q, want %q", got,
			i18n.KeyWizardFontsTitle)
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
	chooser.picker.SetSelected(fontSetLabel(tr, fonts.SetInter))
	if got := chooser.selected(); got != fonts.SetInter {
		t.Fatalf("selected() = %q, want %q", got, fonts.SetInter)
	}
	if len(changed) != 1 || changed[0] != fonts.SetInter {
		t.Fatalf("callback = %q, want [%s]", changed, fonts.SetInter)
	}
	// Every set is labelled by its catalog name, so a new family cannot land
	// in the dropdown as a bare identifier.
	for _, id := range fonts.SetIDs() {
		set, ok := fonts.LookupSet(id)
		if !ok {
			t.Fatalf("LookupSet(%s) failed", id)
		}
		if label := fontSetLabel(tr, id); !strings.Contains(label, set.Name) {
			t.Fatalf("fontSetLabel(%s) = %q, want the catalog name %q",
				id, label, set.Name)
		}
	}
	def := fontSetLabel(tr, fonts.DefaultSet)
	if !strings.HasSuffix(def, "(default)") {
		t.Fatalf("default label = %q, want the translated suffix", def)
	}
	if got := fontSetLabel(tr, "bogus"); got != def {
		t.Fatalf("fontSetLabel(bogus) = %q, want %q", got, def)
	}
}

// errTestFontDownload stands in for a failed preview download.
var errTestFontDownload = errors.New("download failed")

func writePreviewFonts(t *testing.T, dir string, assets []fonts.Asset) {
	t.Helper()
	for _, asset := range assets {
		if err := os.WriteFile(filepath.Join(dir, asset.Name),
			[]byte("pretend "+asset.Name), 0o600); err != nil {
			t.Fatalf("write %s: %v", asset.Name, err)
		}
	}
}

func TestFontPreviewShowsEverySlotFile(t *testing.T) {
	tr := i18n.New(i18n.English)
	dir := t.TempDir()
	assets := fonts.ResolveSet(fonts.SetInter).Assets
	preview := newFontPreview(tr, fontPreviewFiles{dir: dir}, fonts.SetInter)
	if len(preview.slots) != len(assets) {
		t.Fatalf("slots = %d, want %d", len(preview.slots), len(assets))
	}
	if got := preview.slots[0].text.FontSource.Name(); got == assets[0].Name {
		t.Fatalf("preview used %q before the set was installed", got)
	}
	if !preview.slots[0].status.Visible() {
		t.Fatal("a missing file must be named, not silently replaced")
	}
	writePreviewFonts(t, dir, assets)
	preview.selectSet(fonts.SetInter)
	for index, slot := range preview.slots {
		if got := slot.text.FontSource.Name(); got != assets[index].Name {
			t.Fatalf("slot %d font = %q, want %q", index, got,
				assets[index].Name)
		}
		if slot.status.Visible() {
			t.Fatalf("slot %d still reports %q", index, slot.status.Text)
		}
	}
	if !strings.Contains(preview.slots[0].text.Text, "Съешь") {
		t.Fatalf("interface sample = %q, want Latin and Cyrillic",
			preview.slots[0].text.Text)
	}
	if !strings.Contains(preview.slots[2].text.Text, "AddItem") {
		t.Fatalf("console sample = %q, want a console line",
			preview.slots[2].text.Text)
	}
}

func TestFontPreviewDownloadsMissingSet(t *testing.T) {
	tr := i18n.New(i18n.English)
	fontsDir := t.TempDir()
	cacheDir := t.TempDir()
	assets := fonts.ResolveSet(fonts.SetPlex).Assets
	var asked []string
	preview := newFontPreview(tr,
		fontPreviewFiles{dir: fontsDir, cacheDir: cacheDir}, fonts.SetPlex)
	preview.async = func(work func(), done func()) {
		work()
		done()
	}
	preview.files.ensure = func(setID string) error {
		asked = append(asked, setID)
		writePreviewFonts(t, cacheDir, assets)
		return nil
	}
	preview.selectSet(fonts.SetPlex)
	if len(asked) != 1 || asked[0] != fonts.SetPlex {
		t.Fatalf("ensure calls = %v, want one for %q", asked, fonts.SetPlex)
	}
	for index, slot := range preview.slots {
		if got := slot.text.FontSource.Name(); got != assets[index].Name {
			t.Fatalf("slot %d font = %q, want the cached %q", index, got,
				assets[index].Name)
		}
	}
	// A second draw of the same set reads the cache instead of the network.
	preview.selectSet(fonts.SetPlex)
	if len(asked) != 1 {
		t.Fatalf("ensure calls = %v, want no repeat download", asked)
	}
}

func TestFontPreviewNamesADownloadFailure(t *testing.T) {
	tr := i18n.New(i18n.English)
	assets := fonts.ResolveSet(fonts.SetInter).Assets
	preview := newFontPreview(tr,
		fontPreviewFiles{dir: t.TempDir(), cacheDir: t.TempDir()},
		fonts.SetInter)
	preview.async = func(work func(), done func()) {
		work()
		done()
	}
	preview.files.ensure = func(string) error {
		return errTestFontDownload
	}
	preview.selectSet(fonts.SetInter)
	if preview.downloading {
		t.Fatal("preview is still downloading after the failure")
	}
	for index, slot := range preview.slots {
		if got := slot.text.FontSource.Name(); got == assets[index].Name {
			t.Fatalf("slot %d invented %q", index, got)
		}
		if !slot.status.Visible() ||
			!strings.Contains(slot.status.Text, errTestFontDownload.Error()) {
			t.Fatalf("slot %d status = %q", index, slot.status.Text)
		}
	}
	if !preview.retry.Visible() {
		t.Fatal("a failed download must offer Retry")
	}
}

func TestFontPreviewSizeFollowsSettings(t *testing.T) {
	tr := i18n.New(i18n.English)
	dir := t.TempDir()
	writePreviewFonts(t, dir, fonts.ResolveSet(fonts.SetInter).Assets)
	preview := newFontPreview(tr, fontPreviewFiles{dir: dir}, fonts.SetInter)
	if got := preview.slots[0].text.TextSize; got != fontPreviewDefaultSize {
		t.Fatalf("sample size = %v, want the default %d", got,
			fontPreviewDefaultSize)
	}
	if preview.sizeNote.Visible() {
		t.Fatal("the size line belongs to a screen with size fields")
	}

	preview.setScaling(16, 1.25)
	if got := preview.slots[2].text.TextSize; got != 20 {
		t.Fatalf("sample size = %v, want 20", got)
	}
	if !strings.Contains(preview.sizeNote.Text, "16 × 1.25 = 20 px") {
		t.Fatalf("size note = %q", preview.sizeNote.Text)
	}

	preview.setScaling(32, 4)
	if got := preview.slots[0].text.TextSize; got != fontPreviewMaxSize {
		t.Fatalf("sample size = %v, want the clamp %d", got,
			fontPreviewMaxSize)
	}
	if !strings.Contains(preview.sizeNote.Text, "128") ||
		!strings.Contains(preview.sizeNote.Text, "32 px") {
		t.Fatalf("clamped note = %q, want the real and the shown size",
			preview.sizeNote.Text)
	}

	preview.setScaling(0, 1)
	if got := preview.slots[0].text.TextSize; got != fontPreviewMaxSize {
		t.Fatalf("an invalid size changed the sample to %v", got)
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
	for value, want := range map[float64]string{
		1: "1.00", 1.25: "1.25", 0.5: "0.5",
	} {
		if got := formatScaling(value); got != want {
			t.Errorf("formatScaling(%v) = %q, want %q", value, got, want)
		}
	}
}

func TestFooterVersion(t *testing.T) {
	got := footerVersion(i18n.New(i18n.English))
	for _, want := range []string{app.Version, app.TES3MPVersion} {
		if !strings.Contains(got, want) {
			t.Fatalf("footerVersion = %q, want it to contain %q", got, want)
		}
	}
	russian := footerVersion(i18n.New(i18n.Russian))
	if !strings.Contains(russian, app.Version) {
		t.Fatalf("footerVersion(ru) = %q, want the client version", russian)
	}
}
