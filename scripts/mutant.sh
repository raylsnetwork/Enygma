#!/usr/bin/env bash
# Show a check red: replace old with new in file (old must occur exactly
# once), run the test command, restore the file whatever happens, and say
# whether the command went red. Run the whole test file rather than a
# name filter, so a renamed test can't drop out of the run unnoticed.
# Exit 0 when the mutant is killed (red), 1 when it survives, 2 on bad input.
#
#   scripts/mutant.sh enygma_retail_payments/shield_demo/index.html \
#     "hold * 1.3)" "hold * 3)" -- node --test scripts/shield_demo_replay.test.js
set -euo pipefail
[ $# -ge 5 ] && [ "$4" = -- ] || { echo "usage: mutant.sh <file> <old> <new> -- <test command…>" >&2; exit 2; }
file=$1 old=$2 new=$3
shift 4
backup=$(mktemp)
cp "$file" "$backup"
trap 'cp "$backup" "$file"; rm -f "$backup"' EXIT
python3 - "$file" "$old" "$new" <<'PY' || exit 2
import sys
path, old, new = sys.argv[1:]
text = open(path, encoding="utf-8").read()
n = text.count(old)
if n != 1:
    sys.exit(f"mutant.sh: the old text occurs {n} times in {path}, want exactly once")
open(path, "w", encoding="utf-8").write(text.replace(old, new))
PY
out=$(mktemp)
if "$@" >"$out" 2>&1; then
  echo "SURVIVED: the command passed with the mutant in place"
  rm -f "$out"
  exit 1
fi
echo "RED: the command failed with the mutant in place"
grep -E '^not ok|^ *(error|Error|--- FAIL|FAIL)' "$out" | head -8 || tail -8 "$out"
rm -f "$out"
