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
		{name: "bad https address", args: []string{"--https-addr", "bad"}, want: "https address"},
		{name: "bad public address", args: []string{"--tls-mode", "files", "--public-address", "https://x"}, want: "URL scheme"},
		{name: "acme requires public ip", args: []string{"--https-addr", "127.0.0.1:8443"}, want: "required in acme mode"},
		{name: "acme rejects private ip", args: []string{"--public-address", "127.0.0.1"}, want: "public unicast"},
		{name: "empty root", args: []string{"--root="}, want: "--root"},
		{name: "empty data dir", args: []string{"--data-dir="}, want: "--data-dir"},
		{name: "empty https", args: []string{"--https-addr="}, want: "--https-addr"},
		{name: "empty tls mode", args: []string{"--tls-mode="}, want: "--tls-mode"},
		{name: "empty acme directory", args: []string{"--acme-directory="}, want: "--acme-directory"},
		{name: "empty acme profile", args: []string{"--acme-profile="}, want: "--acme-profile"},
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
		case panel.EnvTLSMode:
			return panel.TLSModeFiles
		case panel.EnvACMEDirectory:
			return "https://env.example/directory"
		case panel.EnvACMEProfile:
			return "env-profile"
		default:
			return ""
		}
	}
	cfg, version, err := parseConfig([]string{
		"--root", "/flag/root",
		"--https-addr", "127.0.0.1:8443",
		"--tls-mode", panel.TLSModeACME,
		"--acme-directory", "https://flag.example/directory",
		"--acme-email", "admin@example.test",
		"--acme-profile", "shortlived",
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
	if cfg.HTTPSAddr != "127.0.0.1:8443" {
		t.Fatalf("HTTPSAddr = %q", cfg.HTTPSAddr)
	}
	if cfg.PublicAddress != "198.51.100.1" {
		t.Fatalf("PublicAddress = %q", cfg.PublicAddress)
	}
	if cfg.TLSMode != panel.TLSModeACME ||
		cfg.ACMEDirectory != "https://flag.example/directory" ||
		cfg.ACMEEmail != "admin@example.test" ||
		cfg.ACMEProfile != "shortlived" {
		t.Fatalf("ACME config = %#v", cfg)
	}
}

func TestRunContextCancelledBeforeACMEIssuanceCreatesNoCertificate(t *testing.T) {
	root := t.TempDir()
	env := func(key string) string {
		switch key {
		case panel.EnvRoot:
			return root
		case panel.EnvPublicAddress:
			return "203.0.113.10"
		default:
			return ""
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var stdout, stderr bytes.Buffer
	if got := runContext(ctx, nil, env, &stdout, &stderr); got != 0 {
		t.Fatalf("runContext = %d, want 0; stderr=%s", got, stderr.String())
	}
	for _, name := range []string{"cert.pem", "key.pem"} {
		path := filepath.Join(root, "panel-data", "tls", name)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s exists after a cancelled start: %v", path, err)
		}
	}
	if strings.Contains(stderr.String(), "self-signed") {
		t.Fatalf("stderr = %q, want no self-signed certificate", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func noEnv(string) string { return "" }

func TestRunResetPassword(t *testing.T) {
	root := t.TempDir()
	args := []string{"reset-password", "--root", root}

	var firstOut, firstErr bytes.Buffer
	if got := run(args, noEnv, &firstOut, &firstErr); got != 0 {
		t.Fatalf("reset-password = %d; stderr=%s", got, firstErr.String())
	}
	if firstErr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", firstErr.String())
	}
	line, _, ok := strings.Cut(firstOut.String(), "\n")
	if !ok || !strings.HasPrefix(line, "temporary password: ") {
		t.Fatalf("stdout = %q", firstOut.String())
	}
	password := strings.TrimPrefix(line, "temporary password: ")
	if len(password) != 24 {
		t.Fatalf("password length = %d, want 24", len(password))
	}
	initial := filepath.Join(root, "panel-data", "initial-password.txt")
	data, err := os.ReadFile(initial)
	if err != nil {
		t.Fatalf("read initial password: %v", err)
	}
	if strings.TrimSpace(string(data)) != password {
		t.Fatalf("initial password = %q, want %q", strings.TrimSpace(string(data)), password)
	}

	var secondOut, secondErr bytes.Buffer
	if got := run(args, noEnv, &secondOut, &secondErr); got != 0 {
		t.Fatalf("second reset-password = %d; stderr=%s", got, secondErr.String())
	}
	if secondOut.String() == firstOut.String() {
		t.Fatal("repeated reset produced the same password")
	}
}

func TestRunResetPasswordRecoversCorruptAuthAndPreservesData(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "panel-data")
	files := map[string]string{
		filepath.Join(dataDir, "tls", "cert.pem"):            "certificate",
		filepath.Join(dataDir, "tls", "key.pem"):             "private key",
		filepath.Join(dataDir, "uploads", "pending-upload"):  "upload",
		filepath.Join(root, "backups", "mods", "old.tar.gz"): "backup",
		filepath.Join(dataDir, "auth.json"):                  "corrupt on purpose",
		filepath.Join(dataDir, "initial-password.txt"):       "old temporary password",
	}
	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	var stdout, stderr bytes.Buffer
	if got := run([]string{"reset-password", "--root", root}, noEnv,
		&stdout, &stderr); got != 0 {
		t.Fatalf("reset-password = %d; stderr=%s", got, stderr.String())
	}
	line, _, ok := strings.Cut(stdout.String(), "\n")
	if !ok {
		t.Fatalf("stdout = %q", stdout.String())
	}
	password := strings.TrimPrefix(line, "temporary password: ")
	if len(password) != 24 {
		t.Fatalf("password length = %d, want 24", len(password))
	}

	initialPath := filepath.Join(dataDir, "initial-password.txt")
	initial, err := os.ReadFile(initialPath)
	if err != nil {
		t.Fatalf("read initial password: %v", err)
	}
	if got := strings.TrimSpace(string(initial)); got != password {
		t.Fatalf("initial password = %q, want %q", got, password)
	}
	info, err := os.Stat(filepath.Join(dataDir, "auth.json"))
	if err != nil {
		t.Fatalf("stat auth.json: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("auth.json mode = %o, want 600", got)
	}
	authData, err := os.ReadFile(filepath.Join(dataDir, "auth.json"))
	if err != nil {
		t.Fatalf("read auth.json: %v", err)
	}
	if strings.Contains(string(authData), "corrupt on purpose") ||
		!strings.Contains(string(authData), `"algorithm": "argon2id"`) {
		t.Fatalf("auth.json was not replaced with a valid record: %q", authData)
	}

	for path, want := range files {
		if path == filepath.Join(dataDir, "auth.json") ||
			path == filepath.Join(dataDir, "initial-password.txt") {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read preserved file %s: %v", path, err)
		}
		if string(data) != want {
			t.Fatalf("%s = %q, want %q", path, data, want)
		}
	}
}

func TestRunResetPasswordRejectsBadFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := run([]string{"reset-password", "--data-dir="}, noEnv,
		&stdout, &stderr); got != 2 {
		t.Fatalf("reset-password --data-dir= = %d, want 2", got)
	}
	if !strings.Contains(stderr.String(), "--data-dir") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}
