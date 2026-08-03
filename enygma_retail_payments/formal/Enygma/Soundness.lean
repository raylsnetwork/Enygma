import Enygma.Circuit

/-!
# Enygma.Soundness

Main soundness theorems for the Enygma retail payment protocol.

## What "soundness" means here

The ZK-SNARK Groth16 guarantees that if a verifier accepts a proof `π` for
statement `s`, then there **exists** a witness `w` such that `(s, w)` satisfies
the circuit constraints.  The theorems below take that existence as given and
derive the protocol-level security properties from it.

In other words: we prove that **the circuit constraints correctly capture the
intended security policy**.  No adversary who can produce an accepting proof can
violate these properties, assuming:
1. Groth16 is knowledge-sound (standard assumption).
2. Poseidon is collision-resistant (axiomatized in `Hash.lean`).

## Theorems proved (Payment circuit)

| # | Theorem | Property |
|---|---------|----------|
| T1 | `payment_value_conservation`   | No value created or destroyed |
| T2 | `payment_nullifier_integrity`  | Nullifier is correctly formed |
| T3 | `payment_output_commitment_0`  | Payment commitment matches opening |
| T4 | `payment_output_commitment_1`  | Change commitment matches opening |
| T5 | `payment_change_ownership`     | Change returns to sender |
| T6 | `payment_token_binding`        | Both outputs carry the same tokenId |
| T7 | `payment_no_double_spend`      | Identical nullifiers → same note spent |
| T8 | `payment_cross_deploy_safe`    | Same note → different nullifier per contract |
| T9 | `payment_standalone`           | Message field is zero |

## Theorems proved (PrivateMint circuit)

| # | Theorem | Property |
|---|---------|----------|
| M1 | `mint_commitment_binding`   | Commitment uniquely encodes (pk, salt, amount, tokenId) |
| M2 | `mint_ciphertext_binding`   | Ciphertext uniquely encodes (pk, salt, contractAddr) |
| M3 | `mint_commitment_soundness` | A valid witness fully determines the minted note |
-/

namespace Enygma

/-! ## Payment circuit soundness theorems

Each theorem takes `s`, `w`, `h` as explicit arguments to avoid section-variable
scoping issues when theorems call each other with different instantiations. -/

/-- **T1 — Value Conservation.**

    `valueIn = valueOut[0] + valueOut[1]`  (in `Nat` — no overflow, no inflation) -/
theorem payment_value_conservation
    {s : PaymentStatement} {w : PaymentWitness}
    (h : PaymentCircuit.Satisfies s w) :
    w.valueIn = w.valueOut 0 + w.valueOut 1 :=
  h.2.2.2.2.2.2

/-- **T2 — Nullifier Integrity.**

    The on-chain nullifier is exactly `Poseidon3(sk, leafIndex, contractAddr)`. -/
theorem payment_nullifier_integrity
    {s : PaymentStatement} {w : PaymentWitness}
    (h : PaymentCircuit.Satisfies s w) :
    s.nf = nullifier w.sk w.leafIndex s.contractAddr :=
  h.2.1

/-- **T3 — Payment Output Commitment Integrity.**

    The first output commitment is a well-formed commitment to
    `(pkOut[0], saltOut[0], valueOut[0], tokenId)`. -/
theorem payment_output_commitment_0
    {s : PaymentStatement} {w : PaymentWitness}
    (h : PaymentCircuit.Satisfies s w) :
    s.commitmentsOut 0 = commitmentN (w.pkOut 0) (w.saltOut 0) (w.valueOut 0) w.tokenId :=
  h.2.2.2.1

/-- **T4 — Change Output Commitment Integrity.**

    The second output commitment is a well-formed commitment to
    `(pkOut[1], saltOut[1], valueOut[1], tokenId)`. -/
