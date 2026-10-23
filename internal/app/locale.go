package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/Saagilat/tes3mp-easy/internal/config"
	"github.com/Saagilat/tes3mp-easy/internal/locale"
	"github.com/Saagilat/tes3mp-easy/internal/mods"
)

// LocalizationState reports the installed Russian overlay for a profile.
func (s *Service) LocalizationState(serverID string) (locale.State, error) {
	if s == nil {
		return locale.State{}, errors.New("app service is nil")
	}
	if _, err := s.serverProfile(serverID); err != nil {
		return locale.State{}, err
	}
	workDir, err := s.absoluteWorkDir()
	if err != nil {
		return locale.State{}, err
	}
	return locale.Inspect(workDir, serverID)
}

// InstallLocalization downloads the Russian overlay and, when mods are already
// installed, copies it over the profile Data Files.
func (s *Service) InstallLocalization(ctx context.Context, serverID string,
	voiceover bool, progress func(locale.Progress),
) (locale.Result, error) {
	if s == nil {
		return locale.Result{}, errors.New("app service is nil")
	}
	if _, err := s.serverProfile(serverID); err != nil {
		return locale.Result{}, err
	}
	workDir, err := s.absoluteWorkDir()
	if err != nil {
		return locale.Result{}, err
	}
	cacheDir, err := s.absoluteCacheDir()
	if err != nil {
		return locale.Result{}, err
	}
	options := locale.DefaultOptions(workDir, cacheDir, serverID, voiceover)
	options.Client = s.httpClient
	options.Progress = progress
	result := locale.Result{}
	if s.installLocale != nil {
		result, err = s.installLocale(ctx, options)
	} else {
		result, err = locale.Install(ctx, options)
	}
	if err != nil {
		return result, err
	}
	if err := s.store.Update(func(cfg *config.Config) error {
		cfg.Localization = true
		return nil
	}); err != nil {
		return result, fmt.Errorf("save localization setting: %w", err)
	}
	state, stateErr := mods.Inspect(workDir, serverID)
	if stateErr != nil {
		return result, stateErr
	}
	if state.Installed {
		applied, applyErr := locale.Apply(workDir, serverID)
		result.Warnings = append(result.Warnings, applied.Warnings...)
		if applyErr != nil {
			return result, fmt.Errorf("apply localization overlay: %w", applyErr)
		}
	}
	return result, nil
}

// RemoveLocalization removes the overlay files and disables the setting.
func (s *Service) RemoveLocalization(serverID string) ([]string, error) {
	if s == nil {
		return nil, errors.New("app service is nil")
	}
	if _, err := s.serverProfile(serverID); err != nil {
		return nil, err
	}
	workDir, err := s.absoluteWorkDir()
	if err != nil {
		return nil, err
	}
	warnings, err := locale.RemoveOverlay(workDir, serverID)
	if err != nil {
		return nil, err
	}
	if err := locale.Remove(workDir, serverID); err != nil {
		return nil, err
	}
	if err := s.store.Update(func(cfg *config.Config) error {
		cfg.Localization = false
		return nil
	}); err != nil {
		return warnings, fmt.Errorf("save localization setting: %w", err)
	}
	return warnings, nil
}
