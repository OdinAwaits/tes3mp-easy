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
	t           *testing.T
	dir         string
	dataRoot    string
	installRoot string
	stateDir    string
	commandLog  string
	osRelease   string
	procRoot    string
	systemdRun  string
	bin         string
	env         map[string]string
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

func projectPath(t *testing.T, parts ...string) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	return filepath.Join(append([]string{root}, parts...)...)
}

func bootstrapPath(t *testing.T) string {
	t.Helper()

	path := projectPath(t, "bootstrap.sh")
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
	h.installRoot = filepath.Join(base, "install")
	h.stateDir = filepath.Join(base, "state")
	h.commandLog = filepath.Join(base, "commands.log")

	for _, dir := range []string{
		h.procRoot,
		filepath.Join(h.procRoot, "1"),
		h.systemdRun,
		h.bin,
		h.dataRoot,
		h.installRoot,
		filepath.Join(h.stateDir, "groups"),
		filepath.Join(h.stateDir, "users"),
		filepath.Join(h.stateDir, "members"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	h.writeFile(filepath.Join(h.stateDir, "groups", "systemd-journal"), "", 0o644)

	h.writeOS("debian", "12")
	h.writePID1("systemd")
	h.setRAM(1024)
	h.setSwap(512)
	h.setDisk(8192, 7168)
	h.writeCommands()
	h.env["TES3MP_EASY_INSTALL_ROOT"] = h.installRoot
	h.env["TES3MP_EASY_ROOTCTL_SOURCE"] = projectPath(t, "rootctl", "tes3mp-easy-rootctl")
	h.env["TES3MP_EASY_SUDOERS_SOURCE"] = projectPath(t, "sudoers.d", "tes3mp-easy")
	h.env["TES3MP_FAKE_STATE_DIR"] = h.stateDir
	h.env["TES3MP_FAKE_COMMAND_LOG"] = h.commandLog

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
printf 'systemctl args=%s\n' "$*" >>"${TES3MP_FAKE_COMMAND_LOG:?}"
if [ "${TES3MP_FAKE_SYSTEMCTL_FAIL:-0}" = "1" ]; then
    printf 'systemctl failed\n' >&2
    exit 1
fi
exit 0
`, 0o755)
	h.writeFile(filepath.Join(h.bin, "systemd-analyze"), `#!/bin/sh
printf 'systemd-analyze args=%s\n' "$*" >>"${TES3MP_FAKE_COMMAND_LOG:?}"
if [ "${TES3MP_FAKE_SYSTEMD_ANALYZE_PENDING:-0}" = "1" ]; then
    unit=
    for arg in "$@"; do
        unit=$arg
    done
    unit=${unit##*/}
    case "$unit" in
        tes3mp.service) cmd=/opt/tes3mp-easy/tes3mp/tes3mp-server ;;
        tes3mp-panel.service) cmd=/opt/tes3mp-easy/bin/tes3mp-easy-panel ;;
        tes3mp-easy-backup.service) cmd=/opt/tes3mp-easy/bin/tes3mp-easy-ops ;;
        *) exit 0 ;;
    esac
    printf '%s: Command %s is not executable: No such file or directory\n' "$unit" "$cmd" >&2
    exit 1
fi
if [ "${TES3MP_FAKE_SYSTEMD_ANALYZE_FAIL:-0}" = "1" ]; then
    printf '%s\n' "${TES3MP_FAKE_SYSTEMD_ANALYZE_ERROR:-syntax error in fake unit}" >&2
    exit 1
fi
exit 0
`, 0o755)
	h.writeFile(filepath.Join(h.bin, "sysctl"), `#!/bin/sh
printf 'sysctl args=%s\n' "$*" >>"${TES3MP_FAKE_COMMAND_LOG:?}"
if [ "${TES3MP_FAKE_SYSCTL_FAIL:-0}" = "1" ]; then
    printf 'sysctl failed\n' >&2
    exit 1
fi
exit 0
`, 0o755)
	h.writeFile(filepath.Join(h.bin, "getent"), `#!/bin/sh
state=${TES3MP_FAKE_STATE_DIR:?}
kind=${1:-}
name=${2:-}
case "$kind" in
    group)
        [ -f "$state/groups/$name" ] || exit 2
        printf '%s:x:999:\n' "$name"
        ;;
    passwd)
        [ -f "$state/users/$name" ] || exit 2
        printf '%s:x:999:999::/nonexistent:/usr/sbin/nologin\n' "$name"
        ;;
    *)
        exit 2
        ;;
