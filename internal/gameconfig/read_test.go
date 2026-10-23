package gameconfig

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Saagilat/tes3mp-easy/test/fixtures"
)

func fixtureRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "configs")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir configs: %v", err)
	}
	fixtures.WriteConfigs(t, dir)
	return root
}

func TestReadAllFiles(t *testing.T) {
	root := fixtureRoot(t)

	server, err := Read(root, FileServer)
	if err != nil {
		t.Fatalf("read server: %v", err)
	}
	wantServer := map[string]any{
		"hostname":            "Morrowind with friends",
		"port":                25565,
		"maximumPlayers":      8,
		"password":            "",
		"logLevel":            1,
		"masterServerEnabled": false,
	}
	if !reflect.DeepEqual(server.Values, wantServer) {
		t.Fatalf("server values = %#v, want %#v", server.Values, wantServer)
	}
	if len(server.ReadOnly) != 0 {
		t.Fatalf("server read_only = %v, want empty", server.ReadOnly)
	}

	lua, err := Read(root, FileConfig)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if len(lua.Values) != len(luaSpecs) {
		t.Fatalf("config values = %d, want %d: %#v",
			len(lua.Values), len(luaSpecs), lua.Values)
	}
	if got := lua.Values["gameMode"]; got != "Default" {
		t.Fatalf("gameMode = %#v, want Default", got)
	}
	if got := lua.Values["loginTime"]; got != 60 {
		t.Fatalf("loginTime = %#v, want 60", got)
	}
	if got := lua.Values["passTimeWhenEmpty"]; got != false {
		t.Fatalf("passTimeWhenEmpty = %#v, want false", got)
	}
	if len(lua.ReadOnly) != 0 {
		t.Fatalf("config read_only = %v, want empty", lua.ReadOnly)
	}

	banlist, err := Read(root, FileBanlist)
	if err != nil {
		t.Fatalf("read banlist: %v", err)
	}
	wantBanlist := map[string]any{
		"playerNames": []string{},
		"ipAddresses": []string{},
	}
	if !reflect.DeepEqual(banlist.Values, wantBanlist) {
		t.Fatalf("banlist values = %#v, want %#v", banlist.Values, wantBanlist)
	}
}

func TestReadLuaReadOnlyAndInlineComments(t *testing.T) {
	root := fixtureRoot(t)
	body := `config = {}

-- keep this comment
config.gameMode = "Morrowind -- friends" -- inline comment
config.loginTime = 45 -- keep me
config.someTable = {
  ["key"] = "value",
}
config.someList = { "a", "b" }
config.unknownScalar = 7
`
	path := filepath.Join(root, "configs", "config.lua")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config.lua: %v", err)
	}

	snapshot, err := Read(root, FileConfig)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if got := snapshot.Values["gameMode"]; got != "Morrowind -- friends" {
		t.Fatalf("gameMode = %#v, want inline comment preserved in string", got)
	}
	if got := snapshot.Values["loginTime"]; got != 45 {
		t.Fatalf("loginTime = %#v, want 45", got)
	}
	wantReadOnly := []string{"someList", "someTable"}
	if !reflect.DeepEqual(snapshot.ReadOnly, wantReadOnly) {
		t.Fatalf("read_only = %v, want %v", snapshot.ReadOnly, wantReadOnly)
	}
	if _, ok := snapshot.Values["unknownScalar"]; ok {
		t.Fatal("unknown scalar must not be exposed as an editable value")
	}
}

func TestReadRejectsMissingFiles(t *testing.T) {
	root := fixtureRoot(t)
	if err := os.Remove(filepath.Join(root, "configs", "banlist.json")); err != nil {
		t.Fatalf("remove banlist: %v", err)
	}
	if _, err := Read(root, FileBanlist); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing banlist error = %v, want fs.ErrNotExist", err)
	}
}

func TestSetHeaderDoesNotUseUnknownFile(t *testing.T) {
	if _, err := ParseFile("other"); err == nil {
		t.Fatal("ParseFile accepted an unsupported file")
	}
}

func stringMustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

var testNow = time.Date(2026, 10, 23, 12, 30, 0, 0, time.UTC)

func TestSetServerPreservesCommentsAndBacksUp(t *testing.T) {
	root := fixtureRoot(t)
	path := filepath.Join(root, "configs", "tes3mp-server-default.cfg")
	before := stringMustRead(t, path)

	result, err := Set(root, FileServer, "port", "25566", testNow, false)
	if err != nil {
		t.Fatalf("set port: %v", err)
	}
	if result.Value != 25566 {
		t.Fatalf("result value = %#v, want 25566", result.Value)
	}
	after := stringMustRead(t, path)
	if !strings.Contains(after, "port = 25566") {
		t.Fatalf("updated server config has no new port:\n%s", after)
	}
	if !strings.Contains(after, "Synthetic test configuration") {
		t.Fatal("comment was lost after config set")
	}
	backup := path + ".bak-2026-10-23_12-30-00"
	if data, err := os.ReadFile(backup); err != nil || string(data) != before {
		t.Fatalf("backup = %q, %v; want original", data, err)
	}

	if _, err := Set(root, FileServer, "port", "80", testNow, false); !errors.Is(err, ErrInvalid) {
		t.Fatalf("set invalid port error = %v, want ErrInvalid", err)
	}
	if unchanged := stringMustRead(t, path); unchanged != after {
		t.Fatal("invalid set changed the file")
	}
	if _, err := Set(root, FileServer, "localAddress", "127.0.0.1",
		testNow, false); !errors.Is(err, ErrInvalid) {
		t.Fatalf("set unknown key error = %v, want ErrInvalid", err)
	}
}
