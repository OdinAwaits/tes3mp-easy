#!/usr/bin/env bash
#
# bootstrap.sh — TES3MP Easy v2 bootstrap.
#
# The script validates the host, creates the privilege model and data layout,
# installs systemd units and the verified TES3MP runtime, installs the ops/panel
# binaries when they are shipped next to this script, and starts the services.
#
# Usage: sudo ./bootstrap.sh [--check] [--root PATH]
#
set -Eeuo pipefail
export LC_ALL=C

readonly DEFAULT_ROOT="/tes3mp-easy"

INSTALL_ROOT="${TES3MP_EASY_INSTALL_ROOT:-}"
if [[ -n $INSTALL_ROOT ]]; then
    while [[ $INSTALL_ROOT == */ && $INSTALL_ROOT != "/" ]]; do
        INSTALL_ROOT=${INSTALL_ROOT%/}
    done
    if [[ $INSTALL_ROOT == "/" ]]; then
        INSTALL_ROOT=""
    elif [[ $INSTALL_ROOT != /* ]]; then
        printf 'bootstrap: TES3MP_EASY_INSTALL_ROOT must be absolute\n' >&2
        exit 2
    fi
fi

SCRIPT_PATH=${BASH_SOURCE[0]}
if [[ $SCRIPT_PATH == */* ]]; then
    SCRIPT_DIR=${SCRIPT_PATH%/*}
else
    SCRIPT_DIR=.
fi
SCRIPT_DIR=$(cd -- "$SCRIPT_DIR" >/dev/null 2>&1 && pwd)

# These source paths are environment-overridable so the installer can be tested
# without touching the host. Production runs use the files shipped in the
# release tree.
ROOTCTL_SOURCE="${TES3MP_EASY_ROOTCTL_SOURCE:-$SCRIPT_DIR/rootctl/tes3mp-easy-rootctl}"
SUDOERS_SOURCE="${TES3MP_EASY_SUDOERS_SOURCE:-$SCRIPT_DIR/sudoers.d/tes3mp-easy}"
SYSTEMD_SOURCE_DIR="${TES3MP_EASY_SYSTEMD_SOURCE:-$SCRIPT_DIR/systemd}"
JOURNALD_SOURCE="${TES3MP_EASY_JOURNALD_SOURCE:-$SYSTEMD_SOURCE_DIR/journald.conf.d/tes3mp-easy.conf}"
SYSCTL_SOURCE="${TES3MP_EASY_SYSCTL_SOURCE:-$SCRIPT_DIR/sysctl.d/99-tes3mp-easy.conf}"
TES3MP_VERSION_FILE="${TES3MP_EASY_VERSION_FILE:-$SCRIPT_DIR/server/tes3mp-version.txt}"
# A release archive ships bin/ next to bootstrap.sh. Source checkouts do not,
# so this directory is optional: source installs keep the documented manual
# binary step, while the release archive installs both binaries in one run.
BIN_SOURCE_DIR="${TES3MP_EASY_BIN_SOURCE:-$SCRIPT_DIR/bin}"

readonly -a UNIT_FILES=(
    tes3mp.service
    tes3mp-panel.service
    tes3mp-easy-backup.service
    tes3mp-easy-backup.timer
)

prefixed() {
    printf '%s%s' "$INSTALL_ROOT" "$1"
}

SUDOERS_DIR=$(prefixed /etc/sudoers.d)
SUDOERS_DEST="$SUDOERS_DIR/tes3mp-easy"
LIBEXEC_DIR=$(prefixed /usr/local/libexec)
ROOTCTL_DEST="$LIBEXEC_DIR/tes3mp-easy-rootctl"
SYSTEMD_UNIT_DIR=$(prefixed /etc/systemd/system)
JOURNALD_CONFIG_DIR=$(prefixed /etc/systemd/journald.conf.d)
JOURNALD_CONFIG_DEST="$JOURNALD_CONFIG_DIR/tes3mp-easy.conf"
SYSCTL_CONFIG_DIR=$(prefixed /etc/sysctl.d)
SYSCTL_CONFIG_DEST="$SYSCTL_CONFIG_DIR/99-tes3mp-easy.conf"
PANEL_ENV_DIR=$(prefixed /etc/tes3mp-easy)
PANEL_ENV_DEST="$PANEL_ENV_DIR/panel.env"
PANEL_CERT_DEST="$PANEL_ENV_DIR/panel.crt"
PANEL_KEY_DEST="$PANEL_ENV_DIR/panel.key"
OPT_ROOT=$(prefixed /opt/tes3mp-easy)
TES3MP_RUNTIME_DIR="$OPT_ROOT/tes3mp"
TES3MP_INSTALL_INFO="$TES3MP_RUNTIME_DIR/.tes3mp-install-info"
SWAPFILE_PATH=$(prefixed /swapfile)
FSTAB_PATH=$(prefixed /etc/fstab)

# The base mods archive must satisfy the same contract as user uploads, so the
# metadata contains plain JSON without TES3MP's // comments. Empty CRC lists
# accept any client copy of the three base ESMs.
readonly BASE_REQUIRED_DATA='[{"Morrowind.esm":[]},{"Tribunal.esm":[]},{"Bloodmoon.esm":[]}]'

# A nominal 512 MiB VPS reports about 480-500 MiB MemTotal after firmware and
# kernel reservations; a nominal 7 GiB filesystem reports about 6.3 GiB total.
# A fresh 7 GiB host with a 512 MiB swap file can have about 4.5-5 GiB free, so
# the hard floor is 4.5 GiB and 5 GiB is only a warning. These thresholds still
# reject clearly undersized hosts.
readonly MIN_RAM_MIB=480
readonly RECOMMENDED_RAM_MIB=640
readonly MIN_SWAP_MIB=480
readonly SWAP_SIZE_MIB=512
readonly MIN_DISK_TOTAL_MIB=6144
readonly MIN_DISK_FREE_MIB=4608
readonly RECOMMENDED_DISK_FREE_MIB=5120

# Paths below are environment-overridable only to make the shell preflight
# testable without root, real systemd or the host's /proc. Production runs use
# the defaults.
OS_RELEASE="${TES3MP_EASY_OS_RELEASE:-/etc/os-release}"
PROC_ROOT="${TES3MP_EASY_PROC_ROOT:-/proc}"
MEMINFO="$PROC_ROOT/meminfo"
SYSTEMD_RUN_DIR="${TES3MP_EASY_SYSTEMD_RUN_DIR:-/run/systemd/system}"

DATA_ROOT="$DEFAULT_ROOT"
PUBLIC_ADDRESS="${TES3MP_EASY_PUBLIC_ADDRESS:-}"
# The TLS mode decides where the panel gets its certificate:
#   acme    — Let's Encrypt production, HTTP-01 on port 80 (default);
#   staging — the same flow against the Let's Encrypt staging CA: the
#             certificate is untrusted and the CA has generous rate limits, so
#             the mode is for experiments and pre-production checks;
#   files   — an operator-provided key pair, no CA traffic: hosts behind NAT,
#             without a public IPv4 or with an existing certificate.
# The panel only knows the `acme` and `files` modes; the staging mode reuses
# `acme` and adds the staging directory below.
TLS_MODE=acme
TLS_CERT_SOURCE=""
TLS_KEY_SOURCE=""
readonly -a TLS_MODES=(acme staging files)
readonly ACME_STAGING_DIRECTORY="https://acme-staging-v02.api.letsencrypt.org/directory"
CHECK_ONLY=false

passes=0
warnings=0
failures=0
NEED_SWAP=false

if [[ -t 1 && -z ${NO_COLOR:-} ]]; then
    readonly C_RED=$'\033[0;31m'
    readonly C_GREEN=$'\033[0;32m'
    readonly C_YELLOW=$'\033[1;33m'
    readonly C_BLUE=$'\033[0;34m'
    readonly C_RESET=$'\033[0m'
else
    readonly C_RED=""
    readonly C_GREEN=""
    readonly C_YELLOW=""
    readonly C_BLUE=""
    readonly C_RESET=""
fi

info() {
    printf '%s[bootstrap]%s %s\n' "$C_BLUE" "$C_RESET" "$*"
}

pass() {
    passes=$((passes + 1))
    printf '%s[ok]%s   %s\n' "$C_GREEN" "$C_RESET" "$*"
}

warn() {
    warnings=$((warnings + 1))
    printf '%s[warn]%s %s\n' "$C_YELLOW" "$C_RESET" "$*"
}

fail() {
    failures=$((failures + 1))
    printf '%s[fail]%s %s\n' "$C_RED" "$C_RESET" "$*" >&2
}

usage() {
    cat <<EOF
Usage: sudo ./bootstrap.sh [OPTIONS]

TES3MP Easy v2 bootstrap. It installs the host layout, the TES3MP runtime and,
when bin/ is shipped in a release archive, both server binaries. It can be run
repeatedly: existing users, groups, directories and a running server are kept.

Options:
  --check        run only the preflight and change nothing
  --root PATH    data root to validate and create (default $DEFAULT_ROOT)
  --public-address IP
                 public IPv4 for the Let's Encrypt certificate; auto-detected
                 when omitted (env TES3MP_EASY_PUBLIC_ADDRESS)
  --tls-mode MODE
                 acme (default), staging or files; see "Configures panel TLS"
  --tls-cert PATH
                 certificate for --tls-mode files (PEM, full chain)
  --tls-key PATH private key for --tls-mode files (PEM)
  -h, --help     show this help and exit

Checks:
  root privileges, Debian 12+/Ubuntu 24.04+, x86_64, systemd,
  RAM, swap and free disk. A swap total below 480 MiB is a warning, not a
  failure: a normal install creates a 512 MiB /swapfile.

Installs the privilege model:
  users and groups tes3mp/tes3mp-panel, the data directory layout,
  /usr/local/libexec/tes3mp-easy-rootctl and /etc/sudoers.d/tes3mp-easy.

Creates the swap safety net:
  when total swap is below 480 MiB, creates and activates a 512 MiB
  /swapfile and records it in /etc/fstab. --check only reports the plan.

Installs the systemd layout:
  tes3mp.service, tes3mp-panel.service, tes3mp-easy-backup.service,
  tes3mp-easy-backup.timer, the journald drop-in and vm.swappiness=10.

Installs the runtime:
  the pinned TES3MP runtime under /opt/tes3mp-easy/tes3mp, base configs in
  configs/, base mods-init-* and backups/mods/current.txt.

Installs server binaries:
  tes3mp-easy-ops and tes3mp-easy-panel from bin/ next to this script, when
  present. Source checkouts without bin/ install them manually.

Configures panel TLS:
  writes /etc/tes3mp-easy/panel.env and, in files mode,
  /etc/tes3mp-easy/panel.crt and panel.key (0640 root:tes3mp-panel).
  acme (default) obtains a six-day Let's Encrypt IP certificate through
  HTTP-01, so TCP port 80 must be reachable from the internet while the panel
  issues or renews a certificate. staging runs the same flow against the
  Let's Encrypt staging CA: the panel then serves an untrusted certificate and
  browsers warn. files copies an existing key pair and makes no CA requests,
  so it needs neither a public IPv4 nor port 80. The panel itself always
  listens only on port 443.

Starts:
  tes3mp.service, tes3mp-easy-backup.timer and, after a release install, the
  panel service. It waits for the panel's HTTPS health check, so an ACME
  failure stops the bootstrap with the last panel log lines. A Let's Encrypt
  rate limit is reported together with its retry time.

Exit status:
  0  all checks and installation steps passed (warnings are allowed)
  1  a preflight check or an installation step failed
  2  invalid command line
EOF
}

tls_mode_supported() {
    local mode
    for mode in "${TLS_MODES[@]}"; do
        if [[ $mode == "$1" ]]; then
            return 0
        fi
    done
    return 1
}

parse_args() {
    while (($# > 0)); do
        case "$1" in
            --check)
                CHECK_ONLY=true
                shift
                ;;
            --root)
                if (($# < 2)) || [[ -z $2 ]]; then
                    printf 'bootstrap: --root requires a path\n' >&2
                    usage >&2
                    exit 2
                fi
                DATA_ROOT=$2
                shift 2
                ;;
            --root=*)
                DATA_ROOT=${1#*=}
                shift
                ;;
            --public-address)
                if (($# < 2)) || [[ -z $2 ]]; then
                    printf 'bootstrap: --public-address requires an IPv4 address\n' >&2
                    usage >&2
                    exit 2
                fi
                PUBLIC_ADDRESS=$2
                shift 2
                ;;
            --public-address=*)
                PUBLIC_ADDRESS=${1#*=}
                shift
                ;;
            --tls-mode)
                if (($# < 2)) || [[ -z $2 ]]; then
                    printf 'bootstrap: --tls-mode requires acme, staging or files\n' >&2
                    usage >&2
                    exit 2
                fi
                TLS_MODE=$2
                shift 2
                ;;
            --tls-mode=*)
                TLS_MODE=${1#*=}
                shift
                ;;
            --tls-cert)
                if (($# < 2)) || [[ -z $2 ]]; then
                    printf 'bootstrap: --tls-cert requires a path\n' >&2
                    usage >&2
                    exit 2
                fi
                TLS_CERT_SOURCE=$2
                shift 2
                ;;
            --tls-cert=*)
                TLS_CERT_SOURCE=${1#*=}
                shift
                ;;
            --tls-key)
                if (($# < 2)) || [[ -z $2 ]]; then
                    printf 'bootstrap: --tls-key requires a path\n' >&2
                    usage >&2
                    exit 2
                fi
                TLS_KEY_SOURCE=$2
                shift 2
                ;;
            --tls-key=*)
                TLS_KEY_SOURCE=${1#*=}
                shift
                ;;
            -h | --help)
                usage
                exit 0
                ;;
            *)
                printf 'bootstrap: unknown option %q\n' "$1" >&2
                usage >&2
                exit 2
                ;;
        esac
    done

    if [[ -z $DATA_ROOT ]]; then
        printf 'bootstrap: --root must not be empty\n' >&2
        exit 2
    fi
    if [[ $DATA_ROOT != /* ]]; then
        printf 'bootstrap: --root must be an absolute path: %q\n' "$DATA_ROOT" >&2
        exit 2
    fi
    if [[ $DATA_ROOT == "/" ]]; then
        printf 'bootstrap: --root must not be /\n' >&2
        exit 2
    fi
    while [[ $DATA_ROOT == */ ]]; do
        DATA_ROOT=${DATA_ROOT%/}
    done

    if ! tls_mode_supported "$TLS_MODE"; then
        printf 'bootstrap: --tls-mode must be acme, staging or files: %q\n' \
            "$TLS_MODE" >&2
        exit 2
    fi
    if [[ $TLS_MODE == files ]]; then
        if [[ -z $TLS_CERT_SOURCE || -z $TLS_KEY_SOURCE ]]; then
            printf 'bootstrap: --tls-mode files requires --tls-cert PATH and --tls-key PATH\n' >&2
            exit 2
        fi
    elif [[ -n $TLS_CERT_SOURCE || -n $TLS_KEY_SOURCE ]]; then
        printf 'bootstrap: --tls-cert and --tls-key are only used with --tls-mode files\n' >&2
        exit 2
    fi
}

