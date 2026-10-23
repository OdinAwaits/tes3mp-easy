package ops

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fakeSystemctl = `#!/bin/sh
set -eu

if [ -n "${FAKE_SYSTEMD_LOG:-}" ]; then
	printf '%s\n' "$@" >> "$FAKE_SYSTEMD_LOG"
	printf '%s\n' '--' >> "$FAKE_SYSTEMD_LOG"
fi

mode=${FAKE_SYSTEMD_MODE:-ok}
state=${FAKE_SYSTEMD_STATE:-active}
substate=${FAKE_SYSTEMD_SUBSTATE:-running}
loadstate=${FAKE_SYSTEMD_LOADSTATE:-loaded}
active_enter=${FAKE_SYSTEMD_ACTIVE_ENTER:-Thu 2026-10-22 12:00:00 UTC}
last_trigger=${FAKE_SYSTEMD_LAST_TRIGGER:-Thu 2026-10-22 11:55:00 UTC}
unitfile=${FAKE_SYSTEMD_UNIT_FILE_STATE:-enabled}

case "$1" in
--version)
	if [ "$mode" = "unavailable" ]; then
		printf '%s\n' 'Failed to connect to bus: No such file or directory' >&2
		exit 1
	fi
	printf '%s\n' 'systemd 255 (255.4-1ubuntu8)'
	printf '%s\n' '+PAM +AUDIT'
	;;
show)
	if [ "$mode" = "unavailable" ]; then
		printf '%s\n' 'Failed to connect to bus: No such file or directory' >&2
		exit 1
	fi
	case "$2" in
	tes3mp.service)
		memory_high=402653184
		memory_max=469762048
		;;
	tes3mp-panel.service|tes3mp-easy-backup.service)
		memory_high=50331648
		memory_max=67108864
		;;
	*)
		memory_high=0
		memory_max=0
		;;
	esac
	printf 'ActiveState=%s\n' "$state"
	printf 'SubState=%s\n' "$substate"
	printf 'ActiveEnterTimestamp=%s\n' "$active_enter"
	printf 'LastTriggerUSec=%s\n' "$last_trigger"
	printf 'LoadState=%s\n' "$loadstate"
	printf 'UnitFileState=%s\n' "$unitfile"
	printf 'MemoryHigh=%s\n' "$memory_high"
	printf 'MemoryMax=%s\n' "$memory_max"
	;;
is-active)
	case "$mode" in
	unavailable)
		printf '%s\n' 'Failed to connect to bus: No such file or directory' >&2
		exit 1
		;;
	is-active-unavailable)
		printf '%s\n' 'Failed to get properties: Connection timed out' >&2
		exit 1
		;;
	esac
	printf '%s\n' "$state"
	case "$state" in
	active) exit 0 ;;
	inactive|failed|activating|deactivating) exit 3 ;;
	*) exit 4 ;;
	esac
	;;
*)
	printf 'unexpected systemctl call: %s\n' "$*" >&2
	exit 99
	;;
esac
`

const fakeSudo = `#!/bin/sh
set -eu

if [ -n "${FAKE_SUDO_LOG:-}" ]; then
	printf '%s\n' "$@" >> "$FAKE_SUDO_LOG"
	printf '%s\n' '--' >> "$FAKE_SUDO_LOG"
fi

if [ "${FAKE_SUDO_MODE:-ok}" = "fail" ]; then
	printf '%s\n' 'rootctl failed' >&2
	exit 1
fi
`

const fakeUname = `#!/bin/sh
set -eu

if [ "$#" -ne 1 ] || [ "$1" != "-m" ]; then
	printf 'unexpected uname call: %s\n' "$*" >&2
	exit 99
fi

printf '%s\n' "${FAKE_UNAME_M:-x86_64}"
`

const fakeLdconfig = `#!/bin/sh
set -eu

if [ "$#" -ne 1 ] || [ "$1" != "-p" ]; then
	printf 'unexpected ldconfig call: %s\n' "$*" >&2
	exit 99
fi

printf '%s\n' 'libluajit-5.1.so.2 (libc6,x86-64) => /lib/libluajit-5.1.so.2'
`

const fakeLdd = `#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
	printf 'unexpected ldd call: %s\n' "$*" >&2
	exit 99
fi

printf '%s\n' 'linux-vdso.so.1'
printf '%s\n' 'libluajit-5.1.so.2 => /lib/libluajit-5.1.so.2'
`

const fakeJournalctl = `#!/bin/sh
set -eu

if [ -n "${FAKE_JOURNALD_LOG:-}" ]; then
	printf '%s\n' "$@" >> "$FAKE_JOURNALD_LOG"
	printf '%s\n' '--' >> "$FAKE_JOURNALD_LOG"
fi

mode=${FAKE_JOURNALD_MODE:-ok}
if [ "$mode" = "unavailable" ]; then
	printf '%s\n' 'Failed to open journal: No such file or directory' >&2
	exit 1
fi

output=${FAKE_JOURNALD_OUTPUT-}
printf '%s' "$output"
`

func writeFakeBin(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "systemctl"),
		[]byte(fakeSystemctl), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "journalctl"),
		[]byte(fakeJournalctl), 0o755); err != nil {
		return err
	}
	for name, body := range map[string]string{
		"ldconfig": fakeLdconfig,
		"ldd":      fakeLdd,
		"sudo":     fakeSudo,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(dir, "uname"),
		[]byte(fakeUname), 0o755)
}

func readFakeCalls(t *testing.T, path string) [][]string {
	t.Helper()

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatalf("read fake call log %s: %v", path, err)
	}

	var calls [][]string
	var current []string
	for _, line := range strings.Split(string(data), "\n") {
		switch line {
		case "":
			continue
		case "--":
			calls = append(calls, current)
			current = nil
		default:
			current = append(current, line)
		}
	}
	if len(current) > 0 {
		calls = append(calls, current)
	}
	return calls
}
