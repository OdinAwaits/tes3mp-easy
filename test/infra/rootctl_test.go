package infra_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRootctlSourceUsesFixedSystemctlAndUnit(t *testing.T) {
	body := readRootctlSource(t)

	for _, want := range []string{
		"#!/bin/sh",
		"readonly SYSTEMCTL=/bin/systemctl",
		"readonly UNIT=tes3mp.service",
		"start | stop | restart",
		`exec "$SYSTEMCTL" "$1" "$UNIT"`,
	} {
		assertContains(t, body, want)
	}
}

func TestRootctlPassesOnlyValidatedVerbs(t *testing.T) {
	requireLinux(t)

	for _, verb := range []string{"start", "stop", "restart"} {
		t.Run(verb, func(t *testing.T) {
			script, argsFile := newRootctlHarness(t)

			got := runRootctl(t, script, argsFile, []string{verb})

			if got.exit != 0 {
				t.Fatalf("exit = %d, want 0\nstdout:\n%s\nstderr:\n%s", got.exit, got.stdout, got.stderr)
			}
			args, err := os.ReadFile(argsFile)
			if err != nil {
				t.Fatalf("read systemctl args: %v", err)
			}
			if want := verb + "\ntes3mp.service\n"; string(args) != want {
				t.Fatalf("systemctl args = %q, want %q", args, want)
			}
		})
	}
}

func TestRootctlRejectsMissingOrExtraArguments(t *testing.T) {
	requireLinux(t)

	tests := []struct {
		name string
		args []string
	}{
		{name: "missing", args: nil},
		{name: "extra", args: []string{"start", "extra"}},
		{name: "status", args: []string{"status"}},
		{name: "help", args: []string{"--help"}},
		{name: "explicit unit", args: []string{"start", "tes3mp.service"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script, argsFile := newRootctlHarness(t)

			got := runRootctl(t, script, argsFile, tt.args)

			if got.exit != 2 {
				t.Fatalf("exit = %d, want 2\nstdout:\n%s\nstderr:\n%s", got.exit, got.stdout, got.stderr)
			}
			assertContains(t, got.stderr, "Usage: tes3mp-easy-rootctl start|stop|restart")
			if _, err := os.Stat(argsFile); !os.IsNotExist(err) {
				t.Fatalf("systemctl was invoked for invalid args (stat err = %v)", err)
			}
		})
	}
}

func TestRootctlPropagatesSystemctlFailure(t *testing.T) {
	requireLinux(t)

	script, argsFile := newRootctlHarness(t)

	got := runRootctl(t, script, argsFile, []string{"restart"},
		"TES3MP_ROOTCTL_EXIT=7")

	if got.exit != 7 {
		t.Fatalf("exit = %d, want 7\nstdout:\n%s\nstderr:\n%s", got.exit, got.stdout, got.stderr)
	}
}

func readRootctlSource(t *testing.T) string {
	t.Helper()

	data, err := os.ReadFile(projectPath(t, "rootctl", "tes3mp-easy-rootctl"))
	if err != nil {
		t.Fatalf("read rootctl source: %v", err)
	}
	return string(data)
}

func newRootctlHarness(t *testing.T) (script string, argsFile string) {
	t.Helper()

	source := readRootctlSource(t)
	const marker = "readonly SYSTEMCTL=/bin/systemctl"
	if strings.Count(source, marker) != 1 {
		t.Fatalf("rootctl source must contain %q exactly once", marker)
	}

	dir := t.TempDir()
	fakeSystemctl := filepath.Join(dir, "systemctl")
	fake := `#!/bin/sh
printf '%s\n' "$@" >"${TES3MP_ROOTCTL_ARGS:?}"
exit "${TES3MP_ROOTCTL_EXIT:-0}"
`
	if err := os.WriteFile(fakeSystemctl, []byte(fake), 0o755); err != nil {
		t.Fatalf("write fake systemctl: %v", err)
	}

	argsFile = filepath.Join(dir, "systemctl.args")
	copied := strings.Replace(source, marker, "readonly SYSTEMCTL="+fakeSystemctl, 1)
	script = filepath.Join(dir, "tes3mp-easy-rootctl")
	if err := os.WriteFile(script, []byte(copied), 0o755); err != nil {
		t.Fatalf("write rootctl copy: %v", err)
	}

	return script, argsFile
}

func runRootctl(t *testing.T, script, argsFile string, args []string, extraEnv ...string) result {
	t.Helper()

	cmd := exec.Command(script, args...)
	cmd.Env = append(os.Environ(), "TES3MP_ROOTCTL_ARGS="+argsFile)
	cmd.Env = append(cmd.Env, extraEnv...)

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exit := 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("run rootctl: %v", err)
		}
		exit = exitErr.ExitCode()
	}

	return result{exit: exit, stdout: stdout.String(), stderr: stderr.String()}
}

func requireLinux(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("rootctl requires Linux")
	}
}
