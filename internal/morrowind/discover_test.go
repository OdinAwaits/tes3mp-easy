package morrowind

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeRegistry map[string]string

func (r fakeRegistry) ReadString(key, value string) (string, error) {
	result, ok := r[key+"\x00"+value]
	if !ok {
		return "", os.ErrNotExist
	}
	return result, nil
}

func fakeInspector(valid map[string]bool) func(string) (Installation, error) {
	return func(path string) (Installation, error) {
		if valid[path] {
			return Installation{Root: path, DataFiles: path + "/Data Files"}, nil
		}
		return Installation{}, fmt.Errorf("%s: %w", path, os.ErrNotExist)
	}
}

func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestDiscoverLinuxSteamLibraryFromVDF(t *testing.T) {
	home := t.TempDir()
	vdfPath := filepath.Join(home, ".steam", "steam", "steamapps",
		"libraryfolders.vdf")
	library := filepath.Join(home, "games", "SteamLibrary")
	want := filepath.Join(library, "steamapps", "common", "Morrowind")
	installation, err := Discover(Options{
		GOOS:   "linux",
		Getenv: env(map[string]string{"HOME": home}),
		ReadFile: func(path string) ([]byte, error) {
			if path != vdfPath {
				return nil, os.ErrNotExist
			}
			return []byte(fmt.Sprintf(`"libraryfolders"
{
  "0" { "path" "%s" }
}`, strings.ReplaceAll(library, `\`, `\\`))), nil
		},
		Inspect: fakeInspector(map[string]bool{want: true}),
	})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if installation.Root != want {
		t.Fatalf("root = %q, want %q", installation.Root, want)
	}
}

func TestDiscoverWindowsRegistryWins(t *testing.T) {
	registryPath := `C:\Games\Morrowind`
	want := `C:\Games\Morrowind`
	installation, err := Discover(Options{
		GOOS: "windows",
		Getenv: env(map[string]string{
			"ProgramFiles(x86)": `C:\Program Files (x86)`,
		}),
		Registry: fakeRegistry{
			`SOFTWARE\WOW6432Node\Bethesda Softworks\Morrowind` + "\x00Install Path": registryPath,
		},
		ReadDir: func(string) ([]os.DirEntry, error) { return nil, os.ErrNotExist },
		Inspect: fakeInspector(map[string]bool{want: true}),
	})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if installation.Root != want {
		t.Fatalf("root = %q, want %q", installation.Root, want)
	}
}

func TestDiscoverWindowsSteamVDF(t *testing.T) {
	want := `D:\SteamLibrary\steamapps\common\Morrowind`
	vdfPath := `C:\Program Files (x86)\Steam\steamapps\libraryfolders.vdf`
	var readPath string
	installation, err := Discover(Options{
		GOOS: "windows",
		Getenv: env(map[string]string{
			"ProgramFiles(x86)": `C:\Program Files (x86)`,
		}),
		Registry: fakeRegistry{},
		ReadFile: func(path string) ([]byte, error) {
			readPath = path
			if path != vdfPath {
				return nil, os.ErrNotExist
			}
			return []byte(`"libraryfolders"
{
  "0" { "path" "D:\\SteamLibrary" }
  "apps" { "220" "123456789" }
}`), nil
		},
		ReadDir: func(string) ([]os.DirEntry, error) { return nil, os.ErrNotExist },
		Inspect: fakeInspector(map[string]bool{want: true}),
	})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if readPath != vdfPath {
		t.Fatalf("read %q, want %q", readPath, vdfPath)
	}
	if installation.Root != want {
		t.Fatalf("root = %q, want %q", installation.Root, want)
	}
}

func TestDiscoverConfiguredRootIsFirst(t *testing.T) {
	root := t.TempDir()
	installation, err := Discover(Options{Root: root, GOOS: "linux",
		Getenv:  env(map[string]string{"HOME": t.TempDir()}),
		Inspect: fakeInspector(map[string]bool{root: true}),
	})
	if err != nil || installation.Root != root {
		t.Fatalf("Discover = %+v, %v", installation, err)
	}
}

func TestDiscoverReturnsConfiguredRootError(t *testing.T) {
	root := t.TempDir()
	permission := errors.New("permission denied")
	_, err := Discover(Options{
		Root:   root,
		GOOS:   "linux",
		Getenv: env(map[string]string{"HOME": t.TempDir()}),
		Inspect: func(string) (Installation, error) {
			return Installation{}, permission
		},
	})
	if !errors.Is(err, permission) {
		t.Fatalf("Discover = %v, want %v", err, permission)
	}
}

func TestDiscoverReportsNotFound(t *testing.T) {
	_, err := Discover(Options{
		GOOS:    "linux",
		Getenv:  env(map[string]string{"HOME": t.TempDir()}),
		Inspect: fakeInspector(nil),
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Discover = %v, want %v", err, ErrNotFound)
	}
	if !strings.Contains(err.Error(), "checked") {
		t.Fatalf("error %q does not list candidates", err)
	}
}

func TestParseSteamLibraryPaths(t *testing.T) {
	data := []byte(`"libraryfolders"
{
  "0"
  {
    "path" "C:\\Program Files (x86)\\Steam"
    "apps" { "220" "1234" }
  }
  "1" { "path" "/games/SteamLibrary" }
  "2" "D:\\OldLibrary"
}
`)
	got := parseSteamLibraryPaths(data, "windows")
	want := []string{
		`C:\Program Files (x86)\Steam`,
		`D:\OldLibrary`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("paths = %v, want %v", got, want)
	}
}
