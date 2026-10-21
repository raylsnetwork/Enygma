# Glossary

**Statement layout**: the order of a circuit's public signals, as named fields (offset, size) with a total length. A circuit package derives its own from the circuit's public fields (every payments circuit: transfer, transfer-with-fee, USDr, withdraw, deposit and burn); `gnark-server/cmd/genlayout` renders the layouts into the Go package `enygma_payments/layout` and the Solidity library `StatementLayout`.

**Statement length**: a statement layout's total number of public signals (`layout.<Circuit>Len` in Go, `<CIRCUIT>_LEN` in Solidity). It is the fixed arity of the circuit's public-signal array wherever one is declared: the relayer's accepted request length, `IEnygma`'s proof structs, and the test helpers that build signal arrays.

**Statement layout field names** (transfer circuit): `Fingerprint` (k×k matrix, row-major), `PublicKeys`, `PreviousCommitments` and `TxCommitments` (one (X, Y) point per participant, X first), `BlockNumber`, `AnonymitySet`, `MessageTags`, `Nullifier`, `DomainId`. The Solidity names are the same words in SCREAMING_SNAKE_CASE.

**Statement layout field names** (transfer-with-fee circuit, prefix `TransferFee`): `HashedSharedSecrets`, then the transfer circuit's `PublicKeys` through `Nullifier`, then `FeeAmount` (the public protocol fee), `SumTxCommitments` (the (X, Y) sum of the tx commitments), `SumTxValuesWithFee`, `DomainId`.

**Statement layout field names** (USDr circuit, prefix `Usdr`): the transfer circuit's `Fingerprint` through `Nullifier`, then `FeeAmount` (the fixed USDr fee), `DomainId`, `FeeRecipientKey` (spend public key of the account paid the fee).

**Statement layout field names** (withdraw circuit, prefix `Withdraw`): the transfer-with-fee circuit's `HashedSharedSecrets` through `Nullifier`, then `TotalDepositValue` (Σ of the DvP deposit amounts), `DomainId`, `DepositCommitments` (the DvP note commitment each of the 10 deposit slots creates, 0 if unused). Its first `DepositLen` signals share the deposit circuit's layout everywhere except the field after `Nullifier` (`TotalDepositValue` here, `Hash` in deposit).

**Statement layout field names** (deposit circuit, prefix `Deposit`): `HashedSharedSecrets` through `Nullifier` as in withdraw, then `Hash` (the deposit note commitment), `DomainId`.

**Statement layout field names** (burn circuit, prefix `Burn`): `PublicKey` (the burned account's spend public key), `PreviousCommitment` and `NewCommitment` (one (X, Y) balance commitment each, X first), `Amount`, `BlockNumber`, `Nullifier`, `DomainId`.
