#!/usr/bin/env bash
# The verdict on an on-chain suite's `go test -v` log, for the suites' make
# targets (make payments-test, make dvp-test, make retail-test, make retail-tags-test):
# prints `suite PASS/FAIL/SKIP` counts (top-level tests), lists the failed and
# skipped tests, and exits non-zero when go test did, when no test passed (a
# suite that ran nothing), when any test failed, or when a test (or subtest)
# skipped that is not on the suite's known-skip list.
#
# Usage: suite_result.sh <log> <go test exit status> "<known skip> ..."
#   Known skips are full test names (TestA, or TestA/sub for a subtest),
#   space-separated; the list sits next to the suite's make target.
#
#   node --test scripts/suite_result.test.js   (make page-check)
set -uo pipefail
log=$1 status=$2 known=${3:-}

# Every verdict go test printed, one per line as "<PASS|FAIL|SKIP> <name> (<time>)",
# wherever it sits: a test whose own output ends without a newline gets its
# verdict glued on (progress…--- SKIP: TestX (0.00s)), so a match at the
# line's start alone would miss it. A t.Log line (    x_test.go:9: …) ends
# with its newline, so nothing is glued onto it, and verdict text a test logs
# there is not a verdict. A name with a / is a subtest's.
verdicts=$(grep -vE '^ +[^ ]+\.go:[0-9]+: ' "$log" |
    grep -oE -- '--- (PASS|FAIL|SKIP): [^ ]+ \([^)]*\)' | sed -E 's/^--- ([A-Z]+): /\1 /')
# top_count <verdict> counts the top-level tests with that verdict.
top_count() { awk -v v="$1" '$1 == v && $2 !~ /\// { n++ } END { print n + 0 }' <<<"$verdicts"; }
for k in PASS FAIL SKIP; do echo "suite $k: $(top_count $k)"; done
# The failed and skipped tests, a subtest indented under its parent (gsub
# returns the number of /s, its depth).
awk '$1 != "PASS" && NF { depth = gsub(/\//, "/", $2); printf "%*s--- %s: %s %s\n", 4 * depth, "", $1, $2, $3 }' <<<"$verdicts"

ok=1
[ "$status" = 0 ] || { echo "go test exited $status"; ok=0; }
[ "$(top_count PASS)" -gt 0 ] || { echo "no test passed: the suite ran no tests"; ok=0; }
grep -q '^FAIL ' <<<"$verdicts" && ok=0
# Also any line-start FAIL, in a shape the parse above does not read.
grep -qE '^ *--- FAIL' "$log" && ok=0
# read, not for over $(sed …): a subtest name may hold * or [ ], which an
# unquoted substitution would expand against the files in the working dir.
while IFS= read -r name; do
    case " $known " in
    *" $name "*) ;;
    *) echo "unexpected skip: $name"; ok=0 ;;
    esac
done < <(sed -nE 's/^SKIP ([^ ]+) .*/\1/p' <<<"$verdicts")
[ "$ok" = 1 ]
