package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Saagilat/tes3mp-easy/test/fixtures"
)

func TestConfigArgumentErrors(t *testing.T) {
	root := configContractRoot(t)
	tests := []struct {
		name string
		args []string
	}{
		{"get without file", []string{"config", "get"}},
		{"get unknown file", []string{"config", "get", "--file", "other"}},
		{"get extra argument", []string{"config", "get", "--file", "config", "extra"}},
		{"set without file", []string{"config", "set", "--key", "loginTime", "--value", "45"}},
		{"set without key", []string{"config", "set", "--file", "config", "--value", "45"}},
		{"set without value", []string{"config", "set", "--file", "config", "--key", "loginTime"}},
		{"missing subcommand", []string{"config"}},
		{"unknown subcommand", []string{"config", "delete"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"--json"}, tt.args...)
			res := testHarness.RunRoot(t, root, args...)
			res.requireJSONError(t, 2, "invalid_argument")
		})
	}
}

func TestConfigGetMissingFile(t *testing.T) {
	root := t.TempDir()
	res := testHarness.RunRoot(t, root, "--json",
		"config", "get", "--file", "config")
	res.requireJSONError(t, 3, "not_found")
}

func TestConfigValidationErrors(t *testing.T) {
	root := configContractRoot(t)
	tests := []struct {
		name     string
		file     string
		key      string
		value    string
		wantFile string
		wantKey  string
	}{
		{"port below range", "server", "port", "80", "server", "port"},
		{"port above range", "server", "port", "70000", "server", "port"},
		{"unknown server key", "server", "localAddress", "127.0.0.1", "server", "localAddress"},
		{"login time below range", "config", "loginTime", "5", "config", "loginTime"},
		{"login time not integer", "config", "loginTime", "soon", "config", "loginTime"},
		{"unknown config key", "config", "notAKey", "x", "config", "notAKey"},
		{"invalid IP", "banlist", "ipAddresses", `["not-an-ip"]`, "banlist", "ipAddresses"},
		{"unknown banlist key", "banlist", "banned", "x", "banlist", "banned"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := testHarness.RunRoot(t, root, "--json",
				"config", "set", "--file", tt.file, "--key", tt.key,
				"--value", tt.value)
			env := res.requireJSONError(t, 9, "config_invalid")
			if env.Error.Details == nil {
				t.Fatal("error.details is missing")
			}
			if env.Error.Details["file"] != tt.wantFile {
				t.Fatalf("details.file = %#v, want %q",
					env.Error.Details["file"], tt.wantFile)
			}
			if env.Error.Details["key"] != tt.wantKey {
				t.Fatalf("details.key = %#v, want %q",
					env.Error.Details["key"], tt.wantKey)
			}
		})
	}
}

func TestConfigSetTableIsReadOnly(t *testing.T) {
	root := configContractRoot(t)
	path := filepath.Join(root, "configs", "config.lua")
	body := fixtures.DefaultConfigLua().Body + "config.gameSettings = { difficulty = 0 }\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config.lua: %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config.lua: %v", err)
	}

	res := testHarness.RunRoot(t, root, "--json",
		"config", "set", "--file", "config", "--key", "gameSettings", "--value", "x")
	res.requireJSONError(t, 9, "config_invalid")
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatalf("read-only rejection changed config.lua: %v", err)
	}
}

func TestConfigSetRejectsSymlink(t *testing.T) {
	root := configContractRoot(t)
	path := filepath.Join(root, "configs", "config.lua")
	target := filepath.Join(root, "configs", "target.lua")
	if err := os.Rename(path, target); err != nil {
		t.Fatalf("rename config.lua: %v", err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("symlink config.lua: %v", err)
	}
	res := testHarness.RunRoot(t, root, "--json",
		"config", "get", "--file", "config")
	res.requireJSONError(t, 9, "config_invalid")
	res = testHarness.RunRoot(t, root, "--json",
		"config", "set", "--file", "config", "--key", "loginTime", "--value", "45")
	res.requireJSONError(t, 9, "config_invalid")
}

func TestConfigHumanOutput(t *testing.T) {
	root := configContractRoot(t)
	res := testHarness.RunRoot(t, root, "config", "get", "--file", "config")
	res.requireExit(t, 0)
	if res.Stderr != "" {
		t.Fatalf("stderr = %q, want empty", res.Stderr)
	}
	for _, want := range []string{"file: config\n", "  loginTime=60\n", "read_only: -"} {
		if !strings.Contains(res.Stdout, want) {
			t.Fatalf("stdout = %q, want substring %q", res.Stdout, want)
		}
	}

	res = testHarness.RunRoot(t, root, "config", "set", "--file",
		"config", "--key", "loginTime", "--value", "45")
	res.requireExit(t, 0)
	if res.Stderr != "" {
		t.Fatalf("stderr = %q, want empty", res.Stderr)
	}
	want := "file: config\nkey: loginTime\nvalue: 45\nrestart_required: true\n"
	if res.Stdout != want {
		t.Fatalf("stdout = %q, want %q", res.Stdout, want)
	}
}
