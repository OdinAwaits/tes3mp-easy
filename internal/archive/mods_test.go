package archive

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/pathguard"
)

func modCRC(body string) string {
	return fmt.Sprintf("0x%08X", crc32.ChecksumIEEE([]byte(body)))
}

func requiredJSON(entries ...string) string {
	return "[" + strings.Join(entries, ",") + "]"
}

func requiredEntry(name string, crcs ...string) string {
	if crcs == nil {
		crcs = []string{}
	}
	data, err := json.Marshal(map[string][]string{name: crcs})
	if err != nil {
		panic(err)
	}
	return string(data)
}

func baseRequiredEntries() []string {
	return []string{
		requiredEntry("Morrowind.esm"),
		requiredEntry("Tribunal.esm"),
		requiredEntry("Bloodmoon.esm"),
	}
}

func validateTarGz(t *testing.T, entries []fixtureEntry) (ModsInfo, error) {
	t.Helper()

	reader, err := NewReader(bytes.NewReader(makeTarGz(t, entries)))
	if err != nil {
		return ModsInfo{}, err
	}
	defer reader.Close()
	return ValidateMods(reader)
}

func TestValidateModsAcceptsMinimalArchive(t *testing.T) {
	const body = "mod data"
	entries := []fixtureEntry{
		{name: "plugins/", typ: tar.TypeDir},
		{name: "plugins/mod.esp", body: body},
		{name: "plugins/requiredDataFiles.json", body: requiredJSON(
			append(baseRequiredEntries(), requiredEntry("mod.esp", modCRC(body)))...,
		)},
	}

	info, err := validateTarGz(t, entries)
	if err != nil {
		t.Fatalf("ValidateMods: %v", err)
	}
	if info.Plugins != 1 || info.Scripts != 0 {
		t.Fatalf("info = %+v, want one plugin and no scripts", info)
	}
}

func TestValidateModsAcceptsScripts(t *testing.T) {
	const body = "mod data"
	entries := []fixtureEntry{
		{name: "plugins/", typ: tar.TypeDir},
		{name: "plugins/mod.omwaddon", body: body},
		{name: "plugins/requiredDataFiles.json", body: requiredJSON(
			append(baseRequiredEntries(), requiredEntry("mod.omwaddon", modCRC(body)))...,
		)},
		{name: "scripts/", typ: tar.TypeDir},
		{name: "scripts/test.lua", body: "print(1)"},
		{name: "scripts/other_1-Lua.lua", body: "print(2)"},
	}

	info, err := validateTarGz(t, entries)
	if err != nil {
		t.Fatalf("ValidateMods: %v", err)
	}
	if info.Plugins != 1 || info.Scripts != 2 {
		t.Fatalf("info = %+v, want one plugin and two scripts", info)
	}
}

func TestValidateModsAcceptsNoCRC(t *testing.T) {
	entries := []fixtureEntry{
		{name: "plugins/mod.esp", body: "mod data"},
		{name: "plugins/requiredDataFiles.json", body: requiredJSON(
			append(baseRequiredEntries(), requiredEntry("mod.esp"))...,
		)},
	}

	if _, err := validateTarGz(t, entries); err != nil {
		t.Fatalf("ValidateMods: %v", err)
	}
}

func TestValidateModsAcceptsAnyMatchingCRC(t *testing.T) {
	const body = "mod data"
	entries := []fixtureEntry{
		{name: "plugins/mod.esp", body: body},
		{name: "plugins/requiredDataFiles.json", body: requiredJSON(
			append(baseRequiredEntries(),
				requiredEntry("mod.esp", "0x00000000", modCRC(body)))...,
		)},
	}

	if _, err := validateTarGz(t, entries); err != nil {
		t.Fatalf("ValidateMods: %v", err)
	}
}

func TestValidateModsAcceptsInitArchive(t *testing.T) {
	entries := []fixtureEntry{
		{name: "plugins/", typ: tar.TypeDir},
		{name: "plugins/requiredDataFiles.json", body: requiredJSON(baseRequiredEntries()...)},
		{name: "scripts/", typ: tar.TypeDir},
	}

	info, err := validateTarGz(t, entries)
	if err != nil {
		t.Fatalf("ValidateMods: %v", err)
	}
	if info.Plugins != 0 || info.Scripts != 0 {
		t.Fatalf("info = %+v, want empty archive", info)
	}
}

