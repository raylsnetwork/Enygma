# Gnark Server — Proof Endpoints

Base URL: `http://localhost:8081`

All proof endpoints accept a JSON body, compile the circuit on the fly, generate a Groth16 proof over BN254, verify it locally, and return the proof and public signal in Solidity/Remix-compatible format.

All numeric values are **decimal strings** (big integers).

---

## POST /proof/privateMint

Generates a proof that a privately minted commitment is correctly formed from the owner's public key, token ID, amount, and salt — without revealing the private inputs.

### Request

```json
{
  "commitment":      "string (required) — Poseidon4(pkSpend, salt, amount, tokenId)",
  "contractAddress": "string (required) — address of the vault contract (as uint256)",
  "tokenId":         "string (required) — ERC-20 token identifier",
  "salt":            "string (required) — random salt (private)",
  "amount":          "string (required) — token amount being minted (private)",
  "publicKey":       "string (required) — owner's spend public key",
  "cipherText":      "string (required) — encrypted note data for the recipient"
}
```

| Field | Visibility | Description |
|---|---|---|
| `commitment` | public | Output leaf inserted into the vault Merkle tree |
| `contractAddress` | public | Vault contract address as a field element |
| `tokenId` | public | Token ID |
| `cipherText` | public | Encrypted note (for off-chain note discovery) |
| `salt` | private | Random blinding factor |
| `amount` | private | Minted amount |
| `publicKey` | private | Owner's spend public key |

### Response

```json
{
  "proof": [
    "Ar.X", "Ar.Y",
    "Bs.X.A1", "Bs.X.A0",
    "Bs.Y.A1", "Bs.Y.A0",
    "Krs.X", "Krs.Y"
  ],
  "publicSignal": [
    "commitment",
    "contractAddress",
    "tokenId",
    "cipherText"
  ]
}
```

| Field | Type | Description |
|---|---|---|
| `proof` | `[8]string` | Groth16 proof in Remix/Solidity order: `[Ar.X, Ar.Y, Bs.X.A1, Bs.X.A0, Bs.Y.A1, Bs.Y.A0, Krs.X, Krs.Y]` |
| `publicSignal` | `[4]string` | Public inputs in circuit order |

---

## POST /proof/payment

Generates a proof for a 2-input / 2-output private ERC-20 transfer. Input 0 is Alice's real note; input 1 is a dummy note (value = 0, Merkle check skipped by the circuit). Output 0 is the payment to Bob; output 1 is change back to Alice.

### Request

```json
{
  "stMessage":        "string (required) — public message / transaction hash",
  "stTreeNumbers":    ["string", "string"],
  "stMerkleRoots":    ["string", "string"],
  "stNullifiers":     ["string", "string"],
  "stCommitmentsOut": ["string", "string"],

  "wtPrivateKeysIn":      ["string", "string"],
  "wtValuesIn":           ["string", "string"],
  "wtSaltsIn":            ["string", "string"],
  "wtPathElements":       [["string x8"], ["string x8"]],
  "wtPathIndices":        ["string", "string"],
  "wtTokenId":            "string",
  "wtSpendPublicKeysOut": ["string", "string"],
  "wtValuesOut":          ["string", "string"],
  "wtSaltsOut":           ["string", "string"]
}
```

**Public inputs (`st` prefix):**

| Field | Size | Description |
|---|---|---|
| `stMessage` | 1 | Arbitrary public message (e.g. tx intent hash) |
| `stTreeNumbers` | [2] | Merkle tree number for each input note |
| `stMerkleRoots` | [2] | Merkle root for each input note |
| `stNullifiers` | [2] | Nullifier for each input note |
| `stCommitmentsOut` | [2] | Output commitments (leaves to insert) |

**Private witnesses (`wt` prefix):**

