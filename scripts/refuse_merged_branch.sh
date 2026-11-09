#!/usr/bin/env bash
# pre-push hook: refuses a push to origin on a branch whose pull request has
# already merged, because a new PR from that branch would carry the merged
# commits again (main rebases or squashes them, so their hashes differ).
#
# A branch with an open PR passes, as does any push when gh can't answer
# (not installed, offline): the hook is a warning light, so it never blocks
# a push for its own failure. `git push --no-verify` overrides it.
set -uo pipefail

REPO=raylsnetwork/Enygma
[ "${PRE_COMMIT_REMOTE_NAME:-}" = origin ] || exit 0
branch=${PRE_COMMIT_REMOTE_BRANCH#refs/heads/}

merged=$(gh pr list -R "$REPO" --head "$branch" --state all --limit 100 \
  --json number,state,isCrossRepository \
  --jq '[.[] | select(.isCrossRepository | not)]
        | if any(.state == "OPEN") then empty
          else (map(select(.state == "MERGED")) | .[0].number // empty) end') || exit 0

[ -n "$merged" ] || exit 0
echo "PR #$merged already merged branch $branch." >&2
echo "Start a new branch from origin/main and cherry-pick your commits onto it." >&2
exit 1
