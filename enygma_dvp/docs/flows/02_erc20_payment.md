# 02 — ERC20 Payment (JoinSplit with Change)

**Test:** `TestV2Erc20Payment`
**File:** `test/02_v2_erc20_payment_test.go`

Alice deposits 40 USDT and privately pays 30 USDT to Bob, keeping 10 USDT as change.
The Payment circuit consumes 2 input notes and produces 2 output notes.
Bob's output is encrypted to his ML-KEM view key, so only he can decrypt it. Alice's change
has no ciphertext: its salt is `Poseidon(sk_A, nf0)`, so she rebuilds it from her spend key,
the spent note's nullifier and the change amount.

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
    Alice->>Alice: encData = EncryptPayload(encKey, tokenId=0, amount=40) (AES-256-GCM)
    Alice->>Vault: depositV2([40, pk_A, saltBField, 0], capsule, encData)
    Vault->>Vault: commitment = Poseidon4(pk_A, saltBField, 40, 0)
    Vault->>Vault: insert commitment as Merkle leaf
    Vault-->>Alice: emit Commitment(vaultId, commitment)
    Vault-->>Alice: emit EncryptedNote(vaultId, commitment, capsule, encData)
    Alice->>Alice: build Merkle proof for commitment

    Note over Alice,GnarkServer: Step 2 — Payment proof (BoundPaymentProof)
    Bob->>Bob: NewSpendKeyPair() → (sk_B, pk_B)
    Bob->>Bob: NewViewKeyPair() → (decapsKey_B, encapsKey_B)
    Alice->>Alice: Encapsulate(encapsKey_B) → (ss_B, cipherText)
    Alice->>Alice: DerivePaymentSalt(ss_B) → saltB_B, DerivePaymentKey(ss_B) → encKey_B
    Alice->>Alice: encTxData = EncryptPayload(encKey_B, tokenId=0, amount=30)
    Alice->>Alice: nf0 = nullifier of her 40 USDT note<br/>change salt = WithdrawChangeSalt(sk_A, nf0) = Poseidon(sk_A, nf0)
    Alice->>GnarkServer: POST /proof/payment2in<br/>inputs=[40, dummy=0]<br/>saltsIn=[saltBField, 0]<br/>outputs=[30→pk_B, 10→pk_A]<br/>saltsOut=[saltBField_B, change salt]<br/>ciphertextHash of (cipherText, encTxData)
    GnarkServer->>GnarkServer: prove JoinSplit: 40 = 30 + 10
    GnarkServer-->>Alice: proof[8], statement[11]

    Note over Alice,DVP: Step 2 (cont.) — On-chain submission
    Alice->>DVP: payment(receipt, vaultId=0, cipherText, encTxData)
    DVP->>Vault: checkReceiptConditions(receipt): proof, vault, root, nullifier unspent
    DVP->>DVP: receipt's CiphertextHash == hash of (cipherText, encTxData)
    DVP->>Vault: nullifyFromReceipt, then insertCommitmentsFromReceipt
    Vault-->>Alice: emit Nullifier(vaultId, treeNum, nf0)<br/>emit Commitment for cmtBob and cmtAliceChange
    DVP-->>Alice: emit Payment(vaultId, cmtBob, cipherText, encTxData)  ← Bob's note only

    Note over Bob: Step 3 — Bob scans his 30 USDT note
    Bob->>Bob: Decapsulate(decapsKey_B, cipherText) → ss_B
    Bob->>Bob: DerivePaymentSalt(ss_B) → saltB_B
    Bob->>Bob: DerivePaymentKey(ss_B) → encKey_B
    Bob->>Bob: DecryptPayload(encKey_B, encTxData) → tokenId=0, amount=30
    Bob->>Bob: Poseidon4(pk_B, saltBField_B, 30, 0) == cmtBob ✓

    Note over Alice: Step 4 — Alice keeps her 10 USDT change note
    Alice->>Alice: WithdrawChangeNote(sk_A, nf0, 10, 0) → (change salt, cmtAliceChange)<br/>no salt stored: nf0 is in the Nullifier event
```

---

## Circuit Statement Layout (Payment2in, 11 elements)

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
| 10 | `CiphertextHash` | Hash of the note ciphertexts the payment publishes (`CiphertextHash.sol`) |

## Key Contracts

The deposit's calldata holds the note's whole opening (Alice's spend public key, the salt and the amount), so it is public who owns the deposited note; see [what the chain sees](../../dvp_protocol.md#what-the-chain-sees-deposits-spends-and-exits).

| Contract | Function | Purpose |
|----------|----------|---------|
| `Erc20CoinVault` | `depositV2(params, ciphertextI, ciphertextII)` | Deposit with an `EncryptedNote`; `params = [amount, pkSpend, salt, tokenId]` and the vault computes the commitment |
| `EnygmaDvp` | `payment(receipt, vaultId, ctxt, encTxData)` | JoinSplit entry point; publishes output 0's ciphertexts only |
| `GenericGroth16Verifier` | `verifyProof(vk, proof, statement)` | On-chain proof check |
