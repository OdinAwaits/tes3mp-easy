package infra_test

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
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

	"github.com/Saagilat/tes3mp-easy/internal/archive"
	"github.com/Saagilat/tes3mp-easy/test/fixtures"
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
	t              *testing.T
	dir            string
	dataRoot       string
	installRoot    string
	stateDir       string
	commandLog     string
	osRelease      string
	procRoot       string
	systemdRun     string
	bin            string
	runtimeArchive string
	versionFile    string
	env            map[string]string
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
	h.runtimeArchive = filepath.Join(base, "fixtures", "tes3mp-server-0.8.1.tar.gz")
	h.versionFile = filepath.Join(base, "fixtures", "tes3mp-version.txt")

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
	h.writeRuntimeFixture()
	h.writeCommands()
	h.env["TES3MP_EASY_INSTALL_ROOT"] = h.installRoot
	h.env["TES3MP_EASY_ROOTCTL_SOURCE"] = projectPath(t, "rootctl", "tes3mp-easy-rootctl")
	h.env["TES3MP_EASY_SUDOERS_SOURCE"] = projectPath(t, "sudoers.d", "tes3mp-easy")
	h.env["TES3MP_EASY_VERSION_FILE"] = h.versionFile
	h.env["TES3MP_FAKE_STATE_DIR"] = h.stateDir
	h.env["TES3MP_FAKE_COMMAND_LOG"] = h.commandLog
	h.env["TES3MP_FAKE_CURL_SOURCE"] = h.runtimeArchive
	h.env["TES3MP_EASY_PUBLIC_ADDRESS"] = "203.0.113.10"

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

func (h *fakeHost) writeRuntimeFixture() {
	h.t.Helper()

	dir := func(name string) fixtures.Entry {
		return fixtures.Entry{Name: "TES3MP-server/" + name, Type: tar.TypeDir, Mode: 0o755}
	}
	file := func(name, body string, mode int64) fixtures.Entry {
		return fixtures.Entry{Name: "TES3MP-server/" + name, Body: body, Mode: mode}
	}
	entries := []fixtures.Entry{
		dir("lib"),
		file("lib/libboost_system.so.1.67.0", "synthetic library\n", 0o644),
		file("tes3mp-server", "#!/bin/sh\nexit 0\n", 0o755),
		file("tes3mp-server.x86_64", "#!/bin/sh\nexit 0\n", 0o755),
		file("tes3mp-prelaunch", "#!/bin/sh\nexit 0\n", 0o755),
		file("openmw.cfg", "resources = ./resources\n", 0o644),
		file("tes3mp-server-default.cfg", "[General]\nhome = ./server\n", 0o644),
		dir("server"),
		dir("server/scripts"),
		file("server/scripts/config.lua", "config = {}\n", 0o644),
		file("server/scripts/customScripts.lua", "-- custom scripts\n", 0o644),
		file("server/scripts/serverCore.lua", "require(\"config\")\n", 0o644),
		dir("server/scripts/custom"),
		file("server/scripts/custom/.gitkeep", "", 0o644),
		dir("server/data"),
		dir("server/data/player"),
		file("server/data/player/.gitkeep", "", 0o644),
		dir("server/data/cell"),
		file("server/data/cell/.gitkeep", "", 0o644),
		dir("server/data/world"),
		file("server/data/world/.gitkeep", "", 0o644),
		dir("server/data/map"),
		file("server/data/map/.gitkeep", "", 0o644),
		dir("server/data/recordstore"),
		file("server/data/recordstore/.gitkeep", "", 0o644),
		dir("server/data/custom"),
		file("server/data/custom/.gitkeep", "", 0o644),
		file("server/data/requiredDataFiles.json", "[]\n", 0o644),
		file("server/data/banlist.json", "{\"playerNames\":[],\"ipAddresses\":[]}\n", 0o644),
	}
	data := fixtures.TarGz(h.t, entries)
	if err := os.MkdirAll(filepath.Dir(h.runtimeArchive), 0o755); err != nil {
		h.t.Fatalf("mkdir runtime fixture: %v", err)
	}
	if err := os.WriteFile(h.runtimeArchive, data, 0o644); err != nil {
		h.t.Fatalf("write runtime fixture: %v", err)
	}

	h.writeRuntimeVersion(h.runtimeArchive)
}

func (h *fakeHost) writeRuntimeVersion(archivePath string) {
	h.t.Helper()

	data, err := os.ReadFile(archivePath)
	if err != nil {
		h.t.Fatalf("read runtime archive: %v", err)
	}
	sum := sha256.Sum256(data)
	h.writeFile(h.versionFile, fmt.Sprintf(
		"TES3MP_VERSION=0.8.1\nTES3MP_URL=https://example.invalid/tes3mp-server-0.8.1.tar.gz\nTES3MP_SHA256=%x\n",
		sum,
	), 0o644)
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
case " $* " in
    *" is-failed --quiet tes3mp-panel.service "*)
        if [ "${TES3MP_FAKE_PANEL_FAILED:-0}" = "1" ]; then
            exit 0
        fi
        exit 1
        ;;
    *" reset-failed tes3mp-panel.service "*)
        if [ "${TES3MP_FAKE_PANEL_FAILED:-0}" != "1" ]; then
            printf 'Failed to reset failed state of unit tes3mp-panel.service: Unit tes3mp-panel.service not loaded.\n' >&2
            exit 1
        fi
        ;;
    *" start tes3mp.service "*)
        if [ "${TES3MP_FAKE_SERVICE_START_FAIL:-0}" = "1" ]; then
            printf 'fake tes3mp.service start failed\n' >&2
            exit 1
        fi
        ;;
    *" enable --now tes3mp-easy-backup.timer "*)
        if [ "${TES3MP_FAKE_TIMER_FAIL:-0}" = "1" ]; then
            printf 'fake backup timer start failed\n' >&2
            exit 1
        fi
        ;;
    *" is-active "*)
        if [ -n "${TES3MP_FAKE_INACTIVE_UNIT:-}" ]; then
            case " $* " in
                *" ${TES3MP_FAKE_INACTIVE_UNIT} "*)
                    exit 3
                    ;;
            esac
        fi
        ;;
