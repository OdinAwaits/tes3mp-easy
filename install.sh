#!/usr/bin/env bash
#
# install.sh — download a TES3MP Easy server release and run bootstrap.sh.
#
# Usage:
#   bash <(curl -Ls https://raw.githubusercontent.com/Saagilat/tes3mp-easy/master/install.sh)
#
# TES3MP_EASY_VERSION pins a server release (for example 0.2.2). Arguments are
# passed to bootstrap.sh, so --check and --root work as usual.
set -Eeuo pipefail
export LC_ALL=C

readonly REPO="Saagilat/tes3mp-easy"
# Releases are split into server-v<version> and client-v<version> channels, so
# the newest GitHub release is not necessarily a server release: scan the list
# and take the newest server archive instead of trusting /releases/latest.
readonly RELEASES_API="https://api.github.com/repos/${REPO}/releases?per_page=30"
readonly TAGS_API="https://api.github.com/repos/${REPO}/releases/tags"
readonly SERVER_ASSET='https://[^"]+/tes3mp-easy-[0-9][^"]*-linux-amd64\.tar\.gz"'

fail() {
    printf 'install: %s\n' "$*" >&2
    exit 1
}

for tool in curl tar sha256sum mktemp grep; do
    command -v "$tool" >/dev/null 2>&1 || fail "$tool is required"
done

fetch_json() {
    curl -fsSL --retry 2 --connect-timeout 10 "$1"
}

server_asset() {
    printf '%s\n' "$1" | grep -oE "$SERVER_ASSET" | head -n 1 || true
}

asset_url=""
if [[ -n ${TES3MP_EASY_RELEASES_API:-} ]]; then
    release_json=$(fetch_json "$TES3MP_EASY_RELEASES_API") ||
        fail "cannot read release metadata from $TES3MP_EASY_RELEASES_API"
    asset_url=$(server_asset "$release_json")
elif [[ -n ${TES3MP_EASY_VERSION:-} ]]; then
    version=${TES3MP_EASY_VERSION#v}
    if ! [[ $version =~ ^[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z.-]+)?$ ]]; then
        fail "TES3MP_EASY_VERSION must look like 0.3.0, got $TES3MP_EASY_VERSION"
    fi
    printf 'TES3MP Easy: looking up the server release %s...\n' "$version"
    # Releases made before the channel split used the plain v<version> tag.
    for tag in "server-v${version}" "v${version}"; do
        release_json=$(fetch_json "$TAGS_API/${tag}" || true)
        asset_url=$(server_asset "$release_json")
        [[ -n $asset_url ]] && break
    done
    if [[ -z $asset_url ]]; then
        fail "server release $version has no tes3mp-easy-${version}-linux-amd64.tar.gz asset"
    fi
else
    printf 'TES3MP Easy: looking up the latest server release...\n'
    release_json=$(fetch_json "$RELEASES_API") ||
        fail "cannot read release metadata from $RELEASES_API"
    asset_url=$(server_asset "$release_json")
    if [[ -z $asset_url ]]; then
        fail "no server release found in $RELEASES_API; set TES3MP_EASY_VERSION to a server release"
    fi
fi
asset_url=${asset_url%\"}

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
