#!/usr/bin/env bash
#
# install-smoke.sh — verify that install.sh downloads, checks and runs a
# server release without network access or root privileges.
#
set -Eeuo pipefail
export LC_ALL=C

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
work=$(mktemp -d "${TMPDIR:-/tmp}/tes3mp-easy-install-test.XXXXXX")
trap 'rm -rf -- "$work"' EXIT

release=tes3mp-easy-9.9.9-linux-amd64
fixture="$work/fixture"
mkdir -p "$work/bin" "$fixture" "$work/payload/$release"

cat >"$work/payload/$release/bootstrap.sh" <<'BOOTSTRAP'
#!/usr/bin/env bash
printf '%s\n' "$@" >"$TES3MP_EASY_TEST_ARGS"
BOOTSTRAP
chmod +x "$work/payload/$release/bootstrap.sh"

tar -czf "$fixture/$release.tar.gz" -C "$work/payload" "$release"
(
    cd "$fixture"
    sha256sum "$release.tar.gz" >"$release.tar.gz.sha256"
)

cat >"$fixture/release.json" <<JSON
{
  "assets": [
    {
      "browser_download_url": "https://example.test/$release.tar.gz"
    }
  ]
}
JSON

cat >"$work/bin/curl" <<'CURL'
#!/usr/bin/env bash
set -Eeuo pipefail

out=""
url=""
while (($# > 0)); do
    case "$1" in
        -o)
            out=$2
            shift 2
            ;;
        --retry | --connect-timeout)
            shift 2
            ;;
        -*)
            shift
            ;;
        *)
            url=$1
            shift
            ;;
    esac
done

case "$url" in
    https://api.test/latest | https://api.github.com/repos/Saagilat/tes3mp-easy/releases/tags/v9.9.9)
        cat "$TES3MP_EASY_TEST_FIXTURE/release.json"
        ;;
    https://example.test/*)
        cp "$TES3MP_EASY_TEST_FIXTURE/${url##*/}" "$out"
        ;;
    *)
        printf 'curl: unexpected URL %s\n' "$url" >&2
        exit 22
        ;;
esac
CURL
chmod +x "$work/bin/curl"

cat >"$work/bin/sudo" <<'SUDO'
#!/usr/bin/env bash
exec "$@"
SUDO
chmod +x "$work/bin/sudo"

PATH="$work/bin:$PATH" \
    TES3MP_EASY_TEST_FIXTURE="$fixture" \
    TES3MP_EASY_RELEASES_API="https://api.test/latest" \
    TES3MP_EASY_TEST_ARGS="$work/args" \
    bash "$repo_root/install.sh" --check --root /tmp/tes3mp-easy-test

expected=$'--check\n--root\n/tmp/tes3mp-easy-test'
check_args() {
    local file=$1
    local actual
    actual=$(cat "$file")
    if [[ $actual != "$expected" ]]; then
        printf 'install-smoke: unexpected bootstrap arguments:\n%s\n' "$actual" >&2
        exit 1
    fi
}
check_args "$work/args"

PATH="$work/bin:$PATH" \
    TES3MP_EASY_TEST_FIXTURE="$fixture" \
    TES3MP_EASY_VERSION=9.9.9 \
    TES3MP_EASY_TEST_ARGS="$work/args-pinned" \
    bash "$repo_root/install.sh" --root /tmp/tes3mp-easy-test

expected=$'--root\n/tmp/tes3mp-easy-test'
check_args "$work/args-pinned"

printf 'install-smoke: PASS\n'
