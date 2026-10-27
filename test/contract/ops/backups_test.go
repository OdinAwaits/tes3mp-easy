package ops

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

var backupsContractBase = time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)

type backupsPayload struct {
	Type       string               `json:"type"`
	Current    *string              `json:"current"`
	Items      []backupsPayloadItem `json:"items"`
	NextBefore *string              `json:"next_before"`
}

type backupsPayloadItem struct {
	Name    string  `json:"name"`
	Size    int64   `json:"size"`
	MTime   string  `json:"mtime"`
	Current bool    `json:"current"`
	SHA256  *string `json:"sha256"`
}

func backupsContractRoot(t *testing.T, types ...string) string {
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

func writeContractBackup(t *testing.T, dir, name, body string, mtime time.Time) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o640); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("chtimes %s: %v", path, err)
	}
}

func writeContractCurrent(t *testing.T, dir, name string) {
	t.Helper()

	body := strings.Repeat("a", 64) + " " + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, "current.txt"),
		[]byte(body), 0o640); err != nil {
		t.Fatalf("write current.txt: %v", err)
	}
}

func decodeBackups(t *testing.T, env envelope, withSHA256 bool) backupsPayload {
	t.Helper()

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(env.Data, &raw); err != nil {
		t.Fatalf("decode backups data: %v; data=%s", err, env.Data)
	}
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	wantKeys := []string{"current", "items", "next_before", "type"}
	if !reflect.DeepEqual(keys, wantKeys) {
		t.Fatalf("backups keys = %v, want %v", keys, wantKeys)
	}

	wantItemKeys := []string{"current", "mtime", "name", "size"}
	if withSHA256 {
		wantItemKeys = []string{"current", "mtime", "name", "sha256", "size"}
	}
	var itemKeys struct {
		Type  string                       `json:"type"`
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(env.Data, &itemKeys); err != nil {
		t.Fatalf("decode items: %v", err)
	}
	allowed := [][]string{wantItemKeys}
	if itemKeys.Type == "state" {
		available := append([]string{"mods_available"}, wantItemKeys...)
		sort.Strings(available)
		allowed = append(allowed, available)
		both := append([]string{"mods_available", "mods_archive"}, wantItemKeys...)
		sort.Strings(both)
		allowed = append(allowed, both)
	}
	for i, item := range itemKeys.Items {
		keys := make([]string, 0, len(item))
		for key := range item {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		matched := false
		for _, want := range allowed {
			if reflect.DeepEqual(keys, want) {
				matched = true
				break
			}
		}
		if !matched {
			t.Fatalf("item %d keys = %v, want one of %v", i, keys, allowed)
		}
	}

	var data backupsPayload
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("decode backups data: %v", err)
	}
	return data
}

func TestBackupsListEmptyContract(t *testing.T) {
	for _, root := range []string{
		t.TempDir(),
		backupsContractRoot(t, "state"),
	} {
		res := testHarness.RunRoot(t, root, "--json",
			"backups", "list", "--type", "state")
		res.requireExit(t, 0)
		if res.Stderr != "" {
			t.Fatalf("stderr = %q, want empty", res.Stderr)
		}

		data := decodeBackups(t, res.decodeEnvelope(t), false)
		if data.Type != "state" {
			t.Fatalf("type = %q, want state", data.Type)
		}
		if data.Items == nil {
			t.Fatal("items = null, want []")
		}
		if len(data.Items) != 0 {
			t.Fatalf("items = %#v, want empty", data.Items)
		}
		if data.Current != nil {
			t.Fatalf("current = %q, want null", *data.Current)
		}
		if data.NextBefore != nil {
			t.Fatalf("next_before = %q, want null", *data.NextBefore)
		}
	}
}

func TestBackupsListSortCurrentSHAAndPagesContract(t *testing.T) {
	root := backupsContractRoot(t, "mods", "state")
	dir := filepath.Join(root, "backups", "mods")

	files := []struct {
		name  string
		body  string
		mtime time.Time
	}{
		{"mods-2026-10-22_12-00-03.tar.gz", "newest", backupsContractBase.Add(3 * time.Second)},
		{"mods-2026-10-22_12-00-02.tar.gz", "second", backupsContractBase.Add(2 * time.Second)},
		{"mods-2026-10-22_12-00-01.tar.gz", "third", backupsContractBase.Add(time.Second)},
		{"mods-init-2026-10-22_12-00-00.tar.gz", "fourth", backupsContractBase},
	}
	for _, file := range files {
		writeContractBackup(t, dir, file.name, file.body, file.mtime)
	}
	writeContractBackup(t, dir, "state-2026-10-22_13-00-00.tar.gz",
		"wrong type", backupsContractBase.Add(4*time.Second))
	writeContractBackup(t, dir, "notes.tar.gz",
		"not an archive", backupsContractBase.Add(5*time.Second))
	writeContractCurrent(t, dir, files[0].name)

	first := testHarness.RunRoot(t, root, "--json", "backups", "list",
		"--type", "mods", "--limit", "2", "--sha256")
	first.requireExit(t, 0)
	if first.Stderr != "" {
		t.Fatalf("stderr = %q, want empty", first.Stderr)
	}
	data := decodeBackups(t, first.decodeEnvelope(t), true)
	if data.Current == nil || *data.Current != files[0].name {
		t.Fatalf("current = %v, want %q", data.Current, files[0].name)
	}
	wantNames := []string{files[0].name, files[1].name}
	if got := contractBackupNames(data.Items); !reflect.DeepEqual(got, wantNames) {
		t.Fatalf("first page = %#v, want %#v", got, wantNames)
	}
	if data.NextBefore == nil || *data.NextBefore != wantNames[1] {
		t.Fatalf("next_before = %v, want %q", data.NextBefore, wantNames[1])
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
		if item.Current != (i == 0) {
			t.Fatalf("item %d current = %v", i, item.Current)
		}
	}

	second := testHarness.RunRoot(t, root, "--json", "backups", "list",
		"--type", "mods", "--limit", "2", "--before", *data.NextBefore)
	second.requireExit(t, 0)
	next := decodeBackups(t, second.decodeEnvelope(t), false)
	wantSecond := []string{files[2].name, files[3].name}
	if got := contractBackupNames(next.Items); !reflect.DeepEqual(got, wantSecond) {
		t.Fatalf("second page = %#v, want %#v", got, wantSecond)
	}
	if next.Current == nil || *next.Current != files[0].name {
		t.Fatalf("second current = %v, want %q", next.Current, files[0].name)
	}
	if next.NextBefore == nil || *next.NextBefore != wantSecond[1] {
		t.Fatalf("second next_before = %v, want %q",
			next.NextBefore, wantSecond[1])
	}

	third := testHarness.RunRoot(t, root, "--json", "backups", "list",
		"--type", "mods", "--limit", "2", "--before", *next.NextBefore)
	third.requireExit(t, 0)
	empty := decodeBackups(t, third.decodeEnvelope(t), false)
	if empty.Items == nil || len(empty.Items) != 0 {
		t.Fatalf("third items = %#v, want []", empty.Items)
	}
	if empty.NextBefore != nil {
		t.Fatalf("third next_before = %q, want null", *empty.NextBefore)
	}
}

func contractBackupNames(items []backupsPayloadItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Name)
	}
	return out
}

