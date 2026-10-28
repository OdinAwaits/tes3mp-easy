// Package app contains client scenarios that do not depend on Fyne.
//
// It exposes configuration, setup, download and launch operations for the
// shell; the UI only calls this package and renders the result.
package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/clientlog"
	"github.com/Saagilat/tes3mp-easy/internal/config"
	"github.com/Saagilat/tes3mp-easy/internal/fonts"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/launcher"
	"github.com/Saagilat/tes3mp-easy/internal/locale"
	"github.com/Saagilat/tes3mp-easy/internal/mods"
	"github.com/Saagilat/tes3mp-easy/internal/serverapi"
	"github.com/Saagilat/tes3mp-easy/internal/tes3mp"
	"github.com/Saagilat/tes3mp-easy/internal/version"
)

// TailLines is the number of recent client log lines shown by the UI.
const TailLines = 300

var (
	// Version is the client version shown in About and the CLI.
	Version = version.Client
)

const (
	// TES3MPVersion is the bundled TES3MP version expected by the client.
	TES3MPVersion = tes3mp.Version

	// ProjectURL is the public project page.
	ProjectURL = "https://github.com/Saagilat/tes3mp-easy"
)

// Settings are the subset of configuration edited from the Settings dialog.
// Game localization is a separate optional install action, not a setting.
type Settings struct {
	Language      string
	Theme         string
	MorrowindPath string
	WorkDir       string
	ProtonPath    string
	SkipFonts     bool
}

// Service owns the configuration store used by the UI.
type Service struct {
	store   *config.Store
	paths   config.Paths
	getenv  func(string) string
	goos    string
	homeDir string

	// installTES3MP is replaceable in tests; production uses tes3mp.Install.
	installTES3MP func(context.Context, tes3mp.Options) (tes3mp.Result, error)
	// installMods is replaceable in tests; production uses mods.Install.
	installMods func(context.Context, mods.Options) (mods.Result, error)
	// installFonts is replaceable in tests; production uses fonts.Install.
	installFonts func(context.Context, fonts.Options) (fonts.Result, error)
	// installLocale is replaceable in tests; production uses locale.Install.
	installLocale func(context.Context, locale.Options) (locale.Result, error)
	// launchClient is replaceable in tests; production uses launcher.Launch.
	launchClient func(context.Context, launcher.LaunchOptions) (launcher.Result, error)
	// newServerAPI is replaceable in tests; production uses serverapi.New.
	newServerAPI func(serverapi.Options) (*serverapi.Client, error)
	// httpClient is used by the public API and archive download; nil means the
	// production default from serverapi.New.
	httpClient *http.Client
	// logFile receives diagnostics and user-facing error details.
	logFile *clientlog.Logger
	// launchGuard keeps a single TES3MP instance: a repeated Play click must not
	// start a second client that would share the same profile and config.
	launchGuard launchGuard
}

// Open loads the configuration at paths.ConfigFile.
func Open(paths config.Paths) (*Service, error) {
	return OpenWithEnv(paths, os.Getenv)
}

// OpenWithEnv is the testable form of Open.
func OpenWithEnv(paths config.Paths, getenv func(string) string) (*Service, error) {
	if strings.TrimSpace(paths.ConfigFile) == "" {
		return nil, errors.New("config file path is empty")
	}
	if getenv == nil {
		getenv = os.Getenv
	}
	store, err := config.OpenStore(paths.ConfigFile)
	if err != nil {
		return nil, err
	}
	logFile, err := clientlog.Open(paths.LogsDir)
	if err != nil {
		return nil, fmt.Errorf("open client log: %w", err)
	}
	homeDir, _ := os.UserHomeDir()
	service := &Service{
		store:   store,
		paths:   paths,
		getenv:  getenv,
		goos:    runtime.GOOS,
		homeDir: homeDir,
		logFile: logFile,
	}
	service.LogInfof("client started; config=%s work=%s", paths.ConfigFile,
		service.WorkDir())
	return service, nil
}

// Close releases the client log. It is safe to call more than once.
func (s *Service) Close() error {
	if s == nil || s.logFile == nil {
		return nil
	}
	return s.logFile.Close()
}

// NeedsSetup reports whether the local installation has not been completed.
// Server profiles deliberately do not affect this: adding a server is a
// separate step, not part of installing TES3MP and fonts.
func (s *Service) NeedsSetup() bool {
	return s == nil || !s.Config().SetupComplete
}

// CompleteSetup records that the first-run wizard reached its final step.
func (s *Service) CompleteSetup() error {
	if s == nil {
		return errors.New("app service is nil")
	}
	return s.store.Update(func(cfg *config.Config) error {
		cfg.SetupComplete = true
		return nil
	})
}

// LogPath returns the client log path even if logging is not available.
func (s *Service) LogPath() string {
	if s == nil {
		return ""
	}
	if s.logFile != nil {
		return s.logFile.Path()
	}
	return filepath.Join(s.paths.LogsDir, clientlog.FileName)
}

// LogInfof writes an informational entry to the client log.
func (s *Service) LogInfof(format string, args ...any) {
	if s != nil && s.logFile != nil {
		s.logFile.Infof(format, args...)
	}
}

// LogWarnf writes a warning entry to the client log.
func (s *Service) LogWarnf(format string, args ...any) {
	if s != nil && s.logFile != nil {
		s.logFile.Warnf(format, args...)
	}
}

// LogError records a failed operation without changing the returned error.
// The stable failure code makes the log searchable without matching UI copy.
func (s *Service) LogError(scope string, err error) {
	if s == nil || s.logFile == nil || err == nil {
		return
	}
	failure := Explain(err)
	s.logFile.Errorf("%s: %v (code=%s kind=%s)", strings.TrimSpace(scope),
		err, failure.Code, failure.Kind)
}

// ReadLogTail returns the last lines of the client log for the Logs dialog.
func (s *Service) ReadLogTail(maxLines int) ([]string, error) {
	if s == nil {
		return nil, errors.New("app service is nil")
	}
	return clientlog.Tail(s.LogPath(), maxLines)
}

// Config returns an independent configuration snapshot.
func (s *Service) Config() config.Config {
	if s == nil {
		return config.Default()
	}
	return s.store.Snapshot()
}

// Paths returns the per-user paths used by the client.
func (s *Service) Paths() config.Paths {
	if s == nil {
		return config.Paths{}
	}
	return s.paths
}

// ConfigPath returns the configuration file path.
func (s *Service) ConfigPath() string {
	if s == nil {
		return ""
	}
	return s.store.Path()
}

// WorkDir returns the configured work directory or the platform default.
func (s *Service) WorkDir() string {
	if s == nil {
		return ""
	}
	if value := s.Config().WorkDir; value != "" {
		return value
	}
	return s.paths.DataDir
}

// Language returns the effective interface language.
func (s *Service) Language() i18n.Language {
	if s == nil {
		return i18n.English
	}
	return i18n.Resolve(s.Config().Language, s.getenv)
}

// Translator returns a translator for the effective language.
func (s *Service) Translator() *i18n.Translator {
	return i18n.New(s.Language())
}
