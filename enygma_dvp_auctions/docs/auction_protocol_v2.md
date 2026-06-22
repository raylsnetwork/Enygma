# Enygma Auction Protocol — v2

Changes from v1:

- **Revert commitments**: Bob pre-commits a fresh NFT revert note at `initAuction` time; each bidder pre-commits a fresh USDC revert note at `submitBid` time. Both are proven well-formed in-circuit (`saltRevert ≠ saltLocked / saltA`). This enables permissionless timeout recovery without a new ZK proof.
- **`settlementDeadline`**: Bob sets it at `initAuction` time (must be `> biddingDeadline`). If the auctioneer does not settle before this deadline, anyone can call `recoverAuction()`.
- **`recoverAuction()`**: permissionless timeout recovery that inserts Bob's pre-committed revert note into the NFT tree and marks the auction CANCELED.
- **`reclaimBid()`**: now inserts `bid.revertCommit` (fresh salt, unlinkable) instead of the original `commitA`, preserving bidder privacy after cancellation.
- **No bond mechanics**: the optimistic challenge path is deadline-gated only; no ETH bonds are required from either auctioneer or challenger.
- **`revertAuction()` anytime cancel**: Bob can cancel the auction at any time while the state is BIDDING — before or after the bidding deadline. The deadline guard was removed because sealed ML-KEM bids give Bob no information advantage from early cancellation. After CANCELED, all bidders reclaim their USDC via `reclaimBid()` using their pre-committed `revertCommit`.
- `initAuction` / `submitBid` now accept `statement[7]` (was `statement[6]`); the 7th element is `revertCommit`.
- **Single settlement path**: `settle()` (immediate on-chain verification) was removed; only the optimistic path remains (`settleOptimistic` → challenge window → `finalizeSettlement` or `challengeSettlement`).

