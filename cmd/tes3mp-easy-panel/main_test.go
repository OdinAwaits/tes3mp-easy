package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Saagilat/tes3mp-easy/internal/panel"
)

func TestRunHelp(t *testing.T) {
	for _, args := range [][]string{
		{"--help"},
		{"-h"},
		{"-help"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := run(args, noEnv, &stdout, &stderr); got != 0 {
				t.Fatalf("run(%v) = %d, want 0", args, got)
			}
			if !strings.Contains(stdout.String(), "Usage: tes3mp-easy-panel") {
				t.Fatalf("stdout = %q, want usage", stdout.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestRunVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := run([]string{"--version"}, noEnv, &stdout, &stderr); got != 0 {
		t.Fatalf("run --version = %d, want 0", got)
	}
	want := "tes3mp-easy-panel " + panel.Version + "\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "bad http address", args: []string{"--http-addr", "bad"}, want: "http address"},
		{name: "bad https address", args: []string{"--https-addr", "bad"}, want: "https address"},
		{name: "bad public address", args: []string{"--public-address", "https://x"}, want: "URL scheme"},
		{name: "same address", args: []string{"--http-addr", "127.0.0.1:8080", "--https-addr", "127.0.0.1:8080"}, want: "must differ"},
		{name: "empty root", args: []string{"--root="}, want: "--root"},
		{name: "empty data dir", args: []string{"--data-dir="}, want: "--data-dir"},
		{name: "empty http", args: []string{"--http-addr="}, want: "--http-addr"},
		{name: "empty https", args: []string{"--https-addr="}, want: "--https-addr"},
		{name: "empty cert", args: []string{"--tls-cert="}, want: "--tls-cert"},
		{name: "empty key", args: []string{"--tls-key="}, want: "--tls-key"},
		{name: "unexpected argument", args: []string{"status"}, want: "unexpected argument"},
		{name: "unknown flag", args: []string{"--unknown"}, want: "flag provided but not defined"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := run(tt.args, noEnv, &stdout, &stderr); got != 2 {
				t.Fatalf("run(%v) = %d, want 2", tt.args, got)
			}
			if !strings.Contains(stderr.String(), tt.want) {
				t.Fatalf("stderr = %q, want %q", stderr.String(), tt.want)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestParseConfigFlagsOverrideEnv(t *testing.T) {
	env := func(key string) string {
		switch key {
		case panel.EnvRoot:
			return "/env/root"
		case panel.EnvPublicAddress:
			return "198.51.100.1"
		default:
			return ""
		}
	}
	cfg, version, err := parseConfig([]string{
		"--root", "/flag/root",
		"--https-addr", "127.0.0.1:8443",
		"--version",
	}, env)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if !version {
		t.Fatal("version = false")
	}
	if cfg.Root != "/flag/root" {
		t.Fatalf("Root = %q", cfg.Root)
	}
	if cfg.DataDir != filepath.Join("/flag/root", "panel-data") {
		t.Fatalf("DataDir = %q", cfg.DataDir)
	}
	if cfg.HTTPSAddr != "127.0.0.1:8443" || cfg.HTTPAddr != panel.DefaultHTTPAddr {
		t.Fatalf("addresses = %q, %q", cfg.HTTPAddr, cfg.HTTPSAddr)
	}
	if cfg.PublicAddress != "198.51.100.1" {
		t.Fatalf("PublicAddress = %q", cfg.PublicAddress)
	}
}

func TestRunContextGeneratesTLSForEnvRoot(t *testing.T) {
	root := t.TempDir()
	env := func(key string) string {
		if key == panel.EnvRoot {
			return root
		}
		return ""
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var stdout, stderr bytes.Buffer
	if got := runContext(ctx, nil, env, &stdout, &stderr); got != 0 {
		t.Fatalf("runContext = %d, want 0; stderr=%s", got, stderr.String())
	}
	for _, name := range []string{"cert.pem", "key.pem"} {
		path := filepath.Join(root, "panel-data", "tls", name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
	}
	if !strings.Contains(stderr.String(), "generated self-signed certificate") {
		t.Fatalf("stderr = %q, want self-signed warning", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func noEnv(string) string { return "" }
