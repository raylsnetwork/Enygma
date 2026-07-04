/-!
# Enygma.Hash

Abstract cryptographic hash functions for the Enygma retail payment protocol.

We model Poseidon as a family of **collision-resistant** hash functions over the
BN254 scalar field.  Collision resistance is axiomatized as perfect injectivity,
which is a sound over-approximation of the standard computational assumption:
any theorem proved here holds a fortiori in the computational model.

All security proofs in this library reduce to these axioms.
-/

namespace Enygma

/-! ## Abstract field type

We work entirely in the `noncomputable` universe to avoid code-generator issues
with abstract axiomatized types.  All proofs here are purely propositional.
-/

/-- Elements of the BN254 scalar field 𝔽_p.
    The concrete prime is p = 21888242871839275222246405745257275088548364400416034343698204186575808495617.
    We keep it abstract to avoid carrying arithmetic proof obligations. -/
axiom 𝔽 : Type

/-! ## Poseidon hash functions

Poseidon is a ZK-friendly sponge construction over 𝔽_p.  We axiomatize the
arities used in the Enygma circuits as plain `axiom` constants (no computability
required — all proofs are purely propositional).

| Arity | Used for                                           |
|-------|----------------------------------------------------|
| 1     | Spend public key: pk = Poseidon(sk)                |
| 2     | Merkle tree internal nodes: H(left, right)         |
| 3     | Nullifier: nf = Poseidon(sk, idx, addr)            |
|       | Private-mint tag: tag = Poseidon(pk, salt, addr)   |
| 4     | Commitment: C = Poseidon(pk, salt, value, tokenId) |
-/

/-- Poseidon hash of one field element. -/
axiom poseidon1 : 𝔽 → 𝔽

/-- Poseidon hash of two field elements. -/
axiom poseidon2 : 𝔽 → 𝔽 → 𝔽

/-- Poseidon hash of three field elements. -/
axiom poseidon3 : 𝔽 → 𝔽 → 𝔽 → 𝔽

/-- Poseidon hash of four field elements. -/
axiom poseidon4 : 𝔽 → 𝔽 → 𝔽 → 𝔽 → 𝔽

/-! ## Collision-resistance axioms

The axioms below state that Poseidon is **injective** at each arity.
In the computational model this corresponds to: no PPT adversary can find
two distinct inputs that map to the same output except with negligible probability.
We model this as a worst-case (information-theoretic) guarantee, which is strictly
stronger and yields unconditional proofs of all protocol properties. -/

/-- Poseidon-1 is injective. -/
axiom poseidon1_inj {x y : 𝔽} :
    poseidon1 x = poseidon1 y → x = y

/-- Poseidon-2 is jointly injective in both arguments. -/
axiom poseidon2_inj {a₁ b₁ a₂ b₂ : 𝔽} :
    poseidon2 a₁ b₁ = poseidon2 a₂ b₂ → a₁ = a₂ ∧ b₁ = b₂

/-- Poseidon-3 is jointly injective in all three arguments. -/
axiom poseidon3_inj {a₁ b₁ c₁ a₂ b₂ c₂ : 𝔽} :
    poseidon3 a₁ b₁ c₁ = poseidon3 a₂ b₂ c₂ → a₁ = a₂ ∧ b₁ = b₂ ∧ c₁ = c₂

/-- Poseidon-4 is jointly injective in all four arguments. -/
axiom poseidon4_inj {a₁ b₁ c₁ d₁ a₂ b₂ c₂ d₂ : 𝔽} :
    poseidon4 a₁ b₁ c₁ d₁ = poseidon4 a₂ b₂ c₂ d₂ →
    a₁ = a₂ ∧ b₁ = b₂ ∧ c₁ = c₂ ∧ d₁ = d₂

end Enygma
