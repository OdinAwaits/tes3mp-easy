package main

import (
	"archive/tar"
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
	"github.com/Saagilat/tes3mp-easy/internal/lock"
	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/pathguard"
	"github.com/Saagilat/tes3mp-easy/test/fixtures"
)

var modsImportTestNow = time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)

func modsImportRoot(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	for _, dir := range []string{
		"panel-data/uploads",
		"backups/mods",
		".locks",
	} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o750); err != nil {
			t.Fatalf("create %s: %v", dir, err)
		}
	}
	return root
}

func fixModsImportNow(t *testing.T, at time.Time) {
	t.Helper()

	old := modsImportNowFunc
	modsImportNowFunc = func() time.Time { return at }
	t.Cleanup(func() { modsImportNowFunc = old })
}

func writeModsUpload(t *testing.T, root string,
	mods fixtures.ModsArchive,
) (string, []byte) {
	t.Helper()

	path := filepath.Join(root, "panel-data", "uploads", "upload.tar.gz")
	mods.Write(t, path)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read upload: %v", err)
	}
	return path, data
}

func runModsImportCommand(t *testing.T, root string, args ...string,
) (int, modsImportData, string, string) {
	t.Helper()

	full := append([]string{"--root", root, "--json", "mods", "import"}, args...)
	var stdout, stderr bytes.Buffer
	code := run(full, &stdout, &stderr)

	var data modsImportData
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

func TestParseModsImportArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		exit ops.ExitCode
		file string
	}{
		{
			name: "valid",
			args: []string{"--file", "/tmp/upload.tar.gz"},
			exit: ops.ExitOK, file: "/tmp/upload.tar.gz",
		},
		{
			name: "equals form",
			args: []string{"--file=/tmp/upload.tar.gz"},
			exit: ops.ExitOK, file: "/tmp/upload.tar.gz",
		},
		{name: "missing file", args: nil, exit: ops.ExitInvalidArgument},
		{name: "empty file", args: []string{"--file="}, exit: ops.ExitInvalidArgument},
		{name: "unknown flag", args: []string{"--bogus"}, exit: ops.ExitInvalidArgument},
		{
			name: "extra argument",
			args: []string{"--file", "a", "b"},
			exit: ops.ExitInvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseModsImportArgs(tt.args)
			if err != nil {
				if tt.exit == ops.ExitOK {
					t.Fatalf("parse error: %v", err)
				}
				if err.Exit != tt.exit {
					t.Fatalf("exit = %d, want %d", err.Exit, tt.exit)
				}
				if !strings.Contains(err.Message, "mods import") {
					t.Fatalf("message = %q, want command prefix", err.Message)
				}
				return
			}
			if tt.exit != ops.ExitOK {
				t.Fatal("parse succeeded, want error")
			}
			if got.file != tt.file {
				t.Fatalf("file = %q, want %q", got.file, tt.file)
			}
		})
	}
}

