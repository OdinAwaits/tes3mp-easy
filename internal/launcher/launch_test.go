package launcher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/morrowind"
)

type fakeProcess struct{}

func (fakeProcess) Wait() error { return nil }

func fakeInstallation(t *testing.T) (workDir, morrowindRoot, profile string) {
	t.Helper()
	morrowindRoot = t.TempDir()
	dataFiles := filepath.Join(morrowindRoot, "Data Files")
	if err := os.MkdirAll(dataFiles, 0o700); err != nil {
		t.Fatalf("create Morrowind: %v", err)
	}
	for _, name := range morrowind.RequiredFiles {
		if err := os.WriteFile(filepath.Join(dataFiles, name), []byte(name), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	workDir = t.TempDir()
	tes3mpDir := filepath.Join(workDir, "tes3mp")
	if err := os.MkdirAll(tes3mpDir, 0o700); err != nil {
		t.Fatalf("create TES3MP: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tes3mpDir, "tes3mp.exe"),
		[]byte("exe"), 0o600); err != nil {
		t.Fatalf("write tes3mp.exe: %v", err)
	}
	profile = filepath.Join(t.TempDir(), "openmw.cfg")
	if err := os.WriteFile(profile, []byte(testProfile), 0o600); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	return workDir, morrowindRoot, profile
}

func TestLaunchWindowsStartsTES3MPDirectly(t *testing.T) {
	workDir, morrowindRoot, profile := fakeInstallation(t)
	configDir := t.TempDir()
	var captured Spec
	calls := 0
	result, err := Launch(context.Background(), LaunchOptions{
		Options: Options{
			GOOS:          "windows",
			WorkDir:       workDir,
			MorrowindRoot: morrowindRoot,
			ConfigDir:     configDir,
			HomeDir:       t.TempDir(),
		},
		ProfilePath:  profile,
		Localization: true,
		Fonts:        true,
		Executor: func(_ context.Context, spec Spec) (Process, error) {
			calls++
			captured = spec
			return fakeProcess{}, nil
		},
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if calls != 1 || result.Process == nil {
		t.Fatalf("executor calls = %d, process = %v", calls, result.Process)
	}
	wantExecutable := filepath.Join(workDir, "tes3mp", "tes3mp.exe")
	if captured.Name != wantExecutable ||
		captured.Dir != filepath.Dir(wantExecutable) ||
		len(captured.Args) != 0 || len(captured.Env) != 0 {
		t.Fatalf("spec = %+v", captured)
	}
	data, err := os.ReadFile(filepath.Join(configDir, "openmw.cfg"))
	if err != nil {
		t.Fatalf("active config: %v", err)
	}
	if !strings.Contains(string(data), "encoding=win1251") ||
		!strings.Contains(string(data), "fallback=Fonts_Font_0,") {
		t.Fatalf("active config:\n%s", data)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("warnings = %v", result.Warnings)
	}
}

func TestLaunchUsesEncodingFromMorrowindData(t *testing.T) {
	workDir, morrowindRoot, profile := fakeInstallation(t)
	marker := []byte{0xC7, 0xE0, 0xE3, 0xF0, 0xF3, 0xE7, 0xEA, 0xE0}
	esm := filepath.Join(morrowindRoot, "Data Files", "Morrowind.esm")
	body := append([]byte("TES3"), marker...)
	if err := os.WriteFile(esm, body, 0o600); err != nil {
		t.Fatalf("write Morrowind.esm: %v", err)
	}
	configDir := t.TempDir()
	_, err := Launch(context.Background(), LaunchOptions{
		Options: Options{
			GOOS:          "windows",
			WorkDir:       workDir,
			MorrowindRoot: morrowindRoot,
			ConfigDir:     configDir,
			HomeDir:       t.TempDir(),
		},
		ProfilePath: profile,
		Executor: func(_ context.Context, _ Spec) (Process, error) {
			return fakeProcess{}, nil
		},
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(configDir, "openmw.cfg"))
	if err != nil {
		t.Fatalf("active config: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "encoding="+morrowind.EncodingWin1251+"\n") ||
		strings.Contains(text, "win1252") {
		t.Fatalf("detected Morrowind encoding not applied:\n%s", text)
	}
}

func TestLaunchLinuxUsesProton(t *testing.T) {
	workDir, morrowindRoot, profile := fakeInstallation(t)
	protonDir := filepath.Join(t.TempDir(), "Proton 9.0")
	if err := os.MkdirAll(protonDir, 0o700); err != nil {
		t.Fatalf("create Proton: %v", err)
	}
	protonExe := filepath.Join(protonDir, "proton")
	if err := os.WriteFile(protonExe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write proton: %v", err)
	}
	prefixDir := filepath.Join(t.TempDir(), "prefix")
	var captured Spec
	result, err := Launch(context.Background(), LaunchOptions{
		Options: Options{
			GOOS:          "linux",
			WorkDir:       workDir,
			MorrowindRoot: morrowindRoot,
			ProtonPath:    protonDir,
			PrefixDir:     prefixDir,
			HomeDir:       t.TempDir(),
		},
		ProfilePath: profile,
		Executor: func(_ context.Context, spec Spec) (Process, error) {
			captured = spec
			return fakeProcess{}, nil
		},
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	wantExecutable := filepath.Join(workDir, "tes3mp", "tes3mp.exe")
	if captured.Name != protonExe || captured.Dir != filepath.Dir(wantExecutable) {
		t.Fatalf("spec = %+v", captured)
	}
	if len(captured.Args) != 2 || captured.Args[0] != "run" ||
		captured.Args[1] != wantExecutable {
		t.Fatalf("args = %v", captured.Args)
	}
	for _, want := range []string{
		"SteamAppId=22320",
		"SteamGameId=22320",
		"STEAM_COMPAT_DATA_PATH=" + prefixDir,
	} {
		found := false
		for _, entry := range captured.Env {
			if entry == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("environment %v is missing %s", captured.Env, want)
		}
	}
	link := filepath.Join(prefixDir, "pfx", "drive_c", "Morrowind")
	target, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("read prefix link: %v", err)
	}
	if filepath.Clean(target) != filepath.Clean(morrowindRoot) {
		t.Fatalf("prefix link = %q, want %q", target, morrowindRoot)
	}
	if result.Environment.ConfigDir == "" {
		t.Fatal("resolved config directory is empty")
	}
	if _, err := os.Stat(result.Config.Path); err != nil {
		t.Fatalf("active config was not written: %v", err)
	}
}

func TestOpenMWCSMissingExplainsHowToInstall(t *testing.T) {
	workDir, morrowindRoot, profile := fakeInstallation(t)
	_, err := OpenMWCS(context.Background(), LaunchOptions{
		Options: Options{
			GOOS:          "windows",
			WorkDir:       workDir,
			MorrowindRoot: morrowindRoot,
			ConfigDir:     t.TempDir(),
			HomeDir:       t.TempDir(),
		},
		ProfilePath: profile,
		Executor: func(_ context.Context, spec Spec) (Process, error) {
			return fakeProcess{}, nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "openmw.org") {
		t.Fatalf("error = %v, want OpenMW-CS download hint", err)
	}
}

func TestLaunchRejectsMissingMorrowind(t *testing.T) {
	workDir, _, profile := fakeInstallation(t)
	_, err := Launch(context.Background(), LaunchOptions{
		Options: Options{
			GOOS:    "windows",
			WorkDir: workDir,
			HomeDir: t.TempDir(),
		},
		ProfilePath: profile,
		Executor: func(_ context.Context, spec Spec) (Process, error) {
			return fakeProcess{}, nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "Morrowind folder") {
		t.Fatalf("error = %v, want missing Morrowind", err)
	}
}
