package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/fonts"
	"github.com/Saagilat/tes3mp-easy/internal/launcher"
)

// OpenMWConfigDir returns the active OpenMW configuration directory for the
// current platform and configuration.
func (s *Service) OpenMWConfigDir() (string, error) {
	if s == nil {
		return "", errors.New("app service is nil")
	}
	workDir, err := s.absoluteWorkDir()
	if err != nil {
		return "", err
	}
	return launcher.OpenMWConfigDir(launcher.Options{
		GOOS:      s.goos,
		WorkDir:   workDir,
		PrefixDir: filepath.Join(workDir, "prefix"),
		HomeDir:   s.homeDir,
	})
}

// InstallFonts downloads the fixed font set into the OpenMW config directory.
// A failed download is returned as warnings, not as a launch error.
func (s *Service) InstallFonts(ctx context.Context) (fonts.Result, error) {
	if s == nil {
		return fonts.Result{}, errors.New("app service is nil")
	}
	configDir, err := s.OpenMWConfigDir()
	if err != nil {
		return fonts.Result{}, err
	}
	cacheDir, err := s.absoluteCacheDir()
	if err != nil {
		return fonts.Result{}, err
	}
	options := fonts.DefaultOptions(configDir, cacheDir)
	if s.httpClient != nil {
		options.Client = s.httpClient
	}
	result := fonts.Result{}
	if s.installFonts != nil {
		result, err = s.installFonts(ctx, options)
	} else {
		result, err = fonts.Install(ctx, options)
	}
	if err != nil {
		s.LogError("install fonts", err)
		return result, err
	}
	if len(result.Warnings) > 0 {
		s.LogWarnf("fonts: %s", strings.Join(result.Warnings, "; "))
	} else {
		s.LogInfof("fonts are installed")
	}
	return result, nil
}

// FontsState verifies the files in the active OpenMW Fonts directory.
func (s *Service) FontsState() (fonts.State, error) {
	configDir, err := s.OpenMWConfigDir()
	if err != nil {
		return fonts.State{}, err
	}
	return fonts.Inspect(filepath.Join(configDir, "Fonts"))
}

func (s *Service) absoluteCacheDir() (string, error) {
	cacheDir := strings.TrimSpace(s.paths.CacheDir)
	if cacheDir == "" {
		return "", errors.New("client cache directory is empty")
	}
	if !filepath.IsAbs(cacheDir) {
		return "", fmt.Errorf("client cache directory %q is not absolute", cacheDir)
	}
	return filepath.Clean(cacheDir), nil
}
