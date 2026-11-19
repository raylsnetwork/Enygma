#!/usr/bin/env bash
# make test-name-skip-check: fail when an entry of a Makefile list of test
# names (a suite's allowed skips, or a -skip or -run regex) names no test in
# that list's module. A -skip alternative naming no test skips nothing, so a
# renamed test starts running in the suite; a -run alternative naming no test
# runs nothing, and go test passes; an allowed-skip name naming no test lets
# the list drift from the tests. An entry must be the name of a
# func TestX( in a tracked _test.go of the module, outside any module
# nested in it (go test ./... does not run those). The suites need a stack,
# so only the names are checked, not that the tests skip or pass.
#
# Each list comes as four arguments: its kind, its name (for the message),
# its module's dir and its entries as one word.
#   names  entries split on spaces (DVP_KNOWN_SKIPS).
#   regex  a go test -skip or -run regex: split on |, a trailing $ anchor
#          dropped. An empty alternative fails: it matches every test, so
#          a -skip skips them all and a -run runs them all.
#
#   scripts/test_name_skip_check.sh {names|regex} <list> <module dir> <entries> ...
#
# Tested by scripts/test_name_skip_check.test.js.
set -euo pipefail
set -f
usage='usage: scripts/test_name_skip_check.sh {names|regex} <list> <module dir> <entries> ...'
[ $# -gt 0 ] && [ $(($# % 4)) -eq 0 ] || { echo "$usage" >&2; exit 2; }

tracked=" "
for gomod in $(git ls-files '*go.mod'); do tracked="$tracked$(dirname "$gomod") "; done

# in_nested: whether file $1 lies in a module nested in the list's ($nested).
in_nested() {
    local n
    for n in $nested; do [ "${1#"$n"/}" = "$1" ] || return 0; done
    return 1
}

fail=0
# check_entry fails on entry $1 of the current list when no test has its name.
check_entry() {
    if [ -z "$1" ]; then
        echo "$list has an empty alternative, which matches every test (a -skip skips them all, a -run runs them all): delete the extra |" >&2
        fail=1
    elif ! printf '%s\n' "$names" | grep -qxF -- "$1"; then
        echo "$list entry $1 names no test in $dir (no func $1( in its tracked _test.go files): fix it to the test's full name, or delete it" >&2
        fail=1
    else
        count=$((count + 1))
    fi
}

while [ $# -gt 0 ]; do
    kind=$1 list=$2 dir=$3 entries=$4
    shift 4
    [ "$kind" = names ] || [ "$kind" = regex ] || { echo "$usage" >&2; exit 2; }
    case "$tracked" in
        *" $dir "*) ;;
        *)
            echo "$list's module $dir is not a tracked Go module (make modules lists them): fix the dir beside $list in the Makefile" >&2
            fail=1
            continue
            ;;
    esac
    nested=$(git ls-files -- "$dir/*go.mod" | sed 's#/go\.mod$##' | grep -vxF -- "$dir" || true)
    names=$(git ls-files -- "$dir/*_test.go" | while IFS= read -r f; do
        in_nested "$f" || sed -n 's/^func \(Test[A-Za-z0-9_]*\)(.*/\1/p' "$f"
    done)
    count=0 before=$fail
    if [ "$kind" = names ]; then
        for e in $entries; do check_entry "$e"; done
    elif [ -n "$entries" ]; then
        rest=$entries
        while :; do
            e=${rest%%|*}
            e=${e%\$}
            check_entry "$e"
            [ "${rest#*|}" != "$rest" ] || break
            rest=${rest#*|}
        done
    fi
    if [ "$fail" = "$before" ]; then
        if [ $count = 0 ]; then echo "test names: $list: no entries"; else echo "test names: $list: $count entries, each a test in $dir"; fi
    fi
done
exit $fail
