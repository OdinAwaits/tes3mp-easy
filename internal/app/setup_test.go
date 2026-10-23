package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/config"
	"github.com/Saagilat/tes3mp-easy/internal/morrowind"
	"github.com/Saagilat/tes3mp-easy/internal/tes3mp"
)

func makeMorrowind(t *testing.T, files ...string) string {
	t.Helper()
	root := t.TempDir()
	dataFiles := filepath.Join(root, "Data Files")
	if err := os.MkdirAll(dataFiles, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(dataFiles, name),
			[]byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestDetectMorrowindPersistsPath(t *testing.T) {
	paths := testPaths(t)
	service, err := OpenWithEnv(paths, nil)
	if err != nil {
		t.Fatal(err)
	}
	root := makeMorrowind(t, morrowind.RequiredFiles...)
	if err := service.UpdateSettings(Settings{MorrowindPath: root}); err != nil {
		t.Fatal(err)
	}
	installation, err := service.DetectMorrowind()
	if err != nil {
		t.Fatalf("DetectMorrowind: %v", err)
	}
	if installation.Root != root {
		t.Fatalf("root = %q, want %q", installation.Root, root)
	}
	reloaded, err := config.Load(paths.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.MorrowindPath != root {
		t.Fatalf("saved path = %q, want %q", reloaded.MorrowindPath, root)
	}
}

func TestCheckMorrowind(t *testing.T) {
	service, err := OpenWithEnv(testPaths(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	root := makeMorrowind(t, "Morrowind.esm")
	if _, err := service.CheckMorrowind(root); !errors.Is(err, morrowind.ErrIncomplete) {
		t.Fatalf("CheckMorrowind = %v, want %v", err, morrowind.ErrIncomplete)
	}
	if _, err := service.CheckMorrowind(""); err == nil {
		t.Fatal("empty path accepted")
	}
	root = makeMorrowind(t, morrowind.RequiredFiles...)
	installation, err := service.CheckMorrowind(root)
	if err != nil || installation.Root != root {
		t.Fatalf("CheckMorrowind = %+v, %v", installation, err)
	}
}

func TestTES3MPStateUsesEffectiveWorkDir(t *testing.T) {
	paths := testPaths(t)
	service, err := OpenWithEnv(paths, nil)
	if err != nil {
		t.Fatal(err)
	}
	dir := tes3mp.Dir(paths.DataDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, tes3mp.Executable),
		[]byte("exe"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, tes3mp.VersionFile),
		[]byte(tes3mp.Version+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	state, err := service.TES3MPState()
	if err != nil {
		t.Fatalf("TES3MPState: %v", err)
	}
	if !state.Installed || !state.Current {
		t.Fatalf("state = %+v", state)
	}
}

func TestInstallTES3MPWiring(t *testing.T) {
	paths := testPaths(t)
	service, err := OpenWithEnv(paths, nil)
	if err != nil {
		t.Fatal(err)
	}
	workDir := filepath.Join(t.TempDir(), "work")
	if err := service.UpdateSettings(Settings{WorkDir: workDir}); err != nil {
		t.Fatal(err)
	}
	var captured tes3mp.Options
	service.installTES3MP = func(_ context.Context,
		options tes3mp.Options,
	) (tes3mp.Result, error) {
		captured = options
		return tes3mp.Result{Downloaded: true}, nil
	}
	called := false
	result, err := service.InstallTES3MP(t.Context(), func(tes3mp.Progress) {
		called = true
	})
	if err != nil || !result.Downloaded {
		t.Fatalf("InstallTES3MP = %+v, %v", result, err)
	}
	if captured.Dir != filepath.Join(workDir, "tes3mp") {
		t.Fatalf("dir = %q", captured.Dir)
	}
	if captured.CacheDir != paths.CacheDir {
		t.Fatalf("cache = %q, want %q", captured.CacheDir, paths.CacheDir)
	}
	if captured.Progress == nil || called {
		t.Fatalf("progress callback is nil or called early")
	}
	captured.Progress(tes3mp.Progress{Stage: tes3mp.StageVerifying})
	if !called {
		t.Fatal("progress callback was not forwarded")
	}

	if err := service.UpdateSettings(Settings{WorkDir: "relative"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.InstallTES3MP(t.Context(), nil); err == nil {
		t.Fatal("relative work directory accepted")
	}
}
