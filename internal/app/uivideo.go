package app

import (
	"path/filepath"

	"github.com/Saagilat/tes3mp-easy/internal/openmwcfg"
)

// ClientSettingsPath returns the settings.cfg path inside the active OpenMW
// configuration directory. OpenMW creates the file on the first game launch,
// so a missing file is a normal state.
func (s *Service) ClientSettingsPath() (string, error) {
	configDir, err := s.OpenMWConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, openmwcfg.FileName), nil
}

// ClientSettings reads the UI and video values the launcher manages.
func (s *Service) ClientSettings() (openmwcfg.State, error) {
	path, err := s.ClientSettingsPath()
	if err != nil {
		return openmwcfg.State{}, err
	}
	state, err := openmwcfg.Load(path)
	if err != nil {
		s.LogError("read settings.cfg", err)
		return openmwcfg.State{}, err
	}
	return state, nil
}

// UpdateClientSettings writes the four launcher-owned keys and keeps every
// other line of settings.cfg, including comments and the line ending style.
func (s *Service) UpdateClientSettings(
	values openmwcfg.Settings,
) (openmwcfg.State, error) {
	path, err := s.ClientSettingsPath()
	if err != nil {
		return openmwcfg.State{}, err
	}
	state, err := openmwcfg.Update(path, values)
	if err != nil {
		s.LogError("update settings.cfg", err)
		return openmwcfg.State{}, err
	}
	s.LogInfof("settings.cfg updated: ttf=%d font=%d scaling=%g fps=%d",
		values.TTFResolution, values.FontSize, values.ScalingFactor,
		values.FramerateLimit)
	return state, nil
}
