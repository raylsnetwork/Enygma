# 03 — DvP Atomic Swap (ERC20 ↔ ERC721)

**Test:** `TestV2DvP`
**File:** `test/03_v2_dvp_test.go`

Alice delivers 30 USDT (ERC20) to Bob. Bob delivers an ERC721 ticket (tokenId=42) to Alice.
The swap is atomic: both legs are proved separately but cross-reference each other's output commitments,
so neither party can claim their asset without the other's proof being valid.

---

## Diagram

```mermaid
sequenceDiagram
    participant Alice
    participant Bob
    participant ERC20Vault as Erc20CoinVault
    participant NFTVault as Erc721CoinVault
    participant GnarkServer as Gnark Server :8081
    participant DVP as EnygmaDvp

    Note over Alice,ERC20Vault: Step 1 — Alice deposits 30 USDT (ML-KEM)
    Alice->>Alice: NewSpendKeyPair() → (sk_A, pk_A)
    Alice->>Alice: NewViewKeyPair() → (decapsKey_A, encapsKey_A)
    Alice->>Alice: Encapsulate(encapsKey_A) → (ss_A, capsule_A)
    Alice->>Alice: saltBField_A = DerivePaymentSalt(ss_A)
    Alice->>Alice: commitment_A = Poseidon4(pk_A, saltBField_A, 30, 0)
    Alice->>ERC20Vault: depositV2([30, pk_A, saltBField_A, 0], capsule_A, encData_A)
    ERC20Vault-->>Alice: emit Commitment(...)

    Note over Bob,NFTVault: Step 2 — Bob deposits ERC721 ticket (tokenId=42)
    Bob->>Bob: NewSpendKeyPair() → (sk_B, pk_B)
    Bob->>Bob: NewViewKeyPair() → (decapsKey_B, encapsKey_B)
    Bob->>Bob: bobNftSalt = RandomInField()
    Bob->>Bob: commitment_B = Poseidon4(pk_B, bobNftSalt, 1, 42)
    Bob->>NFTVault: deposit([tokenId=42, pk_B, bobNftSalt])
    NFTVault-->>Bob: emit Commitment(...)

    Note over Alice,GnarkServer: Step 3 — Alice runs DvPInitiator proof
    Alice->>GnarkServer: POST /proof/dvpInitiator<br/>delivers: amount=30, tokenId=0 (USDT)<br/>expects:  amount=1, tokenId=42 (ticket)<br/>bobSpend.pk, bobView.encapsKey<br/>aliceMerkleProof
    GnarkServer->>GnarkServer: prove Alice owns 30 USDT note
    GnarkServer->>GnarkServer: COMMIT_B = Poseidon4(pk_B, saltB_B, 30, 0)<br/>(Bob's USDT output — ML-KEM for Bob)
    GnarkServer->>GnarkServer: COMMIT_A = Poseidon4(pk_A, saltA_A, 1, 42)<br/>(Alice's NFT output)
    GnarkServer->>GnarkServer: REVERT_COMMIT_A = Poseidon4(pk_A, revertSalt, 30, 0)<br/>(Alice's fallback if timeout)
    GnarkServer-->>Alice: proof[8], COMMIT_B, COMMIT_A, REVERT_COMMIT_A<br/>cipherText (ML-KEM capsule for Bob), encTxData

    Note over Bob: Step 4 — Bob scans and verifies
    Bob->>Bob: Decapsulate(decapsKey_B, cipherText) → ss_B
    Bob->>Bob: saltB_B = DerivePaymentSalt(ss_B)
    Bob->>Bob: saltA_B = DeriveDvpSaltInit(ss_B)
    Bob->>Bob: encKey_B = DerivePaymentKey(ss_B)
    Bob->>Bob: DecryptPayload(encKey_B, encTxData)<br/>→ tokenId=0, amount=30 (Alice delivers USDT)
    Bob->>Bob: verify Poseidon4(pk_B, saltB_B, 30, 0) == COMMIT_B ✓
    Bob->>Bob: verify Poseidon4(pk_A, saltA_B, 1, 42) == COMMIT_A ✓

    Note over Bob,GnarkServer: Step 5 — Bob runs DvPDestination proof
    Bob->>GnarkServer: POST /proof/dvpDestination<br/>delivers: amount=1, tokenId=42 (ticket)<br/>aliceSpend.pk, saltA_B, COMMIT_A<br/>bobMerkleProof
    GnarkServer->>GnarkServer: prove Bob owns ticket note
    GnarkServer->>GnarkServer: prove COMMIT_A encodes same (amount=1, tokenId=42)
    GnarkServer-->>Bob: proof[8], statement[5]

    Note over Alice,Bob: Swap complete
    Note right of DVP: Both proofs are submitted on-chain<br/>COMMIT_B → ERC20 vault (Bob gets USDT)<br/>COMMIT_A → NFT vault (Alice gets ticket)
```

---

## Cross-Commitment Consistency

The DvP circuit enforces that Alice's `COMMIT_A` and Bob's `COMMIT_A` reference the same
note — Bob cannot substitute a different tokenId or amount.

| Commitment | Owner | Asset | Derived by |
|-----------|-------|-------|-----------|
| `COMMIT_B` | Bob | 30 USDT | Alice (ML-KEM, `DerivePaymentSalt`) |
| `COMMIT_A` | Alice | ticket tokenId=42 | Alice (ML-KEM, `DeriveDvpSaltInit`) |
| `REVERT_COMMIT_A` | Alice | 30 USDT fallback | Alice (timeout recovery) |

## Key Contracts

| Contract | Function | Purpose |
|----------|----------|---------|
| `Erc20CoinVault` | `depositV2` | Alice's USDT deposit |
| `Erc721CoinVault` | `deposit` | Bob's NFT deposit |
| `EnygmaDvp` | `submitPartialSettlement` | Submit each leg; cross-reference commitments |
| `GenericGroth16Verifier` | `verifyProof` | On-chain proof check for both legs |
