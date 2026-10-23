package launcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testProfile = "encoding=win1252\ndata=\"/games/Morrowind/Data Files\"\n" +
	"data=\"/work/servers/srv/data/Data Files\"\n" +
	"content=Morrowind.esm\ncontent=Tribunal.esm\ncontent=Bloodmoon.esm\n"

func writeProfile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "openmw.cfg")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	return path
}

func TestPrepareConfigCopiesAndBacksUp(t *testing.T) {
	configDir := t.TempDir()
	profile := writeProfile(t, testProfile)
	active := filepath.Join(configDir, "openmw.cfg")
	if err := os.WriteFile(active, []byte("old config\n"), 0o600); err != nil {
		t.Fatalf("write active config: %v", err)
	}
	result, err := PrepareConfig(ConfigOptions{
		ConfigDir:    configDir,
		ProfilePath:  profile,
		Localization: true,
		Fonts:        true,
		Now: func() time.Time {
			return time.Date(2026, 10, 23, 12, 34, 56, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("PrepareConfig: %v", err)
	}
	if !result.Created || result.Path != active {
		t.Fatalf("result = %+v", result)
	}
	if result.Backup != active+".bak-20261023-123456" {
		t.Fatalf("backup = %q", result.Backup)
	}
	backup, err := os.ReadFile(result.Backup)
	if err != nil || string(backup) != "old config\n" {
		t.Fatalf("backup content = %q, %v", backup, err)
	}
	data, err := os.ReadFile(active)
	if err != nil {
		t.Fatalf("read active config: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "encoding=win1251\n") ||
		strings.Contains(text, "win1252") {
		t.Fatalf("localization encoding not applied:\n%s", text)
	}
	if !strings.Contains(text, "fallback=Fonts_Font_0,") {
		t.Fatalf("font fallbacks not applied:\n%s", text)
	}
	profileData, err := os.ReadFile(profile)
	if err != nil || string(profileData) != testProfile {
		t.Fatalf("profile was modified: %q, %v", profileData, err)
	}
}

func TestPrepareConfigIsIdempotent(t *testing.T) {
	configDir := t.TempDir()
	profile := writeProfile(t, testProfile)
	options := ConfigOptions{
		ConfigDir:    configDir,
		ProfilePath:  profile,
		Localization: true,
	}
	first, err := PrepareConfig(options)
	if err != nil {
		t.Fatalf("first PrepareConfig: %v", err)
	}
	before, err := os.ReadFile(first.Path)
	if err != nil {
		t.Fatalf("read first: %v", err)
	}
	second, err := PrepareConfig(options)
	if err != nil {
		t.Fatalf("second PrepareConfig: %v", err)
	}
	after, err := os.ReadFile(second.Path)
	if err != nil {
		t.Fatalf("read second: %v", err)
	}
	if second.Created || second.Backup != "" {
		t.Fatalf("second result = %+v", second)
	}
	if string(before) != string(after) {
		t.Fatalf("config changed on second run:\nfirst:\n%s\nsecond:\n%s",
			before, after)
	}
}

func TestPrepareConfigRemovesEncodingWhenLocalizationDisabled(t *testing.T) {
	configDir := t.TempDir()
	profile := writeProfile(t, testProfile)
	result, err := PrepareConfig(ConfigOptions{
		ConfigDir:   configDir,
		ProfilePath: profile,
	})
	if err != nil {
		t.Fatalf("PrepareConfig: %v", err)
	}
	data, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatalf("read active config: %v", err)
	}
	if string(data) != testProfile {
		t.Fatalf("disabled localization changed the profile:\n%s", data)
	}
}

func TestPrepareConfigRejectsInvalidProfile(t *testing.T) {
	cases := map[string]string{
		"empty":            " \n",
		"no data":          "content=Morrowind.esm\n",
		"no content":       "data=\"/games/Morrowind/Data Files\"\n",
		"no Morrowind.esm": "data=\"/games\"\ncontent=Tribunal.esm\n",
		"NUL":              "data=\"/games\"\ncontent=Morrowind.esm\x00\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := PrepareConfig(ConfigOptions{
				ConfigDir:   t.TempDir(),
				ProfilePath: writeProfile(t, body),
			})
			if err == nil {
				t.Fatal("PrepareConfig accepted an invalid profile")
			}
		})
	}
}
