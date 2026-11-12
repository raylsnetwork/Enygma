#!/usr/bin/env bash
# commit-msg hook: a commit that adds a check names its red (CODING_STANDARDS.md
# § Tests and checks). When the staged diff adds a Go test or subtest
# (func Test…, t.Run) or a JS test (test(…), it(…)), the message must carry a
# line with RED (the mutation that fails the check, as scripts/mutant.sh
# prints it), or a "No red:" line saying why (for example, live-chain only, or
# "stated in the PR").
#
#   scripts/check_red_named.sh <commit message file>
#
# Merges, fixup!/squash! commits and a message git will drop (only comments)
# pass. `git commit --no-verify` overrides it.
set -uo pipefail

msg_file=${1:?usage: check_red_named.sh <commit message file>}
git_dir=$(git rev-parse --git-dir)
[ -f "$git_dir/MERGE_HEAD" ] && exit 0

msg=$(grep -v '^#' "$msg_file")
case "$msg" in fixup!* | squash!* | amend!*) exit 0 ;; esac
[ -n "$(printf '%s' "$msg" | tr -d '[:space:]')" ] || exit 0

added=$(git diff --cached -U0 --no-color --diff-filter=AM -- '*.go' '*.js' '*.ts' |
  grep -E '^\+[^+]' |
  grep -E '^\+(func Test[A-Za-z0-9_]*\(|[[:space:]]*t\.Run\(|[[:space:]]*(test|it)\([[:space:]]*['"'"'"`])' |
  sed -E 's/^\+[[:space:]]*//; s/[[:space:]]*\{?[[:space:]]*$//')
[ -n "$added" ] || exit 0

if printf '%s\n' "$msg" | grep -qE '(^|[^A-Za-z])RED([^A-Za-z]|$)|^No red:'; then
  exit 0
fi

echo "This commit adds checks but its message names no red:" >&2
printf '%s\n' "$added" | sed 's/^/  /' >&2
echo "Add the mutation that fails each one, with RED (scripts/mutant.sh prints it)," >&2
echo "or a \"No red: <why>\" line (CODING_STANDARDS.md § Tests and checks)." >&2
exit 1
