package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/test/fixtures"
)

var stateImportTestNow = time.Date(2026, 10, 22, 15, 0, 0, 0, time.UTC)

func stateImportRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{
		"panel-data/uploads",
		"backups/state",
		"backups/mods",
		".locks",
	} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o750); err != nil {
			t.Fatalf("create %s: %v", dir, err)
		}
	}
	return root
}

func fixStateImportNow(t *testing.T, at time.Time) {
	t.Helper()
	old := stateImportNowFunc
	stateImportNowFunc = func() time.Time { return at }
	t.Cleanup(func() { stateImportNowFunc = old })
}

func runStateImportCommand(t *testing.T, root string, args ...string,
) (int, stateImportData, string, string) {
	t.Helper()
	full := append([]string{"--root", root, "--json", "state", "import"}, args...)
	var stdout, stderr bytes.Buffer
	code := run(full, &stdout, &stderr)

	var data stateImportData
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

func writeStateImportFixture(t *testing.T, root string,
) (fixtures.ModsManifest, fixtures.StateArchive, string, []byte) {
	t.Helper()
	modsDir := filepath.Join(root, "backups", "mods")
	manifest := fixtures.ScriptedMods().Install(t, modsDir)
	state := fixtures.FullState(manifest)
	state.Name = fixtures.StateManualName(fixtures.DefaultTime)
	state.Configs = []fixtures.StateFile{{Name: "config.lua", Body: "config = {}\n"}}
	upload := filepath.Join(root, "panel-data", "uploads", "upload.tar.gz")
	state.Write(t, upload)
	body, err := os.ReadFile(upload)
	if err != nil {
		t.Fatalf("read state upload: %v", err)
	}
	return manifest, state, upload, body
}

func TestParseStateImportArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		exit ops.ExitCode
		file string
		arch string
	}{
		{name: "file", args: []string{"--file", "/tmp/state.tar.gz"},
			exit: ops.ExitOK, file: "/tmp/state.tar.gz"},
		{name: "file and name",
			args: []string{"--file", "/tmp/upload", "--name",
				"state-manual-2026-10-22_12-00-00.tar.gz"},
			exit: ops.ExitOK, file: "/tmp/upload",
			arch: "state-manual-2026-10-22_12-00-00.tar.gz"},
		{name: "missing file", exit: ops.ExitInvalidArgument},
		{name: "empty file", args: []string{"--file="}, exit: ops.ExitInvalidArgument},
		{name: "mods name", args: []string{"--file", "a", "--name",
			"mods-2026-10-22_12-00-00.tar.gz"}, exit: ops.ExitInvalidArgument},
		{name: "unknown flag", args: []string{"--bogus"}, exit: ops.ExitInvalidArgument},
		{name: "extra argument", args: []string{"--file", "a", "b"},
			exit: ops.ExitInvalidArgument},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseStateImportArgs(tt.args)
			if err != nil {
				if tt.exit == ops.ExitOK {
					t.Fatalf("parse error: %v", err)
				}
				if err.Exit != tt.exit {
					t.Fatalf("exit = %d, want %d", err.Exit, tt.exit)
				}
				return
			}
			if got.file != tt.file || got.name != tt.arch {
				t.Fatalf("args = %+v", got)
			}
		})
	}
}

func TestRunStateImportSuccess(t *testing.T) {
	root := stateImportRoot(t)
	fixStateImportNow(t, stateImportTestNow)
	manifest, state, upload, body := writeStateImportFixture(t, root)
	name := state.Name
	modsCurrent := fixtures.ReadCurrent(t, filepath.Join(root, "backups", "mods"))

	code, data, stdout, stderr := runStateImportCommand(t, root,
		"--file", upload, "--name", name)
	if code != int(ops.ExitOK) {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q", code, stdout, stderr)
	}
	if data.Archive != name || data.SHA256 != fixtures.SHA256Hex(body) ||
		data.ModsArchive != manifest.Name || data.Players != 1 {
		t.Fatalf("import data = %+v", data)
	}
	target := filepath.Join(root, "backups", "state", name)
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("imported state: %v", err)
	}
	if info.Mode().Perm()&0o400 == 0 {
		t.Fatalf("imported state mode = %04o, want owner-readable", info.Mode().Perm())
	}
	if _, err := archive.ValidateStateFile(target,
		filepath.Join(root, "backups", "mods")); err != nil {
		t.Fatalf("ValidateStateFile: %v", err)
	}
	if _, err := os.Stat(upload); !os.IsNotExist(err) {
		t.Fatalf("upload still exists: %v", err)
	}
	if got := fixtures.ReadCurrent(t, filepath.Join(root, "backups", "mods")); got != modsCurrent {
		t.Fatalf("current.txt changed: %+v -> %+v", modsCurrent, got)
	}
}

func TestRunStateImportUsesUploadBasename(t *testing.T) {
	root := stateImportRoot(t)
	fixStateImportNow(t, stateImportTestNow)
	modsDir := filepath.Join(root, "backups", "mods")
	manifest := fixtures.ScriptedMods().Install(t, modsDir)
	state := fixtures.FullState(manifest)
	state.Name = fixtures.StateManualName(fixtures.DefaultTime)
	upload := filepath.Join(root, "panel-data", "uploads", state.Name)
	state.Write(t, upload)

	code, data, stdout, stderr := runStateImportCommand(t, root, "--file", upload)
	if code != int(ops.ExitOK) {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q", code, stdout, stderr)
	}
	if data.Archive != state.Name {
		t.Fatalf("archive = %q, want %q", data.Archive, state.Name)
	}
	if _, err := os.Stat(filepath.Join(root, "backups", "state", state.Name)); err != nil {
		t.Fatalf("imported state not found: %v", err)
	}
}

