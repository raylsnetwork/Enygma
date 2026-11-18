#!/usr/bin/env bash
# Show a check red: run the test command once as the code stands, which
# must pass; then replace old with new in file (old must occur exactly
# once), run it again, restore the file whatever happens, and say whether
# the command went red. Run the whole test file rather than a name filter,
# so a renamed test can't drop out of the run unnoticed. With --expect, the
# red counts only if the output matches regex, naming the check under test
# (a subtest's name, say), so a red from an earlier check failing first
# doesn't pass for the new check's. regex matches one line of output: go
# test prints a test's name and its assertion on separate lines, so name
# one or the other.
# It refuses a file the stacks are wired from (suite_changes.sh --wiring)
# while a stack make started runs, up or still starting: a stack reads its
# scripts and helpers as it starts and runs (bash reads a script as it goes),
# so a mutant applied then could run inside the live stack and pass or fail
# it for the wrong reason.
# Each run of the command has a deadline, --timeout seconds (default 600):
# it runs in its own process group, as make's *-stack targets run a stack,
# and a run past the deadline is killed with everything it started, the
# file is restored and the verdict is HUNG. A node test awaiting a child
# that never exits fails on its own timeout, but node lives on with the
# child. A hang is never RED: a hung test is a test bug.
# Exit 0 when the mutant is killed (red), 1 when it survives, goes red
# elsewhere or hangs, 2 on bad input: a wiring file of a live stack, a
# command already red without the mutant or hung without it, or one that
# did not run (exit 126 or 127, as a command passed as one word).
# A mutant of mutant.sh itself (the same file, symlinks resolved) runs
# from a temporary copy of mutant.sh and suite_changes.sh, with the same
# arguments: bash reads a script as it goes, so the run would otherwise pick
# up the mutated text partway through. The copy mutates and restores the
# real file as usual and is removed on exit.
# The verdict (RED:, SURVIVED:, HUNG: or BAD INPUT:) is always the last
# line, after the failure lines it rests on, so `| tail` keeps it.
#
#   scripts/mutant.sh enygma_retail_payments/shield_demo/index.html \
#     "hold * 1.3)" "hold * 3)" -- node --test scripts/shield_demo_replay.test.js
#   scripts/mutant.sh --expect 'TestProtocolPrimer/audit' <file> <old> <new> -- go test …
#   scripts/mutant.sh <file> <old> <new> -- go -C enygma_retail_payments/shield_demo test ./engine/
# Go runs inside its module (go -C <module dir>): from the repo root it
# finds no go.mod.
set -euo pipefail
usage() { echo "usage: mutant.sh [--expect <regex>] [--timeout <seconds>] <file> <old> <new> -- <test command…>" >&2; exit 2; }
args=("$@")
expect= timeout=600
while :; do
  case ${1:-} in
    --expect) [ $# -ge 2 ] || usage; expect=$2; shift 2 ;;
    --timeout)
      [ $# -ge 2 ] && [[ $2 =~ ^[1-9][0-9]*$ ]] || usage
      timeout=$2; shift 2 ;;
    *) break ;;
  esac
