# Replacing Poseidon with SHA3 in Enygma DvP — Impact Analysis

Branch: `feat/poseidon_to_hash3` · Date: 2026-09-30

Companion to `enygma_payments/` and `enygma_retail_payments/SHA3_VS_POSEIDON_ANALYSIS.md`.
The DvP `Payment` circuit is identical to retail's (only comments differ), and both
projects use the same `Merkle.sol`, so **the retail report's payment numbers apply to DvP
payments unchanged.** This report covers what is specific to DvP: the **two-proof swap**
(`DvPInitiator` for Alice + `DvPDestination` for Bob; ERC20 ↔ ERC721 in the tests).

## TL;DR

| Per swap (both proofs) | Poseidon (today) | SHA3-256 | Keccak-256 † | SHA3-384 | SHA3-512 |
|---|---|---|---|---|---|
| Constraints (Initiator + Destination) | 15,049 | 1,824,140 (121×) | 1,824,140 (121×) | 2,214,693 (147×) | 2,213,601 (147×) |
| Setup time (both circuits) | 1.2 s | 5 m 02 s | 4 m 54 s | 6 m 40 s | 6 m 32 s |
| Proof time (both proofs, mean) | ≈0.10 s | ≈13.8 s | ≈15.5 s | ≈19.6 s | ≈20.4 s |
| Proving keys (both) | 2.4 MB | 502 MB | 502 MB | 664 MB | 663 MB |
| Peak prover memory | ≈40 MB | ≈3.4 GB | ≈3.5 GB | ≈4.6 GB | ≈4.6 GB |
| `verifyProof` gas, Initiator / Destination | 258 k / 244 k | 382 k / 368 k | 382 k / 368 k | 382 k / 368 k | 382 k / 368 k |
| Merkle insert, 1 leaf (per vault) | 366,186 | ≈1,082,000 ‡ | **114,275** | ≈1,071,700 ‡ | ≈1,061,100 ‡ |
| **Hash-dependent gas per swap** § | **≈1.61 M** | ≈3.54 M (+120 %) | **≈1.60 M (−0.5 %)** | ≈3.52 M (+118 %) | ≈3.50 M (+117 %) |

† Keccak-256 = Ethereum's `keccak256` (original Keccak padding), not FIPS 202 SHA3-256.
It uses the same permutation and security level and is the only SHA3-family hash the EVM computes natively.
‡ NIST SHA3 has no EVM opcode or precompile. Measured with an inline-assembly Keccak-f
(legacy optimizer); under the project's via-IR settings it costs ≈2× more.
§ A swap on-chain = **4 `verifyProof` calls** (see "On-chain swap flow") **+ 2 single-leaf
Merkle inserts**. Each component is counted as its measured gas minus the 21,000 intrinsic
charge. Nullifier storage, locks, events and calldata are the same for every variant and
not included.

**Recommendation for DvP:**
1. **Keep Poseidon.** A swap's proofs take ~0.1 s instead of ~14–20 s, keys are 2.4 MB
   instead of 0.5–0.66 GB, and gas is the same as or lower than every alternative.
2. **If a SHA3-family hash is required, use Keccak-256.** It costs the same in-circuit as
   SHA3-256. On-chain it breaks even with Poseidon per swap: the Merkle inserts get ≈500 k
   cheaper, and the four SHA3-style verifications get ≈495 k more expensive.
3. **If FIPS 202 SHA3 is mandated, use SHA3-256.** SHA3-384 and SHA3-512 cost ≈21 % more
   constraints for both swap circuits and give no usable security gain.

## Where Poseidon is used in a swap

Per circuit, counted during compilation (depth-8 tree, as in `server/circuits/dvp*/handler.go`):

| Use | Inputs | Bytes | Initiator | Destination |
|---|---|---|---|---|
| Spend public key `pk = H(sk)` | 1 | 32 | 1 | 1 |
| Nullifier `H(sk, treeNumber·2^8 + leafIdx)` | 2 | 64 | 1 | 1 |
| Merkle path `H(left, right)` | 2 | 64 | 8 | 8 |
| Commitments `H(pk, salt, value, tokenId)` | 4 | 128 | 4 (in, B, A, revert) | 3 (in, A, B) |