func TestRunModsImportSuccess(t *testing.T) {
	root := modsImportRoot(t)
	fixModsImportNow(t, modsImportTestNow)
	modsDir := filepath.Join(root, "backups", "mods")
	fixtures.DefaultModsInit().Install(t, modsDir)
	currentBefore := fixtures.ReadCurrent(t, modsDir)

	mods := fixtures.ScriptedMods()
	upload, body := writeModsUpload(t, root, mods)
	code, data, stdout, stderr := runModsImportCommand(t, root, "--file", upload)
	if code != int(ops.ExitOK) {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q", code, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}

	wantName := fixtures.ModsName(modsImportTestNow)
	if data.Archive != wantName {
		t.Fatalf("archive = %q, want %q", data.Archive, wantName)
	}
	if data.SHA256 != fixtures.SHA256Hex(body) {
		t.Fatalf("sha256 = %s, want %s", data.SHA256, fixtures.SHA256Hex(body))
	}
	if data.Plugins != 1 || data.Scripts != 2 {
		t.Fatalf("plugins/scripts = %d/%d, want 1/2", data.Plugins, data.Scripts)
	}
	if len(data.Pruned) != 0 {
		t.Fatalf("pruned = %v, want empty", data.Pruned)
	}

	target := filepath.Join(modsDir, data.Archive)
	info, err := archive.ValidateModsFile(target)
	if err != nil {
		t.Fatalf("ValidateModsFile: %v", err)
	}
	if info.Plugins != data.Plugins || info.Scripts != data.Scripts {
		t.Fatalf("validated counts = %d/%d, want %d/%d",
			info.Plugins, info.Scripts, data.Plugins, data.Scripts)
	}
	if _, err := os.Stat(upload); !os.IsNotExist(err) {
		t.Fatalf("upload still exists after import: %v", err)
	}
	if _, err := os.Stat(target + ".part"); !os.IsNotExist(err) {
		t.Fatalf(".part exists after import: %v", err)
	}
	if currentAfter := fixtures.ReadCurrent(t, modsDir); currentAfter != currentBefore {
		t.Fatalf("current.txt changed: %q -> %q", currentBefore, currentAfter)
	}
	if _, err := os.Stat(filepath.Join(root, ".locks", "ops.lock")); err != nil {
		t.Fatalf("ops lock was not created: %v", err)
	}
}

func TestRunModsImportAddsCollisionSuffix(t *testing.T) {
	root := modsImportRoot(t)
	fixModsImportNow(t, modsImportTestNow)
	modsDir := filepath.Join(root, "backups", "mods")
	existing := fixtures.ModsName(modsImportTestNow)
	fixtures.WriteFile(t, filepath.Join(modsDir, existing), "existing")

	upload, _ := writeModsUpload(t, root, fixtures.DefaultMods())
	code, data, stdout, stderr := runModsImportCommand(t, root, "--file", upload)
	if code != int(ops.ExitOK) {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q", code, stdout, stderr)
	}
	want := fixtures.Name("mods-", modsImportTestNow, 1)
	if data.Archive != want {
		t.Fatalf("archive = %q, want collision name %q", data.Archive, want)
	}
	if _, err := os.Stat(filepath.Join(modsDir, existing)); err != nil {
		t.Fatalf("existing archive was removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(modsDir, want)); err != nil {
		t.Fatalf("imported archive: %v", err)
	}
}

func TestRunModsImportRejectsInvalidArchives(t *testing.T) {
	rootJSON := fixtures.DefaultMods()
	rootJSON.MetadataPath = fixtures.MetadataName
	missingMetadata := fixtures.DefaultMods()
	missingMetadata.OmitMetadata = true
	badCRC := fixtures.ModsArchive{
		Plugins: []fixtures.Plugin{{
			Name: "mod.esp", Body: "synthetic",
			CRCs: []string{"0x00000000"},
		}},
	}
	baseESM := fixtures.DefaultMods()
	baseESM.Extra = []fixtures.Entry{{
		Name: "plugins/Morrowind.esm", Body: "synthetic",
	}}
	symlink := fixtures.DefaultMods()
	symlink.Extra = []fixtures.Entry{{
		Name: "plugins/link.esp", Type: tar.TypeSymlink, Linkname: "mod.esp",
	}}
	duplicate := fixtures.DefaultMods()
	duplicate.Extra = []fixtures.Entry{{
		Name: "plugins/mod.esp", Body: "synthetic duplicate",
	}}
	nestedScript := fixtures.DefaultMods()
	nestedScript.Extra = []fixtures.Entry{{
		Name: "scripts/nested/test.lua", Body: "print('nested')",
	}}
	traversal := fixtures.DefaultMods()
	traversal.Extra = []fixtures.Entry{{
		Name: "../escape", Body: "escape",
	}}
	absolute := fixtures.DefaultMods()
	absolute.Extra = []fixtures.Entry{{
		Name: "/escape", Body: "escape",
	}}

	tests := []struct {
		name string
		mods fixtures.ModsArchive
		exit ops.ExitCode
	}{
		{name: "root json", mods: rootJSON, exit: ops.ExitArchiveInvalid},
		{name: "missing metadata", mods: missingMetadata, exit: ops.ExitArchiveInvalid},
		{name: "bad crc", mods: badCRC, exit: ops.ExitArchiveInvalid},
		{name: "base esm", mods: baseESM, exit: ops.ExitArchiveInvalid},
		{name: "symlink", mods: symlink, exit: ops.ExitArchiveInvalid},
		{name: "duplicate", mods: duplicate, exit: ops.ExitArchiveInvalid},
		{name: "nested script", mods: nestedScript, exit: ops.ExitArchiveInvalid},
		{name: "traversal", mods: traversal, exit: ops.ExitUnsafePath},
		{name: "absolute path", mods: absolute, exit: ops.ExitUnsafePath},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := modsImportRoot(t)
			upload, _ := writeModsUpload(t, root, tt.mods)
			code, _, stdout, _ := runModsImportCommand(t, root,
				"--file", upload)
			if code != int(tt.exit) {
				t.Fatalf("exit = %d, want %d; stdout=%q",
					code, tt.exit, stdout)
			}
			assertJSONError(t, stdout, tt.exit, "mods import")
			if _, err := os.Stat(upload); err != nil {
				t.Fatalf("upload was removed after failure: %v", err)
			}
			entries, err := os.ReadDir(filepath.Join(root, "backups", "mods"))
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("mods directory contains %d entries, want none",
					len(entries))
			}
		})
	}
}

