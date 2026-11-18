# Coding standards

Rules for review that no check enforces. What `make check` and the pre-commit hooks already catch is not repeated here.

## All code

- **Comments state what the code does now**, and change with it: a comment that describes an earlier version of its code is a finding.

## Demo pages

The demo pages (`*.html`, such as `enygma_retail_payments/shield_demo/index.html`) carry their whole UI as inline CSS and script.

- **Colours are tokens.** Every colour is a custom property on `:root`, redefined for the dark theme; rules use `var(--…)`, never a literal colour.
- **Engine and service text is escaped.** Any text that comes from the engine or a service reaches the page through `esc()`, or as `textContent`, never interpolated raw into HTML.
- **Every function and event handler has a one-line comment** above it saying what it does, in the page's plain-English style; one comment may cover a run of related one-line definitions.
- **A security claim matches the deployed code.** Any text saying what a constraint or contract check stops names the mechanism by which the deployed circuit or contract stops it; a claim the code doesn't bear out, or a reason the code doesn't use, is a finding.
- **A security claim states its reach.** A bound says whom it holds against (an honest sender, any payer), and a stated weakness gives its widest reach, read from the deployed code.
- **A claim agrees with the page's other steps.** A statement about who holds or sees a value matches every other step that mentions that value. A pointer to another step ("step 9 shows …") names what that step shows as merged; a later ticket's work is not there yet.
- **The primer's "the run checked" matches a real assertion.** Text on the Protocol primer saying the run checked or found something matches an assertion in `TestProtocolPrimer`: a sentence with no such assertion is a finding.
- **A hand check follows the page it checks.** A page change that alters what a `HAND_CHECK.md` check describes (a narration line, a count, a label) updates that check in the same change, and a recorded run there that the change outdates says so. A check that still describes the old page is a finding.
- **Engine and service JSON is read for its real shape.** A list field may be absent (Go's `omitempty` drops an empty list), so lists are read as `x || []`. A service body with bare big numbers (the gnark server's field elements) goes through the hop panel's `body()`, which keeps them digit for digit, never plain `JSON.parse` into numbers.

## Tests and checks

- **A check never passes vacuously.** A check that reads its input itself (a workflow's YAML, a Makefile's variables, a tool's output) fails on a shape, syntax or key it can't read, rather than reading it as empty or absent, and that failure has a red like any guard. Its pass matches the outcome it tests and nothing a different outcome also prints (npm's `error code EBADENGINE`, not `EBADENGINE`, which its warning prints too). A reader that turns what it can't parse into "nothing to check" is a finding.

- **A new check is shown red.** A change that adds a test or check names the mutation of the code under test that makes it fail (in the commit message or PR), and says it was run: on a line with `RED`, as `mutant.sh` prints it, or a `No red: <why>` line, which the `red-named` commit hook (`scripts/check_red_named.sh`) checks on each commit, and CI's `red-named` job on each PR (its body or any of its commits). The mutated code still compiles: the check fails when it runs, not the build. The red comes from the new check itself, not from an earlier check failing first and stopping the run. `scripts/mutant.sh` runs the test command green as the code stands, applies one mutation, runs it again and restores the file; `--expect <regex>` makes it require the output to name the check under test. The reviewer leaves a PR's missing red to that job; when it looks for a merged commit's red, it reads the commit's PR body (`gh pr view N`), not only the squash message. In review of an uncommitted diff, list each new check and the mutation its commit must name, rather than reporting the message missing.
- **A new guard is shown red too.** A guard in new code (a wait, a refusal, a failure on a bad state) has a mutation that some check kills, even when no new test was written for it, and the change names it like a new check's red. The mutant brings back the failure the guard prevents, often by restoring the code from before the change, not just by deleting the guard's line: a survivor may be a mutant that changes nothing, so check that first. A guard whose real mutant survives is a finding: add or sharpen a test until it goes red, or drop the guard. The `red-named` hook sees only new tests, so this is the reviewer's call.
- **Shared state survives its other writers.** Code that writes or removes a pid file, lock or marker that another run or tool reads is checked against a second run of itself and against each other writer (a stop, a failed start, a stale-file cleanup). A run that leaves another run's state wrong (a second start overwriting the live run's pid file, then removing it when its own start fails) is a finding, and the fix gets a test with a named red.
- **Cleanup acts only on complete state.** A cleanup or EXIT trap set before the state it restores or removes is complete acts on that state only once it is whole: set the variable it reads after the save succeeds (`copy=$(mktemp); cp "$file" "$copy"; backup=$copy`). A cleanup that can restore a partial or empty save over the original is a finding.