esac
`, 0o755)
	h.writeFile(filepath.Join(h.bin, "groupadd"), `#!/bin/sh
state=${TES3MP_FAKE_STATE_DIR:?}
name=
for arg in "$@"; do
    case "$arg" in
        --system) ;;
        *) name=$arg ;;
    esac
done
[ -n "$name" ] || exit 2
[ ! -e "$state/groups/$name" ] || exit 9
: >"$state/groups/$name"
printf 'groupadd name=%s\n' "$name" >>"${TES3MP_FAKE_COMMAND_LOG:?}"
`, 0o755)
	h.writeFile(filepath.Join(h.bin, "useradd"), `#!/bin/sh
state=${TES3MP_FAKE_STATE_DIR:?}
primary=
supplementary=
user=
while [ "$#" -gt 0 ]; do
    case "$1" in
        --gid)
            primary=$2
            shift 2
            ;;
        --groups)
            supplementary=$2
            shift 2
            ;;
        --home-dir | --shell)
            shift 2
            ;;
        --system | --no-create-home)
            shift
            ;;
        *)
            user=$1
            shift
            ;;
    esac
done
[ -n "$user" ] || exit 2
[ ! -e "$state/users/$user" ] || exit 9
printf '%s:x:999:999::/nonexistent:/usr/sbin/nologin\n' "$user" >"$state/users/$user"
printf 'useradd user=%s primary=%s supplementary=%s\n' \
    "$user" "$primary" "$supplementary" >>"${TES3MP_FAKE_COMMAND_LOG:?}"
`, 0o755)
	h.writeFile(filepath.Join(h.bin, "usermod"), `#!/bin/sh
state=${TES3MP_FAKE_STATE_DIR:?}
group=
user=
while [ "$#" -gt 0 ]; do
    case "$1" in
        -aG)
            group=$2
            shift 2
            ;;
        *)
            user=$1
            shift
            ;;
    esac
done
[ -n "$group" ] && [ -n "$user" ] || exit 2
command -p mkdir -p "$state/members"
members="$state/members/$user"
command -p touch "$members"
if ! command -p grep -qx "$group" "$members"; then
    printf '%s\n' "$group" >>"$members"
fi
printf 'usermod user=%s groups=%s\n' "$user" "$group" >>"${TES3MP_FAKE_COMMAND_LOG:?}"
`, 0o755)
	h.writeFile(filepath.Join(h.bin, "install"), `#!/bin/sh
set -eu
mode=
owner=
group=
isdir=0
while [ "$#" -gt 0 ]; do
    case "$1" in
        -d)
            isdir=1
            shift
            ;;
        -m)
            mode=$2
            shift 2
            ;;
        -o)
            owner=$2
            shift 2
            ;;
        -g)
            group=$2
            shift 2
            ;;
        --)
            shift
            break
            ;;
        -*)
            printf 'fake install: unsupported option %s\n' "$1" >&2
            exit 2
            ;;
        *)
            break
            ;;
    esac
done
[ -n "$mode" ] || {
    printf 'fake install: missing mode\n' >&2
    exit 2
}
if [ "$isdir" -eq 1 ]; then
    for path in "$@"; do
        command -p mkdir -p "$path"
        command -p chmod "$mode" "$path"
        printf 'install -d owner=%s group=%s mode=%s path=%s\n' \
            "$owner" "$group" "$mode" "$path" >>"${TES3MP_FAKE_COMMAND_LOG:?}"
    done
else
    [ "$#" -eq 2 ] || {
        printf 'fake install: expected source and destination\n' >&2
        exit 2
    }
    src=$1
    dest=$2
    command -p mkdir -p "$(command -p dirname "$dest")"
    command -p rm -f "$dest"
    command -p cp "$src" "$dest"
    command -p chmod "$mode" "$dest"
    printf 'install owner=%s group=%s mode=%s src=%s dest=%s\n' \
        "$owner" "$group" "$mode" "$src" "$dest" >>"${TES3MP_FAKE_COMMAND_LOG:?}"
fi
`, 0o755)
	h.writeFile(filepath.Join(h.bin, "visudo"), `#!/bin/sh
file=
for arg in "$@"; do
    file=$arg