func TestValidateModsCaseInsensitiveExtension(t *testing.T) {
	const body = "mod data"
	entries := []fixtureEntry{
		{name: "plugins/MOD.ESP", body: body},
		{name: "plugins/requiredDataFiles.json", body: requiredJSON(
			append(baseRequiredEntries(), requiredEntry("MOD.ESP", modCRC(body)))...,
		)},
	}

	if _, err := validateTarGz(t, entries); err != nil {
		t.Fatalf("ValidateMods: %v", err)
	}
}

func TestValidateModsFileAndReader(t *testing.T) {
	const body = "mod data"
	entries := []fixtureEntry{
		{name: "plugins/mod.esp", body: body},
		{name: "plugins/requiredDataFiles.json", body: requiredJSON(
			append(baseRequiredEntries(), requiredEntry("mod.esp", modCRC(body)))...,
		)},
	}
	data := makeTarGz(t, entries)

	if _, err := ValidateModsReader(bytes.NewReader(data)); err != nil {
		t.Fatalf("ValidateModsReader: %v", err)
	}

	file := filepath.Join(t.TempDir(), "mods.tar.gz")
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := ValidateModsFile(file); err != nil {
		t.Fatalf("ValidateModsFile: %v", err)
	}
}

func TestParseRequiredDataFiles(t *testing.T) {
	data := requiredJSON(append(baseRequiredEntries(),
		requiredEntry("mod.esp", "0x1A2B3C4D", "0x00000001"))...)

	files, err := ParseRequiredDataFiles([]byte(data))
	if err != nil {
		t.Fatalf("ParseRequiredDataFiles: %v", err)
	}
	if len(files) != 4 {
		t.Fatalf("len(files) = %d, want 4", len(files))
	}
	if files[0].Name != "Morrowind.esm" || len(files[0].CRCs) != 0 {
		t.Fatalf("files[0] = %+v", files[0])
	}
	if files[3].Name != "mod.esp" {
		t.Fatalf("files[3].Name = %q", files[3].Name)
	}
	if len(files[3].CRCs) != 2 ||
		files[3].CRCs[0] != 0x1A2B3C4D ||
		files[3].CRCs[1] != 0x00000001 {
		t.Fatalf("files[3].CRCs = %v", files[3].CRCs)
	}

	empty, err := ParseRequiredDataFiles([]byte("[]"))
	if err != nil {
		t.Fatalf("ParseRequiredDataFiles([]): %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("empty = %+v", empty)
	}
}

func TestValidateModsRejectsInvalidLayout(t *testing.T) {
	const body = "mod data"

	cases := []struct {
		name    string
		entries []fixtureEntry
		wantErr error
		wantMsg string
	}{
		{
			name: "missing metadata",
			entries: []fixtureEntry{
				{name: "plugins/", typ: tar.TypeDir},
				{name: "scripts/", typ: tar.TypeDir},
			},
			wantErr: ErrInvalid,
			wantMsg: "missing",
		},
		{
			name: "metadata in root",
			entries: []fixtureEntry{
				{name: "requiredDataFiles.json", body: "[]"},
			},
			wantErr: ErrInvalid,
			wantMsg: "inside plugins",
		},
		{
			name:    "unexpected top-level file",
			entries: []fixtureEntry{{name: "README.txt", body: "x"}},
			wantErr: ErrInvalid,
			wantMsg: "top-level file",
		},
		{
			name:    "unexpected top-level directory",
			entries: []fixtureEntry{{name: "docs/", typ: tar.TypeDir}},
			wantErr: ErrInvalid,
			wantMsg: "top-level directory",
		},
		{
			name:    "nested plugin directory",
			entries: []fixtureEntry{{name: "plugins/sub/", typ: tar.TypeDir}},
			wantErr: ErrInvalid,
			wantMsg: "nested directories",
		},
		{
			name:    "nested plugin file",
			entries: []fixtureEntry{{name: "plugins/sub/mod.esp", body: body}},
			wantErr: ErrInvalid,
			wantMsg: "nested directories",
		},
		{
			name:    "unsupported plugin extension",
			entries: []fixtureEntry{{name: "plugins/readme.txt", body: "x"}},
			wantErr: ErrInvalid,
			wantMsg: "unsupported plugin extension",
		},
		{
			name:    "empty plugin basename",
			entries: []fixtureEntry{{name: "plugins/.esp", body: "x"}},
			wantErr: ErrInvalid,
			wantMsg: "unsupported plugin extension",
		},
		{
			name:    "nested script file",
			entries: []fixtureEntry{{name: "scripts/sub/test.lua", body: "x"}},
			wantErr: ErrInvalid,
			wantMsg: "nested directories",
		},
		{
			name:    "nested script directory",
			entries: []fixtureEntry{{name: "scripts/sub/", typ: tar.TypeDir}},
			wantErr: ErrInvalid,
			wantMsg: "nested directories",
		},
		{
			name:    "script extension",
			entries: []fixtureEntry{{name: "scripts/test.txt", body: "x"}},
			wantErr: ErrInvalid,
			wantMsg: "must have the .lua extension",
		},
		{
			name:    "script uppercase extension",
			entries: []fixtureEntry{{name: "scripts/test.LUA", body: "x"}},
			wantErr: ErrInvalid,
			wantMsg: "must have the .lua extension",
		},
		{
			name:    "script space",
			entries: []fixtureEntry{{name: "scripts/bad name.lua", body: "x"}},
			wantErr: ErrInvalid,
			wantMsg: "may contain only",
		},
		{
			name:    "script dot",
			entries: []fixtureEntry{{name: "scripts/bad.name.lua", body: "x"}},
			wantErr: ErrInvalid,
			wantMsg: "may contain only",
		},
		{
			name:    "empty script basename",
			entries: []fixtureEntry{{name: "scripts/.lua", body: "x"}},
			wantErr: ErrInvalid,
			wantMsg: "must not be empty",
		},
		{
			name: "base ESM in archive",
			entries: []fixtureEntry{
				{name: "plugins/requiredDataFiles.json", body: "[]"},
				{name: "plugins/Morrowind.esm", body: "base"},
			},
			wantErr: ErrInvalid,
			wantMsg: "base ESM",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := validateTarGz(t, tc.entries)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantMsg != "" && !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("error = %q, want substring %q", err, tc.wantMsg)
			}
		})
	}
}