# version_ge REQUIRED ACTUAL returns success when ACTUAL >= REQUIRED.
# Only numeric dotted versions are accepted; os-release guarantees that shape
# for Debian and Ubuntu VERSION_ID.
version_ge() {
    local required=$1
    local actual=$2
    local -a required_parts=()
    local -a actual_parts=()
    local i max part actual_part

    IFS=. read -r -a required_parts <<<"$required"
    IFS=. read -r -a actual_parts <<<"$actual"

    max=${#required_parts[@]}
    if ((${#actual_parts[@]} > max)); then
        max=${#actual_parts[@]}
    fi

    for ((i = 0; i < max; i++)); do
        part=${required_parts[i]:-0}
        actual_part=${actual_parts[i]:-0}
        if [[ ! $part =~ ^[0-9]+$ || ! $actual_part =~ ^[0-9]+$ ]]; then
            return 1
        fi
        if ((10#$actual_part > 10#$part)); then
            return 0
        fi
        if ((10#$actual_part < 10#$part)); then
            return 1
        fi
    done

    return 0
}

check_root() {
    local uid
    if ! uid=$(id -u 2>/dev/null); then
        fail "root: cannot determine the current user id"
        return
    fi
    if [[ $uid != "0" ]]; then
        fail "root: bootstrap must run as root (use sudo)"
        return
    fi
    pass "root privileges"
}

check_os() {
    if [[ ! -r $OS_RELEASE ]]; then
        fail "OS: cannot read $OS_RELEASE"
        return
    fi

    local line key value os_id="" version=""
    while IFS= read -r line || [[ -n $line ]]; do
        line=${line%$'\r'}
        if [[ -z $line || $line == \#* ]]; then
            continue
        fi
        key=${line%%=*}
        value=${line#*=}
        value=${value#\"}
        value=${value%\"}
        case "$key" in
            ID) os_id=$value ;;
            VERSION_ID) version=$value ;;
        esac
    done <"$OS_RELEASE"

    local name min_version
    case "$os_id" in
        debian)
            name="Debian"
            min_version="12"
            ;;
        ubuntu)
            name="Ubuntu"
            min_version="24.04"
            ;;
        "")
            fail "OS: ID is missing from $OS_RELEASE"
            return
            ;;
        *)
            fail "OS: $os_id is not supported (need Debian 12+ or Ubuntu 24.04+)"
            return
            ;;
    esac

    if [[ -z $version ]]; then
        fail "OS: VERSION_ID is missing for $name"
        return
    fi
    if ! version_ge "$min_version" "$version"; then
        fail "OS: $name $version is too old (need $min_version+)"
        return
    fi

    pass "OS: $name $version"
}

check_arch() {
    local machine
    if ! machine=$(uname -m 2>/dev/null); then
        fail "architecture: cannot run uname -m"
        return
    fi
    if [[ $machine != "x86_64" ]]; then
        fail "architecture: $machine is not supported (need x86_64)"
        return
    fi
    pass "architecture: x86_64"
}

check_systemd() {
    local reason=""
    local comm=""

    if [[ -r "$PROC_ROOT/1/comm" ]]; then
        IFS= read -r comm <"$PROC_ROOT/1/comm" || true
        comm=${comm%$'\r'}
    fi
    if [[ $comm != "systemd" ]]; then
        reason="PID 1 is ${comm:-unknown}, not systemd"
    elif [[ ! -d $SYSTEMD_RUN_DIR ]]; then
        reason="$SYSTEMD_RUN_DIR is missing"
    elif ! command -v systemctl >/dev/null 2>&1; then
        reason="systemctl is not installed"
    elif ! systemctl --version >/dev/null 2>&1; then
        reason="systemctl is not usable"
    fi

    if [[ -n $reason ]]; then
        fail "systemd: $reason"
        return
    fi
    pass "systemd: PID 1 is systemd"
}


meminfo_kb() {
    local key=$1
    local file=$2
    local name value unit

    while read -r name value unit; do
        if [[ $name == "$key:" ]]; then
            printf '%s\n' "$value"
            return 0
        fi
    done <"$file"

    return 1
}

check_ram() {
    if [[ ! -r $MEMINFO ]]; then
        fail "RAM: cannot read $MEMINFO"
        return
    fi

    local mem_kb mem_mib
    if ! mem_kb=$(meminfo_kb MemTotal "$MEMINFO"); then
        fail "RAM: MemTotal is missing from $MEMINFO"
        return
    fi
    if [[ ! $mem_kb =~ ^[0-9]+$ ]]; then
        fail "RAM: invalid MemTotal value '$mem_kb'"
        return
    fi

    mem_mib=$((mem_kb / 1024))
    if ((mem_mib < MIN_RAM_MIB)); then
        fail "RAM: $mem_mib MiB is below the ${MIN_RAM_MIB} MiB minimum"
        return
    fi

    pass "RAM: $mem_mib MiB total"
    if ((mem_mib < RECOMMENDED_RAM_MIB)); then
        warn "RAM: $mem_mib MiB is below the recommended ${RECOMMENDED_RAM_MIB} MiB"
    fi
}

swap_total_kb() {
    local file=$1
    local filename type size used priority
    local header=1
    local total=0

    while read -r filename type size used priority; do
        if ((header)); then
            header=0
            if [[ $filename == "Filename" ]]; then
                continue
            fi
        fi
        if [[ $size =~ ^[0-9]+$ ]]; then
            total=$((total + size))
        fi
    done <"$file"

    printf '%s\n' "$total"
}

check_swap() {
    local swaps_file="$PROC_ROOT/swaps"
    if [[ ! -r $swaps_file ]]; then
        fail "swap: cannot read $swaps_file"
        return
    fi

    local swap_kb swap_mib
    swap_kb=$(swap_total_kb "$swaps_file")
    swap_mib=$((swap_kb / 1024))
    pass "swap: $swap_mib MiB total"
    if ((swap_mib < MIN_SWAP_MIB)); then
        NEED_SWAP=true
        if [[ $CHECK_ONLY == true ]]; then
            warn "swap: $swap_mib MiB is below the ${MIN_SWAP_MIB} MiB minimum; would create ${SWAP_SIZE_MIB} MiB at $SWAPFILE_PATH"
        else
            warn "swap: $swap_mib MiB is below the ${MIN_SWAP_MIB} MiB minimum; installer will create ${SWAP_SIZE_MIB} MiB at $SWAPFILE_PATH"
        fi
    fi
}

check_disk() {
    local probe=$DATA_ROOT
    while [[ ! -e $probe ]]; do
        if [[ $probe == "/" ]]; then
            break
        fi
        probe=${probe%/*}
        if [[ -z $probe ]]; then
            probe="/"
        fi
    done

    local df_output
    if ! df_output=$(df -Pk -- "$probe" 2>/dev/null); then
        fail "disk: cannot read free space for $probe"
        return
    fi

    local -a df_lines=()
    mapfile -t df_lines <<<"$df_output"
    if ((${#df_lines[@]} < 2)); then
        fail "disk: unexpected df output for $probe"
        return
    fi

    local filesystem total_kb used_kb available_kb capacity rest
    read -r filesystem total_kb used_kb available_kb capacity rest <<<"${df_lines[1]}"
    if [[ ! $total_kb =~ ^[0-9]+$ || ! $available_kb =~ ^[0-9]+$ ]]; then
        fail "disk: cannot parse df output for $probe"
        return
    fi

    local total_mib=$((total_kb / 1024))
    local available_mib=$((available_kb / 1024))

    if ((total_mib < MIN_DISK_TOTAL_MIB)); then
        fail "disk: $total_mib MiB total is below the ${MIN_DISK_TOTAL_MIB} MiB minimum (nominal 7 GiB)"
        return
    fi
    if ((available_mib < MIN_DISK_FREE_MIB)); then
        fail "disk: $available_mib MiB free, need at least ${MIN_DISK_FREE_MIB} MiB"
        return
    fi

    pass "disk: $available_mib MiB free of $total_mib MiB at $probe"
    if ((available_mib < RECOMMENDED_DISK_FREE_MIB)); then
        warn "disk: $available_mib MiB free is below the recommended ${RECOMMENDED_DISK_FREE_MIB} MiB"
    fi
}

print_summary() {
    local result="Preflight passed"
    if ((failures > 0)); then
        result="Preflight failed"
    fi
    if ((warnings > 0)); then
        result+=" with warnings"
    fi

    local checks=$((passes + failures))
    local summary
    summary=$(printf '%s: %d checks, %d warning(s), %d error(s).' \
        "$result" "$checks" "$warnings" "$failures")

    if ((failures > 0)); then
        printf '%s%s%s\n' "$C_RED" "$summary" "$C_RESET" >&2
    else
        printf '%s%s%s\n' "$C_GREEN" "$summary" "$C_RESET"
    fi
}

abort_install() {
    printf '%s[fail]%s %s\n' "$C_RED" "$C_RESET" "$*" >&2
    printf '%sInstallation failed.%s\n' "$C_RED" "$C_RESET" >&2
    exit 1
}

run_install() {
    if ! "$@"; then
        abort_install "command failed: $*"
    fi
}

swap_file_active() {
    local path=$1
    local line first
    [[ -r "$PROC_ROOT/swaps" ]] || return 1
    while IFS= read -r line || [[ -n $line ]]; do
        first=${line%%[[:space:]]*}
        if [[ $first == "$path" ]]; then
            return 0
        fi
    done <"$PROC_ROOT/swaps"
    return 1
}

create_swapfile() {
    if command -v fallocate >/dev/null 2>&1 &&
        fallocate -l "${SWAP_SIZE_MIB}M" "$SWAPFILE_PATH"; then
        return 0
    fi
    if command -v dd >/dev/null 2>&1 &&
        dd if=/dev/zero of="$SWAPFILE_PATH" bs=1M count="$SWAP_SIZE_MIB" status=none; then
        return 0
    fi
    return 1
}

ensure_swap_fstab() {
    if [[ -f $FSTAB_PATH ]]; then
        local line
        while IFS= read -r line || [[ -n $line ]]; do
            if [[ $line == "$SWAPFILE_PATH"[[:space:]]*"none"[[:space:]]*"swap"* ]]; then
                pass "swap: $SWAPFILE_PATH is already recorded in $FSTAB_PATH"
                return
            fi
        done <"$FSTAB_PATH"
    fi
    if ! printf '%s none swap sw 0 0\n' "$SWAPFILE_PATH" >>"$FSTAB_PATH"; then
        warn "swap: cannot record $SWAPFILE_PATH in $FSTAB_PATH"
        return
    fi
    pass "swap: recorded $SWAPFILE_PATH in $FSTAB_PATH"
}

install_swap() {
    if [[ $NEED_SWAP != true ]]; then
        return
    fi

    if swap_file_active "$SWAPFILE_PATH"; then
        pass "swap: $SWAPFILE_PATH is already active"
        ensure_swap_fstab
        return
    fi

    info "Creating a ${SWAP_SIZE_MIB} MiB swap file at $SWAPFILE_PATH"
    local cmd
    for cmd in mkswap swapon; do
        if ! command -v "$cmd" >/dev/null 2>&1; then
            warn "swap: cannot create $SWAPFILE_PATH: $cmd is not installed"
            return
        fi
    done

    if ! create_swapfile; then
        warn "swap: cannot create $SWAPFILE_PATH"
        return
    fi
    if ! chmod 0600 "$SWAPFILE_PATH"; then
        warn "swap: cannot set mode 0600 on $SWAPFILE_PATH"
        return
    fi
    if command -v chown >/dev/null 2>&1 && ! chown root:root "$SWAPFILE_PATH"; then
        warn "swap: cannot make $SWAPFILE_PATH root-owned"
        return
    fi
    if ! mkswap "$SWAPFILE_PATH" >/dev/null; then
        warn "swap: mkswap failed for $SWAPFILE_PATH"
        return
    fi
    if ! swapon "$SWAPFILE_PATH"; then
        warn "swap: swapon failed for $SWAPFILE_PATH"
        return
    fi

    pass "swap: activated $SWAPFILE_PATH (${SWAP_SIZE_MIB} MiB)"
    ensure_swap_fstab
}

ensure_group() {
    local name=$1

    if getent group "$name" >/dev/null 2>&1; then
        pass "group: $name already exists"
        return
    fi

    run_install groupadd --system "$name"
    pass "group: $name created"
}

# ensure_user USER PRIMARY_GROUP SUPPLEMENTARY_GROUP
ensure_user() {
    local user=$1
    local primary_group=$2
    local supplementary_group=$3

    if getent passwd "$user" >/dev/null 2>&1; then
        pass "user: $user already exists"
    else
        run_install useradd --system \
            --gid "$primary_group" \
            --groups "$supplementary_group" \
            --home-dir /nonexistent \
            --no-create-home \
            --shell /usr/sbin/nologin \
            "$user"
        pass "user: $user created"
    fi

    # -aG is idempotent: it adds groups if missing and keeps existing ones. The
    # primary group is included so a pre-existing account with a different
    # primary group still ends up in the required group.
    run_install usermod -aG "$primary_group,$supplementary_group" "$user"
}

# ensure_dir PATH OWNER GROUP MODE
ensure_dir() {
    local path=$1
    local owner=$2
    local group=$3
    local mode=$4

    run_install install -d -m "$mode" -o "$owner" -g "$group" "$path"
    pass "directory: $path ($owner:$group $mode)"
}

install_runtime_packages() {
    if ! command -v apt-get >/dev/null 2>&1; then
        abort_install "apt-get is required to install the TES3MP runtime dependencies"
    fi

    local -x DEBIAN_FRONTEND=noninteractive
    # unattended-upgrades can hold the dpkg frontend lock for several minutes
    # right after boot. Wait for it instead of failing the installation.
    run_install apt-get -o DPkg::Lock::Timeout=300 update
    run_install apt-get -o DPkg::Lock::Timeout=300 install -y --no-install-recommends \
        libluajit-5.1-2 ca-certificates openssl curl tar gzip rsync coreutils iproute2
    pass "packages: TES3MP runtime dependencies installed"
}

# load_tes3mp_release fills TES3MP_VERSION, TES3MP_URL and TES3MP_SHA256 from
# server/tes3mp-version.txt. The checksum is mandatory: an unverified download
# must never be extracted as root.
load_tes3mp_release() {
    if [[ ! -r $TES3MP_VERSION_FILE ]]; then
        abort_install "TES3MP version file not found: $TES3MP_VERSION_FILE"
    fi

    TES3MP_VERSION=""
    TES3MP_URL=""
    TES3MP_SHA256=""
    local line
    while IFS= read -r line || [[ -n $line ]]; do
        line=${line%$'\r'}
        [[ -z $line || $line == \#* ]] && continue
        case "$line" in
            TES3MP_VERSION=*)
                [[ -z $TES3MP_VERSION ]] || abort_install "duplicate TES3MP_VERSION"
                TES3MP_VERSION=${line#*=}
                ;;
            TES3MP_URL=*)
                [[ -z $TES3MP_URL ]] || abort_install "duplicate TES3MP_URL"
                TES3MP_URL=${line#*=}
                ;;
            TES3MP_SHA256=*)
                [[ -z $TES3MP_SHA256 ]] || abort_install "duplicate TES3MP_SHA256"
                TES3MP_SHA256=${line#*=}
                ;;
            *)
                abort_install "unexpected line in $TES3MP_VERSION_FILE: $line"
                ;;
        esac
    done <"$TES3MP_VERSION_FILE"

    if [[ ! $TES3MP_VERSION =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        abort_install "TES3MP_VERSION is missing or invalid in $TES3MP_VERSION_FILE"
    fi
    if [[ ! $TES3MP_URL =~ ^https://[^[:space:]]+$ && ! $TES3MP_URL =~ ^file://[^[:space:]]+$ ]]; then
        abort_install "TES3MP_URL is missing or invalid in $TES3MP_VERSION_FILE"
    fi
    if [[ ! $TES3MP_SHA256 =~ ^[0-9a-f]{64}$ ]]; then
        abort_install "TES3MP_SHA256 is missing or invalid in $TES3MP_VERSION_FILE"
    fi
}

file_sha256() {
    local output
    if ! output=$(sha256sum -- "$1"); then
        return 1
    fi
    printf '%s' "${output%% *}"
}

runtime_marker_matches() {
    local version="" url="" sha="" line
    if [[ ! -f $TES3MP_INSTALL_INFO ]]; then
        return 1
    fi
    while IFS= read -r line || [[ -n $line ]]; do
        case "$line" in
            TES3MP_VERSION=*) version=${line#*=} ;;
            TES3MP_URL=*) url=${line#*=} ;;
            TES3MP_SHA256=*) sha=${line#*=} ;;
        esac
    done <"$TES3MP_INSTALL_INFO"

    [[ $version == "$TES3MP_VERSION" &&
        $url == "$TES3MP_URL" &&
        $sha == "$TES3MP_SHA256" ]]
}

runtime_files_present() {
    local path
    for path in \
        tes3mp-server \
        tes3mp-server.x86_64 \
        tes3mp-prelaunch \
        openmw.cfg \
        tes3mp-server-default.cfg \
        server/scripts/config.lua \
        server/scripts/customScripts.lua \
        server/scripts/serverCore.lua \
        server/data/requiredDataFiles.json; do
        if [[ ! -f $TES3MP_RUNTIME_DIR/$path ]]; then
            return 1
        fi
    done
    [[ -d $TES3MP_RUNTIME_DIR/lib &&
        -x $TES3MP_RUNTIME_DIR/tes3mp-server &&
        -x $TES3MP_RUNTIME_DIR/tes3mp-server.x86_64 ]]
}

install_tes3mp_runtime() {
    if [[ -e $TES3MP_RUNTIME_DIR || -L $TES3MP_RUNTIME_DIR ]]; then
        if ! runtime_marker_matches; then
            abort_install "$TES3MP_RUNTIME_DIR exists without the expected install marker; remove it before reinstalling"
        fi
        if ! runtime_files_present; then
            abort_install "$TES3MP_RUNTIME_DIR is incomplete; remove it before reinstalling"
        fi
        pass "runtime: TES3MP $TES3MP_VERSION already installed"
        return
    fi

    ensure_dir "$OPT_ROOT" root root 0755

    local archive="$OPT_ROOT/.tes3mp-$TES3MP_VERSION.tar.gz.$$"
    local staging="$OPT_ROOT/.tes3mp-$TES3MP_VERSION.$$"
    local digest

    if ! curl -fsSL --retry 3 -o "$archive" "$TES3MP_URL"; then
        rm -f -- "$archive"
        abort_install "cannot download TES3MP $TES3MP_VERSION from $TES3MP_URL"
    fi
    if ! digest=$(file_sha256 "$archive"); then
        rm -f -- "$archive"
        abort_install "cannot compute sha256 of $archive"
    fi
    if [[ $digest != "$TES3MP_SHA256" ]]; then
        rm -f -- "$archive"
        abort_install "TES3MP archive checksum mismatch: got $digest, want $TES3MP_SHA256"
    fi
    pass "runtime: TES3MP $TES3MP_VERSION archive verified (sha256)"

    rm -rf -- "$staging"
    if ! mkdir -m 0700 -- "$staging"; then
        rm -f -- "$archive"
        abort_install "cannot create runtime staging directory $staging"
    fi
    if ! tar -xzf "$archive" --strip-components=1 --no-same-owner -C "$staging"; then
        rm -rf -- "$staging"
        rm -f -- "$archive"
        abort_install "cannot extract TES3MP $TES3MP_VERSION archive"
    fi
    rm -f -- "$archive"

    if [[ ! -f $staging/tes3mp-server ||
        ! -f $staging/tes3mp-server.x86_64 ||
        ! -d $staging/lib ||
        ! -f $staging/server/scripts/serverCore.lua ]]; then
        rm -rf -- "$staging"
        abort_install "TES3MP archive does not have the expected TES3MP-server layout"
    fi

    if ! chown -R root:root "$staging"; then
        rm -rf -- "$staging"
        abort_install "cannot make the TES3MP runtime root-owned"
    fi
    if ! chmod 0755 "$staging" "$staging/tes3mp-server" \
        "$staging/tes3mp-server.x86_64" "$staging/tes3mp-prelaunch"; then
        rm -rf -- "$staging"
        abort_install "cannot set TES3MP runtime executable modes"
    fi

    # Write the marker while the tree is still private staging. After the
    # atomic rename, an existing runtime directory always means a complete
    # install; there is no window where a partial tree lacks the marker.
    if ! {
        printf 'TES3MP_VERSION=%s\n' "$TES3MP_VERSION"
        printf 'TES3MP_URL=%s\n' "$TES3MP_URL"
        printf 'TES3MP_SHA256=%s\n' "$TES3MP_SHA256"
    } >"$staging/.tes3mp-install-info"; then
        rm -rf -- "$staging"
        abort_install "cannot write the TES3MP install marker"
    fi
    if ! chmod 0644 "$staging/.tes3mp-install-info"; then
        rm -rf -- "$staging"
        abort_install "cannot set the TES3MP install marker mode"
    fi
    # -T makes this a true atomic rename: without it, a runtime directory
    # appearing between the check and this line would receive the staging tree
    # as a subdirectory instead of making mv fail.
    if ! mv -T -- "$staging" "$TES3MP_RUNTIME_DIR"; then
        rm -rf -- "$staging"
        abort_install "cannot move the TES3MP runtime into $TES3MP_RUNTIME_DIR"
    fi
    pass "runtime: TES3MP $TES3MP_VERSION installed under $TES3MP_RUNTIME_DIR"
}

# normalize_config_file PATH
#
# Existing installations may carry config files created by an older release
# with a wider mode. Doctor and the mutation gate require 0600, so upgrade must
# normalize ownership and mode without touching the contents.
normalize_config_file() {
    local path=$1

    run_install chown tes3mp:tes3mp "$path"
    run_install chmod 0600 "$path"
    pass "config: $path already exists (tes3mp:tes3mp 0600)"
}

# ensure_config_file SOURCE DESTINATION
ensure_config_file() {
    local source=$1
    local destination=$2

    if [[ -L $destination ]]; then
        abort_install "config path $destination is a symlink; refusing to write through it"
    fi
    if [[ -e $destination ]]; then
        if [[ ! -f $destination ]]; then
            abort_install "config path $destination is not a regular file"
        fi
        normalize_config_file "$destination"
        return
    fi
    if [[ ! -f $source ]]; then
        abort_install "runtime config source not found: $source"
    fi
    run_install install -m 0600 -o tes3mp -g tes3mp "$source" "$destination"
    pass "config: $destination seeded from the TES3MP runtime"
}

# disable_master_server fresh-install config. The runtime TES3MP archive ships
# [MasterServer] enabled = true; a fresh tes3mp-easy install must not announce
# the server until the admin explicitly enables it.
disable_master_server() {
    local path=$1
    local temp="$DATA_ROOT/.staging/master-server.$$"
    local line in_section=0

    if ! {
        while IFS= read -r line || [[ -n $line ]]; do
            if [[ $line =~ ^\[(.*)\]$ ]]; then
                if [[ ${BASH_REMATCH[1]} == MasterServer ]]; then
                    in_section=1
                else
                    in_section=0
                fi
            elif ((in_section)) &&
                [[ $line =~ ^[[:space:]]*enabled[[:space:]]*= ]]; then
                line="enabled = false"
            fi
            printf '%s\n' "$line"
        done <"$path"
    } >"$temp"; then
        rm -f -- "$temp"
        abort_install "cannot rewrite MasterServer in $path"
    fi
    run_install install -m 0600 -o tes3mp -g tes3mp "$temp" "$path"
    rm -f -- "$temp"
    pass "config: MasterServer disabled on fresh install"
}

install_base_configs() {
    local server_cfg="$DATA_ROOT/configs/tes3mp-server-default.cfg"
    local server_seeded=0
    if [[ ! -e $server_cfg ]]; then
        server_seeded=1
    fi
    ensure_config_file "$TES3MP_RUNTIME_DIR/tes3mp-server-default.cfg" \
        "$server_cfg"
    if ((server_seeded)); then
        disable_master_server "$server_cfg"
    fi
    ensure_config_file "$TES3MP_RUNTIME_DIR/server/scripts/config.lua" \
        "$DATA_ROOT/configs/config.lua"
    ensure_config_file "$TES3MP_RUNTIME_DIR/server/scripts/customScripts.lua" \
        "$DATA_ROOT/configs/customScripts.lua"

    local banlist="$DATA_ROOT/configs/banlist.json"
    if [[ -L $banlist ]]; then
        abort_install "config path $banlist is a symlink; refusing to write through it"
    fi
    if [[ -e $banlist ]]; then
        if [[ ! -f $banlist ]]; then
            abort_install "config path $banlist is not a regular file"
        fi
        normalize_config_file "$banlist"
    else
        local temp="$DATA_ROOT/.staging/banlist.json.$$"
        if ! printf '{\n  "playerNames": [],\n  "ipAddresses": []\n}\n' >"$temp"; then
            abort_install "cannot write $temp"
        fi
        run_install install -m 0600 -o tes3mp -g tes3mp "$temp" "$banlist"
        rm -f -- "$temp"
        pass "config: $banlist created"
    fi
}

ensure_base_required_data() {
    local required="$DATA_ROOT/mods/plugins/requiredDataFiles.json"
    if [[ -L $required ]]; then
        abort_install "mods path $required is a symlink; refusing to write through it"
    fi
    if [[ -e $required ]]; then
        if [[ ! -f $required ]]; then
            abort_install "mods path $required is not a regular file"
        fi
        pass "mods: $required already exists"
        return
    fi

    local temp="$DATA_ROOT/.staging/requiredDataFiles.json.$$"
    if ! printf '%s\n' "$BASE_REQUIRED_DATA" >"$temp"; then
        abort_install "cannot write $temp"
    fi
    run_install install -m 0640 -o tes3mp -g tes3mp "$temp" "$required"
    rm -f -- "$temp"
    pass "mods: base $required created"
}

# archive_timestamp_is_valid accepts only a real UTC timestamp in the compact
# form used by archive names. GNU date normalizes invalid dates, so its rendered
# result is compared with the input instead of trusting the exit status alone.
archive_timestamp_is_valid() {
    local timestamp=$1
    [[ $timestamp =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}_[0-9]{2}-[0-9]{2}-[0-9]{2}$ ]] || return 1

    local date_part=${timestamp%%_*}
    local time_part=${timestamp#*_}
    local rendered
    if ! rendered=$(date -u -d "$date_part ${time_part//-/:}" \
        '+%Y-%m-%d_%H-%M-%S' 2>/dev/null); then
        return 1
    fi
    [[ $rendered == "$timestamp" ]]
}

# mods_current_is_valid accepts the exact canonical one-line format used by
# ops, checks that the archive still exists and verifies its sha256.
mods_current_is_valid() {
    local current="$DATA_ROOT/backups/mods/current.txt"
    if [[ ! -f $current || -L $current ]]; then
        return 1
    fi
    local content=""
    LC_ALL=C IFS= read -r -N 4097 content <"$current" || true
    if ((${#content} > 4096)); then
        return 1
    fi
    [[ $content == *$'\n' ]] || return 1
    content=${content%$'\n'}
    [[ $content != *$'\n'* ]] || return 1

    local sha name extra
    read -r sha name extra <<<"$content"
    [[ -z $extra && $content == "$sha $name" ]] || return 1
    [[ $sha =~ ^[0-9a-f]{64}$ ]] || return 1

    if [[ $name =~ ^mods(-init)?-([0-9]{4}-[0-9]{2}-[0-9]{2}_[0-9]{2}-[0-9]{2}-[0-9]{2})(-(0[1-9]|[1-9][0-9]+))?\.tar\.gz$ ]]; then
        archive_timestamp_is_valid "${BASH_REMATCH[2]}" || return 1
    else
        return 1
    fi
    [[ -f $DATA_ROOT/backups/mods/$name && ! -L $DATA_ROOT/backups/mods/$name ]] || return 1

    local digest
    digest=$(file_sha256 "$DATA_ROOT/backups/mods/$name") || return 1
    [[ $digest == "$sha" ]]
}

install_base_mods_archive() {
    if [[ -e $DATA_ROOT/backups/mods/current.txt ||
        -L $DATA_ROOT/backups/mods/current.txt ]]; then
        if ! mods_current_is_valid; then
            abort_install "$DATA_ROOT/backups/mods/current.txt is not canonical; refusing to overwrite it"
        fi
        pass "mods: canonical current.txt already points to a valid archive"
        return
    fi

    local timestamp name suffix=1
    timestamp=$(date -u +%Y-%m-%d_%H-%M-%S)
    name="mods-init-$timestamp.tar.gz"
    while [[ -e $DATA_ROOT/backups/mods/$name ]]; do
        if ((suffix > 999)); then
            abort_install "cannot allocate a unique mods-init archive name"
        fi
        name=$(printf 'mods-init-%s-%02d.tar.gz' "$timestamp" "$suffix")
        suffix=$((suffix + 1))
    done

    local staging="$DATA_ROOT/.staging/mods-init.$$"
    local archive_part="$DATA_ROOT/.staging/$name.part"
    local current_part="$DATA_ROOT/.staging/current.txt.$$"

    rm -rf -- "$staging"
    if ! mkdir -p -- "$staging/plugins" "$staging/scripts"; then
        abort_install "cannot create base mods staging directory"
    fi
    if ! install -m 0644 "$DATA_ROOT/mods/plugins/requiredDataFiles.json" \
        "$staging/plugins/requiredDataFiles.json"; then
        rm -rf -- "$staging"
        abort_install "cannot stage requiredDataFiles.json"
    fi
    if ! tar -czf "$archive_part" -C "$staging" plugins scripts; then
        rm -rf -- "$staging"
        rm -f -- "$archive_part"
        abort_install "cannot create the base mods archive"
    fi
    rm -rf -- "$staging"

    run_install install -m 0640 -o tes3mp -g tes3mp \
        "$archive_part" "$DATA_ROOT/backups/mods/$name"
    rm -f -- "$archive_part"

    local digest
    if ! digest=$(file_sha256 "$DATA_ROOT/backups/mods/$name"); then
        abort_install "cannot checksum the base mods archive"
    fi
    if ! printf '%s %s\n' "$digest" "$name" >"$current_part"; then
        abort_install "cannot write $current_part"
    fi
    run_install install -m 0640 -o tes3mp -g tes3mp \
        "$current_part" "$DATA_ROOT/backups/mods/.current.txt.$$"
    run_install mv -f -T -- "$DATA_ROOT/backups/mods/.current.txt.$$" \
        "$DATA_ROOT/backups/mods/current.txt"
    rm -f -- "$current_part"
    pass "mods: created $name and canonical backups/mods/current.txt"
}

replace_runtime_link() {
    local link=$1
    local target=$2

    if [[ -L $link ]]; then
        if [[ $(readlink -- "$link") == "$target" ]]; then
            return
        fi
        run_install rm -f -- "$link"
    elif [[ -e $link ]]; then
        if [[ -d $link ]]; then
            # The pinned TES3MP archive ships empty data directories with
            # .gitkeep placeholders. They are root-owned runtime content, not
            # user data, so replace the whole tree with the data-root symlink.
            run_install rm -rf -- "$link"
        else
            run_install rm -f -- "$link"
        fi
    fi

    run_install ln -s -- "$target" "$link"
    pass "runtime link: $link -> $target"
}

wire_runtime_to_data_root() {
    replace_runtime_link "$TES3MP_RUNTIME_DIR/tes3mp-server-default.cfg" \
        "$DATA_ROOT/configs/tes3mp-server-default.cfg"
    replace_runtime_link "$TES3MP_RUNTIME_DIR/server/scripts/config.lua" \
        "$DATA_ROOT/configs/config.lua"
    replace_runtime_link "$TES3MP_RUNTIME_DIR/server/scripts/customScripts.lua" \
        "$DATA_ROOT/configs/customScripts.lua"
    replace_runtime_link "$TES3MP_RUNTIME_DIR/server/scripts/custom" \
        "$DATA_ROOT/mods/scripts"

    replace_runtime_link "$TES3MP_RUNTIME_DIR/server/data/player" \
        "$DATA_ROOT/players"
    replace_runtime_link "$TES3MP_RUNTIME_DIR/server/data/cell" \
        "$DATA_ROOT/world/cell"
    replace_runtime_link "$TES3MP_RUNTIME_DIR/server/data/world" \
        "$DATA_ROOT/world/world"
    replace_runtime_link "$TES3MP_RUNTIME_DIR/server/data/map" \
        "$DATA_ROOT/world/map"
    replace_runtime_link "$TES3MP_RUNTIME_DIR/server/data/recordstore" \
        "$DATA_ROOT/world/recordstore"
    replace_runtime_link "$TES3MP_RUNTIME_DIR/server/data/custom" \
        "$DATA_ROOT/world/custom"
    replace_runtime_link "$TES3MP_RUNTIME_DIR/server/data/requiredDataFiles.json" \
        "$DATA_ROOT/mods/plugins/requiredDataFiles.json"
    replace_runtime_link "$TES3MP_RUNTIME_DIR/server/data/banlist.json" \
        "$DATA_ROOT/configs/banlist.json"
}

install_runtime_home() {
    # tes3mp-prelaunch reads $HOME/.config/openmw before falling back to the
    # package directory. HOME is /tes3mp-easy in the unit; pre-create the XDG
    # directories so ConfigurationManager does not need to write under the
    # read-only systemd mount namespace.
    ensure_dir "$DATA_ROOT/.config" tes3mp tes3mp 0700
    ensure_dir "$DATA_ROOT/.config/openmw" tes3mp tes3mp 0700
    ensure_dir "$DATA_ROOT/.local" tes3mp tes3mp 0700
    ensure_dir "$DATA_ROOT/.local/share" tes3mp tes3mp 0700
    ensure_dir "$DATA_ROOT/.local/share/openmw" tes3mp tes3mp 0700
}

install_privilege_layout() {
    if ! command -v getent >/dev/null 2>&1; then
        abort_install "getent is required to check users and groups"
    fi

    ensure_group tes3mp
    ensure_group tes3mp-panel

    if ! getent group systemd-journal >/dev/null 2>&1; then
        abort_install "group systemd-journal is missing"
    fi

    ensure_user tes3mp tes3mp systemd-journal
    ensure_user tes3mp-panel tes3mp-panel tes3mp

    ensure_dir "$DATA_ROOT" root root 0755
    ensure_dir "$DATA_ROOT/configs" tes3mp tes3mp 0700

    local subdir
    for subdir in players world mods backups; do
        ensure_dir "$DATA_ROOT/$subdir" tes3mp tes3mp 0750
    done

    for subdir in cell world map recordstore custom; do
        ensure_dir "$DATA_ROOT/world/$subdir" tes3mp tes3mp 0750
    done
    for subdir in plugins scripts; do
        ensure_dir "$DATA_ROOT/mods/$subdir" tes3mp tes3mp 0750
    done
    for subdir in mods state; do
        ensure_dir "$DATA_ROOT/backups/$subdir" tes3mp tes3mp 0750
    done

    ensure_dir "$DATA_ROOT/panel-data" tes3mp-panel tes3mp 0750
    ensure_dir "$DATA_ROOT/panel-data/tls" tes3mp-panel tes3mp 0700
    ensure_dir "$DATA_ROOT/panel-data/acme" tes3mp-panel tes3mp 0700
    ensure_dir "$DATA_ROOT/panel-data/uploads" tes3mp-panel tes3mp 2770
    ensure_dir "$DATA_ROOT/.locks" tes3mp tes3mp 0700
    ensure_dir "$DATA_ROOT/.staging" tes3mp tes3mp 0700

    if [[ ! -f $ROOTCTL_SOURCE ]]; then
        abort_install "rootctl source not found: $ROOTCTL_SOURCE"
    fi
    ensure_dir "$LIBEXEC_DIR" root root 0755
    # sudo checks that the invoking user can execute the command before it
    # elevates. root:tes3mp 0750 keeps the helper root-owned while allowing the
    # only account with a sudoers rule to execute it.
    run_install install -m 0750 -o root -g tes3mp "$ROOTCTL_SOURCE" "$ROOTCTL_DEST"
    pass "rootctl: $ROOTCTL_DEST (root:tes3mp 0750)"

    if [[ ! -f $SUDOERS_SOURCE ]]; then
        abort_install "sudoers source not found: $SUDOERS_SOURCE"
    fi
    if ! command -v visudo >/dev/null 2>&1; then
        abort_install "visudo is required to validate sudoers"
    fi
    ensure_dir "$SUDOERS_DIR" root root 0750

    # Validate before replacing the live file so a syntax error cannot lock out
    # the panel or the runtime account.
    if ! visudo -cf "$SUDOERS_SOURCE" >/dev/null; then
        abort_install "visudo rejected $SUDOERS_SOURCE"
    fi
    run_install install -m 0440 -o root -g root "$SUDOERS_SOURCE" "$SUDOERS_DEST"
    pass "sudoers: $SUDOERS_DEST"
}

# verify_unit_source UNIT
#
# This stage runs before the TES3MP runtime and the optional ops/panel binaries
# are installed. systemd-analyze treats an absent ExecStart target as a
# verification failure, so before the target exists only that exact diagnostic
# is accepted. Syntax errors, unknown directives and dependency problems still
# abort the install.
verify_unit_source() {
    local unit=$1
    local unit_name=${unit##*/}
    local expected_exec=""
    local output line
    local saw_pending=0
    local unexpected=0

    case "$unit_name" in
        tes3mp.service) expected_exec=/opt/tes3mp-easy/tes3mp/tes3mp-server ;;
        tes3mp-panel.service) expected_exec=/opt/tes3mp-easy/bin/tes3mp-easy-panel ;;
        tes3mp-easy-backup.service) expected_exec=/opt/tes3mp-easy/bin/tes3mp-easy-ops ;;
    esac

    if output=$(systemd-analyze verify --man=no "$unit" 2>&1); then
        pass "unit syntax: $unit_name"
        return
    fi

    while IFS= read -r line; do
        [[ -z $line ]] && continue
        if [[ -n $expected_exec &&
            $line == *": Command $expected_exec is not executable: No such file or directory" ]]; then
            # Expected until the matching binary is installed.
            # A typo in ExecStart, a wrong binary for this unit or any other
            # diagnostic still fails verification.
            saw_pending=1
            continue
        fi
        printf '%s\n' "$line" >&2
        unexpected=1
    done <<<"$output"

    if ((unexpected || saw_pending == 0)); then
        abort_install "systemd-analyze rejected $unit"
    fi
    pass "unit syntax: $unit_name (runtime executable pending)"
}

install_systemd_layout() {
    local unit src

    if ! command -v systemd-analyze >/dev/null 2>&1; then
        abort_install "systemd-analyze is required to validate systemd units"
    fi
    if ! command -v sysctl >/dev/null 2>&1; then
        abort_install "sysctl is required to apply vm.swappiness"
    fi

    for unit in "${UNIT_FILES[@]}"; do
        src="$SYSTEMD_SOURCE_DIR/$unit"
        if [[ ! -f $src ]]; then
            abort_install "systemd unit source not found: $src"
        fi
    done
    if [[ ! -f $JOURNALD_SOURCE ]]; then
        abort_install "journald source not found: $JOURNALD_SOURCE"
    fi
    if [[ ! -f $SYSCTL_SOURCE ]]; then
        abort_install "sysctl source not found: $SYSCTL_SOURCE"
    fi

    # Validate every unit before writing any of them. This keeps a typo from
    # leaving the host with a half-updated unit set.
    for unit in "${UNIT_FILES[@]}"; do
        verify_unit_source "$SYSTEMD_SOURCE_DIR/$unit"
    done

    ensure_dir "$SYSTEMD_UNIT_DIR" root root 0755
    for unit in "${UNIT_FILES[@]}"; do
        src="$SYSTEMD_SOURCE_DIR/$unit"
        run_install install -m 0644 -o root -g root "$src" "$SYSTEMD_UNIT_DIR/$unit"
        pass "unit: $SYSTEMD_UNIT_DIR/$unit"
    done

    ensure_dir "$JOURNALD_CONFIG_DIR" root root 0755
    run_install install -m 0644 -o root -g root \
        "$JOURNALD_SOURCE" "$JOURNALD_CONFIG_DEST"
    pass "journald: $JOURNALD_CONFIG_DEST (SystemMaxUse=100M, RuntimeMaxUse=32M)"

    ensure_dir "$SYSCTL_CONFIG_DIR" root root 0755
    run_install install -m 0644 -o root -g root \
        "$SYSCTL_SOURCE" "$SYSCTL_CONFIG_DEST"
    pass "sysctl: $SYSCTL_CONFIG_DEST (vm.swappiness=10)"

    run_install systemctl --no-ask-password daemon-reload
    pass "systemd: daemon reloaded"

    # journald reads drop-ins at process start; restart it now so the caps are
    # active before the first service start.
    run_install systemctl --no-ask-password restart systemd-journald.service
    pass "journald: restarted with the TES3MP Easy limits"

    run_install sysctl -q -p "$SYSCTL_CONFIG_DEST"
    pass "sysctl: applied $SYSCTL_CONFIG_DEST"
}

install_server_binaries() {
    if [[ ! -d $BIN_SOURCE_DIR ]]; then
        info "binaries: $BIN_SOURCE_DIR not present; source installs must install ops and the panel manually"
        return
    fi

    local name src dest staged
    for name in tes3mp-easy-ops tes3mp-easy-panel; do
        src="$BIN_SOURCE_DIR/$name"
        if [[ ! -f $src || -L $src ]]; then
            abort_install "release binary not found or not a regular file: $src"
        fi
    done

    ensure_dir "$OPT_ROOT/bin" root root 0755
    for name in tes3mp-easy-ops tes3mp-easy-panel; do
        src="$BIN_SOURCE_DIR/$name"
        dest="$OPT_ROOT/bin/$name"
        staged="$dest.new.$$"
        rm -f -- "$staged"
        run_install install -m 0755 -o root -g root "$src" "$staged"
        run_install mv -f -T -- "$staged" "$dest"
        pass "binary: $dest"
    done
}

valid_ipv4() {
    local ip=$1
    local octet
    [[ $ip =~ ^[0-9]{1,3}(\.[0-9]{1,3}){3}$ ]] || return 1
    local IFS=.
    for octet in $ip; do
        [[ $octet =~ ^[0-9]+$ ]] || return 1
        # net.ParseIP rejects leading zeroes, so accept only their canonical form.
        [[ $octet == "0" || $octet != 0* ]] || return 1
        ((10#$octet <= 255)) || return 1
    done
    return 0
}

# public_ipv4 rejects addresses that are not publicly routable. The panel
# repeats these checks, but failing here avoids a slow health timeout.
public_ipv4() {
    local ip=$1
    valid_ipv4 "$ip" || return 1
    local IFS=.
    local octets=()
    read -r -a octets <<<"$ip"
    local a=${octets[0]} b=${octets[1]}
    ((a == 0 || a == 10 || a == 127)) && return 1
    ((a == 100 && b >= 64 && b <= 127)) && return 1
    ((a == 169 && b == 254)) && return 1
    ((a == 172 && b >= 16 && b <= 31)) && return 1
    ((a == 192 && b == 168)) && return 1
    ((a >= 224)) && return 1
    return 0
}

detect_public_ipv4() {
    if [[ -n $PUBLIC_ADDRESS ]]; then
        printf '%s\n' "$PUBLIC_ADDRESS"
        return 0
    fi

    if ! command -v curl >/dev/null 2>&1; then
        return 1
    fi
    local url candidate
    for url in https://api.ipify.org https://ifconfig.me/ip; do
        candidate=$(curl -4 -fsS --max-time 10 "$url" 2>/dev/null || true)
        candidate=${candidate//$'\r'/}
        candidate=${candidate%%$'\n'*}
        candidate=$(printf '%s' "$candidate" | tr -d '[:space:]')
        if public_ipv4 "$candidate"; then
            printf '%s\n' "$candidate"
            return 0
        fi
    done
    return 1
}

port_80_is_free() {
    if ! command -v ss >/dev/null 2>&1; then
        abort_install "ss (iproute2) is required to verify that port 80 is free"
    fi

    local listeners
    if ! listeners=$(ss -H -ltn 2>/dev/null); then
        abort_install "cannot inspect listening TCP ports with ss"
    fi
    if [[ $listeners =~ (^|[[:space:]])[^[:space:]]*:80([[:space:]]|$) ]]; then
        return 1
    fi
    return 0
}

# clear_panel_failure_state removes a previous start-limit hit so an operator
# can retry the panel immediately after fixing port 80, the firewall, the public
# IP or a bad certificate. Only call reset-failed for a unit that is actually
# failed: systemd 252 returns an error for an inactive unit that was never
# started, and that must not abort a first install.
clear_panel_failure_state() {
    if systemctl --no-ask-password is-failed --quiet tes3mp-panel.service; then
        run_install systemctl --no-ask-password reset-failed tes3mp-panel.service
        pass "service: tes3mp-panel.service failed state cleared"
    else
        pass "service: tes3mp-panel.service has no failed state to clear"
    fi
}

configure_panel_tls() {
    if [[ ! -x "$OPT_ROOT/bin/tes3mp-easy-panel" ]]; then
        info "panel TLS: skipped; the panel binary is not installed"
        return
    fi

    if [[ $TLS_MODE == files ]]; then
        configure_panel_tls_files
        return
    fi

    local address
    if ! address=$(detect_public_ipv4); then
        abort_install "cannot determine the public IPv4; rerun with --public-address IP"
    fi
    if ! valid_ipv4 "$address"; then
        abort_install "public address is not a valid IPv4 address: $address"
    fi
    if ! public_ipv4 "$address"; then
        abort_install "public address is not a public IPv4 address: $address"
    fi
    PUBLIC_ADDRESS=$address

    # On upgrade an old panel process can still own :80. Stop the unit before
    # the challenge check; restart in start_runtime_services brings the new
    # binary up with the ACME settings.
    clear_panel_failure_state
    run_install systemctl --no-ask-password stop tes3mp-panel.service
    pass "service: tes3mp-panel.service stopped before the ACME port check"

    if ! port_80_is_free; then
        abort_install "TCP port 80 is already in use; ACME HTTP-01 needs it"
    fi
    pass "network: TCP port 80 is free for ACME HTTP-01"

    ensure_dir "$PANEL_ENV_DIR" root tes3mp-panel 0750
    # A previous --tls-mode files run left its pair in /etc/tes3mp-easy. The
    # acme panel reads panel-data/tls, so drop the unused copy: otherwise a
    # later manual switch to files silently serves a forgotten certificate.
    if [[ -e $PANEL_CERT_DEST || -e $PANEL_KEY_DEST ]]; then
        run_install rm -f -- "$PANEL_CERT_DEST" "$PANEL_KEY_DEST"
        pass "panel TLS: removed the files-mode key pair from $PANEL_ENV_DIR"
    fi
    local directory="" temp
    if [[ $TLS_MODE == staging ]]; then
        directory=$ACME_STAGING_DIRECTORY
    fi
    if ! temp=$(mktemp); then
        abort_install "cannot create a temporary panel environment file"
    fi
    if ! {
        printf '%s\n' '# Managed by bootstrap.sh; rerunning bootstrap overwrites this file.'
        # The panel implements acme and files; staging only changes the CA
        # directory, so the panel keeps the acme mode.
        printf 'TES3MP_PANEL_TLS_MODE=acme\n'
        printf 'TES3MP_PANEL_PUBLIC_ADDRESS=%s\n' "$PUBLIC_ADDRESS"
        # Read-only for the panel: the admin header shows the installed TES3MP
        # release from .tes3mp-install-info, so a custom prefix keeps working.
        printf 'TES3MP_PANEL_RUNTIME_ROOT=%s\n' "$TES3MP_RUNTIME_DIR"
        if [[ -n $directory ]]; then
            printf 'TES3MP_PANEL_ACME_DIRECTORY=%s\n' "$directory"
        fi
    } >"$temp"; then
        rm -f -- "$temp"
        abort_install "cannot write the temporary panel environment file"
    fi
    run_install install -m 0640 -o root -g tes3mp-panel "$temp" "$PANEL_ENV_DEST"
    rm -f -- "$temp"
    pass "panel TLS: $TLS_MODE for $PUBLIC_ADDRESS ($PANEL_ENV_DEST)"
    if [[ $TLS_MODE == staging ]]; then
        warn "panel TLS: staging certificates are untrusted; browsers warn until --tls-mode acme runs"
    fi
}

# configure_panel_tls_files installs an operator-provided key pair. It makes no
# CA requests, so it neither detects a public IPv4 nor checks port 80: the panel
# only reads the files. --public-address stays optional and is written only when
# the operator supplied it, because a NAT host may have no public address at all.
configure_panel_tls_files() {
    local cert=$TLS_CERT_SOURCE key=$TLS_KEY_SOURCE
    if [[ ! -f $cert ]]; then
        abort_install "TLS certificate is not a file: $cert"
    fi
    if [[ ! -r $cert ]]; then
        abort_install "TLS certificate is not readable: $cert"
    fi
    if [[ ! -f $key ]]; then
        abort_install "TLS private key is not a file: $key"
    fi
    if [[ ! -r $key ]]; then
        abort_install "TLS private key is not readable: $key"
    fi
    if [[ -n $PUBLIC_ADDRESS ]]; then
        # files mode accepts a private IP or a DNS name, so this is a sanity
        # check only: a host or host:port, never a URL or a path.
        if [[ $PUBLIC_ADDRESS == *[[:space:]]* || $PUBLIC_ADDRESS == *"://"* ||
            $PUBLIC_ADDRESS == *[/?#]* ]]; then
            abort_install "public address must be a host or host:port without scheme or path: $PUBLIC_ADDRESS"
        fi
    fi

    # start_runtime_services restarts the panel and the new process loads the
    # new pair. Stop the unit first so no running process sees a truncated
    # certificate while install(1) rewrites it.
    clear_panel_failure_state
    run_install systemctl --no-ask-password stop tes3mp-panel.service
    pass "service: tes3mp-panel.service stopped before the TLS files were installed"

    ensure_dir "$PANEL_ENV_DIR" root tes3mp-panel 0750
    run_install install -m 0640 -o root -g tes3mp-panel "$cert" "$PANEL_CERT_DEST"
    pass "panel TLS: certificate installed ($PANEL_CERT_DEST)"
    run_install install -m 0640 -o root -g tes3mp-panel "$key" "$PANEL_KEY_DEST"
    pass "panel TLS: private key installed ($PANEL_KEY_DEST)"

    local temp
    if ! temp=$(mktemp); then
        abort_install "cannot create a temporary panel environment file"
    fi
    if ! {
        printf '%s\n' '# Managed by bootstrap.sh; rerunning bootstrap overwrites this file.'
        printf 'TES3MP_PANEL_TLS_MODE=files\n'
        printf 'TES3MP_PANEL_TLS_CERT=%s\n' "$PANEL_CERT_DEST"
        printf 'TES3MP_PANEL_TLS_KEY=%s\n' "$PANEL_KEY_DEST"
        printf 'TES3MP_PANEL_RUNTIME_ROOT=%s\n' "$TES3MP_RUNTIME_DIR"
        if [[ -n $PUBLIC_ADDRESS ]]; then
            printf 'TES3MP_PANEL_PUBLIC_ADDRESS=%s\n' "$PUBLIC_ADDRESS"
        fi
    } >"$temp"; then
        rm -f -- "$temp"
        abort_install "cannot write the temporary panel environment file"
    fi
    run_install install -m 0640 -o root -g tes3mp-panel "$temp" "$PANEL_ENV_DEST"
    rm -f -- "$temp"
    pass "panel TLS: files ($PANEL_ENV_DEST)"
}

panel_journal_tail() {
    if ! command -v journalctl >/dev/null 2>&1; then
        return 0
    fi
    journalctl --no-pager -u tes3mp-panel.service -n 40 -o cat 2>/dev/null || true
}

print_panel_journal_tail() {
    local journal=$1 line
    local -a lines=()
    while IFS= read -r line; do
        # The first start writes the initial admin password to journald. Keep
        # it out of installer output; the operator reads the file directly.
        if [[ $line == *"initial password"* ]]; then
            continue
        fi
        lines+=("$line")
    done <<<"$journal"

    local first=$((${#lines[@]} - 12))
    if ((first < 0)); then
        first=0
    fi
    local i
    for ((i = first; i < ${#lines[@]}; i++)); do
        printf '%s\n' "${lines[i]}" >&2
    done
}

abort_panel_startup() {
    local message=${1:-"tes3mp-panel.service is not active"}
    local journal retry=""
    journal=$(panel_journal_tail)
    if [[ -n $journal ]]; then
        print_panel_journal_tail "$journal"
    fi
    if [[ $journal == *"too many certificates"* ||
        $journal == *"too many failed authorizations"* ||
        $journal == *"rateLimited"* ]]; then
        # The journal can contain several attempts. The last line is the most
        # recent answer from Let's Encrypt, so report its retry time.
        local line
        while IFS= read -r line; do
            if [[ $line =~ retry\ after\ ([0-9]{4}-[0-9]{2}-[0-9]{2}\ [0-9]{2}:[0-9]{2}:[0-9]{2}\ UTC) ]]; then
                retry=${BASH_REMATCH[1]}
            fi
        done <<<"$journal"
        if [[ -n $retry ]]; then
            abort_install "tes3mp-panel.service cannot obtain a TLS certificate: Let's Encrypt rate limit for this public IP; retry after $retry, then rerun the installer (earlier attempts will fail)"
        fi
        abort_install "tes3mp-panel.service cannot obtain a TLS certificate: Let's Encrypt rate limit for this public IP; wait for the retry time in journalctl -u tes3mp-panel.service, then rerun the installer"
    fi
    abort_install "$message; inspect journalctl -u tes3mp-panel.service"
}

wait_for_panel_health() {
    local deadline=$((SECONDS + 300))
    while ((SECONDS < deadline)); do
        if curl -kfsS --noproxy '*' --max-time 2 https://127.0.0.1/healthz >/dev/null 2>&1; then
            pass "panel: HTTPS /healthz is ready"
            return
        fi
        if ! systemctl --no-ask-password is-active --quiet tes3mp-panel.service; then
            abort_panel_startup "tes3mp-panel.service stopped during startup"
        fi
        sleep 2
    done
    abort_panel_startup "panel did not become healthy in 300s"
}

panel_url() {
    if [[ -n $PUBLIC_ADDRESS ]]; then
        printf 'https://%s/' "$PUBLIC_ADDRESS"
    else
        # files mode may run on a host without a public address: do not print
        # the misleading https:/// .
        printf 'https://<server-address>/'
    fi
}

# report_initial_password prints the temporary panel password that the panel
# writes on its first start. The file disappears after the first password
# change, so a rerun of the installer only points at the recovery steps.
report_initial_password() {
    local file="$DATA_ROOT/panel-data/initial-password.txt"
    local password=""
    if [[ -s $file ]]; then
        password=$(cat "$file" 2>/dev/null) || password=""
    fi
    if [[ -z $password ]]; then
        info "panel: open $(panel_url)"
        if [[ -e $file ]]; then
            info "panel: the temporary password is empty; read $file"
        else
            info "panel: the temporary password is already gone, sign in with your own password"
            info "panel: if it is lost, reset it as described in docs/v2/08-runbook.md (reset-password)"
        fi
        return
    fi
    info "panel: open $(panel_url)"
    printf '%s[panel]%s temporary password: %s\n' \
        "$C_GREEN" "$C_RESET" "$password"
    info "panel: change it after the first login; the panel then deletes $file"
}

start_runtime_services() {
    # `enable` makes the game and timer survive a reboot; `start` is
    # deliberately not `restart`, so rerunning the installer never disconnects
    # players. The panel is restarted only when its binary is installed, which
    # both starts it after a release install and loads a replaced binary.
    run_install systemctl --no-ask-password enable tes3mp.service
    pass "service: tes3mp.service enabled for boot"

    run_install systemctl --no-ask-password start tes3mp.service
    if ! systemctl --no-ask-password is-active --quiet tes3mp.service; then
        abort_install "tes3mp.service is not active; inspect journalctl -u tes3mp.service"
    fi
    pass "service: tes3mp.service active"

    run_install systemctl --no-ask-password enable --now tes3mp-easy-backup.timer
    if ! systemctl --no-ask-password is-active --quiet tes3mp-easy-backup.timer; then
        abort_install "tes3mp-easy-backup.timer is not active"
    fi
    pass "timer: tes3mp-easy-backup.timer enabled and active"

    if [[ -x "$OPT_ROOT/bin/tes3mp-easy-panel" ]]; then
        run_install systemctl --no-ask-password enable tes3mp-panel.service
        pass "service: tes3mp-panel.service enabled for boot"
        # restart loads a replaced binary on upgrades and starts the panel on
        # the first release install. The game service keeps using start above,
        # so rerunning bootstrap does not disconnect players.
        run_install systemctl --no-ask-password restart tes3mp-panel.service
        if ! systemctl --no-ask-password is-active --quiet tes3mp-panel.service; then
            abort_panel_startup "tes3mp-panel.service is not active"
        fi
        wait_for_panel_health
        pass "service: tes3mp-panel.service active"
        report_initial_password
    else
        info "panel: tes3mp-panel.service remains inactive until /opt/tes3mp-easy/bin/tes3mp-easy-panel is installed"
    fi
}

main() {
    parse_args "$@"

    info "TES3MP Easy bootstrap preflight"
    info "data root: $DATA_ROOT"

    check_root
    check_os
    check_arch
    check_systemd
    check_ram
    check_swap
    check_disk

    print_summary
    if ((failures > 0)); then
        exit 1
    fi

    if [[ $CHECK_ONLY == true ]]; then
        info "Preflight complete (--check). No changes were made."
        return
    fi

    info "Installing the privilege model"
    install_privilege_layout
    install_swap
    info "Installing systemd units, journald limits and sysctl"
    install_systemd_layout
    info "Installing server binaries from $BIN_SOURCE_DIR"
    install_server_binaries
    info "Installing the TES3MP runtime and base data"
    load_tes3mp_release
    install_runtime_packages
    install_tes3mp_runtime
    install_base_configs
    ensure_base_required_data
    install_base_mods_archive
    wire_runtime_to_data_root
    install_runtime_home
    info "Runtime ready: TES3MP $TES3MP_VERSION, configs and base mods-init archive are in place."
    info "Configuring the panel TLS certificate"
    configure_panel_tls
    info "Starting TES3MP services"
    start_runtime_services
    info "Bootstrap complete: TES3MP $TES3MP_VERSION is running."
}

main "$@"

