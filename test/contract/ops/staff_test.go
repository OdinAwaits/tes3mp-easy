package ops

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Saagilat/tes3mp-easy/test/fixtures"
)

type staffSetPayload struct {
	Player          string `json:"player"`
	Rank            int    `json:"rank"`
	RestartRequired bool   `json:"restart_required"`
}

func staffContractRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "players")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir players: %v", err)
	}
	fixtures.WriteFile(t, filepath.Join(dir, "John Doe.json"), `{
  "name": "John Doe",
  "settings": {
    "staffRank": 0,
    "other": "keep"
  }
}
`)
	return root
}

func decodeStaffSet(t *testing.T, res result) (envelope, staffSetPayload) {
	t.Helper()
	env := requireReadSuccess(t, res,
		[]string{"player", "rank", "restart_required"})
	var data staffSetPayload
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("decode staff set payload: %v", err)
	}
	return env, data
}

func readPlayerSettings(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read player: %v", err)
	}
	var top map[string]any
	if err := json.Unmarshal(data, &top); err != nil {
		t.Fatalf("decode player JSON: %v", err)
	}
	settings, ok := top["settings"].(map[string]any)
	if !ok {
		t.Fatalf("settings = %#v, want object", top["settings"])
	}
	return settings
}

func TestStaffSetContract(t *testing.T) {
	root := staffContractRoot(t)
	path := filepath.Join(root, "players", "John Doe.json")

	res := testHarness.RunRoot(t, root, "--json",
		"staff", "set", "--player", "John Doe", "--rank", "2")
	_, data := decodeStaffSet(t, res)
	if data.Player != "John Doe" || data.Rank != 2 || !data.RestartRequired {
		t.Fatalf("staff set payload = %#v", data)
	}

	settings := readPlayerSettings(t, path)
	if settings["staffRank"] != float64(2) {
		t.Fatalf("staffRank = %#v, want 2", settings["staffRank"])
	}
	if settings["other"] != "keep" {
		t.Fatalf("unrelated setting = %#v, want keep", settings["other"])
	}
	backups, err := filepath.Glob(path + ".bak-*")
	if err != nil || len(backups) != 1 {
		t.Fatalf("staff backups = %v, %v; want one", backups, err)
	}
	backup, err := os.ReadFile(backups[0])
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	var top map[string]any
	if err := json.Unmarshal(backup, &top); err != nil {
		t.Fatalf("decode backup: %v", err)
	}
	oldSettings := top["settings"].(map[string]any)
	if oldSettings["staffRank"] != float64(0) {
		t.Fatalf("backup staffRank = %#v, want 0", oldSettings["staffRank"])
	}

	res = testHarness.RunRoot(t, root,
		"staff", "set", "--player", "John Doe", "--rank", "0")
	res.requireExit(t, 0)
	if res.Stderr != "" {
		t.Fatalf("stderr = %q, want empty", res.Stderr)
	}
	want := "player: John Doe\nrank: 0\nrestart_required: true\n"
	if res.Stdout != want {
		t.Fatalf("stdout = %q, want %q", res.Stdout, want)
	}
}

func TestStaffSetDryRunChangesNothing(t *testing.T) {
	root := staffContractRoot(t)
	path := filepath.Join(root, "players", "John Doe.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read player: %v", err)
	}

	res := testHarness.RunRoot(t, root, "--json", "--dry-run",
		"staff", "set", "--player", "John Doe", "--rank", "3")
	_, data := decodeStaffSet(t, res)
	if data.Rank != 3 {
		t.Fatalf("rank = %d, want 3", data.Rank)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatalf("dry-run changed player JSON: %v", err)
	}
	backups, err := filepath.Glob(path + ".bak-*")
	if err != nil || len(backups) != 0 {
		t.Fatalf("dry-run backups = %v, %v; want none", backups, err)
	}
}
