#!/usr/bin/env bash
# Show a check red: run the test command once as the code stands, which
# must pass; then replace old with new in file (old must occur exactly
# once), run it again, restore the file whatever happens, and say whether
# the command went red. Run the whole test file rather than a name filter,
# so a renamed test can't drop out of the run unnoticed. With --expect, the
# red counts only if the output matches regex, naming the check under test
# (a subtest's name, say), so a red from an earlier check failing first
# doesn't pass for the new check's.
# Exit 0 when the mutant is killed (red), 1 when it survives or goes red
# elsewhere, 2 on bad input: a command already red without the mutant, or
# one that did not run (exit 126 or 127, as a command passed as one word).
#
#   scripts/mutant.sh enygma_retail_payments/shield_demo/index.html \
#     "hold * 1.3)" "hold * 3)" -- node --test scripts/shield_demo_replay.test.js
#   scripts/mutant.sh --expect 'TestProtocolPrimer/audit' <file> <old> <new> -- go test …
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
out=$(mktemp)
backup=$(mktemp)
cp "$file" "$backup"
trap 'cp "$backup" "$file"; rm -f "$backup" "$out"' EXIT

# The command as the code stands: green, or there is nothing to show red.
if ! "$@" >"$out" 2>&1; then
  echo "BAD INPUT: the command fails without the mutant, so a red would show nothing"
  tail -8 "$out"
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
  echo "BAD INPUT: the command did not run with the mutant in place (exit $status)"
  tail -4 "$out"
  exit 2
fi
if [ -n "$expect" ] && ! grep -Eq -- "$expect" "$out"; then
  echo "SURVIVED: the command went red, but not from the expected check (/$expect/)"
  grep -E '^not ok|^ *(error|Error|--- FAIL|FAIL)' "$out" | head -8 || tail -8 "$out"
  exit 1
fi
echo "RED: the command failed with the mutant in place"
grep -E '^not ok|^ *(error|Error|--- FAIL|FAIL)' "$out" | head -8 || tail -8 "$out"
