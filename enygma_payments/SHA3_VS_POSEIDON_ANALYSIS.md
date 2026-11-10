# Replacing Poseidon with SHA3-256 / SHA3-384 / SHA3-512 in Enygma Payments — Impact Analysis

Branch: `feat/poseidon_to_hash3` · Date: 2026-09-29

## TL;DR

Measured end to end on the **transfer (`enygma`) circuit** (real circuit logic, valid witness,
Groth16/BN254, 8-core / 16 GB Mac). Swapping Poseidon for SHA3 inside the circuit:

| | Poseidon (today) | SHA3-256 | SHA3-384 | SHA3-512 |
|---|---|---|---|---|
| R1CS constraints | 99,409 | 2,910,905 (29×) | 2,909,396 (29×) | 3,578,509 (36×) |
| Setup time | 6.5 s | 7 m 26 s (68×) | 7 m 38 s (70×) | 9 m 02 s (83×) |
| Proof time | ≈0.49 s | ≈18.7 s (38×) | ≈23 s (47×) | ≈22.6 s (46×) |
| Off-chain verify time | ≈1.0 ms | ≈1.9 ms | ≈1.9 ms | ≈2.0 ms |
| On-chain `verifyProof` gas | 768,731 | not measured | not measured | 892,940 (+16 %) |
| Verifier deploy gas | 3,384,064 | not measured | not measured | 3,705,703 (+9.5 %) |
| Proving key size | 15.6 MB | 823 MB | 823 MB | 985 MB |
| Peak process memory | 0.3 GB | 9.0 GB | 9.3 GB | 11.1 GB |
| Proof size (raw) | 324 B | 388 B | 388 B | 388 B |

The cost is in setup, proving, key size and memory. Verification stays cheap in wall-clock
terms, but the SHA3 circuit's verifier needs a Pedersen-commitment check (see "Gas" below),
which adds ≈16 % to on-chain verify gas.

| Variant | Cost vs SHA3-256 | Benefit here |
|---|---|---|
| SHA3-256 | baseline | — |
| SHA3-384 | same constraints; proving time within noise | none: output is reduced into a ~254-bit field anyway |
| SHA3-512 | +23 % constraints, +21 % setup, +20 % memory | none, and its 72-byte block forces a second permutation on every 3-input hash |

**Recommendation: keep Poseidon inside circuits.** If a SHA3 variant is ever required,
SHA3-256 (or SHA3-384, which costs the same) is the right choice; SHA3-512 only adds cost.

## Measured transfer-circuit results

Method: the transfer circuit (`gnark-server/pkg/circuits/enygma/circuit.go`, NCommitment = 6)
was copied with every `pos.Poseidon(...)` call routed through a swappable hash gadget
(Poseidon, or SHA3-256/384/512 via gnark `std/hash/sha3`). A native mirror of the hash builds
a full valid witness (sender secret chain, nullifier, fingerprints, message tags, blinding
factors, Pedersen commitments). With the Poseidon variant the copy compiles to exactly
99,409 constraints, matching the real circuit. Each variant then ran `groth16.Setup`,
`groth16.Prove` and `groth16.Verify`; the verifier was exported with `ExportSolidity` and
deployed on go-ethereum's simulated backend (solc 0.8.37, optimizer runs=1, as in
`hardhat.config.js`).

| Variant | Compile | Setup | Prove (runs) | Verify (first / best of 20) |
|---|---|---|---|---|
| Poseidon | 0.64 s | 6.5 s | 0.48 / 0.50 / 0.49 s | 1.10 / 1.00 ms |
| SHA3-256 | 16.0 s | 7 m 26 s | 18.1 / 19.4 s | 2.25 / 1.94 ms |
| SHA3-384 | 16.6 s | 7 m 38 s | 21.7 / 25.0 s | 3.59 / 1.94 ms |
| SHA3-512 | 20.4 s | 9 m 02 s | 21.8 / 23.4 s | 2.93 / 1.98 ms |

Notes and caveats:
- The gnark server recompiles the circuit on every proof request (per the project notes), so
  SHA3 adds a further ≈16–20 s of compile time per request unless the compiled CCS is cached.
- Proof times have run-to-run noise of several seconds on this laptop (only 2 runs per SHA3
  variant), so SHA3-256 vs 384 vs 512 differences in proving time are not significant; the
  constraint counts are exact.