| Field | Size | Description |
|---|---|---|
| `wtPrivateKeysIn` | [2] | Spend private keys for input notes |
| `wtValuesIn` | [2] | Values of input notes |
| `wtSaltsIn` | [2] | Salts of input notes |
| `wtPathElements` | [2][8] | Merkle sibling paths for each input |
| `wtPathIndices` | [2] | Packed path direction bits for each input |
| `wtTokenId` | 1 | Token ID (same for all inputs/outputs) |
| `wtSpendPublicKeysOut` | [2] | Recipient spend public keys |
| `wtValuesOut` | [2] | Output note values |
| `wtSaltsOut` | [2] | Output note salts |

### Response

```json
{
  "proof": ["Ar.X", "Ar.Y", "Bs.X.A1", "Bs.X.A0", "Bs.Y.A1", "Bs.Y.A0", "Krs.X", "Krs.Y"],
  "publicSignal": [
    "stMessage",
    "stTreeNumbers[0]", "stMerkleRoots[0]", "stNullifiers[0]",
    "stTreeNumbers[1]", "stMerkleRoots[1]", "stNullifiers[1]",
    "stCommitmentsOut[0]",
    "stCommitmentsOut[1]"
  ]
}
```

| Field | Type | Description |
|---|---|---|
| `proof` | `[8]string` | Groth16 proof in Remix/Solidity order |
| `publicSignal` | `[9]string` | Public inputs — interleaved layout: `[msg, treeNum0, root0, nf0, treeNum1, root1, nf1, cmt0, cmt1]` |

> **Note:** The on-chain contract expects a non-interleaved layout. The Go client's `ContractStatement()` converts the interleaved public signal to `[msg, treeNums..., roots..., nullifiers..., commitments...]` before submission.

---

## POST /proof/dvpInitiator

Generates a proof for the initiator side of a Delivery-vs-Payment (DvP) swap. Alice locks her ERC-20 note and creates three output commitments: one for Bob (payment), one for Alice's change, and one revert commitment (returned to Alice if the swap is cancelled).

### Request

```json
{
  "stMessage":       "string (required)",
  "stTreeNumber":    "string (required)",
  "stMerkleRoot":    "string (required)",
  "stNullifier":     "string (required)",
  "stCommitB":       "string (required) — output commitment for Bob",
  "stCommitA":       "string (required) — change commitment for Alice",
  "stRevertCommitA": "string (required) — revert commitment for Alice",
  "stCounterVault":  "string (required) — address (decimal) of the vault Alice must be paid from",

  "wtSpendKeyIn":   "string (required)",
  "wtValueIn":      "string (required)",
  "wtSaltIn":       "string (required)",
  "wtTokenIdIn":    "string (required)",
  "wtPathElements": ["string x8"],
  "wtPathIndex":    "string (required)",

  "wtSpendPkBob": "string (required) — Bob's spend public key",
  "wtSaltB":      "string (required)",
  "wtValueBob":   "string (required)",
  "wtTokenIdBob": "string (required)",
  "wtSaltA":      "string (required)",
  "wtRevertSalt": "string (required)"
}
```

**Public inputs (`st` prefix):**

