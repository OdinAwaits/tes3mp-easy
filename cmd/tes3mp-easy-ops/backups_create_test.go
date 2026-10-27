package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
	"github.com/Saagilat/tes3mp-easy/internal/lock"
	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/retention"
	"github.com/Saagilat/tes3mp-easy/internal/systemd"
	"github.com/Saagilat/tes3mp-easy/internal/systemd/systemdtest"
	"github.com/Saagilat/tes3mp-easy/test/fixtures"
)

var backupWorldDirs = []string{"cell", "world", "map", "recordstore", "custom"}

func backupCreateRoot(t *testing.T) (string, fixtures.ModsManifest) {
	t.Helper()

	root := t.TempDir()
	for _, dir := range []string{
		"players",
		"world",
		"configs",
		"mods/plugins",
		"backups/mods",
		"backups/state",
		".locks",
	} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o750); err != nil {
			t.Fatalf("create %s: %v", dir, err)
		}
	}
	for _, name := range backupWorldDirs {
		if err := os.MkdirAll(filepath.Join(root, "world", name), 0o750); err != nil {
			t.Fatalf("create world/%s: %v", name, err)
		}
	}
	fixtures.WriteConfigs(t, filepath.Join(root, "configs"))
	if err := os.Chmod(filepath.Join(root, "configs"), 0o700); err != nil {
		t.Fatalf("chmod configs: %v", err)
	}

	mods := fixtures.DefaultModsInit()
	manifest := mods.Install(t, filepath.Join(root, "backups", "mods"))
	fixtures.WriteFile(t,
		filepath.Join(root, "mods", "plugins", "requiredDataFiles.json"),
		string(mods.RequiredJSON()))
	fixtures.WriteFile(t, filepath.Join(root, "players", "player.json"), "{}\n")
	for _, name := range backupWorldDirs {
		fixtures.WriteFile(t,
			filepath.Join(root, "world", name, name+".json"), "{}\n")
	}
	return root, manifest
}

