# Enygma Auction — Technical Reference

## 1. Overview

Enygma Auction is a ZK sealed-bid NFT auction. Bob locks an ERC-721 NFT; bidders lock USDC notes against it; the auctioneer opens bids in ZK and proves the winner; the winner receives the NFT and Bob receives the winning USDC bid.

**Key properties:**
- All amounts are hidden from the chain until settlement
- Bids are sealed with ML-KEM post-quantum encryption; only the auctioneer can decrypt
- Every step except timeout recovery and bid reclaim requires a Groth16 proof
- Optimistic settlement: proof stored, not verified on-chain unless challenged
- Bidders can withdraw before the deadline; seller can cancel at any time

---

## 2. Cryptographic Primitives

| Primitive | Role |
|---|---|
| Poseidon (BN254 x^5) | Commitments, nullifiers, auction ID, public key derivation |
| Groth16 on BN254 | ZK proof system for all 6 circuits |
| ML-KEM-768 | Post-quantum KEM; seals bid amounts from everyone except the auctioneer |
| HKDF-SHA256 | Derives `saltA`, `saltB`, `k` from ML-KEM shared secret |
| AES-GCM (AEAD) | Encrypts bid payload `(pk_B, tokenId, amount)` |

### Commitment formulas

**ERC-20 (V2):** `C = Poseidon4(pk_spend, salt, amount, tokenId)`

**ERC-721:** `C = Poseidon4(pk_spend, salt, 1, tokenId)`

**Nullifier:** `nf = Poseidon2(sk_spend, leafIndex)` — permanently invalidates a note

**Public key:** `pk = Poseidon1(sk)`

**Auction ID:** `auctionId = Poseidon1(commitLocked)` — binds the auction to the locked NFT

---

## 3. Circuit Architecture

All circuits use Groth16 on BN254 with Merkle tree depth 8.

### AuctionLock (Phase 0a) — 7 public signals
Bob proves he owns an NFT note, nullifies it, and creates a fresh locked commitment plus a pre-committed revert note.

| Signal | Description |
|---|---|
| `StAuctionId` | `Poseidon(commitLocked)` |
| `StTreeNumber` | NFT sub-tree index |
| `StMerkleRoot` | Current NFT Merkle root |
| `StNullifier` | `Poseidon(sk_B, leafIndex)` |
| `StCommitLocked` | `Erc721(pk_B, saltLocked, 1, tokenId)` |
| `StNftTokenId` | NFT token ID |
| `StRevertCommit` | `Erc721(pk_B, saltRevert, 1, tokenId)` — `saltRevert ≠ saltLocked` enforced in-circuit |

### AuctionBid (Phase 0b) — 7 public signals
Alice proves she owns a USDC note, nullifies it (all-in), and creates `commitA` (locked bid), `commitB` (Bob's payout destination), and `revertCommit` (Alice's recovery note).

| Signal | Description |
|---|---|
| `StAuctionId` | Target auction |
| `StTreeNumber` | Alice's USDC sub-tree |
| `StMerkleRoot` | Current USDC Merkle root |
| `StNullifier` | `Poseidon(sk_A, leafIndex)` |
| `StCommitA` | `Erc20V2(pk_A, saltA, amount, tokenId)` |
| `StCommitB` | `Erc20V2(pk_B, saltB, amount, tokenId)` — saltB from ML-KEM |
| `StRevertCommit` | `Erc20V2(pk_A, saltRevert, amount, tokenId)` — `saltRevert ≠ saltA` |

### AuctionWithdraw (Phase 0c) — 2 public signals
Alice proves she knows the preimage of `commitA` (i.e., knows `saltA`) to authenticate a bid withdrawal before the deadline.

| Signal | Description |
|---|---|
| `StAuctionId` | Auction being withdrawn from |
| `StCommitA` | The bid commitment to withdraw |

Constraints: `pk_A = Poseidon(sk_A)` and `Erc20V2(pk_A, saltA, amount, tokenId) = commitA`.

### AuctionBatch (Phase 1) — 104 public signals
The auctioneer proves it honestly decrypted up to 100 bids (by knowing each `commitA` preimage) and correctly identified the highest bidder.

| Signals | Description |
|---|---|
| `StAuctionId` | Auction ID |
| `StBidCommitA[0..99]` | On-chain commitA per slot (0 = inactive) |
| `StBatchWinnerCommit` | commitA of the batch winner |
| `StBatchWinnerPk` | Spend public key of the batch winner |
| `StBatchWinnerAmount` | Winning bid amount |

