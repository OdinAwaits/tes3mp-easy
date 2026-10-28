package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/config"
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

// InstallFonts downloads the selected font set into the OpenMW config
// directory. A failed download is returned as warnings, not as a launch error.
func (s *Service) InstallFonts(ctx context.Context) (fonts.Result, error) {
	return s.InstallFontsWithProgress(ctx, nil)
}

// InstallFontsWithProgress is InstallFonts with per-file download updates for
// the Installation screen. The callback runs on the calling goroutine during
// the download, so the UI must marshal it onto the Fyne thread.
func (s *Service) InstallFontsWithProgress(ctx context.Context,
	progress func(fonts.Progress),
) (fonts.Result, error) {
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
	options.Set = s.FontSet()
	options.Progress = progress
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

// EnsureFontsCached downloads one catalog set into the client cache without
// touching the OpenMW directory. The font preview calls it when the chosen set
// has no files yet, so the sample shows the real font before Finish; the
// install that follows reads the same cache and needs no network. Per-file
// failures come back as warnings, not as an error.
func (s *Service) EnsureFontsCached(ctx context.Context, setID string,
	progress func(fonts.Progress),
) (fonts.Result, error) {
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
	options.Set = strings.TrimSpace(setID)
	if options.Set == "" {
		options.Set = s.FontSet()
	}
	options.Progress = progress
	if s.httpClient != nil {
		options.Client = s.httpClient
	}
	result, err := fonts.EnsureCached(ctx, options)
	if err != nil {
		s.LogError("cache fonts", err)
		return result, err
	}
	if len(result.Warnings) > 0 {
		s.LogWarnf("fonts preview: %s", strings.Join(result.Warnings, "; "))
	}
	return result, nil
}

// FontCacheDir is the folder the preview reads after a cache download. It is
// not the install target: Fonts/ stays untouched until the user applies the
// set.
func (s *Service) FontCacheDir() (string, error) {
	cacheDir, err := s.absoluteCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "fonts"), nil
}

// FontsState verifies the files of the selected set in the active OpenMW Fonts
// directory. Files of another set count as missing.
func (s *Service) FontsState() (fonts.State, error) {
	configDir, err := s.OpenMWConfigDir()
	if err != nil {
		return fonts.State{}, err
	}
	return fonts.InspectSet(filepath.Join(configDir, "Fonts"), s.FontSet())
}

// FontSet returns the configured font set. An empty or unknown font_set in
// config.json resolves to fonts.DefaultSet, so old configs keep working.
func (s *Service) FontSet() string {
	if s == nil {
		return fonts.DefaultSet
	}
	return fonts.ResolveSet(s.Config().FontSet).ID
}

// SetFontSet stores the chosen set. The files are installed by InstallFonts;
// until then FontsState reports the new set as missing.
func (s *Service) SetFontSet(id string) error {
	if s == nil {
		return errors.New("app service is nil")
	}
	set, ok := fonts.LookupSet(id)
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownFontSet, strings.TrimSpace(id))
	}
	if err := s.store.Update(func(cfg *config.Config) error {
		cfg.FontSet = set.ID
		return nil
	}); err != nil {
		return err
	}
	s.LogInfof("font set: %s", set.ID)
	return nil
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