- Gas was measured with `eth_estimateGas` on a `view` call of `verifyProof` (includes the
  21,000 intrinsic gas and calldata), not inside a full `Enygma.transfer`. It was measured
  for Poseidon and SHA3-512 only; SHA3-256/384 verifiers have the same structure (same
  public inputs and one Pedersen commitment) and are expected to cost about the same as
  SHA3-512, but that was not measured.
- **Why the SHA3 verifier costs more gas:** SHA3 in gnark uses lookup tables, which makes the
  proof carry a Pedersen commitment (`commitments`, `commitmentPok`). The generated verifier
  therefore does an extra pairing-based commitment check and a `keccak256` hash-to-field. The
  Poseidon circuit needs neither. Public inputs are 81 in every variant.
- The SHA3 gadget here omits one soundness detail (a canonical-form check on the 254-bit
  decomposition of each input, ≈250 constraints per hash, ≈12 k total), so real costs are
  marginally higher than measured.

## Where Poseidon is used

All Poseidon hashing is **inside gnark circuits** (`gnark-server/pkg/circuits/*`) and
mirrored off-chain in clients (`go_client`, `relayer`, `demo`, `poseidon_calc`). Uses:

- spend public key `pk = Poseidon(sk, sk)`
- sender secret chain `Poseidon(prevR, sk)` and nullifier `Poseidon(secretRemain, blockNumber)`
- shared-secret fingerprints `Poseidon(secret)` / `Poseidon(secret, secret)`
- message tags and randomness factors `Poseidon(HashTag|HashRandom, secret, nonce)`,
  with the direction-tag / per-slot-nonce chain
- deposit/withdraw commitments `Poseidon(Poseidon(address, value), pk)`

`contracts/enygma/contracts/Enygma.sol` only **stores** Poseidon outputs (public keys,
fingerprints); Poseidon appears there only in comments. There is no on-chain Poseidon gas
to save or lose.

## Measured gadget costs (gnark v0.14.0, BN254, Groth16 R1CS)

SHA3 in gnark has a **one-time fixed cost of ≈131.6 k constraints per circuit** (lookup
tables, shared by every hash in that circuit) plus a **marginal cost of ≈56.1 k constraints
per Keccak-f permutation**. The marginal figures below come from compiling circuits with
1, 2 and 10 hashes and differencing.

| Gadget | Marginal constraints per call |
|---|---|
| Poseidon, 2 inputs (t=3) | 241 |
| Poseidon, 3 inputs (t=4) | 262 |
| Poseidon, 1 input (t=2) | ≈215 (estimated, not measured) |
| SHA3-256, 32 / 64 / 96 B input | 56.1 k / 56.2 k / 56.3 k |
| SHA3-384, 32 / 64 / 96 B input | 56.0 k / 56.2 k / 56.3 k |
| SHA3-512, 32 / 64 B input | 56.0 k / 56.1 k |
| SHA3-512, 96 B input | **112.2 k** (2 permutations) |

Rates (bytes absorbed per permutation): SHA3-256 = 136, SHA3-384 = 104, SHA3-512 = 72.
Poseidon inputs here are 1, 2 or 3 field elements = 32, 64 or 96 B. Everything fits one
permutation except the 96 B (3-input) hashes under SHA3-512, which need two.
So per hash SHA3 is ≈**210–260× a Poseidon call**, not the ≈700–870× a naive
single-hash measurement suggests.

## Per-circuit impact (NCommitment = 6)

Poseidon call counts came from instrumenting `poseidon.Poseidon` during compilation
(temporary, reverted). "Current" counts are from compiling the real circuits.
Estimate = current − Poseidon cost + 131.6 k fixed + Σ calls × marginal cost.

| Circuit | Current | Poseidon calls (1/2/3-in) | SHA3-256 | SHA3-384 | SHA3-512 |
|---|---|---|---|---|---|
| enygma (transfer) | 99,409 | 8 / 27 / 12 | ≈2.86 M (29×) | ≈2.86 M (29×) | ≈3.53 M (36×) |
| usdr | 99,446 | 8 / 27 / 12 | ≈2.86 M (29×) | ≈2.86 M (29×) | ≈3.53 M (36×) |
| withdraw | 103,783 | 12 / 29 / 12 | ≈3.20 M (31×) | ≈3.20 M (31×) | ≈3.87 M (37×) |
| deposit | 92,450 | 2 / 11 / 12 | ≈1.62 M (18×) | ≈1.62 M (18×) | ≈2.29 M (25×) |
| enygma_fee | 91,759 | 2 / 9 / 12 | ≈1.51 M (16×) | ≈1.51 M (16×) | ≈2.18 M (24×) |

