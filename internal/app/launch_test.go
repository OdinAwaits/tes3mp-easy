package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/fonts"
	"github.com/Saagilat/tes3mp-easy/internal/launcher"
	"github.com/Saagilat/tes3mp-easy/internal/locale"
)

func installedService(t *testing.T) (*Service, string, string) {
	t.Helper()
	paths := testPaths(t)
	service, err := OpenWithEnv(paths, nil)
	if err != nil {
		t.Fatalf("OpenWithEnv: %v", err)
	}
	workDir := filepath.Join(t.TempDir(), "work")
	if err := service.UpdateSettings(Settings{
		MorrowindPath: t.TempDir(),
		WorkDir:       workDir,
	}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	id, err := service.SaveServer("", testServer())
	if err != nil {
		t.Fatalf("SaveServer: %v", err)
	}
	return service, workDir, id
}

func installFakeMods(t *testing.T, workDir, serverID string) string {
	t.Helper()
	serverDir := filepath.Join(workDir, "servers", serverID)
	dataFiles := filepath.Join(serverDir, "data", "Data Files")
	if err := os.MkdirAll(dataFiles, 0o700); err != nil {
		t.Fatalf("create mods Data Files: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataFiles, "Morrowind.esm"),
		[]byte("vanilla"), 0o600); err != nil {
		t.Fatalf("write Morrowind.esm: %v", err)
	}
	profile := filepath.Join(serverDir, "openmw.cfg")
	if err := os.WriteFile(profile, []byte("content=Morrowind.esm\n"), 0o600); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	digest := strings.Repeat("a", 64)
	if err := os.WriteFile(filepath.Join(serverDir, ".mods-sha256"),
		[]byte(digest+"\n"), 0o600); err != nil {
		t.Fatalf("write digest: %v", err)
	}
	return profile
}

func writeLocaleLayer(t *testing.T, workDir, serverID string) {
	t.Helper()
	layer := filepath.Join(workDir, "servers", serverID, "localization", "main",
		"Data Files")
	if err := os.MkdirAll(layer, 0o700); err != nil {
		t.Fatalf("create locale layer: %v", err)
	}
	if err := os.WriteFile(filepath.Join(layer, "Morrowind.esm"),
		[]byte("localized"), 0o600); err != nil {
		t.Fatalf("write localized Morrowind.esm: %v", err)
	}
	marker := filepath.Join(workDir, "servers", serverID, "localization",
		".locale.json")
	body := `{"language":"russian","main_sha256":"` + strings.Repeat("b", 64) +
		`","voiceover":false}`
	if err := os.WriteFile(marker, []byte(body), 0o600); err != nil {
		t.Fatalf("write locale marker: %v", err)
	}
}

func TestLaunchWiring(t *testing.T) {
	service, workDir, id := installedService(t)
	profile := installFakeMods(t, workDir, id)
	fontsInstalled := false
	service.installFonts = func(_ context.Context, options fonts.Options,
	) (fonts.Result, error) {
		fontsInstalled = true
		return fonts.Result{
			State:    fonts.State{Installed: true},
			Warnings: []string{"font warning"},
		}, nil
	}
	var captured launcher.LaunchOptions
	service.launchClient = func(_ context.Context, options launcher.LaunchOptions,
	) (launcher.Result, error) {
		captured = options
		return launcher.Result{Warnings: []string{"launch warning"}}, nil
	}
	result, err := service.Launch(context.Background(), id)
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if !fontsInstalled {
		t.Fatal("InstallFonts was not called")
	}
	if captured.ProfilePath != profile || captured.Localization ||
		!captured.Fonts || captured.WorkDir != workDir {
		t.Fatalf("launch options = %+v", captured)
	}
	if captured.ServerAddress != testServer().Address ||
		captured.ServerPort != testServer().Port ||
		captured.ServerPassword != testServer().Password {
		t.Fatalf("launch endpoint = %+v", captured)
	}
	if captured.MorrowindRoot == "" || captured.GOOS == "" {
		t.Fatalf("launch options = %+v", captured)
	}
	if len(result.Warnings) != 2 || result.Warnings[0] != "font warning" ||
		result.Warnings[1] != "launch warning" {
		t.Fatalf("warnings = %v", result.Warnings)
	}
}

func TestLaunchReportsFontFailureAsWarning(t *testing.T) {
	service, workDir, id := installedService(t)
	installFakeMods(t, workDir, id)
	service.installFonts = func(_ context.Context, options fonts.Options,
	) (fonts.Result, error) {
		return fonts.Result{}, os.ErrDeadlineExceeded
	}
	launched := false
	service.launchClient = func(_ context.Context, options launcher.LaunchOptions,
	) (launcher.Result, error) {
		launched = true
		if options.Fonts {
			t.Fatal("launch got Fonts=true after a font failure")
		}
		return launcher.Result{}, nil
	}
	result, err := service.Launch(context.Background(), id)
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if !launched {
		t.Fatal("launch was blocked by a font failure")
	}
	if len(result.Warnings) != 1 ||
		!strings.Contains(result.Warnings[0], "fonts were not installed") {
		t.Fatalf("warnings = %v", result.Warnings)
	}
}

func TestLaunchSkipsFontsWhenDisabled(t *testing.T) {
	service, workDir, id := installedService(t)
	installFakeMods(t, workDir, id)
	if err := service.UpdateSettings(Settings{
		MorrowindPath: service.Config().MorrowindPath,
		WorkDir:       workDir,
		SkipFonts:     true,
	}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	fontsCalled := false
	service.installFonts = func(_ context.Context, _ fonts.Options) (fonts.Result, error) {
		fontsCalled = true
		return fonts.Result{State: fonts.State{Installed: true}}, nil
	}
	captured := launcher.LaunchOptions{}
	service.launchClient = func(_ context.Context, options launcher.LaunchOptions,
	) (launcher.Result, error) {
		captured = options
		return launcher.Result{}, nil
	}
	if _, err := service.Launch(context.Background(), id); err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if fontsCalled {
		t.Fatal("InstallFonts was called while fonts are disabled")
	}
	if captured.Fonts {
		t.Fatalf("launch options = %+v", captured)
	}
}

func TestLaunchLocalizationForcesFonts(t *testing.T) {
	service, workDir, id := installedService(t)
	installFakeMods(t, workDir, id)
	writeLocaleLayer(t, workDir, id)
	if _, err := locale.Apply(workDir, id); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if err := service.UpdateSettings(Settings{
		MorrowindPath: service.Config().MorrowindPath,
		WorkDir:       workDir,
		SkipFonts:     true,
		Localization:  true,
	}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	fontsCalled := false
	service.installFonts = func(_ context.Context, _ fonts.Options) (fonts.Result, error) {
		fontsCalled = true
		return fonts.Result{State: fonts.State{Installed: true}}, nil
	}
	captured := launcher.LaunchOptions{}
	service.launchClient = func(_ context.Context, options launcher.LaunchOptions,
	) (launcher.Result, error) {
		captured = options
		return launcher.Result{}, nil
	}
	if _, err := service.Launch(context.Background(), id); err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if !fontsCalled || !captured.Fonts {
		t.Fatalf("fonts were not forced for localization: %+v", captured)
	}
}

func TestLaunchRequiresInstalledMods(t *testing.T) {
	service, _, id := installedService(t)
	if _, err := service.Launch(context.Background(), id); err == nil ||
		!strings.Contains(err.Error(), "mods are not installed") {
		t.Fatalf("error = %v, want mods prerequisite", err)
	}
}

func TestLaunchRejectsMissingLocalization(t *testing.T) {
	service, workDir, id := installedService(t)
	installFakeMods(t, workDir, id)
	if err := service.UpdateSettings(Settings{
		MorrowindPath: service.Config().MorrowindPath,
		WorkDir:       workDir,
		Localization:  true,
	}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	_, err := service.Launch(context.Background(), id)
	if err == nil || !strings.Contains(err.Error(), "localization is enabled") {
		t.Fatalf("error = %v, want missing localization", err)
	}
}

func TestLaunchAllowsInstalledLocalization(t *testing.T) {
	service, workDir, id := installedService(t)
	installFakeMods(t, workDir, id)
	writeLocaleLayer(t, workDir, id)
	if _, err := locale.Apply(workDir, id); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if err := service.UpdateSettings(Settings{
		MorrowindPath: service.Config().MorrowindPath,
		WorkDir:       workDir,
		Localization:  true,
	}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	service.installFonts = func(_ context.Context, options fonts.Options,
	) (fonts.Result, error) {
		return fonts.Result{State: fonts.State{Installed: true}}, nil
	}
	captured := launcher.LaunchOptions{}
	service.launchClient = func(_ context.Context, options launcher.LaunchOptions,
	) (launcher.Result, error) {
		captured = options
		return launcher.Result{}, nil
	}
	if _, err := service.Launch(context.Background(), id); err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if !captured.Localization {
		t.Fatalf("launch options = %+v", captured)
	}
}
