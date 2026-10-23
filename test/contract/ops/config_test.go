package ops

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Saagilat/tes3mp-easy/test/fixtures"
)

type configGetPayload struct {
	File     string         `json:"file"`
	Values   map[string]any `json:"values"`
	ReadOnly []string       `json:"read_only"`
}

type configSetPayload struct {
	File            string `json:"file"`
	Key             string `json:"key"`
	Value           any    `json:"value"`
	RestartRequired bool   `json:"restart_required"`
}

func configContractRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "configs")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir configs: %v", err)
	}
	fixtures.WriteConfigs(t, dir)
	return root
}

func decodeConfigGet(t *testing.T, res result) (envelope, configGetPayload) {
	t.Helper()
	env := requireReadSuccess(t, res, []string{"file", "read_only", "values"})
	var data configGetPayload
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("decode config get payload: %v", err)
	}
	return env, data
}

func decodeConfigSet(t *testing.T, res result) (envelope, configSetPayload) {
	t.Helper()
	env := requireReadSuccess(t, res,
		[]string{"file", "key", "restart_required", "value"})
	var data configSetPayload
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("decode config set payload: %v", err)
	}
	return env, data
}

func TestConfigGetReadOnlyTables(t *testing.T) {
	root := configContractRoot(t)
	path := filepath.Join(root, "configs", "config.lua")
	body := fixtures.DefaultConfigLua().Body + `
config.gameSettings = {
  difficulty = 0,
}
config.someList = { "a", "b" }
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config.lua: %v", err)
	}

	res := testHarness.RunRoot(t, root, "--json",
		"config", "get", "--file", "config")
	_, data := decodeConfigGet(t, res)
	want := []string{"gameSettings", "someList"}
	if !reflect.DeepEqual(data.ReadOnly, want) {
		t.Fatalf("read_only = %v, want %v", data.ReadOnly, want)
	}
	if _, ok := data.Values["gameSettings"]; ok {
		t.Fatal("table key must not appear in editable values")
	}
}

func TestConfigSetDryRunChangesNothing(t *testing.T) {
	root := configContractRoot(t)
	path := filepath.Join(root, "configs", "config.lua")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config.lua: %v", err)
	}

	res := testHarness.RunRoot(t, root, "--json", "--dry-run",
		"config", "set", "--file", "config", "--key", "loginTime", "--value", "45")
	_, data := decodeConfigSet(t, res)
	if data.Value != float64(45) || !data.RestartRequired {
		t.Fatalf("dry-run payload = %#v", data)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatalf("dry-run changed config.lua: %v", err)
	}
	backups, err := filepath.Glob(filepath.Join(root, "configs", "config.lua.bak-*"))
	if err != nil || len(backups) != 0 {
		t.Fatalf("dry-run backups = %v, %v; want none", backups, err)
	}
}

func TestConfigGetContract(t *testing.T) {
	root := configContractRoot(t)

	tests := []struct {
		file string
		want map[string]any
	}{
		{
			file: "server",
			want: map[string]any{
				"hostname":            "Morrowind with friends",
				"port":                float64(25565),
				"maximumPlayers":      float64(8),
				"password":            "",
				"logLevel":            float64(1),
				"masterServerEnabled": false,
			},
		},
		{
			file: "config",
			want: map[string]any{
				"gameMode":            "Default",
				"loginTime":           float64(60),
				"maxClientsPerIP":     float64(3),
				"difficulty":          float64(0),
				"passTimeWhenEmpty":   false,
				"nightStartHour":      float64(20),
				"nightEndHour":        float64(6),
				"allowConsole":        false,
				"allowBedRest":        true,
				"allowWildernessRest": true,
				"allowWait":           true,
				"allowSuicideCommand": true,
				"allowFixmeCommand":   true,
				"shareJournal":        true,
				"shareFactionRanks":   true,
				"shareTopics":         true,
				"shareBounty":         false,
				"shareReputation":     true,
				"enforceDataFiles":    true,
			},
		},
		{
			file: "banlist",
			want: map[string]any{
				"playerNames": []any{},
				"ipAddresses": []any{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			res := testHarness.RunRoot(t, root, "--json",
				"config", "get", "--file", tt.file)
			_, data := decodeConfigGet(t, res)
			if data.File != tt.file {
				t.Fatalf("file = %q, want %q", data.File, tt.file)
			}
			if !reflect.DeepEqual(data.Values, tt.want) {
				t.Fatalf("values = %#v, want %#v", data.Values, tt.want)
			}
			if data.ReadOnly == nil || len(data.ReadOnly) != 0 {
				t.Fatalf("read_only = %#v, want []", data.ReadOnly)
			}
		})
	}
}

func TestConfigSetContract(t *testing.T) {
	root := configContractRoot(t)

	res := testHarness.RunRoot(t, root, "--json",
		"config", "set", "--file", "config", "--key", "loginTime", "--value", "45")
	_, data := decodeConfigSet(t, res)
	if data.File != "config" || data.Key != "loginTime" || data.Value != float64(45) ||
		!data.RestartRequired {
		t.Fatalf("config set payload = %#v", data)
	}
	updated, err := os.ReadFile(filepath.Join(root, "configs", "config.lua"))
	if err != nil {
		t.Fatalf("read config.lua: %v", err)
	}
	if !strings.Contains(string(updated), "config.loginTime = 45") {
		t.Fatalf("config.lua has no new loginTime:\n%s", updated)
	}
	if !strings.Contains(string(updated), "Synthetic test configuration") {
		t.Fatal("comment was lost after config set")
	}
	backups, err := filepath.Glob(filepath.Join(root, "configs", "config.lua.bak-*"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("config backups = %v, %v; want one", backups, err)
	}

	res = testHarness.RunRoot(t, root, "--json",
		"config", "set", "--file", "server", "--key", "port", "--value", "25566")
	_, data = decodeConfigSet(t, res)
	if data.Value != float64(25566) {
		t.Fatalf("server port value = %#v", data.Value)
	}
	res = testHarness.RunRoot(t, root, "--json",
		"config", "get", "--file", "server")
	_, snapshot := decodeConfigGet(t, res)
	if snapshot.Values["port"] != float64(25566) {
		t.Fatalf("server port after set = %#v", snapshot.Values["port"])
	}

	res = testHarness.RunRoot(t, root, "--json",
		"config", "set", "--file", "banlist", "--key", "ipAddresses",
		"--value", `["203.0.113.5"]`)
	_, data = decodeConfigSet(t, res)
	wantIPs := []any{"203.0.113.5"}
	if !reflect.DeepEqual(data.Value, wantIPs) {
		t.Fatalf("banlist value = %#v, want %#v", data.Value, wantIPs)
	}
	res = testHarness.RunRoot(t, root, "--json",
		"config", "get", "--file", "banlist")
	_, snapshot = decodeConfigGet(t, res)
	if !reflect.DeepEqual(snapshot.Values["ipAddresses"], wantIPs) {
		t.Fatalf("banlist after set = %#v", snapshot.Values)
	}
}
