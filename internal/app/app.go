// Package app contains client scenarios that do not depend on Fyne.
//
// K01 exposes configuration and profile operations for the shell. Later client
// steps add setup, update and launch scenarios here; the UI only calls this
// package and renders the result.
package app

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/config"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/tes3mp"
)

const (
	// Version is the client version shown in About and the CLI.
	Version = "0.1.0"

	// TES3MPVersion is the bundled TES3MP version expected by the client.
	TES3MPVersion = tes3mp.Version

	// ProjectURL is the public project page.
	ProjectURL = "https://github.com/Saagilat/tes3mp-easy"
)

// Settings are the subset of configuration edited from the Settings dialog.
type Settings struct {
	Language      string
	MorrowindPath string
	WorkDir       string
	ProtonPath    string
}

// Service owns the configuration store used by the UI.
type Service struct {
	store  *config.Store
	paths  config.Paths
	getenv func(string) string

	// installTES3MP is replaceable in tests; production uses tes3mp.Install.
	installTES3MP func(context.Context, tes3mp.Options) (tes3mp.Result, error)
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
	return &Service{store: store, paths: paths, getenv: getenv}, nil
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
