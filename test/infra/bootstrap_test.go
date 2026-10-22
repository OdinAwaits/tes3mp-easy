package infra_test

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

type result struct {
	exit   int
	stdout string
	stderr string
}

func (r result) combined() string {
	return r.stdout + r.stderr
}

type fakeHost struct {
	t          *testing.T
	dir        string
	dataRoot   string
	osRelease  string
	procRoot   string
	systemdRun string
	bin        string
	env        map[string]string
}

func requireBash(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("bootstrap preflight tests require Linux")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bootstrap preflight tests require bash")
	}
}

func bootstrapPath(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "bootstrap.sh"))
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("bootstrap.sh not found at %s: %v", path, err)
	}
	return path
}

func newFakeHost(t *testing.T) *fakeHost {
	t.Helper()
	requireBash(t)

	base := t.TempDir()
	h := &fakeHost{
		t:          t,
		dir:        base,
		osRelease:  filepath.Join(base, "etc", "os-release"),
		procRoot:   filepath.Join(base, "proc"),
		systemdRun: filepath.Join(base, "run", "systemd", "system"),
		bin:        filepath.Join(base, "bin"),
		env:        make(map[string]string),
	}
	h.dataRoot = filepath.Join(base, "data", "tes3mp-easy")

	for _, dir := range []string{
		h.procRoot,
		filepath.Join(h.procRoot, "1"),
		h.systemdRun,
		h.bin,
		h.dataRoot,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	h.writeOS("debian", "12")
	h.writePID1("systemd")
	h.setRAM(1024)
	h.setSwap(512)
	h.setDisk(8192, 7168)
	h.writeCommands()

	return h
}

func (h *fakeHost) writeFile(path, content string, mode fs.FileMode) {
	h.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		h.t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		h.t.Fatalf("write %s: %v", path, err)
	}
}

func (h *fakeHost) writeOS(id, version string) {
	h.writeFile(h.osRelease, fmt.Sprintf(
		"ID=%s\nVERSION_ID=%q\n", id, version,
	), 0o644)
}

func (h *fakeHost) writePID1(name string) {
	h.writeFile(filepath.Join(h.procRoot, "1", "comm"), name+"\n", 0o644)
}

func (h *fakeHost) setRAM(mib int) {
	kb := mib * 1024
	h.writeFile(filepath.Join(h.procRoot, "meminfo"), fmt.Sprintf(
		"MemTotal:       %d kB\nMemFree:        %d kB\nMemAvailable:   %d kB\n",
		kb, kb/2, kb/2,
	), 0o644)
}

func (h *fakeHost) setSwap(mib int) {
	var content strings.Builder
	content.WriteString("Filename\tType\tSize\tUsed\tPriority\n")
	if mib > 0 {
		fmt.Fprintf(&content, "/swapfile\tfile\t%d\t0\t-2\n", mib*1024)
	}
	h.writeFile(filepath.Join(h.procRoot, "swaps"), content.String(), 0o644)
}

func (h *fakeHost) setDisk(totalMiB, freeMiB int) {
	h.env["TES3MP_FAKE_DISK_TOTAL_KB"] = fmt.Sprint(totalMiB * 1024)
	h.env["TES3MP_FAKE_DISK_FREE_KB"] = fmt.Sprint(freeMiB * 1024)
}

func (h *fakeHost) writeCommands() {
	h.writeFile(filepath.Join(h.bin, "cat"), `#!/bin/sh
exec /bin/cat "$@"
`, 0o755)
	h.writeFile(filepath.Join(h.bin, "id"), `#!/bin/sh
printf '%s\n' "${TES3MP_FAKE_EUID:-0}"
`, 0o755)
	h.writeFile(filepath.Join(h.bin, "uname"), `#!/bin/sh
printf '%s\n' "${TES3MP_FAKE_MACHINE:-x86_64}"
`, 0o755)
	h.writeFile(filepath.Join(h.bin, "df"), `#!/bin/sh
total=${TES3MP_FAKE_DISK_TOTAL_KB:-8388608}
free=${TES3MP_FAKE_DISK_FREE_KB:-7340032}
printf '%s\n' 'Filesystem 1024-blocks Used Available Capacity Mounted on'
printf '/dev/fake %s 0 %s 0%% /\n' "$total" "$free"
`, 0o755)
	h.writeFile(filepath.Join(h.bin, "systemctl"), `#!/bin/sh
if [ "${1:-}" = "--version" ]; then
    printf 'systemd 255\n'
    exit 0
fi
exit 0
`, 0o755)
}

