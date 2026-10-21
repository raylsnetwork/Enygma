# Glossary

**Statement layout**: the order of a circuit's public signals, as named fields (offset, size) with a total length. A circuit package derives its own from the circuit's public fields (so far: the transfer circuit); `gnark-server/cmd/genlayout` renders the layouts into the Go package `enygma_payments/layout` and the Solidity library `StatementLayout`.

**Statement layout field names** (transfer circuit): `Fingerprint` (k×k matrix, row-major), `PublicKeys`, `PreviousCommitments` and `TxCommitments` (one (X, Y) point per participant, X first), `BlockNumber`, `AnonymitySet`, `MessageTags`, `Nullifier`, `DomainId`. The Solidity names are the same words in SCREAMING_SNAKE_CASE.
