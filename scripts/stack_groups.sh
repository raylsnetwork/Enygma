# How a stack's services are stopped, shared by scripts/stack_lib.sh's
# cleanup and scripts/stack_reaper.sh, which source it: the one shell reader
# of a services.pids list, and the stop of the groups it names. Each service
# leads its own process group (own_group.sh), so a kill of its group takes
# what it started (npx → node) too. Written for bash 3.2 (macOS's) and 5,
# with or without set -e.
#
#   node --test scripts/stack_lib.test.js   (make page-check)

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