done
[ -n "$file" ] && [ -f "$file" ] || exit 1
if [ "${TES3MP_FAKE_VISUDO_FAIL:-0}" = "1" ]; then
    printf 'syntax error in fake sudoers\n' >&2
    exit 1
fi
printf 'visudo file=%s\n' "$file" >>"${TES3MP_FAKE_COMMAND_LOG:?}"
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

func TestBootstrapPreflightPassesAndInstalls(t *testing.T) {
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
		"Installing the I02 privilege model",
		"group: tes3mp created",
		"group: tes3mp-panel created",
		"user: tes3mp created",
		"user: tes3mp-panel created",
		"rootctl: " + filepath.Join(h.installRoot, "usr/local/libexec/tes3mp-easy-rootctl"),
		"sudoers: " + filepath.Join(h.installRoot, "etc/sudoers.d/tes3mp-easy"),
		"Installing the I03 systemd units, journald limits and sysctl",
		"unit syntax: tes3mp.service",
		"unit syntax: tes3mp-panel.service",
		"unit syntax: tes3mp-easy-backup.service",
		"unit syntax: tes3mp-easy-backup.timer",
		"systemd: daemon reloaded",
		"journald: restarted with the TES3MP Easy limits",
		"sysctl: applied " + filepath.Join(h.installRoot, "etc/sysctl.d/99-tes3mp-easy.conf"),
		"Bootstrap stage I03 complete",
	} {
		assertContains(t, got.stdout, want)
	}

	for _, dir := range []struct {
		path   string
		perm   fs.FileMode
		setgid bool
	}{
		{h.dataRoot, 0o755, false},
		{filepath.Join(h.dataRoot, "configs"), 0o700, false},
		{filepath.Join(h.dataRoot, "players"), 0o750, false},
		{filepath.Join(h.dataRoot, "world"), 0o750, false},
		{filepath.Join(h.dataRoot, "world", "cell"), 0o750, false},
		{filepath.Join(h.dataRoot, "world", "world"), 0o750, false},
		{filepath.Join(h.dataRoot, "world", "map"), 0o750, false},
		{filepath.Join(h.dataRoot, "world", "recordstore"), 0o750, false},
		{filepath.Join(h.dataRoot, "world", "custom"), 0o750, false},
		{filepath.Join(h.dataRoot, "mods"), 0o750, false},
		{filepath.Join(h.dataRoot, "mods", "plugins"), 0o750, false},
		{filepath.Join(h.dataRoot, "mods", "scripts"), 0o750, false},
		{filepath.Join(h.dataRoot, "backups"), 0o750, false},
		{filepath.Join(h.dataRoot, "backups", "mods"), 0o750, false},
		{filepath.Join(h.dataRoot, "backups", "state"), 0o750, false},
		{filepath.Join(h.dataRoot, "panel-data"), 0o750, false},
		{filepath.Join(h.dataRoot, "panel-data", "tls"), 0o700, false},
		{filepath.Join(h.dataRoot, "panel-data", "uploads"), 0o770, true},
		{filepath.Join(h.dataRoot, ".locks"), 0o700, false},
		{filepath.Join(h.dataRoot, ".staging"), 0o700, false},
		{filepath.Join(h.installRoot, "usr/local/libexec"), 0o755, false},
		{filepath.Join(h.installRoot, "etc/sudoers.d"), 0o750, false},
		{filepath.Join(h.installRoot, "etc/systemd/system"), 0o755, false},
		{filepath.Join(h.installRoot, "etc/systemd/journald.conf.d"), 0o755, false},
		{filepath.Join(h.installRoot, "etc/sysctl.d"), 0o755, false},
	} {
		assertPathMode(t, dir.path, dir.perm, dir.setgid)
	}
	assertPathMode(t, filepath.Join(h.installRoot, "usr/local/libexec/tes3mp-easy-rootctl"), 0o750, false)
	assertPathMode(t, filepath.Join(h.installRoot, "etc/sudoers.d/tes3mp-easy"), 0o440, false)
	for _, file := range []string{
		"etc/systemd/system/tes3mp.service",
		"etc/systemd/system/tes3mp-panel.service",
		"etc/systemd/system/tes3mp-easy-backup.service",
		"etc/systemd/system/tes3mp-easy-backup.timer",
		"etc/systemd/journald.conf.d/tes3mp-easy.conf",
		"etc/sysctl.d/99-tes3mp-easy.conf",
	} {
		assertPathMode(t, filepath.Join(h.installRoot, file), 0o644, false)
	}

	logData, err := os.ReadFile(h.commandLog)
	if err != nil {
		t.Fatalf("read command log: %v", err)
	}
	log := string(logData)
	for _, want := range []string{
		"groupadd name=tes3mp",
		"groupadd name=tes3mp-panel",
		"useradd user=tes3mp primary=tes3mp supplementary=systemd-journal",
		"useradd user=tes3mp-panel primary=tes3mp-panel supplementary=tes3mp",
		"usermod user=tes3mp groups=tes3mp,systemd-journal",
		"usermod user=tes3mp-panel groups=tes3mp-panel,tes3mp",
		"install -d owner=tes3mp group=tes3mp mode=0700 path=" + filepath.Join(h.dataRoot, "configs"),
		"install -d owner=tes3mp-panel group=tes3mp mode=2770 path=" + filepath.Join(h.dataRoot, "panel-data/uploads"),
		"install owner=root group=tes3mp mode=0750 src=" + projectPath(t, "rootctl", "tes3mp-easy-rootctl"),
		"install owner=root group=root mode=0440 src=" + projectPath(t, "sudoers.d", "tes3mp-easy"),
		"visudo file=" + projectPath(t, "sudoers.d", "tes3mp-easy"),
		"systemd-analyze args=verify --man=no " + projectPath(t, "systemd", "tes3mp.service"),
		"systemd-analyze args=verify --man=no " + projectPath(t, "systemd", "tes3mp-easy-backup.timer"),
		"install owner=root group=root mode=0644 src=" + projectPath(t, "systemd", "tes3mp.service"),
		"install owner=root group=root mode=0644 src=" + projectPath(t, "systemd", "tes3mp-panel.service"),
		"install owner=root group=root mode=0644 src=" + projectPath(t, "systemd", "tes3mp-easy-backup.service"),
		"install owner=root group=root mode=0644 src=" + projectPath(t, "systemd", "tes3mp-easy-backup.timer"),
		"install owner=root group=root mode=0644 src=" + projectPath(t, "systemd", "journald.conf.d", "tes3mp-easy.conf"),
		"install owner=root group=root mode=0644 src=" + projectPath(t, "sysctl.d", "99-tes3mp-easy.conf"),
		"systemctl args=--no-ask-password daemon-reload",
		"systemctl args=--no-ask-password restart systemd-journald.service",
		"sysctl args=-q -p " + filepath.Join(h.installRoot, "etc/sysctl.d/99-tes3mp-easy.conf"),
	} {
		assertContains(t, log, want)
	}

	sudoers, err := os.ReadFile(filepath.Join(h.installRoot, "etc/sudoers.d/tes3mp-easy"))
	if err != nil {
		t.Fatalf("read installed sudoers: %v", err)
	}
	assertContains(t, string(sudoers), "tes3mp-panel ALL=(tes3mp) NOPASSWD: /opt/tes3mp-easy/bin/tes3mp-easy-ops")
	assertContains(t, string(sudoers), "tes3mp ALL=(root) NOPASSWD: /usr/local/libexec/tes3mp-easy-rootctl start")

	assertInstalledFile(t, h, "etc/systemd/system/tes3mp.service", "MemoryMax=448M", "OOMScoreAdjust=-500")
	assertInstalledFile(t, h, "etc/systemd/system/tes3mp-panel.service", "CapabilityBoundingSet=CAP_NET_BIND_SERVICE", "NoNewPrivileges=no")
	assertInstalledFile(t, h, "etc/systemd/system/tes3mp-easy-backup.service", "Type=oneshot", "TimeoutStartSec=9min", "Nice=19")
	assertInstalledFile(t, h, "etc/systemd/system/tes3mp-easy-backup.timer", "OnUnitActiveSec=5min", "AccuracySec=30s")
	assertInstalledFile(t, h, "etc/systemd/journald.conf.d/tes3mp-easy.conf", "SystemMaxUse=100M", "RuntimeMaxUse=32M")
	assertInstalledFile(t, h, "etc/sysctl.d/99-tes3mp-easy.conf", "vm.swappiness=10")
}

