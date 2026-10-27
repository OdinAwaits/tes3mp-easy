//go:build linux && !race

package resource

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
	"github.com/Saagilat/tes3mp-easy/test/fixtures"
)

const (
	smallPluginSize = 8 << 20
	largePluginSize = 510 << 20
	smallStateSize  = 1 << 20
	largeStateSize  = 500 << 20
)

func TestModsImportMemoryIsBounded(t *testing.T) {
	cg := newTestCgroup(t)
	small := measureModsImport(t, smallPluginSize, nil)
	large := measureModsImport(t, largePluginSize, cg)
	checkMemory(t, "mods import", small, large)
}

func TestBackupsCreateMemoryIsBounded(t *testing.T) {
	cg := newTestCgroup(t)
	small := measureStateBackup(t, smallStateSize, nil)
	large := measureStateBackup(t, largeStateSize, cg)
	checkMemory(t, "backups create", small, large)
}

func measureModsImport(t *testing.T, payloadSize int64, cg *cgroup) measurement {
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
	// The upload, its compressed copy and retention slack must fit.
	requireSpace(t, root, payloadSize+1<<30)

	upload := filepath.Join(root, "panel-data", "uploads", "upload.tar.gz")
	size := writeModsArchive(t, upload, payloadSize)
	if size > archive.MaxCompressedSize {
		t.Fatalf("generated archive is %s, limit is %s",
			formatBytes(uint64(size)), formatBytes(uint64(archive.MaxCompressedSize)))
	}

	m := runMeasured(t, root, cg,
		"--json", "mods", "import", "--file", upload)
	m.requireSuccess(t)
	if _, err := os.Stat(upload); !os.IsNotExist(err) {
		t.Fatalf("upload still exists after import: %v", err)
	}
	return m
}

func measureStateBackup(t *testing.T, stateSize int64, cg *cgroup) measurement {
	t.Helper()

	root := stateResourceRoot(t, stateSize)
	// The live file and the compressed archive are both in the root.
	requireSpace(t, root, stateSize+1<<30)

	m := runMeasured(t, root, cg,
		"--json", "backups", "create", "--type", "state")
	m.requireSuccess(t)

	entries, err := os.ReadDir(filepath.Join(root, "backups", "state"))
	if err != nil {
		t.Fatalf("read backups/state: %v", err)
	}
	archives := 0
	for _, entry := range entries {
		switch filepath.Ext(entry.Name()) {
		case ".gz":
			archives++
		case ".part":
			t.Fatalf("backups create left %s", entry.Name())
		}
	}
	if archives != 1 {
		t.Fatalf("state archives = %d, want 1", archives)
	}
	return m
}

func stateResourceRoot(t *testing.T, stateSize int64) string {
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
	fixtures.WriteConfigs(t, filepath.Join(root, "configs"))
	worldDirs := []string{"cell", "world", "map", "recordstore", "custom"}
	for _, name := range worldDirs {
		if err := os.MkdirAll(filepath.Join(root, "world", name), 0o750); err != nil {
			t.Fatalf("create world/%s: %v", name, err)
		}
	}

	mods := fixtures.DefaultModsInit()
	mods.Install(t, filepath.Join(root, "backups", "mods"))
	fixtures.WriteFile(t,
		filepath.Join(root, "mods", "plugins", fixtures.MetadataName),
		string(mods.RequiredJSON()))
	fixtures.WriteFile(t, filepath.Join(root, "players", "player.json"), "{}\n")
	for _, name := range worldDirs {
		fixtures.WriteFile(t,
			filepath.Join(root, "world", name, name+".json"), "{}\n")
	}
	writePseudoRandomFile(t,
		filepath.Join(root, "world", "custom", "large.bin"), stateSize)
	return root
}
