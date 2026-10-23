package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	clientapp "github.com/Saagilat/tes3mp-easy/internal/app"
)

func envMap(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func failLaunch(t *testing.T) func(*clientapp.Service) error {
	t.Helper()
	return func(*clientapp.Service) error {
		t.Fatal("UI launched unexpectedly")
		return nil
	}
}

func TestRunHelp(t *testing.T) {
	for _, args := range [][]string{
		{"--help"},
		{"-h"},
		{"-help"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := run(args, envMap(nil), &stdout, &stderr, failLaunch(t)); got != 0 {
				t.Fatalf("run(%v) = %d, want 0", args, got)
			}
			if !strings.Contains(stdout.String(), "Usage: tes3mp-easy-client") {
				t.Fatalf("stdout = %q, want usage", stdout.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestRunVersionWithoutHome(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := run([]string{"--version"}, envMap(nil), &stdout, &stderr, failLaunch(t)); got != 0 {
		t.Fatalf("run --version = %d, want 0", got)
	}
	want := "tes3mp-easy-client " + clientapp.Version + " (TES3MP " + clientapp.TES3MPVersion + ")\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunRejectsBadFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "empty config", args: []string{"--config="}, want: "--config"},
		{name: "unknown flag", args: []string{"--unknown"}, want: "flag provided but not defined"},
		{name: "argument", args: []string{"status"}, want: "unexpected argument"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := run(test.args, envMap(nil), &stdout, &stderr, failLaunch(t)); got != 2 {
				t.Fatalf("run(%v) = %d, want 2", test.args, got)
			}
			if !strings.Contains(stderr.String(), test.want) {
				t.Fatalf("stderr = %q, want %q", stderr.String(), test.want)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestRunRejectsCorruptConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	var stdout, stderr bytes.Buffer
	if got := run([]string{"--config", path}, envMap(map[string]string{"HOME": t.TempDir()}),
		&stdout, &stderr, failLaunch(t)); got != 1 {
		t.Fatalf("run = %d, want 1", got)
	}
	if !strings.Contains(stderr.String(), "parse config") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestRunLaunchesUI(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "custom", "config.json")
	var stdout, stderr bytes.Buffer
	launched := false
	launch := func(service *clientapp.Service) error {
		launched = true
		if service.ConfigPath() != configPath {
			t.Fatalf("ConfigPath = %q, want %q", service.ConfigPath(), configPath)
		}
		if len(service.Config().Servers) != 0 {
			t.Fatalf("servers = %+v", service.Config().Servers)
		}
		return nil
	}
	if got := run([]string{"--config", configPath},
		envMap(map[string]string{"HOME": root, "LANG": "en_US.UTF-8"}),
		&stdout, &stderr, launch); got != 0 {
		t.Fatalf("run = %d; stderr=%s", got, stderr.String())
	}
	if !launched {
		t.Fatal("UI was not launched")
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestRunUsesDefaultConfigPath(t *testing.T) {
	home := t.TempDir()
	var stdout, stderr bytes.Buffer
	launch := func(service *clientapp.Service) error {
		want := filepath.Join(home, ".config", "tes3mp-easy", "config.json")
		if service.ConfigPath() != want {
			t.Fatalf("ConfigPath = %q, want %q", service.ConfigPath(), want)
		}
		return nil
	}
	if got := run(nil, envMap(map[string]string{"HOME": home}), &stdout, &stderr, launch); got != 0 {
		t.Fatalf("run = %d; stderr=%s", got, stderr.String())
	}
}

func TestRunReportsPathAndLaunchErrors(t *testing.T) {
	tests := []struct {
		name   string
		env    map[string]string
		launch func(*clientapp.Service) error
		want   string
	}{
		{
			name:   "missing home",
			env:    map[string]string{},
			launch: failLaunch(t),
			want:   "home directory",
		},
		{
			name: "launch error",
			env:  map[string]string{"HOME": t.TempDir()},
			launch: func(*clientapp.Service) error {
				return errors.New("display unavailable")
			},
			want: "display unavailable",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := run(nil, envMap(test.env), &stdout, &stderr, test.launch); got != 1 {
				t.Fatalf("run = %d, want 1", got)
			}
			if !strings.Contains(stderr.String(), test.want) {
				t.Fatalf("stderr = %q, want %q", stderr.String(), test.want)
			}
		})
	}
}

func TestRunRejectsNilLauncher(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := run(nil, envMap(map[string]string{"HOME": t.TempDir()}),
		&stdout, &stderr, nil); got != 1 {
		t.Fatalf("run = %d, want 1", got)
	}
	if !strings.Contains(stderr.String(), "launcher is nil") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
