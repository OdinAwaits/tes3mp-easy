package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Paths are the per-user locations used by the client. ConfigFile may be
// overridden on the command line; the rest stay tied to the operating system.
type Paths struct {
	ConfigDir  string
	ConfigFile string
	DataDir    string
	ServersDir string
	CacheDir   string
	LogsDir    string
}

// PathsFor is the testable form of the platform defaults. On Windows it follows
// %APPDATA% for config and %LOCALAPPDATA% for data. Linux and other Unix-like
// systems follow XDG_CONFIG_HOME/XDG_DATA_HOME and the documented home
// fallbacks.
func PathsFor(goos string, getenv func(string) string) (Paths, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	home := strings.TrimSpace(getenv("HOME"))
	if goos == "windows" {
		home = strings.TrimSpace(getenv("USERPROFILE"))
	}
	if home == "" {
		return Paths{}, errors.New("cannot determine the user home directory")
	}

	var configBase, dataBase string
	if goos == "windows" {
		configBase = strings.TrimSpace(getenv("APPDATA"))
		dataBase = strings.TrimSpace(getenv("LOCALAPPDATA"))
		if configBase == "" {
			configBase = filepath.Join(home, "AppData", "Roaming")
		}
		if dataBase == "" {
			dataBase = filepath.Join(home, "AppData", "Local")
		}
	} else {
		configBase = strings.TrimSpace(getenv("XDG_CONFIG_HOME"))
		if configBase == "" {
			configBase = filepath.Join(home, ".config")
		}
		dataBase = strings.TrimSpace(getenv("XDG_DATA_HOME"))
		if dataBase == "" {
			dataBase = filepath.Join(home, ".local", "share")
		}
	}

	configDir := filepath.Join(configBase, "tes3mp-easy")
	dataDir := filepath.Join(dataBase, "tes3mp-easy")
	return Paths{
		ConfigDir:  configDir,
		ConfigFile: filepath.Join(configDir, "config.json"),
		DataDir:    dataDir,
		ServersDir: filepath.Join(dataDir, "servers"),
		CacheDir:   filepath.Join(dataDir, "cache"),
		LogsDir:    filepath.Join(dataDir, "logs"),
	}, nil
}

// Validate checks that all required paths are absolute.
func (p Paths) Validate() error {
	required := []struct {
		name  string
		value string
	}{
		{name: "config directory", value: p.ConfigDir},
		{name: "config file", value: p.ConfigFile},
		{name: "data directory", value: p.DataDir},
		{name: "servers directory", value: p.ServersDir},
		{name: "cache directory", value: p.CacheDir},
		{name: "logs directory", value: p.LogsDir},
	}
	for _, item := range required {
		if item.value == "" {
			return fmt.Errorf("%s is empty", item.name)
		}
		if !filepath.IsAbs(item.value) {
			return fmt.Errorf("%s %q is not absolute", item.name, item.value)
		}
	}
	return nil
}

// ServerDir returns the profile directory for a normalized server ID.
func (p Paths) ServerDir(id string) string {
	return filepath.Join(p.ServersDir, id)
}
