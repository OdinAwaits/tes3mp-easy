package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/Saagilat/tes3mp-easy/internal/fonts"
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
	profile, err := s.serverProfile(serverID)
	if err != nil {
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
	warnings := []string{}
	localization, inspectErr := locale.Inspect(workDir)
	if inspectErr != nil {
		s.LogWarnf("localization is ignored: %v", inspectErr)
		warnings = append(warnings,
			fmt.Sprintf("localization is ignored: %v", inspectErr))
		localization = locale.State{}
	}
	localizationRoot := locale.Dir(workDir)
	localizationDataFiles := ""
	encoding := ""
	if localization.Installed {
		localizationDataFiles = localization.DataFiles
		encoding = locale.EncodingFor(localization.Language)
	}

	fontResult := fonts.Result{}
	if !cfg.SkipFonts || localization.Installed {
		var fontErr error
		fontResult, fontErr = s.InstallFonts(ctx)
		warnings = append(warnings, fontResult.Warnings...)
		if fontErr != nil {
			warnings = append(warnings,
				fmt.Sprintf("fonts were not installed: %v", fontErr))
		}
	}
	options := launcher.LaunchOptions{
		Options: launcher.Options{
			GOOS:          s.goos,
			WorkDir:       workDir,
			MorrowindRoot: cfg.MorrowindPath,
			ProtonPath:    cfg.ProtonPath,
			HomeDir:       s.homeDir,
		},
		ProfilePath:           modsState.Profile,
		ServerAddress:         profile.Address,
		ServerPort:            profile.Port,
		ServerPassword:        profile.Password,
		Encoding:              encoding,
		LocalizationRoot:      localizationRoot,
		LocalizationDataFiles: localizationDataFiles,
		Fonts:                 fontResult.State.Installed,
		FontSet:               s.FontSet(),
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
	localization, err := locale.Inspect(workDir)
	if err != nil {
		return launcher.Result{}, err
	}
	localizationDataFiles := ""
	encoding := ""
	if localization.Installed {
		localizationDataFiles = localization.DataFiles
		encoding = locale.EncodingFor(localization.Language)
	}
	options := launcher.LaunchOptions{
		Options: launcher.Options{
			GOOS:          s.goos,
			WorkDir:       workDir,
			MorrowindRoot: cfg.MorrowindPath,
			ProtonPath:    cfg.ProtonPath,
			HomeDir:       s.homeDir,
		},
		ProfilePath:           modsState.Profile,
		Encoding:              encoding,
		LocalizationRoot:      locale.Dir(workDir),
		LocalizationDataFiles: localizationDataFiles,
	}
	return launcher.OpenMWCS(ctx, options)
}