done
[ $# -ge 5 ] && [ "$4" = -- ] || usage
file=$1 old=$2 new=$3
shift 4

# cleanup runs on exit (not across the exec below): it kills the run in
# progress, if any (pgid, its process group), so no test runs on against
# the restored file; restores the file once it is backed up; and removes
# the copy mutant.sh runs from, if it runs from one.
pgid= backup= out=
cleanup() {
  [ -z "$pgid" ] || kill -KILL -- "-$pgid" 2>/dev/null
  [ -z "$backup" ] || cp "$backup" "$file"
  rm -f "$backup" "$out"
  [ -z "$copy_dir" ] || rm -rf -- "$copy_dir"
}
# The copy is told the real repo root and its own dir to remove, and
# unsets both, so no mutant.sh the test command runs inherits them.
root=${MUTANT_SH_ROOT:-$(cd "$(dirname "$0")/.." && pwd -P)}
copy_dir=${MUTANT_SH_COPY_DIR:-}
unset MUTANT_SH_ROOT MUTANT_SH_COPY_DIR
trap cleanup EXIT
if [ "$file" -ef "$0" ]; then
  copy_dir=$(mktemp -d)
  mkdir "$copy_dir/scripts"
  cp "$0" "$(dirname "$0")/suite_changes.sh" "$copy_dir/scripts/"
  MUTANT_SH_ROOT=$root MUTANT_SH_COPY_DIR=$copy_dir exec bash "$copy_dir/scripts/mutant.sh" "${args[@]}"
fi

# A wiring file while a stack make started runs: its pid file (make's
# *-stack target writes enygma_<x>_stack.pid as it starts the stack) names
# a live process. A stale pid file, whose process is gone, blocks nothing,
# nor does no pid file (the glob then stays literal and cat fails).
abs=$(cd "$(dirname "$file")" && pwd -P)/$(basename "$file")
# The list goes in a variable first: grep -q on the pipe could exit before
# suite_changes.sh had written it all, and pipefail would read that SIGPIPE
# as no match.
wiring=$(bash "$(dirname "$0")/suite_changes.sh" --wiring)
if grep -qxF -- "${abs#"$root"/}" <<<"$wiring"; then
  for pid_file in "${TMPDIR:-/tmp}"/enygma_*_stack.pid; do
    pid=$(cat "$pid_file" 2>/dev/null) || continue
    if kill -0 "$pid" 2>/dev/null; then
      stack=${pid_file##*/enygma_}
      echo "BAD INPUT: $file wires the stacks, and the stack in $pid_file (pid $pid) is running, so the mutant would run inside it: stop it first (make ${stack%_stack.pid}-stack-stop)"
      exit 2
    fi
  done
fi
# failures picks the lines a red rests on: node's, go test's (its FAIL
# lines and its indented file_test.go:N: assertions) and errors.
failures='^not ok|^ *(error|Error|--- FAIL|FAIL)|^ +[^ ]+_test\.go:[0-9]+:'
out=$(mktemp)
# backup is set only once the copy is whole, so cleanup never restores a
# partial one.
copy=$(mktemp)
cp "$file" "$copy" || { rm -f "$copy"; exit 1; }
backup=$copy

# run <name> <command…> runs the test command, output to $out, in its own
# process group (perl's setsid: stock macOS has no setsid(1)), and sets
# status to its exit code. Then it kills the group, so nothing the command
# left behind runs on. Past the deadline it ends mutant.sh: HUNG, exit 1
# for the mutant run, 2 for the green run.
# perl execs the command as the shell would, never through one (exec
# { $ARGV[0] }: a command passed as one word is not split by /bin/sh), and
# exits 127 when it is not found, 126 when it can't run.
run() {
  local name=$1
  shift
  perl -e 'use POSIX qw(setsid); setsid(); exec { $ARGV[0] } @ARGV; print STDERR "mutant.sh: $ARGV[0]: $!\n"; exit($!{ENOENT} ? 127 : 126)' -- "$@" >"$out" 2>&1 </dev/null &
  pgid=$!
  # SECONDS counts whole seconds, so the + 1 keeps a run from being cut
  # before timeout seconds have passed.
  local deadline=$((SECONDS + timeout + 1))
  # bash reaps the background child as it exits, so kill -0 then fails.
  while kill -0 "$pgid" 2>/dev/null && [ $SECONDS -lt $deadline ]; do sleep 0.1; done
  local hung=
  if kill -0 "$pgid" 2>/dev/null; then
    hung=1
    # Disowned, bash reports no "Killed" for it; its status goes unused.
    disown "$pgid"
  fi
  kill -KILL -- "-$pgid" 2>/dev/null || true
  if [ -n "$hung" ]; then
    pgid=
    tail -8 "$out"
    echo "HUNG: the $name run passed $timeout s"
    [ "$name" = mutant ] && exit 1
    exit 2
  fi
  status=0
  wait "$pgid" || status=$?
  pgid=
}

# The command as the code stands: green, or there is nothing to show red.
run green "$@"
if [ $status != 0 ]; then
  tail -8 "$out"
  echo "BAD INPUT: the command fails without the mutant, so a red would show nothing"
  exit 2
fi

python3 - "$file" "$old" "$new" <<'PY' || exit 2
import sys
path, old, new = sys.argv[1:]
text = open(path, encoding="utf-8").read()
n = text.count(old)
if n != 1:
    sys.exit(f"mutant.sh: the old text occurs {n} times in {path}, want exactly once")
open(path, "w", encoding="utf-8").write(text.replace(old, new))
PY

run mutant "$@"
if [ $status = 0 ]; then
  echo "SURVIVED: the command passed with the mutant in place"
  exit 1
fi
if [ $status = 126 ] || [ $status = 127 ]; then
  tail -4 "$out"
  echo "BAD INPUT: the command did not run with the mutant in place (exit $status)"
  exit 2
fi
if [ -n "$expect" ] && ! grep -Eq -- "$expect" "$out"; then
  grep -E "$failures" "$out" | head -8 || tail -8 "$out"
  echo "SURVIVED: the command went red, but not from the expected check (/$expect/)"
  exit 1
fi
grep -E "$failures" "$out" | head -8 || tail -8 "$out"
echo "RED: the command failed with the mutant in place"
