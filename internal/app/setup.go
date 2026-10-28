package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/config"
	"github.com/Saagilat/tes3mp-easy/internal/morrowind"
	"github.com/Saagilat/tes3mp-easy/internal/tes3mp"
)

// DetectMorrowind checks the configured folder first, then the standard
// Windows/Linux locations. A successful path is saved in config.json.
func (s *Service) DetectMorrowind() (morrowind.Installation, error) {
	if s == nil {
		return morrowind.Installation{}, errors.New("app service is nil")
	}
	installation, err := morrowind.Discover(morrowind.Options{
		Root:   s.Config().MorrowindPath,
		Getenv: s.getenv,
	})
	if err != nil {
		return morrowind.Installation{}, err
	}
	if err := s.store.Update(func(cfg *config.Config) error {
		cfg.MorrowindPath = installation.Root
		return nil
	}); err != nil {
		return morrowind.Installation{}, fmt.Errorf(
			"save Morrowind folder: %w", err)
	}
	return installation, nil
}

// CheckMorrowind verifies an explicitly selected folder. An empty path uses
// the configured folder. It does not modify config.json.
func (s *Service) CheckMorrowind(path string) (morrowind.Installation, error) {
	if s == nil {
		return morrowind.Installation{}, errors.New("app service is nil")
	}
	path = strings.TrimSpace(path)
	if path == "" {
		path = s.Config().MorrowindPath
	}
	if path == "" {
		return morrowind.Installation{}, ErrMorrowindNotConfigured
	}
	return morrowind.Inspect(path)
}

// TES3MPState reports whether the pinned TES3MP release is installed in the
// effective work directory.
func (s *Service) TES3MPState() (tes3mp.State, error) {
	if s == nil {
		return tes3mp.State{}, errors.New("app service is nil")
	}
	workDir, err := s.absoluteWorkDir()
	if err != nil {
		return tes3mp.State{}, err
	}
	return tes3mp.Inspect(tes3mp.Dir(workDir))
}

// InstallTES3MP downloads and installs the pinned release. The method is safe
// to call from a UI goroutine; progress is delivered through the callback.
func (s *Service) InstallTES3MP(ctx context.Context,
	progress func(tes3mp.Progress),
) (tes3mp.Result, error) {
	if s == nil {
		return tes3mp.Result{}, errors.New("app service is nil")
	}
	workDir, err := s.absoluteWorkDir()
	if err != nil {
		return tes3mp.Result{}, err
	}
	cacheDir := strings.TrimSpace(s.paths.CacheDir)
	if cacheDir == "" {
		return tes3mp.Result{}, errors.New("client cache directory is empty")
	}
	if !filepath.IsAbs(cacheDir) {
		return tes3mp.Result{}, fmt.Errorf("client cache directory %q is not absolute",
			cacheDir)
	}
	options := tes3mp.DefaultOptions(tes3mp.Dir(workDir), filepath.Clean(cacheDir))
	options.Progress = progress
	var result tes3mp.Result
	if s.installTES3MP != nil {
		result, err = s.installTES3MP(ctx, options)
	} else {
		result, err = tes3mp.Install(ctx, options)
	}
	if err != nil {
		s.LogError("install TES3MP", err)
		return result, err
	}
	s.LogInfof("TES3MP %s is ready (skipped=%t, downloaded=%t)",
		result.State.Version, result.Skipped, result.Downloaded)
	return result, nil
}

func (s *Service) absoluteWorkDir() (string, error) {
	workDir := strings.TrimSpace(s.WorkDir())
	if workDir == "" {
		return "", WrapWorkDir(errors.New("client work directory is empty"))
	}
	if !filepath.IsAbs(workDir) {
		return "", WrapWorkDir(fmt.Errorf(
			"client work directory %q is not absolute", workDir))
	}
	return filepath.Clean(workDir), nil
}
