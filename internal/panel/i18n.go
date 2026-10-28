package panel

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
)

const (
	// SettingsFileName keeps panel-wide preferences next to auth.json.
	SettingsFileName = "settings.json"

	settingsFileVersion = 1
	maxSettingsBytes    = 4 << 10
)

// settingsRecord is the settings.json body. The version field leaves room for
// more panel preferences without a second file.
type settingsRecord struct {
	Version  int    `json:"version"`
	Language string `json:"language"`
}

// languageSetting stores the interface language of the whole panel. The choice
// lives in settings.json, so it survives a restart and every browser sees the
// same panel; TES3MP_PANEL_LANGUAGE only supplies the default.
type languageSetting struct {
	path     string
	logger   *log.Logger
	fallback i18n.Language

	mu         sync.RWMutex
	language   i18n.Language
	translator *i18n.Translator
}

// openLanguageSetting loads settings.json. A missing file keeps the fallback
// until an operator switches the language; a broken file is an error, exactly
// like a broken auth.json, so the operator notices it instead of silently
// losing the setting.
func openLanguageSetting(dataDir string, fallback i18n.Language, logger *log.Logger) (*languageSetting, error) {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	if _, ok := i18n.Parse(string(fallback)); !ok {
		fallback = i18n.English
	}
	setting := &languageSetting{
		path:       filepath.Join(dataDir, SettingsFileName),
		logger:     logger,
		fallback:   fallback,
		language:   fallback,
		translator: i18n.New(fallback),
	}
	data, err := readSettingsFile(setting.path)
	if errors.Is(err, fs.ErrNotExist) {
		return setting, nil
	}
	if err != nil {
		return nil, err
	}
	var record settingsRecord
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return nil, fmt.Errorf("decode panel settings: %w", err)
	}
	if record.Version != settingsFileVersion {
		return nil, fmt.Errorf("unsupported panel settings version %d", record.Version)
	}
	language, ok := i18n.Parse(record.Language)
	if !ok {
		return nil, fmt.Errorf("unsupported panel language %q", record.Language)
	}
	setting.language = language
	setting.translator = i18n.New(language)
	return setting, nil
}

func readSettingsFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxSettingsBytes {
		return nil, fmt.Errorf("panel settings are %d bytes, limit is %d",
			info.Size(), maxSettingsBytes)
	}
	return os.ReadFile(path)
}

// Language returns the current interface language.
func (s *languageSetting) Language() i18n.Language {
	if s == nil {
		return i18n.English
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.language
}

// Translator returns a translator for the current language. It is safe to call
// on a nil setting, which keeps templates working in tests with a bare handler.
func (s *languageSetting) Translator() *i18n.Translator {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.translator
}

// Default returns the language configured for the process (panel.env), used by
// the selector to explain what a fresh install starts with.
func (s *languageSetting) Default() i18n.Language {
	if s == nil {
		return i18n.English
	}
	return s.fallback
}

// Set validates and persists language. The file is written before the in-memory
// value changes, so a failed write keeps the previous language.
func (s *languageSetting) Set(language i18n.Language) error {
	normalized, ok := i18n.Parse(string(language))
	if !ok {
		return fmt.Errorf("unsupported panel language %q", language)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if normalized == s.language {
		return nil
	}
	data, err := json.MarshalIndent(settingsRecord{
		Version:  settingsFileVersion,
		Language: string(normalized),
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode panel settings: %w", err)
	}
	data = append(data, '\n')
	if err := atomicfile.WriteFile(s.path, data, 0o600); err != nil {
		return fmt.Errorf("write panel settings: %w", err)
	}
	s.language = normalized
	s.translator = i18n.New(normalized)
	s.logger.Printf("panel: language set to %s", normalized)
	return nil
}

// languageOption feeds the language selector in the panel header.
type languageOption struct {
	Code    string
	Name    string
	Current bool
}

// pageI18n is embedded into every page data struct, so one template set serves
// every language: {{.T "panel.dashboard.start"}} resolves per request.
type pageI18n struct {
	settings *languageSetting
}

// T translates key in the language of the current panel.
func (p pageI18n) T(key string) string { return p.settings.Translator().T(key) }

// Tf translates key and applies fmt.Sprintf arguments.
func (p pageI18n) Tf(key string, args ...any) string {
	return p.settings.Translator().Tf(key, args...)
}

// Pluralf selects the plural form of key for count and formats it.
func (p pageI18n) Pluralf(key string, count int) string {
	return p.settings.Translator().Pluralf(key, count)
}

// Lang returns the language tag for the <html lang> attribute.
func (p pageI18n) Lang() string { return string(p.settings.Language()) }

// Languages lists the switchable languages with the current one marked.
func (p pageI18n) Languages() []languageOption {
	current := p.settings.Language()
	supported := i18n.Supported()
	options := make([]languageOption, 0, len(supported))
	for _, language := range supported {
		options = append(options, languageOption{
			Code:    string(language),
			Name:    i18n.DisplayName(language),
			Current: language == current,
		})
	}
	return options
}

// pageI18n returns the localization block for page data built by this handler.
func (h *panelHandler) pageI18n() pageI18n {
	return pageI18n{settings: h.language}
}

// tr returns the translator of the current panel language for strings that are
// built in Go instead of templates (labels, dates, error messages).
func (h *panelHandler) tr() *i18n.Translator {
	return h.language.Translator()
}