func assertInstalledFile(t *testing.T, h *fakeHost, rel string, wants ...string) {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(h.installRoot, rel))
	if err != nil {
		t.Fatalf("read installed %s: %v", rel, err)
	}
	for _, want := range wants {
		assertContains(t, string(data), want)
	}
}

func assertPathMode(t *testing.T, path string, perm fs.FileMode, setgid bool) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != perm {
		t.Fatalf("%s mode = %04o, want %04o", path, got, perm)
	}
	gotSetgid := info.Mode()&fs.ModeSetgid != 0
	if gotSetgid != setgid {
		t.Fatalf("%s setgid = %v, want %v", path, gotSetgid, setgid)
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

	t.Run("root filesystem", func(t *testing.T) {
		h := newFakeHost(t)
		got := h.run(t, "--root=/")

		assertExit(t, got, 2)
		assertContains(t, got.stderr, "must not be /")
	})

	t.Run("relative install root", func(t *testing.T) {
		h := newFakeHost(t)
		h.env["TES3MP_EASY_INSTALL_ROOT"] = "relative"

		got := h.run(t)

		assertExit(t, got, 2)
		assertContains(t, got.stderr, "TES3MP_EASY_INSTALL_ROOT must be absolute")
	})
}

func TestBootstrapInstallFailures(t *testing.T) {
	t.Run("systemd-journal group missing", func(t *testing.T) {
		h := newFakeHost(t)
		if err := os.Remove(filepath.Join(h.stateDir, "groups", "systemd-journal")); err != nil {
			t.Fatalf("remove systemd-journal state: %v", err)
		}

		got := h.run(t)

		assertExit(t, got, 1)
		assertContains(t, got.stderr, "group systemd-journal is missing")
		assertContains(t, got.stderr, "Installation failed.")
	})

	t.Run("visudo rejects the rule", func(t *testing.T) {
		h := newFakeHost(t)
		h.env["TES3MP_FAKE_VISUDO_FAIL"] = "1"

		got := h.run(t)

		assertExit(t, got, 1)
		assertContains(t, got.stderr, "syntax error in fake sudoers")
		assertContains(t, got.stderr, "visudo rejected")
		assertContains(t, got.stderr, "Installation failed.")
		if _, err := os.Stat(filepath.Join(h.installRoot, "etc/sudoers.d/tes3mp-easy")); !os.IsNotExist(err) {
			t.Fatalf("sudoers file exists after visudo rejection (stat err = %v)", err)
		}
	})
}