func (h *fakeHost) removeCommand(name string) {
	_ = os.Remove(filepath.Join(h.bin, name))
}

func (h *fakeHost) removeSystemdRunDir() {
	_ = os.RemoveAll(h.systemdRun)
}

func (h *fakeHost) removeProcFile(name string) {
	_ = os.Remove(filepath.Join(h.procRoot, name))
}

func (h *fakeHost) removeOSRelease() {
	_ = os.Remove(h.osRelease)
}

func (h *fakeHost) run(t *testing.T, args ...string) result {
	t.Helper()

	scriptArgs := append([]string{bootstrapPath(t), "--root", h.dataRoot}, args...)
	cmd := exec.Command("bash", scriptArgs...)
	cmd.Env = []string{
		"PATH=" + h.bin,
		"LC_ALL=C",
		"NO_COLOR=1",
		"TES3MP_EASY_OS_RELEASE=" + h.osRelease,
		"TES3MP_EASY_PROC_ROOT=" + h.procRoot,
		"TES3MP_EASY_SYSTEMD_RUN_DIR=" + h.systemdRun,
	}
	for key, value := range h.env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exit := 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("run bootstrap: %v", err)
		}
		exit = exitErr.ExitCode()
	}

	return result{exit: exit, stdout: stdout.String(), stderr: stderr.String()}
}

func assertExit(t *testing.T, got result, want int) {
	t.Helper()
	if got.exit != want {
		t.Fatalf("exit = %d, want %d\nstdout:\n%s\nstderr:\n%s",
			got.exit, want, got.stdout, got.stderr)
	}
}

func assertContains(t *testing.T, text, want string) {
	t.Helper()
	if !strings.Contains(text, want) {
		t.Fatalf("output does not contain %q:\n%s", want, text)
	}
}

func (h *fakeHost) setEUID(uid int) {
	h.env["TES3MP_FAKE_EUID"] = fmt.Sprint(uid)
}

func (h *fakeHost) setMachine(machine string) {
	h.env["TES3MP_FAKE_MACHINE"] = machine
}

func TestBootstrapPreflightPasses(t *testing.T) {
	h := newFakeHost(t)
	got := h.run(t)

	assertExit(t, got, 0)
	if got.stderr != "" {
		t.Fatalf("stderr = %q, want empty", got.stderr)
	}
	for _, want := range []string{
		"root privileges",
		"OS: Debian 12",
		"architecture: x86_64",
		"systemd: PID 1 is systemd",
		"RAM: 1024 MiB total",
		"swap: 512 MiB total",
		"disk: 7168 MiB free of 8192 MiB",
		"Preflight passed: 7 checks, 0 warning(s), 0 error(s).",
		"Installation stages are not implemented yet",
	} {
		assertContains(t, got.stdout, want)
	}
}

func TestBootstrapCheckFlagMakesNoChanges(t *testing.T) {
	h := newFakeHost(t)
	before := snapshotTree(t, h.dir)

	got := h.run(t, "--check")

	assertExit(t, got, 0)
	if got.stderr != "" {
		t.Fatalf("stderr = %q, want empty", got.stderr)
	}
	assertContains(t, got.stdout, "Preflight complete (--check). No changes were made.")
	if after := snapshotTree(t, h.dir); !reflect.DeepEqual(before, after) {
		t.Fatalf("--check changed files:\nbefore=%#v\nafter=%#v", before, after)
	}
}