esac
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
	h.writeFile(filepath.Join(h.bin, "apt-get"), `#!/bin/sh
printf 'apt-get args=%s\n' "$*" >>"${TES3MP_FAKE_COMMAND_LOG:?}"
if [ "${TES3MP_FAKE_APT_FAIL:-0}" = "1" ]; then
    printf 'apt-get failed\n' >&2
    exit 1
fi
`, 0o755)
	h.writeFile(filepath.Join(h.bin, "curl"), `#!/bin/sh
out=
original=$*
while [ "$#" -gt 0 ]; do
    case "$1" in
        -o)
            out=$2
            shift 2
            ;;
        *)
            shift
            ;;
    esac
done
if [ -z "$out" ]; then
    case " $original " in
        *" https://127.0.0.1/healthz "*)
            exit 0
            ;;
        *" https://api.ipify.org "* | *" https://ifconfig.me/ip "*)
            if [ -n "${TES3MP_FAKE_PUBLIC_IP:-}" ]; then
                printf '%s\n' "$TES3MP_FAKE_PUBLIC_IP"
                exit 0
            fi
            ;;
    esac
    exit 2
fi
printf 'curl out=%s\n' "$out" >>"${TES3MP_FAKE_COMMAND_LOG:?}"
if [ "${TES3MP_FAKE_CURL_FAIL:-0}" = "1" ]; then
    printf 'curl failed\n' >&2
    exit 22
fi
[ -f "${TES3MP_FAKE_CURL_SOURCE:?}" ] || exit 22
command -p cp "${TES3MP_FAKE_CURL_SOURCE}" "$out"
`, 0o755)
	h.writeFile(filepath.Join(h.bin, "chown"), `#!/bin/sh
printf 'chown args=%s\n' "$*" >>"${TES3MP_FAKE_COMMAND_LOG:?}"
if [ "${TES3MP_FAKE_CHOWN_FAIL:-0}" = "1" ]; then
    printf 'chown failed\n' >&2
    exit 1
fi
exit 0
`, 0o755)
	h.writeFile(filepath.Join(h.bin, "fallocate"), `#!/bin/sh
set -eu
size=
path=
while [ "$#" -gt 0 ]; do
    case "$1" in
        -l)
            size=$2
            shift 2
            ;;
        *)
            path=$1
            shift
            ;;
    esac
done
[ -n "$path" ] || exit 2
: >"$path"
printf 'fallocate size=%s path=%s\n' "$size" "$path" >>"${TES3MP_FAKE_COMMAND_LOG:?}"
if [ "${TES3MP_FAKE_FALLOCATE_FAIL:-0}" = "1" ]; then
    printf 'fallocate failed\n' >&2
    exit 1
fi
`, 0o755)
	h.writeFile(filepath.Join(h.bin, "dd"), `#!/bin/sh
set -eu
out=
while [ "$#" -gt 0 ]; do
    case "$1" in
        of=*)
            out=${1#of=}
            ;;
    esac
    shift
done
[ -n "$out" ] || exit 2
: >"$out"
printf 'dd out=%s\n' "$out" >>"${TES3MP_FAKE_COMMAND_LOG:?}"
`, 0o755)
	h.writeFile(filepath.Join(h.bin, "mkswap"), `#!/bin/sh
printf 'mkswap path=%s\n' "$1" >>"${TES3MP_FAKE_COMMAND_LOG:?}"
if [ "${TES3MP_FAKE_MKSWAP_FAIL:-0}" = "1" ]; then
    printf 'mkswap failed\n' >&2
    exit 1
fi
`, 0o755)
	h.writeFile(filepath.Join(h.bin, "swapon"), `#!/bin/sh
path=${1:-}
[ -n "$path" ] || exit 2
printf 'swapon path=%s\n' "$path" >>"${TES3MP_FAKE_COMMAND_LOG:?}"
if [ "${TES3MP_FAKE_SWAPON_FAIL:-0}" = "1" ]; then
    printf 'swapon failed\n' >&2
    exit 1
fi
printf '%s file 524284 0 -2\n' "$path" >>"${TES3MP_EASY_PROC_ROOT:?}/swaps"
`, 0o755)
	h.writeFile(filepath.Join(h.bin, "ss"), `#!/bin/sh
printf 'ss args=%s\n' "$*" >>"${TES3MP_FAKE_COMMAND_LOG:?}"
if [ "${TES3MP_FAKE_SS_BUSY:-0}" = "1" ]; then
    printf 'LISTEN 0 128 *:80 *:*\n'
fi
exit 0
`, 0o755)
	for name, command := range map[string]string{
		"chmod":     "chmod",
		"date":      "date",
		"gzip":      "gzip",
		"ln":        "ln",
		"mkdir":     "mkdir",
		"mktemp":    "mktemp",
		"mv":        "mv",
		"readlink":  "readlink",
		"rm":        "rm",
		"sha256sum": "sha256sum",
		"tar":       "tar",
		"tr":        "tr",
	} {
		h.writeFile(filepath.Join(h.bin, name), fmt.Sprintf(`#!/bin/sh
exec /usr/bin/%s "$@"
`, command), 0o755)
	}
}

func (h *fakeHost) removeCommand(name string) {
	_ = os.Remove(filepath.Join(h.bin, name))
}

