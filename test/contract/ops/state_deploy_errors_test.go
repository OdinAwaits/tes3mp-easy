package ops

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
	"github.com/Saagilat/tes3mp-easy/internal/systemd"
	"github.com/Saagilat/tes3mp-easy/test/fixtures"
)

func assertNoStatePredeploy(t *testing.T, root string) {
	t.Helper()

	entries, err := os.ReadDir(filepath.Join(root, "backups", "state"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		parsed, err := archive.ParseArchiveName(entry.Name())
		if err == nil && parsed.Kind == archive.KindStatePredeploy {
			t.Fatalf("unexpected predeploy archive %s", entry.Name())
		}
	}
}

func TestStateDeployArchiveErrorsContract(t *testing.T) {
	tests := []struct {
		name  string
		state func(t *testing.T, root string,
			manifest fixtures.ModsManifest) fixtures.StateArchive
	}{
		{
			name: "foreign current",
			state: func(t *testing.T, root string,
				_ fixtures.ModsManifest,
			) fixtures.StateArchive {
				foreign := fixtures.DefaultMods()
				foreign.Name = "mods-2026-10-22_14-00-00.tar.gz"
				body := foreign.Bytes(t)
				fixtures.WriteFile(t,
					filepath.Join(root, "backups", "mods", foreign.Name),
					string(body))
				state := fixtures.FullState(foreign.Manifest(body))
				state.Name = "state-manual-2026-10-22_15-00-00.tar.gz"
				return state
			},
		},
		{
			name: "missing current",
			state: func(t *testing.T, _ string,
				manifest fixtures.ModsManifest,
			) fixtures.StateArchive {
				state := fixtures.FullState(manifest)
				state.Name = "state-manual-2026-10-22_15-00-00.tar.gz"
				state.OmitCurrent = true
				return state
			},
		},
		{
			name: "old current format",
			state: func(t *testing.T, _ string,
				manifest fixtures.ModsManifest,
			) fixtures.StateArchive {
				state := fixtures.FullState(manifest)
				state.Name = "state-manual-2026-10-22_15-00-00.tar.gz"
				state.OmitCurrent = true
				state.Extra = append(state.Extra, fixtures.Entry{
					Name: fixtures.CurrentName,
					Body: manifest.SHA256 + "\n",
				})
				return state
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, manifest, _, _ := stateDeployContractRoot(t)
			state := tt.state(t, root, manifest)
			state.Write(t, filepath.Join(root, "backups", "state", state.Name))

			res := testHarness.RunRoot(t, root, "--json", "state", "deploy",
				"--archive", state.Name)
			res.requireJSONError(t, 7, "archive_invalid")

			assertStateDeployContractOld(t, root)
			assertNoStatePredeploy(t, root)
			assertStateDeployContractLeftovers(t, root)
		})
	}
}

func TestStateDeployExtractionRollbackContract(t *testing.T) {
	root, manifest, _, _ := stateDeployContractRoot(t)
	state := fixtures.FullState(manifest)
	state.Name = "state-manual-2026-10-22_16-00-00.tar.gz"
	state.Players = []fixtures.StateFile{{
		Name: strings.Repeat("a", 300) + ".json",
		Body: "{}\n",
	}}
	state.Write(t, filepath.Join(root, "backups", "state", state.Name))

	sudoLog := filepath.Join(t.TempDir(), "sudo.log")
	res := testHarness.RunRootEnv(t, root, map[string]string{
		"FAKE_SYSTEMD_STATE": "active",
		"FAKE_SUDO_LOG":      sudoLog,
	}, "--json", "state", "deploy", "--archive", state.Name)
	res.requireJSONError(t, 1, "internal")

	assertStateDeployContractOld(t, root)
	assertStateDeployContractLeftovers(t, root)
	wantSudo := [][]string{
		{"-n", systemd.RootctlPath, "stop"},
		{"-n", systemd.RootctlPath, "start"},
	}
	if calls := readFakeCalls(t, sudoLog); !reflect.DeepEqual(calls, wantSudo) {
		t.Fatalf("sudo calls = %#v, want rollback restart %#v", calls, wantSudo)
	}
	entries, err := os.ReadDir(filepath.Join(root, "backups", "state"))
	if err != nil {
		t.Fatal(err)
	}
	predeploys := 0
	for _, entry := range entries {
		parsed, err := archive.ParseArchiveName(entry.Name())
		if err == nil && parsed.Kind == archive.KindStatePredeploy {
			predeploys++
		}
	}
	if predeploys != 1 {
		t.Fatalf("predeploy archives = %d, want one", predeploys)
	}
}

func TestStateDeployLockedContract(t *testing.T) {
	root, _, _, stateName := stateDeployContractRoot(t)
	holdOpsLock(t, root)

	res := testHarness.RunRoot(t, root, "--lock-timeout=0",
		"--json", "state", "deploy", "--archive", stateName)
	res.requireJSONError(t, 4, "locked")

	assertStateDeployContractOld(t, root)
	assertNoStatePredeploy(t, root)
	assertStateDeployContractLeftovers(t, root)
}
