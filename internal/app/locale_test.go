package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/config"
	"github.com/Saagilat/tes3mp-easy/internal/fonts"
	"github.com/Saagilat/tes3mp-easy/internal/locale"
)

func writeLocaleLayer(t *testing.T, workDir string, voiceover bool) {
	t.Helper()
	layer := filepath.Join(workDir, locale.DirName, "main", "Data Files")
	if err := os.MkdirAll(layer, 0o700); err != nil {
		t.Fatalf("create locale layer: %v", err)
	}
	if err := os.WriteFile(filepath.Join(layer, "Morrowind.esm"),
		[]byte("localized"), 0o600); err != nil {
		t.Fatalf("write localized Morrowind.esm: %v", err)
	}
	marker := filepath.Join(workDir, locale.DirName, ".locale.json")
	body := `{"language":"russian","main_sha256":"` + strings.Repeat("b", 64) + `"`
	if voiceover {
		body += `,"voices_sha256":"` + strings.Repeat("c", 64) + `","voiceover":true`
	}
	body += `}`
	if err := os.WriteFile(marker, []byte(body), 0o600); err != nil {
		t.Fatalf("write locale marker: %v", err)
	}
}

func TestInstallLocalizationWiring(t *testing.T) {
	service, workDir, _ := installedService(t)
	var captured locale.Options
	service.installLocale = func(_ context.Context, options locale.Options,
	) (locale.Result, error) {
		captured = options
		writeLocaleLayer(t, workDir, true)
		return locale.Result{
			State: locale.State{Installed: true,
				Dir: locale.Dir(workDir)},
		}, nil
	}
	fontsCalled := false
	service.installFonts = func(_ context.Context, _ fonts.Options,
	) (fonts.Result, error) {
		fontsCalled = true
		return fonts.Result{
			State:    fonts.State{Installed: true},
			Warnings: []string{"font warning"},
		}, nil
	}
	var progressCalls int
	result, err := service.InstallLocalization(context.Background(),
		locale.LanguageRussian, true, func(locale.Progress) { progressCalls++ })
	if err != nil {
		t.Fatalf("InstallLocalization: %v", err)
	}
	if !result.State.Installed || !fontsCalled {
		t.Fatalf("result = %+v, fontsCalled = %t", result, fontsCalled)
	}
	if len(result.Warnings) != 1 || result.Warnings[0] != "font warning" {
		t.Fatalf("warnings = %v", result.Warnings)
	}
	if captured.WorkDir != workDir ||
		captured.CacheDir != service.Paths().CacheDir ||
		captured.Language != locale.LanguageRussian || !captured.Voiceover {
		t.Fatalf("options = %+v", captured)
	}
	if captured.Progress == nil {
		t.Fatal("progress callback is nil")
	}
	captured.Progress(locale.Progress{Stage: locale.StageDownloading})
	if progressCalls != 1 {
		t.Fatalf("progress calls = %d", progressCalls)
	}
	state, err := service.LocalizationState()
	if err != nil || !state.Installed || state.Language != locale.LanguageRussian {
		t.Fatalf("LocalizationState = %+v, %v", state, err)
	}
}

func TestInstallLocalizationReportsFontFailureAsWarning(t *testing.T) {
	service, workDir, _ := installedService(t)
	service.installLocale = func(_ context.Context, _ locale.Options,
	) (locale.Result, error) {
		writeLocaleLayer(t, workDir, true)
		return locale.Result{State: locale.State{Installed: true}}, nil
	}
	service.installFonts = func(_ context.Context, _ fonts.Options,
	) (fonts.Result, error) {
		return fonts.Result{}, os.ErrDeadlineExceeded
	}
	result, err := service.InstallLocalization(context.Background(),
		locale.LanguageRussian, true, nil)
	if err != nil {
		t.Fatalf("InstallLocalization: %v", err)
	}
	if len(result.Warnings) != 1 ||
		!strings.Contains(result.Warnings[0], "Cyrillic fonts") {
		t.Fatalf("warnings = %v", result.Warnings)
	}
}

func TestInstallLocalizationFailureKeepsPreviousOverlay(t *testing.T) {
	service, workDir, _ := installedService(t)
	writeLocaleLayer(t, workDir, true)
	failure := errors.New("download failed")
	service.installLocale = func(_ context.Context, _ locale.Options,
	) (locale.Result, error) {
		return locale.Result{}, failure
	}
	fontsCalled := false
	service.installFonts = func(_ context.Context, _ fonts.Options,
	) (fonts.Result, error) {
		fontsCalled = true
		return fonts.Result{}, nil
	}
	if _, err := service.InstallLocalization(context.Background(),
		locale.LanguageRussian, true, nil); !errors.Is(err, failure) {
		t.Fatalf("error = %v, want download failure", err)
	}
	if fontsCalled {
		t.Fatal("fonts were installed after a failed localization")
	}
	state, err := service.LocalizationState()
	if err != nil || !state.Installed {
		t.Fatalf("previous overlay was lost: %+v, %v", state, err)
	}
}

func TestInstallLocalizationRejectsUnsupportedPackage(t *testing.T) {
	service, _, _ := installedService(t)
	if _, err := service.InstallLocalization(context.Background(),
		"english", true, nil); err == nil {
		t.Fatal("unsupported package accepted")
	}
}

func TestRemoveLocalizationDeletesGlobalOverlay(t *testing.T) {
	service, workDir, _ := installedService(t)
	writeLocaleLayer(t, workDir, true)
	if err := service.RemoveLocalization(); err != nil {
		t.Fatalf("RemoveLocalization: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, locale.DirName)); !os.IsNotExist(err) {
		t.Fatalf("localization directory survived removal: %v", err)
	}
	state, err := service.LocalizationState()
	if err != nil || state.Installed {
		t.Fatalf("LocalizationState = %+v, %v", state, err)
	}
}

func TestInstallLocalizationWithoutVoiceover(t *testing.T) {
	service, workDir, _ := installedService(t)
	var captured locale.Options
	service.installLocale = func(_ context.Context, options locale.Options,
	) (locale.Result, error) {
		captured = options
		return locale.Result{State: locale.State{Installed: true,
			Language: locale.LanguageRussian, Dir: locale.Dir(workDir)}}, nil
	}
	service.installFonts = func(_ context.Context, _ fonts.Options,
	) (fonts.Result, error) {
		return fonts.Result{}, nil
	}
	if _, err := service.InstallLocalization(context.Background(),
		locale.LanguageRussian, false, nil); err != nil {
		t.Fatalf("InstallLocalization: %v", err)
	}
	if captured.Voiceover {
		t.Fatal("voice archive was requested for a text-only install")
	}
}

func TestVoiceoverPreference(t *testing.T) {
	service, _, _ := installedService(t)
	if !service.Voiceover() {
		t.Fatal("voiceover is disabled by default")
	}
	if err := service.SetVoiceover(false); err != nil {
		t.Fatalf("SetVoiceover: %v", err)
	}
	if service.Voiceover() {
		t.Fatal("voiceover preference was not updated")
	}
	stored, err := config.Load(service.ConfigPath())
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if stored.Voiceover == nil || stored.VoiceoverEnabled() {
		t.Fatalf("stored voiceover = %v", stored.Voiceover)
	}
	if err := service.SetVoiceover(true); err != nil {
		t.Fatalf("SetVoiceover: %v", err)
	}
	if !service.Voiceover() {
		t.Fatal("voiceover preference was not restored")
	}
}
