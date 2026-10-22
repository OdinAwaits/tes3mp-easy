package fixtures

import (
	"path/filepath"
	"testing"
)

// ConfigFile is one synthetic TES3MP configuration file.
type ConfigFile struct {
	Name string
	Body string
}

// DefaultConfigs returns the three files from docs/v2/07-testing.md §3.
func DefaultConfigs() []ConfigFile {
	return []ConfigFile{
		DefaultServerConfig(),
		DefaultConfigLua(),
		DefaultBanlist(),
	}
}

// DefaultServerConfig returns a minimal synthetic tes3mp-server-default.cfg.
func DefaultServerConfig() ConfigFile {
	return ConfigFile{
		Name: "tes3mp-server-default.cfg",
		Body: `[General]
# Synthetic test configuration; no production values.
localAddress = 0.0.0.0
port = 25565
maximumPlayers = 8
hostname = Morrowind with friends
logLevel = 1
password =

[Plugins]
home = ./server
plugins = serverCore.lua

[MasterServer]
enabled = false
`,
	}
}

// DefaultConfigLua returns a minimal synthetic config.lua with the editable
// scalar keys used by the future config tests.
func DefaultConfigLua() ConfigFile {
	return ConfigFile{
		Name: "config.lua",
		Body: `config = {}

-- Synthetic test configuration; tables stay read-only.
config.gameMode = "Default"
config.loginTime = 60
config.maxClientsPerIP = 3
config.difficulty = 0
config.passTimeWhenEmpty = false
config.nightStartHour = 20
config.nightEndHour = 6
config.allowConsole = false
config.allowBedRest = true
config.allowWildernessRest = true
config.allowWait = true
config.allowSuicideCommand = true
config.allowFixmeCommand = true
config.shareJournal = true
config.shareFactionRanks = true
config.shareTopics = true
config.shareBounty = false
config.shareReputation = true
config.enforceDataFiles = true
`,
	}
}

// DefaultBanlist returns an empty ban list in the canonical shape.
func DefaultBanlist() ConfigFile {
	return ConfigFile{
		Name: "banlist.json",
		Body: `{
  "playerNames": [],
  "ipAddresses": []
}
`,
	}
}

// Write writes the config file below dir.
func (f ConfigFile) Write(t testing.TB, dir string) {
	t.Helper()
	WriteFile(t, filepath.Join(dir, f.Name), f.Body)
}

// WriteConfigs writes all default config files below dir.
func WriteConfigs(t testing.TB, dir string) {
	t.Helper()
	for _, file := range DefaultConfigs() {
		file.Write(t, dir)
	}
}
