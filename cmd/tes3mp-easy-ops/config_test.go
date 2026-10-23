package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/gameconfig"
	"github.com/Saagilat/tes3mp-easy/internal/lock"
	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/test/fixtures"
)

func configUnitRoot(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	dir := filepath.Join(root, "configs")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir configs: %v", err)
	}
	fixtures.WriteConfigs(t, dir)
	return root
}

func TestParseConfigGetArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		exit ops.ExitCode
		file gameconfig.File
	}{
		{
			name: "server", args: []string{"--file", "server"},
			exit: ops.ExitOK, file: gameconfig.FileServer,
		},
		{
			name: "config equals", args: []string{"--file=config"},
			exit: ops.ExitOK, file: gameconfig.FileConfig,
		},
		{
			name: "banlist", args: []string{"--file=banlist"},
			exit: ops.ExitOK, file: gameconfig.FileBanlist,
		},
		{name: "missing file", args: nil, exit: ops.ExitInvalidArgument},
		{name: "empty file", args: []string{"--file="}, exit: ops.ExitInvalidArgument},
		{name: "unsupported file", args: []string{"--file", "world"}, exit: ops.ExitInvalidArgument},
		{name: "unknown flag", args: []string{"--bogus"}, exit: ops.ExitInvalidArgument},
		{
			name: "extra argument", args: []string{"--file", "server", "extra"},
			exit: ops.ExitInvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseConfigGetArgs(tt.args)
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
			if got != tt.file {
				t.Fatalf("file = %q, want %q", got, tt.file)
			}
		})
	}
}

func TestParseConfigSetArgs(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		exit  ops.ExitCode
		file  gameconfig.File
		key   string
		value string
	}{
		{
			name: "valid", args: []string{
				"--file", "config", "--key", "loginTime", "--value", "45",
			},
			exit: ops.ExitOK, file: gameconfig.FileConfig,
			key: "loginTime", value: "45",
		},
		{
			name: "empty value", args: []string{
				"--file=server", "--key=password", "--value=",
			},
			exit: ops.ExitOK, file: gameconfig.FileServer,
			key: "password", value: "",
		},
		{name: "missing file", args: []string{"--key", "a", "--value", "b"}, exit: ops.ExitInvalidArgument},
		{name: "empty file", args: []string{"--file=", "--key", "a", "--value", "b"}, exit: ops.ExitInvalidArgument},
		{name: "unsupported file", args: []string{"--file", "world", "--key", "a", "--value", "b"}, exit: ops.ExitInvalidArgument},
		{name: "missing key", args: []string{"--file", "server", "--value", "b"}, exit: ops.ExitInvalidArgument},
		{name: "empty key", args: []string{"--file", "server", "--key=", "--value", "b"}, exit: ops.ExitInvalidArgument},
		{name: "missing value", args: []string{"--file", "server", "--key", "a"}, exit: ops.ExitInvalidArgument},
		{name: "unknown flag", args: []string{"--bogus"}, exit: ops.ExitInvalidArgument},
		{
			name: "extra argument", args: []string{
				"--file", "server", "--key", "a", "--value", "b", "extra",
			},
			exit: ops.ExitInvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseConfigSetArgs(tt.args)
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
			if got.file != tt.file || got.key != tt.key || got.value != tt.value {
				t.Fatalf("args = %#v, want %#v", got,
					configSetArgs{file: tt.file, key: tt.key, value: tt.value})
			}
		})
	}
}

func TestConfigCommandError(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		exit    ops.ExitCode
		details map[string]any
	}{
		{
			name: "invalid value",
			err: &gameconfig.InvalidError{
				File: gameconfig.FileConfig, Key: "loginTime", Message: "must be an integer",
			},
			exit: ops.ExitConfigInvalid,
			details: map[string]any{
				"file": "config", "key": "loginTime",
			},
		},
		{
			name: "invalid file only",
			err: &gameconfig.InvalidError{
				File: gameconfig.FileServer, Message: "bad file",
			},
			exit:    ops.ExitConfigInvalid,
			details: map[string]any{"file": "server"},
		},
		{
			name: "missing", err: fmt.Errorf("read: %w", fs.ErrNotExist),
			exit: ops.ExitNotFound,
		},
		{name: "other", err: errors.New("boom"), exit: ops.ExitInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := configCommandError("config set", tt.err)
			if got.Exit != tt.exit {
				t.Fatalf("exit = %d, want %d", got.Exit, tt.exit)
			}
			if !strings.HasPrefix(got.Message, "config set: ") {
				t.Fatalf("message = %q, want command prefix", got.Message)
			}
			if !reflect.DeepEqual(got.Details, tt.details) {
				t.Fatalf("details = %#v, want %#v", got.Details, tt.details)
			}
		})
	}
}

