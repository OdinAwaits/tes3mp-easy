package i18n

import "testing"

func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestParse(t *testing.T) {
	tests := []struct {
		value string
		want  Language
		ok    bool
	}{
		{value: "en", want: English, ok: true},
		{value: "ru", want: Russian, ok: true},
		{value: " RU ", want: Russian, ok: true},
		{value: "ru_RU.UTF-8", want: Russian, ok: true},
		{value: "en-US", want: English, ok: true},
		{value: "de_DE.UTF-8", ok: false},
		{value: "C", ok: false},
		{value: "", ok: false},
	}
	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			got, ok := Parse(test.value)
			if ok != test.ok || got != test.want {
				t.Fatalf("Parse(%q) = %q, %v; want %q, %v",
					test.value, got, ok, test.want, test.ok)
			}
		})
	}
}

func TestDetectUsesLocalePriority(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want Language
	}{
		{
			name: "LC_ALL wins",
			env:  map[string]string{"LC_ALL": "ru_RU.UTF-8", "LANG": "en_US.UTF-8"},
			want: Russian,
		},
		{
			name: "LC_MESSAGES wins over LANG",
			env:  map[string]string{"LC_MESSAGES": "ru_RU.UTF-8", "LANG": "en_US.UTF-8"},
			want: Russian,
		},
		{
			name: "LANG is used",
			env:  map[string]string{"LANG": "ru_RU.UTF-8"},
			want: Russian,
		},
		{
			name: "LANGUAGE fallback",
			env:  map[string]string{"LANGUAGE": "ru:en"},
			want: Russian,
		},
		{
			name: "unknown locale becomes English",
			env:  map[string]string{"LC_ALL": "C", "LANG": "de_DE.UTF-8"},
			want: English,
		},
		{
			name: "empty environment becomes English",
			env:  map[string]string{},
			want: English,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Detect(env(test.env)); got != test.want {
				t.Fatalf("Detect() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestResolvePrefersExplicitSetting(t *testing.T) {
	locale := env(map[string]string{"LANG": "ru_RU.UTF-8"})
	if got := Resolve("en", locale); got != English {
		t.Fatalf("Resolve(en) = %q", got)
	}
	if got := Resolve("", locale); got != Russian {
		t.Fatalf("Resolve(auto) = %q", got)
	}
	if got := Resolve("de", locale); got != Russian {
		t.Fatalf("Resolve(unsupported) = %q", got)
	}
}

func TestCatalogsHaveEveryKey(t *testing.T) {
	for _, key := range Keys() {
		for _, language := range Supported() {
			if value := catalogs[language][key]; value == "" {
				t.Errorf("%s is missing %q", language, key)
			}
		}
	}
	for language, catalog := range catalogs {
		for key := range catalog {
			if _, ok := catalogs[English][key]; !ok && language != English {
				t.Errorf("%s has key %q missing from English", language, key)
			}
		}
	}
}

func TestTranslatorFallback(t *testing.T) {
	translator := New(Russian)
	if got := translator.T(KeyCommonPlay); got != "Играть" {
		t.Fatalf("T(KeyCommonPlay) = %q", got)
	}
	if got := translator.T("missing.key"); got != "missing.key" {
		t.Fatalf("T(missing.key) = %q", got)
	}
	if got := translator.Tf(KeyInstallFontsReady, "Noto"); got != "Набор шрифтов Noto установлен." {
		t.Fatalf("Tf(KeyInstallFontsReady) = %q", got)
	}
	if got := New("de").Language(); got != English {
		t.Fatalf("New(de).Language() = %q", got)
	}
	if got := ForSetting("ru", env(map[string]string{"LANG": "en_US.UTF-8"})).Language(); got != Russian {
		t.Fatalf("ForSetting(ru) = %q", got)
	}
	if got := DisplayName(Russian); got != "Русский" {
		t.Fatalf("DisplayName(Russian) = %q", got)
	}
}

func TestPluralf(t *testing.T) {
	russian := New(Russian)
	english := New(English)
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "ru one", got: russian.Pluralf(KeyPanelTimeMinuteAgo, 1), want: "1 минуту назад"},
		{name: "ru few", got: russian.Pluralf(KeyPanelTimeMinuteAgo, 3), want: "3 минуты назад"},
		{name: "ru many", got: russian.Pluralf(KeyPanelTimeMinuteAgo, 11), want: "11 минут назад"},
		{name: "ru 21", got: russian.Pluralf(KeyPanelTimeMinuteAgo, 21), want: "21 минуту назад"},
		{name: "ru years", got: russian.Pluralf(KeyPanelTimeYearAgo, 5), want: "5 лет назад"},
		{name: "en one", got: english.Pluralf(KeyPanelTimeMinuteAgo, 1), want: "1 minute ago"},
		{name: "en other", got: english.Pluralf(KeyPanelTimeMinuteAgo, 5), want: "5 minutes ago"},
		{name: "missing key", got: english.Pluralf("panel.time.unknown_ago", 2), want: "panel.time.unknown_ago"},
	}
	for _, test := range tests {
		if test.got != test.want {
			t.Errorf("%s = %q, want %q", test.name, test.got, test.want)
		}
	}
}

func TestEnglishCatalogHasNoCyrillic(t *testing.T) {
	for key, value := range catalogs[English] {
		for _, r := range value {
			if r >= '\u0400' && r <= '\u04FF' {
				t.Errorf("English catalog %q contains Cyrillic: %q", key, value)
				break
			}
		}
	}
}

func TestPanelKeysAreUsable(t *testing.T) {
	for _, key := range []string{
		KeyPanelNavDashboard, KeyPanelLoginHeading, KeyPanelDashboardStart,
		KeyPanelStateRestoreButton, KeyPanelModsDeploy, KeyPanelConfigSave,
		KeyPanelStaffPlayers, KeyPanelLogsShow,
	} {
		if got := New(English).T(key); got == key {
			t.Errorf("English catalog is missing %q", key)
		}
		if got := New(Russian).T(key); got == key {
			t.Errorf("Russian catalog is missing %q", key)
		}
	}
}