Constraints: preimage of each active `commitA` is known; winner amount ≥ all active slot amounts (dominance); winner slot is active.

### AuctionFinal (Phase 2) — 38 public signals
The auctioneer proves the overall winner across all batches and constructs the settlement output notes.

| Signals | Description |
|---|---|
| `StAuctionId` | — |
| `StBatchWinnerCommit[0..9]` | Per-batch winner commitA |
| `StBatchWinnerPk[0..9]` | Per-batch winner spend pk |
| `StBatchWinnerAmount[0..9]` | Per-batch winner amount |
| `StOverallWinnerCommit` | Overall winning commitA |
| `StWinnerPk` | Winner's spend public key |
| `StWinnerCommitB` | Bob's USDC payout commitment |
| `StWinnerNftCommit` | Winner's new NFT commitment |
| `StNftTokenId` | NFT token ID (must match on-chain) |
| `StWinningAmount` | Winning amount (enforced > 0) |
| `StFloorPrice` | Seller reserve price (enforced ≤ winningAmount) |

### AuctionRevert (Phase 3a) — 4 public signals
Bob proves he owns the locked NFT note and mints it back under a fresh salt — no Merkle proof needed since `commitLocked` is stored on-chain.

| Signal | Description |
|---|---|
| `StAuctionId` | — |
| `StCommitLocked` | Must match `auctions[id].commitLocked` |
| `StNftTokenId` | Must match `auctions[id].nftTokenId` |
| `StRevertedCommit` | Fresh output note; `saltOut ≠ saltLocked` enforced |

---

## 4. Protocol Flow

### Phase 0a — Bob locks the NFT (`initAuction`)
1. Bob generates `saltLocked`, `saltRevert` (random, distinct)
2. Computes `commitLocked`, `revertCommit`, `auctionId`, `nf_B`
3. Calls gnark server `/proof/auctionLock` to generate `π_lock`
4. Submits `initAuction(π_lock, statement[7], deadline, settlementDeadline, floorPrice)`
5. Contract verifies proof, calls `nftVault.lockCoin(nf_B)`, records `commitLocked` + `revertCommit`

### Phase 0b — Alice submits a sealed bid (`submitBid`)
1. Alice runs `ML-KEM.Encaps(pk_auction)` → `(ctxt1, ss)`
2. Derives `saltA = HKDF(ss, "bid salt A")`, `saltB = HKDF(ss, "note salt")`, `k = HKDF(ss, "bid key")`
3. Chooses `saltRevert` (random, ≠ saltA)
4. Encrypts bid: `ctxt2 = AES-GCM(k, pk_B || tokenId || amount, aad=ctxt1)`
5. Calls gnark server `/proof/auctionBid` to generate `π_bid`
6. Submits `submitBid(π_bid, statement[7], ctxt1, ctxt2)`
7. Contract verifies proof, calls `usdcVault.lockCoin(nf_A)` (note locked, not spent), stores `BidData`

### Phase 0c — Alice withdraws a bid (`withdrawBid`)
Only possible while state == BIDDING and before deadline.
1. Alice calls gnark server `/proof/auctionWithdraw` with `(sk_A, saltA, amount, tokenId)`
2. Submits `withdrawBid(π_withdraw, statement[2])`
3. Contract verifies proof, calls `unlockCoin(nf_A) + nullifyCoin(nf_A)`, inserts `bid.revertCommit`
4. Marks `bid.active = false` — auctioneer cannot include this commitA in any batch

### Phase 1 — Auctioneer opens bids in batches (`submitBatch`)
After `deadline`:
1. Auctioneer reads `BidSubmitted` events, decapsulates each `ctxt1` with `sk_auction`
2. For each batch of ≤ 100 bids: decrypts amounts, identifies highest bid, calls gnark `/proof/auctionBatch`
3. Submits `submitBatch(batchId, π_batch, statement[104])`
4. Contract cross-checks each non-zero `commitA` against stored bids (phantom bid prevention), verifies proof, stores `BatchResult`

### Phase 2 — Optimistic settlement (`settleOptimistic`)
1. Auctioneer identifies overall winner across all batches
2. Derives `commitB = Erc20V2(pk_B, saltB_winner, amount_winner, usdcTokenId)`
3. Mints winner's NFT commitment: `winnerNftCommit = Erc721(winnerPk, saltAlice, 1, nftTokenId)`
4. Calls gnark `/proof/auctionFinal`, submits `settleOptimistic(π_final, statement[38], batchIds[10])`
5. Contract cross-checks batch results (free — already on-chain), stores proof without verifying, state → PENDING_SETTLEMENT
6. After `challengeWindow` (default 1 day): anyone calls `finalizeSettlement()` — state → SETTLED without verification

