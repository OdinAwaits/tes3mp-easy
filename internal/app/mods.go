package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/config"
	"github.com/Saagilat/tes3mp-easy/internal/locale"
	"github.com/Saagilat/tes3mp-easy/internal/mods"
	"github.com/Saagilat/tes3mp-easy/internal/serverapi"
)

// CheckServer reads the public panel status for a saved profile. It never sends
// the game password.
func (s *Service) CheckServer(ctx context.Context,
	id string,
) (serverapi.Server, error) {
	if s == nil {
		return serverapi.Server{}, errors.New("app service is nil")
	}
	profile, err := s.serverProfile(id)
	if err != nil {
		return serverapi.Server{}, err
	}
	client, err := s.serverAPIClient(profile)
	if err != nil {
		return serverapi.Server{}, err
	}
	return client.Server(ctx)
}

// ModsState reports installed mod metadata for one profile.
func (s *Service) ModsState(id string) (mods.State, error) {
	if s == nil {
		return mods.State{}, errors.New("app service is nil")
	}
	workDir, err := s.absoluteWorkDir()
	if err != nil {
		return mods.State{}, err
	}
	return mods.Inspect(workDir, strings.TrimSpace(id))
}

// UpdateMods downloads and installs the current mod pack for one profile. The
// method is safe to call from a UI goroutine; progress is delivered through the
// callback.
func (s *Service) UpdateMods(ctx context.Context, id string,
	progress func(mods.Progress),
) (mods.Result, error) {
	if s == nil {
		return mods.Result{}, errors.New("app service is nil")
	}
	profile, err := s.serverProfile(id)
	if err != nil {
		return mods.Result{}, err
	}
	installation, err := s.CheckMorrowind(s.Config().MorrowindPath)
	if err != nil {
		return mods.Result{}, err
	}
	workDir, err := s.absoluteWorkDir()
	if err != nil {
		return mods.Result{}, err
	}
	cacheDir := strings.TrimSpace(s.paths.CacheDir)
	if cacheDir == "" || !filepath.IsAbs(cacheDir) {
		return mods.Result{}, fmt.Errorf(
			"client cache directory %q is not absolute", cacheDir)
	}
	client, err := s.serverAPIClient(profile)
	if err != nil {
		return mods.Result{}, err
	}
	options := mods.Options{
		WorkDir:   workDir,
		CacheDir:  filepath.Clean(cacheDir),
		ServerID:  profile.ID,
		Morrowind: installation,
		Client:    client,
		Progress:  progress,
	}
	var result mods.Result
	if s.installMods != nil {
		result, err = s.installMods(ctx, options)
	} else {
		result, err = mods.Install(ctx, options)
	}
	if err != nil {
		s.LogError("update mods", err)
		return result, err
	}
	s.LogInfof("mods for %s are installed (skipped=%t, sha256=%s)",
		profile.ID, result.Skipped, result.State.SHA256)
	if result.Skipped {
		return result, nil
	}
	if !s.Config().Localization {
		return result, nil
	}
	state, stateErr := locale.Inspect(workDir, profile.ID)
	if stateErr != nil {
		return result, stateErr
	}
	if !state.Installed {
		return result, errors.New(
			"mods were updated but Russian localization is not installed")
	}
	applied, applyErr := locale.Apply(workDir, profile.ID)
	result.Warnings = append(result.Warnings, applied.Warnings...)
	if applyErr != nil {
		return result, fmt.Errorf("reapply localization overlay: %w", applyErr)
	}
	return result, nil
}

func (s *Service) serverProfile(id string) (config.Server, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return config.Server{}, errors.New("server id is empty")
	}
	for _, profile := range s.Config().Servers {
		if profile.ID == id {
			return profile, nil
		}
	}
	return config.Server{}, fmt.Errorf("profile %q not found", id)
}

func (s *Service) serverAPIClient(profile config.Server) (*serverapi.Client, error) {
	options := serverapi.Options{
		BaseURL:    profile.EffectivePanelURL(),
		HTTPClient: s.httpClient,
	}
	if s.newServerAPI != nil {
		return s.newServerAPI(options)
	}
	return serverapi.New(options)
}
