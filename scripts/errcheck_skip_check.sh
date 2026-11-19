#!/usr/bin/env bash
# make errcheck-skip-check: fail when a module the Makefile's
# ERRCHECK_SKIP_MODULES leaves out of make errcheck is now errcheck-clean,
# so its entry is deleted rather than letting new dropped errors in; and
# fail on an entry that is not a tracked module (a module renamed or moved
# since), which errcheck's $(filter-out) never matches, so it skips nothing
# while reading as skipped; and fail on one errcheck could not check (a
# build break), whose findings it cannot tell. An entry runs only when it is one of
# <modules>: CI's go matrix runs each module's entry in that module's leg,
# with its go.mod's go. An entry naming no tracked module fails in every
# leg. <errcheck> is the command make errcheck runs in each module (with
# ./... appended), as one word list.
#
#   scripts/errcheck_skip_check.sh "<errcheck>" "<module dir>..." [<skipped module dir>...]
#
# Tested by scripts/errcheck_skip_check.test.js.
set -euo pipefail
[ $# -ge 2 ] || { echo 'usage: scripts/errcheck_skip_check.sh "<errcheck>" "<module dir>..." [<skipped module dir>...]' >&2; exit 2; }
read -r -a errcheck <<<"$1"
modules=" $2 "
shift 2
[ $# -gt 0 ] || { echo "errcheck skip: no ERRCHECK_SKIP_MODULES entries to check"; exit 0; }

tracked=" "
for gomod in $(git ls-files '*go.mod'); do tracked="$tracked$(dirname "$gomod") "; done

fail=0
for dir in "$@"; do
    case "$tracked" in
        *" $dir "*) ;;
        *)
            echo "ERRCHECK_SKIP_MODULES entry $dir is not a tracked Go module (make modules lists them): fix it to the module's dir, or delete it" >&2
            fail=1
            continue
            ;;
    esac
    case "$modules" in
        *" $dir "*) ;;
        *) echo "errcheck skip: $dir not run here"; continue ;;
    esac
    if out=$(cd "$dir" && "${errcheck[@]}" ./... 2>&1); then
        echo "ERRCHECK_SKIP_MODULES entry $dir is errcheck-clean: delete it from the Makefile so make errcheck checks it" >&2
        fail=1
    # errcheck prints each finding as file.go:line:col:<TAB>source; a build
    # break or a failed download prints none, and is not the red the entry
    # records. (Through go run, both exit 1.)
    elif printf '%s\n' "$out" | grep -q $'\\.go:[0-9]*:[0-9]*:\t'; then
        n=$(printf '%s\n' "$out" | grep -c $'\\.go:[0-9]*:[0-9]*:\t')
        echo "errcheck skip: $dir still has $n errcheck findings, as ERRCHECK_SKIP_MODULES says"
    else
        echo "ERRCHECK_SKIP_MODULES entry $dir: errcheck did not check it, so it cannot tell it still has findings; it printed:" >&2
        printf '%s\n' "$out" | awk 'NR <= 20 { print "    " $0 }' >&2
        fail=1
    fi
done
exit $fail
