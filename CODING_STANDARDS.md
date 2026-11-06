# Coding standards

Rules for review that no check enforces. What `make check` and the pre-commit hooks already catch is not repeated here.

## Demo pages

The demo pages (`*.html`, such as `enygma_retail_payments/shield_demo/index.html`) carry their whole UI as inline CSS and script.

- **Colours are tokens.** Every colour is a custom property on `:root`, redefined for the dark theme; rules use `var(--…)`, never a literal colour.
- **Engine and service text is escaped.** Any text that comes from the engine or a service reaches the page through `esc()`, or as `textContent`, never interpolated raw into HTML.
- **Every function and event handler has a one-line comment** above it saying what it does, in the page's plain-English style; one comment may cover a run of related one-line definitions.
- **Comments state what the code does now**, and change with it: a comment that describes an earlier version of its code is a finding.
- **A security claim matches the deployed code.** Any text saying what a constraint or contract check stops names the mechanism by which the deployed circuit or contract stops it; a claim the code doesn't bear out, or a reason the code doesn't use, is a finding.
- **Engine and service JSON is read for its real shape.** A list field may be absent (Go's `omitempty` drops an empty list), so lists are read as `x || []`. A service body with bare big numbers (the gnark server's field elements) goes through the hop panel's `body()`, which keeps them digit for digit, never plain `JSON.parse` into numbers.

## Tests and checks

- **A new check is shown red.** A change that adds a test or check names the mutation of the code under test that makes it fail (in the commit message or PR), and says it was run. The mutated code still compiles: the check fails when it runs, not the build. A commit or PR with a check that has no stated red is a finding. In review of an uncommitted diff, list each new check and the mutation its commit must name, rather than reporting the message missing.
