import Enygma.Hash
import Enygma.Commitment
import Enygma.Nullifier
import Enygma.MerkleTree

/-!
# Enygma.Circuit

Lean models of the two gnark circuits in the Enygma retail payment protocol:

1. **`PaymentCircuit`** (`gnark_circuits/templates/Payment.go`) — spends one input
   note and creates two output notes (payment + change).
2. **`PrivateMintCircuit`** (`gnark_circuits/templates/PrivateMint.go`) — proves
   knowledge of the opening of a privately minted commitment.

## Design note on value types

Token amounts are represented as `Nat` (natural numbers) rather than `𝔽` (field
elements) because:
- The circuit enforces `0 ≤ value < 2^64`, so values are semantically naturals.
- `Nat` arithmetic makes conservation proofs clean (no field axioms needed).
- The Poseidon hash `commitment` takes a `Nat` amount and a field tokenId,
  mirroring the `Poseidon4(pk, salt, value, tokenId)` formula where value is
  range-constrained.

Hashes, keys, salts, commitments, nullifiers, addresses, and token IDs are `𝔽`.
-/

namespace Enygma

/-! ## Commitment with Nat amount

We extend `Commitment.lean`'s definition to accept a `Nat` amount, reflecting
the range-constrained nature of token values. -/

/-- Commitment to a note with a `Nat` amount.
    C = Poseidon4(pkSpend, salt, value_as_𝔽_encoding, tokenId)
    The `Nat` value is passed through as a field element via the protocol's
    standard encoding (little-endian, fits in 64 bits). -/
axiom commitmentN : 𝔽 → 𝔽 → Nat → 𝔽 → 𝔽

/-- Binding for commitmentN: identical commitments imply identical openings. -/
axiom commitmentN_binding {pk₁ s₁ v₁ t₁ pk₂ s₂ v₂ t₂ : _} :
    commitmentN pk₁ s₁ v₁ t₁ = commitmentN pk₂ s₂ v₂ t₂ →
    pk₁ = pk₂ ∧ s₁ = s₂ ∧ v₁ = v₂ ∧ t₁ = t₂

/-! ## Payment Circuit -/

/-- Public inputs (statement) for the Payment circuit.

    These are committed on-chain and verified by the smart contract. -/
structure PaymentStatement where
  /-- Domain-separation tag; must be 0 for a standalone payment. -/
  messageIsZero  : Prop    -- abstractly: the on-chain check s.message = 0
  /-- Sub-tree index for the input note. -/
  treeNumber     : 𝔽
  /-- Merkle root of the sub-tree containing the input note.
      Must be 0 for zero-value (padding) inputs. -/
  merkleRoot     : 𝔽
  /-- Nullifier for the input note: Poseidon3(sk, leafIndex, contractAddr). -/
  nf             : 𝔽
  /-- Output commitments: index 0 = payment, index 1 = change. -/
  commitmentsOut : Fin 2 → 𝔽
  /-- Address of the vault contract — binds the proof to one deployment. -/
  contractAddr   : 𝔽

/-- Private witness for the Payment circuit. -/
structure PaymentWitness where
  /-- Spend secret key of the sender (Alice). -/
  sk         : 𝔽
  /-- Amount being spent (opening of the input note), as a natural number. -/
  valueIn    : Nat
  /-- Salt of the input note (received when Alice was paid). -/
  saltIn     : 𝔽
  /-- Merkle authentication path for the input note.
      Each element is (bit, sibling): `bit=false` → current is left child. -/
  path       : List (Bool × 𝔽)
  /-- Leaf index (position in the Merkle tree), as a field element. -/
  leafIndex  : 𝔽
  /-- Token identifier (shared across all notes in this proof). -/
  tokenId    : 𝔽
  /-- Spend public keys of the two output recipients.
      `pkOut 0` = recipient's key;  `pkOut 1` = sender's own key (change). -/
  pkOut      : Fin 2 → 𝔽
  /-- Output amounts as naturals: `valueOut 0 + valueOut 1 = valueIn`. -/
  valueOut   : Fin 2 → Nat
  /-- Salts for the two output notes. -/
  saltOut    : Fin 2 → 𝔽

