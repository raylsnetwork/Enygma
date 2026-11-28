#!/usr/bin/env bash
# The setup ceremony's release, as .github/workflows/ceremony-release.yml
# makes it from a ceremony-v* tag:
#
#   ceremony_release.sh check          refuse a tag, coordinator address,
#                                      repository or commit that is not plain
#   ceremony_release.sh notes DIST     write DIST/notes.md and DIST/COMMANDS.txt
#                                      from DIST/SHA256SUMS
#   ceremony_release.sh publish DIST   publish DIST as the tag's release, only
#                                      while the tag is the commit built
#
# It reads TAG, CEREMONY_URL, GITHUB_REPOSITORY and GITHUB_SHA from the
# environment. Each reaches participants' shells in the commands the notes
# give, so each is checked to be plain before any is used (audit L14, M8),
# and quoted where it is used all the same.
#
#   node --test scripts/ceremony_release.test.js   (make page-check)
set -euo pipefail

die() { echo "ceremony_release: $*" >&2; exit 1; }

check() {
  [[ "${TAG:-}" =~ ^ceremony-v[0-9]+(\.[0-9]+)*$ ]] ||
    die "the tag is ceremony-v<number>[.<number>...], nothing else"
  [[ "${CEREMONY_URL:-}" =~ ^https://[A-Za-z0-9.-]+(:[0-9]{1,5})?$ ]] ||
    die "set the repository variable CEREMONY_URL to the coordinator's address, a plain https://host[:port]"
  [[ "${GITHUB_REPOSITORY:-}" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] ||
    die "GITHUB_REPOSITORY is no owner/repository"
  [[ "${GITHUB_SHA:-}" =~ ^[0-9a-f]{40}$ ]] ||
    die "GITHUB_SHA is no commit"
}

# The files the release holds, one per system.
FILES=(ceremony-darwin-arm64 ceremony-darwin-amd64 ceremony-linux-amd64 ceremony-linux-arm64
  ceremony-windows-amd64.exe ceremony-windows-arm64.exe)

notes() {
  local dist=$1
  check
  local repo="https://github.com/$GITHUB_REPOSITORY" name="${GITHUB_REPOSITORY#*/}"
  local dl="$repo/releases/download/$TAG"
  local workflow="$GITHUB_REPOSITORY/.github/workflows/ceremony-release.yml"
  sum() { awk -v f="$1" '$2 == f { print $1 }' "$dist/SHA256SUMS"; }
  local f
  for f in "${FILES[@]}"; do
    [[ "$(sum "$f")" =~ ^[0-9a-f]{64}$ ]] || die "no sha256 for $f in $dist/SHA256SUMS"
  done

  # Two steps for each system: download and check, then run. Between them,
  # the participant compares the sha256 with the announcement, which these
  # notes, editable after publishing, cannot change (audit M9). The run
  # checks again, so a pasted block never runs a program whose sha256 is
  # not this release's. No step holds a comment: interactive zsh, macOS's
  # shell, runs a pasted "#" line as a command.
  local systems=(
    "macOS (Apple silicon)|ceremony-darwin-arm64|shasum -a 256"
    "macOS (Intel)|ceremony-darwin-amd64|shasum -a 256"
    "Linux (x86-64)|ceremony-linux-amd64|sha256sum"
    "Linux (ARM64)|ceremony-linux-arm64|sha256sum"
    "Windows (x86-64)|ceremony-windows-amd64.exe|"
    "Windows (ARM64)|ceremony-windows-arm64.exe|"
  )
  # printf, not echo, for each command: a PowerShell one holds ".\c",
  # which an echo that reads escapes (sh, on macOS) takes as "stop here".
  download() { # file, checker (none: PowerShell)
    if [[ -n "$2" ]]; then
      printf '%s\n' "curl -fsSLO '$dl/$1' && echo '$(sum "$1")  $1' | $2 -c"
    else
      printf '%s\n' "Invoke-WebRequest -UseBasicParsing '$dl/$1' -OutFile '$1'; if ((Get-FileHash '$1' -Algorithm SHA256).Hash -eq '$(sum "$1")') { '$1 matches this release' } else { throw '$1 is not this release''s' }"
    fi
  }
  runs() { # file, checker (none: PowerShell)
    if [[ -n "$2" ]]; then
      printf '%s\n' "echo '$(sum "$1")  $1' | $2 -c && chmod +x '$1' && './$1'"
    else
      printf '%s\n' "if ((Get-FileHash '$1' -Algorithm SHA256).Hash -eq '$(sum "$1")') { & '.\\$1' } else { throw '$1 is not this release''s' }"
    fi
  }
  # From source, the second line builds and runs only at the commit
  # announced, however the first went: a clone or a checkout that failed
  # would otherwise build whatever is checked out.
  local from_source_unix="git clone '$repo.git' && cd '$name' && git checkout '$GITHUB_SHA'
[ \"\$(git rev-parse HEAD)\" = '$GITHUB_SHA' ] && cd enygma_ceremony/cmd/ceremony && go build && ./ceremony join -url '$CEREMONY_URL'"
  local from_source_windows="git clone '$repo.git'; cd '$name'; git checkout '$GITHUB_SHA'
if ((git rev-parse HEAD) -eq '$GITHUB_SHA') { cd enygma_ceremony/cmd/ceremony; go build; if (\$?) { .\\ceremony.exe join -url '$CEREMONY_URL' } } else { throw 'this is not the commit announced' }"
  local system label file checker lang
  {
    echo "Commands for the Enygma setup ceremony's participant program, release $TAG of $GITHUB_REPOSITORY, commit $GITHUB_SHA."
    echo "Before running one, compare the commit and the program's sha256 with the ceremony's announcement."
    for system in "${systems[@]}"; do
      IFS='|' read -r label file checker <<< "$system"
      [[ -z "$checker" ]] && label="$label (PowerShell)"
      echo
      echo "== $label =="
      echo "1. Download and check:"
      download "$file" "$checker"
      echo "2. Compare $(sum "$file") with the announcement, then run:"
      runs "$file" "$checker"
    done
    echo
    echo "== From source: the commit announced, not the tag's name, built with go build (go run stamps no commit) =="
    printf '%s\n' "$from_source_unix"
    echo
    echo "== From source, on Windows (PowerShell) =="
    printf '%s\n' "$from_source_windows"
  } > "$dist/COMMANDS.txt"

  {
    echo "**Before you run anything, compare with the ceremony's announcement.** This release's commit, \`$GITHUB_SHA\`, and each program's sha256 below must be the ones the organizers announced. These notes can be edited after the release is published; its files and its tag cannot."
    echo
    echo "The Enygma setup ceremony's participant program, built by this repository's \`ceremony-release\` workflow from tag \`$TAG\`. Run with no command, it joins the coordinator at $CEREMONY_URL and contributes. Leave it running until it says you are done, then publish the receipt it prints."
    echo
    echo "| Program | sha256 |"
    echo "| --- | --- |"
    for f in "${FILES[@]}"; do echo "| \`$f\` | \`$(sum "$f")\` |"; done
    echo
    echo "Copy the commands for your system from here, or from \`COMMANDS.txt\`, an attested file of this release, never from the coordinator. \`gh release verify-asset '$TAG' <file> --repo '$GITHUB_REPOSITORY'\` checks a downloaded file is this release's."
    for system in "${systems[@]}"; do
      IFS='|' read -r label file checker <<< "$system"
      lang='sh'
      [[ -z "$checker" ]] && { lang=powershell; label="$label (PowerShell)"; }
      echo
      echo "**$label.** Download and check:"
      echo
      echo "\`\`\`$lang"
      download "$file" "$checker"
      echo '```'
      echo
      echo "Compare \`$(sum "$file")\` with the announcement, then run:"
      echo
      echo "\`\`\`$lang"
      runs "$file" "$checker"
      echo '```'
    done
    echo
    echo "**Where a program came from:** \`gh attestation verify <file> --repo '$GITHUB_REPOSITORY' --signer-workflow '$workflow' --source-ref 'refs/tags/$TAG' --source-digest '$GITHUB_SHA' --deny-self-hosted-runners\` checks GitHub's runner built it, with this workflow, from that commit. It does not cover the repository variable the coordinator's address came from, nor what the commit's code does: people independent of the organizers review the commit and rebuild it as \`BUILD\` says, and the announcement names them."
    echo
    echo "**From source**, the strongest option: build the commit announced, not the tag's name, with \`go build\`, which stamps the commit into the program; it prints it, to compare with the announcement (a program built so has its own sha256: only the commit compares)."
    echo
    echo '```sh'
    printf '%s\n' "$from_source_unix"
    echo '```'
    echo
    echo "On Windows (PowerShell):"
    echo
    echo '```powershell'
    printf '%s\n' "$from_source_windows"
    echo '```'
  } > "$dist/notes.md"
}

# tag_commit prints the commit the tag names now, through annotated tags.
# GitHub's API not answering is not the tag moving: then it fails, saying
# so, and what state the release is in (stage).
tag_commit() {
  local stage=$1 sha type followed=0
  read -r sha type < <(gh api "repos/$GITHUB_REPOSITORY/git/ref/tags/$TAG" --jq '.object.sha + " " + .object.type')
  while [[ "$type" == tag && $followed -lt 5 ]]; do
    read -r sha type < <(gh api "repos/$GITHUB_REPOSITORY/git/tags/$sha" --jq '.object.sha + " " + .object.type')
    followed=$((followed + 1))
  done
  [[ "$sha" =~ ^[0-9a-f]{40}$ && "$type" == commit ]] ||
    die "GitHub's API did not say which commit $TAG names (after following $followed tags): $stage"
  echo "$sha"
}

publish() {
  local dist=$1
  check
  local at
  at=$(tag_commit "nothing is published; run the workflow again")
  [[ "$at" == "$GITHUB_SHA" ]] || die "$TAG names $at now, not $GITHUB_SHA, the commit built: nothing published"
  # The release is found by its tag, as gh finds it: another for the tag,
  # a draft an earlier run left say, could be the one published.
  local others
  others=$(gh api "repos/$GITHUB_REPOSITORY/releases" --paginate --jq ".[] | select(.tag_name == \"$TAG\") | .id") ||
    die "GitHub's API did not list the repository's releases: nothing is published; run the workflow again"
  [[ -z "$others" ]] ||
    die "a release of $TAG, or a draft of one, exists already: nothing published; delete a draft an earlier run left (gh release delete $TAG), and run the workflow again"
  # A draft first: a release takes its tag's commit when it is published,
  # so the tag is checked again before and after (audit M10).
  gh release create "$TAG" --repo "$GITHUB_REPOSITORY" --draft --verify-tag \
    --title "Enygma setup ceremony $TAG" --notes-file "$dist/notes.md" \
    "$dist"/ceremony-* "$dist/SHA256SUMS" "$dist/BUILD" "$dist/COMMANDS.txt" ||
    die "the draft release of $TAG was not made whole: a draft may be left, unpublished: delete it (gh release delete $TAG), then run the workflow again"
  at=$(tag_commit "a draft release of $TAG is left, unpublished: delete it (gh release delete $TAG), then run the workflow again")
  if [[ "$at" != "$GITHUB_SHA" ]]; then
    gh release delete "$TAG" --repo "$GITHUB_REPOSITORY" --yes ||
      die "$TAG moved to $at while the release was a draft, and the draft could not be deleted: delete it by hand; nothing published"
    die "$TAG moved to $at while the release was a draft: draft deleted, nothing published"
  fi
  gh release edit "$TAG" --repo "$GITHUB_REPOSITORY" --draft=false ||
    die "$TAG's draft release was not published: see where it stands (gh release view $TAG); a draft left unpublished is deleted with gh release delete $TAG"
  at=$(tag_commit "$TAG is published, but not checked: see that the tag still names $GITHUB_SHA, then run gh release verify $TAG")
  [[ "$at" == "$GITHUB_SHA" ]] ||
    die "$TAG moved to $at as the release was published: it names another commit than its files were built from; take it down, and find who moved the tag"
  # The release's attestation may take a moment to appear.
  local tries=6
  while ! gh release verify "$TAG" --repo "$GITHUB_REPOSITORY"; do
    ((--tries > 0)) || die "$TAG is published, but gh release verify finds no valid attestation for it: run it again by hand; if it still fails, check immutable releases are on (an immutable release cannot be released again under this tag), and say so in the announcement"
    sleep "${VERIFY_WAIT:-10}"
  done
}

case "${1:-}" in
check) check ;;
notes) [[ $# -eq 2 ]] || die "notes DIST"; notes "$2" ;;
publish) [[ $# -eq 2 ]] || die "publish DIST"; publish "$2" ;;
*) die "usage: ceremony_release.sh check | notes DIST | publish DIST" ;;
esac
