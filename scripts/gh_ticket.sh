#!/usr/bin/env bash
# Publish one ticket on raylsnetwork/Enygma: create the issue as a sub-issue
# of its parent, labelled ready-for-agent, add a native blocked-by edge for
# each blocker, and print the new issue's number.
#
#   scripts/gh_ticket.sh --title "..." --body-file ticket.md --parent 191 \
#     [--blocked-by 198]... [--label ready-for-agent] [--dry-run]
#
# One call per ticket, blockers first, so each later ticket can name the
# numbers the earlier calls printed. --dry-run prints the gh commands instead.
set -euo pipefail

REPO=raylsnetwork/Enygma
title= body= parent= label=ready-for-agent dry=
blockers=()
while [ $# -gt 0 ]; do
  case "$1" in
    --title) title=$2; shift 2 ;;
    --body-file) body=$2; shift 2 ;;
    --parent) parent=$2; shift 2 ;;
    --blocked-by) blockers+=("$2"); shift 2 ;;
    --label) label=$2; shift 2 ;;
    --dry-run) dry=1; shift ;;
    *) echo "gh_ticket: unknown argument $1" >&2; exit 2 ;;
  esac
done
[ -n "$title" ] || { echo "gh_ticket: --title is required and must not be empty" >&2; exit 2; }
[ -f "$body" ] || { echo "gh_ticket: --body-file $body is not a file" >&2; exit 2; }
for n in ${parent:+"$parent"} ${blockers[@]+"${blockers[@]}"}; do
  [[ "$n" =~ ^[0-9]+$ ]] || { echo "gh_ticket: issue number $n is not a number" >&2; exit 2; }
done

# A ticket may cite GLOSSARY.md or an ADR, and whoever picks it up works
# from the remote: publish once both are committed and pushed.
root=$(git -C "$(dirname "$0")" rev-parse --show-toplevel)
local=$(git -C "$root" status --porcelain -- GLOSSARY.md docs/adr; git -C "$root" log --oneline HEAD --not --remotes -- GLOSSARY.md docs/adr)
[ -z "$local" ] || { printf 'gh_ticket: commit and push GLOSSARY.md and docs/adr first; only this clone has:\n%s\n' "$local" >&2; exit 2; }

# run runs its arguments, or prints them with --dry-run.
run() { if [ -n "$dry" ]; then printf '%q ' "$@" >&2; echo >&2; else "$@"; fi; }

url=$(run gh issue create -R "$REPO" --title "$title" --body-file "$body" --label "$label" ${parent:+--parent "$parent"})
num=${url##*/}
[ -n "$dry" ] && num=NEW
for b in ${blockers[@]+"${blockers[@]}"}; do
  # The dependencies API takes the blocker's database id, not its number.
  id=$( [ -n "$dry" ] && echo "<id of #$b>" || gh api "repos/$REPO/issues/$b" --jq .id )
  run gh api --method POST "repos/$REPO/issues/$num/dependencies/blocked_by" -F "issue_id=$id" >/dev/null
done
echo "$num"