func (h *fakeHost) writeServerBinaries() string {
	h.t.Helper()
	dir := filepath.Join(h.dir, "release-bin")
	for _, name := range []string{"tes3mp-easy-ops", "tes3mp-easy-panel"} {
		h.writeFile(filepath.Join(dir, name),
			"#!/bin/sh\nprintf '"+name+"\\n'\n", 0o755)
	}
	h.env["TES3MP_EASY_BIN_SOURCE"] = dir
	return dir
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
		"Installing the privilege model",
		"group: tes3mp created",
		"group: tes3mp-panel created",
		"user: tes3mp created",
		"user: tes3mp-panel created",
		"rootctl: " + filepath.Join(h.installRoot, "usr/local/libexec/tes3mp-easy-rootctl"),
		"sudoers: " + filepath.Join(h.installRoot, "etc/sudoers.d/tes3mp-easy"),
		"Installing systemd units, journald limits and sysctl",
		"unit syntax: tes3mp.service",
		"unit syntax: tes3mp-panel.service",
		"unit syntax: tes3mp-easy-backup.service",
		"unit syntax: tes3mp-easy-backup.timer",
		"systemd: daemon reloaded",
		"journald: restarted with the TES3MP Easy limits",
		"sysctl: applied " + filepath.Join(h.installRoot, "etc/sysctl.d/99-tes3mp-easy.conf"),
		"packages: TES3MP runtime dependencies installed",
		"runtime: TES3MP 0.8.1 archive verified (sha256)",
		"runtime: TES3MP 0.8.1 installed under " + filepath.Join(h.installRoot, "opt/tes3mp-easy/tes3mp"),
		"config: " + filepath.Join(h.dataRoot, "configs/tes3mp-server-default.cfg") + " seeded",
		"config: " + filepath.Join(h.dataRoot, "configs/config.lua") + " seeded",
		"config: " + filepath.Join(h.dataRoot, "configs/customScripts.lua") + " seeded",
		"mods: base " + filepath.Join(h.dataRoot, "mods/plugins/requiredDataFiles.json") + " created",
		"mods: created mods-init-",
		"canonical backups/mods/current.txt",
		"runtime link: " + filepath.Join(h.installRoot, "opt/tes3mp-easy/tes3mp/server/data/player"),
		"Runtime ready: TES3MP 0.8.1",
		"Starting TES3MP services",
		"service: tes3mp.service enabled for boot",
		"service: tes3mp.service active",
		"timer: tes3mp-easy-backup.timer enabled and active",
		"Bootstrap complete: TES3MP 0.8.1 is running.",
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
		{filepath.Join(h.dataRoot, "panel-data", "acme"), 0o700, false},
		{filepath.Join(h.dataRoot, "panel-data", "uploads"), 0o770, true},
		{filepath.Join(h.dataRoot, ".locks"), 0o700, false},
		{filepath.Join(h.dataRoot, ".staging"), 0o700, false},
		{filepath.Join(h.dataRoot, ".config"), 0o700, false},
		{filepath.Join(h.dataRoot, ".config", "openmw"), 0o700, false},
		{filepath.Join(h.dataRoot, ".local"), 0o700, false},
		{filepath.Join(h.dataRoot, ".local", "share"), 0o700, false},
		{filepath.Join(h.dataRoot, ".local", "share", "openmw"), 0o700, false},
		{filepath.Join(h.installRoot, "opt/tes3mp-easy"), 0o755, false},
		{filepath.Join(h.installRoot, "opt/tes3mp-easy/tes3mp"), 0o755, false},
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
	for _, file := range []string{
		"configs/tes3mp-server-default.cfg",
		"configs/config.lua",
		"configs/customScripts.lua",
		"configs/banlist.json",
	} {
		assertPathMode(t, filepath.Join(h.dataRoot, file), 0o600, false)
	}
	assertPathMode(t, filepath.Join(h.dataRoot, "mods/plugins/requiredDataFiles.json"), 0o640, false)
	markerPath := filepath.Join(h.installRoot, "opt/tes3mp-easy/tes3mp/.tes3mp-install-info")
	assertPathMode(t, markerPath, 0o644, false)
	marker, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("read runtime marker: %v", err)
	}
	version, err := os.ReadFile(h.versionFile)
	if err != nil {
		t.Fatalf("read version fixture: %v", err)
	}
	if string(marker) != string(version) {
		t.Fatalf("runtime marker = %q, want version file %q", marker, version)
	}

	runtimeDir := filepath.Join(h.installRoot, "opt/tes3mp-easy/tes3mp")
	for link, target := range map[string]string{
		"tes3mp-server-default.cfg":          filepath.Join(h.dataRoot, "configs/tes3mp-server-default.cfg"),
		"server/scripts/config.lua":          filepath.Join(h.dataRoot, "configs/config.lua"),
		"server/scripts/customScripts.lua":   filepath.Join(h.dataRoot, "configs/customScripts.lua"),
		"server/scripts/custom":              filepath.Join(h.dataRoot, "mods/scripts"),
		"server/data/player":                 filepath.Join(h.dataRoot, "players"),
		"server/data/cell":                   filepath.Join(h.dataRoot, "world/cell"),
		"server/data/world":                  filepath.Join(h.dataRoot, "world/world"),
		"server/data/map":                    filepath.Join(h.dataRoot, "world/map"),
		"server/data/recordstore":            filepath.Join(h.dataRoot, "world/recordstore"),
		"server/data/custom":                 filepath.Join(h.dataRoot, "world/custom"),
		"server/data/requiredDataFiles.json": filepath.Join(h.dataRoot, "mods/plugins/requiredDataFiles.json"),
		"server/data/banlist.json":           filepath.Join(h.dataRoot, "configs/banlist.json"),
	} {
		assertSymlink(t, filepath.Join(runtimeDir, link), target)
	}
	for _, archived := range []string{
		"server/scripts/custom/.gitkeep",
		"server/data/player/.gitkeep",
		"server/data/world/.gitkeep",
	} {
		if _, err := os.Lstat(filepath.Join(runtimeDir, archived)); !os.IsNotExist(err) {
			t.Fatalf("archive placeholder %s survived runtime wiring (err=%v)", archived, err)
		}
	}

	currentPath := filepath.Join(h.dataRoot, "backups/mods/current.txt")
	current, err := archive.ReadCurrentFile(currentPath)
	if err != nil {
		t.Fatalf("read canonical mods current.txt: %v", err)
	}
	if !strings.HasPrefix(current.Name, "mods-init-") {
		t.Fatalf("current.txt name = %q, want mods-init-*.tar.gz", current.Name)
	}
	modsArchive := filepath.Join(h.dataRoot, "backups/mods", current.Name)
	archiveData, err := os.ReadFile(modsArchive)
	if err != nil {
		t.Fatalf("read base mods archive: %v", err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(archiveData)); got != current.SHA256 {
		t.Fatalf("current.txt sha256 = %s, archive sha256 = %s", current.SHA256, got)
	}
	modsInfo, err := archive.ValidateModsFile(modsArchive)
	if err != nil {
		t.Fatalf("validate base mods archive: %v", err)
	}
	if modsInfo.Plugins != 0 || modsInfo.Scripts != 0 {
		t.Fatalf("base mods archive has %d plugins and %d scripts, want 0/0", modsInfo.Plugins, modsInfo.Scripts)
	}
	assertPathMode(t, modsArchive, 0o640, false)
	assertPathMode(t, currentPath, 0o640, false)

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
		"apt-get args=update",
		"apt-get args=install -y --no-install-recommends libluajit-5.1-2 ca-certificates openssl curl tar gzip rsync coreutils iproute2",
		"curl out=" + filepath.Join(h.installRoot, "opt/tes3mp-easy") + "/.tes3mp-0.8.1.tar.gz.",
		"chown args=-R root:root " + filepath.Join(h.installRoot, "opt/tes3mp-easy") + "/.tes3mp-0.8.1.",
		"systemctl args=--no-ask-password enable tes3mp.service",
		"systemctl args=--no-ask-password start tes3mp.service",
		"systemctl args=--no-ask-password is-active --quiet tes3mp.service",
		"systemctl args=--no-ask-password enable --now tes3mp-easy-backup.timer",
		"systemctl args=--no-ask-password is-active --quiet tes3mp-easy-backup.timer",
	} {
		assertContains(t, log, want)
	}
	for _, unwanted := range []string{
		" start tes3mp-panel.service",
		" enable tes3mp-panel.service",
		" start tes3mp-easy-backup.service",
	} {
		if strings.Contains(log, unwanted) {
			t.Fatalf("bootstrap started a service before its binary exists: %s\n%s", unwanted, log)
		}
	}
	serverStart := strings.Index(log, "start tes3mp.service")
	serverActive := strings.Index(log, "is-active --quiet tes3mp.service")
	timerStart := strings.Index(log, "enable --now tes3mp-easy-backup.timer")
	if serverStart < 0 || serverActive < 0 || timerStart < 0 ||
		serverActive < serverStart || timerStart < serverActive {
		t.Fatalf("services started out of order:\n%s", log)
	}

	sudoers, err := os.ReadFile(filepath.Join(h.installRoot, "etc/sudoers.d/tes3mp-easy"))
	if err != nil {
		t.Fatalf("read installed sudoers: %v", err)
	}
	assertContains(t, string(sudoers), "tes3mp-panel ALL=(tes3mp) NOPASSWD: /opt/tes3mp-easy/bin/tes3mp-easy-ops")
	assertContains(t, string(sudoers), "tes3mp ALL=(root) NOPASSWD: /usr/local/libexec/tes3mp-easy-rootctl start")

	assertInstalledFile(t, h, "etc/systemd/system/tes3mp.service", "MemoryMax=448M", "OOMScoreAdjust=-500")
	assertInstalledFile(t, h, "etc/systemd/system/tes3mp-panel.service", "CapabilityBoundingSet=CAP_NET_BIND_SERVICE CAP_SETUID CAP_SETGID CAP_AUDIT_WRITE CAP_SYS_RESOURCE", "NoNewPrivileges=no")
	assertInstalledFile(t, h, "etc/systemd/system/tes3mp-easy-backup.service", "Type=oneshot", "TimeoutStartSec=9min", "Nice=19", "ConditionPathExists=/opt/tes3mp-easy/bin/tes3mp-easy-ops")
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

