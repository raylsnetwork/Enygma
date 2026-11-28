#!/usr/bin/env bash
# Rebuilds the setup ceremony's release twice, each from a clean clone of
# HEAD at a path of its own, and fails unless both give the same
# SHA256SUMS: the rebuild that independent reviewers make, and that the
# announcement's sha256s rest on (audit L15, M7).
#
#   make ceremony-repro
set -euo pipefail
root=$(git rev-parse --show-toplevel)
if [[ -n "$(git -C "$root" status --porcelain --untracked-files=no)" ]]; then
  echo "ceremony_reproducible: rebuilding HEAD, $(git -C "$root" rev-parse HEAD): the changes not committed are left out" >&2
fi
work=$(mktemp -d)
trap 'chmod -R u+w "$work" 2>/dev/null; rm -r "$work"' EXIT
# Each side compiles from scratch, in a build cache of its own: with
# -trimpath, a shared one would hand the second side the first's packages.
for side in a b/deeper; do
  git clone --quiet --no-local "$root" "$work/$side"
  GOCACHE="$work/cache-${side%%/*}" make -C "$work/$side" --no-print-directory ceremony-dist CEREMONY_URL=https://ceremony.example.org >/dev/null
done
if ! diff "$work/a/enygma_ceremony/dist/SHA256SUMS" "$work/b/deeper/enygma_ceremony/dist/SHA256SUMS"; then
  echo "ceremony_reproducible: two clean clones of $(git rev-parse HEAD) built different programs" >&2
  exit 1
fi
echo "Two clean clones of $(git rev-parse HEAD), at different paths, built the same programs:"
cat "$work/a/enygma_ceremony/dist/SHA256SUMS"
