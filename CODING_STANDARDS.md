# Coding standards

Rules for review that no check enforces. What `make check` and the pre-commit hooks already catch is not repeated here.

## Demo pages

The demo pages (`*.html`, such as `enygma_retail_payments/shield_demo/index.html`) carry their whole UI as inline CSS and script.

- **Colours are tokens.** Every colour is a custom property on `:root`, redefined for the dark theme; rules use `var(--…)`, never a literal colour.
- **Engine and service text is escaped.** Any text that comes from the engine or a service reaches the page through `esc()`, or as `textContent`, never interpolated raw into HTML.
- **Every function and event handler has a one-line comment** above it saying what it does, in the page's plain-English style; one comment may cover a run of related one-line definitions.
- **Comments state what the code does now**, and change with it: a comment that describes an earlier version of its code is a finding.
- **A security claim matches the deployed code.** Any text saying what a constraint or contract check stops names the mechanism by which the deployed circuit or contract stops it; a claim the code doesn't bear out, or a reason the code doesn't use, is a finding.
- **A security claim states its reach.** A bound says whom it holds against (an honest sender, any payer), and a stated weakness gives its widest reach, read from the deployed code.
- **A claim agrees with the page's other steps.** A statement about who holds or sees a value matches every other step that mentions that value. A pointer to another step ("step 9 shows …") names what that step shows as merged; a later ticket's work is not there yet.
- **The primer's "the run checked" matches a real assertion.** Text on the Protocol primer saying the run checked or found something matches an assertion in `TestProtocolPrimer`: a sentence with no such assertion is a finding.
- **A hand check follows the page it checks.** A page change that alters what a `HAND_CHECK.md` check describes (a narration line, a count, a label) updates that check in the same change, and a recorded run there that the change outdates says so. A check that still describes the old page is a finding.
- **Engine and service JSON is read for its real shape.** A list field may be absent (Go's `omitempty` drops an empty list), so lists are read as `x || []`. A service body with bare big numbers (the gnark server's field elements) goes through the hop panel's `body()`, which keeps them digit for digit, never plain `JSON.parse` into numbers.

## Tests and checks

- **A new check is shown red.** A change that adds a test or check names the mutation of the code under test that makes it fail (in the commit message or PR), and says it was run. The mutated code still compiles: the check fails when it runs, not the build. The red comes from the new check itself, not from an earlier check failing first and stopping the run. `scripts/mutant.sh` runs the test command green as the code stands, applies one mutation, runs it again and restores the file; `--expect <regex>` makes it require the output to name the check under test. A commit or PR with a check that has no stated red is a finding. In review of an uncommitted diff, list each new check and the mutation its commit must name, rather than reporting the message missing.