func TestBootstrapHelpAndUsage(t *testing.T) {
	t.Run("help", func(t *testing.T) {
		h := newFakeHost(t)
		got := h.run(t, "--help")

		assertExit(t, got, 0)
		assertContains(t, got.stdout, "Usage: sudo ./bootstrap.sh")
		if got.stderr != "" {
			t.Fatalf("stderr = %q, want empty", got.stderr)
		}
	})

	t.Run("unknown option", func(t *testing.T) {
		h := newFakeHost(t)
		got := h.run(t, "--bogus")

		assertExit(t, got, 2)
		assertContains(t, got.stderr, "unknown option")
		assertContains(t, got.stderr, "Usage: sudo ./bootstrap.sh")
	})

	t.Run("empty root", func(t *testing.T) {
		h := newFakeHost(t)
		got := h.run(t, "--root=")

		assertExit(t, got, 2)
		assertContains(t, got.stderr, "must not be empty")
	})

	t.Run("relative root", func(t *testing.T) {
		h := newFakeHost(t)
		got := h.run(t, "--root=relative")

		assertExit(t, got, 2)
		assertContains(t, got.stderr, "must be an absolute path")
	})
}

func TestBootstrapReportsRAMWarning(t *testing.T) {
	h := newFakeHost(t)
	h.setRAM(512)

	got := h.run(t)

	assertExit(t, got, 0)
	if got.stderr != "" {
		t.Fatalf("stderr = %q, want empty", got.stderr)
	}
	assertContains(t, got.stdout, "RAM: 512 MiB is below the recommended 640 MiB")
	assertContains(t, got.stdout, "Preflight passed with warnings: 7 checks, 1 warning(s), 0 error(s).")
}

func TestBootstrapAcceptsMinimumHardware(t *testing.T) {
	h := newFakeHost(t)
	h.setRAM(480)
	h.setSwap(480)
	h.setDisk(6144, 4608)

	got := h.run(t)

	assertExit(t, got, 0)
	if got.stderr != "" {
		t.Fatalf("stderr = %q, want empty", got.stderr)
	}
	assertContains(t, got.stdout, "RAM: 480 MiB total")
	assertContains(t, got.stdout, "swap: 480 MiB total")
	assertContains(t, got.stdout, "disk: 4608 MiB free of 6144 MiB")
	assertContains(t, got.stdout, "disk: 4608 MiB free is below the recommended 5120 MiB")
	assertContains(t, got.stdout, "Preflight passed with warnings: 7 checks, 2 warning(s), 0 error(s).")
}

