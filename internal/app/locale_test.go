package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/locale"
)

func TestInstallLocalizationAppliesAndRemovesOverlay(t *testing.T) {
	service, workDir, id := installedService(t)
	installFakeMods(t, workDir, id)
	writeLocaleLayer(t, workDir, id)
	var captured locale.Options
	service.installLocale = func(_ context.Context, options locale.Options,
	) (locale.Result, error) {
		captured = options
		return locale.Result{State: locale.State{Installed: true}}, nil
	}
	result, err := service.InstallLocalization(context.Background(), id, true, nil)
	if err != nil {
		t.Fatalf("InstallLocalization: %v", err)
	}
	if !result.State.Installed {
		t.Fatalf("result = %+v", result)
	}
	if captured.WorkDir != workDir || captured.CacheDir != service.Paths().CacheDir ||
		captured.ServerID != id || !captured.Voiceover {
		t.Fatalf("options = %+v", captured)
	}
	if !service.Config().Localization {
		t.Fatal("localization setting was not saved")
	}
	state, err := service.LocalizationState(id)
	if err != nil || !state.Installed {
		t.Fatalf("LocalizationState = %+v, %v", state, err)
	}
	dataFiles := filepath.Join(workDir, "servers", id, "data", "Data Files")
	body, err := os.ReadFile(filepath.Join(dataFiles, "Morrowind.esm"))
	if err != nil || string(body) != "localized" {
		t.Fatalf("profile Morrowind.esm = %q, %v", body, err)
	}

	warnings, err := service.RemoveLocalization(id)
	if err != nil {
		t.Fatalf("RemoveLocalization: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
	body, err = os.ReadFile(filepath.Join(dataFiles, "Morrowind.esm"))
	if err != nil || string(body) != "vanilla" {
		t.Fatalf("restored Morrowind.esm = %q, %v", body, err)
	}
	if service.Config().Localization {
		t.Fatal("localization setting was not cleared")
	}
	if _, err := os.Stat(filepath.Join(workDir, "servers", id, "localization")); !os.IsNotExist(err) {
		t.Fatalf("localization directory survived removal: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, "servers", id, locale.AppliedName)); !os.IsNotExist(err) {
		t.Fatalf("applied manifest survived removal: %v", err)
	}
}

func TestInstallLocalizationWithoutModsOnlySavesSetting(t *testing.T) {
	service, _, id := installedService(t)
	service.installLocale = func(_ context.Context, options locale.Options,
	) (locale.Result, error) {
		return locale.Result{State: locale.State{Installed: true}}, nil
	}
	if _, err := service.InstallLocalization(context.Background(), id, false,
		nil); err != nil {
		t.Fatalf("InstallLocalization: %v", err)
	}
	if !service.Config().Localization {
		t.Fatal("localization setting was not saved")
	}
	if _, err := service.InstallLocalization(context.Background(), "missing",
		false, nil); err == nil {
		t.Fatal("missing profile accepted")
	}
}
