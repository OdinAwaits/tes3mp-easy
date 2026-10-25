#!/usr/bin/env bash
#
# install.sh — download a TES3MP Easy server release and run bootstrap.sh.
#
# Usage:
#   bash <(curl -Ls https://raw.githubusercontent.com/Saagilat/tes3mp-easy/master/install.sh)
#
# TES3MP_EASY_VERSION pins a release (for example 0.1.4). Arguments are passed
# to bootstrap.sh, so --check and --root work as usual.
set -Eeuo pipefail
export LC_ALL=C

readonly REPO="Saagilat/tes3mp-easy"
readonly LATEST_API="https://api.github.com/repos/${REPO}/releases/latest"

fail() {
    printf 'install: %s\n' "$*" >&2
    exit 1
}

for tool in curl tar sha256sum mktemp; do
    command -v "$tool" >/dev/null 2>&1 || fail "$tool is required"
done

release_api=${TES3MP_EASY_RELEASES_API:-}
if [[ -z $release_api ]]; then
    if [[ -n ${TES3MP_EASY_VERSION:-} ]]; then
        version=${TES3MP_EASY_VERSION#v}
        release_api="https://api.github.com/repos/${REPO}/releases/tags/v${version}"
    else
        release_api=$LATEST_API
    fi
fi

printf 'TES3MP Easy: looking up the server release...\n'
if ! release_json=$(curl -fsSL --retry 2 --connect-timeout 10 "$release_api"); then
    fail "cannot read release metadata from $release_api"
fi

asset_url=$(printf '%s\n' "$release_json" |
    grep -oE 'https://[^"]+/tes3mp-easy-[0-9][^"]*-linux-amd64\.tar\.gz"' |
    head -n 1 || true)
asset_url=${asset_url%\"}
if [[ -z $asset_url ]]; then
    fail "the release has no tes3mp-easy-<version>-linux-amd64.tar.gz asset; set TES3MP_EASY_VERSION to a server release"
fi

archive_name=${asset_url##*/}
release_name=${archive_name%.tar.gz}
if [[ -z $archive_name || $release_name == "$archive_name" ]]; then
    fail "unexpected release asset name: $archive_name"
fi

tmp=$(mktemp -d "${TMPDIR:-/var/tmp}/tes3mp-easy-install.XXXXXX")
trap 'rm -rf -- "$tmp"' EXIT
archive="$tmp/$archive_name"

printf 'TES3MP Easy: downloading %s...\n' "$archive_name"
curl -fsSL --retry 2 --connect-timeout 10 "$asset_url" -o "$archive" ||
    fail "cannot download $asset_url"
curl -fsSL --retry 2 --connect-timeout 10 "$asset_url.sha256" \
    -o "$archive.sha256" ||
    fail "cannot download $asset_url.sha256"

(
    cd "$tmp"
    sha256sum -c "$archive_name.sha256"
) || fail "release checksum verification failed"

tar -xzf "$archive" -C "$tmp" ||
    fail "cannot extract $archive_name"
bootstrap="$tmp/$release_name/bootstrap.sh"
if [[ ! -x $bootstrap ]]; then
    fail "the release archive has no executable bootstrap.sh"
fi

SUDO=()
if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
    command -v sudo >/dev/null 2>&1 ||
        fail "run as root or install sudo"
    SUDO=(sudo)
fi

printf 'TES3MP Easy: starting the installer...\n\n'
"${SUDO[@]}" "$bootstrap" "$@"
