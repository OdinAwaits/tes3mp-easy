package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/fonts"
)

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
