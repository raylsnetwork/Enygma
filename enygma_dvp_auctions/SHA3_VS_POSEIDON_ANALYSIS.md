# Replacing Poseidon with SHA3 in Enygma DvP Auctions — Impact Analysis & Migration Notes

Branch: `feat/poseidon_to_hash3` · Date: 2026-09-30

Companion to the `enygma_payments`, `enygma_retail_payments` and `enygma_dvp` reports. The
Poseidon → SHA3 change is planned, so this report covers both the cost and **how to migrate
the auction protocol with the least damage**.

## TL;DR

- **Use Keccak-256 as the SHA3 variant.** It is the hash the EVM computes natively
  (`keccak256` opcode). If the goal is alignment with Ethereum, this is the matching hash:
  the EVM has no opcode or precompile for FIPS 202 SHA3-256/384/512. Keccak-256 costs the
  same in-circuit as SHA3-256, is the only option whose on-chain Merkle hashing is cheaper
  than Poseidon, and has the same security. Use SHA3-256 only if FIPS 202 is mandated.
  Never use SHA3-384/512 here: +18–91 % constraints and no usable security gain.
- **The Batch circuit is the blocker.** It hashes 100 bid commitments in one proof: 159 k
  constraints today, **6.1 M with SHA3-256/Keccak (38×)** and **11.7 M with SHA3-384/512
  (73×)**. Compiling the SHA3-384 Batch alone peaked at 14.7 GB of RAM, so full Groth16
  setup and proving were not possible on the 16 GB test machine. `AuctionBatchSize` (and
  with it `AuctionFinalBatches`) must be re-dimensioned as part of the migration.
- **Every other circuit goes from milliseconds to seconds.** Bids prove in ≈7 s instead of
  ≈47 ms, with ≈260 MB proving keys and ≈3.5 GB prover memory.
- **Gas per 100-bid auction:** Poseidon ≈99.0 M, **Keccak-256 ≈98.5 M (−0.6 %)**, SHA3-256
  ≈196 M (+98 %), SHA3-384 ≈195 M, SHA3-512 ≈194 M.
- **Pre-existing bug found (independent of the hash):** `CHALLENGE_VERIFY_GAS = 750_000` is
  too low for the real Final-proof verification path. A challenger who sends between
  771 k and 882 k gas passes the check, but the *valid* proof runs out of gas inside
  `try` and returns `false`, which would void a valid settlement. SHA3 would widen this
  by ≈124 k gas. See "Challenge-gas finding" below.

## Measured results per circuit

**Method.** Each `templates.Auction*Circuit` was copied with every Poseidon call routed
through a swappable hash gadget (Poseidon, or gnark `std/hash/sha3` `New256`/`New384`/
`New512`/`NewLegacyKeccak256`; field elements serialized as 32 bytes big-endian, digest read
big-endian mod the BN254 field). With Poseidon every copy compiles to exactly the real
circuit's constraint count. Valid witnesses were built natively: a Lock of NFT #7, a 500-USDC
bid, a full 100-bid Batch, and a Final over 3 active batches of 10. Each variant ran
`groth16.Setup`, `Prove` (5 runs for Poseidon, 3 for SHA3) and 20× `Verify`. Hardware: 8-core / 16 GB Mac.

| Circuit (what it proves) | Poseidon | SHA3-256 | Keccak-256 | SHA3-384 | SHA3-512 |
|---|---|---|---|---|---|
| **Lock** (seller locks NFT) | 3,501 | 933,420 (267×) | 933,420 | 1,100,722 | 1,100,176 |
| **Bid** (bidder locks USDC) | 5,637 | 939,281 (167×) | 939,281 | 1,162,500 | 1,161,922 |
| **Batch** (winner of 100 bids) | 159,468 | 6,110,877 (38×) | 6,110,877 | 11,699,393 (73×) | 11,692,972 |
| **Final** (winner of ≤10 batches) | 15,520 | 262,925 (17×) | 262,925 | 374,695 | 374,567 |
| **Withdraw** (losing bid refund) | 512 | 246,725 | 246,725 | 302,578 | 302,482 |
| **Revert** (cancel auction) | 1,019 | 360,536 | 360,536 | 472,242 | 472,049 |

