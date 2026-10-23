package panel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeSudo stands in for the narrow sudoers rule: it validates the exact
// production argv, logs every call, and synthesizes ops JSON envelopes. It never
// executes a real process.
const fakeSudo = `#!/bin/sh
set -eu

if [ -n "${FAKE_OPS_LOG:-}" ]; then
	printf '%s\n' "$@" >> "$FAKE_OPS_LOG"
	printf '%s\n' '--' >> "$FAKE_OPS_LOG"
fi

if [ "$#" -lt 5 ] || [ "$1" != "-n" ] || [ "$2" != "-u" ] ||
	[ "$3" != "tes3mp" ] ||
	[ "$4" != "/opt/tes3mp-easy/bin/tes3mp-easy-ops" ]; then
	printf 'unexpected sudo call: %s\n' "$*" >&2
	exit 99
fi
shift 4

if [ "$#" -lt 4 ] || [ "$1" != "--root" ] || [ "$3" != "--json" ] ||
	[ -z "$2" ] || [ -z "$4" ]; then
	printf 'unexpected ops call: %s\n' "$*" >&2
	exit 99
fi
shift 3
command=$1
shift

mode=ok
if [ -n "${FAKE_OPS_MODE_FILE:-}" ] && [ -f "$FAKE_OPS_MODE_FILE" ]; then
	mode=$(cat "$FAKE_OPS_MODE_FILE")
fi

case "$command" in
status)
	if [ "$mode" = status-fail ]; then
		printf '%s\n' 'ops: systemd is not running' >&2
		printf '{"ok":false,"error":{"code":"service_unavailable","message":"systemd is not running"}}\n'
		exit 6
	fi
	unit=tes3mp.service
	while [ "$#" -gt 0 ]; do
		case "$1" in
		--unit) unit=$2; shift 2 ;;
		*) printf 'unexpected status arg: %s\n' "$1" >&2; exit 99 ;;
		esac
	done
	started='"2026-10-23T10:00:00Z"'
	if [ "$unit" = "tes3mp-easy-backup.timer" ]; then
		started=null
	fi
	printf '{"ok":true,"data":{"unit":"%s","state":"running","health":"healthy","started_at":%s}}\n' \
		"$unit" "$started"
	;;
logs)
	while [ "$#" -gt 0 ]; do
		case "$1" in
		--tail) shift 2 ;;
		*) printf 'unexpected logs arg: %s\n' "$1" >&2; exit 99 ;;
		esac
	done
	printf '%s\n' '{"ok":true,"data":{"lines":["first line","second line"]}}'
	;;
backups)
	if [ "${1:-}" != "list" ]; then
		printf 'unexpected backups subcommand: %s\n' "${1:-}" >&2
		exit 99
	fi
	shift
	backup_type=
	while [ "$#" -gt 0 ]; do
		case "$1" in
		--type) backup_type=$2; shift 2 ;;
		--limit|--before) shift 2 ;;
		*) printf 'unexpected backups arg: %s\n' "$1" >&2; exit 99 ;;
		esac
	done
	if [ -z "$backup_type" ]; then
		printf 'backups list without --type\n' >&2
		exit 99
	fi
	printf '{"ok":true,"data":{"type":"%s","current":null,"items":[]}}\n' \
		"$backup_type"
	;;
doctor)
	if [ "$#" -ne 0 ]; then
		printf 'unexpected doctor arg: %s\n' "$1" >&2
		exit 99
	fi
	printf '%s\n' '{"ok":true,"data":{"status":"ok","checks":[{"name":"disk","status":"ok","message":"synthetic"}]}}'
	;;
start|stop|restart)
	if [ "$#" -ne 0 ]; then
		printf 'unexpected action arg: %s\n' "$1" >&2
		exit 99
	fi
	case "$mode" in
	fail)
		printf '%s\n' 'ops: systemd is not running' >&2
		printf '{"ok":false,"error":{"code":"service_unavailable","message":"systemd is not running"}}\n'
		exit 6
		;;
	locked)
		printf '%s\n' 'ops: lock is held' >&2
		printf '{"ok":false,"error":{"code":"locked","message":"another operation is running"}}\n'
		exit 4
		;;
	ok) ;;
	*)
		printf 'unknown fake mode: %s\n' "$mode" >&2
		exit 99
		;;
	esac
	printf '{"ok":true,"data":{"unit":"tes3mp.service","action":"%s"}}\n' "$command"
	;;
*)
	printf 'unexpected ops command: %s\n' "$command" >&2
	exit 99
	;;
esac
`

func readOpsCalls(t *testing.T, path string) [][]string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fake ops log: %v", err)
	}
	var calls [][]string
	var current []string
	for _, line := range strings.Split(string(data), "\n") {
		switch {
		case line == "--":
			calls = append(calls, current)
			current = nil
		case line != "":
			current = append(current, line)
		}
	}
	if len(current) != 0 {
		t.Fatalf("fake ops log has an unterminated call: %v", current)
	}
	return calls
}

func requireOpsCall(t *testing.T, calls [][]string, want ...string) {
	t.Helper()

	for _, call := range calls {
		if len(call) != len(want) {
			continue
		}
		matched := true
		for i := range want {
			if call[i] != want[i] {
				matched = false
				break
			}
		}
		if matched {
			return
		}
	}
	t.Fatalf("no ops call %v in %v", want, calls)
}

func writeFakeMode(t *testing.T, path, mode string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(mode+"\n"), 0o600); err != nil {
		t.Fatalf("write fake mode: %v", err)
	}
}

func fakeBinPath(name string) string {
	return filepath.Join(testFakeBin, name)
}
