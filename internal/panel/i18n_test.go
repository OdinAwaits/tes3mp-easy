package panel

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/i18n"
)

func TestLanguageSettingDefaultsAndPersistence(t *testing.T) {
	dir := t.TempDir()
	setting, err := openLanguageSetting(dir, i18n.English, nil)
	if err != nil {
		t.Fatalf("openLanguageSetting: %v", err)
	}
	if got := setting.Language(); got != i18n.English {
		t.Fatalf("default language = %q, want %q", got, i18n.English)
	}
	if _, err := os.Stat(filepath.Join(dir, SettingsFileName)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("default language created settings.json: %v", err)
	}

	if err := setting.Set(i18n.Russian); err != nil {
		t.Fatalf("Set(ru): %v", err)
	}
	if got := setting.Translator().T(i18n.KeyPanelNavDashboard); got != "Обзор" {
		t.Fatalf("translator after Set(ru) = %q", got)
	}
	path := filepath.Join(dir, SettingsFileName)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat settings.json: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("settings.json mode = %v, want 0600", perm)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read settings.json: %v", err)
	}
	var record settingsRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		t.Fatalf("decode settings.json: %v", err)
	}
	if record.Version != settingsFileVersion || record.Language != "ru" {
		t.Fatalf("settings.json = %+v", record)
	}

	// A restart reads the stored choice instead of the environment default.
	reopened, err := openLanguageSetting(dir, i18n.English, nil)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got := reopened.Language(); got != i18n.Russian {
		t.Fatalf("reopened language = %q, want ru", got)
	}

	if err := setting.Set(i18n.Language("de")); err == nil {
		t.Fatal("Set(de) succeeded, want an error")
	}
	if got := setting.Language(); got != i18n.Russian {
		t.Fatalf("failed Set changed the language to %q", got)
	}
}

func TestLanguageSettingRejectsBrokenFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, SettingsFileName)
	if err := os.WriteFile(path, []byte(`{"version":1,"language":"de"}`), 0o600); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}
	if _, err := openLanguageSetting(dir, i18n.English, nil); err == nil {
		t.Fatal("unsupported language in settings.json was accepted")
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}
	if _, err := openLanguageSetting(dir, i18n.English, nil); err == nil {
		t.Fatal("broken settings.json was accepted")
	}
}

