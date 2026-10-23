package ops

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Saagilat/tes3mp-easy/test/fixtures"
)

func TestHelpContract(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "global", args: []string{"--help"}},
		{name: "after command", args: []string{"status", "--help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := testHarness.Run(t, tt.args...)
			res.requireExit(t, 0)
			if res.Stderr != "" {
				t.Fatalf("stderr = %q, want empty", res.Stderr)
			}
			if !strings.Contains(res.Stdout, "Usage: tes3mp-easy-ops") {
				t.Fatalf("stdout = %q, want usage", res.Stdout)
			}
		})
	}
}

func TestJSONErrorContract(t *testing.T) {
	tests := []struct {
		name string
		args []string
		exit int
		code string
	}{
		{
			name: "missing command",
			args: []string{"--json"},
			exit: 2,
			code: "invalid_argument",
		},
		{
			name: "unknown flag",
			args: []string{"--json", "--unknown"},
			exit: 2,
			code: "invalid_argument",
		},
		{
			name: "unknown command",
			args: []string{"--json", "bogus"},
			exit: 2,
			code: "invalid_argument",
		},
		{
			name: "empty root",
			args: []string{"--json", "--root="},
			exit: 2,
			code: "invalid_argument",
		},
		{
			name: "missing root value",
			args: []string{"--json", "--root"},
			exit: 2,
			code: "invalid_argument",
		},
		{
			name: "invalid lock timeout",
			args: []string{"--json", "--lock-timeout=-1s", "status"},
			exit: 2,
			code: "invalid_argument",
		},
		{
			name: "invalid command timeout",
			args: []string{"--json", "--command-timeout=bad", "status"},
			exit: 2,
			code: "invalid_argument",
		},
		{
			name: "known command is unfinished",
			args: []string{"--json", "start"},
			exit: 1,
			code: "internal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := testHarness.Run(t, tt.args...)
			res.requireJSONError(t, tt.exit, tt.code)
		})
	}
}

func TestHumanErrorContract(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		exit    int
		message string
	}{
		{name: "missing command", exit: 2, message: "command is required"},
		{name: "unknown flag", args: []string{"--unknown"}, exit: 2, message: "unknown flag"},
		{name: "unknown command", args: []string{"bogus"}, exit: 2, message: "unknown command"},
		{name: "unfinished", args: []string{"start"}, exit: 1, message: "not implemented"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := testHarness.Run(t, tt.args...)
			res.requireHumanError(t, tt.exit, tt.message)
		})
	}
}

func TestJSONOutputIsOneEnvelope(t *testing.T) {
	res := testHarness.Run(t, "--json", "start")
	env := res.requireJSONError(t, 1, "internal")

	if len(env.Data) != 0 {
		t.Fatalf("data = %s, want absent", env.Data)
	}
	if env.Error.Details == nil {
		return
	}
	if _, ok := env.Error.Details["backend"]; ok {
		t.Fatal("error.details contains forbidden backend field")
	}
}

func TestRootFixtureIsReadOnly(t *testing.T) {
	root := t.TempDir()
	fixtures.WriteConfigs(t, filepath.Join(root, "configs"))

	modsDir := filepath.Join(root, "backups", "mods")
	manifest := fixtures.DefaultMods().Install(t, modsDir)

	state := fixtures.FullState(manifest)
	state.Write(t, filepath.Join(
		root, "backups", "state", state.ArchiveName()))

	fixtures.JSONFile(t, filepath.Join(root, "api", "server.json"),
		fixtures.ServerWithMods(manifest))
	fixtures.JSONFile(t, filepath.Join(root, "api", "mods-current.json"),
		fixtures.DefaultModsCurrent(manifest, 123))
	fixtures.JSONFile(t, filepath.Join(root, "api", "errors.json"),
		fixtures.DefaultAPIErrors())

	before := snapshotTree(t, root)
	if len(before) < 10 {
		t.Fatalf("fixture root has %d entries, want at least 10", len(before))
	}

	res := testHarness.RunRoot(t, root, "--json", "status")
	if res.ExitCode != 0 {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q",
			res.ExitCode, res.Stdout, res.Stderr)
	}

	after := snapshotTree(t, root)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("command changed temp root\nbefore: %#v\nafter:  %#v", before, after)
	}
}

func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()

	snapshot := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if entry.IsDir() {
			snapshot[rel] = "dir"
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		snapshot[rel] = fixtures.SHA256Hex(data)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return snapshot
}