func TestBackupsListStateContract(t *testing.T) {
	root := backupsContractRoot(t, "mods", "state")
	stateDir := filepath.Join(root, "backups", "state")
	names := []string{
		"state-predeploy-2026-10-22_12-00-02.tar.gz",
		"state-manual-2026-10-22_12-00-01.tar.gz",
		"state-2026-10-22_12-00-00.tar.gz",
	}
	for i, name := range names {
		writeContractBackup(t, stateDir, name, "state",
			backupsContractBase.Add(time.Duration(i)*time.Second))
	}
	writeContractBackup(t, stateDir, "mods-2026-10-22_12-00-00.tar.gz",
		"wrong type", backupsContractBase.Add(3*time.Second))
	writeContractCurrent(t, stateDir, "state-2026-10-22_12-00-00.tar.gz")

	res := testHarness.RunRoot(t, root, "--json",
		"backups", "list", "--type", "state")
	res.requireExit(t, 0)
	data := decodeBackups(t, res.decodeEnvelope(t), false)
	want := []string{names[2], names[1], names[0]}
	if got := contractBackupNames(data.Items); !reflect.DeepEqual(got, want) {
		t.Fatalf("state items = %#v, want %#v", got, want)
	}
	if data.Current != nil {
		t.Fatalf("state current = %q, want null", *data.Current)
	}
}

