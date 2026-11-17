## Agent skills

### Issue tracker

Issues are tracked in GitHub Issues on `raylsnetwork/Enygma` (always pass `-R raylsnetwork/Enygma`). See `docs/agents/issue-tracker.md`. Ticket work starts on a new branch from a freshly fetched `origin/main`. A `ready-for-agent` ticket has no human to confirm with: what a skill asks the user to agree first (`/tdd`'s seams), pick, state in the session and proceed.

### Triage labels

Default vocabulary: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Shield demo changes

A page change is done when its section of `enygma_retail_payments/shield_demo/HAND_CHECK.md` passes in Chrome, following that file's Steps, after review and before the commit. An engine change, after review, runs `make shield-fixtures` and commits the replay fixtures it rewrites (that file's § What the replay test covers).

### New checks

**Show each new check red** with `scripts/mutant.sh` (CODING_STANDARDS.md § Tests and checks), and name the mutation in the commit.

### Domain docs

Single-context: one `GLOSSARY.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.
