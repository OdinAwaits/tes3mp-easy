package app

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/config"
	"github.com/Saagilat/tes3mp-easy/internal/fonts"
)

func TestInstallFontsUsesSelectedSet(t *testing.T) {
	service, _, _ := installedService(t)
	if got := service.FontSet(); got != fonts.DefaultSet {
		t.Fatalf("FontSet() = %q, want %q", got, fonts.DefaultSet)
	}
	var captured fonts.Options
	service.installFonts = func(_ context.Context, options fonts.Options,
	) (fonts.Result, error) {
		captured = options
		return fonts.Result{State: fonts.State{Installed: true}}, nil
	}
	if _, err := service.InstallFonts(context.Background()); err != nil {
		t.Fatalf("InstallFonts: %v", err)
	}
	if captured.Set != fonts.DefaultSet || len(captured.Assets) != 0 {
		t.Fatalf("options = %+v", captured)
	}
	if err := service.SetFontSet(fonts.SetInter); err != nil {
		t.Fatalf("SetFontSet: %v", err)
	}
	if got := service.Config().FontSet; got != fonts.SetInter {
		t.Fatalf("config font_set = %q", got)
	}
	if got := service.FontSet(); got != fonts.SetInter {
		t.Fatalf("FontSet() = %q", got)
	}
	if _, err := service.InstallFonts(context.Background()); err != nil {
		t.Fatalf("InstallFonts after switch: %v", err)
	}
	if captured.Set != fonts.SetInter {
		t.Fatalf("options.Set = %q, want %q", captured.Set, fonts.SetInter)
	}
	resolved, ok := fonts.LookupSet(captured.Set)
	if !ok || resolved.Assets[0].Name != "Inter.ttf" {
		t.Fatalf("set %q does not resolve to Inter.ttf", captured.Set)
	}
}

func TestInstallFontsForwardsProgress(t *testing.T) {
	service, _, _ := installedService(t)
	var captured fonts.Options
	service.installFonts = func(_ context.Context, options fonts.Options,
	) (fonts.Result, error) {
		captured = options
		if options.Progress != nil {
			options.Progress(fonts.Progress{
				Stage: fonts.StageDownloading, Asset: "NotoSans-Regular.ttf",
				Downloaded: 1 << 20, Total: 2 << 20,
			})
		}
		return fonts.Result{State: fonts.State{Installed: true}}, nil
	}
	var updates []fonts.Progress
	if _, err := service.InstallFontsWithProgress(context.Background(),
		func(update fonts.Progress) {
			updates = append(updates, update)
		}); err != nil {
		t.Fatalf("InstallFontsWithProgress: %v", err)
	}
	if captured.Progress == nil {
		t.Fatal("progress callback was not passed to the font installer")
	}
	if len(updates) != 1 || updates[0].Asset != "NotoSans-Regular.ttf" ||
		updates[0].Downloaded != 1<<20 {
		t.Fatalf("updates = %+v", updates)
	}
}

func TestSetFontSetRejectsUnknownAndKeepsDefault(t *testing.T) {
	service, _, _ := installedService(t)
	if err := service.SetFontSet("bogus"); !errors.Is(err, ErrUnknownFontSet) {
		t.Fatalf("SetFontSet error = %v, want ErrUnknownFontSet", err)
	}
	if got := service.Config().FontSet; got != "" {
		t.Fatalf("config font_set = %q, want empty", got)
	}
	if got := service.FontSet(); got != fonts.DefaultSet {
		t.Fatalf("FontSet() = %q", got)
	}
}

func TestFontSetIgnoresHandEditedValue(t *testing.T) {
	service, _, _ := installedService(t)
	if err := service.store.Update(func(cfg *config.Config) error {
		cfg.FontSet = "  INTER "
		return nil
	}); err != nil {
		t.Fatalf("store.Update: %v", err)
	}
	if got := service.FontSet(); got != fonts.SetInter {
		t.Fatalf("FontSet() = %q, want the normalized value", got)
	}
	if err := service.store.Update(func(cfg *config.Config) error {
		cfg.FontSet = "removed-set"
		return nil
	}); err != nil {
		t.Fatalf("store.Update: %v", err)
	}
	if got := service.FontSet(); got != fonts.DefaultSet {
		t.Fatalf("FontSet() = %q, want the default for an unknown set", got)
	}
}

func TestInstallFontsUsesOpenMWConfigDir(t *testing.T) {
	service, workDir, _ := installedService(t)
	var captured fonts.Options
	service.installFonts = func(_ context.Context, options fonts.Options,
	) (fonts.Result, error) {
		captured = options
		return fonts.Result{State: fonts.State{Installed: true}}, nil
	}
	result, err := service.InstallFonts(context.Background())
	if err != nil {
		t.Fatalf("InstallFonts: %v", err)
	}
	if !result.State.Installed {
		t.Fatalf("result = %+v", result)
	}
	if captured.CacheDir != service.Paths().CacheDir {
		t.Fatalf("cache dir = %q", captured.CacheDir)
	}
	if service.goos == "linux" &&
		!strings.HasPrefix(captured.ConfigDir, workDir) {
		t.Fatalf("config dir = %q, want below %q", captured.ConfigDir, workDir)
	}
	if service.goos == "windows" &&
		!strings.HasSuffix(filepath.ToSlash(captured.ConfigDir),
			"Documents/My Games/OpenMW") {
		t.Fatalf("config dir = %q", captured.ConfigDir)
	}
}
