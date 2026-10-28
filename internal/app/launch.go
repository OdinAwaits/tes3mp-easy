package app

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/Saagilat/tes3mp-easy/internal/launcher"
	"github.com/Saagilat/tes3mp-easy/internal/locale"
	"github.com/Saagilat/tes3mp-easy/internal/mods"
)

// launchGuard serializes launches so a repeated Play click cannot start a
// second TES3MP instance. Both the game and OpenMW-CS share one work directory
// and one OpenMW config, so only one of them may start at a time.
type launchGuard struct {
	mu       sync.Mutex
	starting bool
	running  bool
	serverID string
}

// begin reserves the launch slot. It fails with ErrLaunchInFlight while another
// launch prepares files and with ErrGameRunning while a game we started is
// still running.
func (g *launchGuard) begin(serverID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	switch {
	case g.starting:
		return ErrLaunchInFlight
	case g.running:
		return ErrGameRunning
	}
	g.starting = true
	g.serverID = serverID
	return nil
}

// finish releases the slot. started reports whether a game process was spawned
// and therefore keeps the running latch until stop is called.
func (g *launchGuard) finish(started bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.starting = false
	if started {
		g.running = true
		return
	}
	g.running = false
	g.serverID = ""
}

// stop releases the running latch after the game process exited.
func (g *launchGuard) stop() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.running = false
	g.serverID = ""
}

// state returns the profile of the last accepted launch together with the
// in-flight and running flags.
func (g *launchGuard) state() (string, bool, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.serverID, g.starting, g.running
}

// GameRunning reports whether a game started by this service has not exited
// yet. The UI disables Play while it runs because a second client would use the
// same profile and the same OpenMW configuration.
func (s *Service) GameRunning() (string, bool) {
	if s == nil {
		return "", false
	}
	serverID, _, running := s.launchGuard.state()
	return serverID, running
}

// GameStopped releases the running latch after the game process exited.
func (s *Service) GameStopped() {
	if s == nil {
		return
	}
	s.launchGuard.stop()
}

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
	if err := s.launchGuard.begin(serverID); err != nil {
		s.LogWarnf("launch %s rejected: %v", serverID, err)
		return launcher.Result{}, err
	}
	started := false
	defer func() { s.launchGuard.finish(started) }()
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

	fontResult, fontErr := s.InstallFonts(ctx)
	warnings = append(warnings, fontResult.Warnings...)
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
	started = result.Process != nil
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
	if err := s.launchGuard.begin(serverID); err != nil {
		s.LogWarnf("openmw-cs %s rejected: %v", serverID, err)
		return launcher.Result{}, err
	}
	defer s.launchGuard.finish(false)
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