theorem payment_output_commitment_1
    {s : PaymentStatement} {w : PaymentWitness}
    (h : PaymentCircuit.Satisfies s w) :
    s.commitmentsOut 1 = commitmentN (w.pkOut 1) (w.saltOut 1) (w.valueOut 1) w.tokenId :=
  h.2.2.2.2.1

/-- **T5 — Change Output Ownership.**

    The change commitment is owned by the sender: `pkOut[1] = Poseidon1(sk)`. -/
theorem payment_change_ownership
    {s : PaymentStatement} {w : PaymentWitness}
    (h : PaymentCircuit.Satisfies s w) :
    w.pkOut 1 = spendPk w.sk :=
  h.2.2.2.2.2.1

/-- **T6 — Token ID Binding.**

    Both output commitments encode the same token ID.
    A prover cannot "transmute" one token type into another. -/
theorem payment_token_binding_out0_out1
    {s : PaymentStatement} {w : PaymentWitness}
    (h : PaymentCircuit.Satisfies s w) :
    ∃ t, s.commitmentsOut 0 = commitmentN (w.pkOut 0) (w.saltOut 0) (w.valueOut 0) t ∧
         s.commitmentsOut 1 = commitmentN (w.pkOut 1) (w.saltOut 1) (w.valueOut 1) t :=
  ⟨w.tokenId, payment_output_commitment_0 h, payment_output_commitment_1 h⟩

/-- **T7 — Nullifier Uniqueness / No Double Spend.**

    If two valid payment proofs produce the same nullifier for the same contract,
    they must have spent the same note (same `sk` and same `leafIndex`). -/
theorem payment_no_double_spend
    {s s₂ : PaymentStatement} {w w₂ : PaymentWitness}
    (h  : PaymentCircuit.Satisfies s  w)
    (h₂ : PaymentCircuit.Satisfies s₂ w₂)
    (h_same_addr : s.contractAddr = s₂.contractAddr)
    (h_nf_eq     : s.nf = s₂.nf) :
    w.sk = w₂.sk ∧ w.leafIndex = w₂.leafIndex := by
  have hnf₁ := payment_nullifier_integrity h
  have hnf₂ := payment_nullifier_integrity h₂
  have heq : nullifier w.sk w.leafIndex s.contractAddr =
             nullifier w₂.sk w₂.leafIndex s₂.contractAddr := by
    rw [← hnf₁, ← hnf₂]; exact h_nf_eq
  rw [h_same_addr] at heq
  obtain ⟨hsk, hidx, _⟩ := nullifier_unique heq
  exact ⟨hsk, hidx⟩

/-- **T8 — Cross-Deployment Safety.**

    The same note (same `sk`, `leafIndex`) produces a different nullifier when
    the contract address differs.  A proof accepted by vault A cannot be
    replayed against vault B. -/
theorem payment_cross_deploy_safe
    {s s₂ : PaymentStatement} {w w₂ : PaymentWitness}
    (h  : PaymentCircuit.Satisfies s  w)
    (h₂ : PaymentCircuit.Satisfies s₂ w₂)
    (h_same_note : w.sk = w₂.sk ∧ w.leafIndex = w₂.leafIndex)
    (h_diff_addr : s.contractAddr ≠ s₂.contractAddr) :
    s.nf ≠ s₂.nf := by
  obtain ⟨hsk, hidx⟩ := h_same_note
  have hnf₁ := payment_nullifier_integrity h
  have hnf₂ := payment_nullifier_integrity h₂
  intro h_eq
  have : s.contractAddr = s₂.contractAddr := by
    have heq : nullifier w.sk w.leafIndex s.contractAddr =
               nullifier w₂.sk w₂.leafIndex s₂.contractAddr := by
      rw [← hnf₁, ← hnf₂]; exact h_eq
    rw [hsk, hidx] at heq
    exact (nullifier_unique heq).2.2
  exact h_diff_addr this

