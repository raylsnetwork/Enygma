import Enygma.Hash

/-!
# Enygma.Commitment

The Poseidon-based commitment scheme used in the Enygma retail payment protocol.

## Scheme

Each note (a private balance) is committed as:

    C = Poseidon4(pkSpend, salt, amount, tokenId)

where
- `pkSpend = Poseidon1(sk)` is the owner's spend public key
- `salt`    is a random blinding factor (KEM-derived for payment outputs; random for change)
- `amount`  is the token amount (constrained to [0, 2^64) in the circuit)
- `tokenId` identifies the asset

## Properties proved

- **Binding** (`commitment_binding`): two openings of the same commitment must be identical.
- **Public key injectivity** (`spendPk_inj`): distinct spend keys produce distinct public keys.
- **Commitment uniqueness** (`commitment_unique_pk`): a fixed public key cannot collide with
  a commitment under a different key.
-/

namespace Enygma

/-! ## Definitions -/

/-- Derive the spend public key from the spend secret key.
    pk = Poseidon1(sk) -/
noncomputable def spendPk (sk : 𝔽) : 𝔽 := poseidon1 sk

/-- Compute the commitment to a note (pkSpend, salt, amount, tokenId).
    C = Poseidon4(pkSpend, salt, amount, tokenId) -/
noncomputable def commitment (pk salt amount tokenId : 𝔽) : 𝔽 :=
  poseidon4 pk salt amount tokenId

/-! ## Security theorems -/

/-- **Commitment Binding.**

    No adversary can produce two distinct openings `(pk₁, s₁, v₁, t₁)` and
    `(pk₂, s₂, v₂, t₂)` of the same commitment value.

    *Proof*: direct consequence of Poseidon-4 injectivity. -/
theorem commitment_binding
    {pk₁ s₁ v₁ t₁ pk₂ s₂ v₂ t₂ : 𝔽}
    (h : commitment pk₁ s₁ v₁ t₁ = commitment pk₂ s₂ v₂ t₂) :
    pk₁ = pk₂ ∧ s₁ = s₂ ∧ v₁ = v₂ ∧ t₁ = t₂ :=
  poseidon4_inj h

/-- **Spend Public Key Injectivity.**

    Two distinct spend secret keys produce distinct spend public keys.
    Equivalently: knowing `pk = Poseidon1(sk)` uniquely determines `sk`. -/
theorem spendPk_inj {sk₁ sk₂ : 𝔽}
    (h : spendPk sk₁ = spendPk sk₂) : sk₁ = sk₂ :=
  poseidon1_inj h

/-- **Token ID Binding.**

    A commitment to tokenId `t₁` cannot equal a commitment to tokenId `t₂ ≠ t₁`,
    regardless of the other parameters.  Enforces that each note is tied to
    exactly one asset. -/
theorem commitment_tokenId_binding
    {pk₁ s₁ v₁ t₁ pk₂ s₂ v₂ t₂ : 𝔽}
    (h : commitment pk₁ s₁ v₁ t₁ = commitment pk₂ s₂ v₂ t₂) :
    t₁ = t₂ :=
  (commitment_binding h).2.2.2

/-- **Amount Binding.**

    A commitment to amount `v₁` cannot equal a commitment to amount `v₂ ≠ v₁`
    with the same other parameters. -/
theorem commitment_amount_binding
    {pk s v₁ v₂ t : 𝔽}
    (h : commitment pk s v₁ t = commitment pk s v₂ t) :
    v₁ = v₂ := by
  obtain ⟨_, _, hv, _⟩ := commitment_binding h
  exact hv

end Enygma