func assertSymlink(t *testing.T, path, want string) {
	t.Helper()

	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("lstat %s: %v", path, err)
	}
	if info.Mode()&fs.ModeSymlink == 0 {
		t.Fatalf("%s is not a symlink", path)
	}
	got, err := os.Readlink(path)
	if err != nil {
		t.Fatalf("readlink %s: %v", path, err)
	}
	if got != want {
		t.Fatalf("%s -> %s, want %s", path, got, want)
	}
}

func TestBootstrapInstallsReleaseBinariesAndStartsPanel(t *testing.T) {
	h := newFakeHost(t)
	binDir := h.writeServerBinaries()

	got := h.run(t)

	assertExit(t, got, 0)
	if got.stderr != "" {
		t.Fatalf("stderr = %q, want empty", got.stderr)
	}
	for _, want := range []string{
		"Installing server binaries from " + binDir,
		"binary: " + filepath.Join(h.installRoot, "opt/tes3mp-easy/bin/tes3mp-easy-ops"),
		"binary: " + filepath.Join(h.installRoot, "opt/tes3mp-easy/bin/tes3mp-easy-panel"),
		"Configuring the panel TLS certificate",
		"network: TCP port 80 is free for ACME HTTP-01",
		"panel TLS: acme for 203.0.113.10 (" +
			filepath.Join(h.installRoot, "etc/tes3mp-easy/panel.env") + ")",
		"service: tes3mp-panel.service has no failed state to clear",
		"service: tes3mp-panel.service stopped before the ACME port check",
		"service: tes3mp-panel.service enabled for boot",
		"panel: HTTPS /healthz is ready",
		"service: tes3mp-panel.service active",
		"panel: open https://203.0.113.10/",
	} {
		assertContains(t, got.stdout, want)
	}

	for _, name := range []string{"tes3mp-easy-ops", "tes3mp-easy-panel"} {
		path := filepath.Join(h.installRoot, "opt/tes3mp-easy/bin", name)
		assertPathMode(t, path, 0o755, false)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		assertContains(t, string(data), name)
	}
	assertInstalledFile(t, h, "opt/tes3mp-easy/bin/tes3mp-easy-ops")
	assertInstalledFile(t, h, "opt/tes3mp-easy/bin/tes3mp-easy-panel")
	assertInstalledFile(t, h, "etc/tes3mp-easy/panel.env",
		"TES3MP_PANEL_TLS_MODE=acme",
		"TES3MP_PANEL_PUBLIC_ADDRESS=203.0.113.10")
	assertPathMode(t, filepath.Join(h.installRoot, "etc/tes3mp-easy/panel.env"),
		0o640, false)

	logData, err := os.ReadFile(h.commandLog)
	if err != nil {
		t.Fatalf("read command log: %v", err)
	}
	logText := string(logData)
	for _, want := range []string{
		"systemctl args=--no-ask-password is-failed --quiet tes3mp-panel.service",
		"systemctl args=--no-ask-password stop tes3mp-panel.service",
		"ss args=-H -ltn",
		"install owner=root group=tes3mp-panel mode=0640 src=",
		"dest=" + filepath.Join(h.installRoot, "etc/tes3mp-easy/panel.env"),
		"systemctl args=--no-ask-password start tes3mp.service",
		"systemctl args=--no-ask-password enable --now tes3mp-easy-backup.timer",
		"systemctl args=--no-ask-password enable tes3mp-panel.service",
		"systemctl args=--no-ask-password restart tes3mp-panel.service",
		"systemctl args=--no-ask-password is-active --quiet tes3mp-panel.service",
	} {
		assertContains(t, logText, want)
	}
	if strings.Contains(logText, "reset-failed tes3mp-panel.service") {
		t.Fatalf("reset-failed called for a panel that never failed:\n%s", logText)
	}
	if strings.Index(logText, "start tes3mp.service") >
		strings.Index(logText, "restart tes3mp-panel.service") {
		t.Fatalf("panel started before the game service:\n%s", logText)
	}
}