| Field | Description |
|---|---|
| `stMessage` | Public message / swap ID |
| `stTreeNumber` | Alice's input note tree number |
| `stMerkleRoot` | Alice's input note Merkle root |
| `stNullifier` | Alice's input note nullifier |
| `stCommitB` | Output commitment for Bob's payment |
| `stCommitA` | Output change commitment for Alice |
| `stRevertCommitA` | Revert commitment returned to Alice on cancellation |
| `stCounterVault` | Address of the vault Alice expects to be paid from (the counterparty's vault). Must be non-zero; `EnygmaDvp` rejects a settlement from any other vault with `CounterVaultMismatch` |

**Private witnesses (`wt` prefix):**

| Field | Description |
|---|---|
| `wtSpendKeyIn` | Alice's spend private key |
| `wtValueIn` | Alice's input note value |
| `wtSaltIn` | Alice's input note salt |
| `wtTokenIdIn` | Input token ID |
| `wtPathElements` | Merkle sibling path (8 elements) |
| `wtPathIndex` | Packed path direction bits |
| `wtSpendPkBob` | Bob's spend public key |
| `wtSaltB` | Salt for Bob's output note |
| `wtValueBob` | Value for Bob's output note |
| `wtTokenIdBob` | Bob's token ID |
| `wtSaltA` | Salt for Alice's change note |
| `wtRevertSalt` | Salt for Alice's revert note |

### Response

```json
{
  "proof": ["Ar.X", "Ar.Y", "Bs.X.A1", "Bs.X.A0", "Bs.Y.A1", "Bs.Y.A0", "Krs.X", "Krs.Y"],
  "publicSignal": [
    "stMessage",
    "stTreeNumber", "stMerkleRoot", "stNullifier",
    "stCommitB", "stCommitA", "stRevertCommitA",
    "stCounterVault"
  ]
}
```

| Field | Type | Description |
|---|---|---|
| `proof` | `[8]string` | Groth16 proof in Remix/Solidity order |
| `publicSignal` | `[8]string` | `[msg, treeNum, root, nullifier, commitB, commitA, revertCommitA, counterVault]` |

---

## POST /proof/dvpDestination

Generates a proof for the destination side of a DvP swap. Bob proves ownership of the asset being exchanged and creates one output commitment for Alice.

### Request

```json
{
  "stMessage":    "string (required)",
  "stTreeNumber": "string (required)",
  "stMerkleRoot": "string (required)",
  "stNullifier":  "string (required)",
  "stCommitA":    "string (required) — output commitment for Alice",
  "stCounterVault": "string (required) — address (decimal) of the vault Bob must be paid from",

  "wtSpendKeyIn":   "string (required)",
  "wtValueIn":      "string (required)",
  "wtSaltIn":       "string (required)",
  "wtTokenIdIn":    "string (required)",
  "wtPathElements": ["string x8"],
  "wtPathIndex":    "string (required)",

  "wtSpendPkAlice": "string (required) — Alice's spend public key",
  "wtSaltA":        "string (required)"
}
```

**Public inputs (`st` prefix):**

| Field | Description |
|---|---|
| `stMessage` | Public message / swap ID (must match initiator's) |
| `stTreeNumber` | Bob's input note tree number |
| `stMerkleRoot` | Bob's input note Merkle root |
| `stNullifier` | Bob's input note nullifier |
| `stCommitA` | Output commitment for Alice |
| `stCounterVault` | Address of the vault Bob expects to be paid from (Alice's vault). Must be non-zero; checked by `EnygmaDvp` at settlement |

**Private witnesses (`wt` prefix):**

| Field | Description |
|---|---|
| `wtSpendKeyIn` | Bob's spend private key |
| `wtValueIn` | Bob's input note value |
| `wtSaltIn` | Bob's input note salt |
| `wtTokenIdIn` | Input token ID |
| `wtPathElements` | Merkle sibling path (8 elements) |
| `wtPathIndex` | Packed path direction bits |
| `wtSpendPkAlice` | Alice's spend public key |
| `wtSaltA` | Salt for Alice's output note |

### Response

```json
{
  "proof": ["Ar.X", "Ar.Y", "Bs.X.A1", "Bs.X.A0", "Bs.Y.A1", "Bs.Y.A0", "Krs.X", "Krs.Y"],
  "publicSignal": [
    "stMessage",
    "stTreeNumber", "stMerkleRoot", "stNullifier",
    "stCommitA",
    "stCounterVault"
  ]
}
```

| Field | Type | Description |
|---|---|---|
| `proof` | `[8]string` | Groth16 proof in Remix/Solidity order |
| `publicSignal` | `[6]string` | `[msg, treeNum, root, nullifier, commitA, counterVault]` |

---

## Proof format

All proof endpoints return the Groth16 proof as 8 decimal strings in the order expected by Solidity verifiers:

```
[Ar.X, Ar.Y, Bs.X.A1, Bs.X.A0, Bs.Y.A1, Bs.Y.A0, Krs.X, Krs.Y]
```

Where `Bs` is a G2 point over Fp² — `A1` is the imaginary part and `A0` is the real part (EIP-197 convention).