func TestBootstrapAcceptsPendingSystemdExecutables(t *testing.T) {
	h := newFakeHost(t)
	h.env["TES3MP_FAKE_SYSTEMD_ANALYZE_PENDING"] = "1"

	got := h.run(t)

	assertExit(t, got, 0)
	if got.stderr != "" {
		t.Fatalf("stderr = %q, want empty", got.stderr)
	}
	for _, unit := range []string{
		"tes3mp.service",
		"tes3mp-panel.service",
		"tes3mp-easy-backup.service",
	} {
		assertContains(t, got.stdout, "unit syntax: "+unit+" (runtime executable pending)")
	}
	assertInstalledFile(t, h, "etc/systemd/system/tes3mp-panel.service")
}

func TestBootstrapRejectsUnexpectedMissingSystemdExecutable(t *testing.T) {
	h := newFakeHost(t)
	h.env["TES3MP_FAKE_SYSTEMD_ANALYZE_FAIL"] = "1"
	h.env["TES3MP_FAKE_SYSTEMD_ANALYZE_ERROR"] = "tes3mp.service: Command /opt/tes3mp-easy/tes3mp/tes3mp-servr is not executable: No such file or directory"

	got := h.run(t)

	assertExit(t, got, 1)
	assertContains(t, got.stderr, "tes3mp-servr")
	assertContains(t, got.stderr, "systemd-analyze rejected")
	assertContains(t, got.stderr, "Installation failed.")
}

func TestBootstrapRejectsInvalidSystemdUnit(t *testing.T) {
	h := newFakeHost(t)
	h.env["TES3MP_FAKE_SYSTEMD_ANALYZE_FAIL"] = "1"
	h.env["TES3MP_FAKE_SYSTEMD_ANALYZE_ERROR"] = "tes3mp.service: Unknown key name 'Bogus' in section 'Service', ignoring."

	got := h.run(t)

	assertExit(t, got, 1)
	assertContains(t, got.stderr, "Unknown key name 'Bogus'")
	assertContains(t, got.stderr, "systemd-analyze rejected")
	assertContains(t, got.stderr, "Installation failed.")
	if _, err := os.Stat(filepath.Join(h.installRoot, "etc/systemd/system/tes3mp.service")); !os.IsNotExist(err) {
		t.Fatalf("unit exists after verification failure (stat err = %v)", err)
	}
}

