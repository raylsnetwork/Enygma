#!/usr/bin/env bash
# Show a check red: run the test command once as the code stands, which
# must pass; then replace old with new in file (old must occur exactly
# once), run it again, restore the file whatever happens, and say whether
# the command went red. Run the whole test file rather than a name filter,
# so a renamed test can't drop out of the run unnoticed. With --expect, the
# red counts only if the output matches regex, naming the check under test
# (a subtest's name, say), so a red from an earlier check failing first
# doesn't pass for the new check's. regex matches one line of output: go
# test prints a test's name and its assertion on separate lines, so name
# one or the other.
# It refuses a file the stacks are wired from (suite_changes.sh --wiring)
# while a stack make started runs, up or still starting: a stack reads its
# scripts and helpers as it starts and runs (bash reads a script as it goes),
# so a mutant applied then could run inside the live stack and pass or fail
# it for the wrong reason.
# Each run of the command has a deadline, --timeout seconds (default 600):
# it runs in its own process group, as make's *-stack targets run a stack,
# and a run past the deadline is killed with everything it started, the
# file is restored and the verdict is HUNG. A node test awaiting a child
# that never exits fails on its own timeout, but node lives on with the
# child. A hang is never RED: a hung test is a test bug.
# Exit 0 when the mutant is killed (red), 1 when it survives, goes red
# elsewhere or hangs, 2 on bad input: a wiring file of a live stack, a
# command already red without the mutant or hung without it, or one that
# did not run (exit 126 or 127, as a command passed as one word).
# A mutant of mutant.sh itself (the same file, symlinks resolved), or of
# own_group.sh, which runs the test command, runs from a temporary copy of
# mutant.sh, suite_changes.sh and own_group.sh, with the same arguments: bash
# reads a script as it goes, so the run would otherwise pick up the mutated
# text partway through, and a mutated own_group.sh would change how the
# test command runs, not just the code under test. The copy mutates and
# restores the real file as usual and is removed on exit.
# The verdict (RED:, SURVIVED:, HUNG: or BAD INPUT:) is always the last
# line, after the failure lines it rests on, so `| tail` keeps it; a red
# elsewhere names the first failure on that line too.
# With --batch <list.json> it runs a list of mutants, for a change with
# several checks: a JSON list of entries, each
#   {"expect": <regex>, "file": <path>, "old": <text>, "new": <text>,
#    "cmd": [<test command's words>…]}
# (old and new may span lines, as "\n" in JSON). A list it cannot read, or
# an entry of another shape or naming no file, is bad input naming the
# entry, and nothing runs. Each entry runs as mutant.sh --expect <regex> runs it, with the
# batch's --timeout, and its file is restored after; but its green run is
# once per test command: an entry whose command (word for word) an earlier
# entry's green run passed skips its own, as every entry restores its file.
# A test command that changes the tree otherwise (writes a file) is then no
# longer caught by a later entry's green run. Then it prints each
# RED as the line a commit names it with,
#   - `<old>` → `<new>`: RED: "<regex>"
# (for one hunk, each text is on one line and cut past 60 characters;
# if the cut leaves the two the same, a window shows their first difference.
# For several hunks, the line names each change, separated by …). It also
# names each mutant that was not red, with the last line of its run (its verdict).
# The last line is RED: when every mutant was red (exit 0); else SURVIVED:
# (exit 1), or BAD INPUT: (exit 2)
# when a run was on bad input.
#
#   scripts/mutant.sh enygma_retail_payments/shield_demo/index.html \
#     "hold * 1.3)" "hold * 3)" -- node --test scripts/shield_demo_replay.test.js
#   scripts/mutant.sh --expect 'TestProtocolPrimer/audit' <file> <old> <new> -- go test …
#   scripts/mutant.sh <file> <old> <new> -- go -C enygma_retail_payments/shield_demo test ./engine/
#   scripts/mutant.sh --batch mutants.json
# Go runs inside its module (go -C <module dir>): from the repo root it
# finds no go.mod.
set -euo pipefail
usage() {
  echo "usage: mutant.sh [--expect <regex>] [--timeout <seconds>] <file> <old> <new> -- <test command…>" >&2
  echo "       mutant.sh [--timeout <seconds>] --batch <list.json>" >&2
  exit 2
}
args=("$@")
expect= timeout=600 batch=
while :; do
  case ${1:-} in
    --expect) [ $# -ge 2 ] || usage; expect=$2; shift 2 ;;
    --batch) [ $# -ge 2 ] || usage; batch=$2; shift 2 ;;
    --timeout)
      [ $# -ge 2 ] && [[ $2 =~ ^[1-9][0-9]*$ ]] || usage
      timeout=$2; shift 2 ;;
    *) break ;;
  esac
done

