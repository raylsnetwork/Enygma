#!/usr/bin/env bash
# The verdict on an on-chain suite's `go test -v` log, for the suites' make
# targets (make payments-test; built for dvp-test and retail-test too):
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

for k in PASS FAIL SKIP; do echo "suite $k: $(grep -c "^--- $k" "$log")"; done
grep -E '^ *--- (FAIL|SKIP)' "$log"

ok=1
[ "$status" = 0 ] || { echo "go test exited $status"; ok=0; }
[ "$(grep -c '^--- PASS' "$log")" -gt 0 ] || { echo "no test passed: the suite ran no tests"; ok=0; }
grep -qE '^ *--- FAIL' "$log" && ok=0
# read, not for over $(sed …): a subtest name may hold * or [ ], which an
# unquoted substitution would expand against the files in the working dir.
while IFS= read -r name; do
    case " $known " in
    *" $name "*) ;;
    *) echo "unexpected skip: $name"; ok=0 ;;
    esac
done < <(sed -nE 's/^ *--- SKIP: ([^ ]+) .*/\1/p' "$log")
[ "$ok" = 1 ]
