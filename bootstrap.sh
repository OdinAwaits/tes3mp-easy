#!/usr/bin/env bash
#
# bootstrap.sh — TES3MP Easy v2 bootstrap.
#
# I01 implements the read-only preflight. Installation stages (users, units,
# runtime, initial backups) are added by I02-I05. Running this script now only
# verifies the host and never changes it.
#
# Usage: sudo ./bootstrap.sh [--check] [--root PATH]
#
set -Eeuo pipefail
export LC_ALL=C

readonly DEFAULT_ROOT="/tes3mp-easy"

# A nominal 512 MiB VPS reports about 480-500 MiB MemTotal after firmware and
# kernel reservations; a nominal 7 GiB filesystem reports about 6.3 GiB total.
# A fresh 7 GiB host with a 512 MiB swap file can have about 4.5-5 GiB free, so
# the hard floor is 4.5 GiB and 5 GiB is only a warning. These thresholds still
# reject clearly undersized hosts.
readonly MIN_RAM_MIB=480
readonly RECOMMENDED_RAM_MIB=640
readonly MIN_SWAP_MIB=480
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
CHECK_ONLY=false

passes=0
warnings=0
failures=0

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

TES3MP Easy v2 bootstrap preflight. The current stage is read-only and can be
run repeatedly.

Options:
  --check        run only the preflight (currently the default)
  --root PATH    data root to validate (default $DEFAULT_ROOT)
  -h, --help     show this help and exit

Checks:
  root privileges, Debian 12+/Ubuntu 24.04+, x86_64, systemd,
  RAM, swap and free disk.

Exit status:
  0  all checks passed (warnings are allowed)
  1  one or more preflight checks failed
  2  invalid command line
EOF
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
    if ((swap_mib < MIN_SWAP_MIB)); then
        fail "swap: $swap_mib MiB is below the ${MIN_SWAP_MIB} MiB minimum"
        return
    fi

    pass "swap: $swap_mib MiB total"
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
    else
        info "Preflight complete. Installation stages are not implemented yet (I02-I05)."
    fi
}

main "$@"

