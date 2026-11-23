#!/usr/bin/env bash
# stack_reaper.sh <stack pid> <log dir> <stack command line> — stops a stack's services once its
# stack script is gone without its cleanup having run: a SIGKILL, which no
# trap catches, or a closed session. stack_lib.sh's start launches it on the
# stack's first service, in its own process group (own_group.sh) and with
# stdin, stdout and stderr detached, so a kill of the stack's group spares it
# and a reader of the stack's output sees that output end with the stack.
# The stack's cleanup stops it, and waits for it, before it stops the
# services itself.
#
# It polls the stack script's pid once a second, with kill -0 and a check
# that the pid still runs the command line the stack read for it at launch
# (its own read could come after the stack had moved on), so a reused pid
# can't keep it waiting. Then it stops every group LOG DIR's services.pids
# names at that moment (make restart targets' relaunched and added services
# included, scripts/stack_service.sh) with cleanup's own stop
# (scripts/stack_groups.sh): TERM to each group, else to its pid, then a
# wait of about 10 s for the groups to exit, so its exit means they have.
# With no list it has nothing to stop.
#
#   node --test scripts/stack_lib.test.js   (make page-check)
set -uo pipefail
# One block, read whole before it runs: bash reads a script as it goes, and
# a reaper outlives edits to this file (scripts/mutant.sh's included). The
# source reads stack_groups.sh whole too, at launch.
{
    [ $# -eq 3 ] || { echo "usage: stack_reaper.sh <stack pid> <log dir> <stack command line>" >&2; exit 2; }
    . "$(dirname "${BASH_SOURCE[0]}")/stack_groups.sh"
    pid=$1 list="$2/services.pids" cmd=$3
    while kill -0 "$pid" 2>/dev/null && [ "$(ps -o command= -p "$pid" 2>/dev/null)" = "$cmd" ]; do sleep 1; done
    [ -f "$list" ] || exit 0
    pids=$(listed_pids "$list")
    # Unquoted: one argument a pid.
    stop_groups $pids
    exit 0
}
