package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/Saagilat/tes3mp-easy/internal/launcher"
	"github.com/Saagilat/tes3mp-easy/internal/locale"
	"github.com/Saagilat/tes3mp-easy/internal/mods"
)

// Launch prepares fonts, the active OpenMW config and starts TES3MP. The
// returned process is already running; the caller should wait in a goroutine.
func (s *Service) Launch(ctx context.Context, serverID string) (launcher.Result, error) {
	if s == nil {
		return launcher.Result{}, errors.New("app service is nil")
	}
	if _, err := s.serverProfile(serverID); err != nil {
		return launcher.Result{}, err
	}
	workDir, err := s.absoluteWorkDir()
	if err != nil {
		return launcher.Result{}, err
	}
	modsState, err := mods.Inspect(workDir, serverID)
	if err != nil {
		return launcher.Result{}, err
	}
	if !modsState.Installed {
		return launcher.Result{}, fmt.Errorf(
			"%w; update mods before launching", ErrModsNotInstalled)
	}
	cfg := s.Config()
	if cfg.MorrowindPath == "" {
		return launcher.Result{}, ErrMorrowindNotConfigured
	}
	if cfg.Localization {
		state, err := locale.Inspect(workDir, serverID)
		if err != nil {
			return launcher.Result{}, err
		}
		if !state.Installed {
			return launcher.Result{}, fmt.Errorf(
				"%w; open Settings and install it again",
				ErrLocalizationNotInstalled)
		}
	}

	fontResult, fontErr := s.InstallFonts(ctx)
	warnings := append([]string(nil), fontResult.Warnings...)
	if fontErr != nil {
		warnings = append(warnings,
			fmt.Sprintf("fonts were not installed: %v", fontErr))
	}
	options := launcher.LaunchOptions{
		Options: launcher.Options{
			GOOS:          s.goos,
			WorkDir:       workDir,
			MorrowindRoot: cfg.MorrowindPath,
			ProtonPath:    cfg.ProtonPath,
			HomeDir:       s.homeDir,
		},
		ProfilePath:  modsState.Profile,
		Localization: cfg.Localization,
		Fonts:        fontResult.State.Installed,
	}
	var result launcher.Result
	if s.launchClient != nil {
		result, err = s.launchClient(ctx, options)
	} else {
		result, err = launcher.Launch(ctx, options)
	}
	if err != nil {
		s.LogError("launch", err)
		return result, err
	}
	s.LogInfof("launch %s started", serverID)
	result.Warnings = append(warnings, result.Warnings...)
	return result, nil
}

// OpenMWCS starts the editor from the TES3MP directory when it is present.
func (s *Service) OpenMWCS(ctx context.Context,
	serverID string,
) (launcher.Result, error) {
	if s == nil {
		return launcher.Result{}, errors.New("app service is nil")
	}
	if _, err := s.serverProfile(serverID); err != nil {
		return launcher.Result{}, err
	}
	workDir, err := s.absoluteWorkDir()
	if err != nil {
		return launcher.Result{}, err
	}
	modsState, err := mods.Inspect(workDir, serverID)
	if err != nil {
		return launcher.Result{}, err
	}
	if !modsState.Installed {
		return launcher.Result{}, fmt.Errorf(
			"%w; update mods before launching OpenMW-CS", ErrModsNotInstalled)
	}
	cfg := s.Config()
	options := launcher.LaunchOptions{
		Options: launcher.Options{
			GOOS:          s.goos,
			WorkDir:       workDir,
			MorrowindRoot: cfg.MorrowindPath,
			ProtonPath:    cfg.ProtonPath,
			HomeDir:       s.homeDir,
		},
		ProfilePath:  modsState.Profile,
		Localization: cfg.Localization,
	}
	return launcher.OpenMWCS(ctx, options)
}
