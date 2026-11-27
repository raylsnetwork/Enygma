# 04 — DvP with Deadline (Three Scenarios)

**Test:** `TestV2DvP_WithDeadline`
**File:** `test/04_v2_dvp_deadline_test.go`

Alice swaps 50 USDT (ERC20) for Bob's ERC721 ticket. Alice proves the deadline and input vault group before submitting the first leg.
Three sub-scenarios test the full lifecycle of the deadline-protected DvP.

The setup (deposits + proof generation) is identical across all three scenarios.
They diverge only after Alice submits her first leg on-chain.

---

## Shared Setup (all three scenarios)

```mermaid
sequenceDiagram
    participant Alice
    participant Bob
    participant ERC20Vault as Erc20CoinVault
    participant NFTVault as Erc721CoinVault
    participant GnarkServer as Gnark Server :8081

    Alice->>ERC20Vault: depositV2([50, pk_A, salt_A, tokenId], capsule_A, encData_A)
    ERC20Vault-->>Alice: emit Commitment(...)

    Bob->>NFTVault: deposit([tokenId, pk_B, salt_B])
    NFTVault-->>Bob: emit Commitment(...)

    Alice->>GnarkServer: POST /proof/dvpInitiator<br/>delivers 50 USDT, expects ticket
    GnarkServer-->>Alice: proof_A, COMMIT_B, COMMIT_A, REVERT_COMMIT_A, cipherText, encTxData

    Bob->>Bob: Decapsulate → verify COMMIT_B and COMMIT_A ✓

    Bob->>GnarkServer: POST /proof/dvpDestination<br/>delivers ticket, references COMMIT_A
    GnarkServer-->>Bob: proof_B, statement_B
```

---

## Scenario A — FullSwap (happy path)

```mermaid
sequenceDiagram
    participant Alice
    participant Bob
    participant DVP as EnygmaDvp
    participant ERC20Vault as Erc20CoinVault
    participant NFTVault as Erc721CoinVault

    Alice->>DVP: submitPartialSettlement(receipt_A, vaultId=0, groupId=0, provenDeadline)
    DVP->>DVP: verify proof_A (DvPInitiator VK)
    DVP->>ERC20Vault: addPendingProofReceipt(receipt_A), locking Alice's input
    DVP-->>Alice: emit SwapInitiated(COMMIT_B, COMMIT_A, COMMIT_B, deadline)
    Note over DVP: _pendingTransactions[COMMIT_B] = {targetId: COMMIT_A, groupId: 0, deadline: provenDeadline}

    Bob->>DVP: submitPartialSettlement(receipt_B)  ← before deadline
    DVP->>DVP: verify proof_B (DvPDestination VK)
    DVP->>DVP: check _pendingTransactions[COMMIT_B].targetId == COMMIT_A ✓
    DVP->>ERC20Vault: nullify input and insertCommitmentFromDvpReceipt(receipt_A)
    DVP->>NFTVault: nullify input and insertCommitmentFromDvpReceipt(receipt_B)
    ERC20Vault-->>Bob: emit Commitment(COMMIT_B)   ← Bob gets USDT
    NFTVault-->>Alice: emit Commitment(COMMIT_A)   ← Alice gets ticket
    Note over DVP: swap settled atomically
```

---

## Scenario B — DeadlineExpired (Bob never responds)

```mermaid
sequenceDiagram
    participant Alice
    participant Bob
    participant DVP as EnygmaDvp
    participant ERC20Vault as Erc20CoinVault
    participant Hardhat as Hardhat Node

    Alice->>DVP: submitPartialSettlement(receipt_A, vaultId=0, groupId=0, provenDeadline)
    DVP->>ERC20Vault: addPendingProofReceipt(receipt_A), locking Alice's input
    DVP-->>Alice: emit SwapInitiated(COMMIT_B, COMMIT_A, COMMIT_B, deadline)

    Note over Bob: Bob does nothing

    Hardhat->>Hardhat: evm_increaseTime(+65s) + evm_mine()

    Bob--xDVP: submitPartialSettlement(receipt_B) — REVERTS SwapDeadlineExpired()

    Alice->>DVP: claimSwapTimeout(COMMIT_B)
    DVP->>DVP: check block.timestamp > deadline ✓
    DVP->>ERC20Vault: nullify Alice's input and registerCoins([REVERT_COMMIT_A])
    DVP->>DVP: delete _pendingTransactions[COMMIT_B]
    DVP-->>Alice: emit SwapTimedOut(pendingReceiptId=COMMIT_B)

    Alice--xDVP: claimSwapTimeout(COMMIT_B) again — REVERTS SwapNotFound()

    Note over Alice: REVERT_COMMIT_A is spendable to recover funds
```