func TestRunModsImportMissingUpload(t *testing.T) {
	root := modsImportRoot(t)
	missing := filepath.Join(root, "panel-data", "uploads", "missing.tar.gz")
	code, _, stdout, _ := runModsImportCommand(t, root, "--file", missing)
	if code != int(ops.ExitNotFound) {
		t.Fatalf("exit = %d, want 3; stdout=%q", code, stdout)
	}
	assertJSONError(t, stdout, ops.ExitNotFound, "does not exist")
}

func TestRunModsImportRejectsSymlinkUpload(t *testing.T) {
	root := modsImportRoot(t)
	target := filepath.Join(root, "panel-data", "uploads", "target.tar.gz")
	fixtures.DefaultMods().Write(t, target)
	link := filepath.Join(root, "panel-data", "uploads", "link.tar.gz")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink is unavailable: %v", err)
	}

	code, _, stdout, _ := runModsImportCommand(t, root, "--file", link)
	if code != int(ops.ExitInvalidArgument) {
		t.Fatalf("exit = %d, want 2; stdout=%q", code, stdout)
	}
	assertJSONError(t, stdout, ops.ExitInvalidArgument, "symlink")
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("target upload was removed: %v", err)
	}
}

func TestRunModsImportRejectsDirectoryUpload(t *testing.T) {
	root := modsImportRoot(t)
	upload := filepath.Join(root, "panel-data", "uploads", "directory")
	if err := os.Mkdir(upload, 0o750); err != nil {
		t.Fatal(err)
	}
	code, _, stdout, _ := runModsImportCommand(t, root, "--file", upload)
	if code != int(ops.ExitInvalidArgument) {
		t.Fatalf("exit = %d, want 2; stdout=%q", code, stdout)
	}
	assertJSONError(t, stdout, ops.ExitInvalidArgument, "not a regular file")
}