| Proof time (mean) / setup / proving key | Poseidon | SHA3-256 | Keccak-256 | SHA3-384 | SHA3-512 |
|---|---|---|---|---|---|
| Lock | 48 ms / 0.3 s / 0.6 MB | 6.9 s / 2 m 55 s / 257 MB | 8.2 s / 2 m 47 s / 257 MB | 10.1 s / 3 m 23 s / 331 MB | 9.0 s / 3 m 12 s / 331 MB |
| Bid | 47 ms / 0.5 s / 1.0 MB | 7.0 s / 2 m 40 s / 258 MB | 7.0 s / 2 m 44 s / 258 MB | 11.0 s / 3 m 23 s / 345 MB | 9.8 s / 3 m 15 s / 345 MB |
| Batch | 0.64 s / 11.8 s / 27 MB | ≈46 s / ≈17 min / ≈1.7 GB ‡ | same as SHA3-256 ‡ | ≈88 s / ≈33 min / ≈3.2 GB ‡ | same as SHA3-384 ‡ |
| Final | 46 ms / 1.1 s / 2.3 MB | 2.1 s / 49 s / 76 MB | 2.0 s / 41 s / 76 MB | 2.5 s / 60 s / 104 MB | 2.6 s / 60 s / 103 MB |
| Withdraw | 9 ms / 0.06 s / 0.08 MB | 1.8 s / 41 s / 66 MB | 1.6 s / 44 s / 66 MB | 2.5 s / 59 s / 88 MB | 2.4 s / 53 s / 88 MB |
| Revert | 15 ms / 0.09 s / 0.17 MB | 2.9 s / 64 s / 102 MB | 2.7 s / 67 s / 102 MB | 3.2 s / 78 s / 129 MB | 3.2 s / 77 s / 129 MB |

‡ Batch SHA3 constraint counts are exact (compiled). Setup, proof time and key size are
**linear extrapolations** from the measured Bid circuit (≈170 s setup, ≈7.5 s proving and
≈274 MB of proving key per million constraints). Prover memory would be roughly 23 GB
(SHA3-256) and 43 GB (SHA3-384/512); compiling alone peaked at 6.6 GB and 14.7 GB. The real
figures are likely somewhat worse, because the FFT domain doubles at 2²³ and 2²⁴ constraints.

Off-chain verification is ≈0.9 ms (Poseidon) vs ≈1.8 ms (any SHA3) for every circuit.
Proof-time differences between SHA3-256 and Keccak-256 are run-to-run noise: they have
identical constraint counts.

**Why SHA3-384 and SHA3-512 cost the same here:** every expensive hash is a 4-input,
128-byte commitment (`Erc20CommitmentV2`, and `Erc721Commitment`, which is the same hash with
amount = 1). That needs 1 Keccak permutation under SHA3-256 (136-byte rate) but 2 under
SHA3-384 (104) and SHA3-512 (72). The auction nullifier has only 2 inputs (64 bytes), so
SHA3-512 needs no permutation beyond SHA3-384.

## On-chain gas

**Verification: the real deployed path.** `EnygmaAuction` verifies through `Verifier.sol`,
which keeps the VK in storage and forwards it to `GenericGroth16Verifier.sol`. Each proof
above was generated with the project's real `gnark_circuits/scripts/keys/*.pk` (all six
verify, so the keys match the current templates) and verified on-chain through those two
contracts compiled from the repo (solc 0.8.37, via-IR, runs = 200 as in `hardhat.config.js`,
go-ethereum simulated backend). Transaction-level gas:

| Proof | Public inputs | Real stack (Poseidon) | gnark verifier, Poseidon | gnark verifier, SHA3 | SHA3 overhead |
|---|---|---|---|---|---|
| Lock | 7 | 359,248 | 257,977 | 381,867 | +123,890 |
| Bid | 8 | 372,894 | 265,411 | 389,194 | +123,783 |
| Batch | 104 | 1,650,930 | 944,386 † | not deployable † | ≈+124 k (est.) |
| Final | 38 | 761,116 | 465,616 | 589,506 | +123,890 |
| Withdraw | 2 | 293,781 | 223,736 | 347,591 | +123,855 |
| Revert | 4 | 319,891 | 237,373 | 361,195 | +123,822 |