/-- **T9 — Standalone Payment Tag.**

    The `messageIsZero` constraint holds, confirming this is a standalone payment
    (not part of a DVP or other cross-circuit composition). -/
theorem payment_standalone
    {s : PaymentStatement} {w : PaymentWitness}
    (h : PaymentCircuit.Satisfies s w) :
    s.messageIsZero :=
  h.1

/-! ## Private Mint circuit soundness theorems -/

/-- **M1 — Mint Commitment Binding.**

    If two witnesses open the same minted commitment, they must be identical.
    No adversary can find a second opening `(pk', salt', amount')` of `C`. -/
theorem mint_commitment_binding
    {s : PrivateMintStatement} {w w₂ : PrivateMintWitness}
    (h  : PrivateMintCircuit.Satisfies s w)
    (h₂ : PrivateMintCircuit.Satisfies s w₂) :
    w.pk = w₂.pk ∧ w.salt = w₂.salt ∧ w.amount = w₂.amount := by
  have heq : commitmentN w.pk w.salt w.amount s.tokenId =
             commitmentN w₂.pk w₂.salt w₂.amount s.tokenId := by
    rw [← h.1, ← h₂.1]
  obtain ⟨hpk, hsalt, hamt, _⟩ := commitmentN_binding heq
  exact ⟨hpk, hsalt, hamt⟩

/-- **M2 — Ciphertext Binding.**

    The tag `Poseidon3(pk, salt, contractAddr)` uniquely determines `(pk, salt)`. -/
theorem mint_ciphertext_binding
    {s : PrivateMintStatement} {w w₂ : PrivateMintWitness}
    (h  : PrivateMintCircuit.Satisfies s w)
    (h₂ : PrivateMintCircuit.Satisfies s w₂) :
    w.pk = w₂.pk ∧ w.salt = w₂.salt := by
  have heq : poseidon3 w.pk w.salt s.contractAddr =
             poseidon3 w₂.pk w₂.salt s.contractAddr := by
    rw [← h.2, ← h₂.2]
  obtain ⟨hpk, hsalt, _⟩ := poseidon3_inj heq
  exact ⟨hpk, hsalt⟩

/-- **M3 — Mint Commitment Soundness.**

    A valid PrivateMint witness fully determines both the minted commitment
    and the ciphertext tag. -/
theorem mint_commitment_soundness
    {s : PrivateMintStatement} {w : PrivateMintWitness}
    (h : PrivateMintCircuit.Satisfies s w) :
    s.commitment' = commitmentN w.pk w.salt w.amount s.tokenId ∧
    s.ciphertext  = poseidon3 w.pk w.salt s.contractAddr :=
  h

/-! ## Blockchain-level no-double-spend -/

/-- A nullifier set models the set of nullifiers already recorded on-chain. -/
noncomputable def NullifierSet := 𝔽 → Prop

/-- A nullifier is fresh if it has not yet been recorded. -/
noncomputable def NullifierSet.fresh (nfSet : NullifierSet) (nf : 𝔽) : Prop := ¬ nfSet nf

/-- Adding a nullifier to the set. -/
noncomputable def NullifierSet.record (nfSet : NullifierSet) (nf : 𝔽) : NullifierSet :=
  fun x => nfSet x ∨ x = nf

/-- **Global No-Double-Spend.**

    Once a nullifier is recorded on-chain, it is no longer fresh.

    Combined with T7, this means the same note cannot be spent twice. -/
theorem nullifierSet_no_replay (nfSet : NullifierSet) (nf : 𝔽) :
    ¬ (nfSet.record nf).fresh nf := by
  intro h
  exact h (Or.inr rfl)

/-- **Monotonicity.**

    Once a nullifier is spent it remains spent in any future state. -/
theorem nullifierSet_monotone (nfSet : NullifierSet) (nf nf' : 𝔽)
    (h : nfSet nf) : (nfSet.record nf') nf :=
  Or.inl h

end Enygma