func TestRunModsImportRejectsDifferentFilesystems(t *testing.T) {
	root := modsImportRoot(t)
	upload, _ := writeModsUpload(t, root, fixtures.DefaultMods())

	old := modsImportDeviceOfPathFunc
	modsImportDeviceOfPathFunc = func(path string) (uint64, error) {
		if strings.Contains(path, "uploads") {
			return 11, nil
		}
		return 22, nil
	}
	defer func() { modsImportDeviceOfPathFunc = old }()

	code, _, stdout, _ := runModsImportCommand(t, root, "--file", upload)
	if code != int(ops.ExitInvalidArgument) {
		t.Fatalf("exit = %d, want 2; stdout=%q", code, stdout)
	}
	assertJSONError(t, stdout, ops.ExitInvalidArgument, "different filesystems")
	if _, err := os.Stat(upload); err != nil {
		t.Fatalf("upload was removed: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(root, "backups", "mods"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("mods directory contains %d entries, want none", len(entries))
	}
}

func TestRunModsImportRejectsOversizedUpload(t *testing.T) {
	root := modsImportRoot(t)
	upload, body := writeModsUpload(t, root, fixtures.DefaultMods())

	old := modsImportMaxCompressed
	modsImportMaxCompressed = int64(len(body) - 1)
	defer func() { modsImportMaxCompressed = old }()

	code, _, stdout, _ := runModsImportCommand(t, root, "--file", upload)
	if code != int(ops.ExitArchiveInvalid) {
		t.Fatalf("exit = %d, want 7; stdout=%q", code, stdout)
	}
	assertJSONError(t, stdout, ops.ExitArchiveInvalid, "compressed size")
	if _, err := os.Stat(upload); err != nil {
		t.Fatalf("upload was removed: %v", err)
	}
}

func TestRunModsImportAppliesRetention(t *testing.T) {
	root := modsImportRoot(t)
	fixModsImportNow(t, modsImportTestNow)
	modsDir := filepath.Join(root, "backups", "mods")

	old := make([]string, 0, 5)
	for hour := 7; hour <= 11; hour++ {
		name := fixtures.ModsName(modsImportTestNow.Add(
			time.Duration(hour-12) * time.Hour))
		old = append(old, name)
		fixtures.WriteFile(t, filepath.Join(modsDir, name), "old")
	}

	upload, _ := writeModsUpload(t, root, fixtures.DefaultMods())
	code, data, stdout, stderr := runModsImportCommand(t, root, "--file", upload)
	if code != int(ops.ExitOK) {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q", code, stdout, stderr)
	}
	if len(data.Pruned) != 1 || data.Pruned[0] != old[0] {
		t.Fatalf("pruned = %v, want only %s", data.Pruned, old[0])
	}
	if _, err := os.Stat(filepath.Join(modsDir, old[0])); !os.IsNotExist(err) {
		t.Fatalf("oldest archive still exists: %v", err)
	}
	for _, name := range old[1:] {
		if _, err := os.Stat(filepath.Join(modsDir, name)); err != nil {
			t.Fatalf("kept archive %s: %v", name, err)
		}
	}
}

func TestRunModsImportLocked(t *testing.T) {
	root := modsImportRoot(t)
	upload, _ := writeModsUpload(t, root, fixtures.DefaultMods())

	held, err := lock.Acquire(filepath.Join(root, ".locks", "ops.lock"),
		lock.Exclusive, 0)
	if err != nil {
		t.Fatalf("hold lock: %v", err)
	}
	defer func() { _ = held.Close() }()

	code, _, stdout, _ := runModsImportCommand(t, root,
		"--lock-timeout=0", "--file", upload)
	if code != int(ops.ExitLocked) {
		t.Fatalf("exit = %d, want 4; stdout=%q", code, stdout)
	}
	assertJSONError(t, stdout, ops.ExitLocked, "lock")
	if _, err := os.Stat(upload); err != nil {
		t.Fatalf("upload was removed: %v", err)
	}
}

func TestRunModsImportDryRunWritesNothing(t *testing.T) {
	root := t.TempDir()
	fixModsImportNow(t, modsImportTestNow)
	upload, body := writeModsUpload(t, root, fixtures.ScriptedMods())

	code, data, stdout, stderr := runModsImportCommand(t, root,
		"--dry-run", "--file", upload)
	if code != int(ops.ExitOK) {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q", code, stdout, stderr)
	}
	if data.Archive != fixtures.ModsName(modsImportTestNow) {
		t.Fatalf("archive = %q, want dry-run name", data.Archive)
	}
	if data.SHA256 != fixtures.SHA256Hex(body) ||
		data.Plugins != 1 || data.Scripts != 2 {
		t.Fatalf("dry-run data = %+v, want hash and counts", data)
	}
	if _, err := os.Stat(upload); err != nil {
		t.Fatalf("dry-run removed upload: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "backups")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created backups directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".locks")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created .locks directory: %v", err)
	}
}

func TestRunModsImportRejectsSymlinkDestination(t *testing.T) {
	root := modsImportRoot(t)
	modsDir := filepath.Join(root, "backups", "mods")
	if err := os.Remove(modsDir); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.Mkdir(outside, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, modsDir); err != nil {
		t.Skipf("symlink is unavailable: %v", err)
	}
	upload, _ := writeModsUpload(t, root, fixtures.DefaultMods())

	code, _, stdout, _ := runModsImportCommand(t, root, "--file", upload)
	if code != int(ops.ExitInternal) {
		t.Fatalf("exit = %d, want 1; stdout=%q", code, stdout)
	}
	assertJSONError(t, stdout, ops.ExitInternal, "not a directory")
	if _, err := os.Stat(upload); err != nil {
		t.Fatalf("upload was removed: %v", err)
	}
	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("symlink target contains %d entries, want none", len(entries))
	}
}