**Keccak-f permutations:** SHA3-256 / Keccak-256 need 14 (Initiator) and 13 (Destination).
SHA3-384 and SHA3-512 each need one extra permutation per 128-byte commitment: 18 and 16. Unlike
retail, the DvP nullifier has only 2 inputs (64 bytes), so SHA3-512 has no extra
permutation beyond SHA3-384, and the two cost the same here.

## On-chain swap flow (what the gas figure counts)

From `EnygmaDvp.submitPartialSettlement` / `_settleOnGroupPair` and the vaults:

1. **Tx 1: Alice submits her Initiator proof.** `Erc20CoinVault.checkReceiptConditions`
   verifies it (`VK_ID_DVP_INITIATOR`) and the note is locked. No Merkle insert.
2. **Tx 2: Bob submits his Destination proof.** `Erc721CoinVault.checkReceiptConditions`
   verifies it (`VK_ID_DVP_DESTINATION`). Then `_settleOnGroupPair` calls
   `checkReceiptConditions` on **both** receipts again, which re-verifies both proofs. Each vault then
   nullifies one note and inserts one commitment (`NumberOfOutputs = 1`: commitB into the
   ERC20 vault, commitA into the ERC721 vault).

The total is **4 Groth16 verifications and 2 single-leaf inserts per swap.** Because each
proof is verified twice, the verifier's SHA3 overhead (≈+124 k gas per verification) is
also paid twice. The second check also rechecks state (root validity, nullifier unspent),
which must stay; only the repeated pairing check is redundant. Without the repeated check,
Keccak-256 would beat Poseidon clearly: ≈0.90 M vs ≈1.15 M hash-dependent gas per swap.
That is an observation, not a recommendation to change settlement logic without a security review.

## Measured results — details

**Method.** `templates.DvPInitiatorCircuit` and `templates.DvPDestinationCircuit` were copied
with every Poseidon call routed through a swappable hash gadget (Poseidon, or gnark
`std/hash/sha3` `New256`/`New384`/`New512`/`NewLegacyKeccak256`; field elements serialized as
32 bytes big-endian, digest read big-endian mod the BN254 field). With Poseidon the copies
compile to exactly 7,674 and 7,375 constraints, matching the real circuits. A native mirror
built a valid swap witness: Alice's 30-unit ERC20 note at leaf 5 for Bob's NFT at leaf 2, with
matching commitA/commitB across both proofs. Each variant ran `groth16.Setup`, `Prove`
(5 runs for Poseidon, 3 for SHA3), 20× `Verify`. A Solidity-targeted proof was then verified
on-chain against the gnark-exported verifier (go-ethereum simulated backend, solc 0.8.37,
via-IR, runs = 1600, as in `hardhat.config.js`). Hardware: 8-core / 16 GB Mac.

**DvPInitiator** (7 public inputs):

| Variant | Constraints | Setup | Prove runs (s) | Verify best | PK | Memory | `verifyProof` gas |
|---|---|---|---|---|---|---|---|
| Poseidon | 7,674 | 0.63 s | .046/.045/.045/.047/.054 | 0.90 ms | 1.2 MB | 38 MB | 258,377 |
| SHA3-256 | 941,318 | 2 m 42 s | 7.98/6.64/6.32 | 1.83 ms | 258 MB | 3.2 GB | 382,242 |
| Keccak-256 | 941,318 | 2 m 32 s | 6.98/9.45/8.88 | 1.80 ms | 258 MB | 3.5 GB | 382,230 |
| SHA3-384 | 1,164,537 | 3 m 30 s | 9.27/9.85/10.04 | 1.78 ms | 346 MB | 4.6 GB | 382,254 |
| SHA3-512 | 1,163,959 | 3 m 20 s | 9.37/9.31/9.57 | 1.84 ms | 345 MB | 4.6 GB | 382,218 |

**DvPDestination** (5 public inputs):