```mermaid
---
config:
  look: handDrawn
  theme: redux
---

sequenceDiagram
    participant bob    as Bob (NFT Seller)
    participant alice  as Alice (Bidder)
    participant auctnr as Auctioneer
    participant gnark  as gnark Server
    participant chain  as EnygmaAuction
    participant nft    as NftVault
    participant usdc   as UsdcVault
    participant verif  as Groth16 Verifier

    %% ═══════════════════════════════════════════════════════════════
    %% PRE-AUCTION SETUP
    %% ═══════════════════════════════════════════════════════════════

    note over bob,verif: PRE-AUCTION SETUP

    note right of auctnr: (sk_auction, pk_auction) ← ML-KEM.Keygen()<br>pk_auction published (e.g. on-chain or in contract config)

    note over bob: Has ERC-721 note already in NftVault<br>commitIn = Erc721Commitment(tokenId, pk_B, saltIn)<br>Holds: saltIn, Merkle proof (root, pathElements, leafIndex)

    note over alice: Has ERC-20 note already in UsdcVault<br>commitIn = Erc20CommitmentV2(pk_A, saltIn, amount, usdcTokenId)<br>Holds: saltIn, amount, Merkle proof (root, pathElements, leafIndex)

    note over chain: Deployed with:<br>• verifier address (Groth16 on BN254)<br>• NftVault + UsdcVault addresses<br>• VK_LOCK, VK_BID, VK_BATCH, VK_FINAL, VK_REVERT

    %% ═══════════════════════════════════════════════════════════════
    %% PHASE 0a — BOB LOCKS HIS NFT AND OPENS THE AUCTION
    %% ═══════════════════════════════════════════════════════════════

    rect rgba(100,149,237,0.12)
        note over bob,verif: ── PHASE 0a: Bob Locks NFT  →  initAuction() ──────────────────────────────────

        note over bob: Locally computes:<br>saltLocked   ← random field element<br>saltRevert   ← random field element  (≠ saltLocked)<br>commitLocked = Erc721Commitment(tokenId, pk_B, saltLocked)<br>revertCommit = Erc721Commitment(tokenId, pk_B, saltRevert)   [pre-committed revert note]<br>auctionId    = Poseidon(commitLocked)<br>nullifier    = Poseidon(sk_B, leafIndex)

        bob->>gnark: POST /proof/auctionLock<br>PUBLIC:  stAuctionId, stTreeNumber, stMerkleRoot,<br>         stNullifier, stCommitLocked, stNftTokenId,<br>         stRevertCommit                              ← NEW<br>PRIVATE: wtSpendKey(sk_B), wtTokenId, wtSaltIn,<br>         wtPathElements[8], wtPathIndex, wtSaltLocked,<br>         wtSaltRevert                                ← NEW

        note over gnark: Groth16.Prove(AuctionLock circuit)<br>Circuit checks:<br>① nullifier     = Poseidon(sk_B, pathIndex)<br>② commitIn      = Erc721Commitment(tokenId, pk_B, saltIn)<br>③ MerkleRoot(commitIn, path) == stMerkleRoot<br>④ commitLocked  = Erc721Commitment(tokenId, pk_B, saltLocked)<br>⑤ auctionId     = Poseidon(commitLocked)<br>⑥ revertCommit  = Erc721Commitment(tokenId, pk_B, saltRevert)   ← NEW<br>⑦ saltRevert ≠ saltLocked                                        ← NEW

        gnark-->>bob: { proof: π[8], publicSignal: [auctionId, treeNumber, merkleRoot,<br>                                    nullifier, commitLocked, nftTokenId,<br>                                    revertCommit] }   ← 7 signals (was 6)

        bob->>chain: initAuction(π_lock, statement[7], deadline, settlementDeadline, floorPrice)

        chain->>verif: verifyProof(VK_LOCK, π_lock, statement[7])
        verif-->>chain: ✓ valid

        chain->>nft: verifyRoot(treeNumber, merkleRoot)
        nft-->>chain: ✓ root accepted

        chain->>nft: lockCoin(treeNumber, nullifier)
        note over nft: Marks nullifier as LOCKED<br>Prevents Bob from spending the NFT note while auction is live

        note over chain: Validates: settlementDeadline > deadline<br>state[auctionId] = BIDDING<br>Records: commitLocked, revertCommit, nftTokenId, nftNullifier,<br>         nftTreeNumber, deadline, settlementDeadline, floorPrice, bidCount=0<br>Emits: AuctionInitialized(auctionId, commitLocked, revertCommit,<br>                           nftTokenId, deadline, settlementDeadline, floorPrice)
    end

    %% ═══════════════════════════════════════════════════════════════
    %% PHASE 0b — BIDDERS SUBMIT SEALED BIDS (before deadline)
    %% ═══════════════════════════════════════════════════════════════

    rect rgba(60,179,113,0.12)
        note over bob,verif: ── PHASE 0b: Bidders Submit Sealed Bids  →  submitBid()  (repeated, before deadline) ─

        loop Each bidder (100 per batch, up to ~1000 total)
            note over alice: Locally computes:<br>ss, ctxt1   ← ML-KEM.Encapsulate(pk_auction)        [fresh shared secret]<br>saltA       = HKDF(ss, "bid salt A")                [locked bid salt]<br>saltB       = HKDF(ss, "note salt")                 [Bob's payout salt]<br>saltRevert  ← random field element  (≠ saltA)       [NEW: fresh revert salt]<br>k           = HKDF(ss, "aead key")<br>commitA     = Erc20CommitmentV2(pk_A, saltA, amount, usdcTokenId)<br>commitB     = Erc20CommitmentV2(pk_B, saltB, amount, usdcTokenId)<br>revertCommit = Erc20CommitmentV2(pk_A, saltRevert, amount, usdcTokenId)  [NEW]<br>nullifier   = Poseidon(sk_A, leafIndex)<br>ctxt2       ← AEAD.Enc(k, auctionId ∥ pk_B ∥ amount ∥ usdcTokenId)

            alice->>gnark: POST /proof/auctionBid<br>PUBLIC:  stAuctionId, stTreeNumber, stMerkleRoot,<br>         stNullifier, stCommitA, stCommitB,<br>         stRevertCommit                              ← NEW<br>PRIVATE: wtSpendKey(sk_A), wtAmount, wtTokenId, wtSaltIn,<br>         wtPathElements[8], wtPathIndex,<br>         wtSpendPkBob(pk_B), wtSaltA, wtSaltB, wtBidAmount,<br>         wtSaltRevert                                ← NEW

            note over gnark: Groth16.Prove(AuctionBid circuit)<br>Circuit checks:<br>① nullifier     = Poseidon(sk_A, pathIndex)<br>② commitIn      = Erc20CommitmentV2(pk_A, saltIn, amount, tokenId)<br>③ MerkleRoot(commitIn, path) == stMerkleRoot<br>④ commitA       = Erc20CommitmentV2(pk_A, saltA, amount, tokenId)<br>⑤ commitB       = Erc20CommitmentV2(pk_B, saltB, amount, tokenId)<br>⑥ all amounts equal (all-in bid)<br>⑦ revertCommit  = Erc20CommitmentV2(pk_A, saltRevert, amount, tokenId)  ← NEW<br>⑧ saltRevert ≠ saltA                                                      ← NEW

            gnark-->>alice: { proof: π[8], publicSignal: [auctionId, treeNumber, merkleRoot,<br>                                       nullifier, commitA, commitB,<br>                                       revertCommit] }   ← 7 signals (was 6)

            alice->>chain: submitBid(π_bid, statement[7], ctxt1, ctxt2)
            note over chain: Guards:<br>① state[auctionId] == BIDDING<br>② block.timestamp < deadline<br>③ _bids[auctionId][commitA] not already active

            chain->>verif: verifyProof(VK_BID, π_bid, statement[7])
            verif-->>chain: ✓ valid

            chain->>usdc: verifyRoot(treeNumber, merkleRoot)
            usdc-->>chain: ✓ root accepted

            chain->>usdc: nullifyCoin(treeNumber, nullifier)
            note over usdc: Permanently burns Alice's input USDC note

            note over chain: Stores BidData{ commitB, revertCommit, nullifier, treeNumber }   ← revertCommit NEW<br>bidCount++<br>Emits: BidSubmitted(auctionId, commitA, commitB, revertCommit, ctxt1, ctxt2)

            chain-->>auctnr: event BidSubmitted — auctioneer monitors on-chain
        end

        note over chain: block.timestamp >= deadline → bidding window CLOSED
    end

    %% ═══════════════════════════════════════════════════════════════
    %% AUCTIONEER DECRYPTS ALL BIDS (off-chain, after deadline)
    %% ═══════════════════════════════════════════════════════════════

    rect rgba(255,165,0,0.10)
        note over bob,verif: ── OFF-CHAIN: Auctioneer Decrypts Bids ─────────────────────────────────────────

        note over auctnr: For each BidSubmitted event(auctionId, commitA, commitB, revertCommit, ctxt1, ctxt2):<br>  ss     ← ML-KEM.Decapsulate(sk_auction, ctxt1)<br>  saltA  = HKDF(ss, "bid salt A")<br>  saltB  = HKDF(ss, "note salt")<br>  k      = HKDF(ss, "aead key")<br>  msg    ← AEAD.Dec(k, ctxt2)   → recovers auctionId, pk_B, amount, tokenId<br>  Validates: Erc20CommitmentV2(pk_A, saltA, amount, tokenId) == commitA  ✓<br>             Erc20CommitmentV2(pk_B, saltB, amount, tokenId) == commitB  ✓<br>  Stores: (commitA, pk_A, saltA, amount, saltB) per bid<br>  Groups bids into batches of 100 for Phase 1
    end

    %% ═══════════════════════════════════════════════════════════════
    %% PHASE 1 — AUCTIONEER OPENS BATCHES
    %% ═══════════════════════════════════════════════════════════════

    rect rgba(255,200,50,0.12)
        note over bob,verif: ── PHASE 1: AuctionBatch Proofs  →  submitBatch()  (one per 100 bids, up to 10 batches) ─

        loop Each batch (batchId = 1, 2, …, N ≤ 10)
            note over auctnr: Batch window: bids [(b-1)×100 .. b×100)<br>Pads inactive slots with zeros<br>Identifies batchWinner = argmax(amount[i] where active[i])

            auctnr->>gnark: POST /proof/auctionBatch<br>PUBLIC:  stAuctionId<br>         stBidCommitA[100]   (0 for inactive slots)<br>         stBatchWinnerCommit, stBatchWinnerPk, stBatchWinnerAmount<br>PRIVATE: wtActive[100], wtPkBidder[100], wtSaltA[100],<br>         wtAmount[100], wtTokenId[100], wtWinnerIdx

            note over gnark: Groth16.Prove(AuctionBatch circuit, 100-slot batch)<br>Circuit checks (for every active slot i):<br>① stBidCommitA[i] = Erc20CommitmentV2(pk_A[i], saltA[i], amount[i], tokenId)<br>② stBidCommitA[i] = 0 for inactive slots<br>③ batchWinner = slots[wtWinnerIdx]<br>④ batchWinnerAmount ≥ amount[i] for all active i

            gnark-->>auctnr: { proof: π[8], publicSignal: [auctionId,<br>                              commitA[0], …, commitA[99],<br>                              batchWinnerCommit, batchWinnerPk, batchWinnerAmount] }

            auctnr->>chain: submitBatch(batchId, π_batch, statement[104])
            note over chain: Guards:<br>① state[auctionId] == BIDDING<br>② block.timestamp >= deadline<br>③ batch not already submitted<br>④ For every commitA[i] ≠ 0: _bids[auctionId][commitA[i]].active == true

            chain->>verif: verifyProof(VK_BATCH, π_batch, statement[104])
            verif-->>chain: ✓ valid

            note over chain: Stores BatchResult{ batchWinnerCommit, batchWinnerPk, batchWinnerAmount }<br>batchCount++<br>Emits: BatchProcessed(auctionId, batchId, batchWinnerCommit, batchWinnerAmount)
        end
    end

    %% ═══════════════════════════════════════════════════════════════
    %% PHASE 2 — SETTLEMENT
    %% ═══════════════════════════════════════════════════════════════

    rect rgba(220,50,50,0.10)
        note over bob,verif: ── PHASE 2: Settlement ─────────────────────────────────────────────────────────

        note over auctnr: Collects all N batch winners (commit, pk, amount)<br>Overall winner = argmax(batchWinnerAmount[b] where active[b])<br>Recovers winner's saltB from ML-KEM decryption<br>Computes fresh saltAlice for winner's new NFT commitment:<br>  commitB         = Erc20CommitmentV2(pk_B, saltB_winner, winningAmount, usdcTokenId)<br>  winnerNftCommit = Erc721Commitment(nftTokenId, pk_winner, saltAlice)

        auctnr->>gnark: POST /proof/auctionFinal<br>PUBLIC:  stAuctionId<br>         stBatchWinnerCommit[10], stBatchWinnerPk[10], stBatchWinnerAmount[10]<br>         stOverallWinnerCommit, stWinnerPk<br>         stWinnerCommitB, stWinnerNftCommit<br>         stNftTokenId, stWinningAmount, stFloorPrice<br>PRIVATE: wtBatchActive[10], wtWinnerBatchIdx<br>         wtPkBob(pk_B), wtSaltB, wtUsdcTokenId<br>         wtSaltAlice, wtNftTokenId

        note over gnark: Groth16.Prove(AuctionFinal circuit)<br>Circuit checks:<br>① Overall winner = active batch slot with max amount<br>② stWinnerCommitB   = Erc20CommitmentV2(pk_B, saltB, winningAmount, usdcTokenId)<br>③ stWinnerNftCommit = Erc721Commitment(nftTokenId, pk_winner, saltAlice)<br>④ stWinningAmount ≥ stFloorPrice<br>⑤ stOverallWinnerCommit == batchWinners[wtWinnerBatchIdx].commit

        gnark-->>auctnr: { proof: π[8], publicSignal: [auctionId,<br>                              batchWinnerCommit[0..9],<br>                              batchWinnerPk[0..9],<br>                              batchWinnerAmount[0..9],<br>                              overallWinnerCommit, winnerPk,<br>                              winnerCommitB, winnerNftCommit,<br>                              nftTokenId, winningAmount, floorPrice] }

        auctnr->>chain: settleOptimistic(π_final, statement[38], batchIds[10])
        note over chain: _crossCheckBatches: for each active slot i,<br>asserts BatchResults[batchIds[i]] == {commit[i], pk[i], amount[i]}<br>Stores claim: { proof, statement, batchIds, challengeDeadline=now+1day }<br>state[auctionId] = PENDING_SETTLEMENT<br>No bond required<br>Emits: AuctionSettlementProposed(auctionId, auctioneer, challengeDeadline)

        alt If challenged before challengeDeadline

            note over alice: Anyone may challenge (no bond required)
            alice->>chain: challengeSettlement(auctionId)

            chain->>verif: verifyProof(VK_FINAL, storedProof, storedStatement)

            alt Proof is INVALID (auctioneer was dishonest)
                note over chain: Delete claim<br>state[auctionId] = BIDDING  (auction reopened — auctioneer must resubmit)<br>No ETH transfers<br>Emits: AuctionSettlementChallenged(auctionId, challenger, proofValid=false)
            else Proof is VALID (challenge was frivolous)
                note over chain: Apply settlement (see _applySettlement below)<br>No ETH transfers<br>Emits: AuctionSettlementChallenged(auctionId, challenger, proofValid=true)
            end

        else If challengeDeadline passes unchallenged

            note over alice: Anyone may call finalizeSettlement()
            alice->>chain: finalizeSettlement(auctionId)
            note over chain: Apply settlement — no proof verification needed

        end

        note over chain: _applySettlement(auctionId, statement):<br>① NftVault.unlockCoin(nftTreeNumber, nftNullifier)<br>② NftVault.nullifyCoin(nftTreeNumber, nftNullifier)<br>③ NftVault.registerCoins([winnerNftCommit])<br>④ UsdcVault.registerCoins([winnerCommitB])<br>⑤ Mark _bids[auctionId][overallWinnerCommit].claimed = true<br>⑥ state[auctionId] = SETTLED<br>Emits: AuctionSettled(auctionId, overallWinnerCommit, winnerCommitB, winnerNftCommit, winningAmount)

        note over alice: Winner can now spend winnerNftCommit → owns the NFT
        note over bob: Bob can now spend winnerCommitB → receives winning USDC amount
    end

    %% ═══════════════════════════════════════════════════════════════
    %% TIMEOUT PATH — PERMISSIONLESS RECOVERY (NEW in v2)
    %% ═══════════════════════════════════════════════════════════════

    rect rgba(180,80,180,0.12)
        note over bob,verif: ── TIMEOUT PATH: Permissionless Recovery  →  recoverAuction()  ← NEW in v2 ──────
        note over bob,verif: Triggered when: block.timestamp ≥ settlementDeadline AND state == BIDDING<br>(auctioneer failed to settle in time, no ZK proof needed)

        alice->>chain: recoverAuction(auctionId)   [anyone can call]
        note over chain: Guards:<br>① state[auctionId] == BIDDING<br>② block.timestamp ≥ settlementDeadline

        chain->>nft: unlockCoin(nftTreeNumber, nftNullifier)
        chain->>nft: nullifyCoin(nftTreeNumber, nftNullifier)
        chain->>nft: registerCoins([revertCommit_Bob])
        note over nft: Bob's original NFT note is burned<br>Pre-committed revertCommit inserted — no new ZK proof needed<br>On-chain observer cannot link revertCommit back to this auction

        note over chain: state[auctionId] = CANCELED<br>Emits: AuctionRecovered(auctionId, revertCommit)

        note over alice: All bidders may now call reclaimBid() to recover their USDC
    end

    %% ═══════════════════════════════════════════════════════════════
    %% PHASE 3 — LOSING BIDDERS RECLAIM USDC (after SETTLED or CANCELED)
    %% ═══════════════════════════════════════════════════════════════

    rect rgba(100,100,220,0.10)
        note over bob,verif: ── PHASE 3: Losers Reclaim USDC  →  reclaimBid()  (no ZK proof needed) ──────────

        loop Each losing bidder (commitA ≠ overallWinnerCommit)
            alice->>chain: reclaimBid(auctionId, commitA)
            note over chain: Checks (no proof needed):<br>① auction is SETTLED or CANCELED<br>② _bids[auctionId][commitA].active == true<br>③ commitA ≠ auctions[auctionId].overallWinnerCommit<br>④ bid.claimed == false<br>Marks bid.claimed = true

            chain->>usdc: registerCoins([bid.revertCommit])   ← CHANGED (was commitA)
            note over usdc: Inserts bid.revertCommit (fresh salt) into the USDC Merkle tree<br>Alice recovers her full USDC amount under a new, unlinkable commitment<br>On-chain observer cannot link revertCommit back to this auction
            note over chain: Emits: BidReclaimed(auctionId, commitA)
        end
    end

    %% ═══════════════════════════════════════════════════════════════
    %% CANCEL PATH — BOB CANCELS THE AUCTION (ZK proof required)
    %% ═══════════════════════════════════════════════════════════════

    rect rgba(150,100,150,0.10)
        note over bob,verif: ── CANCEL PATH: Bob Cancels Auction  →  revertAuction()  (any time while BIDDING) ─
        note over bob,verif: Bob may cancel before OR after the bidding deadline, at any time while state == BIDDING<br>Bidders who already submitted bids recover their USDC via reclaimBid() after CANCELED

        note over bob: Computes:<br>saltOut        ← fresh random field element<br>revertedCommit  = Erc721Commitment(tokenId, pk_B, saltOut)<br>(new salt → revertedCommit is unlinkable to commitLocked on-chain)

        bob->>gnark: POST /proof/auctionRevert<br>PUBLIC:  stAuctionId, stCommitLocked, stNftTokenId, stRevertedCommit<br>PRIVATE: wtSpendKey(sk_B), wtTokenId, wtSaltLocked, wtSaltOut

        note over gnark: Groth16.Prove(AuctionRevert circuit)<br>Circuit checks:<br>① commitLocked   = Erc721Commitment(tokenId, pk_B, saltLocked)<br>② revertedCommit = Erc721Commitment(tokenId, pk_B, saltOut)

        gnark-->>bob: { proof: π[8], publicSignal: [auctionId, commitLocked, nftTokenId, revertedCommit] }

        bob->>chain: revertAuction(π_revert, statement[4])
        note over chain: Checks:<br>① state[auctionId] == BIDDING  (no timing restriction)<br>② statement[1] == auctions[auctionId].commitLocked<br>③ statement[2] == auctions[auctionId].nftTokenId

        chain->>verif: verifyProof(VK_REVERT, π_revert, statement[4])
        verif-->>chain: ✓ valid

        chain->>nft: unlockCoin(nftTreeNumber, nftNullifier)
        chain->>nft: nullifyCoin(nftTreeNumber, nftNullifier)
        chain->>nft: registerCoins([revertedCommit])
        note over nft: Bob's original NFT note is burned<br>Fresh revertedCommit inserted into the NFT Merkle tree

        note over chain: state[auctionId] = CANCELED<br>Emits: AuctionReverted(auctionId, revertedCommit)

        note over alice: All bidders (including those who bid before the cancellation)<br>may now call reclaimBid() to recover their USDC via their pre-committed revertCommit
    end
```
