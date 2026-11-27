# A Phase 2 names the Phase 1 it builds on, and starts only from the SRS announced for it

A protocol's keys are sound if one contributor in Phase 1 and one in that protocol's Phase 2 kept their randomness secret. The second half holds only if the Phase 2 started from the SRS that *the* Phase 1, the one whose head and drand round were announced, seals to. gnark's keys take α, β and the public-input points from the SRS and fix γ to the generator, and a Phase 2 contribution rescales only δ and the commitment σs. So whoever knows α·β of the SRS a Phase 2 starts from can forge a proof for any public input, however many honest people contributed to that Phase 2 (audit H1).

Before this, a Phase 2 named its SRS by the sha256 of whatever file `-srs` named, and nothing tied that file to Phase 1. An SRS computed from chosen τ, α and β, or sealed from a private Phase 1 chain, passed `phase2-new`, `coordinate`, `verify -srs` and `phase2-seal`.

- **The SRS carries its provenance.** `phase1-seal` writes, in the SRS file itself, the Phase 1 head it sealed, how many contributions, the drand round and close the beacon came from (or that it came from no drand round: a rehearsal's), and the beacon. The SRS's sha256, which every Phase 2 state records and so every Phase 2 receipt names, then names that Phase 1 too. The file format's version is 2.
- **A Phase 2 starts only from the SRS announced.** Every command that takes `-srs` takes the sha256 announced for it, `-srs-sha256`, and the library compares it with the bytes it reads (`ceremony.SRS`). `verify` and `phase2-seal` can instead take `-phase1 DIR -phase1-head HEAD`: they check that the SRS says it was sealed up to the announced head, reseal Phase 1 from DIR as the SRS's provenance says, and use the SRS only if the reseal gives the same file. Without the head, any Phase 1 transcript would reseal to the SRS sealed from it. A rehearsal's SRS (no drand round) is refused unless `-rehearsal` says otherwise.
- **The provenance is shown.** `phase2-new`, `coordinate`, `verify`, `info`, `contribute` and `join` print the SRS's sha256 and the Phase 1 it names, the status page shows them, and `ceremony-seal.json` records them.

## Considered Options

- **A separate record (`phase1.srs.json`) every Phase 2 command reads.** Rejected: a record beside a file can be swapped apart from it, and no Phase 2 state would name it. Inside the SRS, the provenance is part of what every Phase 2 receipt names.
- **Only `-srs-sha256`.** Not enough alone: it binds a Phase 2 to the announced sha256, but a verifier still needs to know that sha256 is Phase 1's seal. `-phase1` makes that check one command, and the provenance makes the announced sha256 checkable against the announced head and round.
- **Resealing Phase 1 inside every Phase 2 command.** Rejected for `phase2-new` and `coordinate`: a reseal replays every Phase 1 contribution, which takes about a minute each at 2^18. The operator pins the announced sha256, which independent resealers confirmed before the Phase 2 opened; verifiers reseal.

## Consequences

- **A forged SRS can claim any provenance.** What makes the claim true is a reseal: an independent party reseals Phase 1 from its announced head, round and close and publishes the sha256 they got, before any Phase 2 opens, and verifiers use `-phase1` or that sha256.
- **State files of format 1 are refused,** Phase 1 and Phase 2 too: their header's version byte changed with the SRS's layout. No ceremony had run.
