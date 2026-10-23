package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/ops"
)

var backupTestBase = time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)

func backupsRoot(t *testing.T, types ...string) string {
	t.Helper()

	root := t.TempDir()
	for _, typ := range types {
		dir := filepath.Join(root, "backups", typ)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatalf("create %s: %v", dir, err)
		}
	}
	return root
}

func writeBackupFile(t *testing.T, dir, name, body string, mtime time.Time) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o640); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("chtimes %s: %v", path, err)
	}
}

func writeCurrent(t *testing.T, dir, name string) {
	t.Helper()

	body := strings.Repeat("a", 64) + " " + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, "current.txt"),
		[]byte(body), 0o640); err != nil {
		t.Fatalf("write current.txt: %v", err)
	}
}

func names(items []backupsItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Name)
	}
	return out
}

func TestParseBackupsListArgs(t *testing.T) {
	root := t.TempDir()
	modsName := "mods-2026-10-22_12-00-00.tar.gz"
	stateName := "state-manual-2026-10-22_12-00-00.tar.gz"

	tests := []struct {
		name     string
		args     []string
		exit     ops.ExitCode
		wantType string
		limit    int
		before   string
		sha256   bool
	}{
		{
			name: "defaults",
			args: []string{"--type", "mods"},
			exit: ops.ExitOK, wantType: "mods", limit: 200,
		},
		{
			name: "state with flags",
			args: []string{
				"--type=state", "--limit=1", "--before", stateName,
				"--sha256",
			},
			exit: ops.ExitOK, wantType: "state", limit: 1,
			before: stateName, sha256: true,
		},
		{
			name: "missing type",
			args: []string{},
			exit: ops.ExitInvalidArgument,
		},
		{
			name: "unknown type",
			args: []string{"--type", "both"},
			exit: ops.ExitInvalidArgument,
		},
		{
			name: "zero limit",
			args: []string{"--type", "mods", "--limit=0"},
			exit: ops.ExitInvalidArgument,
		},
		{
			name: "limit too large",
			args: []string{"--type", "mods", "--limit=501"},
			exit: ops.ExitInvalidArgument,
		},
		{
			name: "limit not a number",
			args: []string{"--type", "mods", "--limit=many"},
			exit: ops.ExitInvalidArgument,
		},
		{
			name: "before empty",
			args: []string{"--type", "mods", "--before="},
			exit: ops.ExitInvalidArgument,
		},
		{
			name: "before unsafe",
			args: []string{"--type", "mods", "--before=../evil"},
			exit: ops.ExitUnsafePath,
		},
		{
			name: "before wrong type",
			args: []string{"--type", "mods", "--before", stateName},
			exit: ops.ExitInvalidArgument,
		},
		{
			name: "before invalid name",
			args: []string{"--type", "state", "--before", "state.tar.gz"},
			exit: ops.ExitInvalidArgument,
		},
		{
			name: "unknown flag",
			args: []string{"--type", "mods", "--bogus"},
			exit: ops.ExitInvalidArgument,
		},
		{
			name: "extra argument",
			args: []string{"--type", "mods", modsName},
			exit: ops.ExitInvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseBackupsListArgs(root, tt.args)
			if tt.exit != ops.ExitOK {
				if err == nil {
					t.Fatalf("parse error = nil, want exit %d", tt.exit)
				}
				if err.Exit != tt.exit {
					t.Fatalf("exit = %d, want %d; message=%q",
						err.Exit, tt.exit, err.Message)
				}
				return
			}
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if got.typ != tt.wantType || got.limit != tt.limit ||
				got.before != tt.before || got.sha256 != tt.sha256 {
				t.Fatalf("options = %#v, want type=%q limit=%d before=%q sha=%v",
					got, tt.wantType, tt.limit, tt.before, tt.sha256)
			}
		})
	}
}

