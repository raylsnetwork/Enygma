import Enygma.Hash
import Enygma.Commitment
import Enygma.Nullifier
import Enygma.MerkleTree
import Enygma.Circuit
import Enygma.Soundness

/-!
# Enygma Formal Verification

Formal soundness proofs for the Enygma retail payment protocol, written in Lean 4.

## Scope

This library proves that the **circuit constraints** of the Enygma payment system
correctly enforce the intended security policy, under the assumption that Poseidon
is collision-resistant (axiomatized in `Enygma.Hash`).

The proofs do **not** verify:
- The cryptographic security of Poseidon itself (a computational assumption).
- The soundness of the Groth16 proof system (standard knowledge-soundness assumption).
- The correctness of the gnark circuit implementation vs. this Lean model.

## Modules

| Module               | Contents                                          |
|----------------------|---------------------------------------------------|
| `Enygma.Hash`        | Abstract Poseidon hash axioms (collision-free)    |
| `Enygma.Commitment`  | Commitment scheme + binding theorem               |
| `Enygma.Nullifier`   | Nullifier scheme + uniqueness + anti-replay       |
| `Enygma.MerkleTree`  | Binary Merkle tree + second-preimage resistance   |
| `Enygma.Circuit`     | Payment & PrivateMint circuit constraint models   |
| `Enygma.Soundness`   | Main soundness theorems (T1–T9, M1–M3)            |

## Key theorems

### Payment circuit

- **T1 `payment_value_conservation`** — `valueIn = valueOut[0] + valueOut[1]`
- **T2 `payment_nullifier_integrity`** — nullifier is `Poseidon3(sk, leafIdx, addr)`
- **T3/T4 `payment_output_commitment_{0,1}`** — output commitments are well-formed
- **T5 `payment_change_ownership`** — change returns to sender's key
- **T6 `payment_token_binding_out0_out1`** — both outputs share the same tokenId
- **T7 `payment_no_double_spend`** — equal nullifiers → same note → double spend blocked
- **T8 `payment_cross_deploy_safe`** — same note, different vault → different nullifier
- **T9 `payment_standalone`** — message tag is zero

### PrivateMint circuit

- **M1 `mint_commitment_binding`** — unique opening of the minted commitment
- **M2 `mint_ciphertext_binding`** — ciphertext tag uniquely encodes (pk, salt)
- **M3 `mint_commitment_soundness`** — witness fully determines on-chain outputs

### Blockchain layer

- **`nullifierSet_no_replay`** — a recorded nullifier can never be submitted again
- **`nullifierSet_monotone`** — spent nullifiers remain spent in future states

## Known gaps

- `merkleRoot_sibling_inj` in `MerkleTree.lean` is marked `sorry`.  Proving full
  sibling injectivity requires a stronger induction hypothesis (that the IH applies
  to sub-paths with swapped roles of leaf and sibling).  The weaker
  `merkleRoot_leaf_inj` (second-preimage resistance in the leaf) is fully proved.
-/
