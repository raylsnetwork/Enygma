# Gnark Server — Poseidon Utility Endpoints

Base URL: `http://localhost:8081`

> **Note:** These endpoints do not currently exist in the codebase — the handler
> files (`server/utils/poseidon_encrypt.go`, `poseidon_decrypt.go`) were removed
> 2026-09-21, since their routes were already unregistered and nothing called
> them. This doc is kept as a spec for re-adding them, not a description of
> current behavior. The underlying native functions they would have wrapped
> (see the bottom of this doc) are unaffected and still used internally by the
> auditor circuits.

All numeric values are **decimal strings** (big integers over the BN254 scalar field).

---

## POST /util/poseidonEncrypt

Encrypts a plaintext array using the Poseidon symmetric encryption scheme.

### Request

```json
{
  "key":        ["string", "string"],
  "nonce":      "string",
  "realLength": 3,
  "plaintext":  ["string", "string", "string"]
}
```

| Field | Type | Description |
|---|---|---|
| `key` | `[2]string` | Two field elements forming the symmetric key |
| `nonce` | `string` | Encryption nonce (field element) |
| `realLength` | `int` | Number of meaningful plaintext elements — must equal `len(plaintext)` |
| `plaintext` | `[]string` | Field elements to encrypt |

### Response

```json
{
  "encrypted": ["string", "string", "string", "string"]
}
```

| Field | Type | Description |
|---|---|---|
| `encrypted` | `[]string` | Ciphertext — length is `ceil(realLength / 3) * 3 + 1` |

---

## POST /util/poseidonDecrypt

Decrypts a ciphertext array produced by `/util/poseidonEncrypt`.

### Request

```json
{
  "key":        ["string", "string"],
  "nonce":      "string",
  "realLength": 3,
  "encrypted":  ["string", "string", "string", "string"]
}
```

| Field | Type | Description |
|---|---|---|
| `key` | `[2]string` | Same key used during encryption |
| `nonce` | `string` | Same nonce used during encryption |
| `realLength` | `int` | Number of plaintext elements to recover |
| `encrypted` | `[]string` | Ciphertext from `/util/poseidonEncrypt` |

### Response

```json
{
  "plaintext": ["string", "string", "string"]
}
```

| Field | Type | Description |
|---|---|---|
| `plaintext` | `[]string` | Recovered plaintext — `realLength` elements |

---

## Poseidon encryption scheme

The implementation follows the Poseidon sponge-based symmetric encryption used in the Enygma auditor circuits:

- Permutation: Poseidon over BN254 scalar field
- Absorption rate: 3 field elements per block
- Ciphertext length: `ceil(realLength / 3) * 3 + 1` elements (includes authentication tag)
- Key: two field elements `[k0, k1]`
- Nonce: one field element

The same scheme is implemented in gnark circuits via `NativePoseidonEncrypt` / `NativePoseidonDecrypt` in `gnark_circuits/poseidon/native.go`.
