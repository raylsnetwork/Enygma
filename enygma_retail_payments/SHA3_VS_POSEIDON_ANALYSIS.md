# Replacing Poseidon with SHA3 in Enygma Retail Payments — Impact Analysis

Branch: `feat/poseidon_to_hash3` · Date: 2026-09-30

Companion to `enygma_payments/SHA3_VS_POSEIDON_ANALYSIS.md`. Retail payments differs from
enygma_payments in one way that changes the conclusions: **it also hashes on-chain.** The
vault's Merkle tree (`enygma_dvp/.../vaults/Merkle.sol`) hashes every inserted commitment up
the tree with `PoseidonT3`, so the choice of hash affects gas as well as proving.

## TL;DR

Measured on the retail **`Payment` circuit** (1 input, 2 outputs, Merkle depth 8 — the
`/proof/payment` configuration) and on a copy of `Merkle.sol`'s `insertLeaves`:

| | Poseidon (today) | SHA3-256 | Keccak-256 † | SHA3-384 | SHA3-512 |
|---|---|---|---|---|---|
| R1CS constraints | 9,661 | 942,095 (98×) | 942,095 (98×) | 1,109,397 (115×) | 1,164,736 (121×) |
| Setup time | 0.76 s | 2 m 31 s | 2 m 51 s | 3 m 10 s | 2 m 56 s |
| Proof time (mean of 5) | 63 ms | 6.2 s | 7.3 s | 10.2 s | 8.3 s |
| Off-chain verify (best of 20) | 0.90 ms | 1.84 ms | 1.77 ms | 1.84 ms | 1.77 ms |
| Proving key | 1.7 MB | 258 MB | 258 MB | 332 MB | 346 MB |
| Peak prover memory | 43 MB | 3.0 GB | 3.2 GB | 4.7 GB | 5.0 GB |
| On-chain hash, per call | 32,057 gas | ≈122,400 gas ‡ | **392 gas** | ≈121,100 gas ‡ | ≈119,800 gas ‡ |
| Merkle insert of 2 commitments (tx gas) | 374,698 | ≈1,090,500 ‡ | **122,787** | ≈1,080,200 ‡ | ≈1,069,600 ‡ |
| `verifyProof` gas | 257,832 | 381,637 | 381,698 | 381,698 | 381,685 |
| **Hash-dependent gas per payment** § | **≈611 k** | ≈1.45 M (+137 %) | **≈483 k (−21 %)** | ≈1.44 M (+136 %) | ≈1.43 M (+134 %) |

† Keccak-256 is the original Keccak submission used by Ethereum (`keccak256` opcode), not
the FIPS 202 SHA3-256 standard. They share the Keccak-f permutation and security level and
differ only in padding. It is included because it is the only SHA3-family hash the EVM
computes natively.
‡ NIST SHA3 has no EVM opcode or precompile, so it has to be implemented in Solidity.
Measured with a hand-unrolled inline-assembly Keccak-f compiled with the legacy optimizer.
Under the project's own via-IR settings the same code costs ≈262 k gas per hash.
§ `verifyProof` gas + Merkle-insert gas − one 21,000 intrinsic tx charge. These are the
parts of `payment()` that change with the hash; the rest of `payment()` (nullifier storage,
events, encrypted-note calldata) is the same for every variant and was not measured.

**Recommendation for retail:**
1. **Keep Poseidon.** Proofs take ~60 ms instead of 6–10 s, keys are 1.7 MB instead of
   ~260–350 MB, and on-chain gas is reasonable.
2. **If a SHA3-family hash is required, use Keccak-256.** It costs the same in-circuit as
   SHA3-256, it is the only option that *lowers* on-chain gas (−21 % hash-dependent gas,
   −67 % on the Merkle insert), and it offers the same security as SHA3-256.
3. **If FIPS 202 SHA3 is mandated, use SHA3-256.** SHA3-384 and SHA3-512 cost 18–24 % more
   constraints here and give no security benefit that survives the reduction into the
   BN254 field.

This differs from the enygma_payments recommendation (SHA3-384) because retail hashes
4-input commitments (128 bytes), which fit one SHA3-256 block (136 B) but need two
permutations under SHA3-384 (104 B) and SHA3-512 (72 B). Retail's hash outputs also are
not used as Pedersen blinding factors, which is where SHA3-384's better output uniformity
paid off in enygma_payments.

