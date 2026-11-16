#!/usr/bin/env bash
# Fail when a hook type .pre-commit-config.yaml installs is missing from this
# clone: a clone set up before a type joined default_install_hook_types never
# runs that type's hooks (a missing commit-msg skips red-named), and nothing
# else says so. Fix with `make hooks`. CI has no hooks: it skips there.
#
#   scripts/check_hooks_installed.sh   (make check)
#
# CONFIG and HOOKS_DIR override the config file and the hooks dir (for tests).
set -euo pipefail

[ -z "${CI:-}" ] || exit 0
root=$(git rev-parse --show-toplevel)
config=${CONFIG:-$root/.pre-commit-config.yaml}
hooks=${HOOKS_DIR:-$(cd "$root" && git rev-parse --path-format=absolute --git-path hooks)}

types=$(sed -nE 's/^default_install_hook_types:[[:space:]]*\[(.*)\].*/\1/p' "$config" | tr ',' ' ')
[ -n "$types" ] || { echo "no default_install_hook_types in $config" >&2; exit 1; }

missing=
for t in $types; do
  # pre-commit writes each hook with its own marker line.
  grep -qs 'pre-commit' "$hooks/$t" || missing="$missing $t"
done
[ -z "$missing" ] && exit 0
echo "git hooks not installed in $hooks:$missing" >&2
echo "run: make hooks" >&2
exit 1
