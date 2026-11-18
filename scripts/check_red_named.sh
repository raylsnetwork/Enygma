#!/usr/bin/env bash
# commit-msg hook: a commit that adds a check names its red (CODING_STANDARDS.md
# § Tests and checks). When the staged diff adds a Go test or subtest
# (func Test…, t.Run) or a JS test (test(…), it(…)), the message must carry a
# line with RED (the mutation that fails the check, as scripts/mutant.sh
# prints it), or a "No red:" line saying why (for example, live-chain only, or
# "stated in the PR").
# With --pr, the same rule on a pull request, for CI's red-named workflow:
# the checks its diff (base...head) adds, and a red in its body or any of
# its commit messages. The hook alone misses a clone without it installed,
# and a squash merge's message, which never passes through it.
#
#   scripts/check_red_named.sh <commit message file>
#   scripts/check_red_named.sh --pr <base> <head> <PR body file>
#
# As the hook, merges, fixup!/squash! commits and a message git will drop
# (only comments) pass, and `git commit --no-verify` overrides it.
set -uo pipefail

if [ "${1:-}" = --pr ]; then
  [ $# -eq 4 ] || { echo "usage: check_red_named.sh --pr <base> <head> <PR body file>" >&2; exit 2; }
  base=$2 head=$3 body=$4
  # A range git cannot read fails at the diff below, not here.
  log=$(git log --format=%B "$base..$head")
  msg=$(cat "$body") || exit 2
  msg="$msg
$log"
  what="This PR adds checks but neither its body nor its commit messages name a red:"
  diff_args=("$base...$head")
else
  msg_file=${1:?usage: check_red_named.sh <commit message file>}
  git_dir=$(git rev-parse --git-dir)
  [ -f "$git_dir/MERGE_HEAD" ] && exit 0

  msg=$(grep -v '^#' "$msg_file")
  case "$msg" in fixup!* | squash!* | amend!*) exit 0 ;; esac
  [ -n "$(printf '%s' "$msg" | tr -d '[:space:]')" ] || exit 0
  what="This commit adds checks but its message names no red:"
  diff_args=(--cached)
fi

# --no-renames, with deletions: a renamed file is its old path's removed lines
# and its new path's added lines, so a test that only moved is on both (and
# needs no red) and a test new to the renamed file is on the added side only.
diff=$(git diff "${diff_args[@]}" -U0 --no-color --no-renames --diff-filter=AMD -- '*.go' '*.js' '*.ts') || {
  echo "check_red_named.sh: git diff ${diff_args[*]} failed, so the checks it adds are unknown" >&2; exit 2; }

# checks <sign> — "<name>	<line>" for each check declared on the diff's
# <sign> (+ or -) lines; the name is the Go test's, or the subtest's or JS
# test's quoted title.
checks() {
  printf '%s\n' "$diff" | grep -E "^[$1][^$1]" | sed -nE \
    -e 's/^.(func (Test[A-Za-z0-9_]*)\(.*)$/go:\2	\1/p' \
    -e 's/^.[[:space:]]*(t\.Run\([[:space:]]*"([^"]*)".*)$/run:\2	\1/p' \
    -e "s/^.[[:space:]]*((test|it)\\([[:space:]]*['\"\`]([^'\"\`]*).*)\$/js:\\3	\\1/p"
}

# A check whose name is also on a removed line was edited or moved, not added.
removed=$(checks - | cut -f1)
added=$(checks + | while IFS=$'\t' read -r name line; do
  printf '%s\n' "$removed" | grep -qxF -- "$name" || printf '%s\n' "$line"
done | sed -E 's/[[:space:]]*\{?[[:space:]]*$//')
[ -n "$added" ] || exit 0

if printf '%s\n' "$msg" | grep -qE '(^|[^A-Za-z])RED([^A-Za-z]|$)|^No red:'; then
  exit 0
fi

echo "$what" >&2
printf '%s\n' "$added" | sed 's/^/  /' >&2
echo "Add the mutation that fails each one, with RED (scripts/mutant.sh prints it)," >&2
echo "or a line that starts \"No red: <why>\" (CODING_STANDARDS.md § Tests and checks)." >&2
exit 1
