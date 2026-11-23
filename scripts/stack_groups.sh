# How a stack's services are listed and stopped, shared by scripts/stack_lib.sh,
# scripts/stack_service.sh and scripts/stack_reaper.sh, which source it: the
# lock every writer of a services.pids list takes, the one shell reader of
# the list, and the stop of the groups it names. Each service
# leads its own process group (own_group.sh), so a kill of its group takes
# what it started (npx → node) too. Written for bash 3.2 (macOS's) and 5,
# with or without set -e.
#
#   node --test scripts/stack_lib.test.js   (make page-check)

# lock_list LIST — takes LIST's lock, a LIST.lock file, for a write to LIST:
# stack_lib.sh's start appends while a make restart target's
# stack_service.sh may be rewriting it (#404), and two restarts may record
# at once. Returns 1, saying so, after about 10 s without it. The lock is
# bash's own exclusive create (set -C: O_CREAT|O_EXCL), not mkdir's exit
# status: uutils mkdir 0.10, ubuntu-26.04's, can exit 0 for a dir another
# process just made, letting two writers in at once (#398). unlock_list
# LIST drops it.
lock_list() {
    local lock="$1.lock"
    for _ in $(seq 1 100); do (set -C; : >"$lock") 2>/dev/null && return 0; sleep 0.1; done
    echo "$(basename "$0"): $lock held for about 10 s: another write to the list is under way, or one died holding it (rm it)" >&2
    return 1
}
unlock_list() { rm -f "$1.lock"; }

# listed_pids FILE — the pids FILE's `<name> <pid>` lines name, one a line,
# skipping any that isn't an integer above 1, as scripts/test_stack.js's
# reader does: kill -- -0 signals the caller's own group, and kill -- -1
# every process the user owns.
listed_pids() { awk '$2 ~ /^[0-9]+$/ && $2 > 1 { print $2 }' "$1"; }

# signal_groups PID… — TERM to each PID's group, else (a PID that leads no
# group) to PID itself.
signal_groups() {
    local pid
    for pid in "$@"; do
        kill -TERM -- "-$pid" 2>/dev/null || kill -TERM "$pid" 2>/dev/null || true
    done
}

# await_groups PID… — waits, about 10 s at most, until none of the PIDs'
# groups runs: all of them under one deadline.
await_groups() {
    local pid alive
    for _ in $(seq 1 50); do
        alive=
        for pid in "$@"; do kill -0 -- "-$pid" 2>/dev/null && alive=1; done
        [ -n "$alive" ] || break
        sleep 0.2
    done
}

# stop_groups PID… — signal_groups, then waits for the groups: wait reaps
# those that are this shell's children, and await_groups polls the rest (a
# service a restart target relaunched is no child of the stack script's, and
# none is the reaper's).
stop_groups() {
    signal_groups "$@"
    wait 2>/dev/null || true
    await_groups "$@"
}