func TestBootstrapFailures(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*fakeHost)
		want  string
	}{
		{
			name:  "not root",
			setup: func(h *fakeHost) { h.setEUID(1000) },
			want:  "root: bootstrap must run as root",
		},
		{
			name:  "missing os-release",
			setup: func(h *fakeHost) { h.removeOSRelease() },
			want:  "OS: cannot read",
		},
		{
			name:  "missing os id",
			setup: func(h *fakeHost) { h.writeOS("", "12") },
			want:  "OS: ID is missing",
		},
		{
			name:  "missing os version",
			setup: func(h *fakeHost) { h.writeOS("debian", "") },
			want:  "OS: VERSION_ID is missing",
		},
		{
			name:  "unsupported os",
			setup: func(h *fakeHost) { h.writeOS("alpine", "3.20") },
			want:  "OS: alpine is not supported",
		},
		{
			name:  "old debian",
			setup: func(h *fakeHost) { h.writeOS("debian", "11") },
			want:  "OS: Debian 11 is too old",
		},
		{
			name:  "old ubuntu",
			setup: func(h *fakeHost) { h.writeOS("ubuntu", "22.04") },
			want:  "OS: Ubuntu 22.04 is too old",
		},
		{
			name:  "wrong architecture",
			setup: func(h *fakeHost) { h.setMachine("aarch64") },
			want:  "architecture: aarch64 is not supported",
		},
		{
			name:  "pid1 is not systemd",
			setup: func(h *fakeHost) { h.writePID1("bash") },
			want:  "systemd: PID 1 is bash",
		},
		{
			name:  "systemd run dir missing",
			setup: func(h *fakeHost) { h.removeSystemdRunDir() },
			want:  "run/systemd/system is missing",
		},
		{
			name:  "systemctl missing",
			setup: func(h *fakeHost) { h.removeCommand("systemctl") },
			want:  "systemctl is not installed",
		},
		{
			name:  "ram below minimum",
			setup: func(h *fakeHost) { h.setRAM(479) },
			want:  "RAM: 479 MiB is below the 480 MiB minimum",
		},
		{
			name:  "meminfo missing",
			setup: func(h *fakeHost) { h.removeProcFile("meminfo") },
			want:  "RAM: cannot read",
		},
		{
			name:  "swap below minimum",
			setup: func(h *fakeHost) { h.setSwap(479) },
			want:  "swap: 479 MiB is below the 480 MiB minimum",
		},
		{
			name:  "swaps missing",
			setup: func(h *fakeHost) { h.removeProcFile("swaps") },
			want:  "swap: cannot read",
		},
		{
			name:  "disk too small",
			setup: func(h *fakeHost) { h.setDisk(4096, 4096) },
			want:  "disk: 4096 MiB total is below",
		},
		{
			name:  "disk too full",
			setup: func(h *fakeHost) { h.setDisk(8192, 4607) },
			want:  "disk: 4607 MiB free",
		},
		{
			name:  "df missing",
			setup: func(h *fakeHost) { h.removeCommand("df") },
			want:  "disk: cannot read free space",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newFakeHost(t)
			tt.setup(h)

			got := h.run(t)

			assertExit(t, got, 1)
			assertContains(t, got.stderr, tt.want)
			assertContains(t, got.stderr, "Preflight failed:")
			assertContains(t, got.stderr, "1 error(s)")
		})
	}
}

func TestBootstrapReportsAllFailures(t *testing.T) {
	h := newFakeHost(t)
	h.setEUID(1000)
	h.writeOS("alpine", "3.20")
	h.setMachine("aarch64")
	h.writePID1("bash")
	h.setRAM(256)
	h.setSwap(0)
	h.setDisk(4096, 4096)

	got := h.run(t)

	assertExit(t, got, 1)
	for _, want := range []string{
		"root: bootstrap must run as root",
		"OS: alpine is not supported",
		"architecture: aarch64 is not supported",
		"systemd: PID 1 is bash",
		"RAM: 256 MiB is below",
		"swap: 0 MiB is below",
		"disk: 4096 MiB total is below",
		"Preflight failed: 7 checks, 0 warning(s), 7 error(s).",
	} {
		assertContains(t, got.stderr, want)
	}
}

func TestBootstrapIsIdempotent(t *testing.T) {
	h := newFakeHost(t)
	before := snapshotTree(t, h.dir)

	first := h.run(t)
	second := h.run(t)

	assertExit(t, first, 0)
	assertExit(t, second, 0)
	if first.stderr != "" || second.stderr != "" {
		t.Fatalf("stderr = %q / %q, want empty", first.stderr, second.stderr)
	}
	if first.stdout != second.stdout {
		t.Fatalf("repeated run changed output:\nfirst:\n%s\nsecond:\n%s",
			first.stdout, second.stdout)
	}
	if after := snapshotTree(t, h.dir); !reflect.DeepEqual(before, after) {
		t.Fatalf("preflight changed files:\nbefore=%#v\nafter=%#v", before, after)
	}
}

type fileSnapshot struct {
	mode    fs.FileMode
	content string
}

func snapshotTree(t *testing.T, root string) map[string]fileSnapshot {
	t.Helper()

	snapshot := make(map[string]fileSnapshot)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			snapshot[rel] = fileSnapshot{mode: info.Mode()}
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		snapshot[rel] = fileSnapshot{mode: info.Mode(), content: string(data)}
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return snapshot
}