func runCreateCommand(t *testing.T, root string, client *systemd.Client,
	args ...string,
) (int, backupsCreateData, string, string) {
	t.Helper()

	full := append([]string{"--root", root, "--json", "backups", "create"}, args...)
	var stdout, stderr bytes.Buffer
	var code int
	if client == nil {
		code = run(full, &stdout, &stderr)
	} else {
		code = runWithClient(full, &stdout, &stderr, client)
	}

	var data backupsCreateData
	if stdout.Len() > 0 {
		var envelope struct {
			OK   bool            `json:"ok"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
			t.Fatalf("decode envelope: %v; stdout=%q", err, stdout.String())
		}
		if envelope.OK {
			if err := json.Unmarshal(envelope.Data, &data); err != nil {
				t.Fatalf("decode data: %v; data=%s", err, envelope.Data)
			}
		}
	}
	return code, data, stdout.String(), stderr.String()
}

func TestRunBackupsCreateJSON(t *testing.T) {
	root, _ := backupCreateRoot(t)
	code, data, stdout, stderr := runCreateCommand(t, root, nil,
		"--type", "state")
	if code != int(ops.ExitOK) {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q", code, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if data.Skipped {
		t.Fatal("skipped = true, want a created archive")
	}
	if data.Size == nil || *data.Size <= 0 {
		t.Fatalf("size = %v, want positive", data.Size)
	}
	if data.SHA256 == nil || len(*data.SHA256) != 64 {
		t.Fatalf("sha256 = %v, want 64 hex characters", data.SHA256)
	}
	if len(data.Pruned) != 0 {
		t.Fatalf("pruned = %v, want empty", data.Pruned)
	}

	parsed, err := archive.ParseArchiveName(data.Archive)
	if err != nil {
		t.Fatalf("archive name: %v", err)
	}
	if parsed.Kind != archive.KindStateAuto {
		t.Fatalf("kind = %s, want state", parsed.Kind)
	}

	path := filepath.Join(root, "backups", "state", data.Archive)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat archive: %v", err)
	}
	if info.Size() != *data.Size {
		t.Fatalf("size = %d, want %d", info.Size(), *data.Size)
	}
	sum, err := backupSHA256(path)
	if err != nil {
		t.Fatalf("sha256: %v", err)
	}
	if sum != *data.SHA256 {
		t.Fatalf("sha256 = %s, want %s", sum, *data.SHA256)
	}
	if _, err := os.Stat(path + ".part"); !os.IsNotExist(err) {
		t.Fatalf(".part exists after create: %v", err)
	}

	result, err := archive.ValidateStateFile(path,
		filepath.Join(root, "backups", "mods"))
	if err != nil {
		t.Fatalf("ValidateStateFile: %v", err)
	}
	if result.Players != 1 || result.WorldFiles != len(backupWorldDirs) {
		t.Fatalf("state info = %+v", result)
	}
}

func TestRunBackupsCreateManualAndRetention(t *testing.T) {
	root, _ := backupCreateRoot(t)
	stateDir := filepath.Join(root, "backups", "state")

	now := time.Now().UTC()
	old := make([]string, 0, 6)
	for i := 1; i <= 6; i++ {
		name := archive.ArchiveName{
			Kind: archive.KindStateManual,
			Time: now.AddDate(0, 0, -i),
		}.String()
		if err := os.WriteFile(filepath.Join(stateDir, name),
			[]byte("old"), 0o640); err != nil {
			t.Fatal(err)
		}
		old = append(old, name)
	}

	code, data, stdout, stderr := runCreateCommand(t, root, nil,
		"--type", "state", "--manual")
	if code != int(ops.ExitOK) {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q", code, stdout, stderr)
	}
	parsed, err := archive.ParseArchiveName(data.Archive)
	if err != nil {
		t.Fatalf("archive name: %v", err)
	}
	if parsed.Kind != archive.KindStateManual {
		t.Fatalf("kind = %s, want state-manual", parsed.Kind)
	}
	if len(data.Pruned) != 2 {
		t.Fatalf("pruned = %v, want 2 oldest manual archives", data.Pruned)
	}
	for _, name := range old[len(old)-2:] {
		if _, err := os.Stat(filepath.Join(stateDir, name)); !os.IsNotExist(err) {
			t.Fatalf("old archive %s still exists: %v", name, err)
		}
	}
}

func TestRunBackupsPruneJSON(t *testing.T) {
	root, _ := backupCreateRoot(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{"--root", root, "--json", "backups", "prune"},
		&stdout, &stderr)
	if code != int(ops.ExitOK) {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q",
			code, stdout.String(), stderr.String())
	}
	var envelope struct {
		OK   bool             `json:"ok"`
		Data backupsPruneData `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode: %v; stdout=%q", err, stdout.String())
	}
	if !envelope.OK || envelope.Data.Removed == nil ||
		len(envelope.Data.Removed) != 0 {
		t.Fatalf("prune = %+v", envelope.Data)
	}
}

func TestRunBackupsCreateIfRunningSkipped(t *testing.T) {
	root, _ := backupCreateRoot(t)
	runner := systemdtest.New()
	runner.Respond("systemctl", []string{"is-active", systemd.UnitTES3MPService},
		"inactive\n", commandError("systemctl",
			[]string{"is-active", systemd.UnitTES3MPService}, 3, ""))

	code, data, stdout, stderr := runCreateCommand(t, root,
		systemd.New(runner), "--type", "state", "--if-running")
	if code != int(ops.ExitOK) {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !data.Skipped || data.Reason != "not_running" {
		t.Fatalf("data = %+v, want skipped not_running", data)
	}
	if data.Archive != "" || data.Size != nil || data.SHA256 != nil {
		t.Fatalf("skipped data contains archive fields: %+v", data)
	}
	entries, err := os.ReadDir(filepath.Join(root, "backups", "state"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("state archives = %d, want none", len(entries))
	}

	wantCalls := []systemdtest.Call{{
		Name: "systemctl",
		Args: []string{"is-active", systemd.UnitTES3MPService},
	}}
	if calls := runner.Calls(); !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", calls, wantCalls)
	}
}

func TestRunBackupsCreateIfRunningActive(t *testing.T) {
	root, _ := backupCreateRoot(t)
	runner := systemdtest.New()
	runner.Respond("systemctl", []string{"is-active", systemd.UnitTES3MPService},
		"active\n", nil)

	code, data, stdout, stderr := runCreateCommand(t, root,
		systemd.New(runner), "--type", "state", "--if-running")
	if code != int(ops.ExitOK) {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q", code, stdout, stderr)
	}
	if data.Skipped || data.Archive == "" {
		t.Fatalf("data = %+v, want a created archive", data)
	}
	if _, err := os.Stat(filepath.Join(root, "backups", "state", data.Archive)); err != nil {
		t.Fatalf("stat archive: %v", err)
	}
	if calls := runner.Calls(); len(calls) != 1 {
		t.Fatalf("calls = %#v, want one is-active call", calls)
	}
}

func TestRunBackupsCreateIfRunningServiceUnavailable(t *testing.T) {
	root, _ := backupCreateRoot(t)
	runner := systemdtest.New()
	runner.Respond("systemctl", []string{"is-active", systemd.UnitTES3MPService},
		"", commandError("systemctl",
			[]string{"is-active", systemd.UnitTES3MPService}, 1,
			"Failed to connect to bus"))

	code, _, stdout, _ := runCreateCommand(t, root,
		systemd.New(runner), "--type", "state", "--if-running")
	if code != int(ops.ExitServiceUnavailable) {
		t.Fatalf("exit = %d, want 6; stdout=%q", code, stdout)
	}
	assertJSONError(t, stdout, ops.ExitServiceUnavailable, "systemctl")
	entries, err := os.ReadDir(filepath.Join(root, "backups", "state"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("state archives = %d, want none", len(entries))
	}
}

func TestRunBackupsCreateLocked(t *testing.T) {
	root, _ := backupCreateRoot(t)
	held, err := lock.Acquire(filepath.Join(root, ".locks", "ops.lock"),
		lock.Exclusive, 0)
	if err != nil {
		t.Fatalf("hold lock: %v", err)
	}
	defer func() { _ = held.Close() }()

	code, _, stdout, _ := runCreateCommand(t, root, nil,
		"--type", "state", "--lock-timeout=0")
	if code != int(ops.ExitLocked) {
		t.Fatalf("exit = %d, want 4; stdout=%q", code, stdout)
	}
	assertJSONError(t, stdout, ops.ExitLocked, "lock")
	entries, err := os.ReadDir(filepath.Join(root, "backups", "state"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("state archives = %d, want none", len(entries))
	}
}

func TestRunBackupsCreateDryRunRespectsHeldLock(t *testing.T) {
	root, _ := backupCreateRoot(t)
	held, err := lock.Acquire(filepath.Join(root, ".locks", "ops.lock"),
		lock.Exclusive, 0)
	if err != nil {
		t.Fatalf("hold lock: %v", err)
	}
	defer func() { _ = held.Close() }()

	code, _, stdout, _ := runCreateCommand(t, root, nil,
		"--type", "state", "--dry-run", "--lock-timeout=0")
	if code != int(ops.ExitLocked) {
		t.Fatalf("exit = %d, want 4; stdout=%q", code, stdout)
	}
	assertJSONError(t, stdout, ops.ExitLocked, "lock")
}

func TestRunBackupsCreateDryRunWritesNothing(t *testing.T) {
	root, _ := backupCreateRoot(t)
	stateDir := filepath.Join(root, "backups", "state")
	lockPath := filepath.Join(root, ".locks", "ops.lock")

	code, data, stdout, stderr := runCreateCommand(t, root, nil,
		"--type", "state", "--dry-run")
	if code != int(ops.ExitOK) {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q", code, stdout, stderr)
	}
	if data.Archive == "" || data.Size == nil || data.SHA256 == nil {
		t.Fatalf("dry-run data = %+v, want a plan", data)
	}
	if _, err := os.Stat(filepath.Join(stateDir, data.Archive)); !os.IsNotExist(err) {
		t.Fatalf("dry-run created archive: %v", err)
	}
	if _, err := os.Stat(filepath.Join(stateDir, data.Archive+".part")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created .part: %v", err)
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("dry-run created lock file: %v", err)
	}
}

func TestRunBackupsCreateRejectsInvalidArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing type", args: []string{}},
		{name: "mods type", args: []string{"--type", "mods"}},
		{name: "unknown flag", args: []string{"--type", "state", "--bogus"}},
		{name: "extra argument", args: []string{"--type", "state", "extra"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, _ := backupCreateRoot(t)
			code, _, stdout, _ := runCreateCommand(t, root, nil, tt.args...)
			if code != int(ops.ExitInvalidArgument) {
				t.Fatalf("exit = %d, want 2; stdout=%q", code, stdout)
			}
			assertJSONError(t, stdout, ops.ExitInvalidArgument, "backups create")
		})
	}
}

func TestRunBackupsCreateInvalidCurrent(t *testing.T) {
	root, _ := backupCreateRoot(t)
	if err := os.Remove(filepath.Join(root, "backups", "mods", "current.txt")); err != nil {
		t.Fatal(err)
	}
	code, _, stdout, _ := runCreateCommand(t, root, nil, "--type", "state")
	if code != int(ops.ExitArchiveInvalid) {
		t.Fatalf("exit = %d, want 7; stdout=%q", code, stdout)
	}
	assertJSONError(t, stdout, ops.ExitArchiveInvalid, "current.txt")
}

func TestRunBackupsCreateMissingCurrentArchive(t *testing.T) {
	root, manifest := backupCreateRoot(t)
	if err := os.Remove(filepath.Join(root, "backups", "mods", manifest.Name)); err != nil {
		t.Fatal(err)
	}
	code, _, stdout, _ := runCreateCommand(t, root, nil, "--type", "state")
	if code != int(ops.ExitArchiveInvalid) {
		t.Fatalf("exit = %d, want 7; stdout=%q", code, stdout)
	}
	assertJSONError(t, stdout, ops.ExitArchiveInvalid, manifest.Name)
}

func TestRunBackupsCreateMissingConfig(t *testing.T) {
	root, _ := backupCreateRoot(t)
	config := filepath.Join(root, "configs", "config.lua")
	if err := os.Remove(config); err != nil {
		t.Fatal(err)
	}
	code, _, stdout, _ := runCreateCommand(t, root, nil, "--type", "state")
	if code != int(ops.ExitArchiveInvalid) {
		t.Fatalf("exit = %d, want 7; stdout=%q", code, stdout)
	}
	assertJSONError(t, stdout, ops.ExitArchiveInvalid, "config.lua")
	entries, err := os.ReadDir(filepath.Join(root, "backups", "state"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("state directory contains %d entries, want none", len(entries))
	}
}

func TestRunBackupsCreateWriteFailureCleansPart(t *testing.T) {
	root, _ := backupCreateRoot(t)
	if err := os.Mkdir(filepath.Join(root, "players", "nested"), 0o750); err != nil {
		t.Fatal(err)
	}
	code, _, stdout, _ := runCreateCommand(t, root, nil, "--type", "state")
	if code != int(ops.ExitInternal) {
		t.Fatalf("exit = %d, want 1; stdout=%q", code, stdout)
	}
	entries, err := os.ReadDir(filepath.Join(root, "backups", "state"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("state directory contains %d entries, want none", len(entries))
	}
}

func TestRunBackupsCreateNoSpace(t *testing.T) {
	root, _ := backupCreateRoot(t)
	old := backupDiskFreeFunc
	backupDiskFreeFunc = func(string) (int64, error) {
		return retention.DefaultMinFree - 1, nil
	}
	defer func() { backupDiskFreeFunc = old }()

	code, _, stdout, _ := runCreateCommand(t, root, nil, "--type", "state")
	if code != int(ops.ExitNoSpace) {
		t.Fatalf("exit = %d, want 5; stdout=%q", code, stdout)
	}
	assertJSONError(t, stdout, ops.ExitNoSpace, "free space")
	entries, err := os.ReadDir(filepath.Join(root, "backups", "state"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("state archives = %d, want none", len(entries))
	}
}

func TestBackupCreateErrorNoSpace(t *testing.T) {
	err := backupCreateError("write", &os.PathError{
		Op: "write", Path: "archive.part", Err: syscall.ENOSPC,
	})
	if err.Exit != ops.ExitNoSpace || err.Code != ops.CodeNoSpace {
		t.Fatalf("error = %#v, want no_space/exit 5", err)
	}
}

func TestCheckBackupSpace(t *testing.T) {
	old := backupDiskFreeFunc
	defer func() { backupDiskFreeFunc = old }()

	tests := []struct {
		name         string
		free         int64
		wantExit     ops.ExitCode
		wantWarnings int
	}{
		{name: "below reserve", free: retention.DefaultMinFree - 1,
			wantExit: ops.ExitNoSpace},
		{name: "at reserve", free: retention.DefaultMinFree,
			wantExit: ops.ExitOK, wantWarnings: 1},
		{name: "below warning", free: backupWarnFree - 1,
			wantExit: ops.ExitOK, wantWarnings: 1},
		{name: "at warning", free: backupWarnFree, wantExit: ops.ExitOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backupDiskFreeFunc = func(string) (int64, error) {
				return tt.free, nil
			}
			warnings, err := checkBackupSpace(t.TempDir())
			if err != nil && err.Exit != tt.wantExit {
				t.Fatalf("exit = %d, want %d", err.Exit, tt.wantExit)
			}
			if err == nil && tt.wantExit != ops.ExitOK {
				t.Fatalf("checkBackupSpace succeeded, want exit %d", tt.wantExit)
			}
			if len(warnings) != tt.wantWarnings {
				t.Fatalf("warnings = %v, want %d", warnings, tt.wantWarnings)
			}
		})
	}
}
