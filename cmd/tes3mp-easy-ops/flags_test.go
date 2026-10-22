package main

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/ops"
)

func TestParseGlobalDefaults(t *testing.T) {
	opts, rest, help, err := parseGlobal([]string{"status"})
	if err != nil {
		t.Fatalf("parseGlobal(status) error: %v", err)
	}
	if help {
		t.Fatal("help = true, want false")
	}
	if !reflect.DeepEqual(rest, []string{"status"}) {
		t.Fatalf("rest = %#v, want [status]", rest)
	}
	if opts.root != defaultRoot {
		t.Fatalf("root = %q, want %q", opts.root, defaultRoot)
	}
	if opts.json {
		t.Fatal("json = true, want false")
	}
	if opts.dryRun {
		t.Fatal("dryRun = true, want false")
	}
	if opts.lockTimeout != defaultLockTimeout {
		t.Fatalf("lockTimeout = %v, want %v", opts.lockTimeout, defaultLockTimeout)
	}
	if opts.commandTimeout != defaultCommandTimeout {
		t.Fatalf("commandTimeout = %v, want %v",
			opts.commandTimeout, defaultCommandTimeout)
	}
}

func TestParseGlobalFlagsAnywhere(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want options
		rest []string
	}{
		{
			name: "before command",
			args: []string{
				"--root", "/tmp/root", "--json", "--dry-run",
				"--lock-timeout", "5s", "--command-timeout", "1m",
				"backups", "list",
			},
			want: options{
				root:           "/tmp/root",
				json:           true,
				dryRun:         true,
				lockTimeout:    5 * time.Second,
				commandTimeout: time.Minute,
			},
			rest: []string{"backups", "list"},
		},
		{
			name: "after command",
			args: []string{
				"backups", "list", "--root=/tmp/root", "--json=true",
				"--dry-run=false", "--lock-timeout=5s",
				"--command-timeout=1m",
			},
			want: options{
				root:           "/tmp/root",
				json:           true,
				dryRun:         false,
				lockTimeout:    5 * time.Second,
				commandTimeout: time.Minute,
			},
			rest: []string{"backups", "list"},
		},
		{
			name: "single dash",
			args: []string{
				"-root=/tmp/root", "-json", "-dry-run",
				"-lock-timeout=5s", "-command-timeout=1m", "status",
			},
			want: options{
				root:           "/tmp/root",
				json:           true,
				dryRun:         true,
				lockTimeout:    5 * time.Second,
				commandTimeout: time.Minute,
			},
			rest: []string{"status"},
		},
		{
			name: "mixed with command args",
			args: []string{
				"--json", "mods", "deploy", "--archive", "name",
			},
			want: options{
				root:           defaultRoot,
				json:           true,
				lockTimeout:    defaultLockTimeout,
				commandTimeout: defaultCommandTimeout,
			},
			rest: []string{"mods", "deploy", "--archive", "name"},
		},
		{
			name: "delimiter stops global parsing",
			args: []string{"--json", "--", "status", "--root", "x"},
			want: options{
				root:           defaultRoot,
				json:           true,
				lockTimeout:    defaultLockTimeout,
				commandTimeout: defaultCommandTimeout,
			},
			rest: []string{"status", "--root", "x"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, rest, help, err := parseGlobal(tt.args)
			if err != nil {
				t.Fatalf("parseGlobal(%v) error: %v", tt.args, err)
			}
			if help {
				t.Fatal("help = true, want false")
			}
			if opts != tt.want {
				t.Fatalf("options = %#v, want %#v", opts, tt.want)
			}
			if !reflect.DeepEqual(rest, tt.rest) {
				t.Fatalf("rest = %#v, want %#v", rest, tt.rest)
			}
		})
	}
}

func TestParseGlobalErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "empty root", args: []string{"--root=", "status"}, want: "--root"},
		{name: "missing root", args: []string{"--root"}, want: "root"},
		{name: "zero lock", args: []string{"--lock-timeout=0s", "status"}, want: "positive"},
		{name: "negative lock", args: []string{"--lock-timeout=-1s", "status"}, want: "positive"},
		{name: "bad lock", args: []string{"--lock-timeout=bad", "status"}, want: "invalid value"},
		{name: "zero command", args: []string{"--command-timeout=0s", "status"}, want: "positive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, err := parseGlobal(tt.args)
			if err == nil {
				t.Fatalf("parseGlobal(%v) error = nil", tt.args)
			}
			if err.Exit != ops.ExitInvalidArgument {
				t.Fatalf("exit = %d, want %d", err.Exit, ops.ExitInvalidArgument)
			}
			if err.Code != ops.CodeInvalidArgument {
				t.Fatalf("code = %q, want %q", err.Code, ops.CodeInvalidArgument)
			}
			if !strings.Contains(err.Message, tt.want) {
				t.Fatalf("message = %q, want substring %q", err.Message, tt.want)
			}
		})
	}
}

func TestParseGlobalLeavesUnknownFlagsForCommand(t *testing.T) {
	_, rest, help, err := parseGlobal([]string{"--unknown", "status"})
	if err != nil {
		t.Fatalf("parseGlobal error: %v", err)
	}
	if help {
		t.Fatal("help = true, want false")
	}
	if !reflect.DeepEqual(rest, []string{"--unknown", "status"}) {
		t.Fatalf("rest = %#v, want [--unknown status]", rest)
	}
}

func TestWantsJSON(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "absent", args: nil, want: false},
		{name: "long", args: []string{"--json"}, want: true},
		{name: "after command", args: []string{"status", "--json"}, want: true},
		{name: "true", args: []string{"--json=true"}, want: true},
		{name: "false", args: []string{"--json=false"}, want: false},
		{name: "last wins true", args: []string{"--json=false", "--json"}, want: true},
		{name: "last wins false", args: []string{"--json", "--json=false"}, want: false},
		{name: "after bad duration", args: []string{
			"--lock-timeout", "bad", "--json", "status",
		}, want: true},
		{name: "root value", args: []string{
			"--root", "--json", "status",
		}, want: false},
		{name: "after delimiter", args: []string{"--", "--json"}, want: false},
		{name: "bad bool", args: []string{"--json=bad"}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := wantsJSON(tt.args); got != tt.want {
				t.Fatalf("wantsJSON(%v) = %t, want %t", tt.args, got, tt.want)
			}
		})
	}
}
