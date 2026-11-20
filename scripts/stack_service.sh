#!/usr/bin/env bash
# stack_service.sh <log dir> <name> <pid> — records that service NAME of the
# stack logging to LOG DIR now runs as PID, which leads its own process group
# (own_group.sh started it: the caller's $!), so the stack's stop stops it.
# make shield-demo-restart and shield-gnark-restart relaunch a service under
# a running run.sh stack, or start the demo on one that has none. It replaces
# NAME's line in LOG DIR's services.pids, the list stack_lib.sh's start
# writes and its cleanup kills at stop time, or adds one. Two restarts may
# record at once: each takes the list's lock (a services.pids.lock file)
# for its read and rewrite, and fails after about 10 s without it. The lock
# is bash's own exclusive create (set -C: O_CREAT|O_EXCL), not mkdir's exit
# status: uutils mkdir 0.10, ubuntu-26.04's, can exit 0 for a dir another
# process just made, letting two records in at once (#398).
#
#   (exec scripts/own_group.sh <cmd…> >>"$logs/demo.log" 2>&1) &
#   scripts/stack_service.sh "$logs" demo $!
#
#   node --test scripts/stack_lib.test.js   (make page-check)
set -euo pipefail
[ $# -eq 3 ] || { echo "usage: stack_service.sh <log dir> <name> <pid>" >&2; exit 2; }
list="$1/services.pids" name=$2 pid=$3
lock="$list.lock" locked=
for _ in $(seq 1 100); do (set -C; : >"$lock") 2>/dev/null && { locked=1; break; }; sleep 0.1; done
[ -n "$locked" ] || { echo "stack_service.sh: $lock held for about 10 s: another restart is recording, or one died holding it (rm -r it)" >&2; exit 1; }
trap 'rm "$lock"' EXIT
tmp=$(mktemp "$list.XXXXXX")
{ [ ! -f "$list" ] || awk -v n="$name" '$1 != n' "$list"; echo "$name $pid"; } >"$tmp"
mv "$tmp" "$list"
