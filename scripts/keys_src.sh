#!/usr/bin/env bash
# Print a fingerprint of the circuit sources a module's keys are built from:
# the content of every non-test .go file in the packages its keygen package
# builds (go list -deps, so a new import counts at once) that live in this
# repo, the module's own or a local module it replaces (enygma_gnark_shared),
# and the go.mod and go.sum of each of those modules; committed or not.
# Paths are repo-relative, so the fingerprint is the same in any clone;
# file times play no part, so a rebase or a fresh checkout leaves it.
# It fails, rather than fingerprint nothing, on a package with no source
# it can find in the repo (a local replace outside it, or files git
# ignores).
# Each keygen target writes it beside the keys, and its stack's preflight
# compares it with the working tree's (check_keys_stamp, scripts/stack_lib.sh),
# so keys built from other circuit sources fail before anything builds. CI's on-chain workflow keys its
# cache of those keys on it.
#   make dvp-keygen            scripts/keys_src.sh enygma_dvp/gnark_circuits ./cmd/keygen
#                              -> enygma_dvp/gnark_circuits/scripts/keys/keys.src (enygma_dvp/scripts/stack.sh)
#   make payments-burn-keygen  scripts/keys_src.sh enygma_payments/gnark-server ./keygen
#                              -> enygma_payments/gnark-server/keys/burn.src (enygma_payments/run_scripts/stack.sh)
#
#   scripts/keys_src.sh <module dir> <keygen package>
set -euo pipefail
if [ $# -ne 2 ]; then
    echo "usage: scripts/keys_src.sh <module dir> <keygen package>  (module dir repo-relative, package as go list takes it from there)" >&2
    exit 2
fi
module=$1 keygen=$2
root="$(git -C "$(dirname "$0")" rev-parse --show-toplevel)"
cd "$root/$module"
# One line per package: its dir, then its module's go.mod; only packages of
# the main module or of a module replaced by a local path.
pkgs=$(go list -deps -f '{{with .Module}}{{if or .Main (and .Replace (not .Replace.Version))}}{{$.Dir}}	{{.GoMod}}{{end}}{{end}}' "$keygen")
cd "$root"
# sources prints the repo-relative path of every file the fingerprint covers.
sources() {
    local dir gomod rel files mod
    while IFS=$'\t' read -r dir gomod; do
        case "$dir" in
            "$root"/*) ;;
            *) echo "keys_src.sh: package $dir is outside the repo ($root)" >&2; exit 1 ;;
        esac
        rel=${dir#"$root"/}
        files=$(git ls-files -co --exclude-standard -- "$rel" | grep -E "^$rel/[^/]+\.go$" | grep -v '_test\.go$') ||
            { echo "keys_src.sh: no non-test .go files found for package $rel" >&2; exit 1; }
        echo "$files"
        mod=$(dirname "${gomod#"$root"/}")
        echo "$mod/go.mod"
        echo "$mod/go.sum"
    done <<<"$pkgs"
}
list=$(sources)
LC_ALL=C sort -u <<<"$list" |
    while IFS= read -r f; do if [ -f "$f" ]; then shasum -a 256 "$f"; fi; done |
    shasum -a 256 | cut -d' ' -f1