The SHA3 overhead is the same for every circuit (and for SHA3-256, Keccak-256, SHA3-384 and
SHA3-512): gnark's SHA3 gadget uses lookup tables, so every proof carries a Pedersen commitment
and the verifier runs an extra commitment check. **The real `GenericGroth16Verifier` cannot
check that commitment**, so SHA3 proofs will not verify through the current `Verifier.sol`
at all; the verifier must be extended (see migration plan). SHA3 gas on the real stack is
therefore estimated as the real-stack figure + ≈124 k.

† Batch's gnark-exported verifier is 24,930 bytes of runtime code at the project's optimizer
setting, over EIP-170's 24,576-byte limit. It fits only when compiled for size (runs = 1:
23,490 bytes; gas measured that way). SHA3 adds ≈2.1 KB of verifier code, which puts a
standalone SHA3 Batch verifier over the limit even at runs = 1. The storage-VK design
(`Verifier.sol`) doesn't have this problem, which is another reason to extend it rather
than switch to per-circuit gnark verifiers.

**Merkle inserts.** `AuctionCoinVault._insertLeaves`/`_hash` is the same algorithm (through
the same `PoseidonWrapper`) as `enygma_dvp`'s `Merkle.sol`, so the per-hash numbers measured
there apply directly. Single-leaf insert (tx gas, depth 8): Poseidon 366,186 ·
**Keccak-256 114,275** · SHA3-256 ≈1,082,000 · SHA3-384 ≈1,071,700 · SHA3-512 ≈1,061,100
(NIST SHA3 via inline-assembly Keccak-f; no EVM opcode or precompile exists for it).

**Per auction (hash-dependent gas).** Happy path, unchallenged, N = 100 bids (1 batch):
Lock + 100 Bids + 1 Batch + settlement (2 single-leaf inserts: NFT to winner, USDC to seller)
+ 99 losing bidders' Withdraws (verify + 1 insert each). Each component is counted as
measured gas minus the 21,000 intrinsic charge; SHA3 verification is the real stack + 124 k.

| | Poseidon | Keccak-256 | SHA3-256 | SHA3-384 | SHA3-512 |
|---|---|---|---|---|---|
| Per bidder (Bid + refund of a losing bid) | ≈0.97 M | ≈0.97 M | ≈1.93 M | ≈1.92 M | ≈1.91 M |
| **100-bid auction total** | **≈99.0 M** | **≈98.5 M (−0.6 %)** | ≈196 M (+98 %) | ≈195 M | ≈194 M |

With Keccak-256 the cheaper Merkle inserts (≈−252 k per insert) almost exactly cancel the
extra verification cost (+124 k per proof), so gas is flat. With NIST SHA3 every insert costs
≈+716 k and the auction roughly doubles in gas.

## Challenge-gas finding (pre-existing; worse with SHA3)

`challengeSettlement` requires `gasleft() >= CHALLENGE_VERIFY_GAS` (750,000) and then calls
`this.verifyFinalProof`, which reads the 38-word statement and the proof from storage and
calls `IVerifier.verifyProof`, which in turn calls `GenericGroth16Verifier.verify`. Its comment
assumes the pairing check costs "~200k–400k", but the real Final verification costs
761,116 gas at transaction level (38 public inputs, VK read from storage). The call is three
CALL boundaries deep, and each boundary keeps back 1/64 of the gas.

**Reproduced** with a contract that copies this call pattern (same storage reads, same
`try/catch`, same 750 k floor) against the real `Verifier.sol` + `GenericGroth16Verifier`
and a valid Final proof made with the project's real keys:

- The smallest transaction gas that passes the floor is **771,305**, and at that gas the
  valid proof returns **`false`** (out of gas, caught by `catch`).
