package gameconfig

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSetLuaScalar(t *testing.T) {
	root := fixtureRoot(t)
	path := filepath.Join(root, "configs", "config.lua")
	now := time.Date(2026, 10, 23, 13, 0, 0, 0, time.UTC)

	tests := []struct {
		key   string
		value string
		want  string
	}{
		{"gameMode", `Morrowind "friends"`, `config.gameMode = "Morrowind \"friends\""`},
		{"loginTime", "45", "config.loginTime = 45"},
		{"passTimeWhenEmpty", "true", "config.passTimeWhenEmpty = true"},
	}
	for _, tt := range tests {
		result, err := Set(root, FileConfig, tt.key, tt.value, now, false)
		if err != nil {
			t.Fatalf("set %s: %v", tt.key, err)
		}
		if result.Key != tt.key {
			t.Fatalf("result key = %q, want %q", result.Key, tt.key)
		}
		data := stringMustRead(t, path)
		if !strings.Contains(data, tt.want) {
			t.Fatalf("config after %s has no %q:\n%s", tt.key, tt.want, data)
		}
	}
	if !strings.Contains(stringMustRead(t, path), "Synthetic test configuration") {
		t.Fatal("comment was lost after config set")
	}

	if _, err := Set(root, FileConfig, "loginTime", "5", now, false); !errors.Is(err, ErrInvalid) {
		t.Fatalf("set out-of-range loginTime error = %v, want ErrInvalid", err)
	}
	if _, err := Set(root, FileConfig, "someTable", "x", now, false); !errors.Is(err, ErrInvalid) {
		t.Fatalf("set table error = %v, want ErrInvalid", err)
	}
}

func TestSetLuaInlineCommentIsPreserved(t *testing.T) {
	root := fixtureRoot(t)
	path := filepath.Join(root, "configs", "config.lua")
	body := "config.loginTime = 60 -- keep me\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if _, err := Set(root, FileConfig, "loginTime", "45",
		time.Date(2026, 10, 23, 13, 0, 0, 0, time.UTC), false); err != nil {
		t.Fatalf("set loginTime: %v", err)
	}
	if got := stringMustRead(t, path); got != "config.loginTime = 45 -- keep me\n" {
		t.Fatalf("config = %q", got)
	}
}

func TestSetDryRunDoesNotWrite(t *testing.T) {
	root := fixtureRoot(t)
	path := filepath.Join(root, "configs", "config.lua")
	before := stringMustRead(t, path)

	if _, err := Set(root, FileConfig, "loginTime", "45",
		time.Date(2026, 10, 23, 14, 0, 0, 0, time.UTC), true); err != nil {
		t.Fatalf("dry-run set: %v", err)
	}
	if after := stringMustRead(t, path); after != before {
		t.Fatal("dry-run changed config file")
	}
	matches, err := filepath.Glob(path + ".bak-*")
	if err != nil {
		t.Fatalf("glob backups: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("dry-run created backups: %v", matches)
	}
}

func TestSetBanlist(t *testing.T) {
	root := fixtureRoot(t)
	now := time.Date(2026, 10, 23, 15, 0, 0, 0, time.UTC)

	result, err := Set(root, FileBanlist, "playerNames",
		`["John Doe","Jane"]`, now, false)
	if err != nil {
		t.Fatalf("set playerNames: %v", err)
	}
	wantNames := []string{"John Doe", "Jane"}
	if !reflect.DeepEqual(result.Value, wantNames) {
		t.Fatalf("result value = %#v, want %#v", result.Value, wantNames)
	}
	if _, err := Set(root, FileBanlist, "ipAddresses",
		"203.0.113.5\n2001:db8::1", now, false); err != nil {
		t.Fatalf("set ipAddresses: %v", err)
	}

	snapshot, err := Read(root, FileBanlist)
	if err != nil {
		t.Fatalf("read banlist: %v", err)
	}
	want := map[string]any{
		"playerNames": wantNames,
		"ipAddresses": []string{"203.0.113.5", "2001:db8::1"},
	}
	if !reflect.DeepEqual(snapshot.Values, want) {
		t.Fatalf("banlist values = %#v, want %#v", snapshot.Values, want)
	}
	if _, err := Set(root, FileBanlist, "ipAddresses", `["not-an-ip"]`,
		now, false); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid IP error = %v, want ErrInvalid", err)
	}
}

func TestReadRejectsSymlink(t *testing.T) {
	root := fixtureRoot(t)
	target := filepath.Join(root, "configs", "config.lua")
	link := filepath.Join(root, "configs", "tes3mp-server-default.cfg")
	if err := os.Remove(link); err != nil {
		t.Fatalf("remove server config: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if _, err := Read(root, FileServer); !errors.Is(err, ErrInvalid) {
		t.Fatalf("symlink server error = %v, want ErrInvalid", err)
	}
}

func TestSetServerPreservesInlineCommentsAndAlias(t *testing.T) {
	root := fixtureRoot(t)
	path := filepath.Join(root, "configs", "tes3mp-server-default.cfg")
	body := `[General]
hostname = Old name ; keep this
port = 25565 # and this
password =
[MasterServer]
enabled = false
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write server config: %v", err)
	}
	snapshot, err := Read(root, FileServer)
	if err != nil {
		t.Fatalf("read server: %v", err)
	}
	if snapshot.Values["hostname"] != "Old name" ||
		snapshot.Values["port"] != 25565 {
		t.Fatalf("inline comments leaked into values: %#v", snapshot.Values)
	}

	now := time.Date(2026, 10, 23, 16, 0, 0, 0, time.UTC)
	result, err := Set(root, FileServer, "enabled", "true", now, false)
	if err != nil {
		t.Fatalf("set alias enabled: %v", err)
	}
	if result.Key != "masterServerEnabled" || result.Value != true {
		t.Fatalf("alias result = %#v", result)
	}
	if _, err := Set(root, FileServer, "port", "25566", now, false); err != nil {
		t.Fatalf("set port: %v", err)
	}
	if _, err := Set(root, FileServer, "hostname", "New name", now, false); err != nil {
		t.Fatalf("set hostname: %v", err)
	}
	got := stringMustRead(t, path)
	for _, want := range []string{
		"hostname = New name ; keep this",
		"port = 25566 # and this",
		"enabled = true",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("server config has no %q:\n%s", want, got)
		}
	}
}

func TestSetBackupCollisionSuffix(t *testing.T) {
	root := fixtureRoot(t)
	path := filepath.Join(root, "configs", "config.lua")
	now := time.Date(2026, 10, 23, 17, 0, 0, 0, time.UTC)
	first := path + ".bak-2026-10-23_17-00-00"
	if err := os.WriteFile(first, []byte("occupied\n"), 0o600); err != nil {
		t.Fatalf("write existing backup: %v", err)
	}
	if _, err := Set(root, FileConfig, "loginTime", "45", now, false); err != nil {
		t.Fatalf("set loginTime: %v", err)
	}
	second := first + "-01"
	if got := stringMustRead(t, second); !strings.Contains(got, "config.loginTime = 60") {
		t.Fatalf("collision backup = %q, want original config", got)
	}
	if got := stringMustRead(t, first); got != "occupied\n" {
		t.Fatalf("existing backup was overwritten: %q", got)
	}
}
