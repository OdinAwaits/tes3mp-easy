//go:build linux

package ops

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/Saagilat/tes3mp-easy/test/fixtures"
)

func TestModsImportCrossFilesystemContract(t *testing.T) {
	if _, err := os.Stat("/dev/shm"); err != nil {
		t.Skipf("/dev/shm is unavailable: %v", err)
	}

	root := modsImportContractRoot(t)
	shm, err := os.MkdirTemp("/dev/shm", "tes3mp-easy-import-*")
	if err != nil {
		t.Skipf("create /dev/shm directory: %v", err)
	}
	defer os.RemoveAll(shm)

	upload := filepath.Join(shm, "upload.tar.gz")
	fixtures.DefaultMods().Write(t, upload)

	device := func(path string) uint64 {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			t.Fatalf("%s has no syscall stat", path)
		}
		return uint64(stat.Dev)
	}
	if device(upload) == device(filepath.Join(root, "backups", "mods")) {
		t.Skip("/dev/shm and temp root share a filesystem")
	}

	res := testHarness.RunRoot(t, root, "--json", "mods", "import",
		"--file", upload)
	res.requireJSONError(t, 2, "invalid_argument")
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
