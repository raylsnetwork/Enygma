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
# while a stack make started is up: a stack reads its scripts and helpers
# as it starts and runs (bash reads a script as it goes), so a mutant
# applied then could run inside the live stack and pass or fail it for the
# wrong reason.
# Exit 0 when the mutant is killed (red), 1 when it survives or goes red
# elsewhere, 2 on bad input: a wiring file of a live stack, a command
# already red without the mutant, or one that did not run (exit 126 or 127,
# as a command passed as one word).
# The verdict (RED:, SURVIVED: or BAD INPUT:) is always the last line, after
# the failure lines it rests on, so `| tail` keeps it.
#
#   scripts/mutant.sh enygma_retail_payments/shield_demo/index.html \
#     "hold * 1.3)" "hold * 3)" -- node --test scripts/shield_demo_replay.test.js
#   scripts/mutant.sh --expect 'TestProtocolPrimer/audit' <file> <old> <new> -- go test …
#   scripts/mutant.sh <file> <old> <new> -- go -C enygma_retail_payments/shield_demo test ./engine/
# Go runs inside its module (go -C <module dir>): from the repo root it
# finds no go.mod.
set -euo pipefail
expect=
if [ "${1:-}" = --expect ]; then
  [ $# -ge 2 ] || { echo "usage: mutant.sh [--expect <regex>] <file> <old> <new> -- <test command…>" >&2; exit 2; }
  expect=$2
  shift 2
fi
[ $# -ge 5 ] && [ "$4" = -- ] || { echo "usage: mutant.sh [--expect <regex>] <file> <old> <new> -- <test command…>" >&2; exit 2; }
file=$1 old=$2 new=$3
shift 4

# A wiring file while a stack make started is up: its pid file (make's
# *-stack target writes enygma_<x>_stack.pid) names a live process. A stale
# pid file, whose process is gone, blocks nothing, nor does no pid file (the
# glob then stays literal and cat fails).
root=$(cd "$(dirname "$0")/.." && pwd -P)
abs=$(cd "$(dirname "$file")" && pwd -P)/$(basename "$file")
# The list goes in a variable first: grep -q on the pipe could exit before
# suite_changes.sh had written it all, and pipefail would read that SIGPIPE
# as no match.
wiring=$(bash "$root/scripts/suite_changes.sh" --wiring)
if grep -qxF -- "${abs#"$root"/}" <<<"$wiring"; then
  for pid_file in "${TMPDIR:-/tmp}"/enygma_*_stack.pid; do
    pid=$(cat "$pid_file" 2>/dev/null) || continue
    if kill -0 "$pid" 2>/dev/null; then
      stack=${pid_file##*/enygma_}
      echo "BAD INPUT: $file wires the stacks, and the stack in $pid_file (pid $pid) is up, so the mutant would run inside it: stop it first (make ${stack%_stack.pid}-stack-stop)"
      exit 2
    fi
  done
fi
# failures picks the lines a red rests on: node's, go test's (its FAIL
# lines and its indented file_test.go:N: assertions) and errors.
failures='^not ok|^ *(error|Error|--- FAIL|FAIL)|^ +[^ ]+_test\.go:[0-9]+:'
out=$(mktemp)
backup=$(mktemp)
cp "$file" "$backup"
trap 'cp "$backup" "$file"; rm -f "$backup" "$out"' EXIT

# The command as the code stands: green, or there is nothing to show red.
if ! "$@" >"$out" 2>&1; then
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

status=0
"$@" >"$out" 2>&1 || status=$?
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