**Challenge path:** anyone can call `challengeSettlement()` within the window to force on-chain Groth16 verification. Invalid proof → state → BIDDING (auctioneer resubmits). Valid proof → SETTLED immediately.

### Phase 3a — Bob cancels (`revertAuction`)
Bob can cancel at any time while state == BIDDING (before or after deadline).
1. Calls gnark `/proof/auctionRevert`, submits `revertAuction(π_revert, statement[4])`
2. Contract unlocks + nullifies the NFT note, inserts `revertedCommit` (fresh salt), state → CANCELED

### Phase 3b — Timeout recovery (`recoverAuction`)
If the auctioneer hasn't settled by `settlementDeadline`:
1. Anyone calls `recoverAuction(auctionId)` — no ZK proof needed
2. Contract unlocks + nullifies NFT note, inserts stored `revertCommit` (proven well-formed at init time), state → CANCELED

### Phase 4 — Losers reclaim USDC (`reclaimBid`)
After SETTLED or CANCELED:
1. Any loser (or anyone on their behalf) calls `reclaimBid(auctionId, commitA)` — no ZK proof
2. Contract checks `commitA ≠ overallWinnerCommit`, unlocks + nullifies the locked USDC note, inserts `bid.revertCommit`

---

## 5. Smart Contract

### `EnygmaAuction.sol`

**State machine:** `INACTIVE → BIDDING → PENDING_SETTLEMENT → SETTLED` or `BIDDING → CANCELED`

**Key storage:**
```solidity
struct AuctionCore {
    AuctionState state;
    uint256 nftTokenId;
    uint256 commitLocked;
    uint256 revertCommit;        // NFT recovery note, pre-committed at initAuction
    uint256 nftNullifier;
    uint256 nftTreeNumber;
    uint256 overallWinnerCommit; // zero until settled
    uint256 batchCount;
    uint256 deadline;            // bidding closes
    uint256 settlementDeadline;  // timeout recovery becomes available
    uint256 floorPrice;
    uint256 bidCount;
}

struct BidData {
    bool    active;       // false after withdrawal or reclaim
    bool    claimed;      // revertCommit has been inserted
    uint256 commitB;      // seller payout note if this bid wins
    uint256 revertCommit; // bidder recovery note, pre-committed at submitBid
    uint256 nullifier;    // bidder's input USDC nullifier
    uint256 treeNumber;   // bidder's USDC sub-tree
}
```

**VK IDs (registered in `Verifier` contract):**
| ID | Circuit |
|---|---|
| 0 | AuctionLock |
| 1 | AuctionBid |
| 2 | AuctionBatch |
| 3 | AuctionFinal |
| 4 | AuctionRevert |
| 5 | AuctionWithdraw |

**Access control:** `OWNER_ROLE`, `AUCTIONEER_ROLE` (for `submitBatch`, `settleOptimistic`)

**Errors:** `AuctionAlreadyExists`, `AuctionStateMismatch`, `BidAlreadyExists`, `BidNotFound`, `BidAlreadyClaimed`, `WinnerCannotReclaim`, `UnknownBid`, `BatchAlreadySubmitted`, `BatchNotFound`, `BatchResultMismatch`, `InvalidMerkleRoot`, `InvalidNftTokenId`, `CommitLockedMismatch`, `InvalidDeadline`, `InvalidSettlementDeadline`, `BiddingClosed`, `BidsStillOpen`, `DeadlineNotReached`, `SettlementDeadlineNotReached`, `FloorPriceMismatch`, `NoPendingSettlement`, `ChallengeWindowClosed`, `ChallengeWindowOpen`, `WithdrawalDeadlinePassed`

---

## 6. gnark Server

**Start:** `cd gnark_circuits && go run main.go` (starts on `:8081`)

**Key generation:** `cd gnark_circuits && go run generation.go` (writes to `./scripts/keys/`)

**Endpoints:**

| Method | Path | Phase | Request |
|---|---|---|---|
| POST | `/proof/auctionLock` | 0a | `AuctionLockRequest` |
| POST | `/proof/auctionBid` | 0b | `AuctionBidRequest` |
| POST | `/proof/auctionWithdraw` | 0c | `AuctionWithdrawRequest` |
| POST | `/proof/auctionBatch` | 1 | `AuctionBatchRequest` |
| POST | `/proof/auctionFinal` | 2 | `AuctionFinalRequest` |
| POST | `/proof/auctionRevert` | 3a | `AuctionRevertRequest` |