func TestRunStateImportRejectsCollision(t *testing.T) {
	root := stateImportRoot(t)
	_, state, upload, _ := writeStateImportFixture(t, root)
	target := filepath.Join(root, "backups", "state", state.Name)
	fixtures.WriteFile(t, target, "existing")

	code, _, stdout, _ := runStateImportCommand(t, root,
		"--file", upload, "--name", state.Name)
	if code != int(ops.ExitInvalidArgument) {
		t.Fatalf("exit = %d, want %d; stdout=%q", code,
			ops.ExitInvalidArgument, stdout)
	}
	if _, err := os.Stat(upload); err != nil {
		t.Fatalf("upload removed after collision: %v", err)
	}
}

func TestRunStateImportRejectsMissingModsPackage(t *testing.T) {
	root := stateImportRoot(t)
	manifest := fixtures.ScriptedMods().Manifest(fixtures.ScriptedMods().Bytes(t))
	state := fixtures.FullState(manifest)
	state.Name = fixtures.StateManualName(fixtures.DefaultTime)
	upload := filepath.Join(root, "panel-data", "uploads", "upload.tar.gz")
	state.Write(t, upload)

	code, _, stdout, _ := runStateImportCommand(t, root,
		"--file", upload, "--name", state.Name)
	if code != int(ops.ExitArchiveInvalid) {
		t.Fatalf("exit = %d, want %d; stdout=%q", code,
			ops.ExitArchiveInvalid, stdout)
	}
	if _, err := os.Stat(filepath.Join(root, "backups", "state", state.Name)); !os.IsNotExist(err) {
		t.Fatalf("invalid state was imported: %v", err)
	}
}

func TestRunStateImportDryRunWritesNothing(t *testing.T) {
	root := stateImportRoot(t)
	fixStateImportNow(t, stateImportTestNow)
	_, state, upload, _ := writeStateImportFixture(t, root)

	code, data, stdout, stderr := runStateImportCommand(t, root,
		"--dry-run", "--file", upload, "--name", state.Name)
	if code != int(ops.ExitOK) {
		t.Fatalf("dry run = %d; stdout=%q stderr=%q", code, stdout, stderr)
	}
	if data.Archive != state.Name {
		t.Fatalf("dry-run archive = %q", data.Archive)
	}
	if _, err := os.Stat(filepath.Join(root, "backups", "state", state.Name)); !os.IsNotExist(err) {
		t.Fatalf("dry run created state: %v", err)
	}
	if _, err := os.Stat(upload); err != nil {
		t.Fatalf("dry run removed upload: %v", err)
	}
}

func TestRunStateImportFallsBackToCopyOnEXDEV(t *testing.T) {
	root := stateImportRoot(t)
	fixStateImportNow(t, stateImportTestNow)
	_, state, upload, body := writeStateImportFixture(t, root)

	old := stateImportRenameFunc
	stateImportRenameFunc = func(oldpath, newpath string) error {
		return &os.LinkError{Op: "rename", Old: oldpath, New: newpath,
			Err: syscall.EXDEV}
	}
	t.Cleanup(func() { stateImportRenameFunc = old })

	code, data, stdout, stderr := runStateImportCommand(t, root,
		"--file", upload, "--name", state.Name)
	if code != int(ops.ExitOK) {
		t.Fatalf("exit = %d; stdout=%q stderr=%q", code, stdout, stderr)
	}
	if data.Archive != state.Name {
		t.Fatalf("archive = %q", data.Archive)
	}
	copied, err := os.ReadFile(filepath.Join(root, "backups", "state", data.Archive))
	if err != nil {
		t.Fatalf("read copied state: %v", err)
	}
	if !bytes.Equal(copied, body) {
		t.Fatalf("copied state differs: %d vs %d bytes", len(copied), len(body))
	}
	if _, err := os.Stat(upload); !os.IsNotExist(err) {
		t.Fatalf("upload still exists after copy fallback: %v", err)
	}
}

func TestStateImportErrorMapping(t *testing.T) {
	tests := []struct {
		err  error
		exit ops.ExitCode
	}{
		{archive.ErrInvalid, ops.ExitArchiveInvalid},
		{archive.ErrLimit, ops.ExitArchiveInvalid},
		{os.ErrNotExist, ops.ExitNotFound},
		{os.ErrExist, ops.ExitInvalidArgument},
		{syscall.ENOSPC, ops.ExitNoSpace},
		{errors.New("boom"), ops.ExitInternal},
	}
	for _, tt := range tests {
		got := stateImportError("validate", tt.err)
		if got.Exit != tt.exit {
			t.Fatalf("%v: exit = %d, want %d", tt.err, got.Exit, tt.exit)
		}
		if !strings.Contains(got.Message, "state import: validate") {
			t.Fatalf("message = %q", got.Message)
		}
	}
}

func TestWriteStateImportHuman(t *testing.T) {
	var out bytes.Buffer
	err := writeStateImportHuman(&out, stateImportData{
		Archive: "state.tar.gz", SHA256: "abc",
		ModsArchive: "mods.tar.gz", Players: 2, WorldFiles: 3,
	})
	if err != nil {
		t.Fatalf("writeStateImportHuman: %v", err)
	}
	for _, want := range []string{"archive: state.tar.gz", "mods_archive: mods.tar.gz",
		"players: 2", "world_files: 3"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output does not contain %q:\n%s", want, out.String())
		}
	}
}
