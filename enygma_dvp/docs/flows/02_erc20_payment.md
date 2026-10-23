# 02 — ERC20 Payment (JoinSplit with Change)

**Test:** `TestV2Erc20Payment`
**File:** `test/02_v2_erc20_payment_test.go`

Alice deposits 40 USDT and privately pays 30 USDT to Bob, keeping 10 USDT as change.
The Payment circuit consumes 2 input notes and produces 2 output notes.
Each output is encrypted with the recipient's ML-KEM public key so only they can decrypt it.

---

## Diagram

```mermaid
sequenceDiagram
    participant Alice
    participant Bob
    participant ERC20 as RaylsERC20
    participant Vault as Erc20CoinVault
    participant GnarkServer as Gnark Server :8081
    participant DVP as EnygmaDvp

    Note over Alice,Vault: Setup — Mint & approve
    Alice->>ERC20: mint(alice, 400)
    Alice->>ERC20: approve(vault, 40)

    Note over Alice,Vault: Step 1 — depositV2 (40 USDT)
    Alice->>Alice: NewSpendKeyPair() → (sk_A, pk_A)
    Alice->>Alice: NewViewKeyPair() → (decapsKey_A, encapsKey_A)
    Alice->>Alice: Encapsulate(encapsKey_A) → (ss, capsule)
    Alice->>Alice: DerivePaymentSalt(ss) → saltB
    Alice->>Alice: DerivePaymentKey(ss) → encKey
    Alice->>Alice: commitment = Poseidon4(pk_A, saltBField, 40, 0)
    Alice->>Alice: encData = ChaCha20Enc(encKey, tokenId=0, amount=40)
    Alice->>Vault: depositV2([40, commitment], capsule, encData)
    Vault->>Vault: insert commitment as Merkle leaf
    Vault-->>Alice: emit Commitment(treeNum, leafIndex, commitment)
    Alice->>Alice: build Merkle proof for commitment

    Note over Alice,GnarkServer: Step 2 — Payment proof
    Bob->>Bob: NewSpendKeyPair() → (sk_B, pk_B)
    Bob->>Bob: NewViewKeyPair() → (decapsKey_B, encapsKey_B)
    Alice->>GnarkServer: POST /proof/payment<br/>inputs=[40, dummy=0]<br/>saltsIn=[saltBField, 0]<br/>outputs=[30→pk_B, 10→pk_A]<br/>viewKeys=[encapsKey_B, encapsKey_A]
    GnarkServer->>GnarkServer: prove JoinSplit: 40 = 30 + 10
    GnarkServer->>GnarkServer: ML-KEM encrypt output 0 for Bob
    GnarkServer->>GnarkServer: ML-KEM encrypt output 1 for Alice
    GnarkServer-->>Alice: proof[8], statement[9]<br/>cipherText[2], encTxData[2]

    Note over Alice,DVP: Step 2 (cont.) — On-chain submission
    Alice->>DVP: payment(vaultId=0, receipt, cipherTexts, encTxDatas)
    DVP->>DVP: verifyProof via GenericGroth16Verifier
    DVP->>DVP: check nullifier not spent
    DVP->>DVP: emit Nullifier(treeNum, leafIdx, nullifier)
    DVP->>Vault: registerCoins([cmtBob, cmtAliceChange])
    DVP-->>Alice: emit Payment(vaultId, leafIdx, cipherText[0], encTxData[0])  ← Bob's note
    DVP-->>Alice: emit Payment(vaultId, leafIdx, cipherText[1], encTxData[1])  ← Alice's change

    Note over Bob: Step 3 — Bob scans his 30 USDT note
    Bob->>Bob: Decapsulate(decapsKey_B, cipherText[0]) → ss_B
    Bob->>Bob: DerivePaymentSalt(ss_B) → saltB_B
    Bob->>Bob: DerivePaymentKey(ss_B) → encKey_B
    Bob->>Bob: DecryptPayload(encKey_B, encTxData[0]) → tokenId=0, amount=30
    Bob->>Bob: Poseidon4(pk_B, saltBField_B, 30, 0) == cmtBob ✓

    Note over Alice: Step 4 — Alice scans her 10 USDT change note
    Alice->>Alice: Decapsulate(decapsKey_A, cipherText[1]) → ss_A2
    Alice->>Alice: DerivePaymentSalt(ss_A2) → saltB_change
    Alice->>Alice: DerivePaymentKey(ss_A2) → encKey_change
    Alice->>Alice: DecryptPayload(encKey_change, encTxData[1]) → tokenId=0, amount=10
    Alice->>Alice: Poseidon4(pk_A, saltBField_change, 10, 0) == cmtAliceChange ✓
```

---

## Circuit Statement Layout (Payment2in, 10 elements)

Grouped by field, in the order of the Payment2in statement layout
(`layout.Payment2in*` in `enygma_dvp/layout`, `PAYMENT2IN_*` in
`PaymentStatementLayout`); read offsets from there, not from this table.

| Index | Field | Description |
|-------|-------|-------------|
| 0 | `Message` | `0` (standalone payment) |
| 1 | `TreeNumbers[0]` | ERC20 vault tree number (Alice's note) |
| 2 | `TreeNumbers[1]` | `0` (dummy input) |
| 3 | `MerkleRoots[0]` | Merkle root at time of proof |
| 4 | `MerkleRoots[1]` | `0` (dummy) |
| 5 | `Nullifiers[0]` | Nullifier for Alice's 40 USDT note |
| 6 | `Nullifiers[1]` | `0` (dummy) |
| 7 | `CommitmentsOut[0]` | Bob's 30 USDT commitment |
| 8 | `CommitmentsOut[1]` | Alice's 10 USDT change commitment |
| 9 | `ContractAddress` | Vault the proof is bound to |

## Key Contracts

| Contract | Function | Purpose |
|----------|----------|---------|
| `Erc20CoinVault` | `depositV2(amounts, capsule, encData)` | ML-KEM deposit |
| `EnygmaDvp` | `payment(vaultId, receipt, ciphers, encDatas)` | JoinSplit entry point |
| `GenericGroth16Verifier` | `verifyProof(vk, proof, statement)` | On-chain proof check |
