# Glossary

**Statement layout**: the order of a circuit's public signals, as named fields (offset, size) with a total length. A circuit package derives its own from the circuit's public fields (so far: the transfer, transfer-with-fee and USDr circuits); `gnark-server/cmd/genlayout` renders the layouts into the Go package `enygma_payments/layout` and the Solidity library `StatementLayout`.

**Statement layout field names** (transfer circuit): `Fingerprint` (k×k matrix, row-major), `PublicKeys`, `PreviousCommitments` and `TxCommitments` (one (X, Y) point per participant, X first), `BlockNumber`, `AnonymitySet`, `MessageTags`, `Nullifier`, `DomainId`. The Solidity names are the same words in SCREAMING_SNAKE_CASE.

**Statement layout field names** (transfer-with-fee circuit, prefix `TransferFee`): `HashedSharedSecrets`, then the transfer circuit's `PublicKeys` through `Nullifier`, then `FeeAmount` (the public protocol fee), `SumTxCommitments` (the (X, Y) sum of the tx commitments), `SumTxValuesWithFee`, `DomainId`.

**Statement layout field names** (USDr circuit, prefix `Usdr`): the transfer circuit's `Fingerprint` through `Nullifier`, then `FeeAmount` (the fixed USDr fee), `DomainId`, `FeeRecipientKey` (spend public key of the account paid the fee).