func TestListBackupsSortsFiltersAndHashes(t *testing.T) {
	root := backupsRoot(t, "mods", "state")
	dir := filepath.Join(root, "backups", "mods")

	equalTime := backupTestBase.Add(-2 * time.Hour)
	files := []struct {
		name  string
		body  string
		mtime time.Time
	}{
		{"mods-2026-10-22_12-00-02.tar.gz", "newest", backupTestBase.Add(2 * time.Second)},
		{"mods-2026-10-22_12-00-01.tar.gz", "middle", backupTestBase.Add(time.Second)},
		{"mods-2026-10-22_12-00-00.tar.gz", "current", backupTestBase},
		{"mods-init-2026-10-22_11-00-00.tar.gz", "init", backupTestBase.Add(-time.Hour)},
		{"mods-2026-10-22_10-00-01.tar.gz", "tie one", equalTime},
		{"mods-2026-10-22_10-00-00.tar.gz", "tie zero", equalTime},
	}
	for _, file := range files {
		writeBackupFile(t, dir, file.name, file.body, file.mtime)
	}
	writeBackupFile(t, dir, "notes.tar.gz", "ignored", backupTestBase.Add(time.Hour))
	if err := os.Mkdir(filepath.Join(dir,
		"mods-2026-10-21_00-00-00.tar.gz"), 0o750); err != nil {
		t.Fatalf("create archive directory: %v", err)
	}
	if err := os.Symlink(filepath.Join(dir, "notes.tar.gz"),
		filepath.Join(dir, "mods-2026-10-20_00-00-00.tar.gz")); err != nil {
		t.Logf("symlink is unavailable: %v", err)
	}
	writeCurrent(t, dir, "mods-2026-10-22_12-00-00.tar.gz")

	opts, parseErr := parseBackupsListArgs(root,
		[]string{"--type", "mods", "--limit", "20", "--sha256"})
	if parseErr != nil {
		t.Fatalf("parse args: %v", parseErr)
	}
	data, err := listBackups(opts)
	if err != nil {
		t.Fatalf("list backups: %v", err)
	}

	wantNames := []string{
		"mods-2026-10-22_12-00-02.tar.gz",
		"mods-2026-10-22_12-00-01.tar.gz",
		"mods-2026-10-22_12-00-00.tar.gz",
		"mods-init-2026-10-22_11-00-00.tar.gz",
		"mods-2026-10-22_10-00-01.tar.gz",
		"mods-2026-10-22_10-00-00.tar.gz",
	}
	if got := names(data.Items); !reflect.DeepEqual(got, wantNames) {
		t.Fatalf("names = %#v, want %#v", got, wantNames)
	}
	if data.Type != "mods" {
		t.Fatalf("type = %q, want mods", data.Type)
	}
	if data.Current == nil || *data.Current != wantNames[2] {
		t.Fatalf("current = %v, want %q", data.Current, wantNames[2])
	}
	if data.NextBefore != nil {
		t.Fatalf("next_before = %q, want null for an incomplete page",
			*data.NextBefore)
	}

	for i, item := range data.Items {
		if item.SHA256 == nil {
			t.Fatalf("item %d has no sha256", i)
		}
		sum := sha256.Sum256([]byte(files[i].body))
		if *item.SHA256 != hex.EncodeToString(sum[:]) {
			t.Fatalf("item %d sha256 = %s, want %s",
				i, *item.SHA256, hex.EncodeToString(sum[:]))
		}
		if item.Size != int64(len(files[i].body)) {
			t.Fatalf("item %d size = %d, want %d",
				i, item.Size, len(files[i].body))
		}
		if item.Current != (i == 2) {
			t.Fatalf("item %d current = %v", i, item.Current)
		}
	}
	if data.Items[0].MTime != "2026-10-22T12:00:02Z" {
		t.Fatalf("mtime = %q, want UTC RFC3339", data.Items[0].MTime)
	}
}

