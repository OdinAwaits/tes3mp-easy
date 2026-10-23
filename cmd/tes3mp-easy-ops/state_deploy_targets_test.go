package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/systemd"
	"github.com/Saagilat/tes3mp-easy/test/fixtures"
)

func TestRunStateDeployTargetErrorsDoNotTouchService(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		exit   ops.ExitCode
		want   string
		before func(t *testing.T, root string)
	}{
		{
			name: "traversal",
			args: []string{"--archive", "../state-manual-2026-10-22_13-00-00.tar.gz"},
			exit: ops.ExitUnsafePath, want: "unsafe",
		},
		{
			name: "not a state archive",
			args: []string{"--archive", "mods-2026-10-22_13-00-00.tar.gz"},
			exit: ops.ExitArchiveInvalid, want: "state archive",
		},
		{
			name: "missing archive",
			args: []string{"--archive", "state-2026-10-23_00-00-00.tar.gz"},
			exit: ops.ExitNotFound, want: "does not exist",
		},
		{
			name: "no archives for latest", args: []string{"--latest"},
			exit: ops.ExitNotFound, want: "no state archive",
			before: func(t *testing.T, root string) {
				dir := filepath.Join(root, "backups", "state")
				entries, err := os.ReadDir(dir)
				if err != nil {
					t.Fatal(err)
				}
				for _, entry := range entries {
					if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil {
						t.Fatal(err)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixStateDeployHooks(t)
			root, _, _, _ := stateDeployRoot(t)
			if tt.before != nil {
				tt.before(t, root)
			}
			runner := activeRunner(t)

			code, _, stdout, stderr := runStateDeployCommand(t, root,
				systemd.New(runner), tt.args...)
			if code != int(tt.exit) {
				t.Fatalf("exit = %d, want %d; stdout=%q stderr=%q",
					code, tt.exit, stdout, stderr)
			}
			assertJSONError(t, stdout, tt.exit, tt.want)
			if calls := runner.Calls(); len(calls) != 0 {
				t.Fatalf("target error called systemd: %#v", calls)
			}
			assertStateDeployOldState(t, root)
			entries, err := os.ReadDir(filepath.Join(root, "backups", "state"))
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				parsed, err := archive.ParseArchiveName(entry.Name())
				if err == nil && parsed.Kind == archive.KindStatePredeploy {
					t.Fatalf("target error created %s", entry.Name())
				}
			}
			assertNoStateDeployLeftovers(t, root)
		})
	}
}

func TestRunStateDeployRejectsForeignOrMissingCurrent(t *testing.T) {
	tests := []struct {
		name  string
		state func(t *testing.T, root string,
			current fixtures.ModsManifest) (fixtures.StateArchive, string)
	}{
		{
			name: "foreign current",
			state: func(t *testing.T, root string,
				_ fixtures.ModsManifest,
			) (fixtures.StateArchive, string) {
				foreign := fixtures.DefaultMods()
				foreign.Name = "mods-2026-10-22_14-00-00.tar.gz"
				body := foreign.Bytes(t)
				fixtures.WriteFile(t,
					filepath.Join(root, "backups", "mods", foreign.Name),
					string(body))
				state := fixtures.FullState(foreign.Manifest(body))
				state.Name = "state-manual-2026-10-22_15-00-00.tar.gz"
				return state, state.Name
			},
		},
		{
			name: "missing current",
			state: func(t *testing.T, _ string,
				current fixtures.ModsManifest,
			) (fixtures.StateArchive, string) {
				state := fixtures.FullState(current)
				state.Name = "state-manual-2026-10-22_15-00-00.tar.gz"
				state.OmitCurrent = true
				return state, state.Name
			},
		},
		{
			name: "old current format",
			state: func(t *testing.T, _ string,
				current fixtures.ModsManifest,
			) (fixtures.StateArchive, string) {
				state := fixtures.FullState(current)
				state.Name = "state-manual-2026-10-22_15-00-00.tar.gz"
				state.OmitCurrent = true
				state.Extra = append(state.Extra, fixtures.Entry{
					Name: fixtures.CurrentName,
					Body: current.SHA256 + "\n",
				})
				return state, state.Name
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixStateDeployHooks(t)
			root, manifest, _, _ := stateDeployRoot(t)
			state, name := tt.state(t, root, manifest)
			state.Write(t, filepath.Join(root, "backups", "state", name))
			runner := activeRunner(t)

			code, _, stdout, stderr := runStateDeployCommand(t, root,
				systemd.New(runner), "--archive", name)
			if code != int(ops.ExitArchiveInvalid) {
				t.Fatalf("exit = %d, want %d; stdout=%q stderr=%q",
					code, ops.ExitArchiveInvalid, stdout, stderr)
			}
			assertJSONError(t, stdout, ops.ExitArchiveInvalid, "archive")
			if calls := runner.Calls(); len(calls) != 0 {
				t.Fatalf("invalid state called systemd: %#v", calls)
			}
			assertStateDeployOldState(t, root)
			assertNoStateDeployLeftovers(t, root)
		})
	}
}
