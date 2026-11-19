#!/usr/bin/env bash
# own_group.sh <command…> — runs the command in a new session and process
# group, which it leads, so its caller can kill the command and everything it
# starts with one kill -- -<pid>. The stacks start their services through it
# (scripts/stack_lib.sh's start, scripts/stack_start.sh), as do make's demo
# restarts and scripts/mutant.sh's test runs.
#
# It execs perl, and perl execs the command (perl's setsid: stock macOS has
# no setsid(1)), so the command keeps the pid the caller started: the pid a
# caller records ($!) is the command's own, and its process group's. Never
# run perl as a child here: a stop that kills the recorded group, or finds
# the stack by its command line (scripts/stack_stop.sh), would miss it.
#
# The command runs as a shell would run it, never through one (exec
# { $ARGV[0] }: a command passed as one word is not split by /bin/sh), and
# own_group.sh exits 127 when it is not found, 126 when it can't run, with a
# line on stderr naming it. setsid fails only in a process that already
# leads its group (a job an interactive shell started), and the command
# then leads that group.
#
#   scripts/own_group.sh npx hardhat node &
#
#   node --test scripts/own_group.test.js   (make page-check)
set -euo pipefail
[ $# -gt 0 ] || { echo "usage: own_group.sh <command…>" >&2; exit 2; }
exec perl -e 'use POSIX qw(setsid); setsid(); exec { $ARGV[0] } @ARGV; print STDERR "own_group.sh: $ARGV[0]: $!\n"; exit($!{ENOENT} ? 127 : 126)' -- "$@"
