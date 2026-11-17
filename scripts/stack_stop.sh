#!/usr/bin/env bash
# Stops the stack a make *-stack target started, for make's *-stack-stop
# (make dvp-stack-stop, make payments-stack-stop, make shield-stack-stop):
# signals only the recorded stack script, waits about 30 s for it to exit and
# its ports to free, and removes its pid files. It fails when the stack
# doesn't stop in time, or when its log is missing or reports a service dead.
#
# Usage: stack_stop.sh <pid file> <process match> <stack log> <start target> <port>...
#   The pid file holds the stack script's pid, <pid file>.logs its log dir;
#   the process match is a fixed string its command line holds; the stack log
#   is the script's own output, in that log dir; the start target names the
#   make target, for the message when no such stack runs.
#
#   node --test scripts/stack_stop.test.js   (make page-check)
set -uo pipefail
pid_file=$1 match=$2 stack_log=$3 target=$4
shift 4

pid=$(cat "$pid_file" 2>/dev/null)
log=$(cat "$pid_file.logs" 2>/dev/null)/$stack_log
ps -o command= -p "$pid" 2>/dev/null | grep -qF -- "$match" || { echo "no stack started by make $target"; exit 1; }
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