func TestListBackupsPaginationAndCursor(t *testing.T) {
	root := backupsRoot(t, "state")
	dir := filepath.Join(root, "backups", "state")
	for i := 0; i < 5; i++ {
		at := backupTestBase.Add(time.Duration(i) * time.Second)
		name := "state-2026-10-22_12-00-0" + string(rune('0'+i)) + ".tar.gz"
		writeBackupFile(t, dir, name, name, at)
	}

	firstOpts, parseErr := parseBackupsListArgs(root,
		[]string{"--type", "state", "--limit", "2"})
	if parseErr != nil {
		t.Fatalf("parse first page: %v", parseErr)
	}
	first, err := listBackups(firstOpts)
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	wantFirst := []string{
		"state-2026-10-22_12-00-04.tar.gz",
		"state-2026-10-22_12-00-03.tar.gz",
	}
	if got := names(first.Items); !reflect.DeepEqual(got, wantFirst) {
		t.Fatalf("first page = %#v, want %#v", got, wantFirst)
	}
	if first.NextBefore == nil || *first.NextBefore != wantFirst[1] {
		t.Fatalf("first next_before = %v, want %q", first.NextBefore, wantFirst[1])
	}
	if first.Current != nil {
		t.Fatalf("state current = %q, want null", *first.Current)
	}

	secondOpts, parseErr := parseBackupsListArgs(root, []string{
		"--type", "state", "--limit", "2", "--before", *first.NextBefore,
	})
	if parseErr != nil {
		t.Fatalf("parse second page: %v", parseErr)
	}
	second, err := listBackups(secondOpts)
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	wantSecond := []string{
		"state-2026-10-22_12-00-02.tar.gz",
		"state-2026-10-22_12-00-01.tar.gz",
	}
	if got := names(second.Items); !reflect.DeepEqual(got, wantSecond) {
		t.Fatalf("second page = %#v, want %#v", got, wantSecond)
	}
	if second.NextBefore == nil || *second.NextBefore != wantSecond[1] {
		t.Fatalf("second next_before = %v, want %q",
			second.NextBefore, wantSecond[1])
	}

	thirdOpts, parseErr := parseBackupsListArgs(root, []string{
		"--type", "state", "--limit", "2", "--before", *second.NextBefore,
	})
	if parseErr != nil {
		t.Fatalf("parse third page: %v", parseErr)
	}
	third, err := listBackups(thirdOpts)
	if err != nil {
		t.Fatalf("third page: %v", err)
	}
	if got := names(third.Items); !reflect.DeepEqual(got,
		[]string{"state-2026-10-22_12-00-00.tar.gz"}) {
		t.Fatalf("third page = %#v, want the oldest archive", got)
	}
	if third.NextBefore != nil {
		t.Fatalf("third next_before = %q, want null", *third.NextBefore)
	}
}

func TestListBackupsCursorSurvivesDeletedArchive(t *testing.T) {
	root := backupsRoot(t, "state")
	dir := filepath.Join(root, "backups", "state")
	for i := 0; i < 4; i++ {
		at := backupTestBase.Add(time.Duration(i) * time.Second)
		name := "state-2026-10-22_12-00-0" + string(rune('0'+i)) + ".tar.gz"
		writeBackupFile(t, dir, name, name, at)
	}

	deleted := "state-2026-10-22_12-00-02.tar.gz"
	cursorTime := backupTestBase.Add(2 * time.Second)
	options := backupsListOptions{
		root: root, typ: "state", limit: 10,
		before: deleted, beforeTime: cursorTime,
	}
	data, err := listBackups(options)
	if err != nil {
		t.Fatalf("list backups: %v", err)
	}
	want := []string{
		"state-2026-10-22_12-00-01.tar.gz",
		"state-2026-10-22_12-00-00.tar.gz",
	}
	if got := names(data.Items); !reflect.DeepEqual(got, want) {
		t.Fatalf("names = %#v, want %#v", got, want)
	}
}

func TestListBackupsEmptyAndMissingDirectory(t *testing.T) {
	for _, root := range []string{
		t.TempDir(),
		backupsRoot(t, "state"),
		backupsRoot(t, "state", "mods"),
	} {
		data, err := listBackups(backupsListOptions{
			root: root, typ: "state", limit: 200,
		})
		if err != nil {
			t.Fatalf("list backups in %s: %v", root, err)
		}
		if data.Items == nil || len(data.Items) != 0 {
			t.Fatalf("items = %#v, want []", data.Items)
		}
		if data.Current != nil || data.NextBefore != nil {
			t.Fatalf("current/next = %v/%v, want null/null",
				data.Current, data.NextBefore)
		}
	}
}

