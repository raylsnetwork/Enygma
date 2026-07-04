import Enygma.Hash

/-!
# Enygma.Nullifier

The nullifier scheme for the Enygma retail payment protocol.

## Scheme

Each spent note produces a nullifier:

    nf = Poseidon3(sk, leafIndex, contractAddr)

Binding the contract address into the nullifier (VULN-5 fix) ensures that a proof
generated for one vault deployment cannot be replayed against a different deployment.

## Properties proved

- **Uniqueness** (`nullifier_unique`): the nullifier injectively encodes the triple
  (sk, leafIndex, contractAddr); identical nullifiers imply identical inputs.
- **No cross-deployment replay** (`nullifier_no_replay`): the same note (sk, leafIndex)
  produces different nullifiers under different contract addresses.
- **Spend-key binding** (`nullifier_sk_binding`): given the same nullifier, the spend
  key must be the same.
-/

namespace Enygma

/-! ## Definition -/

/-- Compute the nullifier for a note at position `leafIndex` in the Merkle tree,
    bound to a specific contract deployment `contractAddr`.

    nf = Poseidon3(sk, leafIndex, contractAddr) -/
noncomputable def nullifier (sk leafIndex contractAddr : 𝔽) : 𝔽 :=
  poseidon3 sk leafIndex contractAddr

/-! ## Security theorems -/

/-- **Nullifier Uniqueness.**

    The nullifier injectively encodes the triple (sk, leafIndex, contractAddr).
    If two nullifiers are equal, all three inputs must be equal.

    *Consequence*: a note can produce at most one distinct nullifier per
    contract deployment, ensuring double-spend prevention is well-defined. -/
theorem nullifier_unique
    {sk₁ idx₁ addr₁ sk₂ idx₂ addr₂ : 𝔽}
    (h : nullifier sk₁ idx₁ addr₁ = nullifier sk₂ idx₂ addr₂) :
    sk₁ = sk₂ ∧ idx₁ = idx₂ ∧ addr₁ = addr₂ :=
  poseidon3_inj h

/-- **No Cross-Deployment Replay.**

    The same note (sk, leafIndex) produces different nullifiers when the contract
    address differs.  A nullifier accepted by vault A cannot be replayed against
    vault B (assuming addr₁ ≠ addr₂). -/
theorem nullifier_no_replay
    {sk idx addr₁ addr₂ : 𝔽}
    (h_nf : nullifier sk idx addr₁ = nullifier sk idx addr₂) :
    addr₁ = addr₂ := by
  obtain ⟨_, _, h⟩ := nullifier_unique h_nf
  exact h

/-- **Spend-Key Binding.**

    Two identical nullifiers imply the same spend secret key was used.
    (Together with leaf-index binding, this means a nullifier uniquely
    identifies both the key and the position of the spent note.) -/
theorem nullifier_sk_binding
    {sk₁ sk₂ idx addr : 𝔽}
    (h : nullifier sk₁ idx addr = nullifier sk₂ idx addr) :
    sk₁ = sk₂ := by
  obtain ⟨h, _, _⟩ := nullifier_unique h
  exact h

/-- **Leaf-Index Binding.**

    Two identical nullifiers (same key and address) imply the same leaf index.
    A single key cannot spend two different notes and produce the same nullifier. -/
theorem nullifier_idx_binding
    {sk idx₁ idx₂ addr : 𝔽}
    (h : nullifier sk idx₁ addr = nullifier sk idx₂ addr) :
    idx₁ = idx₂ := by
  obtain ⟨_, h, _⟩ := nullifier_unique h
  exact h

end Enygma
