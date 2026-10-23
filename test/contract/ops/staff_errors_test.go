package ops

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Saagilat/tes3mp-easy/test/fixtures"
)

func TestStaffSetErrors(t *testing.T) {
	root := staffContractRoot(t)
	tests := []struct {
		name string
		args []string
		exit int
		code string
	}{
		{"missing player", []string{"staff", "set", "--player", "Nobody", "--rank", "1"}, 3, "not_found"},
		{"rank too high", []string{"staff", "set", "--player", "John Doe", "--rank", "4"}, 2, "invalid_argument"},
		{"rank negative", []string{"staff", "set", "--player", "John Doe", "--rank", "-1"}, 2, "invalid_argument"},
		{"rank missing", []string{"staff", "set", "--player", "John Doe"}, 2, "invalid_argument"},
		{"player missing flag", []string{"staff", "set", "--rank", "1"}, 2, "invalid_argument"},
		{"player empty", []string{"staff", "set", "--player=", "--rank", "1"}, 2, "invalid_argument"},
		{"player traversal", []string{"staff", "set", "--player", "../evil", "--rank", "1"}, 8, "unsafe_path"},
		{"player separator", []string{"staff", "set", "--player", "John/Doe", "--rank", "1"}, 8, "unsafe_path"},
		{"extra argument", []string{"staff", "set", "--player", "John Doe", "--rank", "1", "extra"}, 2, "invalid_argument"},
		{"missing subcommand", []string{"staff"}, 2, "invalid_argument"},
		{"unknown subcommand", []string{"staff", "delete"}, 2, "invalid_argument"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"--json"}, tt.args...)
			res := testHarness.RunRoot(t, root, args...)
			res.requireJSONError(t, tt.exit, tt.code)
		})
	}
}

func TestStaffSetUnsafeAndInvalidFiles(t *testing.T) {
	root := staffContractRoot(t)
	players := filepath.Join(root, "players")
	if err := os.Symlink(filepath.Join(players, "John Doe.json"),
		filepath.Join(players, "Linked.json")); err != nil {
		t.Fatalf("symlink player: %v", err)
	}
	if err := os.Mkdir(filepath.Join(players, "Directory.json"), 0o700); err != nil {
		t.Fatalf("mkdir player directory: %v", err)
	}
	fixtures.WriteFile(t, filepath.Join(players, "Broken.json"), "{not json}\n")
	fixtures.WriteFile(t, filepath.Join(players, "Settings.json"),
		`{"settings": "not an object"}`)
	fixtures.WriteFile(t, filepath.Join(players, "Array.json"), `[]`)

	tests := []struct {
		player string
		exit   int
		code   string
	}{
		{"Linked", 8, "unsafe_path"},
		{"Directory", 8, "unsafe_path"},
		{"Broken", 9, "config_invalid"},
		{"Settings", 9, "config_invalid"},
		{"Array", 9, "config_invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.player, func(t *testing.T) {
			res := testHarness.RunRoot(t, root, "--json", "staff", "set",
				"--player", tt.player, "--rank", "1")
			res.requireJSONError(t, tt.exit, tt.code)
		})
	}
}

func TestStaffSetCreatesSettingsForEmptyObject(t *testing.T) {
	root := staffContractRoot(t)
	path := filepath.Join(root, "players", "Fresh.json")
	fixtures.WriteFile(t, path, "{}\n")

	res := testHarness.RunRoot(t, root, "--json", "staff", "set",
		"--player", "Fresh", "--rank", "1")
	_, data := decodeStaffSet(t, res)
	if data.Rank != 1 {
		t.Fatalf("rank = %d, want 1", data.Rank)
	}
	settings := readPlayerSettings(t, path)
	if !reflect.DeepEqual(settings, map[string]any{"staffRank": float64(1)}) {
		t.Fatalf("settings = %#v, want staffRank only", settings)
	}
}
