package openmwcfg

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readFileText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func backups(t *testing.T, path string) []string {
	t.Helper()
	found, err := filepath.Glob(path + ".bak-*")
	if err != nil {
		t.Fatalf("glob backups: %v", err)
	}
	return found
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	state, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if state.Exists || state.Configured {
		t.Fatalf("state = %+v, want missing", state)
	}
	if state.Path != path {
		t.Fatalf("path = %q, want %q", state.Path, path)
	}
	if state.Values != Defaults() {
		t.Fatalf("values = %+v, want defaults", state.Values)
	}
}

func TestLoadRejectsUnparsableValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	writeFile(t, path, "[GUI]\nfont size = big\n")
	_, err := Load(path)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("error = %v, want ErrInvalid", err)
	}
	if !strings.Contains(err.Error(), "font size") {
		t.Fatalf("error = %v, want the key in the message", err)
	}
}

func TestUpdateCreatesConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	state, err := Update(path, Settings{
		TTFResolution:  20,
		FontSize:       18,
		ScalingFactor:  1.25,
		FramerateLimit: 120,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !state.Exists || !state.Configured {
		t.Fatalf("state = %+v, want configured", state)
	}
	want := "[GUI]\nttf resolution = 20\nfont size = 18\nscaling factor = 1.25\n" +
		"\n[Video]\nframerate limit = 120\n"
	if got := readFileText(t, path); got != want {
		t.Fatalf("file =\n%q\nwant\n%q", got, want)
	}
	if len(backups(t, path)) != 0 {
		t.Fatalf("a new file must not create a backup")
	}
}

func TestUpdatePreservesCommentsOrderAndCRLF(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	original := strings.Join([]string{
		"; written by OpenMW",
		"[GUI]",
		"# interface",
		"font size = 14",
		"ttf resolution = 20",
		"",
		"[Video]",
		"framerate limit = 0",
		"vsync = true",
		"",
		"[Saves]",
		"character = Saagilat",
		"",
	}, "\r\n")
	writeFile(t, path, original)

	if _, err := Update(path, Settings{
		TTFResolution:  22,
		FontSize:       16,
		ScalingFactor:  1,
		FramerateLimit: 60,
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got := readFileText(t, path)
	want := strings.Replace(original,
		"ttf resolution = 20\r\n", "ttf resolution = 22\r\nscaling factor = 1\r\n", 1)
	want = strings.Replace(want, "font size = 14", "font size = 16", 1)
	want = strings.Replace(want, "framerate limit = 0", "framerate limit = 60", 1)
	if got != want {
		t.Fatalf("file =\n%q\nwant\n%q", got, want)
	}
	if stripped := strings.ReplaceAll(got, "\r\n", ""); strings.Contains(stripped, "\n") {
		t.Fatalf("line endings are not preserved: %q", got)
	}
	if len(backups(t, path)) != 1 {
		t.Fatalf("backups = %v, want one", backups(t, path))
	}
	if backup := readFileText(t, backups(t, path)[0]); backup != original {
		t.Fatalf("backup = %q, want the original", backup)
	}
}

func TestUpdateAppendsMissingSectionAndKeepsBOM(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	writeFile(t, path, "\ufeff[GUI]\nttf resolution = 30\n")
	state, err := Update(path, Settings{
		TTFResolution:  24,
		FontSize:       16,
		ScalingFactor:  1.5,
		FramerateLimit: 0,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !state.Configured {
		t.Fatalf("state = %+v, want configured", state)
	}
	got := readFileText(t, path)
	if !strings.HasPrefix(got, "\ufeff[GUI]\n") {
		t.Fatalf("file = %q, want the BOM and GUI first", got)
	}
	if !strings.Contains(got, "\n[Video]\nframerate limit = 0\n") {
		t.Fatalf("file = %q, want an appended Video section", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("file = %q, want a trailing newline", got)
	}
}

func TestUpdateCollapsesDuplicateKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	writeFile(t, path, "[GUI]\nfont size = 10\nfont size = 12\n")
	if _, err := Update(path, Defaults()); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got := readFileText(t, path)
	if strings.Count(got, "font size") != 1 {
		t.Fatalf("file = %q, want one font size line", got)
	}
	if !strings.Contains(got, "font size = 16") {
		t.Fatalf("file = %q, want the new value", got)
	}
}

func TestUpdateRejectsOutOfRangeValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	original := "[GUI]\nfont size = 16\n"
	writeFile(t, path, original)
	_, err := Update(path, Settings{
		TTFResolution:  18,
		FontSize:       160,
		ScalingFactor:  1,
		FramerateLimit: 60,
	})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("error = %v, want ErrInvalid", err)
	}
	if got := readFileText(t, path); got != original {
		t.Fatalf("file = %q, want it untouched", got)
	}
}

func TestUpdateWithoutChangesWritesNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	values := Defaults()
	if _, err := Update(path, values); err != nil {
		t.Fatalf("Update: %v", err)
	}
	before := readFileText(t, path)
	if _, err := Update(path, values); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got := readFileText(t, path); got != before {
		t.Fatalf("file = %q, want an unchanged file", got)
	}
	if len(backups(t, path)) != 0 {
		t.Fatalf("backups = %v, want none", backups(t, path))
	}
}

func TestLoadRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real.cfg")
	writeFile(t, target, "[GUI]\nfont size = 16\n")
	link := filepath.Join(dir, FileName)
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink is not supported: %v", err)
	}
	if _, err := Load(link); !errors.Is(err, ErrInvalid) {
		t.Fatalf("error = %v, want ErrInvalid", err)
	}
}

func TestLoadRejectsRelativePath(t *testing.T) {
	if _, err := Load(FileName); err == nil {
		t.Fatal("Load accepted a relative path")
	}
}