func TestBootstrapClearsFailedPanelBeforeACME(t *testing.T) {
	h := newFakeHost(t)
	h.writeServerBinaries()
	h.env["TES3MP_FAKE_PANEL_FAILED"] = "1"

	got := h.run(t)

	assertExit(t, got, 0)
	assertContains(t, got.stdout, "service: tes3mp-panel.service failed state cleared")

	logData, err := os.ReadFile(h.commandLog)
	if err != nil {
		t.Fatalf("read command log: %v", err)
	}
	logText := string(logData)
	for _, want := range []string{
		"systemctl args=--no-ask-password is-failed --quiet tes3mp-panel.service",
		"systemctl args=--no-ask-password reset-failed tes3mp-panel.service",
		"systemctl args=--no-ask-password stop tes3mp-panel.service",
	} {
		assertContains(t, logText, want)
	}
}

func TestBootstrapRejectsBusyPort80ForACME(t *testing.T) {
	h := newFakeHost(t)
	h.writeServerBinaries()
	h.env["TES3MP_FAKE_SS_BUSY"] = "1"

	got := h.run(t)

	assertExit(t, got, 1)
	assertContains(t, got.stderr, "TCP port 80 is already in use")
	assertContains(t, got.stderr, "Installation failed.")
}

func TestBootstrapRejectsInvalidPublicAddress(t *testing.T) {
	h := newFakeHost(t)
	h.writeServerBinaries()
	h.env["TES3MP_EASY_PUBLIC_ADDRESS"] = "not-an-ip"

	got := h.run(t)

	assertExit(t, got, 1)
	assertContains(t, got.stderr, "not a valid IPv4 address")
	assertContains(t, got.stderr, "Installation failed.")
}

func TestBootstrapRejectsNonCanonicalPublicAddress(t *testing.T) {
	h := newFakeHost(t)
	h.writeServerBinaries()
	h.env["TES3MP_EASY_PUBLIC_ADDRESS"] = "203.0.113.010"

	got := h.run(t)

	assertExit(t, got, 1)
	assertContains(t, got.stderr, "not a valid IPv4 address")
	assertContains(t, got.stderr, "Installation failed.")
}

func TestBootstrapRejectsNonPublicAddress(t *testing.T) {
	for _, address := range []string{"10.0.0.1", "100.64.0.1", "127.0.0.1", "240.0.0.1"} {
		t.Run(address, func(t *testing.T) {
			h := newFakeHost(t)
			h.writeServerBinaries()
			h.env["TES3MP_EASY_PUBLIC_ADDRESS"] = address

			got := h.run(t)

			assertExit(t, got, 1)
			assertContains(t, got.stderr, "not a public IPv4 address")
			assertContains(t, got.stderr, "Installation failed.")
		})
	}
}