func TestListBackupsRejectsBrokenCurrent(t *testing.T) {
	root := backupsRoot(t, "mods")
	dir := filepath.Join(root, "backups", "mods")
	if err := os.WriteFile(filepath.Join(dir, "current.txt"),
		[]byte("broken\n"), 0o640); err != nil {
		t.Fatalf("write current.txt: %v", err)
	}

	_, err := listBackups(backupsListOptions{
		root: root, typ: "mods", limit: 200,
	})
	if err == nil || err.Exit != ops.ExitArchiveInvalid {
		t.Fatalf("error = %#v, want archive_invalid", err)
	}
}

func TestListBackupsIgnoresCurrentForState(t *testing.T) {
	root := backupsRoot(t, "state")
	writeCurrent(t, filepath.Join(root, "backups", "state"),
		"state-2026-10-22_12-00-00.tar.gz")

	data, err := listBackups(backupsListOptions{
		root: root, typ: "state", limit: 200,
	})
	if err != nil {
		t.Fatalf("list backups: %v", err)
	}
	if data.Current != nil {
		t.Fatalf("current = %q, want null", *data.Current)
	}
}

func TestRunBackupsListJSON(t *testing.T) {
	root := backupsRoot(t, "state")
	dir := filepath.Join(root, "backups", "state")
	name := "state-2026-10-22_12-00-00.tar.gz"
	writeBackupFile(t, dir, name, "archive", backupTestBase)

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--root", root, "--json", "backups", "list",
		"--type", "state", "--limit=1", "--sha256",
	}, &stdout, &stderr)
	if code != int(ops.ExitOK) {
		t.Fatalf("exit = %d, want 0; stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	var data backupsData
	decodeJSONData(t, stdout.String(), &data)
	if data.Type != "state" || len(data.Items) != 1 {
		t.Fatalf("data = %#v, want one state item", data)
	}
	if data.Items[0].SHA256 == nil {
		t.Fatal("sha256 is absent with --sha256")
	}
	if data.NextBefore == nil || *data.NextBefore != name {
		t.Fatalf("next_before = %v, want %q", data.NextBefore, name)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout.String()), &raw); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
}

func TestRunBackupsListHuman(t *testing.T) {
	root := backupsRoot(t, "state")
	name := "state-2026-10-22_12-00-00.tar.gz"
	writeBackupFile(t, filepath.Join(root, "backups", "state"),
		name, "archive", backupTestBase)

	var stdout, stderr bytes.Buffer
	code := run([]string{"--root", root, "backups", "list",
		"--type", "state"}, &stdout, &stderr)
	if code != int(ops.ExitOK) {
		t.Fatalf("exit = %d, want 0; stderr=%q", code, stderr.String())
	}
	for _, want := range []string{
		"type: state\n",
		"current: -\n",
		name,
		"2026-10-22T12:00:00Z",
		"next_before: -\n",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want substring %q", stdout.String(), want)
		}
	}
}

func TestRunBackupsDispatchErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		exit int
		code string
	}{
		{
			name: "missing subcommand",
			args: []string{"--json", "backups"},
			exit: 2, code: "invalid_argument",
		},
		{
			name: "unknown subcommand",
			args: []string{"--json", "backups", "bogus"},
			exit: 2, code: "invalid_argument",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(append([]string{"--root", t.TempDir()}, tt.args...),
				&stdout, &stderr)
			if code != tt.exit {
				t.Fatalf("exit = %d, want %d; stderr=%q",
					code, tt.exit, stderr.String())
			}
			var env struct {
				OK    bool `json:"ok"`
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
				t.Fatalf("decode error envelope: %v; stdout=%q",
					err, stdout.String())
			}
			if env.OK || env.Error.Code != tt.code {
				t.Fatalf("envelope = %#v, want code %q", env, tt.code)
			}
		})
	}
}