func TestModsImportErrorMapping(t *testing.T) {
	tests := []struct {
		name string
		err  error
		exit ops.ExitCode
	}{
		{name: "invalid", err: archive.ErrInvalid, exit: ops.ExitArchiveInvalid},
		{name: "limit", err: archive.ErrLimit, exit: ops.ExitArchiveInvalid},
		{name: "unsafe", err: pathguard.ErrUnsafePath, exit: ops.ExitUnsafePath},
		{name: "no space", err: syscall.ENOSPC, exit: ops.ExitNoSpace},
		{name: "cross device", err: syscall.EXDEV, exit: ops.ExitInvalidArgument},
		{name: "other", err: errors.New("boom"), exit: ops.ExitInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := modsImportError("test", tt.err)
			if err.Exit != tt.exit {
				t.Fatalf("exit = %d, want %d", err.Exit, tt.exit)
			}
			if !strings.Contains(err.Message, "mods import: test") {
				t.Fatalf("message = %q, want operation prefix", err.Message)
			}
		})
	}
}

func TestWriteModsImportHuman(t *testing.T) {
	var out bytes.Buffer
	err := writeModsImportHuman(&out, modsImportData{
		Archive: "mods-2026-10-22_12-00-00.tar.gz",
		SHA256:  strings.Repeat("a", 64),
		Plugins: 3,
		Scripts: 2,
		Pruned:  []string{"mods-2026-10-21_12-00-00.tar.gz"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"archive: mods-2026-10-22_12-00-00.tar.gz",
		"plugins: 3", "scripts: 2",
		"pruned: mods-2026-10-21_12-00-00.tar.gz",
		"warnings: -",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output = %q, want %q", out.String(), want)
		}
	}
}

func TestRunModsSubcommands(t *testing.T) {
	tests := []struct {
		name string
		args []string
		exit ops.ExitCode
		want string
	}{
		{
			name: "missing subcommand",
			args: []string{"--json", "mods"},
			exit: ops.ExitInvalidArgument, want: "subcommand is required",
		},
		{
			name: "unknown subcommand",
			args: []string{"--json", "mods", "bogus"},
			exit: ops.ExitInvalidArgument, want: "unknown subcommand",
		},
		{
			name: "deploy requires a selector",
			args: []string{"--json", "mods", "deploy"},
			exit: ops.ExitInvalidArgument, want: "one of --archive, --latest or --current",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)
			if code != int(tt.exit) {
				t.Fatalf("exit = %d, want %d; stdout=%q", code, tt.exit, stdout.String())
			}
			assertJSONError(t, stdout.String(), tt.exit, tt.want)
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}
