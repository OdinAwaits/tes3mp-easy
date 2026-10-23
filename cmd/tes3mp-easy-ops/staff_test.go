package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
	"github.com/Saagilat/tes3mp-easy/internal/lock"
	"github.com/Saagilat/tes3mp-easy/internal/ops"
)

var staffUnitNow = time.Date(2026, 10, 23, 18, 0, 0, 0, time.UTC)

func staffUnitRoot(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	dir := filepath.Join(root, "players")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir players: %v", err)
	}
	body := `{
  "name": "John Doe",
  "settings": {
    "staffRank": 0,
    "other": "keep"
  }
}
`
	if err := os.WriteFile(filepath.Join(dir, "John Doe.json"),
		[]byte(body), 0o640); err != nil {
		t.Fatalf("write player: %v", err)
	}
	return root
}

func fixStaffUnitNow(t *testing.T, at time.Time) {
	t.Helper()

	old := staffNowFunc
	staffNowFunc = func() time.Time { return at }
	t.Cleanup(func() { staffNowFunc = old })
}

func TestParseStaffSetArgs(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		exit   ops.ExitCode
		player string
		rank   int
	}{
		{
			name: "valid owner", args: []string{"--player", "John Doe", "--rank", "3"},
			exit: ops.ExitOK, player: "John Doe", rank: 3,
		},
		{
			name: "equals and zero", args: []string{"--player=Jane", "--rank=0"},
			exit: ops.ExitOK, player: "Jane", rank: 0,
		},
		{
			name: "negative rank", args: []string{"--player", "Jane", "--rank", "-1"},
			exit: ops.ExitInvalidArgument,
		},
		{
			name: "rank too high", args: []string{"--player", "Jane", "--rank", "4"},
			exit: ops.ExitInvalidArgument,
		},
		{name: "missing player", args: []string{"--rank", "1"}, exit: ops.ExitInvalidArgument},
		{name: "empty player", args: []string{"--player=", "--rank", "1"}, exit: ops.ExitInvalidArgument},
		{name: "missing rank", args: []string{"--player", "Jane"}, exit: ops.ExitInvalidArgument},
		{name: "unknown flag", args: []string{"--bogus"}, exit: ops.ExitInvalidArgument},
		{
			name: "extra argument", args: []string{"--player", "Jane", "--rank", "1", "extra"},
			exit: ops.ExitInvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseStaffSetArgs(tt.args)
			if err != nil {
				if tt.exit == ops.ExitOK {
					t.Fatalf("parse error: %v", err)
				}
				if err.Exit != tt.exit {
					t.Fatalf("exit = %d, want %d", err.Exit, tt.exit)
				}
				return
			}
			if tt.exit != ops.ExitOK {
				t.Fatalf("parse succeeded, want exit %d", tt.exit)
			}
			if got.player != tt.player || got.rank != tt.rank {
				t.Fatalf("args = %#v, want player %q rank %d", got, tt.player, tt.rank)
			}
		})
	}
}

func TestSetStaffRankLockedWritesAndBacksUp(t *testing.T) {
	root := staffUnitRoot(t)
	path := filepath.Join(root, "players", "John Doe.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read player: %v", err)
	}

	result, opsErr := setStaffRankLocked(staffSetRequest{
		root: root, player: "John Doe", rank: 2, now: staffUnitNow,
		writeFn: atomicfile.WriteFile,
	})
	if opsErr != nil {
		t.Fatalf("set rank: %v", opsErr)
	}
	if result.player != "John Doe" || result.rank != 2 {
		t.Fatalf("result = %#v", result)
	}

	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read updated player: %v", err)
	}
	if !bytes.Contains(updated, []byte(`"staffRank": 2`)) {
		t.Fatalf("player JSON has no staffRank 2:\n%s", updated)
	}
	if !bytes.Contains(updated, []byte(`"other": "keep"`)) {
		t.Fatalf("unrelated setting was lost:\n%s", updated)
	}
	backup := path + ".bak-2026-10-23_18-00-00"
	contents, err := os.ReadFile(backup)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if !bytes.Equal(contents, before) {
		t.Fatalf("backup differs from original:\n%s", contents)
	}
}

