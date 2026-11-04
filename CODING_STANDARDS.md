# Coding standards

Rules for review that no check enforces. What `make check` and the pre-commit hooks already catch is not repeated here.

## Demo pages

The demo pages (`*.html`, such as `enygma_retail_payments/shield_demo/index.html`) carry their whole UI as inline CSS and script.

- **Colours are tokens.** Every colour is a custom property on `:root`, redefined for the dark theme; rules use `var(--…)`, never a literal colour.
- **Engine and service text is escaped.** Any text that comes from the engine or a service reaches the page through `esc()`, or as `textContent`, never interpolated raw into HTML.
- **Every function and event handler has a one-line comment** above it saying what it does, in the page's plain-English style.
- **Comments state what the code does now**, and change with it: a comment that describes an earlier version of its code is a finding.
- **Engine and service JSON is read for its real shape.** A list field may be absent (Go's `omitempty` drops an empty list), so lists are read as `x || []`. A service body with bare big numbers (the gnark server's field elements) goes through the hop panel's `body()`, which keeps them digit for digit, never plain `JSON.parse` into numbers.
