package fixtures_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/archive"
	"github.com/Saagilat/tes3mp-easy/test/fixtures"
)

func TestModsFixturesAreAccepted(t *testing.T) {
	tests := []struct {
		name string
		mods fixtures.ModsArchive
		want archive.ModsInfo
	}{
		{
			name: "minimal",
			mods: fixtures.DefaultMods(),
			want: archive.ModsInfo{Plugins: 1, Scripts: 0},
		},
		{
			name: "scripts",
			mods: fixtures.ScriptedMods(),
			want: archive.ModsInfo{Plugins: 1, Scripts: 2},
		},
		{
			name: "mods-init",
			mods: fixtures.DefaultModsInit(),
			want: archive.ModsInfo{Plugins: 0, Scripts: 0},
		},
		{
			name: "no-crc",
			mods: fixtures.ModsArchive{
				Required: []fixtures.RequiredFile{
					{Name: "Morrowind.esm"},
					{Name: "Tribunal.esm"},
					{Name: "Bloodmoon.esm"},
					{Name: "mod.esp", CRCs: []string{}},
				},
				Plugins: []fixtures.Plugin{{Name: "mod.esp", Body: "synthetic"}},
			},
			want: archive.ModsInfo{Plugins: 1, Scripts: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := archive.ValidateModsReader(bytes.NewReader(tt.mods.Bytes(t)))
			if err != nil {
				t.Fatalf("ValidateModsReader: %v", err)
			}
			if info != tt.want {
				t.Fatalf("info = %+v, want %+v", info, tt.want)
			}
		})
	}
}

func TestModsFailureFixturesAreRejected(t *testing.T) {
	tests := []struct {
		name string
		mods fixtures.ModsArchive
	}{
		{
			name: "bad-crc",
			mods: fixtures.ModsArchive{
				Required: []fixtures.RequiredFile{
					{Name: "Morrowind.esm"},
					{Name: "Tribunal.esm"},
					{Name: "Bloodmoon.esm"},
					{Name: "mod.esp", CRCs: []string{"0x00000000"}},
				},
				Plugins: []fixtures.Plugin{{Name: "mod.esp", Body: "synthetic"}},
			},
		},
		{
			name: "root-json",
			mods: func() fixtures.ModsArchive {
				mods := fixtures.DefaultMods()
				mods.MetadataPath = fixtures.MetadataName
				return mods
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := archive.ValidateModsReader(bytes.NewReader(tt.mods.Bytes(t)))
			if !errors.Is(err, archive.ErrInvalid) {
				t.Fatalf("error = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestStateFixtures(t *testing.T) {
	modsDir := t.TempDir()
	manifest := fixtures.DefaultMods().Install(t, modsDir)

	t.Run("empty", func(t *testing.T) {
		state := fixtures.DefaultState(manifest)
		info, err := archive.ValidateStateReader(
			bytes.NewReader(state.Bytes(t)), modsDir)
		if err != nil {
			t.Fatalf("ValidateStateReader: %v", err)
		}
		if info.Current.Name != manifest.Name || info.Players != 0 ||
			info.ConfigFiles != 1 {
			t.Fatalf("info = %+v", info)
		}
	})

	t.Run("full", func(t *testing.T) {
		state := fixtures.FullState(manifest)
		info, err := archive.ValidateStateReader(
			bytes.NewReader(state.Bytes(t)), modsDir)
		if err != nil {
			t.Fatalf("ValidateStateReader: %v", err)
		}
		if info.Players != 1 || info.WorldFiles != 5 {
			t.Fatalf("info = %+v, want one player and five world files", info)
		}
	})

	t.Run("flat-world", func(t *testing.T) {
		state := fixtures.DefaultState(manifest)
		state.FlatWorld = true
		if _, err := archive.ValidateStateReader(
			bytes.NewReader(state.Bytes(t)), modsDir); !errors.Is(err, archive.ErrInvalid) {
			t.Fatalf("error = %v, want ErrInvalid", err)
		}
	})

	t.Run("no-current", func(t *testing.T) {
		state := fixtures.DefaultState(manifest)
		state.OmitCurrent = true
		if _, err := archive.ValidateStateReader(
			bytes.NewReader(state.Bytes(t)), modsDir); !errors.Is(err, archive.ErrInvalid) {
			t.Fatalf("error = %v, want ErrInvalid", err)
		}
	})
}

func TestInstallWritesCanonicalCurrent(t *testing.T) {
	dir := t.TempDir()
	manifest := fixtures.DefaultMods().Install(t, dir)

	data := fixtures.ReadCurrent(t, dir)
	want := fixtures.CurrentLine(manifest.SHA256, manifest.Name)
	if data != want {
		t.Fatalf("current.txt = %q, want %q", data, want)
	}
	if _, err := archive.ReadCurrentFile(filepath.Join(dir, fixtures.CurrentName)); err != nil {
		t.Fatalf("ReadCurrentFile: %v", err)
	}
}

func TestConfigFixtures(t *testing.T) {
	dir := t.TempDir()
	fixtures.WriteConfigs(t, dir)

	want := []string{
		"tes3mp-server-default.cfg",
		"config.lua",
		"banlist.json",
	}
	got := fixtures.DefaultConfigs()
	if len(got) != len(want) {
		t.Fatalf("configs = %d, want %d", len(got), len(want))
	}
	for i, file := range got {
		if file.Name != want[i] {
			t.Fatalf("config[%d].Name = %q, want %q", i, file.Name, want[i])
		}
		if file.Body == "" {
			t.Fatalf("config %s is empty", file.Name)
		}
	}

	var banlist struct {
		PlayerNames []string `json:"playerNames"`
		IPAddresses []string `json:"ipAddresses"`
	}
	if err := json.Unmarshal([]byte(fixtures.DefaultBanlist().Body), &banlist); err != nil {
		t.Fatalf("banlist JSON: %v", err)
	}
	if len(banlist.PlayerNames) != 0 || len(banlist.IPAddresses) != 0 {
		t.Fatalf("banlist = %+v, want empty lists", banlist)
	}
}

func TestAPIFixtures(t *testing.T) {
	server := fixtures.DefaultServer()
	data := fixtures.JSON(t, server)

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("server JSON: %v", err)
	}
	if _, ok := raw["backend"]; ok {
		t.Fatal("server fixture contains forbidden backend field")
	}
	if raw["mods_sha256"] != nil {
		t.Fatalf("mods_sha256 = %#v, want null", raw["mods_sha256"])
	}

	manifest := fixtures.ModsManifest{
		Name:   fixtures.ModsName(fixtures.DefaultTime),
		SHA256: "abc",
	}
	current := fixtures.DefaultModsCurrent(manifest, 123)
	if current.Name != manifest.Name || current.Size != 123 ||
		current.URL != "/api/v1/mods/current/archive" {
		t.Fatalf("current = %+v", current)
	}

	errorsList := fixtures.DefaultAPIErrors()
	if len(errorsList) != 5 {
		t.Fatalf("api errors = %d, want 5", len(errorsList))
	}
}