func TestWriteConfigHuman(t *testing.T) {
	var out bytes.Buffer
	err := writeConfigGetHuman(&out, configGetData{
		File: "config",
		Values: map[string]any{
			"port": 25565, "hostname": "Morrowind with friends",
			"password": "", "flags": []string{"a", "b"},
		},
		ReadOnly: []string{"gameSettings"},
	})
	if err != nil {
		t.Fatalf("write get human: %v", err)
	}
	want := "file: config\n" +
		"values:\n" +
		"  flags=[\"a\",\"b\"]\n" +
		"  hostname=Morrowind with friends\n" +
		"  password=\n" +
		"  port=25565\n" +
		"read_only: gameSettings\n"
	if out.String() != want {
		t.Fatalf("get output = %q, want %q", out.String(), want)
	}

	out.Reset()
	err = writeConfigSetHuman(&out, configSetData{
		File: "banlist", Key: "ipAddresses",
		Value: []any{"203.0.113.5"}, RestartRequired: true,
	})
	if err != nil {
		t.Fatalf("write set human: %v", err)
	}
	want = "file: banlist\nkey: ipAddresses\nvalue: [\"203.0.113.5\"]\nrestart_required: true\n"
	if out.String() != want {
		t.Fatalf("set output = %q, want %q", out.String(), want)
	}

	out.Reset()
	if err := writeConfigGetHuman(&out, configGetData{File: "server"}); err != nil {
		t.Fatalf("write empty get human: %v", err)
	}
	if !strings.HasSuffix(out.String(), "read_only: -\n") {
		t.Fatalf("empty read_only output = %q", out.String())
	}
}

func TestConfigGetIgnoresOpsLock(t *testing.T) {
	root := configUnitRoot(t)
	if err := os.MkdirAll(filepath.Join(root, ".locks"), 0o700); err != nil {
		t.Fatalf("mkdir .locks: %v", err)
	}
	held, err := lock.Acquire(filepath.Join(root, ".locks", "ops.lock"),
		lock.Exclusive, 0)
	if err != nil {
		t.Fatalf("hold lock: %v", err)
	}
	defer func() { _ = held.Close() }()

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--root", root, "--json", "--lock-timeout=0",
		"config", "get", "--file", "server",
	}, &stdout, &stderr)
	if code != int(ops.ExitOK) {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q",
			code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), `"file":"server"`) {
		t.Fatalf("stdout has no server snapshot: %q", stdout.String())
	}
}

func TestConfigSetLockedChangesNothing(t *testing.T) {
	root := configUnitRoot(t)
	path := filepath.Join(root, "configs", "config.lua")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config.lua: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".locks"), 0o700); err != nil {
		t.Fatalf("mkdir .locks: %v", err)
	}
	held, err := lock.Acquire(filepath.Join(root, ".locks", "ops.lock"),
		lock.Exclusive, 0)
	if err != nil {
		t.Fatalf("hold lock: %v", err)
	}
	defer func() { _ = held.Close() }()

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--root", root, "--json", "--lock-timeout=0",
		"config", "set", "--file", "config", "--key", "loginTime", "--value", "45",
	}, &stdout, &stderr)
	if code != int(ops.ExitLocked) {
		t.Fatalf("exit = %d, want %d; stdout=%q stderr=%q",
			code, ops.ExitLocked, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), `"code":"locked"`) {
		t.Fatalf("stdout has no locked error: %q", stdout.String())
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, before) {
		t.Fatalf("locked config set changed config.lua: %v", err)
	}
	if backups, _ := filepath.Glob(path + ".bak-*"); len(backups) != 0 {
		t.Fatalf("locked config set created backups: %v", backups)
	}
}
