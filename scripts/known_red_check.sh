#!/usr/bin/env bash
# make known-red-check: fail when a known-red package (the Makefile's
# TEST_SKIP_PACKAGES, which test-unit leaves out) passes its tests, so its
# entry is deleted rather than keeping a working test out of CI; and fail on
# an entry that names no package, which test-unit's filter never matches, so
# it skips nothing while reading as known red. Each entry runs from the
# tracked module that owns it (the longest module path its import path
# starts with), and only when that module is one of <modules>: CI's go
# matrix runs each module's entries in that module's leg, with its LFS
# objects and its go.mod's go. An entry no tracked module owns fails in
# every leg. It fails, in every leg too, on a TEST_SKIP_MODULES entry (the
# second argument) that is not a tracked module: test-unit's $(filter-out)
# never matches it, so it skips nothing. Those modules' tests need a stack,
# so only the name is checked.
#
#   scripts/known_red_check.sh "<module dir>..." "<skipped module dir>..." [<package>...]
#
# Tested by scripts/known_red_check.test.js.
set -euo pipefail
[ $# -ge 2 ] || { echo 'usage: scripts/known_red_check.sh "<module dir>..." "<skipped module dir>..." [<package>...]' >&2; exit 2; }
modules=" $1 "
skipped_modules=$2
shift 2

fail=0
tracked=" "
for gomod in $(git ls-files '*go.mod'); do tracked="$tracked$(dirname "$gomod") "; done
for dir in $skipped_modules; do
    case "$tracked" in
        *" $dir "*) ;;
        *)
            echo "TEST_SKIP_MODULES entry $dir is not a tracked Go module (make modules lists them): fix it to the module's dir, or delete it" >&2
            fail=1
            ;;
    esac
done
[ $# -gt 0 ] || { echo "known red: no TEST_SKIP_PACKAGES entries to check"; exit $fail; }

# owner prints the dir of the tracked module whose module path is the
# longest prefix of import path $1, or nothing.
owner() {
    local best= bestlen=0 gomod dir mod
    for gomod in $(git ls-files '*go.mod'); do
        dir=$(dirname "$gomod")
        mod=$(awk '$1 == "module" { print $2; exit }' "$gomod")
        [ -n "$mod" ] || continue
        if [ "$1" = "$mod" ] || [ "${1#"$mod"/}" != "$1" ]; then
            [ ${#mod} -le $bestlen ] || { best=$dir; bestlen=${#mod}; }
        fi
    done
    printf '%s' "$best"
}

for pkg in "$@"; do
    dir=$(owner "$pkg")
    if [ -z "$dir" ]; then
        echo "TEST_SKIP_PACKAGES entry $pkg names no package: no tracked module's path is a prefix of it. Fix it to the package's import path (go list ./... in its module), or delete it" >&2
        fail=1
        continue
    fi
    case "$modules" in
        *" $dir "*) ;;
        *) echo "known red: $pkg is in $dir, not run here"; continue ;;
    esac
    # Exactly the package, as test-unit's grep -v -x matches it: a pattern
    # (example/m/...) lists other paths, and test-unit's filter skips none.
    if [ "$(cd "$dir" && go list "$pkg" 2>/dev/null)" != "$pkg" ]; then
        echo "TEST_SKIP_PACKAGES entry $pkg names no package in $dir (go list prints no package of that import path). Fix it to the package's import path, or delete it" >&2
        fail=1
        continue
    fi
    if out=$(cd "$dir" && go test -count=1 "$pkg" 2>&1); then
        echo "TEST_SKIP_PACKAGES entry $pkg passes its tests (in $dir): delete it, and its comment, from the Makefile so test-unit runs it" >&2
        fail=1
    elif printf '%s\n' "$out" | grep -q -- '^--- FAIL: '; then
        echo "known red: $pkg still fails its tests, as TEST_SKIP_PACKAGES says:"
        printf '%s\n' "$out" | awk '/^--- FAIL: / && n++ < 20 { print "    " $0 }'
    else
        # Not a test's failure (a build break, a failed download): not the red the entry records.
        echo "TEST_SKIP_PACKAGES entry $pkg did not run its tests (in $dir), so it cannot tell them still red; go test printed:" >&2
        printf '%s\n' "$out" | awk 'NR <= 20 { print "    " $0 }' >&2
        fail=1
    fi
done
exit $fail
