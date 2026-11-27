#!/usr/bin/env bash
# Shared paths and ownership checks for make's detached stack state.

stack_state_dir() {
    local base=${TMPDIR:-/tmp}
    printf '%s/enygma-%s\n' "${base%/}" "$(id -u)"
}

stack_state_exists() { [ -e "$1" ] || [ -L "$1" ]; }
stack_file_is_owned() { [ -f "$1" ] && [ ! -L "$1" ] && [ -O "$1" ]; }

require_stack_file() {
    stack_file_is_owned "$1" && return 0
    echo "refusing stack state file $1: expected an owned regular file without symlinks" >&2
    return 1
}

check_stack_state_dir() {
    local mode
    if [ -d "$1" ] && [ ! -L "$1" ] && [ -O "$1" ]; then
        if [ "$(uname -s)" = Darwin ]; then mode=$(stat -f '%Lp' "$1")
        else mode=$(stat -c '%a' -- "$1")
        fi
        [ "$mode" = 700 ] && return 0
    fi
    echo "refusing stack state directory $1: expected an owned directory with mode 700" >&2
    return 1
}

ensure_stack_state_dir() {
    # Another start may create the directory at the same time.
    if ! stack_state_exists "$1"; then mkdir -m 700 -- "$1" 2>/dev/null || true; fi
    check_stack_state_dir "$1"
}

# Keep an owned PID from a stack launched before the directory move usable.
# New starts always write to the requested path.
stack_read_pid_path() {
    local requested=$1 dir legacy
    dir=$(stack_state_dir)
    if [ "${requested%/*}" = "$dir" ]; then
        if stack_state_exists "$dir"; then check_stack_state_dir "$dir" || return 1; fi
        legacy="${dir%/*}/${requested##*/}"
        if ! stack_state_exists "$requested" && stack_file_is_owned "$legacy"; then
            printf '%s\n' "$legacy"
            return 0
        fi
    fi
    printf '%s\n' "$requested"
}

stack_read_logs() {
    local pid_file
    pid_file=$(stack_read_pid_path "$1") || return 1
    if stack_state_exists "$pid_file.logs"; then
        require_stack_file "$pid_file.logs" || return 1
        cat "$pid_file.logs"
    fi
}

if [ "${BASH_SOURCE[0]}" = "$0" ]; then
    case "$1" in
        dir) stack_state_dir ;;
        logs) stack_read_logs "$2" ;;
        check) require_stack_file "$2" ;;
        *) echo "usage: stack_state.sh dir | logs PID_FILE | check FILE" >&2; exit 2 ;;
    esac
fi