## Where Poseidon is used

**In the circuit** (`gnark_circuits/primitives`, `templates/Payment.go`), per 1-in/2-out
payment (counted during compilation):

| Use | Inputs | Bytes | Calls |
|---|---|---|---|
| Spend public key `pk = H(sk)` (sender + input owner) | 1 | 32 | 2 |
| Merkle path `H(left, right)`, depth 8 | 2 | 64 | 8 |
| Nullifier `H(sk, globalLeafIdx, contractAddress)` | 3 | 96 | 1 |
| Commitment `H(pk, salt, amount, tokenId)` (1 input + 2 outputs) | 4 | 128 | 3 |

**On-chain:** `Merkle.sol` hashes every inserted leaf up the tree via
`PoseidonWrapper → PoseidonT3` (≈8 hashes for a 2-leaf insert at depth 8). `Erc20CoinVault`
also calls `PoseidonT5` once per deposit to build the commitment (not part of `payment()`).

## Why the SHA3 variants differ from each other

Keccak-f permutations per hash (rate = bytes absorbed per permutation):

| Input | SHA3-256 / Keccak-256 (136 B) | SHA3-384 (104 B) | SHA3-512 (72 B) |
|---|---|---|---|
| 32 B (pk) | 1 | 1 | 1 |
| 64 B (Merkle) | 1 | 1 | 1 |
| 96 B (nullifier) | 1 | 1 | **2** |
| 128 B (commitment) | 1 | **2** | **2** |
| Total per payment | 14 | 17 | 18 |

At ≈56 k constraints per extra permutation this accounts exactly for the gaps:
SHA3-384 is +3 permutations (+167 k), SHA3-512 is +4 (+223 k). On-chain, only the 64-byte Merkle
hash is computed, which is one permutation for all three, so their gas is the same.

## Measured results — details

**Method.** `templates.PaymentCircuit` was copied with every Poseidon call routed through a
swappable hash gadget: Poseidon, or gnark `std/hash/sha3` `New256`/`New384`/`New512`/
`NewLegacyKeccak256`. Each field element is serialized as 32 bytes big-endian and the digest is
read as a big-endian integer mod the BN254 field. With Poseidon the copy compiles to
exactly 9,661 constraints, matching the real circuit. A native mirror built a full valid
witness (note at leaf 5 of an 8-level tree, 1000 → 300 to recipient + 700 change). Each
variant ran `groth16.Setup`, 5× `Prove`, 20× `Verify`, then a Solidity-targeted proof was
verified on-chain against the gnark-exported verifier (go-ethereum simulated backend,
solc 0.8.37, via-IR, optimizer runs = 1600, matching `enygma_dvp/hardhat.config.js`).
Hardware: 8-core / 16 GB Mac.

| Variant | Compile | Setup | Prove runs (s) | Verify first / best |
|---|---|---|---|---|
| Poseidon | 0.07 s | 0.76 s | 0.065 / 0.056 / 0.067 / 0.054 / 0.074 | 1.01 / 0.90 ms |
| SHA3-256 | 5.4 s | 2 m 31 s | 6.56 / 6.09 / 5.90 / 6.06 / 6.32 | 2.03 / 1.84 ms |
| Keccak-256 | 5.6 s | 2 m 51 s | 8.45 / 7.14 / 6.97 / 7.58 / 6.46 | 2.07 / 1.77 ms |
| SHA3-384 | 6.2 s | 3 m 10 s | 9.28 / 10.05 / 10.47 / 10.42 / 10.70 | 3.08 / 1.84 ms |
| SHA3-512 | 6.4 s | 2 m 56 s | 8.40 / 8.33 / 8.37 / 8.38 / 8.19 | 2.02 / 1.77 ms |

SHA3-256 and Keccak-256 have identical constraint counts, so their proof-time gap
(and SHA3-384 proving slower than SHA3-512) is run-to-run and thermal noise on a laptop that
ran the variants back to back. Compare variants by constraint count; treat proof times as
"≈6–10 s for any SHA3 variant".

