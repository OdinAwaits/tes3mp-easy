package fixtures

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"
)

const (
	// MetadataName is the required mods metadata file.
	MetadataName = "requiredDataFiles.json"
	// CurrentName is the current mods metadata file.
	CurrentName = "current.txt"
)

// RequiredFile is one entry of requiredDataFiles.json.
type RequiredFile struct {
	Name string
	CRCs []string
}

// Plugin is one synthetic plugin in a mods archive. A nil CRCs slice means
// "compute the CRC from Body"; an empty non-nil slice means "presence only".
type Plugin struct {
	Name string
	Body string
	CRCs []string
}

// Script is one synthetic Lua script in a mods archive.
type Script struct {
	Name string
	Body string
}

// ModsArchive describes a synthetic mods tar.gz archive.
type ModsArchive struct {
	// Name is the archive filename. The default is ModsName(DefaultTime).
	Name string
	// Plugins are written to plugins/ and included in requiredDataFiles.json.
	Plugins []Plugin
	// Scripts are written to scripts/.
	Scripts []Script
	// Required overrides the generated requiredDataFiles.json entries.
	Required []RequiredFile
	// MetadataPath defaults to plugins/requiredDataFiles.json. Set it to
	// requiredDataFiles.json for the invalid root-json fixture.
	MetadataPath string
	// OmitMetadata removes requiredDataFiles.json from the archive.
	OmitMetadata bool
	// IncludeScriptsDir adds an empty scripts/ directory even without scripts.
	IncludeScriptsDir bool
	// Extra entries are appended verbatim for failure-mode fixtures.
	Extra []Entry
}

// ModsManifest describes an installed mods archive.
type ModsManifest struct {
	Name     string
	SHA256   string
	Required []RequiredFile
}

// DefaultMods returns the minimal valid mods archive used by most tests.
func DefaultMods() ModsArchive {
	return ModsArchive{
		Plugins: []Plugin{{
			Name: "mod.esp",
			Body: "synthetic mod data\n",
		}},
	}
}

// ScriptedMods returns a valid mods archive with one plugin and two scripts.
func ScriptedMods() ModsArchive {
	mods := DefaultMods()
	mods.Scripts = []Script{
		{Name: "test.lua", Body: "print(\"synthetic\")\n"},
		{Name: "other_1-Lua.lua", Body: "print(\"synthetic 2\")\n"},
	}
	return mods
}

// DefaultModsInit returns the valid bootstrap archive with only the three base
// ESM requirements and an empty scripts/ directory.
func DefaultModsInit() ModsArchive {
	return ModsArchive{
		Name:              ModsInitName(DefaultTime),
		IncludeScriptsDir: true,
	}
}

// ArchiveName returns the effective archive filename.
func (m ModsArchive) ArchiveName() string {
	if m.Name != "" {
		return m.Name
	}
	return ModsName(DefaultTime)
}

// Entries returns archive entries in deterministic order.
func (m ModsArchive) Entries() []Entry {
	entries := []Entry{{Name: "plugins/", Type: tar.TypeDir}}
	for _, plugin := range m.Plugins {
		entries = append(entries, Entry{
			Name: "plugins/" + plugin.Name,
			Body: plugin.Body,
		})
	}

	if !m.OmitMetadata {
		metadataPath := m.MetadataPath
		if metadataPath == "" {
			metadataPath = "plugins/" + MetadataName
		}
		entries = append(entries, Entry{
			Name: metadataPath,
			Body: string(m.requiredJSON()),
		})
	}

	if len(m.Scripts) > 0 || m.IncludeScriptsDir {
		entries = append(entries, Entry{Name: "scripts/", Type: tar.TypeDir})
	}
	for _, script := range m.Scripts {
		entries = append(entries, Entry{
			Name: "scripts/" + script.Name,
			Body: script.Body,
		})
	}
	entries = append(entries, m.Extra...)
	return entries
}

// Bytes returns the gzip-compressed archive.
func (m ModsArchive) Bytes(t testing.TB) []byte {
	t.Helper()
	return TarGz(t, m.Entries())
}

// Write writes the archive to path.
func (m ModsArchive) Write(t testing.TB, path string) {
	t.Helper()
	WriteTarGz(t, path, m.Entries())
}

// Manifest returns the archive metadata for an already-built archive body.
func (m ModsArchive) Manifest(body []byte) ModsManifest {
	return ModsManifest{
		Name:     m.ArchiveName(),
		SHA256:   SHA256Hex(body),
		Required: m.requiredFiles(),
	}
}

// Install writes the archive and a canonical current.txt into dir, returning
// the metadata needed by state fixtures.
func (m ModsArchive) Install(t testing.TB, dir string) ModsManifest {
	t.Helper()

	body := m.Bytes(t)
	manifest := m.Manifest(body)
	WriteFile(t, filepath.Join(dir, manifest.Name), string(body))
	WriteCurrent(t, dir, manifest)
	return manifest
}

// SHA256Hex returns the lowercase hex SHA-256 of data.
func SHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// CurrentLine returns the only accepted current.txt representation.
func CurrentLine(sha, name string) string {
	return sha + " " + name + "\n"
}

// CRC32Hex returns an upper-case 0x-prefixed IEEE CRC32 value.
func CRC32Hex(body string) string {
	return fmt.Sprintf("0x%08X", crc32.ChecksumIEEE([]byte(body)))
}

func (m ModsArchive) requiredFiles() []RequiredFile {
	if m.Required != nil {
		return cloneRequired(m.Required)
	}

	required := []RequiredFile{
		{Name: "Morrowind.esm", CRCs: []string{}},
		{Name: "Tribunal.esm", CRCs: []string{}},
		{Name: "Bloodmoon.esm", CRCs: []string{}},
	}
	for _, plugin := range m.Plugins {
		crcs := plugin.CRCs
		if crcs == nil {
			crcs = []string{CRC32Hex(plugin.Body)}
		}
		required = append(required, RequiredFile{
			Name: plugin.Name,
			CRCs: append([]string(nil), crcs...),
		})
	}
	return required
}

func (m ModsArchive) requiredJSON() []byte {
	return requiredJSONFrom(m.requiredFiles())
}

func requiredJSONFrom(required []RequiredFile) []byte {
	objects := make([]map[string][]string, 0, len(required))
	for _, file := range required {
		crcs := file.CRCs
		if crcs == nil {
			crcs = []string{}
		}
		objects = append(objects, map[string][]string{file.Name: crcs})
	}
	data, err := json.Marshal(objects)
	if err != nil {
		panic("fixtures: marshal requiredDataFiles.json: " + err.Error())
	}
	return data
}

func cloneRequired(required []RequiredFile) []RequiredFile {
	cloned := make([]RequiredFile, 0, len(required))
	for _, file := range required {
		cloned = append(cloned, RequiredFile{
			Name: file.Name,
			CRCs: append([]string(nil), file.CRCs...),
		})
	}
	return cloned
}

// RequiredJSON exposes the generated metadata for tests that inspect it.
func (m ModsArchive) RequiredJSON() []byte {
	return m.requiredJSON()
}

// WriteCurrent writes current.txt into dir.
func WriteCurrent(t testing.TB, dir string, manifest ModsManifest) {
	t.Helper()
	WriteFile(t, filepath.Join(dir, CurrentName),
		CurrentLine(manifest.SHA256, manifest.Name))
}

// ReadCurrent is a small helper for tests that need the raw current.txt.
func ReadCurrent(t testing.TB, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, CurrentName))
	if err != nil {
		t.Fatalf("fixtures: read current.txt: %v", err)
	}
	return string(data)
}
