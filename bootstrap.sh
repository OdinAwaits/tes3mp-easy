#!/usr/bin/env bash
#
# bootstrap.sh — TES3MP Easy v2 bootstrap.
#
# I01 implements the read-only preflight. I02 adds the privilege model: system
# users and groups, the data directory layout, sudoers and the fixed rootctl
# helper. I03 installs the systemd units, the backup timer, journald limits and
# the sysctl drop-in. Runtime, initial data and the first service start remain
# in I04-I05.
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

TES3MP Easy v2 bootstrap. The current stage installs the privilege model and
can be run repeatedly: existing users, groups and directories are kept.

Options:
  --check        run only the preflight and change nothing
  --root PATH    data root to validate and create (default $DEFAULT_ROOT)
  -h, --help     show this help and exit

Checks:
  root privileges, Debian 12+/Ubuntu 24.04+, x86_64, systemd,
  RAM, swap and free disk.

Installs (I02):
  users and groups tes3mp/tes3mp-panel, the data directory layout,
  /usr/local/libexec/tes3mp-easy-rootctl and /etc/sudoers.d/tes3mp-easy.

Installs (I03):
  tes3mp.service, tes3mp-panel.service, tes3mp-easy-backup.service,
  tes3mp-easy-backup.timer, the journald drop-in and vm.swappiness=10.
  The TES3MP runtime and the first service start are installed by I04-I05.

Exit status:
  0  all checks and installation steps passed (warnings are allowed)
  1  a preflight check or an installation step failed
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
    if [[ $DATA_ROOT == "/" ]]; then
        printf 'bootstrap: --root must not be /\n' >&2
        exit 2
    fi
    while [[ $DATA_ROOT == */ ]]; do
        DATA_ROOT=${DATA_ROOT%/}
    done
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
# The runtime binaries arrive in I04-I05, after this stage. systemd-analyze
# treats an absent ExecStart target as a verification failure, so before the
# runtime exists only that exact diagnostic is accepted. Syntax errors,
# unknown directives and dependency problems still abort the install.
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
            # Expected until the matching runtime binary is installed in
            # I04-I05. A typo in ExecStart, a wrong binary for this unit or
            # any other diagnostic still fails verification.
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
    # active before the first service start in I05.
    run_install systemctl --no-ask-password restart systemd-journald.service
    pass "journald: restarted with the TES3MP Easy limits"

    run_install sysctl -q -p "$SYSCTL_CONFIG_DEST"
    pass "sysctl: applied $SYSCTL_CONFIG_DEST"
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

    info "Installing the I02 privilege model"
    install_privilege_layout
    info "Installing the I03 systemd units, journald limits and sysctl"
    install_systemd_layout
    info "Bootstrap stage I03 complete: users, directories, rootctl, sudoers, units, journald and sysctl are in place."
}

main "$@"

