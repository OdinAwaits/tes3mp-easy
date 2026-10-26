package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUninstallRemovesOnlySelectedDirectories(t *testing.T) {
	service, workDir, _ := installedService(t)
	cacheDir := service.Paths().CacheDir
	if err := os.MkdirAll(workDir, 0o700); err != nil {
		t.Fatalf("create work dir: %v", err)
	}
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		t.Fatalf("create cache: %v", err)
	}

	result, err := service.Uninstall(UninstallScope{GameFiles: true})
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if result.ConfigRemoved {
		t.Fatal("game files scope removed the configuration")
	}
	if len(result.Removed) != 1 || result.Removed[0] != workDir {
		t.Fatalf("removed = %v", result.Removed)
	}
	if _, err := os.Stat(workDir); !os.IsNotExist(err) {
		t.Fatalf("work directory survived: %v", err)
	}
	if _, err := os.Stat(cacheDir); err != nil {
		t.Fatalf("cache was removed: %v", err)
	}
	if _, err := os.Stat(service.ConfigPath()); err != nil {
		t.Fatalf("config was removed: %v", err)
	}
}

func TestUninstallEverythingReportsConfigRemoval(t *testing.T) {
	service, workDir, _ := installedService(t)
	cacheDir := service.Paths().CacheDir
	configDir := service.Paths().ConfigDir
	if err := os.MkdirAll(workDir, 0o700); err != nil {
		t.Fatalf("create work dir: %v", err)
	}
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		t.Fatalf("create cache: %v", err)
	}

	result, err := service.Uninstall(UninstallScope{
		GameFiles: true, Cache: true, Settings: true})
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if !result.ConfigRemoved {
		t.Fatal("configuration removal was not reported")
	}
	if len(result.Removed) != 3 {
		t.Fatalf("removed = %v", result.Removed)
	}
	for _, dir := range []string{workDir, cacheDir, configDir} {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("%s survived: %v", dir, err)
		}
	}
}

func TestUninstallSkipsMissingDirectories(t *testing.T) {
	service, workDir, _ := installedService(t)
	if err := os.RemoveAll(workDir); err != nil {
		t.Fatalf("remove work dir: %v", err)
	}
	result, err := service.Uninstall(UninstallScope{GameFiles: true})
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if len(result.Removed) != 0 {
		t.Fatalf("removed = %v", result.Removed)
	}
}

func TestUninstallRejectsEmptyScope(t *testing.T) {
	service, _, _ := installedService(t)
	if _, err := service.Uninstall(UninstallScope{}); !errors.Is(
		err, ErrUninstallEmpty) {
		t.Fatalf("error = %v, want ErrUninstallEmpty", err)
	}
}

func TestUninstallRefusesWorkDirAroundMorrowind(t *testing.T) {
	service, _, _ := installedService(t)
	base := t.TempDir()
	morrowind := filepath.Join(base, "Morrowind")
	if err := os.MkdirAll(morrowind, 0o700); err != nil {
		t.Fatalf("create Morrowind folder: %v", err)
	}
	if err := service.UpdateSettings(Settings{
		MorrowindPath: morrowind,
		WorkDir:       base,
	}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	if _, err := service.Uninstall(UninstallScope{GameFiles: true}); err == nil {
		t.Fatal("uninstall accepted a work directory containing Morrowind")
	}
	if _, err := os.Stat(morrowind); err != nil {
		t.Fatalf("Morrowind folder was removed: %v", err)
	}
}

func TestRemoveClientDirGuards(t *testing.T) {
	root := t.TempDir()
	protected := filepath.Join(root, "Morrowind")
	if err := os.MkdirAll(protected, 0o700); err != nil {
		t.Fatalf("create protected dir: %v", err)
	}
	if _, err := removeClientDir(root, []string{protected}); err == nil {
		t.Fatal("removed a directory that contains a guarded path")
	}
	if _, err := os.Stat(protected); err != nil {
		t.Fatalf("guarded path was removed: %v", err)
	}
	if _, err := removeClientDir(string(filepath.Separator), nil); err == nil {
		t.Fatal("removed the filesystem root")
	}

	target := filepath.Join(root, "work")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatalf("create work dir: %v", err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	if _, err := removeClientDir(link, nil); err == nil ||
		!strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("symlink error = %v", err)
	}
	if _, err := os.Lstat(link); err != nil {
		t.Fatalf("symlink was removed: %v", err)
	}

	removed, err := removeClientDir(target, nil)
	if err != nil || !removed {
		t.Fatalf("removeClientDir = %t, %v", removed, err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target survived: %v", err)
	}
}