**On-chain Merkle hashing.** `MerkleBench` reproduces `Merkle.sol`'s `hashLeftRight` and
`insertLeaves` verbatim with a swappable hash. Poseidon goes through the real circomlibjs
`PoseidonT3`/`PoseidonT5` bytecode and a linked `PoseidonWrapper` from
`contracts/abis/`, as `scripts/cmd/deploy` does. Every variant's on-chain output was
checked against Go's native implementation (iden3 Poseidon, `crypto/sha3`,
`x/crypto/sha3` Keccak) and all matched. The Solidity SHA3 was also run with Keccak padding
and matched the EVM `keccak256` opcode. Insert gas is the mean of 32 consecutive 2-leaf
inserts (min/max: Poseidon 356 k / 538 k, Keccak 104 k / 286 k; the max is the insert that
rolls over to a new tree).

| On-chain SHA3 implementation | Gas per hash |
|---|---|
| Plain Solidity (arrays, bounds checks) | ≈872 k |
| Inline assembly, via-IR optimizer (project setting) | ≈262 k |
| Inline assembly, legacy optimizer (used in the table) | ≈120–122 k |

An expert stack-only Keccak-f could plausibly be ~2× cheaper again (not tested). Even
then, NIST SHA3 would cost more per hash than Poseidon's 32 k, and far more than the
392-gas opcode.

**Verifier.** All SHA3/Keccak verifiers cost ≈124 k more gas per `verifyProof` (+48 %).
gnark's SHA3 gadget uses lookup tables, which makes each proof carry a Pedersen commitment,
so the verifier does an extra pairing-based commitment check. Public inputs stay at 7.

## Integration consequences

- **The deployed verifier would have to change.** Retail currently verifies through
  `GenericGroth16Verifier` with a circom-format VK exported from gnark
  (`cmd/export_vk_init`). That verifier has no support for the Pedersen commitment that
  SHA3 proofs carry, so a SHA3 switch needs gnark's exported verifier (as measured here) or
  an extended generic verifier. The gas table uses gnark-exported verifiers for every
  variant, including Poseidon, so the comparison is like-for-like. The currently deployed
  `GenericGroth16Verifier` was not measured.
- **The on-chain tree and the circuit must use the same hash.** The Merkle root the circuit
  proves against is built on-chain. You cannot mix Keccak on-chain with Poseidon in-circuit
  for the Merkle path.
- **Everything off-chain changes.** The Go client (`src/`), relayer, demo, `private_tags`,
  the user registry's `pkSpend = Poseidon(sk)`, `Erc20CoinVault` deposit commitments
  (`PoseidonT5`) and every stored key or commitment would change. Existing deployments and
  `gnark_circuits/scripts/keys/` would be invalidated.
- **Soundness detail not included in the measurements.** The SHA3 gadget here decomposes
  each field element to 254 bits without the canonical-form check, which is about 250 more
  constraints per hash (≈3.5 k per payment, <0.5 %). Real SHA3 circuits would be marginally larger.

## Security

- **Collision resistance is the same for every SHA3 variant.** Digests are reduced into the
  BN254 field (~254 bits), so collision resistance is ≈127 bits whichever variant is used.
  BN254 Groth16 itself gives ~100–110 bits, so the hash is not the weakest link in any option.
- **SHA3-256 and Keccak-256 are equally strong.** They use the same permutation and
  capacity (512 bits) and differ only in padding. Pick SHA3-256 only if FIPS 202 compliance
  is required.
- **SHA3-384/512's extra output is discarded** by the field reduction. Their larger capacity
  (768 / 1024 bits) is a margin that doesn't raise the ≈127-bit cap. In retail, hash outputs
  are commitments, Merkle nodes, nullifiers and public keys, where binding and collision
  resistance matter and output uniformity does not, so SHA3-384's uniformity advantage from
  enygma_payments does not apply here.
- **Poseidon** is sound and well-studied for ZK use; the existing design has no hash weakness
  that SHA3 would fix.

## Option not measured

A hybrid of Keccak-256 for the Merkle tree only (in-circuit and on-chain) with Poseidon
for pk, nullifier and commitments would have ≈8 Keccak permutations in-circuit, an estimated
≈0.59 M constraints (≈60×, versus 98× for full SHA3), while keeping the cheap on-chain
insert. This is an estimate only and was not built or measured.

## Reproducibility

The benchmark code (circuit copy, witness builder, swappable hash gadget, `MerkleBench`
Solidity, go-ethereum harness) was temporary and is not in the repo. The working tree
contains only this document.