func TestSetStaffRankLockedDryRunAndCollision(t *testing.T) {
	root := staffUnitRoot(t)
	path := filepath.Join(root, "players", "John Doe.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read player: %v", err)
	}

	if _, opsErr := setStaffRankLocked(staffSetRequest{
		root: root, player: "John Doe", rank: 3, now: staffUnitNow,
		dryRun: true, writeFn: atomicfile.WriteFile,
	}); opsErr != nil {
		t.Fatalf("dry-run set: %v", opsErr)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, before) {
		t.Fatalf("dry-run changed player JSON: %v", err)
	}
	if matches, _ := filepath.Glob(path + ".bak-*"); len(matches) != 0 {
		t.Fatalf("dry-run created backups: %v", matches)
	}

	first := path + ".bak-2026-10-23_18-00-00"
	if err := os.WriteFile(first, []byte("occupied\n"), 0o600); err != nil {
		t.Fatalf("write occupied backup: %v", err)
	}
	if _, opsErr := setStaffRankLocked(staffSetRequest{
		root: root, player: "John Doe", rank: 1, now: staffUnitNow,
		writeFn: atomicfile.WriteFile,
	}); opsErr != nil {
		t.Fatalf("set with collision: %v", opsErr)
	}
	collision := first + "-01"
	if contents, err := os.ReadFile(collision); err != nil ||
		!bytes.Equal(contents, before) {
		t.Fatalf("collision backup = %q, %v; want original", contents, err)
	}
	if contents, err := os.ReadFile(first); err != nil || string(contents) != "occupied\n" {
		t.Fatalf("occupied backup was overwritten: %q, %v", contents, err)
	}
}

func TestSetStaffRankLockedWriteFailureKeepsPlayer(t *testing.T) {
	root := staffUnitRoot(t)
	path := filepath.Join(root, "players", "John Doe.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read player: %v", err)
	}

	calls := 0
	write := func(name string, data []byte, mode os.FileMode) error {
		calls++
		if calls == 2 {
			return errors.New("simulated write failure")
		}
		return atomicfile.WriteFile(name, data, mode)
	}
	_, opsErr := setStaffRankLocked(staffSetRequest{
		root: root, player: "John Doe", rank: 3, now: staffUnitNow,
		writeFn: write,
	})
	if opsErr == nil || opsErr.Exit != ops.ExitInternal {
		t.Fatalf("error = %v, want internal", opsErr)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, before) {
		t.Fatalf("failed write changed player JSON: %v", err)
	}
}

func TestSetStaffRankLockedErrors(t *testing.T) {
	root := staffUnitRoot(t)
	tests := []struct {
		name   string
		player string
		exit   ops.ExitCode
	}{
		{name: "missing", player: "Nobody", exit: ops.ExitNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, opsErr := setStaffRankLocked(staffSetRequest{
				root: root, player: tt.player, rank: 1, now: staffUnitNow,
				writeFn: atomicfile.WriteFile,
			})
			if opsErr == nil || opsErr.Exit != tt.exit {
				t.Fatalf("error = %v, want exit %d", opsErr, tt.exit)
			}
		})
	}
}

func TestWriteStaffSetHuman(t *testing.T) {
	var out bytes.Buffer
	err := writeStaffSetHuman(&out, staffSetData{
		Player: "John Doe", Rank: 2, RestartRequired: true,
	})
	if err != nil {
		t.Fatalf("write human: %v", err)
	}
	want := "player: John Doe\nrank: 2\nrestart_required: true\n"
	if out.String() != want {
		t.Fatalf("output = %q, want %q", out.String(), want)
	}
}

func TestRunStaffSetLocked(t *testing.T) {
	root := staffUnitRoot(t)
	if err := os.MkdirAll(filepath.Join(root, ".locks"), 0o700); err != nil {
		t.Fatalf("mkdir .locks: %v", err)
	}
	held, err := lock.Acquire(filepath.Join(root, ".locks", "ops.lock"),
		lock.Exclusive, 0)
	if err != nil {
		t.Fatalf("hold lock: %v", err)
	}
	defer func() { _ = held.Close() }()

	path := filepath.Join(root, "players", "John Doe.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read player: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--root", root, "--json", "--lock-timeout=0",
		"staff", "set", "--player", "John Doe", "--rank", "3",
	}, &stdout, &stderr)
	if code != int(ops.ExitLocked) {
		t.Fatalf("exit = %d, want %d; stdout=%q stderr=%q",
			code, ops.ExitLocked, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), `"code":"locked"`) {
		t.Fatalf("stdout has no locked error: %q", stdout.String())
	}
	after, err := os.ReadFile(path)
	if err != nil || !reflect.DeepEqual(after, before) {
		t.Fatalf("locked staff set changed player JSON: %v", err)
	}
}

func TestRunStaffSetRejectsUnsafePlayer(t *testing.T) {
	root := staffUnitRoot(t)

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--root", root, "--json",
		"staff", "set", "--player", "../John Doe", "--rank", "1",
	}, &stdout, &stderr)
	if code != int(ops.ExitUnsafePath) {
		t.Fatalf("exit = %d, want %d; stdout=%q stderr=%q",
			code, ops.ExitUnsafePath, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), `"code":"unsafe_path"`) {
		t.Fatalf("stdout has no unsafe_path error: %q", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".locks")); !os.IsNotExist(err) {
		t.Fatalf("unsafe player created .locks: %v", err)
	}
}