# --batch: each entry of the list runs as its own mutant.sh --expect, one
# after another, and the commit's RED lines follow. An entry may mutate
# mutant.sh itself: its run goes from a copy, as a single mutant's does,
# and restores the file before this run reads on.
if [ -n "$batch" ]; then
  [ $# -eq 0 ] && [ -z "$expect" ] || usage
  fields=$(mktemp) log=$(mktemp) mark=
  trap 'rm -f "$fields" "$log" ${mark:+"$mark"}' EXIT
  # The list is read whole before any entry runs: a shape this cannot read
  # is bad input, naming the entry, never an entry skipped. Each entry comes
  # out as NUL-ended fields: its shortened label, expect, file, old, new,
  # the command's word count and its words.
  python3 - "$batch" >"$fields" <<'PY' || exit 2
import difflib, json, os, re, sys
path = sys.argv[1]
def bad(msg):
    sys.exit(f"BAD INPUT: {path}: {msg}")
try:
    with open(path, encoding="utf-8") as f:
        entries = json.load(f)
except (OSError, ValueError) as e:
    bad(f"not a JSON list of mutants: {e}")
if not isinstance(entries, list) or not entries:
    bad("want a non-empty JSON list of mutants, each {expect, file, old, new, cmd}")
keys = ["expect", "file", "old", "new", "cmd"]
# A commit line quotes old and new on one line, cut past WIDTH characters,
# each in a code span that its own backticks can't end.
WIDTH, LEAD = 60, 20
def span(s):
    return f"`` {s} ``" if "`" in s else f"`{s}`"
def one_line(s):
    return re.sub(r"\s+", " ", s).strip()
def cut(s):
    return s[:WIDTH - 1].rstrip() + "…" if len(s) > WIDTH else s
# window is s from LEAD characters before diff_at, cut past WIDTH with … on
# each cut side.
def window(s, diff_at):
    start = max(0, diff_at - LEAD)
    text = s[start:start + WIDTH - 2]
    head = "…" + text.lstrip() if start > 0 else text
    return head.rstrip() + "…" if start + WIDTH - 2 < len(s) else head
# inline_label is the RED line's old → new. When the cut leaves the two the same
# (the change lies past it), each shows a window around their first
# differing character instead, so the line still says what was mutated.
def inline_label(old, new):
    old, new = one_line(old), one_line(new)
    if cut(old) != cut(new):
        return f"{span(cut(old))} → {span(cut(new))}"
    # One a prefix of the other: they differ where the shorter ends.
    diff_at = next((i for i, (a, b) in enumerate(zip(old, new)) if a != b), min(len(old), len(new)))
    return f"{span(window(old, diff_at))} → {span(window(new, diff_at))}"

# Name the separate edits without quoting all the unchanged lines between them.
def label(old, new):
    old_lines, new_lines = old.splitlines(keepends=True), new.splitlines(keepends=True)
    diff = difflib.SequenceMatcher(None, old_lines, new_lines, autojunk=False)
    edits = [op for op in diff.get_opcodes() if op[0] != "equal"]
    if len(edits) <= 1:
        return inline_label(old, new)

    changes = []
    for tag, i, j, k, l in edits:
        before = "".join(old_lines[i:j])
        after = "".join(new_lines[k:l])
        if tag == "delete":
            changes.append(f"{span(cut(one_line(before)))} (removed)")
        elif tag == "insert":
            changes.append(f"{span(cut(one_line(after)))} (added)")
        else:
            changes.append(inline_label(before, after))
    return " … ".join(changes)
out = []
for i, e in enumerate(entries, 1):
    name = f"entry {i}"
    if isinstance(e, dict) and isinstance(e.get("file"), str):
        name += f" ({e['file']})"
    if not isinstance(e, dict):
        bad(f"{name} is not an object {{expect, file, old, new, cmd}}")
    if set(e) != set(keys):
        missing = [k for k in keys if k not in e]
        extra = sorted(k for k in e if k not in keys)
        bad(f"{name}: " + "; ".join(
            ([f"no {', '.join(missing)}"] if missing else []) + ([f"unknown {', '.join(extra)}"] if extra else [])))
    for k in ["expect", "file", "old", "new"]:
        if not isinstance(e[k], str):
            bad(f"{name}: {k} is not a string")
    for k in ["expect", "file", "old"]:
        if not e[k]:
            bad(f"{name}: {k} is empty")
    if not os.path.isfile(e["file"]):
        bad(f"{name}: no such file")
    cmd = e["cmd"]
    if not isinstance(cmd, list) or not cmd or not all(isinstance(w, str) and w for w in cmd):
        bad(f"{name}: cmd is not a non-empty list of words")
    fields = [label(e["old"], e["new"]),
              e["expect"], e["file"], e["old"], e["new"], str(len(cmd)), *cmd]
    if any("\0" in f for f in fields):
        bad(f"{name}: holds a NUL character")
    out.extend(fields)
sys.stdout.write("".join(f + "\0" for f in out))
PY
  field=()
  while IFS= read -r -d '' x; do field+=("$x"); done <"$fields"
  i=0 n=0 nred=0 bad= reds=() misses=() passed=
  while [ $i -lt ${#field[@]} ]; do
    n=$((n + 1)) label=${field[i]} expect=${field[i + 1]} file=${field[i + 2]}
    old=${field[i + 3]} new=${field[i + 4]} words=${field[i + 5]}
    echo "== mutant $n: $file"
    # The green run: skipped for a command an earlier green run passed (a
    # line of passed); else the entry's mutant.sh writes to its own empty
    # mark once it passes.
    cmdkey=$(printf '%q ' "${field[@]:i + 6:words}")
    if printf '%s\n' "$passed" | grep -qxF -- "$cmdkey"; then
      green=(MUTANT_SH_GREEN_SKIP=1)
    else
      mark=$(mktemp)
      green=(MUTANT_SH_GREEN_MARK="$mark")
    fi
    rc=0
    env "${green[@]}" bash "$0" --expect "$expect" --timeout "$timeout" "$file" "$old" "$new" \
      -- "${field[@]:i + 6:words}" 2>&1 | tee "$log" || rc=$?
    if [ -n "$mark" ]; then
      [ ! -s "$mark" ] || passed+=$cmdkey$'\n'
      rm -f "$mark"
      mark=
    fi
    if [ $rc = 0 ]; then
      nred=$((nred + 1))
      reds+=("- $label: RED: \"$expect\"")
    else
      [ $rc != 2 ] || bad=1
      misses+=("mutant $n, $label (/$expect/): $(tail -1 "$log")")
    fi
    i=$((i + 6 + words))
  done
  echo
  if [ $nred -gt 0 ]; then
    echo "The commit's RED lines:"
    printf '%s\n' "${reds[@]}"
  fi
  if [ $nred = $n ]; then
    echo "RED: all $n mutants"
    exit 0
  fi
  echo "Not red, so no RED line:"
  printf '  %s\n' "${misses[@]}"
  if [ -n "$bad" ]; then
    echo "BAD INPUT: $((n - nred)) of $n mutants not red, one or more on bad input"
    exit 2
  fi
  echo "SURVIVED: $((n - nred)) of $n mutants not red"
  exit 1
fi
[ $# -ge 5 ] && [ "$4" = -- ] || usage
file=$1 old=$2 new=$3
shift 4

# cleanup runs on exit (not across the exec below): it kills the run in
# progress, if any (pgid, its process group), so no test runs on against
# the restored file; restores the file once it is backed up; and removes
# the copy mutant.sh runs from, if it runs from one.
pgid= backup= out=
cleanup() {
  [ -z "$pgid" ] || kill -KILL -- "-$pgid" 2>/dev/null
  [ -z "$backup" ] || cp "$backup" "$file"
  rm -f "$backup" "$out"
  [ -z "$copy_dir" ] || rm -rf -- "$copy_dir"
}
# The copy is told the real repo root and its own dir to remove, and
# unsets both, so no mutant.sh the test command runs inherits them.
root=${MUTANT_SH_ROOT:-$(cd "$(dirname "$0")/.." && pwd -P)}
copy_dir=${MUTANT_SH_COPY_DIR:-}
# From --batch: skip the green run (its command passed one already), or
# write to green_mark once it passes. Unset too, so no test command sees them;
# a self-mutant's copy, below, gets neither and runs its own green run.
green_skip=${MUTANT_SH_GREEN_SKIP:-} green_mark=${MUTANT_SH_GREEN_MARK:-}
unset MUTANT_SH_ROOT MUTANT_SH_COPY_DIR MUTANT_SH_GREEN_SKIP MUTANT_SH_GREEN_MARK
trap cleanup EXIT
if [ "$file" -ef "$0" ] || [ "$file" -ef "$(dirname "$0")/own_group.sh" ]; then
  copy_dir=$(mktemp -d)
  mkdir "$copy_dir/scripts"
  cp "$0" "$(dirname "$0")/suite_changes.sh" "$(dirname "$0")/own_group.sh" "$copy_dir/scripts/"
  MUTANT_SH_ROOT=$root MUTANT_SH_COPY_DIR=$copy_dir exec bash "$copy_dir/scripts/mutant.sh" "${args[@]}"
fi

# A wiring file while a stack make started runs: its pid file (make's
# *-stack target writes enygma_<x>_stack.pid as it starts the stack) names
# a live process. A stale pid file, whose process is gone, blocks nothing,
# nor does no pid file (the glob then stays literal and cat fails).
abs=$(cd "$(dirname "$file")" && pwd -P)/$(basename "$file")
# The list goes in a variable first: grep -q on the pipe could exit before
# suite_changes.sh had written it all, and pipefail would read that SIGPIPE
# as no match.
wiring=$(bash "$(dirname "$0")/suite_changes.sh" --wiring)
if grep -qxF -- "${abs#"$root"/}" <<<"$wiring"; then
  for pid_file in "${TMPDIR:-/tmp}"/enygma_*_stack.pid; do
    pid=$(cat "$pid_file" 2>/dev/null) || continue
    if kill -0 "$pid" 2>/dev/null; then
      stack=${pid_file##*/enygma_}
      echo "BAD INPUT: $file wires the stacks, and the stack in $pid_file (pid $pid) is running, so the mutant would run inside it: stop it first (make ${stack%_stack.pid}-stack-stop)"
      exit 2
    fi
  done
fi
# failures picks the lines a red rests on: node's, go test's (its FAIL
# lines and its indented file_test.go:N: assertions) and errors.
failures='^not ok|^ *(error|Error|--- FAIL|FAIL)|^ +[^ ]+_test\.go:[0-9]+:'
out=$(mktemp)
# backup is set only once the copy is whole, so cleanup never restores a
# partial one.
copy=$(mktemp)
cp "$file" "$copy" || { rm -f "$copy"; exit 1; }
backup=$copy

# run <name> <command…> runs the test command, output to $out, in its own
# process group (own_group.sh), and sets status to its exit code. Then it
# kills the group, so nothing the command left behind runs on. Past the
# deadline it ends mutant.sh: HUNG, exit 1 for the mutant run, 2 for the
# green run.
# own_group.sh runs the command as the shell would, never through one (a
# command passed as one word is not split by /bin/sh), and exits 127 when it
# is not found, 126 when it can't run. It runs through bash: a copy of it
# (the self-mutant copy, a test's scratch repo) need not keep its mode.
run() {
  local name=$1
  shift
  bash "$(dirname "$0")/own_group.sh" "$@" >"$out" 2>&1 </dev/null &
  pgid=$!
  # SECONDS counts whole seconds, so the + 1 keeps a run from being cut
  # before timeout seconds have passed.
  local deadline=$((SECONDS + timeout + 1))
  # bash reaps the background child as it exits, so kill -0 then fails.
  while kill -0 "$pgid" 2>/dev/null && [ $SECONDS -lt $deadline ]; do sleep 0.1; done
  local hung=
  if kill -0 "$pgid" 2>/dev/null; then
    hung=1
    # Disowned, bash reports no "Killed" for it; its status goes unused.
    disown "$pgid"
  fi
  kill -KILL -- "-$pgid" 2>/dev/null || true
  if [ -n "$hung" ]; then
    pgid=
    tail -8 "$out"
    echo "HUNG: the $name run passed $timeout s"
    [ "$name" = mutant ] && exit 1
    exit 2
  fi
  status=0
  wait "$pgid" || status=$?
  pgid=
}

# The command as the code stands: green, or there is nothing to show red.
if [ -n "$green_skip" ]; then
  echo "(green run: passed already, with this command)"
else
  run green "$@"
  if [ $status != 0 ]; then
    tail -8 "$out"
    echo "BAD INPUT: the command fails without the mutant, so a red would show nothing"
    exit 2
  fi
  [ -z "$green_mark" ] || echo passed >"$green_mark"
fi

python3 - "$file" "$old" "$new" <<'PY' || exit 2
import sys
path, old, new = sys.argv[1:]
text = open(path, encoding="utf-8").read()
n = text.count(old)
if n != 1:
    sys.exit(f"mutant.sh: the old text occurs {n} times in {path}, want exactly once")
open(path, "w", encoding="utf-8").write(text.replace(old, new))
PY

run mutant "$@"
if [ $status = 0 ]; then
  echo "SURVIVED: the command passed with the mutant in place"
  exit 1
fi
if [ $status = 126 ] || [ $status = 127 ]; then
  tail -4 "$out"
  echo "BAD INPUT: the command did not run with the mutant in place (exit $status)"
  exit 2
fi
if [ -n "$expect" ] && ! grep -Eq -- "$expect" "$out"; then
  grep -E "$failures" "$out" | head -8 || tail -8 "$out"
  # The first failure goes on the verdict line too, so a regex that just
  # misses the check's name (a path before it, say) shows in a tail.
  first=$(grep -E -m1 "$failures" "$out" | sed 's/^ *//') || first=$(grep -m1 . "$out") || first="(no output)"
  echo "SURVIVED: the command went red, but not from the expected check (/$expect/); first failure: $first"
  exit 1
fi
grep -E "$failures" "$out" | head -8 || tail -8 "$out"
echo "RED: the command failed with the mutant in place"