func TestBootstrapRejectsIncompleteReleaseBinaries(t *testing.T) {
	h := newFakeHost(t)
	binDir := h.writeServerBinaries()
	if err := os.Remove(filepath.Join(binDir, "tes3mp-easy-panel")); err != nil {
		t.Fatalf("remove panel binary: %v", err)
	}

	got := h.run(t)

	assertExit(t, got, 1)
	assertContains(t, got.stderr, "release binary not found or not a regular file")
	ops := filepath.Join(h.installRoot, "opt/tes3mp-easy/bin/tes3mp-easy-ops")
	if _, err := os.Stat(ops); err == nil {
		t.Fatalf("bootstrap installed a partial binary set at %s", ops)
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
	if logData, err := os.ReadFile(h.commandLog); err == nil &&
		strings.Contains(string(logData), "start tes3mp.service") {
		t.Fatalf("--check started tes3mp.service:\n%s", logData)
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

func TestBootstrapCreatesSwapWhenMissing(t *testing.T) {
	h := newFakeHost(t)
	h.setRAM(542)
	h.setSwap(0)
	swapfile := filepath.Join(h.installRoot, "swapfile")
	fstab := filepath.Join(h.installRoot, "etc/fstab")

	first := h.run(t)
	assertExit(t, first, 0)
	if first.stderr != "" {
		t.Fatalf("stderr = %q, want empty", first.stderr)
	}
	assertContains(t, first.stdout, "RAM: 542 MiB is below the recommended 640 MiB")
	assertContains(t, first.stdout,
		"swap: 0 MiB is below the 480 MiB minimum; installer will create 512 MiB at "+swapfile)
	assertContains(t, first.stdout, "Preflight passed with warnings: 7 checks, 2 warning(s), 0 error(s).")
	assertContains(t, first.stdout, "swap: activated "+swapfile+" (512 MiB)")
	assertContains(t, first.stdout, "swap: recorded "+swapfile+" in "+fstab)

	assertInstalledFile(t, h, "etc/fstab", swapfile+" none swap sw 0 0")
	assertPathMode(t, swapfile, 0o600, false)

	logData, err := os.ReadFile(h.commandLog)
	if err != nil {
		t.Fatalf("read command log: %v", err)
	}
	for _, want := range []string{
		"fallocate size=512M path=" + swapfile,
		"mkswap path=" + swapfile,
		"swapon path=" + swapfile,
	} {
		assertContains(t, string(logData), want)
	}

	second := h.run(t)
	assertExit(t, second, 0)
	if second.stderr != "" {
		t.Fatalf("second stderr = %q, want empty", second.stderr)
	}
	assertContains(t, second.stdout, "swap: 511 MiB total")
	assertContains(t, second.stdout, "Preflight passed with warnings: 7 checks, 1 warning(s), 0 error(s).")

	fstabData, err := os.ReadFile(fstab)
	if err != nil {
		t.Fatalf("read fstab: %v", err)
	}
	if count := strings.Count(string(fstabData), swapfile+" none swap"); count != 1 {
		t.Fatalf("fstab contains %d swap entries, want 1:\n%s", count, fstabData)
	}

	logData, err = os.ReadFile(h.commandLog)
	if err != nil {
		t.Fatalf("read command log after second run: %v", err)
	}
	for _, command := range []string{"fallocate size=", "mkswap path=", "swapon path="} {
		if count := strings.Count(string(logData), command); count != 1 {
			t.Fatalf("%q ran %d times, want 1:\n%s", command, count, logData)
		}
	}
}

func TestBootstrapSwapFallsBackToDd(t *testing.T) {
	h := newFakeHost(t)
	h.setRAM(542)
	h.setSwap(0)
	h.env["TES3MP_FAKE_FALLOCATE_FAIL"] = "1"
	swapfile := filepath.Join(h.installRoot, "swapfile")

	got := h.run(t)

	assertExit(t, got, 0)
	assertContains(t, got.stdout, "swap: activated "+swapfile+" (512 MiB)")
	logData, err := os.ReadFile(h.commandLog)
	if err != nil {
		t.Fatalf("read command log: %v", err)
	}
	assertContains(t, string(logData), "dd out="+swapfile)
}

func TestBootstrapCheckReportsSwapWithoutChanges(t *testing.T) {
	h := newFakeHost(t)
	h.setRAM(542)
	h.setSwap(0)
	swapfile := filepath.Join(h.installRoot, "swapfile")

	before := snapshotInstalledState(t, h)
	got := h.run(t, "--check")
	after := snapshotInstalledState(t, h)

	assertExit(t, got, 0)
	if got.stderr != "" {
		t.Fatalf("stderr = %q, want empty", got.stderr)
	}
	assertContains(t, got.stdout,
		"swap: 0 MiB is below the 480 MiB minimum; would create 512 MiB at "+swapfile)
	assertContains(t, got.stdout, "Preflight complete (--check). No changes were made.")
	if !reflect.DeepEqual(before, after) {
		t.Fatal("--check changed installed state")
	}
	if _, err := os.Stat(swapfile); !os.IsNotExist(err) {
		t.Fatalf("--check created %s", swapfile)
	}
	if logData, err := os.ReadFile(h.commandLog); err == nil {
		for _, command := range []string{"fallocate ", "mkswap ", "swapon "} {
			if strings.Contains(string(logData), command) {
				t.Fatalf("--check ran %q:\n%s", command, logData)
			}
		}
	}
}

func TestBootstrapSwapActivationFailureIsWarning(t *testing.T) {
	h := newFakeHost(t)
	h.setRAM(542)
	h.setSwap(0)
	h.env["TES3MP_FAKE_SWAPON_FAIL"] = "1"
	swapfile := filepath.Join(h.installRoot, "swapfile")

	got := h.run(t)

	assertExit(t, got, 0)
	assertContains(t, got.stdout, "swap: swapon failed for "+swapfile)
	assertContains(t, got.stdout, "Preflight passed with warnings: 7 checks, 2 warning(s), 0 error(s).")
	assertContains(t, got.stdout, "Bootstrap complete: TES3MP 0.8.1 is running.")
	if _, err := os.Stat(filepath.Join(h.installRoot, "etc/fstab")); !os.IsNotExist(err) {
		t.Fatal("failed swapon must not record the swap file in fstab")
	}
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
		"disk: 4096 MiB total is below",
		"Preflight failed with warnings: 7 checks, 1 warning(s), 6 error(s).",
	} {
		assertContains(t, got.stderr, want)
	}
	assertContains(t, got.stdout,
		"swap: 0 MiB is below the 480 MiB minimum; installer will create 512 MiB at "+
			filepath.Join(h.installRoot, "swapfile"))
}

func TestBootstrapAutoDetectsPublicAddress(t *testing.T) {
	h := newFakeHost(t)
	h.writeServerBinaries()
	h.env["TES3MP_EASY_PUBLIC_ADDRESS"] = ""
	h.env["TES3MP_FAKE_PUBLIC_IP"] = "203.0.113.10"

	got := h.run(t)

	assertExit(t, got, 0)
	assertContains(t, got.stdout, "panel TLS: acme for 203.0.113.10 (")
	assertInstalledFile(t, h, "etc/tes3mp-easy/panel.env",
		"TES3MP_PANEL_TLS_MODE=acme",
		"TES3MP_PANEL_PUBLIC_ADDRESS=203.0.113.10")
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
	assertContains(t, second.stdout, "runtime: TES3MP 0.8.1 already installed")
	assertContains(t, second.stdout, "mods: canonical current.txt already points to a valid archive")
}

func TestBootstrapRejectsServiceStartFailures(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*fakeHost)
		want  string
	}{
		{
			name: "game start command fails",
			setup: func(h *fakeHost) {
				h.env["TES3MP_FAKE_SERVICE_START_FAIL"] = "1"
			},
			want: "command failed: systemctl --no-ask-password start tes3mp.service",
		},
		{
			name: "game exits immediately",
			setup: func(h *fakeHost) {
				h.env["TES3MP_FAKE_INACTIVE_UNIT"] = "tes3mp.service"
			},
			want: "tes3mp.service is not active",
		},
		{
			name: "timer start command fails",
			setup: func(h *fakeHost) {
				h.env["TES3MP_FAKE_TIMER_FAIL"] = "1"
			},
			want: "command failed: systemctl --no-ask-password enable --now tes3mp-easy-backup.timer",
		},
		{
			name: "timer does not stay active",
			setup: func(h *fakeHost) {
				h.env["TES3MP_FAKE_INACTIVE_UNIT"] = "tes3mp-easy-backup.timer"
			},
			want: "tes3mp-easy-backup.timer is not active",
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
			if strings.Contains(tt.want, "tes3mp.service") &&
				strings.Contains(tt.want, "not active") {
				logData, err := os.ReadFile(h.commandLog)
				if err != nil {
					t.Fatalf("read command log: %v", err)
				}
				if strings.Contains(string(logData), "enable --now tes3mp-easy-backup.timer") {
					t.Fatalf("timer started after the game service failed:\n%s", logData)
				}
			}
		})
	}
}

