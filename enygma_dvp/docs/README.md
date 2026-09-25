# Enygma DvP — Documentation

This directory contains documentation for the Enygma DvP system.

---

## Integration Tests

All integration tests live in `test/`. They require a running Hardhat node and gnark server.
Run with:

```bash
cd test && CC=/usr/bin/clang go test . -v -timeout 600s
```

| File | Test | Flow | Description |
|------|------|------|-------------|
| `01_v2_erc20_private_mint_test.go` | `TestV2Erc20OnChain_PrivateMint` | [diagram](./flows/01_private_mint.md) | Alice privately mints 100 USDT into the ERC20 vault without a public deposit. Proves a V2 commitment on-chain via the hardcoded `PrivateMintVerifier`. Alice then scans the emitted `PrivateMint` event and confirms the note is spendable. |
| `02_v2_erc20_payment_test.go` | `TestV2Erc20Payment` | [diagram](./flows/02_erc20_payment.md) | Alice deposits 40 USDT, then pays 30 USDT to Bob and receives 10 USDT change. Uses the Payment JoinSplit circuit (2 inputs, 2 outputs). Bob and Alice both scan the on-chain events and decrypt their respective notes via ML-KEM. |
| `03_v2_dvp_test.go` | `TestV2DvP` | [diagram](./flows/03_dvp.md) | Alice and Bob perform an atomic DvP swap: Alice delivers 30 USDT (ERC20), Bob delivers an ERC721 ticket (tokenId=42). Uses the DvPInitiator + DvPDestination circuits. Bob scans Alice's encrypted output to confirm he receives the USDT note. |
| `04_v2_dvp_deadline_test.go` | `TestV2DvP_WithDeadline` | [diagram](./flows/04_dvp_deadline.md) | Three sub-scenarios for the deadline-protected DvP swap (Alice 50 USDT ↔ Bob ERC721): |
| | `→ FullSwap` | | Happy path — both legs submitted before deadline, swap settles atomically. |
| | `→ DeadlineExpired` | | Bob never responds. Hardhat time advances past deadline. Alice calls `claimSwapTimeout`; revert commitment is recoverable. |
| | `→ InvalidProof` | | Bob submits a zeroed (invalid) proof — rejected on-chain. Deadline passes. Alice reclaims via `claimSwapTimeout`. |

---

## Unit Tests

Unit tests live alongside the integration tests in `test/` (no server required):

| File | Covers |
|------|--------|
| `merkle_test.go` | Incremental Merkle tree: insert, prove, root computation |
| `utils_test.go` | Crypto primitives: Poseidon, nullifier, commitment, BabyJubJub, ML-KEM, key derivation |

---

## Common Primitives

All flows share the same cryptographic building blocks:

| Primitive | Purpose | Implementation |
|-----------|---------|----------------|
| **Poseidon hash** | Commitment and nullifier computation | `src/core/utils.go` |
| **ML-KEM-768** | Non-interactive note delivery (encapsulate / decapsulate) | `src/core/utils.go` |
| **ChaCha20-Poly1305** | Encrypt tokenId and amount inside encrypted note payload | `src/core/utils.go` |
| **Groth16** | ZK proof generation and on-chain verification | `gnark_circuits/server/circuits/` |
| **Incremental Merkle tree** | On-chain membership proofs (depth 8) | `contracts/core/` |

### Commitment formula

```
commitment = Poseidon4(pk_spend, saltB, amount, tokenId)
```

### Nullifier formula

```
nullifier = Poseidon2(sk_spend, leafIndex)
```

A nullifier is unique per note and per owner. Publishing it on-chain marks the note as spent
without revealing which commitment it corresponds to.

---

## Key Source Files

| Path | Purpose |
|------|---------|
| `src/core/prover.go` | `GnarkClient`, `ProofResult`, shared types, HTTP transport |
| `src/core/prover_erc.go` | Typed provers: ERC20, ERC721, ERC1155, Payment, PrivateMint |
| `src/core/prover_auction.go` | Typed provers: Auction circuits, DvP circuits |
| `src/core/utils.go` | Key derivation, ML-KEM, Poseidon, Merkle helpers |
| `gnark_circuits/server/circuits/` | gnark circuit definitions and HTTP handlers |
| `contracts/core/contracts/` | Solidity: `EnygmaDvp.sol`, `PrivateMintVerifier.sol`, vaults |
| `scripts/cmd/deploy` | Deploy all contracts; writes `build/receipts.json` |
| `scripts/cmd/init` | Register VKs, coin vaults, asset groups |
