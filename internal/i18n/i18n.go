// Package i18n provides the embedded English and Russian interface strings.
package i18n

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// Language is a supported interface language.
type Language string

const (
	English Language = "en"
	Russian Language = "ru"
)

var supported = []Language{English, Russian}

// Supported returns the interface languages in display order.
func Supported() []Language {
	return append([]Language(nil), supported...)
}

// Parse accepts a language tag such as "en", "ru", "ru_RU.UTF-8" or
// "en-US" and returns its supported base language.
func Parse(value string) (Language, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", false
	}
	if index := strings.IndexAny(value, ".@"); index >= 0 {
		value = value[:index]
	}
	value = strings.ReplaceAll(value, "-", "_")
	if index := strings.IndexByte(value, '_'); index >= 0 {
		value = value[:index]
	}
	switch Language(value) {
	case English:
		return English, true
	case Russian:
		return Russian, true
	default:
		return "", false
	}
}

// Detect selects a language from the usual POSIX locale variables. Unknown
// locales and C/POSIX fall back to English.
func Detect(getenv func(string) string) Language {
	if getenv == nil {
		getenv = os.Getenv
	}
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG", "LANGUAGE"} {
		value := getenv(name)
		if name == "LANGUAGE" {
			for _, candidate := range strings.Split(value, ":") {
				if language, ok := Parse(candidate); ok {
					return language
				}
			}
			continue
		}
		if language, ok := Parse(value); ok {
			return language
		}
	}
	return English
}

// Resolve prefers an explicit supported setting and otherwise detects the
// system language.
func Resolve(setting string, getenv func(string) string) Language {
	if language, ok := Parse(setting); ok {
		return language
	}
	return Detect(getenv)
}

// Translator returns strings for one language with English as a fallback.
type Translator struct {
	language Language
}

// New creates a translator. An unsupported language falls back to English.
func New(language Language) *Translator {
	if _, ok := Parse(string(language)); !ok {
		language = English
	}
	return &Translator{language: language}
}

// ForSetting creates a translator from a persisted setting and the system
// environment.
func ForSetting(setting string, getenv func(string) string) *Translator {
	return New(Resolve(setting, getenv))
}

// Language returns the effective language.
func (t *Translator) Language() Language {
	if t == nil {
		return English
	}
	return t.language
}

// T translates key. It falls back to English and finally to the key itself so
// a missing label is visible during development without crashing the UI.
func (t *Translator) T(key string) string {
	language := English
	if t != nil {
		language = t.language
	}
	if catalog := catalogs[language]; catalog != nil {
		if value, ok := catalog[key]; ok && value != "" {
			return value
		}
	}
	if value, ok := catalogs[English][key]; ok && value != "" {
		return value
	}
	return key
}

// Tf translates key and applies fmt.Sprintf arguments.
func (t *Translator) Tf(key string, args ...any) string {
	return fmt.Sprintf(t.T(key), args...)
}

// Pluralf selects the plural form of key for count and formats it with count.
// Catalog entries use CLDR suffixes: key.one, key.few, key.many and key.other.
// English uses one/other, Russian uses one/few/many; a missing form falls back
// to other and finally to the English catalog.
func (t *Translator) Pluralf(key string, count int) string {
	language := t.Language()
	form := pluralForm(language, count)
	for _, candidate := range pluralCandidates(language, form) {
		if value, ok := lookup(language, key+"."+candidate); ok {
			return fmt.Sprintf(value, count)
		}
	}
	return key
}

// pluralForm returns the CLDR plural category of count for the language.
func pluralForm(language Language, count int) string {
	if language == Russian {
		switch {
		case count%10 == 1 && count%100 != 11:
			return "one"
		case count%10 >= 2 && count%10 <= 4 && (count%100 < 12 || count%100 > 14):
			return "few"
		default:
			return "many"
		}
	}
	if count == 1 {
		return "one"
	}
	return "other"
}

// pluralCandidates orders the lookup attempts for a plural category.
func pluralCandidates(language Language, form string) []string {
	switch form {
	case "one":
		return []string{"one", "other", "many"}
	case "few":
		return []string{"few", "other", "many"}
	case "many":
		return []string{"many", "other", "few"}
	default:
		return []string{"other", "many", "one"}
	}
}

// lookup returns a catalog entry for the language, never falling back itself.
func lookup(language Language, key string) (string, bool) {
	catalog := catalogs[language]
	if catalog == nil {
		return "", false
	}
	value, ok := catalog[key]
	if !ok || value == "" {
		return "", false
	}
	return value, true
}

// Keys returns all translation keys in stable order.
func Keys() []string {
	seen := make(map[string]struct{}, len(catalogs[English]))
	for key := range catalogs[English] {
		seen[key] = struct{}{}
	}
	for key := range catalogs[Russian] {
		seen[key] = struct{}{}
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// DisplayName returns the language's own name for selectors.
func DisplayName(language Language) string {
	switch language {
	case Russian:
		return "Русский"
	default:
		return "English"
	}
}