func TestBootstrapRejectsRuntimeChecksumMismatch(t *testing.T) {
	h := newFakeHost(t)
	h.writeFile(h.versionFile, "TES3MP_VERSION=0.8.1\n"+
		"TES3MP_URL=https://example.invalid/tes3mp-server-0.8.1.tar.gz\n"+
		"TES3MP_SHA256="+strings.Repeat("0", 64)+"\n", 0o644)

	got := h.run(t)

	assertExit(t, got, 1)
	assertContains(t, got.stderr, "TES3MP archive checksum mismatch")
	runtimeDir := filepath.Join(h.installRoot, "opt/tes3mp-easy/tes3mp")
	if _, err := os.Lstat(runtimeDir); !os.IsNotExist(err) {
		t.Fatalf("checksum mismatch left %s (err=%v)", runtimeDir, err)
	}
	leftovers, err := filepath.Glob(filepath.Join(h.installRoot, "opt/tes3mp-easy/.tes3mp-*"))
	if err != nil {
		t.Fatalf("glob runtime temp files: %v", err)
	}
	if len(leftovers) != 0 {
		t.Fatalf("checksum mismatch left temp files: %v", leftovers)
	}
}

func TestBootstrapRejectsRuntimeDownloadAndOwnershipFailures(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want string
	}{
		{
			name: "download fails",
			env:  "TES3MP_FAKE_CURL_FAIL",
			want: "cannot download TES3MP 0.8.1 from https://example.invalid/tes3mp-server-0.8.1.tar.gz",
		},
		{
			name: "chown fails",
			env:  "TES3MP_FAKE_CHOWN_FAIL",
			want: "cannot make the TES3MP runtime root-owned",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newFakeHost(t)
			h.env[tt.env] = "1"

			got := h.run(t)

			assertExit(t, got, 1)
			assertContains(t, got.stderr, tt.want)
			runtimeDir := filepath.Join(h.installRoot, "opt/tes3mp-easy/tes3mp")
			if _, err := os.Lstat(runtimeDir); !os.IsNotExist(err) {
				t.Fatalf("failed install left %s (err=%v)", runtimeDir, err)
			}
			leftovers, err := filepath.Glob(filepath.Join(h.installRoot, "opt/tes3mp-easy/.tes3mp-*"))
			if err != nil {
				t.Fatalf("glob runtime temp files: %v", err)
			}
			if len(leftovers) != 0 {
				t.Fatalf("failed install left temp files: %v", leftovers)
			}
		})
	}
}

func TestBootstrapRejectsUnsafeConfigDestinations(t *testing.T) {
	tests := []struct {
		name string
		rel  string
		kind string
		want string
	}{
		{
			name: "config symlink",
			rel:  "configs/config.lua",
			kind: "symlink",
			want: "is a symlink; refusing to write through it",
		},
		{
			name: "banlist directory",
			rel:  "configs/banlist.json",
			kind: "directory",
			want: "is not a regular file",
		},
		{
			name: "required data symlink",
			rel:  "mods/plugins/requiredDataFiles.json",
			kind: "symlink",
			want: "is a symlink; refusing to write through it",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newFakeHost(t)
			destination := filepath.Join(h.dataRoot, tt.rel)
			if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
				t.Fatalf("create destination parent: %v", err)
			}

			target := ""
			switch tt.kind {
			case "symlink":
				target = filepath.Join(h.dir, "outside-"+filepath.Base(tt.rel))
				h.writeFile(target, "keep me\n", 0o600)
				if err := os.Symlink(target, destination); err != nil {
					t.Fatalf("create destination symlink: %v", err)
				}
			case "directory":
				if err := os.Mkdir(destination, 0o700); err != nil {
					t.Fatalf("create destination directory: %v", err)
				}
			default:
				t.Fatalf("unknown destination kind %q", tt.kind)
			}

			got := h.run(t)

			assertExit(t, got, 1)
			assertContains(t, got.stderr, destination+" "+tt.want)
			if tt.kind == "symlink" {
				link, err := os.Readlink(destination)
				if err != nil || link != target {
					t.Fatalf("destination symlink = %q (err=%v), want %q", link, err, target)
				}
				data, err := os.ReadFile(target)
				if err != nil || string(data) != "keep me\n" {
					t.Fatalf("symlink target changed: %q (err=%v)", data, err)
				}
			} else {
				info, err := os.Lstat(destination)
				if err != nil {
					t.Fatalf("lstat destination after failure: %v", err)
				}
				if !info.IsDir() {
					t.Fatal("destination directory was replaced")
				}
			}
		})
	}
}

func TestBootstrapRejectsUnexpectedRuntimeDirectory(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T, *fakeHost, string)
		want  string
	}{
		{
			name: "missing marker",
			setup: func(t *testing.T, h *fakeHost, dir string) {
				t.Helper()
			},
			want: "exists without the expected install marker",
		},
		{
			name: "marker mismatch",
			setup: func(t *testing.T, h *fakeHost, dir string) {
				t.Helper()
				h.writeFile(filepath.Join(dir, ".tes3mp-install-info"),
					"TES3MP_VERSION=0.8.0\nTES3MP_URL=https://example.invalid/old.tar.gz\n"+
						"TES3MP_SHA256="+strings.Repeat("1", 64)+"\n", 0o644)
			},
			want: "exists without the expected install marker",
		},
		{
			name: "incomplete runtime",
			setup: func(t *testing.T, h *fakeHost, dir string) {
				t.Helper()
				marker, err := os.ReadFile(h.versionFile)
				if err != nil {
					t.Fatalf("read version fixture: %v", err)
				}
				h.writeFile(filepath.Join(dir, ".tes3mp-install-info"), string(marker), 0o644)
			},
			want: "is incomplete; remove it before reinstalling",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newFakeHost(t)
			runtimeDir := filepath.Join(h.installRoot, "opt/tes3mp-easy/tes3mp")
			if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
				t.Fatalf("create runtime dir: %v", err)
			}
			tt.setup(t, h, runtimeDir)

			got := h.run(t)

			assertExit(t, got, 1)
			assertContains(t, got.stderr, runtimeDir+" "+tt.want)
			entries, err := os.ReadDir(runtimeDir)
			if err != nil {
				t.Fatalf("read runtime dir: %v", err)
			}
			if tt.name == "missing marker" && len(entries) != 0 {
				t.Fatalf("runtime dir changed before abort: %v", entries)
			}
		})
	}
}

