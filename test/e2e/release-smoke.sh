#!/usr/bin/env bash
#
# release-smoke.sh — run the acceptance smoke test of a server release on a
# disposable Debian 12 / Ubuntu 24.04 VM. It expects root and a clean host.
#
# Usage: sudo test/e2e/release-smoke.sh <extracted-release-dir>
#
set -Eeuo pipefail
export LC_ALL=C

readonly ROOT=/tes3mp-easy
readonly OPS=/opt/tes3mp-easy/bin/tes3mp-easy-ops
readonly PANEL=/opt/tes3mp-easy/bin/tes3mp-easy-panel

release_dir=${1:-}
if [[ -z $release_dir || ! -d $release_dir ]]; then
    printf 'Usage: sudo %s <extracted-release-dir>\n' "$0" >&2
    exit 2
fi
release_dir=$(cd -- "$release_dir" && pwd)

if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
    printf 'release-smoke: run as root (sudo)\n' >&2
    exit 1
fi
if [[ ! -d /run/systemd/system ]]; then
    printf 'release-smoke: systemd is not PID 1\n' >&2
    exit 1
fi
if [[ ! -r /etc/os-release ]]; then
    printf 'release-smoke: /etc/os-release is missing\n' >&2
    exit 1
fi
# shellcheck disable=SC1091
source /etc/os-release
case "${ID:-}:${VERSION_ID:-}" in
    debian:12 | debian:13 | ubuntu:24.04 | ubuntu:24.10 | ubuntu:25.04) ;;
    *)
        printf 'release-smoke: unsupported OS %s %s\n' "${ID:-?}" "${VERSION_ID:-?}" >&2
        exit 1
        ;;
esac
for path in \
    "$release_dir/bootstrap.sh" \
    "$release_dir/bin/tes3mp-easy-ops" \
    "$release_dir/bin/tes3mp-easy-panel"; do
    if [[ ! -f $path ]]; then
        printf 'release-smoke: release is incomplete: %s\n' "$path" >&2
        exit 1
    fi
done

fail() {
    printf 'release-smoke: FAIL: %s\n' "$*" >&2
    exit 1
}
trap 'fail "command failed at line $LINENO"' ERR

printf '== preflight ==\n'
"$release_dir/bootstrap.sh" --check

printf '\n== install ==\n'
"$release_dir/bootstrap.sh" --root "$ROOT"

printf '\n== binaries and services ==\n'
for path in "$OPS" "$PANEL"; do
    [[ -x $path ]] || fail "$path is not executable"
    [[ $(stat -c '%U:%G %a' "$path") == "root:root 755" ]] ||
        fail "$path has unexpected ownership/mode"
done
systemctl is-active --quiet tes3mp.service || fail "tes3mp.service is not active"
systemctl is-active --quiet tes3mp-easy-backup.timer ||
    fail "backup timer is not active"
systemctl is-active --quiet tes3mp-panel.service ||
    fail "tes3mp-panel.service is not active"
[[ -s $ROOT/panel-data/initial-password.txt ]] ||
    fail "initial-password.txt was not created"

printf '\n== doctor ==\n'
sudo -u tes3mp "$OPS" --root "$ROOT" --json doctor

printf '\n== panel health ==\n'
ready=0
for _ in $(seq 1 30); do
    if curl -kfsS --max-time 2 https://127.0.0.1/healthz >/dev/null; then
        ready=1
        break
    fi
    sleep 1
done
((ready)) || fail "panel /healthz did not become ready"
printf 'healthz: ok\n'

printf '\n== backup ==\n'
sudo -u tes3mp "$OPS" --root "$ROOT" --json \
    backups create --type state --manual >/dev/null
sudo -u tes3mp "$OPS" --root "$ROOT" --json \
    backups list --type state --limit 1 | grep -q '"ok":true' ||
    fail "backups list did not return ok"
if ! compgen -G "$ROOT/backups/state/state-*.tar.gz" >/dev/null; then
    fail "no state archive was created"
fi

printf '\n== timer ==\n'
systemctl list-timers tes3mp-easy-backup.timer --no-pager

printf '\nrelease-smoke: PASS (%s %s)\n' "${ID}" "${VERSION_ID}"