func TestValidateModsRejectsInvalidMetadata(t *testing.T) {
	const body = "mod data"
	wrongCRC := requiredJSON(append(baseRequiredEntries(),
		requiredEntry("mod.esp", "0x00000000"))...)

	cases := []struct {
		name    string
		entries []fixtureEntry
		wantMsg string
	}{
		{
			name: "plugin missing from archive",
			entries: []fixtureEntry{
				{name: "plugins/requiredDataFiles.json", body: requiredJSON(
					append(baseRequiredEntries(), requiredEntry("mod.esp"))...)},
				{name: "plugins/other.esp", body: body},
			},
			wantMsg: "missing from plugins",
		},
		{
			name: "plugin not listed in metadata",
			entries: []fixtureEntry{
				{name: "plugins/requiredDataFiles.json", body: "[]"},
				{name: "plugins/mod.esp", body: body},
			},
			wantMsg: "not listed",
		},
		{
			name: "CRC mismatch",
			entries: []fixtureEntry{
				{name: "plugins/requiredDataFiles.json", body: wrongCRC},
				{name: "plugins/mod.esp", body: body},
			},
			wantMsg: "CRC mismatch",
		},
		{
			name: "lowercase CRC",
			entries: []fixtureEntry{
				{name: "plugins/requiredDataFiles.json", body: requiredJSON(
					requiredEntry("mod.esp", "0xabcdef12"))},
			},
			wantMsg: "uppercase hex",
		},
		{
			name: "short CRC",
			entries: []fixtureEntry{
				{name: "plugins/requiredDataFiles.json", body: requiredJSON(
					requiredEntry("mod.esp", "0x1234"))},
			},
			wantMsg: "uppercase hex",
		},
		{
			name: "duplicate JSON key",
			entries: []fixtureEntry{
				{name: "plugins/requiredDataFiles.json",
					body: `[{"mod.esp":[],"mod.esp":[]}]`},
			},
			wantMsg: "exactly one key",
		},
		{
			name: "multiple JSON keys",
			entries: []fixtureEntry{
				{name: "plugins/requiredDataFiles.json",
					body: `[{"mod.esp":[],"other.esp":[]}]`},
			},
			wantMsg: "exactly one key",
		},
		{
			name: "JSON top-level object",
			entries: []fixtureEntry{
				{name: "plugins/requiredDataFiles.json", body: `{}`},
			},
			wantMsg: "must be an array",
		},
		{
			name: "JSON null",
			entries: []fixtureEntry{
				{name: "plugins/requiredDataFiles.json", body: `null`},
			},
			wantMsg: "must be an array",
		},
		{
			name: "JSON trailing data",
			entries: []fixtureEntry{
				{name: "plugins/requiredDataFiles.json", body: `[] []`},
			},
			wantMsg: "trailing data",
		},
		{
			name: "empty JSON object",
			entries: []fixtureEntry{
				{name: "plugins/requiredDataFiles.json", body: `[{}]`},
			},
			wantMsg: "exactly one key",
		},
		{
			name: "JSON value not array",
			entries: []fixtureEntry{
				{name: "plugins/requiredDataFiles.json",
					body: `[{"mod.esp":"0x00000000"}]`},
			},
			wantMsg: "array of CRC strings",
		},
		{
			name: "JSON CRC not string",
			entries: []fixtureEntry{
				{name: "plugins/requiredDataFiles.json",
					body: `[{"mod.esp":[1]}]`},
			},
			wantMsg: "CRC value must be a string",
		},
		{
			name: "base ESM after mod",
			entries: []fixtureEntry{
				{name: "plugins/requiredDataFiles.json", body: requiredJSON(
					requiredEntry("mod.esp"),
					requiredEntry("Morrowind.esm"),
				)},
			},
			wantMsg: "base ESM entries",
		},
		{
			name: "path in JSON plugin name",
			entries: []fixtureEntry{
				{name: "plugins/requiredDataFiles.json", body: requiredJSON(
					requiredEntry("../evil.esp"))},
			},
			wantMsg: "invalid plugin name",
		},
		{
			name: "duplicate case-insensitive JSON names",
			entries: []fixtureEntry{
				{name: "plugins/requiredDataFiles.json", body: requiredJSON(
					requiredEntry("mod.esp"),
					requiredEntry("MOD.ESP"),
				)},
			},
			wantMsg: "duplicate plugin name",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := validateTarGz(t, tc.entries)
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("error = %v, want ErrInvalid", err)
			}
			if tc.wantMsg != "" && !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("error = %q, want substring %q", err, tc.wantMsg)
			}
		})
	}
}

func TestValidateModsRejectsArchiveLevelErrors(t *testing.T) {
	cases := []struct {
		name    string
		entries []fixtureEntry
		wantErr error
		wantMsg string
	}{
		{
			name:    "path traversal",
			entries: []fixtureEntry{{name: "../evil", body: "x"}},
			wantErr: pathguard.ErrUnsafePath,
		},
		{
			name: "symlink",
			entries: []fixtureEntry{
				{name: "plugins/mod.esp", typ: tar.TypeSymlink, link: "target"},
			},
			wantErr: ErrInvalid,
		},
		{
			name: "duplicate case-insensitive names",
			entries: []fixtureEntry{
				{name: "plugins/mod.esp", body: "one"},
				{name: "plugins/MOD.ESP", body: "two"},
			},
			wantErr: ErrInvalid,
			wantMsg: "ambiguous",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := validateTarGz(t, tc.entries)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantMsg != "" && !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("error = %q, want substring %q", err, tc.wantMsg)
			}
		})
	}
}