func TestBootstrapRejectsUnexpectedRuntimeLayout(t *testing.T) {
	h := newFakeHost(t)
	bad := fixtures.TarGz(t, []fixtures.Entry{
		{Name: "TES3MP-server/README", Body: "not a TES3MP runtime\n", Mode: 0o644},
	})
	if err := os.WriteFile(h.runtimeArchive, bad, 0o644); err != nil {
		t.Fatalf("write bad runtime archive: %v", err)
	}
	h.writeRuntimeVersion(h.runtimeArchive)

	got := h.run(t)

	assertExit(t, got, 1)
	assertContains(t, got.stderr, "does not have the expected TES3MP-server layout")
	if _, err := os.Lstat(filepath.Join(h.installRoot, "opt/tes3mp-easy/tes3mp")); !os.IsNotExist(err) {
		t.Fatalf("bad runtime archive left an installed runtime (err=%v)", err)
	}
}

func TestBootstrapKeepsCanonicalModsCurrent(t *testing.T) {
	h := newFakeHost(t)
	first := h.run(t)
	assertExit(t, first, 0)

	modsDir := filepath.Join(h.dataRoot, "backups/mods")
	currentPath := filepath.Join(modsDir, "current.txt")
	current, err := archive.ReadCurrentFile(currentPath)
	if err != nil {
		t.Fatalf("read current.txt: %v", err)
	}
	suffixed := strings.TrimSuffix(current.Name, ".tar.gz") + "-01.tar.gz"
	if err := os.Rename(filepath.Join(modsDir, current.Name), filepath.Join(modsDir, suffixed)); err != nil {
		t.Fatalf("rename mods archive to a collision suffix: %v", err)
	}
	content := current.SHA256 + " " + suffixed + "\n"
	h.writeFile(currentPath, content, 0o640)

	second := h.run(t)
	assertExit(t, second, 0)
	if second.stderr != "" {
		t.Fatalf("second stderr = %q, want empty", second.stderr)
	}
	assertContains(t, second.stdout, "mods: canonical current.txt already points to a valid archive")
	data, err := os.ReadFile(currentPath)
	if err != nil {
		t.Fatalf("read current.txt after second run: %v", err)
	}
	if string(data) != content {
		t.Fatalf("canonical current.txt changed: got %q, want %q", data, content)
	}
	if _, err := os.Stat(filepath.Join(modsDir, suffixed)); err != nil {
		t.Fatalf("collision-suffixed archive disappeared: %v", err)
	}
}

func TestBootstrapRejectsNonCanonicalModsCurrent(t *testing.T) {
	const sha = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "reversed fields",
			content: "mods-init-2026-01-01_00-00-00.tar.gz deadbeef\n",
		},
		{
			name:    "single digit suffix",
			content: sha + " mods-init-2026-01-01_00-00-00-1.tar.gz\n",
		},
		{
			name:    "leading zero long suffix",
			content: sha + " mods-init-2026-01-01_00-00-00-010.tar.gz\n",
		},
		{
			name:    "impossible date",
			content: sha + " mods-init-2026-02-30_00-00-00.tar.gz\n",
		},
		{
			name:    "double space",
			content: sha + "  mods-init-2026-01-01_00-00-00.tar.gz\n",
		},
		{
			name:    "missing newline",
			content: sha + " mods-init-2026-01-01_00-00-00.tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newFakeHost(t)
			modsDir := filepath.Join(h.dataRoot, "backups/mods")
			if err := os.MkdirAll(modsDir, 0o750); err != nil {
				t.Fatalf("create mods backup dir: %v", err)
			}
			bad := filepath.Join(modsDir, "current.txt")
			h.writeFile(bad, tt.content, 0o640)

			got := h.run(t)

			assertExit(t, got, 1)
			assertContains(t, got.stderr, bad+" is not canonical; refusing to overwrite it")
			data, err := os.ReadFile(bad)
			if err != nil {
				t.Fatalf("read current.txt after failure: %v", err)
			}
			if string(data) != tt.content {
				t.Fatalf("failure rewrote current.txt: got %q, want %q", data, tt.content)
			}
		})
	}
}

func TestBootstrapRejectsSymlinkedModsCurrent(t *testing.T) {
	h := newFakeHost(t)
	modsDir := filepath.Join(h.dataRoot, "backups/mods")
	if err := os.MkdirAll(modsDir, 0o750); err != nil {
		t.Fatalf("create mods backup dir: %v", err)
	}
	target := filepath.Join(h.dir, "outside-current.txt")
	h.writeFile(target, strings.Repeat("a", 64)+" mods-init-2026-01-01_00-00-00.tar.gz\n", 0o640)
	currentPath := filepath.Join(modsDir, "current.txt")
	if err := os.Symlink(target, currentPath); err != nil {
		t.Fatalf("create current.txt symlink: %v", err)
	}

	got := h.run(t)

	assertExit(t, got, 1)
	assertContains(t, got.stderr, currentPath+" is not canonical; refusing to overwrite it")
	info, err := os.Lstat(currentPath)
	if err != nil {
		t.Fatalf("lstat current.txt after failure: %v", err)
	}
	if info.Mode()&fs.ModeSymlink == 0 {
		t.Fatal("current.txt symlink was replaced by a regular file")
	}
	if link, err := os.Readlink(currentPath); err != nil || link != target {
		t.Fatalf("current.txt symlink = %q (err=%v), want %q", link, err, target)
	}
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
		if info.Mode()&fs.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			snapshot[rel] = fileSnapshot{mode: info.Mode(), content: "->" + target}
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