func TestAdminSwitchesLanguage(t *testing.T) {
	env := newAdminTestEnv(t, nil)

	rec := env.request(t, http.MethodGet, "/admin", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `<html lang="en">`) {
		t.Fatal("dashboard is not rendered in English by default")
	}
	if !strings.Contains(rec.Body.String(), `>Overview</a>`) {
		t.Fatal("dashboard has no English navigation")
	}

	rec = env.request(t, http.MethodPost, "/admin/language", url.Values{
		"language": {"ru"},
		"next":     {"/admin/staff"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST /admin/language = %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "/admin/staff" {
		t.Fatalf("Location = %q, want /admin/staff", got)
	}
	if _, err := os.Stat(filepath.Join(env.cfg.DataDir, SettingsFileName)); err != nil {
		t.Fatalf("settings.json was not written: %v", err)
	}

	rec = env.request(t, http.MethodGet, "/admin", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin after switch = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `<html lang="ru">`) {
		t.Fatal("dashboard is not rendered in Russian after the switch")
	}
	if !strings.Contains(body, `>Обзор</a>`) {
		t.Fatal("dashboard has no Russian navigation after the switch")
	}

	// The public pages follow the stored panel language.
	public := panelRequest(t, env.handler, http.MethodGet, "/", nil, nil, nil)
	if public.Code != http.StatusOK {
		t.Fatalf("GET / = %d", public.Code)
	}
	if !strings.Contains(public.Body.String(), "Публичные архивы") {
		t.Fatal("public page did not follow the panel language")
	}
}

func TestAdminLanguageRejectsUnknownValueAndForeignRedirect(t *testing.T) {
	env := newAdminTestEnv(t, nil)

	rec := env.request(t, http.MethodPost, "/admin/language", url.Values{
		"language": {"de"},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unsupported language = %d, want 400", rec.Code)
	}

	rec = env.request(t, http.MethodPost, "/admin/language", url.Values{
		"language": {"ru"},
		"next":     {"//evil.example/admin"},
	})
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin" {
		t.Fatalf("foreign redirect = %d %q", rec.Code, rec.Header().Get("Location"))
	}

	rec = panelRequest(t, env.handler, http.MethodPost, "/admin/language",
		url.Values{"language": {"ru"}}, []*http.Cookie{env.cookie}, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("POST /admin/language without CSRF = %d, want 403", rec.Code)
	}
}

func TestPageI18nKeepsMarkupButEscapesArguments(t *testing.T) {
	dir := t.TempDir()
	setting, err := openLanguageSetting(dir, i18n.English, nil)
	if err != nil {
		t.Fatalf("openLanguageSetting: %v", err)
	}
	page := pageI18n{settings: setting}

	// Sentences that wrap <code> around technical terms keep the markup.
	if got := string(page.TH(i18n.KeyPanelDashboardDoctorHint)); !strings.Contains(got, "<code>doctor</code>") {
		t.Fatalf("TH dropped inline markup: %q", got)
	}
	// Archive names come from the operator, so they must never become markup.
	got := string(page.THf(i18n.KeyPanelModsRollbackText, "<script>alert(1)</script>"))
	if strings.Contains(got, "<script>") {
		t.Fatalf("THf escaped the sentence but not the argument: %q", got)
	}
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Fatalf("THf did not escape the argument: %q", got)
	}
}

func TestAdminPagesAreTranslatedInBothLanguages(t *testing.T) {
	env := newAdminTestEnv(t, &fakeAdminOps{
		statuses:        map[string]Status{"tes3mp.service": {State: "running"}},
		backups:         map[string]Backups{"state": stateBackupsFixture(), "mods": modsBackupsFixture()},
		configSnapshots: adminConfigSnapshots(),
	})
	pages := []string{"/admin", "/admin/logs", "/admin/backups",
		"/admin/config", "/admin/players", "/admin/password", "/", "/backups"}

	for _, path := range pages {
		rec := env.request(t, http.MethodGet, path, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d: %s", path, rec.Code, rec.Body.String())
		}
		// The language selector prints every language in its own script
		// ("Русский"), which is not untranslated text.
		if hasCyrillic(withoutLanguageNames(rec.Body.String())) {
			t.Fatalf("GET %s in English still contains Russian text:\n%s", path, rec.Body.String())
		}
		// A misspelled key leaks the catalog key instead of a sentence.
		if key := panelKeyPattern.FindString(rec.Body.String()); key != "" {
			t.Fatalf("GET %s renders the raw catalog key %q:\n%s", path, key, rec.Body.String())
		}
	}

	if err := env.h.language.Set(i18n.Russian); err != nil {
		t.Fatalf("switch to ru: %v", err)
	}
	for _, path := range pages {
		rec := env.request(t, http.MethodGet, path, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s in Russian = %d: %s", path, rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		if !strings.Contains(body, `<html lang="ru">`) {
			t.Fatalf("GET %s has no Russian lang attribute", path)
		}
		if !hasCyrillic(body) {
			t.Fatalf("GET %s did not translate the page body:\n%s", path, body)
		}
		if key := panelKeyPattern.FindString(body); key != "" {
			t.Fatalf("GET %s renders the raw catalog key %q:\n%s", path, key, body)
		}
	}
}

// panelKeyPattern matches a catalog key such as panel.dashboard.noscript, which
// reaches the page only when a template asks for a key the catalog lacks.
var panelKeyPattern = regexp.MustCompile(`panel\.[a-z_]+\.[a-z_]+`)

func TestPasswordPageOffersLanguageSelector(t *testing.T) {
	env := newAdminTestEnv(t, &fakeAdminOps{})
	rec := env.request(t, http.MethodGet, "/admin/password", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin/password = %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		`action="/admin/language"`,
		`name="next" value="/admin/password"`,
		`name="language" value="en"`,
		`name="language" value="ru"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("password page has no language selector (%q):\n%s", want, body)
		}
	}
}

// hasCyrillic reports whether a rendered page still carries untranslated
// Russian text, which is how the bilingual regression test detects leftovers.
func hasCyrillic(text string) bool {
	for _, r := range text {
		if r >= '\u0400' && r <= '\u04FF' {
			return true
		}
	}
	return false
}

// withoutLanguageNames drops the endonyms of the language selector.
func withoutLanguageNames(text string) string {
	for _, language := range i18n.Supported() {
		text = strings.ReplaceAll(text, i18n.DisplayName(language), "")
	}
	return text
}

func TestPageI18nWithoutSettingsFallsBackToEnglish(t *testing.T) {
	var page pageI18n
	if got := page.T(i18n.KeyPanelNavLogs); got != "Logs" {
		t.Fatalf("T without settings = %q, want %q", got, "Logs")
	}
	if got := page.Lang(); got != "en" {
		t.Fatalf("Lang without settings = %q", got)
	}
	if len(page.Languages()) != 2 {
		t.Fatalf("Languages without settings = %+v", page.Languages())
	}
}

func TestConfigLanguage(t *testing.T) {
	if got := DefaultConfig().Normalize().Language; got != string(DefaultLanguage) {
		t.Fatalf("default language = %q, want %q", got, DefaultLanguage)
	}

	cfg := ConfigFromEnv(func(key string) string {
		if key == EnvLanguage {
			return "ru"
		}
		return ""
	})
	if got := cfg.Normalize().Language; got != "ru" {
		t.Fatalf("ConfigFromEnv language = %q, want ru", got)
	}

	cfg = DefaultConfig()
	cfg.TLSMode = TLSModeFiles
	cfg.Language = "de"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate accepted an unsupported language")
	}

	cfg = DefaultConfig()
	cfg.TLSMode = TLSModeFiles
	cfg.Language = "ru_RU.UTF-8"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate rejected a supported locale: %v", err)
	}
}
