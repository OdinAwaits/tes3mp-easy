package ops

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
	"github.com/Saagilat/tes3mp-easy/test/fixtures"
)

type modsImportPayload struct {
	Archive  string   `json:"archive"`
	SHA256   string   `json:"sha256"`
	Plugins  int      `json:"plugins"`
	Scripts  int      `json:"scripts"`
	Pruned   []string `json:"pruned"`
	Warnings []string `json:"warnings"`
}

func modsImportContractRoot(t *testing.T) string {
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

func writeContractModsUpload(t *testing.T, root string,
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

func decodeModsImport(t *testing.T, env envelope) modsImportPayload {
	t.Helper()

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(env.Data, &raw); err != nil {
		t.Fatalf("decode import data: %v; data=%s", err, env.Data)
	}
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	want := []string{
		"archive", "plugins", "pruned", "scripts", "sha256", "warnings",
	}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("import keys = %v, want %v", keys, want)
	}

	var data modsImportPayload
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("decode import payload: %v", err)
	}
	return data
}

func TestModsImportContract(t *testing.T) {
	root := modsImportContractRoot(t)
	modsDir := filepath.Join(root, "backups", "mods")
	fixtures.DefaultModsInit().Install(t, modsDir)
	currentBefore := fixtures.ReadCurrent(t, modsDir)

	mods := fixtures.ScriptedMods()
	upload, body := writeContractModsUpload(t, root, mods)
	res := testHarness.RunRoot(t, root, "--json", "mods", "import",
		"--file", upload)
	env := requireReadSuccess(t, res,
		[]string{"archive", "plugins", "pruned", "scripts", "sha256", "warnings"})
	data := decodeModsImport(t, env)

	parsed, err := archive.ParseArchiveName(data.Archive)
	if err != nil {
		t.Fatalf("archive name: %v", err)
	}
	if parsed.Kind != archive.KindMods {
		t.Fatalf("kind = %s, want mods", parsed.Kind)
	}
	if data.SHA256 != fixtures.SHA256Hex(body) {
		t.Fatalf("sha256 = %s, want %s", data.SHA256, fixtures.SHA256Hex(body))
	}
	if data.Plugins != 1 || data.Scripts != 2 {
		t.Fatalf("plugins/scripts = %d/%d, want 1/2", data.Plugins, data.Scripts)
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
	if currentAfter := fixtures.ReadCurrent(t, modsDir); currentAfter != currentBefore {
		t.Fatalf("current.txt changed: %q -> %q", currentBefore, currentAfter)
	}
	if _, err := os.Stat(filepath.Join(root, ".locks", "ops.lock")); err != nil {
		t.Fatalf("ops lock was not created: %v", err)
	}
}

func TestModsImportInvalidArchiveContract(t *testing.T) {
	rootJSON := fixtures.DefaultMods()
	rootJSON.MetadataPath = fixtures.MetadataName
	badCRC := fixtures.ModsArchive{
		Plugins: []fixtures.Plugin{{
			Name: "mod.esp", Body: "synthetic",
			CRCs: []string{"0x00000000"},
		}},
	}
	traversal := fixtures.DefaultMods()
	traversal.Extra = []fixtures.Entry{{
		Name: "../escape", Body: "escape",
	}}

	tests := []struct {
		name string
		mods fixtures.ModsArchive
		exit int
		code string
	}{
		{name: "root json", mods: rootJSON, exit: 7, code: "archive_invalid"},
		{name: "bad crc", mods: badCRC, exit: 7, code: "archive_invalid"},
		{name: "traversal", mods: traversal, exit: 8, code: "unsafe_path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := modsImportContractRoot(t)
			upload, _ := writeContractModsUpload(t, root, tt.mods)
			res := testHarness.RunRoot(t, root, "--json", "mods", "import",
				"--file", upload)
			res.requireJSONError(t, tt.exit, tt.code)

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

func TestModsImportMissingUploadContract(t *testing.T) {
	root := modsImportContractRoot(t)
	missing := filepath.Join(root, "panel-data", "uploads", "missing.tar.gz")
	res := testHarness.RunRoot(t, root, "--json", "mods", "import",
		"--file", missing)
	res.requireJSONError(t, 3, "not_found")
}

func TestModsImportLockedContract(t *testing.T) {
	root := modsImportContractRoot(t)
	upload, _ := writeContractModsUpload(t, root, fixtures.DefaultMods())
	holdOpsLock(t, root)

	res := testHarness.RunRoot(t, root, "--lock-timeout=0",
		"--json", "mods", "import", "--file", upload)
	res.requireJSONError(t, 4, "locked")
	if _, err := os.Stat(upload); err != nil {
		t.Fatalf("upload was removed after lock failure: %v", err)
	}
}

func TestModsImportDryRunContract(t *testing.T) {
	root := t.TempDir()
	mods := fixtures.ScriptedMods()
	upload, body := writeContractModsUpload(t, root, mods)

	before := snapshotTree(t, root)
	res := testHarness.RunRoot(t, root, "--dry-run",
		"--json", "mods", "import", "--file", upload)
	env := requireReadSuccess(t, res,
		[]string{"archive", "plugins", "pruned", "scripts", "sha256", "warnings"})
	data := decodeModsImport(t, env)
	if data.Archive == "" || data.SHA256 != fixtures.SHA256Hex(body) {
		t.Fatalf("dry-run data = %+v, want name and hash", data)
	}
	if data.Plugins != 1 || data.Scripts != 2 {
		t.Fatalf("plugins/scripts = %d/%d, want 1/2", data.Plugins, data.Scripts)
	}

	after := snapshotTree(t, root)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("dry-run changed temp root\nbefore: %#v\nafter:  %#v",
			before, after)
	}
}
