#!/usr/bin/env bash
# Stops the stack a make *-stack target started, for make's *-stack-stop
# (make dvp-stack-stop, make payments-stack-stop, make shield-stack-stop):
# signals only the recorded stack script, waits about 30 s for it to exit and
# its ports to free, and removes its pid files. It fails when the stack
# doesn't stop in time, or when its log is missing or reports a service dead.
# A pid file whose pid no longer runs the stack script is stale, left by a
# stack that exited by itself: it says so, removes it and signals nothing.
#
# Usage: stack_stop.sh <pid file> <command line> <stack log> <start target> <port>...
#   The pid file holds the stack script's pid, <pid file>.logs its log dir;
#   the command line is the stack script's whole one, as ps prints it: a
#   substring would let one stack's stop take another's; the stack log
#   is the script's own output, in that log dir; the start target names the
#   make target, for the messages when no such stack runs.
#
#   node --test scripts/stack_stop.test.js   (make page-check)
set -uo pipefail
pid_file=$1 cmdline=$2 stack_log=$3 target=$4
shift 4

pid=$(cat "$pid_file" 2>/dev/null)
log=$(cat "$pid_file.logs" 2>/dev/null)/$stack_log
[ -f "$pid_file" ] || { echo "no stack started by make $target"; exit 1; }
if [ "$(ps -o command= -p "$pid" 2>/dev/null)" != "$cmdline" ]; then
    rm -f "$pid_file" "$pid_file.logs"
    echo "stale pid file $pid_file: pid $pid no longer runs the stack make $target started, which has exited (log $log); removed it and signalled nothing"
    exit 1
fi
kill -TERM "$pid"
for _ in $(seq 1 150); do
    busy=
    # stack_lib.sh's listening(), copied: sourcing the lib would install its traps.
    for p in "$@"; do (exec 3<>"/dev/tcp/127.0.0.1/$p") 2>/dev/null && busy="$busy :$p"; done
    kill -0 "$pid" 2>/dev/null && busy="$busy (pid $pid)"
    [ -z "$busy" ] || { sleep 0.2; continue; }
    rm -f "$pid_file" "$pid_file.logs"
    [ -f "$log" ] || { echo "stack stopped, but with no stack log at $log to show it stopped cleanly"; exit 1; }
    # stack_lib.sh's died() prints this line. After the signal it means the
    # stack script took its own stop for a death, as before #301's trap fix.
    if grep -qF -- 'exited — last log lines' "$log"; then
        echo "stack stopped, but it reported a service dead on the way (log $log):"
        sed -n '/exited — last log lines/,$p' "$log"
        exit 1
    fi
    echo "stack stopped"
    exit 0
done
echo "still running:$busy after about 30 s"
exit 1