func TestBootstrapSystemdFailureModes(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*fakeHost)
		want  string
	}{
		{
			name: "systemd-analyze missing",
			setup: func(h *fakeHost) {
				h.removeCommand("systemd-analyze")
			},
			want: "systemd-analyze is required",
		},
		{
			name: "sysctl missing",
			setup: func(h *fakeHost) {
				h.removeCommand("sysctl")
			},
			want: "sysctl is required",
		},
		{
			name: "unit source missing",
			setup: func(h *fakeHost) {
				h.env["TES3MP_EASY_SYSTEMD_SOURCE"] = filepath.Join(h.dir, "missing-systemd")
			},
			want: "systemd unit source not found",
		},
		{
			name: "journald source missing",
			setup: func(h *fakeHost) {
				h.env["TES3MP_EASY_JOURNALD_SOURCE"] = filepath.Join(h.dir, "missing-journald.conf")
			},
			want: "journald source not found",
		},
		{
			name: "sysctl source missing",
			setup: func(h *fakeHost) {
				h.env["TES3MP_EASY_SYSCTL_SOURCE"] = filepath.Join(h.dir, "missing-sysctl.conf")
			},
			want: "sysctl source not found",
		},
		{
			name: "systemd-analyze fails without diagnostics",
			setup: func(h *fakeHost) {
				h.env["TES3MP_FAKE_SYSTEMD_ANALYZE_FAIL"] = "1"
				h.env["TES3MP_FAKE_SYSTEMD_ANALYZE_ERROR"] = ""
			},
			want: "systemd-analyze rejected",
		},
		{
			name: "daemon-reload fails",
			setup: func(h *fakeHost) {
				h.env["TES3MP_FAKE_SYSTEMCTL_FAIL"] = "1"
			},
			want: "command failed: systemctl --no-ask-password daemon-reload",
		},
		{
			name: "sysctl apply fails",
			setup: func(h *fakeHost) {
				h.env["TES3MP_FAKE_SYSCTL_FAIL"] = "1"
			},
			want: "command failed: sysctl -q -p ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newFakeHost(t)
			tt.setup(h)

			got := h.run(t)

			assertExit(t, got, 1)
			assertContains(t, got.stderr, tt.want)
			assertContains(t, got.stderr, "Installation failed.")
		})
	}
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

func TestBootstrapInstallIsIdempotent(t *testing.T) {
	h := newFakeHost(t)

	first := h.run(t)
	assertExit(t, first, 0)
	if first.stderr != "" {
		t.Fatalf("first stderr = %q, want empty", first.stderr)
	}

	before := snapshotInstalledState(t, h)
	if err := os.Remove(h.commandLog); err != nil && !os.IsNotExist(err) {
		t.Fatalf("clear command log: %v", err)
	}

	second := h.run(t)
	assertExit(t, second, 0)
	if second.stderr != "" {
		t.Fatalf("second stderr = %q, want empty", second.stderr)
	}

	after := snapshotInstalledState(t, h)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("repeated install changed files:\nbefore=%#v\nafter=%#v", before, after)
	}

	logData, err := os.ReadFile(h.commandLog)
	if err != nil {
		t.Fatalf("read command log: %v", err)
	}
	for _, unexpected := range []string{"groupadd ", "useradd "} {
		if strings.Contains(string(logData), unexpected) {
			t.Fatalf("second run repeated %q:\n%s", unexpected, logData)
		}
	}
	assertContains(t, second.stdout, "group: tes3mp already exists")
	assertContains(t, second.stdout, "user: tes3mp already exists")
	assertContains(t, second.stdout, "user: tes3mp-panel already exists")
}

func snapshotInstalledState(t *testing.T, h *fakeHost) map[string]fileSnapshot {
	t.Helper()

	snapshot := make(map[string]fileSnapshot)
	for _, root := range []string{h.dataRoot, h.installRoot, h.stateDir} {
		for rel, entry := range snapshotTree(t, root) {
			snapshot[root+"\x00"+rel] = entry
		}
	}
	return snapshot
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
