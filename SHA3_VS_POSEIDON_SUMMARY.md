# Poseidon → SHA3 Across Enygma — Compiled Comparison

Branch: `feat/poseidon_to_hash3` · Dates: 2026-09-29 / 30

Consolidates the four per-protocol reports:

- `enygma_payments/SHA3_VS_POSEIDON_ANALYSIS.md`: transfer
- `enygma_retail_payments/SHA3_VS_POSEIDON_ANALYSIS.md`: payment
- `enygma_dvp/SHA3_VS_POSEIDON_ANALYSIS.md`: two-proof swap
- `enygma_dvp_auctions/SHA3_VS_POSEIDON_ANALYSIS.md`: auction lifecycle and migration plan

## 1. Decision summary

| | Recommendation |
|---|---|
| **Which SHA3-family hash** | **Keccak-256** (Ethereum's `keccak256`) for every protocol. It is the only one the EVM computes natively, it costs the same in-circuit as SHA3-256, and it has the same security. Use **SHA3-256** only if FIPS 202 is a hard requirement. |
| **SHA3-384 / SHA3-512** | Do not use: 0–91 % more constraints depending on the circuit, and no security gain once the output is reduced into the BN254 field. (Narrow exception: enygma_payments blinding factors; see §5.) |
| **Proving cost** | Every protocol's proofs get **~30–270× more constraints** and go from **milliseconds to seconds** (0.05–0.5 s → 6–25 s), with **0.25–1 GB proving keys** and **3–11 GB prover memory** per proof. This is the real price of the migration. |
| **On-chain gas** | With Keccak-256, gas is **flat or lower** in every protocol. With NIST SHA3 it **doubles** wherever the contract hashes on-chain (retail, DvP, auctions). |
| **Blockers to fix before or with the migration** | (1) the on-chain verifier needs gnark commitment support (all protocols); (2) the auction Batch circuit must be re-dimensioned; (3) the auction `CHALLENGE_VERIFY_GAS` bug (pre-existing). See §6. |

About Ethereum alignment: the EVM has a native hash (Keccak-256, via the `keccak256` opcode)
and a SHA-256 precompile. It has **no** opcode or precompile for FIPS SHA3-256/384/512. A
migration that aims to match Ethereum should therefore use Keccak-256.

## 2. Method (common to all four reports)

- For each protocol's main circuit(s), every Poseidon call was routed through a swappable
  gadget: Poseidon, or gnark v0.14 `std/hash/sha3` (`New256`, `New384`, `New512`,
  `NewLegacyKeccak256`). With Poseidon, **every copy compiles to exactly the real circuit's
  constraint count.**
- Valid witnesses were built natively. Each variant ran Groth16 `Setup`, `Prove` (3–5 runs)
  and `Verify` (20 runs) on BN254. Hardware: 8-core / 16 GB Mac.
- On-chain: go-ethereum simulated backend with each project's solc settings. Merkle hashing
  used the real circomlibjs `PoseidonT3` bytecode versus the `keccak256` opcode versus an
  inline-assembly FIPS SHA3. Every on-chain hash output was checked against Go's native
  implementation. Auction verification also ran through the real `Verifier.sol` +
  `GenericGroth16Verifier.sol` with the project's real keys.
- Caveats: proof times have run-to-run noise of a few seconds, so SHA3-256 and Keccak-256
  (identical constraint counts) should be compared by constraints, not time. The SHA3
  gadget omits a canonical-form check (≈250 constraints per hash, <1.5 % in every case).
  Numbers marked "est." are extrapolated.

## 3. Proving side (off-chain)

| Protocol · operation | Metric | Poseidon | SHA3-256 / Keccak-256 | SHA3-384 | SHA3-512 |
|---|---|---|---|---|---|
| **Payments** · transfer (k = 6) | Constraints | 99,409 | 2,910,905 (29×) | 2,909,396 (29×) | 3,578,509 (36×) |
| | Proof / setup | 0.49 s / 6.5 s | 18.7 s / 7 m 26 s | 23 s / 7 m 38 s | 22.6 s / 9 m 02 s |
| | Proving key / memory | 15.6 MB / 0.3 GB | 823 MB / 9.0 GB | 823 MB / 9.3 GB | 985 MB / 11.1 GB |
| **Retail** · payment (1-in / 2-out) | Constraints | 9,661 | 942,095 (98×) | 1,109,397 (115×) | 1,164,736 (121×) |
| | Proof / setup | 63 ms / 0.8 s | 6.2–7.3 s / 2 m 31 s–2 m 51 s | 10.2 s / 3 m 10 s | 8.3 s / 2 m 56 s |
| | Proving key / memory | 1.7 MB / 43 MB | 258 MB / 3.0–3.2 GB | 332 MB / 4.7 GB | 346 MB / 5.0 GB |
| **DvP** · swap (Initiator + Destination) | Constraints | 15,049 | 1,824,140 (121×) | 2,214,693 (147×) | 2,213,601 (147×) |
| | Proof / setup (both) | 0.10 s / 1.2 s | 13.8–15.5 s / ≈5 min | 19.6 s / 6 m 40 s | 20.4 s / 6 m 32 s |
| | Proving keys / memory | 2.4 MB / 40 MB | 502 MB / 3.5 GB | 664 MB / 4.6 GB | 663 MB / 4.6 GB |
| **Auctions** · Bid (per bidder) | Constraints | 5,637 | 939,281 (167×) | 1,162,500 (206×) | 1,161,922 (206×) |
| | Proof / setup | 47 ms / 0.5 s | 7.0 s / ≈2 m 40 s | 11.0 s / 3 m 23 s | 9.8 s / 3 m 15 s |
| **Auctions** · Batch (100 bids) | Constraints | 159,468 | 6,110,877 (38×) | 11,699,393 (73×) | 11,692,972 (73×) |
| | Proof / setup / memory | 0.64 s / 12 s / 0.45 GB | ≈46 s / ≈17 min / ≈23 GB (est.) | ≈88 s / ≈33 min / ≈43 GB (est.) | same as SHA3-384 (est.) |

Off-chain verification stays cheap everywhere: ≈0.9–1.0 ms with Poseidon versus ≈1.8–2.0 ms
with any SHA3 variant.

**Where the cost comes from.** In gnark, SHA3 costs a one-time ≈131.6 k constraints per
circuit (lookup tables) plus ≈56.1 k per Keccak-f permutation. A Poseidon call costs
215–262. How many permutations each hash needs depends on its input size and each variant's
block size (rate):

| Hash input | SHA3-256 / Keccak-256 (136 B) | SHA3-384 (104 B) | SHA3-512 (72 B) | Used by |
|---|---|---|---|---|
| 1–2 field elements (32–64 B) | 1 | 1 | 1 | public keys, Merkle nodes, 2-input nullifiers |
| 3 field elements (96 B) | 1 | 1 | **2** | payments tags / blinding, retail nullifier |
| 4 field elements (128 B) | 1 | **2** | **2** | note commitments (retail, DvP, auctions) |

This table explains why the ranking of 384 versus 512 differs between protocols: payments
has no 4-input hashes, while the note-based protocols are dominated by them.

## 4. On-chain side (gas)

**Per-call building blocks (measured):**

| | Poseidon | Keccak-256 | SHA3-256 / 384 / 512 |
|---|---|---|---|
| On-chain 2-to-1 hash (Merkle node) | 32,057 | **392** | ≈122 k / 121 k / 120 k (assembly; ≈262 k with via-IR; ≈872 k in plain Solidity) |
| Merkle insert, 1 leaf (depth 8, tx gas) | 366,186 | **114,275** | ≈1.06–1.08 M |
| Merkle insert, 2 leaves (tx gas) | 374,698 | **122,787** | ≈1.07–1.09 M |
| Groth16 `verifyProof` overhead vs Poseidon | — | **+≈124 k** | **+≈124 k** |

The +124 k verifier overhead is the same in every circuit and every SHA3 variant: gnark's SHA3
gadget makes each proof carry a Pedersen commitment, which the verifier must check.

**Per protocol operation (hash-dependent gas: verification + Merkle inserts):**

| Protocol · operation | Poseidon | Keccak-256 | SHA3-256 | SHA3-384 | SHA3-512 |
|---|---|---|---|---|---|
| Payments · transfer (no on-chain hashing) | 768,731 | ≈893 k (+16 %) | ≈893 k (+16 %) | ≈893 k | 892,940 (+16 %) |
| Retail · payment (1 verify + 2-leaf insert) | ≈611 k | **≈483 k (−21 %)** | ≈1.45 M (+137 %) | ≈1.44 M | ≈1.43 M |
| DvP · swap (4 verifies + 2 × 1-leaf inserts) | ≈1.61 M | **≈1.60 M (−0.5 %)** | ≈3.54 M (+120 %) | ≈3.52 M | ≈3.50 M |
| Auctions · 100-bid auction (real verifier stack) | ≈99.0 M | **≈98.5 M (−0.6 %)** | ≈196 M (+98 %) | ≈195 M | ≈194 M |

Payments' on-chain figure is `verifyProof` only, because its contract only stores hash
outputs. SHA3-256/384 and Keccak there are estimated from the measured +124 k overhead.

## 5. Per-protocol verdict

| Protocol | Hashes on-chain? | Best SHA3-family choice | Why |
|---|---|---|---|
| **enygma_payments** | No | Keccak-256 or SHA3-256 (identical cost); SHA3-384 if blinding-factor uniformity matters | Only 1–3-input hashes, so SHA3-384 costs the same as SHA3-256 here. Its wider output makes Pedersen blinding factors near-uniform after reduction; a wide hash-to-field with Keccak gets the same result. SHA3-512 is +23 %. |
| **enygma_retail_payments** | Yes (Merkle tree) | **Keccak-256** | −21 % gas per payment. SHA3-384/512 are +18–24 % constraints (128-byte commitments). |
| **enygma_dvp** | Yes (Merkle tree) | **Keccak-256** | Gas break-even per swap (each proof is verified twice at settlement). SHA3-384/512 are +21 %. |
| **enygma_dvp_auctions** | Yes (Merkle tree) | **Keccak-256** | Gas flat per auction. Batch becomes the bottleneck (6.1 M constraints). SHA3-384/512 almost double Batch. |

For consistency across the stack (the shared `enygma_gnark_shared` library, one hash in every
client), **Keccak-256 everywhere** is the simplest coherent choice.

**Security.** Every variant is reduced into the ~254-bit BN254 field, so collision
resistance is ≈127 bits for all of them. BN254 Groth16 itself gives ~100–110 bits, so the
hash is never the weakest link. SHA3-256 and Keccak-256 share the same permutation and
capacity and differ only in padding. No protocol has a Poseidon weakness that SHA3 would fix.
The change is driven by alignment and standardization, not by a security flaw.

## 6. Cross-cutting findings and risks

| # | Finding | Where | Severity / action |
|---|---|---|---|
| 1 | **`CHALLENGE_VERIFY_GAS = 750_000` is too low (pre-existing).** Final verification through the real stack costs 761 k at transaction level. In a reproduction of the call pattern, challenges sent with **771 k–882 k gas void a valid settlement** (out of gas caught by `try/catch`). SHA3 adds ≈124 k more. | auctions `EnygmaAuction.sol` | **Fix now**, independent of the migration: ≥≈860 k of `gasleft()` plus margin today, ≈+124 k with SHA3. Confirm with a full integration test. |
| 2 | **The deployed verifier can't check SHA3 proofs.** `GenericGroth16Verifier` (circom-format VK) has no support for gnark's Pedersen commitment. | retail, DvP, auctions (payments uses gnark's exported verifier) | Extend `GenericGroth16Verifier`/`Verifier.sol` with commitment verification and export the commitment key. |
| 3 | **The auction Batch circuit doesn't fit commodity provers.** 6.1–11.7 M constraints; compiling alone took 6.6–14.7 GB. The gnark standalone Batch verifier also exceeds EIP-170 (24,930 bytes today; +2.1 KB with SHA3). | auctions | Lower `AuctionBatchSize` (≈20 bids ≈ one Bid proof) and raise `AuctionFinalBatches`. Keep the storage-VK verifier design. |
| 4 | **DvP verifies each swap proof twice.** 4 verifications per swap, which doubles the SHA3 verifier overhead. | DvP `_settleOnGroupPair` | Optional: drop the redundant pairing check (keep the root and nullifier re-checks) after a security review. Saves ≈460 k per swap even with Poseidon. |
| 5 | **Recompiling the circuit per request becomes expensive.** SHA3 circuits take 5–20 s just to compile. | gnark servers | Cache the compiled constraint system at startup. |
| 6 | **Prover resources.** 3–11 GB and 6–25 s per proof. | all | Size servers accordingly; consider a GPU prover (ICICLE). |
| 7 | **Existing notes and roots can't be re-hashed.** | retail, DvP, auctions | New vaults/trees plus a user migration window (withdraw and re-deposit, or a one-time migration circuit). |

## 7. Unified migration order

1. **Fix auction `CHALLENGE_VERIFY_GAS`** (finding 1). It is independent of the hash change.
2. **Introduce a hash abstraction** in `enygma_gnark_shared` (the shared Poseidon library)
   and in each project's native Go code, so the switch happens in one place per layer.
3. **Switch to Keccak-256** in-circuit (`NewLegacyKeccak256`) and on-chain (replace
   `PoseidonWrapper`/`PoseidonT3`/`PoseidonT5` calls with `keccak256`).
4. **Extend the generic verifier** for gnark commitments (finding 2) and re-export VKs.
5. **Re-dimension the auction Batch/Final** (finding 3).
6. **Regenerate keys, ABIs and clients**; re-derive and re-audit the domain-separation
   constants and the `mod P` handling in enygma_payments.
7. **Deploy fresh vaults/trees and migrate notes** (finding 7).
8. **Provision prover infrastructure** (findings 5–6).

## 8. Reproducibility

All benchmark code (circuit copies with a swappable hash, witness builders, Solidity SHA3 and
Merkle benches, real-key proof dump, challenge reproduction, go-ethereum harnesses) was
temporary and kept out of the repositories. Each per-protocol report documents its exact
method, per-run timings and caveats.