---

## Scenario C — InvalidProof (Bob submits bad proof)

```mermaid
sequenceDiagram
    participant Alice
    participant Bob
    participant DVP as EnygmaDvp
    participant ERC20Vault as Erc20CoinVault
    participant Hardhat as Hardhat Node

    Alice->>DVP: submitPartialSettlement(receipt_A, vaultId=0, groupId=0, provenDeadline)
    DVP->>ERC20Vault: addPendingProofReceipt(receipt_A), locking Alice's input
    DVP-->>Alice: emit SwapInitiated(COMMIT_B, COMMIT_A, COMMIT_B, deadline)

    Bob->>DVP: submitPartialSettlement(receipt_B_zeroed) ← all-zero proof
    DVP--xBob: REVERTS "Pairing: Pairing Verification Failed"

    Hardhat->>Hardhat: evm_increaseTime(+65s) + evm_mine()

    Alice->>DVP: claimSwapTimeout(COMMIT_B)
    DVP->>ERC20Vault: nullify Alice's input and registerCoins([REVERT_COMMIT_A])
    DVP->>DVP: delete _pendingTransactions[COMMIT_B]
    DVP-->>Alice: emit SwapTimedOut(pendingReceiptId=COMMIT_B)

    Note over Alice: REVERT_COMMIT_A is spendable to recover funds
```

---

## Statement Layout (DvP Initiator, 10 elements)

| Index | Field | Value |
|-------|-------|-------|
| 0 | `stMessage` | `COMMIT_A` (cross-reference target) |
| 1 | `treeNumber` | ERC20 tree number |
| 2 | `merkleRoot` | ERC20 Merkle root |
| 3 | `nullifier` | Alice's USDT nullifier |
| 4 | `COMMIT_B` | Bob's USDT output (receiptUniqueId) |
| 5 | `COMMIT_A` | Alice's NFT output |
| 6 | `REVERT_COMMIT_A` | Alice's USDT fallback if timeout |
| 7 | `counterVault` | Address of the vault Alice must be paid from (the NFT vault) |
| 8 | `deadline` | Unix timestamp in seconds proved by Alice |
| 9 | `groupId` | Alice's input vault group (0 in this flow) |

## Statement Layout (DvP Destination, 6 elements)

| Index | Field | Value |
|-------|-------|-------|
| 0 | `stMessage` | `COMMIT_B` (must match pending swap) |
| 1 | `treeNumber` | NFT tree number |
| 2 | `merkleRoot` | NFT Merkle root |
| 3 | `nullifier` | Bob's ticket nullifier |
| 4 | `COMMIT_A` | Alice's NFT output (must match stored targetId) |
| 5 | `counterVault` | Address of the vault Bob must be paid from (the ERC20 vault) |

## Key Contracts

| Contract | Function | Purpose |
|----------|----------|---------|
| `EnygmaDvp` | `submitPartialSettlement(receipt, vaultId, groupId, deadline)` | Submit first or second leg |
| `EnygmaDvp` | `claimSwapTimeout(commitB)` | Revert after deadline passes |
| `Erc20CoinVault` | `addPendingProofReceipt` | Record the first leg and lock its input |
| `Erc20CoinVault` | `insertCommitmentFromDvpReceipt` | Insert COMMIT_B into the ERC20 tree on settlement |
| `Erc721CoinVault` | `insertCommitmentFromDvpReceipt` | Insert COMMIT_A into the NFT tree on settlement |
| `Erc20CoinVault` | `registerCoins` | Insert REVERT_COMMIT_A on timeout |
