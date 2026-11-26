# Source of Truth

This document explains the design decisions behind Enygma Payment and EnygmaDvP.
Each section answers a single "why" question — the problem it solves, why this
primitive was chosen over alternatives, and where it appears in the codebase.

---

## Table of Contents

**Shared Primitives**

- [Why Poseidon Hash?](#why-poseidon-hash)
- [Why Groth16?](#why-groth16)
- [Why BN254 (alt-bn128)?](#why-bn254-alt-bn128)
- [Why gnark?](#why-gnark)

**Enygma Payment**

- [Why BabyJubJub Elliptic Curve?](#why-babyjubjub-elliptic-curve)
- [Why Pedersen Commitment?](#why-pedersen-commitment)
- [Why Range Proofs?](#why-range-proofs)
- [Why an Anonymity Set?](#why-an-anonymity-set)
- [Why Private Messaging Tags?](#why-private-messaging-tags)
- [Why a Running Balance Model Instead of UTXO?](#why-a-running-balance-model-instead-of-utxo)

**EnygmaDvP**

- [Source of Truth](#source-of-truth)
  - [Table of Contents](#table-of-contents)
  - [Shared Primitives](#shared-primitives)
    - [Why Poseidon Hash?](#why-poseidon-hash)
    - [Why Groth16?](#why-groth16)
    - [Why BN254 (alt-bn128)?](#why-bn254-alt-bn128)
    - [Why gnark?](#why-gnark)
  - [Enygma Payment](#enygma-payment)
    - [Why BabyJubJub Elliptic Curve?](#why-babyjubjub-elliptic-curve)
    - [Why Pedersen Commitment?](#why-pedersen-commitment)
    - [Why Range Proofs?](#why-range-proofs)
    - [Why an Anonymity Set?](#why-an-anonymity-set)
    - [Why Private Messaging Tags?](#why-private-messaging-tags)
    - [Why a Running Balance Model Instead of UTXO?](#why-a-running-balance-model-instead-of-utxo)
  - [EnygmaDvP](#enygmadvp)
    - [Why AES-256-GCM?](#why-aes-256-gcm)
    - [Why ML-KEM-768 (Kyber)?](#why-ml-kem-768-kyber)
    - [Why HKDF?](#why-hkdf)
    - [Why Separate Spend Keys and View Keys?](#why-separate-spend-keys-and-view-keys)
    - [Why a Merkle Tree?](#why-a-merkle-tree)
    - [Why Is Each Token Standard Stored in Its Own Vault?](#why-is-each-token-standard-stored-in-its-own-vault)
    - [Why Include a Salt in the Commitment?](#why-include-a-salt-in-the-commitment)

---

## Shared Primitives

### Why Poseidon Hash?

ZK circuits count arithmetic operations. SHA-256 uses bitwise logic that is cheap on
a CPU but costs ~25,000 operations inside a circuit. Poseidon's core step is `x^5` —
one multiplication — so a full hash costs ~250 operations. 100x cheaper.

**Where it is used in Enygma Payment**:

- **Spend key derivation**: `pk = Hash(sk)` — ZK-friendly public key from private key
- **Nullifier**: `Hash(sk, n_block)` — marks a note spent without revealing the sender
- **Private messaging tags**: `Hash(shared_secret, n_block)` — recipient detection

**Where it is used in EnygmaDvP**:

- **Commitment**: `Poseidon(pk, salt, amount, tokenId)` — hides note contents on-chain
- **Nullifier**: `Poseidon(sk, treeNumber * 2^depth + leafIndex)` — marks a note as spent without revealing it
- **Merkle tree**: `Poseidon(left, right)` — each tree node, used in inclusion proofs

---

### Why Groth16?

On a blockchain, proof size and verification gas cost are what matter most.

Groth16 produces the smallest proofs of any system in use — always 3 curve points
(192 bytes), verified for ~280,000 gas. PLONK is ~600 bytes. STARKs are tens of
kilobytes and not yet practical to verify on-chain.

The trade-off is a one-time trusted setup per circuit. If any one participant
discards their secret, the setup is secure.

**Where it is used in Enygma Payment**:

- **Every circuit**: the transfer circuit is compiled and set up using Groth16 over BN254.
- **On-chain verifier**: the smart contract verifies the Groth16 proof on every transaction.

**Where it is used in EnygmaDvP**:

- **Every circuit**: all circuits — ERC20 JoinSplit, ERC721 Ownership, DvP Initiator,
  DvP Destination, Payment, Auditor — are compiled and set up using Groth16 over BN254.
- **On-chain verifier**: a single Solidity contract verifies any Groth16/BN254 proof
  given a registered verifying key. All proof submissions flow through this contract.

---

### Why BN254 (alt-bn128)?

Groth16 requires a pairing-friendly elliptic curve — a curve that supports a special
mathematical operation (a bilinear pairing) used to verify the proof. Not all curves
support this. Among those that do, the cost of the pairing operation determines the
gas cost of on-chain verification.

BN254 is the only pairing-friendly curve with a precompile built into Ethereum
(EIP-196, EIP-197). A precompile is a native EVM operation, not Solidity code —
it runs orders of magnitude faster and cheaper. Verifying a Groth16 proof over BN254
costs ~280,000 gas. Doing the same over any other curve in pure Solidity would cost
millions of gas and be practically unusable.

It has ~100-bit security — below the 128-bit standard. This is a known trade-off
accepted by the Ethereum community when BN254 was standardized. For the threat model
of private asset transfers, it remains sufficient today.

**Where it is used in Enygma Payment**:

- **Proof generation**: the transfer circuit is compiled over BN254's scalar field,
  which is also why Poseidon and BabyJubJub are chosen — they are native to this field.
- **On-chain verification**: the Groth16 verifier contract calls the BN254 pairing
  precompile directly, keeping verification gas low for every proof submission.

**Where it is used in EnygmaDvP**:

- **Proof generation**: all circuits are compiled over BN254's scalar field, which is
  also why Poseidon and BabyJubJub are chosen — they are native to this same field.
- **On-chain verification**: the Groth16 verifier contract calls the BN254 pairing
  precompile directly, keeping verification gas low for every proof submission.

---

### Why gnark?

ZK circuits need a proving framework — a library that compiles the circuit,
runs the trusted setup, and generates proofs. The main options are Circom
(compiles to WebAssembly, proven in JS/browser) and gnark (Go-native, runs
server-side).

Enygma is a server-side system used by regulated financial institutions.
There is no browser prover. The prover runs as a Go service alongside the
rest of the backend. gnark fits this exactly — it is a Go library, proofs
are generated in-process, and there is no JavaScript dependency or
WebAssembly runtime to manage.

gnark also compiles circuits directly to Go structs. The circuit definition,
witness assignment, and proof generation all live in the same language and
toolchain as the rest of the server code. This makes the circuit easier to
audit, test, and integrate.

Besides that gnark has far superior performance on proof generation time comparing to Circom and other Zk Snarks proof tested.

---

## Enygma Payment

### Why BabyJubJub Elliptic Curve?

ZK circuits run inside BN254's scalar field. Curve arithmetic inside a
circuit must use a curve native to that same field — otherwise every point
addition requires field emulation (~30,000 constraints).

BabyJubJub is native to BN254's field. A point addition costs 3–4
constraints instead of 30,000.

It has ~120-bit security — slightly below the 128-bit standard. Acceptable
because each key is tied to a single note, not a permanent identity.

**Where it is used in Enygma Payment**:

- **Spend key**: the user's BabyJubJub private key proves ownership inside
  the circuit. The public key is embedded in every output note.
- **Stealth addresses**: a fresh one-time public key is derived per payment
  from the recipient's BabyJubJub key, so on-chain notes from different
  senders are unlinkable.

---

### Why Pedersen Commitment?

In Enygma Payment, transfer amounts must stay hidden on-chain. (Mint, USDr
mint, burn, withdraw and deposit publish their amounts, and fees are
public; see
`enygma_payments/protocol_description.md` § 8.) But the circuit still
needs to prove that no value was created or destroyed.

Pedersen commitments make both possible at once. A commitment to value `v`
with blinding factor `r` is:

```
C = v·G + r·H
```

It hides `v` completely, and it is **additively homomorphic**:
`C(v₁) + C(v₂) = C(v₁ + v₂)`. So instead of checking amounts directly,
the circuit checks:

```
sum(input commitments) = sum(output commitments) + fee commitment
```

No transfer amounts are revealed (the fee amount is public). Balance is
proven by a single curve equation.

The blinding factor `r` is derived per note from the shared secret between
sender and recipient, unknown to anyone else.

**Where it is used in Enygma Payment**:

- **Output notes**: each note commits to its amount as `C(amount, r)`,
  stored on-chain in the Merkle tree.
- **Balance proof**: the circuit asserts input and output commitments
  balance, proving conservation of value without revealing any amount.
- **Range proof**: a sub-circuit proves each amount is non-negative,
  preventing negative-value exploits.

---

### Why Range Proofs?

Pedersen commitments hide amounts. That means the contract cannot check
whether an output amount is negative — a malicious prover could commit
to `-100` to inflate another output.

A range proof is a ZK sub-circuit that proves `0 ≤ v < 2^n` without
revealing `v`. It runs alongside the balance proof so the verifier
confirms every output amount is valid without learning what it is.

**Where it is used in Enygma Payment**:

- Every output note in a JoinSplit carries a range proof bounding its
  committed amount to a valid non-negative range.

---

### Why an Anonymity Set?

Each transaction publishes `k` Pedersen commitments — one per participant
in the anonymity set. One commitment is a debit (the sender), the rest are
credits. An on-chain observer sees `k` commitments that sum to zero but
cannot tell which one belongs to the sender and which are recipients.

Without an anonymity set, every transaction would have one input and one
or more outputs, immediately revealing who is spending.

**Where it is used in Enygma Payment**:

- Every transaction carries exactly `k` commitments. The sender's index
  inside the set is known only to the sender, hidden from the contract
  and any observer.

---

### Why Private Messaging Tags?

After a transaction is posted, recipients need to detect if a payment was
sent to them — without asking a server (which would leak who they are
watching).

Each transaction includes `k` tags, one per anonymity set member:

```
t_i = Hash(shared_secret(i, sender), n_block)
```

A recipient computes the tag they expect from each possible sender and
compares it against the published tags. A match means there is a payment
for them in that transaction. No match means they were just cover traffic.

This lets every privacy node scan locally without revealing to anyone
which transactions they are checking.

**Where it is used in Enygma Payment**:

- Every transaction carries `k` tags. Recipients brute-force scan them
  locally each block to detect incoming payments.

---

### Why a Running Balance Model Instead of UTXO?

In a UTXO model each payment creates and consumes individual notes.
For financial institutions making frequent, high-volume payments this
adds overhead — proofs must reference specific input notes and build
Merkle inclusion paths for each one.

Enygma Payment uses a running balance: each privacy node has a single
shielded balance on-chain, updated atomically each block. A transaction
debits the sender's balance and credits the recipients'. There are no
individual notes to track or inclusion proofs to construct.

This makes the circuit simpler and proof generation faster, which is
the right trade-off for regulated institutions transacting continuously.

**Where it is used in Enygma Payment**:

- Each privacy node holds one Pedersen commitment on-chain representing
  their current balance. Every transaction updates it in place.

---

## EnygmaDvP

### Why AES-256-GCM?

Once two parties share a secret key, they need a symmetric cipher to encrypt the
actual payload. AES-256-GCM is the standard choice for this.

AES-256 provides 256-bit key security — the strongest standardized symmetric cipher.
GCM (Galois/Counter Mode) adds **authenticated encryption**: it produces an
authentication tag alongside the ciphertext. If anyone tampers with the ciphertext
in transit, decryption fails and the tampered data is rejected before it is used.
This prevents an attacker from flipping bits in an encrypted note to alter the amount
or tokenId without detection.

It is also hardware-accelerated on virtually every modern CPU (AES-NI instructions),
making encryption and decryption fast regardless of payload size.

**Where it is used in EnygmaDvP**:

- **Note payloads**: `tokenId || amount` is encrypted with AES-256-GCM so only the
  intended recipient can learn the value of a note sent to them.
- **Payment and deposit flows**: each output note gets its own AES-256-GCM ciphertext,
  emitted on-chain inside `EncryptedNote` and `Payment` events for recipient scanning.

---

### Why ML-KEM-768 (Kyber)?

AES-256-GCM encrypts the payload, but first both parties need to agree on a shared
secret key. The classical approach is ECDH — each party has a keypair and they compute
a shared secret from each other's public key. But ECDH is broken by a quantum computer
running Shor's algorithm.

ML-KEM-768 (formerly Kyber-768) is a **Key Encapsulation Mechanism** standardized by
NIST in 2024 as the primary post-quantum replacement for ECDH. It is based on the
hardness of the Module Learning With Errors (MLWE) problem, which has no known quantum
attack. Security level: ~180-bit classical, ~180-bit quantum.

The flow in EnygmaDvP is:

1. Recipient publishes an ML-KEM public key (encapsulation key).
2. Sender calls `Encapsulate(pk)` → gets a shared secret `ss` and a capsule (1088 bytes).
3. Sender derives an AES-256-GCM key from `ss` via HKDF, encrypts the payload.
4. Capsule + ciphertext are emitted on-chain. Recipient decapsulates with their private
   key to recover `ss`, derives the same AES key, and decrypts.

The sender never needs the recipient's private key, and an eavesdropper who records the
capsule today cannot decrypt it later even with a future quantum computer.

**Where it is used in EnygmaDvP**:

- **depositV2 / transferV2**: sender encapsulates the recipient's view key to derive
  the AES key for the note payload, emitting the capsule alongside the ciphertext.
- **DvP Initiator**: Alice encapsulates Bob's view key to encrypt the USDT note details
  so Bob can discover what he will receive before the swap completes.

---

### Why HKDF?

ML-KEM outputs a shared secret `ss` — a random-looking 32-byte value. It is tempting
to use it directly as an encryption key. The older swap flow does exactly this, keying
ChaCha20-Poly1305 with `ss` directly.

The problem is that one shared secret is used for two purposes: deriving the commitment
salt (`saltB`) and deriving the AES encryption key. Using the same bytes for both means
a weakness in one derivation could leak information about the other.

HKDF (HMAC-based Key Derivation Function) solves this by **domain-separating** the
outputs. From a single `ss` it produces independent, cryptographically isolated keys:

```
saltB   = HKDF(ss, info="Bob salt")
encKey  = HKDF(ss, info="encryption key")
```

Each output is computationally independent — knowing `saltB` tells an attacker nothing
about `encKey`, and vice versa. This is standard practice whenever one shared secret
needs to serve multiple purposes.

**Where it is used in EnygmaDvP**:

- **V2 deposit and payment flows**: every output note derives its commitment salt and
  AES-256-GCM key independently from the same ML-KEM shared secret via HKDF.
- The older swap flow skips HKDF and uses `ss` directly — a known limitation that V2
  was designed to fix.

---

### Why Separate Spend Keys and View Keys?

A private note has two operations: **spending** it (transferring ownership) and
**viewing** it (learning its value and tokenId). These require different levels of trust.

If a single key controlled both, sharing it with anyone — an auditor, a compliance
officer, a wallet service — would also give them the ability to spend the funds.
That is an unacceptable risk.

EnygmaDvP separates them into two independent keypairs:

- **Spend key** (BabyJubJub): used inside the ZK circuit to prove ownership and
  generate the nullifier. It never leaves the user's device. Sharing it means
  losing the funds.

- **View key** (ML-KEM-768): used only for note discovery — the sender encapsulates
  it to encrypt `tokenId || amount`. Sharing it with an auditor reveals what notes
  you have received, but gives no ability to move funds.

This mirrors the design of Zcash's incoming viewing keys and is a standard pattern
in privacy-preserving payment systems.

**Where it is used in EnygmaDvP**:

- **Spend key**: committed to inside every circuit via the nullifier and commitment.
  Required to generate any valid proof.
- **View key**: published (or shared selectively) so senders can encrypt notes to you.
  Used by recipients to scan `EncryptedNote` and `Payment` events on-chain.

---

### Why a Merkle Tree?

The ZK circuit needs to prove one thing: "this note exists and I own it." To do that
without revealing which note it is, the circuit proves a Merkle inclusion — that the
commitment is a leaf somewhere in a tree whose root is public on-chain.

The alternative is storing every commitment in a mapping and checking membership
directly. But a membership check inside a ZK circuit over an on-chain mapping is not
possible — the circuit has no access to contract storage. It can only work with values
passed in as witnesses. So the circuit needs a data structure it can verify with
arithmetic alone.

A Merkle tree fits exactly:

- The **root** is a single public value stored on-chain. The circuit takes it as a
  public input and verifies the proof against it.
- The **inclusion proof** (sibling hashes along the path from leaf to root) is passed
  as a private witness. The circuit recomputes the root from the leaf and siblings
  using Poseidon, then asserts it matches the public root.
- This costs exactly `depth` Poseidon hashes inside the circuit — cheap and fixed.

The tree is **append-only** because commitments are never deleted. Spending a note
nullifies it (records the nullifier on-chain) but leaves the commitment in the tree.
This means old Merkle roots remain valid — a proof generated against a past root is
still accepted, because the contract stores all historical roots.

**Where it is used in EnygmaDvP**:

- **Each vault** has its own Merkle tree. ERC20 and ERC721 commitments live in
  separate trees so proofs across asset types cannot be mixed.
- **Every spend circuit** — JoinSplit, Ownership, DvP Initiator, DvP Destination —
  includes a Merkle inclusion proof as a private witness to prove the input note exists.

---

### Why Is Each Token Standard Stored in Its Own Vault?

The alternative is one shared vault for all assets. The problem with that is the
commitment formula.

Both ERC20 and ERC721 notes use the same commitment formula:
`Poseidon(pk, salt, amount, tokenId)`. For ERC721, `amount` is always hardcoded to `1`
since a non-fungible token has no quantity. This means the commitment formats are
identical in structure — only the values differ.

That is exactly the problem. If both asset types shared one Merkle tree, a user could
present an ERC721 note (amount=1, tokenId=X) as if it were an ERC20 note with
amount=1, effectively spending a non-fungible token as fungible balance. The circuit
alone cannot tell them apart — only the verifying key used to check the proof can.

Separate vaults enforce a hard boundary:

- Each vault has its own Merkle tree. A commitment inserted in the ERC20 vault can
  only be spent by an ERC20 circuit proof verified against that vault's VK.
- The verifying key registered for a vault is specific to that asset type. A proof
  generated for ERC721 ownership will fail verification in the ERC20 vault and
  vice versa.
- Token balances are physically held by the vault contract. Only valid proofs can
  trigger a transfer out, and the vault only accepts proofs for its own asset type.

This also has a practical benefit: each vault is independently upgradeable and new
asset standards can be added by deploying a new vault without touching existing ones.

**Where it is used in EnygmaDvP**:

- `Erc20CoinVault` — holds ERC20 tokens, verifies JoinSplit and DvP Initiator proofs.
- `Erc721CoinVault` — holds ERC721 tokens, verifies Ownership and DvP Destination proofs.
- `EnygmaDvp` — the central registry that maps vault IDs to vault addresses and routes
  proof submissions to the correct vault.

---

### Why Include a Salt in the Commitment?

The commitment is `Poseidon(pk, salt, amount, tokenId)`. Without the salt it would be
`Poseidon(pk, amount, tokenId)` — a deterministic value that anyone can compute from
public information.

This breaks privacy in two ways:

**1. Linkability.** If Alice always receives 50 USDT, every commitment for her with
amount=50 produces the same hash. An observer can link all those commitments to the
same owner without knowing Alice's key — just by recognizing the repeated value.

**2. Brute-force.** The set of plausible `(amount, tokenId)` pairs is small. An
attacker can precompute `Poseidon(pk, amount, tokenId)` for all likely values and
scan the on-chain commitment list to find matches. This reveals both the owner and
the note value.

The salt is a random value derived from the ML-KEM shared secret between sender and
recipient. It is unique per note, so two notes with the same owner and amount produce
completely different commitments. An outside observer sees only random-looking hashes
with no pattern to exploit.

The salt also ensures that even if a user receives the same amount from the same
sender twice, the two commitments are unlinkable on-chain.

**Where it is used in EnygmaDvP**:

- `saltB` is derived via `HKDF(ss, "Bob salt")` from the ML-KEM shared secret, making
  it unique per note and unknown to anyone who did not participate in the encapsulation.
- The circuit takes `salt` as a private witness and verifies the commitment matches —
  proving the prover knows the salt without revealing it.