`burn` was not counted. Poseidon is only 6–12 % of today's constraints (the rest is
BabyJubJub/commitment arithmetic), so with SHA3 the hashing would dominate the circuit.
Byte decomposition of field-element inputs and reduction of the digest into the field add
a few hundred constraints per hash on top of these figures (not included above).

## Consequences

**Proving (worse, ≈38–47×).** Measured (see table above): ≈0.49 s with Poseidon vs ≈18–25 s
with SHA3, on the transfer circuit, using 9–11 GB of RAM (this 16 GB machine is near its
limit). My earlier linear extrapolation ("tens of seconds") held up.

**Setup and keys (worse).** Groth16 setup goes from 6.5 s to 7.5–9 min per circuit, and the
proving key from 15.6 MB to 823–985 MB (my earlier estimate of ≈450 MB was too low). A fresh
circuit-specific trusted setup is needed for every circuit.

**On-chain gas and verifier (slightly worse, +16 %).** Groth16 verification does not scale with
circuit size, and the public-input count (81) does not change, but the SHA3 circuit's verifier
adds a Pedersen-commitment check: `verifyProof` gas 768,731 → 892,940 (SHA3-512) and deploy gas
3,384,064 → 3,705,703. Off-chain verify time stays ≈1–2 ms.

**Native (off-circuit) hashing (faster, but irrelevant).** Go benchmark: Poseidon
(3 inputs) 25.8 µs vs SHA3-256 (96 B) 0.24 µs, about 109× faster. SHA3-384/512 native
speed was not measured. It does not offset the in-circuit cost, because every hash is
computed inside a circuit that has to be proven.

**Output width: 384 / 512 bits buy nothing here.** Every digest must be reduced into the
BN254 scalar field (~254 bits) to be used as a circuit value, so collision resistance is
capped at ~127 bits regardless of variant, which SHA3-256 already exceeds. Wider digests
only make the modular reduction slightly more uniform (bias ≈ 2⁻¹³⁰ for 384 bits, 2⁻²⁵⁸ for
512 bits, vs ≈ 2⁻² for 256 bits), which matters only if outputs must be near-uniform field
elements; a hash-to-field step could fix that at small cost for SHA3-256 as well.

**Correctness and design changes required (all variants).**
- Field-element inputs need byte decomposition plus range checks (~256 constraints each).
- NIST SHA3 is not EVM `keccak256` (different padding). Matching on-chain hashing would
  need gnark's legacy Keccak variants, but nothing on-chain hashes these values today.
- Every off-chain implementation (Go client, relayer, demo JS, `poseidon_calc`) and every
  stored public key or fingerprint would change, invalidating existing deployments and the
  `keys/` artifacts.
- The audit fixes' domain-separation tags (`HashTag = 12`, `HashRandom = 21`) and the
  `mod P` handling would have to be re-derived and re-audited.

## Security

All are sound. SHA3 is the more conservative, standardized primitive; Poseidon is newer but
well-analyzed for ZK use. The switch buys no on-chain benefit here, and the cost is the
16–37× proving overhead above.

## Recommendation

Keep Poseidon for in-circuit hashing. If a standardized hash is needed for interoperability
(e.g. binding to external data), use SHA3/Keccak outside the circuit or for a handful of
calls, not for the 23–53 hashes per proof. A cheaper ZK-friendly alternative to consider is
Poseidon2 (gnark ships `std/hash/poseidon2`).

## Method / reproducibility

Constraint counts come from `frontend.Compile(BN254, r1cs.NewBuilder, …)` with gnark v0.14.0
and `std/hash/sha3` (`New256`, `New384`, `New512`); 1-, 2- and 10-hash circuits separate the
fixed table cost (≈131.6 k) from the per-permutation cost (≈56.1 k). Transfer-circuit setup,
proof, verification and gas figures are described under "Measured transfer-circuit results".
Proofs used for the gas run were generated with gnark's Solidity-targeted prover option
(keccak hash-to-field). All benchmark code was temporary and is not in the repo (the working
tree contains only this document); the Poseidon call counter used earlier was reverted.