func TestBackupsListDoesNotModifyRootContract(t *testing.T) {
	root := backupsContractRoot(t, "state")
	writeContractBackup(t, filepath.Join(root, "backups", "state"),
		"state-2026-10-22_12-00-00.tar.gz", "archive", backupsContractBase)

	before := snapshotTree(t, root)
	res := testHarness.RunRoot(t, root, "--json",
		"backups", "list", "--type", "state", "--sha256")
	res.requireExit(t, 0)
	after := snapshotTree(t, root)

	if !reflect.DeepEqual(after, before) {
		t.Fatalf("backups list changed the data root:\nbefore=%v\nafter=%v",
			before, after)
	}
	if _, err := os.Stat(filepath.Join(root, ".locks")); !os.IsNotExist(err) {
		t.Fatalf("backups list created .locks: %v", err)
	}
}

func TestBackupsListBrokenCurrentContract(t *testing.T) {
	root := backupsContractRoot(t, "mods")
	if err := os.WriteFile(
		filepath.Join(root, "backups", "mods", "current.txt"),
		[]byte("broken\n"), 0o640); err != nil {
		t.Fatalf("write current.txt: %v", err)
	}

	res := testHarness.RunRoot(t, root, "--json",
		"backups", "list", "--type", "mods")
	res.requireJSONError(t, 7, "archive_invalid")
}

func TestBackupsListRejectsInvalidArgumentsContract(t *testing.T) {
	tests := []struct {
		name string
		args []string
		exit int
		code string
	}{
		{
			name: "missing type",
			args: []string{"backups", "list"},
			exit: 2, code: "invalid_argument",
		},
		{
			name: "unknown type",
			args: []string{"backups", "list", "--type", "both"},
			exit: 2, code: "invalid_argument",
		},
		{
			name: "zero limit",
			args: []string{"backups", "list", "--type", "mods", "--limit=0"},
			exit: 2, code: "invalid_argument",
		},
		{
			name: "too large limit",
			args: []string{"backups", "list", "--type", "mods", "--limit=501"},
			exit: 2, code: "invalid_argument",
		},
		{
			name: "empty before",
			args: []string{"backups", "list", "--type", "mods", "--before="},
			exit: 2, code: "invalid_argument",
		},
		{
			name: "unsafe before",
			args: []string{
				"backups", "list", "--type", "mods", "--before=../evil",
			},
			exit: 8, code: "unsafe_path",
		},
		{
			name: "wrong before type",
			args: []string{
				"backups", "list", "--type", "state",
				"--before=mods-2026-10-22_12-00-00.tar.gz",
			},
			exit: 2, code: "invalid_argument",
		},
		{
			name: "extra argument",
			args: []string{
				"backups", "list", "--type", "mods", "extra",
			},
			exit: 2, code: "invalid_argument",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"--json"}, tt.args...)
			res := testHarness.Run(t, args...)
			res.requireJSONError(t, tt.exit, tt.code)
		})
	}
}

func TestBackupsListPaginates2016ArchivesContract(t *testing.T) {
	root := backupsContractRoot(t, "state")
	dir := filepath.Join(root, "backups", "state")

	const total = 2016
	want := make([]string, 0, total)
	for i := 0; i < total; i++ {
		at := backupsContractBase.Add(-time.Duration(i) * time.Second)
		name := "state-" + at.Format("2006-01-02_15-04-05") + ".tar.gz"
		writeContractBackup(t, dir, name, "", at)
		want = append(want, name)
	}

	got := make([]string, 0, total)
	before := ""
	pageSizes := []int{500, 500, 500, 500, 16}
	for page, size := range pageSizes {
		args := []string{
			"--json", "backups", "list", "--type", "state", "--limit=500",
		}
		if before != "" {
			args = append(args, "--before", before)
		}
		res := testHarness.RunRoot(t, root, args...)
		res.requireExit(t, 0)
		if res.Stderr != "" {
			t.Fatalf("page %d stderr = %q, want empty", page, res.Stderr)
		}
		data := decodeBackups(t, res.decodeEnvelope(t), false)
		if len(data.Items) != size {
			t.Fatalf("page %d has %d items, want %d",
				page, len(data.Items), size)
		}
		got = append(got, contractBackupNames(data.Items)...)

		switch {
		case page < len(pageSizes)-1 && data.NextBefore == nil:
			t.Fatalf("page %d next_before = null, want a cursor", page)
		case page == len(pageSizes)-1 && data.NextBefore != nil:
			t.Fatalf("last page next_before = %q, want null",
				*data.NextBefore)
		case data.NextBefore != nil:
			before = *data.NextBefore
		}
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("paged archives differ: got %d names, want %d",
			len(got), len(want))
	}
	seen := make(map[string]bool, total)
	for _, name := range got {
		if seen[name] {
			t.Fatalf("archive %s appeared twice", name)
		}
		seen[name] = true
	}
}
