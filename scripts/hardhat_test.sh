#!/usr/bin/env bash
# Run from PROJECT's directory as hardhat_test.sh PROJECT [COMPILE_ONLY ...].
# Keep Hardhat's output and exit status, then check that tests ran where expected.
set -euo pipefail

project=$1
shift
output=$(mktemp)
trap 'rm -f "$output"' EXIT
FORCE_COLOR=0 npx hardhat test | tee "$output"

# The default Mocha reporter writes one "N passing (duration)" and an
# optional "N pending". Fail if the passing count is missing or ambiguous.
counts=$(awk '
    /^[[:blank:]]*[0-9]+ passing \([^)]+\)[[:blank:]]*$/ { passes = $1; passing_lines++ }
    /^[[:blank:]]*[0-9]+ pending[[:blank:]]*$/ { pending = $1; pending_lines++ }
    END {
        if (passing_lines != 1 || pending_lines > 1) exit 1
        print (passes > 0), (pending > 0)
    }' "$output") || { echo "$project: cannot read the Hardhat test summary" >&2; exit 1; }
read -r has_passes has_pending <<<"$counts"

if [[ " $* " == *" $project "* ]]; then
    if [ "$has_passes" -ne 0 ] || [ "$has_pending" -ne 0 ]; then
        echo "$project: has Hardhat tests, remove it from HARDHAT_COMPILE_ONLY" >&2
        exit 1
    fi
elif [ "$has_passes" -eq 0 ]; then
    echo "$project: Hardhat ran no tests" >&2
    exit 1
fi
