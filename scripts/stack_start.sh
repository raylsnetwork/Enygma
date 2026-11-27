#!/usr/bin/env bash
# Starts a stack script detached, for make's *-stack targets (make dvp-stack,
# make payments-stack, make shield-stack), and returns once its log says
# `stack is up`, or with its log when it exits first. A stack script stays in
# the foreground for the stack's whole life, so anything waiting on it (a
# pipe, the next command) would wait forever.
#
# It records the stack script's pid in <pid file>, and its log dir in
# <pid file>.logs, as soon as it has the pid, before the stack is up: a
# stack that is still starting already reads its scripts and helpers, so
# scripts/mutant.sh's guard must see it, and make's *-stack-stop
# (scripts/stack_stop.sh) must be able to stop it. A stack script that exits
# before it comes up takes its pid files with it, so none is left stale.
#
# Usage: stack_start.sh <pid file> <log dir> <log prefix> <stack log> <run dir> <command line> <start target> <test target>
#   The log dir may be empty, for a fresh temp dir named <log prefix>.*; the
#   stack log is the script's own output, in that dir; the command line is
#   the stack script's whole one, run in <run dir>, which stack_stop.sh
#   matches; the start and test targets name make's targets, for the message
#   once the stack is up.
#
#   node --test scripts/stack_start.test.js   (make page-check)
set -uo pipefail
pid_file=$1 logs=$2 prefix=$3 stack_log=$4 run_dir=$5 cmdline=$6 target=$7 test_target=$8
# Absolute: the stack script starts in <run dir>.
script_dir=$(cd "$(dirname "$0")" && pwd)
own_group=$script_dir/own_group.sh
. "$script_dir/stack_state.sh"

# A second start would overwrite the running stack's pid file, then fail on
# its busy ports and remove it, leaving that stack with none.
existing=$(stack_read_pid_path "$pid_file") || exit 1
if stack_state_exists "$existing"; then
    require_stack_file "$existing" || exit 1
    old=$(cat "$existing") || exit 1
    if kill -0 "$old" 2>/dev/null; then
        echo "the stack make $target started (pid $old) still runs: stop it first (make $target-stop)"
        exit 1
    fi
fi
state_dir=$(stack_state_dir)
logs_parent=${TMPDIR:-/tmp}
if [ "${pid_file%/*}" = "$state_dir" ]; then
    ensure_stack_state_dir "$state_dir" || exit 1
    logs_parent=$state_dir
fi
if stack_state_exists "$pid_file.logs"; then require_stack_file "$pid_file.logs" || exit 1; fi
if [ -z "$logs" ]; then logs=$(mktemp -d "$logs_parent/$prefix.XXXXXX") || exit 1; fi
mkdir -p "$logs"
logs=$(cd "$logs" && pwd)
log=$logs/$stack_log
: >"$log"
# The command line is split into words, as make's *_STACK_CMD writes it.
# shellcheck disable=SC2086
(cd "$run_dir" && LOG_DIR=$logs exec "$own_group" $cmdline >"$log" 2>&1) &
pid=$!
# Until own_group.sh has exec'd the stack script, ps shows another command line,
# which stack_stop.sh would take for a stale pid file (about 10 s at most).
for _ in $(seq 1 200); do
    kill -0 "$pid" 2>/dev/null && [ "$(ps -o command= -p "$pid" 2>/dev/null)" != "$cmdline" ] || break
    sleep 0.05
done
echo "$pid" >"$pid_file"
echo "$logs" >"$pid_file.logs"
while kill -0 "$pid" 2>/dev/null; do
    grep -q 'stack is up' "$log" && { echo "stack is up (logs $logs); run make $test_target, then make $target-stop"; exit 0; }
    sleep 1
done
rm -f "$pid_file" "$pid_file.logs"
echo "$cmdline exited before the stack came up:"
cat "$log"
exit 1
