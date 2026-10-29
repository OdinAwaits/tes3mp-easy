#!/usr/bin/env bash
#
# This script installs the latest release of TES3MP Easy client for
# the current user by:
#   - finding the newest client release on GitHub
#   - verifying the archive checksum
#   - unpacking the binary into ~/.local/bin
#   - writing the desktop entry into ~/.local/share/applications.
# !!No root required.
#

readonly REPO="Saagilat/tes3mp-easy"
# Releases are split into server-v<version> and client-v<version> channels and
# the <channel>-latest aliases carry archives without a version in the name, so
# the newest GitHub release is not necessarily a client release: scan the list
# and take the newest versioned client archive.
readonly RELEASES_API="https://api.github.com/repos/${REPO}/releases?per_page=30"
readonly TAGS_API="https://api.github.com/repos/${REPO}/releases/tags"
readonly CLIENT_ASSET='https://[^"]+/tes3mp-easy-client-[0-9][^"]*-linux-amd64\.tar\.gz"'
readonly BINARY=tes3mp-easy-client
readonly DESKTOP=tes3mp-easy.desktop

fail() {
    printf 'client-install: %s\n' "$*" >&2
    exit 1
}

if [[ ${EUID:-$(id -u)} -eq 0 ]]; then
    fail "run as your own user, not root or sudo"
fi

for tool in curl tar sha256sum mktemp grep sed sort install; do
    command -v "$tool" >/dev/null 2>&1 || fail "$tool is required"
done

fetch_json() {
    curl -fsSL --retry 2 --connect-timeout 10 "$1"
}

# The client pattern cannot match the server archive (tes3mp-easy-<v>-…) or the
# versionless client-latest alias (a digit must follow "tes3mp-easy-client-").
client_asset() {
    local urls
    urls=$(printf '%s\n' "$1" | grep -oE "$CLIENT_ASSET" | sed 's/"$//' || true)
    [[ -n $urls ]] || return 0
    # The release list order is not guaranteed, so pick the highest version.
    printf '%s\n' "$urls" |
        sed -E 's#.*/tes3mp-easy-client-(.*)-linux-amd64\.tar\.gz#\1 &#' |
        sort -V -k1,1 |
        tail -n 1 |
        cut -d' ' -f2-
}

asset_url=""
if [[ -n ${TES3MP_EASY_RELEASES_API:-} ]]; then
    release_json=$(fetch_json "$TES3MP_EASY_RELEASES_API") ||
        fail "cannot read release metadata from $TES3MP_EASY_RELEASES_API"
    asset_url=$(client_asset "$release_json")
elif [[ -n ${TES3MP_EASY_CLIENT_VERSION:-} ]]; then
    version=${TES3MP_EASY_CLIENT_VERSION#v}
    if ! [[ $version =~ ^[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z.-]+)?$ ]]; then
        fail "TES3MP_EASY_CLIENT_VERSION must look like 0.4.0, got $TES3MP_EASY_CLIENT_VERSION"
    fi
    printf 'TES3MP Easy: looking up the client release %s...\n' "$version"
    release_json=$(fetch_json "$TAGS_API/client-v${version}") ||
        fail "cannot read release metadata for client-v${version}"
    asset_url=$(client_asset "$release_json")
    [[ -n $asset_url ]] ||
        fail "client release $version has no tes3mp-easy-client-${version}-linux-amd64.tar.gz asset"
else
    printf 'TES3MP Easy: looking up the latest client release...\n'
    release_json=$(fetch_json "$RELEASES_API") ||
        fail "cannot read release metadata from $RELEASES_API"
    asset_url=$(client_asset "$release_json")
    [[ -n $asset_url ]] || fail "no client release found in $RELEASES_API"
fi

archive_name=${asset_url##*/}
release_name=${archive_name%.tar.gz}
if [[ -z $archive_name || $release_name == "$archive_name" ]]; then
    fail "unexpected release asset name: $archive_name"
fi

bin_dir="$HOME/.local/bin"
# ~/.local/applications is the default of XDG_DATA_HOME/share/applications.
applications_dir="${XDG_DATA_HOME:-$HOME/.local}/share/applications"

tmp=$(mktemp -d "${TMPDIR:-/tmp}/tes3mp-easy-client-install.XXXXXX")
staged="$bin_dir/.${BINARY}.new.$$"
trap 'rm -rf -- "$tmp"; if [[ -n ${staged:-} ]]; then rm -f -- "$staged"; fi' EXIT
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

tar -xzf "$archive" -C "$tmp" || fail "cannot extract $archive_name"

# The archive holds the binary at its root today; accept a versioned top-level
# directory as well so the script keeps working if the layout ever changes.
binary=""
for candidate in "$tmp/$BINARY" "$tmp"/*/"$BINARY"; do
    if [[ -f $candidate ]]; then
        binary=$candidate
        break
    fi
done
[[ -n $binary ]] || fail "$archive_name has no $BINARY binary"

mkdir -p "$bin_dir" "$applications_dir"
install -m 0755 "$binary" "$staged"
mv -f -- "$staged" "$bin_dir/$BINARY"   # atomic replace, no ETXTBSY
staged=""

cat >"$tmp/$DESKTOP" <<'DESKTOP'
[Desktop Entry]
Type=Application
Name=TES3MP Easy
Exec=tes3mp-easy-client %U
Icon=openmw
Terminal=false
Categories=Game;Network;
StartupNotify=true
StartupWMClass=io.github.saagilat.tes3mp-easy
DESKTOP
install -m 0644 "$tmp/$DESKTOP" "$applications_dir/$DESKTOP"

if command -v update-desktop-database >/dev/null 2>&1; then
    update-desktop-database "$applications_dir" >/dev/null 2>&1 || true
fi

printf '\nTES3MP Easy: client installed\n'
printf '  binary: %s\n' "$bin_dir/$BINARY"
printf '  menu:   %s\n' "$applications_dir/$DESKTOP"
if installed=$("$bin_dir/$BINARY" --version 2>/dev/null); then
    printf '  version: %s\n' "$installed"
fi
case ":$PATH:" in
    *":$bin_dir:"*) ;;
    *) printf '  note: add %s to PATH so the menu entry can start the client\n' "$bin_dir" ;;
esac