/-- **PaymentCircuit constraint satisfaction.**

    Mirrors the `Define` method of `PaymentCircuit` in `templates/Payment.go`.
    A witness `w` satisfies statement `s` when all the following hold:

    1. **Standalone check** — the on-chain message tag is zero.
    2. **Nullifier correctness** — `s.nf = Poseidon3(sk, leafIndex, contractAddr)`.
    3. **Merkle verification** — the input commitment authenticates to `s.merkleRoot`.
    4. **Output commitment 0** — payment commitment is well-formed.
    5. **Output commitment 1** — change commitment is well-formed.
    6. **Change ownership** — change output uses the sender's public key.
    7. **Value conservation** — input amount equals sum of output amounts (in `Nat`).

    *Scope*: models only the non-zero-value (real spend) case. -/
noncomputable def PaymentCircuit.Satisfies (s : PaymentStatement) (w : PaymentWitness) : Prop :=
  let pk := spendPk w.sk
  let inputCommit := commitmentN pk w.saltIn w.valueIn w.tokenId
  -- (1) Message tag is zero (standalone payment)
  s.messageIsZero ∧
  -- (2) Nullifier correctly formed
  s.nf = nullifier w.sk w.leafIndex s.contractAddr ∧
  -- (3) Input commitment authenticates against the Merkle root
  merkleRoot inputCommit w.path = s.merkleRoot ∧
  -- (4) Payment output commitment is well-formed
  s.commitmentsOut 0 = commitmentN (w.pkOut 0) (w.saltOut 0) (w.valueOut 0) w.tokenId ∧
  -- (5) Change output commitment is well-formed
  s.commitmentsOut 1 = commitmentN (w.pkOut 1) (w.saltOut 1) (w.valueOut 1) w.tokenId ∧
  -- (6) Change output is owned by the sender
  w.pkOut 1 = pk ∧
  -- (7) Value conservation: input = payment + change (Nat arithmetic)
  w.valueIn = w.valueOut 0 + w.valueOut 1

/-! ## Private Mint Circuit -/

/-- Public inputs for the PrivateMint circuit. -/
structure PrivateMintStatement where
  /-- The minted commitment: Poseidon4(pk, salt, amount, tokenId). -/
  commitment'    : 𝔽
  /-- The vault contract address. -/
  contractAddr   : 𝔽
  /-- The token being minted. -/
  tokenId        : 𝔽
  /-- The ciphertext tag: Poseidon3(pk, salt, contractAddr).
      Lets the recipient detect the mint without revealing the amount. -/
  ciphertext     : 𝔽

/-- Private witness for the PrivateMint circuit. -/
structure PrivateMintWitness where
  /-- Recipient's spend public key. -/
  pk     : 𝔽
  /-- Random salt for the commitment. -/
  salt   : 𝔽
  /-- Amount being privately minted (as a natural number). -/
  amount : Nat

/-- **PrivateMintCircuit constraint satisfaction.**

    Mirrors `PrivateMintCircuit.Define` in `templates/PrivateMint.go`.

    1. **Commitment well-formedness**: `s.commitment' = Poseidon4(pk, salt, amount, tokenId)`.
    2. **Ciphertext well-formedness**: `s.ciphertext = Poseidon3(pk, salt, contractAddr)`. -/
noncomputable def PrivateMintCircuit.Satisfies (s : PrivateMintStatement) (w : PrivateMintWitness) : Prop :=
  -- (1) Commitment matches opening
  s.commitment' = commitmentN w.pk w.salt w.amount s.tokenId ∧
  -- (2) Ciphertext tag is correctly formed
  s.ciphertext = poseidon3 w.pk w.salt s.contractAddr

end Enygma