| Variant | Constraints | Setup | Prove runs (s) | Verify best | PK | Memory | `verifyProof` gas |
|---|---|---|---|---|---|---|---|
| Poseidon | 7,375 | 0.59 s | .040/.084/.046/.043/.048 | 0.89 ms | 1.2 MB | 34 MB | 244,342 |
| SHA3-256 | 882,822 | 2 m 20 s | 6.85/7.00/6.55 | 1.80 ms | 244 MB | 3.4 GB | 368,219 |
| Keccak-256 | 882,822 | 2 m 23 s | 7.71/7.06/6.52 | 1.91 ms | 244 MB | 3.2 GB | 368,207 |
| SHA3-384 | 1,050,156 | 3 m 10 s | 9.83/9.52/10.16 | 1.79 ms | 318 MB | 4.5 GB | 368,219 |
| SHA3-512 | 1,049,642 | 3 m 13 s | 13.79/9.67/9.46 | 1.78 ms | 318 MB | 4.5 GB | 368,183 |

SHA3-256 and Keccak-256 have identical constraint counts, so their proof-time differences
are run-to-run and thermal noise (the variants ran back to back on a laptop). Compare
variants by constraint count.

**On-chain Merkle insert.** A copy of `Merkle.sol`'s `insertLeaves`/`hashLeftRight` with a
swappable hash. Poseidon runs through the real circomlibjs `PoseidonT3` bytecode and linked
`PoseidonWrapper`. Every variant's on-chain hash was checked against Go's native
implementation and matched. Mean of 32 single-leaf inserts (min/max: Poseidon 352 k / 380 k,
Keccak 101 k / 128 k, SHA3-256 1.068 M / 1.096 M). Per hash: Poseidon 32,057 gas, `keccak256`
opcode 392, SHA3 ≈120–122 k (assembly, legacy optimizer), ≈262 k (via-IR), ≈872 k (plain Solidity).

## Integration consequences

- **The verifier contract would have to change.** DvP verifies through `IVerifier.verifyProof(vkId, …)` (the
  generic Groth16 verifier with circom-format VKs exported by `cmd/export_vk_init`). gnark's
  SHA3 gadget uses lookup tables, so SHA3 proofs carry a Pedersen commitment that verifier
  cannot check. You would need gnark's exported per-circuit verifiers (as measured here) or
  an extended generic verifier. All gas figures here use gnark-exported verifiers for every
  variant, including Poseidon, so they are like-for-like. The currently deployed generic
  verifier was not measured.
- **Circuits and the on-chain tree must match.** The swap's Merkle proofs are against
  roots built on-chain by `PoseidonT3`. The on-chain hash and the in-circuit Merkle hash must
  be the same function.
- **Other on-chain Poseidon uses would also change.** `Erc20CoinVault` (`PoseidonT5`
  deposit commitments), `Erc721CoinVault`/`Erc1155CoinVault` (NFT and ERC1155 commitments),
  and `EnygmaAuction` (blinded bids) all call the Poseidon wrapper. They are outside the swap
  but would need to move with it.
- **Everything off-chain changes.** `src/core` (commitments, nullifiers, Merkle mirror,
  `ContractStatement`), the relayer, demo, and all keys in `gnark_circuits/scripts/keys/`.
  Existing deployments and notes would be invalidated.
- **Soundness detail not included in the measurements.** The SHA3 gadget decomposes each field
  element to 254 bits without the canonical-form check, about 250 more constraints per hash (<0.5 %).

## Security

- **Collision resistance is the same for every SHA3 variant.** Digests are reduced into the
  ~254-bit BN254 field, so collision resistance is ≈127 bits whichever variant is used.
  BN254 Groth16 gives ~100–110 bits, so the hash is not the weakest link in any option.
- **SHA3-256 and Keccak-256 are equally strong.** They use the same permutation and
  512-bit capacity and differ only in padding. Choose SHA3-256 only for FIPS 202 compliance.
- **SHA3-384/512's extra output bits are discarded** by the field reduction. In a swap, hash
  outputs are commitments (commitA, commitB, revertCommitA), nullifiers, Merkle nodes and
  public keys, where binding and collision resistance matter and output uniformity does not.
  So the uniformity argument for SHA3-384 in enygma_payments does not apply here.
- **The swap's cross-binding is unchanged by the hash choice.** `StMessage = commitA`
  (Initiator) and `StMessage = commitB` (Destination) work identically with any
  collision-resistant hash.

## Reproducibility

The benchmark code (circuit copies, swap witness builder, swappable hash gadget,
`MerkleBench` Solidity, go-ethereum harness) was temporary and is not in the repo. The
working tree contains only this document.