All endpoints return `{ proof: [8]bigint, publicSignal: []bigint }`.

**Config:** JSON file with paths to all 6 `.pk` / `.vk` files. Default: `./scripts/keys/*.pk/vk`.

---

## 7. Go Client Library (`src/core`)

**Client types:**
- `AuctionClient` — wraps `GnarkClient` (HTTP to gnark server) + `SpendKeyPair`
- `GnarkClient` — thin HTTP poster to gnark server

**Proof functions:**

| Function | Phase | Returns |
|---|---|---|
| `AuctionLockProof(AuctionLockParams)` | 0a | `AuctionProofResult` |
| `AuctionBidProof(AuctionBidParams)` | 0b | `AuctionProofResult` |
| `AuctionWithdrawProof(AuctionWithdrawParams)` | 0c | `AuctionProofResult` |
| `AuctionBatchProof(AuctionBatchParams)` | 1 | `AuctionProofResult` |
| `AuctionFinalProof(AuctionFinalParams)` | 2 | `AuctionProofResult` |
| `AuctionRevertProof(AuctionRevertParams)` | 3a | `AuctionProofResult` |

`AuctionProofResult.PublicSignal` is already in on-chain calldata order — pass directly as the `statement` argument to the corresponding contract function.

**Utility functions:**
- `Erc721Commitment(tokenId, pk, salt)` — Poseidon4
- `Erc20CommitmentV2(pk, salt, amount, tokenId)` — Poseidon4
- `GetNullifier(sk, leafIndex)` — Poseidon2
- `GetAuctionId(commitLocked)` — Poseidon1

---

## 8. Privacy Model

| Who sees what | On-chain | Off-chain (bidder) | Off-chain (auctioneer) |
|---|---|---|---|
| Bid amounts | Never | Own bids only | All (after decryption) |
| Bidder identity | `commitA` (unlinkable to address) | Self | Self |
| NFT token ID | Yes (Phase 0a) | Yes | Yes |
| Floor price | Yes (Phase 0a) | Yes | Yes |
| Winning amount | Yes (Phase 2) | Yes | Yes |
| Loser amounts | Never on-chain | Self only | Yes (but ephemeral) |

**Unlinkability:** `revertCommit` uses a fresh salt proven ≠ `saltA/saltLocked` in-circuit, so the returned note after withdrawal, reclaim, or cancel cannot be linked to the original bid or lock.

---

## 9. Security Properties

| Property | Mechanism |
|---|---|
| No double-spend | Nullifier checked on-chain; `lockCoin` prevents concurrent use |
| No phantom bids | `submitBatch` cross-checks each `commitA` against `_bids[auctionId]` |
| Honest batch winner | `commitA` preimage known in-circuit; dominance enforced in-circuit |
| Floor price | Enforced inside `AuctionFinalCircuit`; sub-floor proof cannot exist |
| No false settlement | Optimistic path is challengeable; invalid proof voids claim |
| No forced withdrawal | `withdrawBid` requires ZK proof of `saltA` preimage |
| Timeout safety | `revertCommit` pre-committed at lock/bid time; no new proof needed for recovery |
| Withdrawal deadline | Withdrawal gated to `< deadline`; prevents information-advantage attacks |

---

## 10. Deployment

### Prerequisites
- Hardhat node running (`npx hardhat node`)
- Poseidon artifacts regenerated from circomlibjs (run `node scripts/regen_poseidon.js`)
- gnark keys generated (`cd gnark_circuits && go run generation.go`)
- VKs exported to circom format (`go run ./cmd/export_vk_init_auction/ ../build`)

### Steps
```bash
# 1. Deploy contracts
cd scripts && CC=/usr/bin/clang go build -o /tmp/deploy deploy.go enygma.go
cd .. && /tmp/deploy
# → build/receipts.json

# 2. Initialize (register VKs, grant vault roles)
cd scripts && CC=/usr/bin/clang go build -o /tmp/init init.go enygma.go
cd .. && /tmp/init
```

### Key files
| File | Purpose |
|---|---|
| `enygma_auction.config.json` | Network + circuit VK ID mapping |
| `build/receipts.json` | Deployed contract addresses |
| `build/AuctionLock.json` .. `AuctionWithdraw.json` | Circom-format VKs for `init.go` |
| `gnark_circuits/scripts/keys/*.pk/vk` | Groth16 proving/verifying keys |