- The smallest transaction gas at which the valid proof returns `true` is **882,013**.
- So any challenge sent with **771 k–882 k gas voids a valid settlement** and reopens
  bidding. That is exactly the griefing the constant was meant to prevent.

This reproduces the pattern, not the full `EnygmaAuction` deployment, so it should be
confirmed with a full integration test. The fix is to derive the constant from the measured
cost: at least ≈860 k of `gasleft()` today plus a safety margin, and ≈124 k more for a SHA3
verifier. It was not changed as part of this analysis.

## Security

- **Collision resistance is the same for every SHA3 variant.** Digests are reduced into the
  ~254-bit BN254 field, so collision resistance is ≈127 bits whichever variant is used.
  BN254 Groth16 itself gives ~100–110 bits, so the hash is not the weakest link.
- **SHA3-256 and Keccak-256 are equally strong.** They use the same Keccak-f permutation and
  512-bit capacity and differ only in padding. SHA3-384/512's extra output is discarded by
  the field reduction.
- Auction hash outputs are commitments (bid, payout, NFT, revert), nullifiers, the auction ID
  (`H(commitLocked)`), Merkle nodes and public keys. What matters for them is binding and
  collision resistance, which every variant provides; output uniformity is not needed.
- The contract already uses `keccak256` for the ciphertext hash (`StCtxtHash`), so Keccak-256
  would unify the protocol on one hash.

## Migration plan (recommended order)

1. **Fix `CHALLENGE_VERIFY_GAS` now**, independent of the hash migration, and re-derive it
   after the verifier changes, from measurement rather than an estimate.
2. **Choose Keccak-256** (`NewLegacyKeccak256` in-circuit, the `keccak256` opcode
   on-chain), unless a regulator or customer requires FIPS 202, in which case use SHA3-256.
3. **Re-dimension the Batch/Final split.** The prover cost is dominated by 100 commitment
   hashes per Batch proof. Options:
   - Lower `AuctionBatchSize`. With Keccak each bid costs ≈56 k constraints, so 20 bids ≈
     1.25 M constraints (≈9 s proving, ≈4.5 GB), comparable to a Bid proof. Capacity is
     `AuctionBatchSize × AuctionFinalBatches`, so raise `AuctionFinalBatches` to keep the
     same maximum auction size. Final grows only in public inputs (3 per batch), which costs
     verification gas (≈13 k per input on the real stack, measured from Lock → Final → Batch)
     but no hashing.
   - Or keep 100 per batch and provision prover machines with ≥32 GB (SHA3-256/Keccak).
4. **Extend the on-chain verifier for gnark commitments.** Add Pedersen-commitment
   verification (commitments, proof of knowledge, and the hash-to-field of public plus
   committed inputs) to `GenericGroth16Verifier`/`Verifier.sol`, and export the commitment
   key alongside the VK. Keep the storage-VK design; standalone per-circuit gnark verifiers
   hit the EIP-170 size limit for Batch.
5. **Swap `PoseidonWrapper` for `keccak256`** in `AuctionCoinVault._hash`, so the on-chain
   Merkle tree matches the circuits' Merkle hash.
6. **Regenerate everything derived from the hash.** Keys (`cmd/keygen`), exported VKs,
   `src/` client code (commitments, nullifiers, auction ID, Merkle mirror), relayer, tests.
7. **Plan the note migration.** Existing notes and Merkle roots are Poseidon-based and cannot
   be re-hashed. Deploy new vaults/trees, and give users a window to exit the old ones and
   re-deposit (or run a one-time migration circuit that proves ownership of an old note and
   mints the new one).
8. **Size the prover infrastructure.** Every SHA3 proof except Final, Withdraw and Revert
   needs ≈3–5 GB and ≈7–11 s. A GPU prover (ICICLE, already an indirect dependency of
   `enygma_payments`) and caching the compiled constraint system (instead of recompiling
   per request) are the two largest levers.

## Reproducibility

The benchmark code (circuit copies, witness builders, swappable hash gadget, real-key proof
dump, Solidity challenge reproduction, go-ethereum harnesses) was temporary and is not in the
repo; the working tree contains only this document.
