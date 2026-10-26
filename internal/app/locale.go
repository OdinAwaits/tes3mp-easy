package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/config"
	"github.com/Saagilat/tes3mp-easy/internal/locale"
)

// LocalizationState reports the global optional language overlay.
func (s *Service) LocalizationState() (locale.State, error) {
	if s == nil {
		return locale.State{}, errors.New("app service is nil")
	}
	workDir, err := s.absoluteWorkDir()
	if err != nil {
		return locale.State{}, err
	}
	return locale.Inspect(workDir)
}

// Voiceover reports whether a localization install includes the optional
// Russian voice archive. The stored preference is used for the wizard
// checkbox; a configuration without the field defaults to enabled.
func (s *Service) Voiceover() bool {
	if s == nil {
		return true
	}
	return s.Config().VoiceoverEnabled()
}

// SetVoiceover persists the voiceover preference for the next localization
// install.
func (s *Service) SetVoiceover(enabled bool) error {
	if s == nil {
		return errors.New("app service is nil")
	}
	return s.store.Update(func(cfg *config.Config) error {
		cfg.SetVoiceover(enabled)
		return nil
	})
}

// InstallLocalization downloads and activates one localization package for all
// server profiles. voices adds the optional voice archive; a cancelled
// voiceover download is reported as a warning and the text remains installed.
// The original Morrowind folder is never modified. Cyrillic fonts are installed
// automatically; a font failure is reported as a warning.
func (s *Service) InstallLocalization(ctx context.Context, language string,
	voices bool, progress func(locale.Progress),
) (locale.Result, error) {
	if s == nil {
		return locale.Result{}, errors.New("app service is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	language = strings.ToLower(strings.TrimSpace(language))
	if _, ok := locale.PackFor(language); !ok {
		return locale.Result{}, fmt.Errorf("unsupported localization %q",
			language)
	}
	workDir, err := s.absoluteWorkDir()
	if err != nil {
		return locale.Result{}, err
	}
	cacheDir, err := s.absoluteCacheDir()
	if err != nil {
		return locale.Result{}, err
	}
	options := locale.DefaultOptions(workDir, cacheDir, language, voices)
	options.Client = s.httpClient
	options.Progress = progress
	result := locale.Result{}
	if s.installLocale != nil {
		result, err = s.installLocale(ctx, options)
	} else {
		result, err = locale.Install(ctx, options)
	}
	if err != nil {
		s.LogError("install localization", err)
		return result, err
	}

	fontResult, fontErr := s.InstallFonts(ctx)
	result.Warnings = append(result.Warnings, fontResult.Warnings...)
	if fontErr != nil {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Cyrillic fonts were not installed: %v", fontErr))
	}
	s.LogInfof("localization %s installed (voiceover=%t, skipped=%t)",
		language, result.State.Voiceover, result.Skipped)
	return result, nil
}

// RemoveLocalization deletes the global overlay, returning every server to its
// original Morrowind language.
func (s *Service) RemoveLocalization() error {
	if s == nil {
		return errors.New("app service is nil")
	}
	workDir, err := s.absoluteWorkDir()
	if err != nil {
		return err
	}
	if err := locale.Remove(workDir); err != nil {
		s.LogError("remove localization", err)
		return err
	}
	s.LogInfof("global localization removed")
	return nil
}
