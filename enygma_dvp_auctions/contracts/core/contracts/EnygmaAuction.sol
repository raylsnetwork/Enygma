// SPDX-License-Identifier: BUSL-1.1
pragma solidity ^0.8.20;

import {AccessControl}   from "@openzeppelin/contracts/access/AccessControl.sol";
import {ReentrancyGuard} from "@openzeppelin/contracts/security/ReentrancyGuard.sol";

import {IEnygmaAuction} from "../interfaces/IEnygmaAuction.sol";

// ---------------------------------------------------------------------------
// Minimal external interfaces used by this contract
// ---------------------------------------------------------------------------

interface IVerifier {
    function verifyProof(
        uint256        vkId,
        SnarkProof     calldata proof,
        uint256[]      calldata inputs
    ) external view returns (bool);
}

struct G1Point { uint256 x; uint256 y; }
struct G2Point { uint256[2] x; uint256[2] y; }
struct SnarkProof { G1Point a; G2Point b; G1Point c; }

interface ICoinVault {
    function verifyRoot(uint256 treeNumber, uint256 root) external view returns (bool);
    function lockCoin(uint256 treeNumber, uint256 nullifier) external returns (bool);
    function unlockCoin(uint256 treeNumber, uint256 nullifier) external returns (bool);
    function nullifyCoin(uint256 treeNumber, uint256 nullifier) external returns (bool);
    function registerCoins(uint256[] calldata commitments) external returns (bool);
}

// ---------------------------------------------------------------------------
// EnygmaAuction
// ---------------------------------------------------------------------------

/// @title  EnygmaAuction
/// @notice Two-phase ZK sealed-bid auction for ERC-721 NFTs paid in USDC notes,
///         with a deadline, a seller reserve (floor) price, an optimistic
///         settlement fast-path, and a privacy-preserving revert/cancel path.
///
/// Lifecycle
/// ---------
///   1. Bob calls initAuction()      — locks NFT, sets deadline + settlementDeadline + floorPrice, opens bidding
///   2. Bidders call submitBid()     — locks USDC, publishes ML-KEM capsule (only before deadline)
///   3. Auctioneer calls submitBatch() (one per 100-bid batch) — Phase 1 (only after deadline)
///   4. Auctioneer calls settleOptimistic() — posts Phase-2 claim; proof stored but not verified yet
///       └─ challengeSettlement()   — anyone forces verification during the challenge window
///       └─ finalizeSettlement()    — anyone finalizes an unchallenged claim after the window
///   5a. Bob calls revertAuction()  — cancels at any time while BIDDING; reclaims NFT with fresh commitment
///   5b. Anyone calls recoverAuction() — timeout path once settlementDeadline passes; uses pre-committed revertCommit
///   6. Bidders call reclaimBid()   — releases pre-committed revertCommit into USDC tree (losers after SETTLED,
///                                    everyone after CANCELED)
///
/// Security model
/// --------------
/// - ZK proofs enforce correctness of every step; the floor price is enforced
///   *inside* the AuctionFinal circuit, so a sub-floor settlement proof cannot exist.
/// - submitBatch and settleOptimistic are restricted to AUCTIONEER_ROLE.
/// - Optimistic settlement is "trust, but verifiable on demand": the proof is stored
///   but not verified at submission time to save gas in the honest case. Anyone can
///   force real verification by challenging within the window.
/// - revertAuction() mints the NFT back under a *fresh* salt rather than reusing the
///   already-public locked commitment, so the reverted note cannot be linked back to
///   this specific auction by anyone watching the chain.
/// - reclaimBid requires no ZK proof; the contract simply checks
///   commitA ≠ overallWinnerCommit and that the auction is SETTLED or CANCELED.
contract EnygmaAuction is IEnygmaAuction, AccessControl, ReentrancyGuard {

    // -----------------------------------------------------------------------
    // Constants
    // -----------------------------------------------------------------------

    bytes32 public constant OWNER_ROLE      = keccak256("OWNER_ROLE");
    bytes32 public constant AUCTIONEER_ROLE = keccak256("AUCTIONEER_ROLE");

    // -----------------------------------------------------------------------
    // Storage
    // -----------------------------------------------------------------------

    address private _verifier;
    address private _erc20Vault;   // USDC vault
    address private _erc721Vault;  // NFT vault

    // VK IDs registered in the external IVerifier contract.
    uint256 public VK_LOCK;     // AuctionLock     circuit
    uint256 public VK_BID;      // AuctionBid      circuit
    uint256 public VK_BATCH;    // AuctionBatch    circuit (Phase 1)
    uint256 public VK_FINAL;    // AuctionFinal    circuit (Phase 2)
    uint256 public VK_REVERT;   // AuctionRevert   circuit
    uint256 public VK_WITHDRAW; // AuctionWithdraw circuit (Phase 0c)

    // Optimistic settlement parameters (owner-configurable).
    uint256 public challengeWindow = 1 days;

    // Per-auction core state.
    struct AuctionCore {
        AuctionState state;
        uint256 nftTokenId;
        uint256 commitLocked;
        uint256 revertCommit;        // Erc721Commitment(tokenId, pk_B, saltRevert) — pre-committed recovery
        uint256 nftNullifier;
        uint256 nftTreeNumber;
        uint256 overallWinnerCommit; // zero until settled
        uint256 batchCount;
        uint256 deadline;            // bidding closes; settlement/revert may proceed after
        uint256 settlementDeadline;  // after this, anyone can call recoverAuction()
        uint256 floorPrice;          // seller's reserve price
        uint256 bidCount;            // total bids ever submitted
    }

    mapping(uint256 => AuctionCore)                     private _auctions;
    mapping(uint256 => mapping(uint256 => BidData))     private _bids;     // auctionId → commitA → BidData
    mapping(uint256 => mapping(uint256 => BatchResult)) private _batches;  // auctionId → batchId → BatchResult
    mapping(uint256 => OptimisticClaim)                 private _claims;   // auctionId → pending claim

    // -----------------------------------------------------------------------
    // Constructor
    // -----------------------------------------------------------------------

    /// @param verifier_    Address of the IVerifier contract holding all VKs.
    /// @param erc20Vault_  Address of the USDC coin vault (ICoinVault).
    /// @param erc721Vault_ Address of the NFT coin vault (ICoinVault).
    /// @param vkLock_       VK ID for AuctionLock     proof.
    /// @param vkBid_        VK ID for AuctionBid      proof.
    /// @param vkBatch_      VK ID for AuctionBatch    proof.
    /// @param vkFinal_      VK ID for AuctionFinal    proof.
    /// @param vkRevert_     VK ID for AuctionRevert   proof.
    /// @param vkWithdraw_   VK ID for AuctionWithdraw proof.
    constructor(
        address verifier_,
        address erc20Vault_,
        address erc721Vault_,
        uint256 vkLock_,
        uint256 vkBid_,
        uint256 vkBatch_,
        uint256 vkFinal_,
        uint256 vkRevert_,
        uint256 vkWithdraw_
    ) {
        _verifier    = verifier_;
        _erc20Vault  = erc20Vault_;
        _erc721Vault = erc721Vault_;
        VK_LOCK      = vkLock_;
        VK_BID       = vkBid_;
        VK_BATCH     = vkBatch_;
        VK_FINAL     = vkFinal_;
        VK_REVERT    = vkRevert_;
        VK_WITHDRAW  = vkWithdraw_;

        _grantRole(OWNER_ROLE,      msg.sender);
        _grantRole(AUCTIONEER_ROLE, msg.sender);
        _setRoleAdmin(AUCTIONEER_ROLE, OWNER_ROLE);
    }

    // -----------------------------------------------------------------------
    // Admin
    // -----------------------------------------------------------------------

    function grantAuctioneer(address account) external onlyRole(OWNER_ROLE) {
        grantRole(AUCTIONEER_ROLE, account);
    }

    function revokeAuctioneer(address account) external onlyRole(OWNER_ROLE) {
        revokeRole(AUCTIONEER_ROLE, account);
    }

    function setChallengeWindow(uint256 window_) external onlyRole(OWNER_ROLE) {
        require(window_ >= 1 hours, "EnygmaAuction: window too short");
        challengeWindow = window_;
    }

    // -----------------------------------------------------------------------
    // Phase 0a — Bob locks the NFT and opens bidding
    // -----------------------------------------------------------------------
    //
    // AuctionLock public statement (7 elements):
    //   [0] StAuctionId    = Poseidon(commitLocked)
    //   [1] StTreeNumber   = Bob's NFT sub-tree index
    //   [2] StMerkleRoot   = current ERC-721 Merkle root
    //   [3] StNullifier    = Poseidon(sk_B, leafIndex)
    //   [4] StCommitLocked = Erc721Commitment(tokenId, pk_B, saltLocked)
    //   [5] StNftTokenId
    //   [6] StRevertCommit = Erc721Commitment(tokenId, pk_B, saltRevert)

    function initAuction(
        uint256[8] calldata proof,
        uint256[7] calldata statement,
        uint256    deadline,
        uint256    settlementDeadline,
        uint256    floorPrice
    ) external nonReentrant returns (bool) {
        uint256 auctionId    = statement[0];
        uint256 treeNumber   = statement[1];
        uint256 merkleRoot   = statement[2];
        uint256 nftNullifier = statement[3];
        uint256 commitLocked = statement[4];
        uint256 nftTokenId   = statement[5];
        uint256 revertCommit = statement[6];

        if (_auctions[auctionId].state != AuctionState.INACTIVE) revert AuctionAlreadyExists();
        if (deadline <= block.timestamp)                          revert InvalidDeadline();
        if (settlementDeadline < deadline + 2 days)               revert InvalidSettlementDeadline();

        // Verify the AuctionLock ZK proof.
        _verifyProof(VK_LOCK, proof, _toArr7(statement));

        // Ensure the Merkle root Alice used matches a root the NFT vault accepts.
        if (!ICoinVault(_erc721Vault).verifyRoot(treeNumber, merkleRoot))
            revert InvalidMerkleRoot();

        // Lock the NFT nullifier so Bob cannot spend the note while the auction is live.
        ICoinVault(_erc721Vault).lockCoin(treeNumber, nftNullifier);

        _auctions[auctionId] = AuctionCore({
            state:               AuctionState.BIDDING,
            nftTokenId:          nftTokenId,
            commitLocked:        commitLocked,
            revertCommit:        revertCommit,
            nftNullifier:        nftNullifier,
            nftTreeNumber:       treeNumber,
            overallWinnerCommit: 0,
            batchCount:          0,
            deadline:            deadline,
            settlementDeadline:  settlementDeadline,
            floorPrice:          floorPrice,
            bidCount:            0
        });

        emit AuctionInitialized(auctionId, commitLocked, revertCommit, nftTokenId, deadline, settlementDeadline, floorPrice);
        return true;
    }

    // -----------------------------------------------------------------------
    // Phase 0b — Alice (bidder) submits a sealed bid — only before deadline
    // -----------------------------------------------------------------------
    //
    // AuctionBid public statement (7 elements):
    //   [0] StAuctionId   — must match an open auction
    //   [1] StTreeNumber  — bidder's USDC sub-tree
    //   [2] StMerkleRoot  — current ERC-20 Merkle root
    //   [3] StNullifier   — Poseidon(sk_A, leafIndex) — burns the input USDC note
    //   [4] StCommitA     — bidder's locked bid commitment (held in escrow)
    //   [5] StCommitB     — USDC payout destination for the seller if this bid wins
    //   [6] StRevertCommit — Erc20CommitmentV2(pk_A, saltRevert, amount, tokenId)

    function submitBid(
        uint256[8] calldata proof,
        uint256[7] calldata statement,
        bytes      calldata ctxt1,
        bytes      calldata ctxt2
    ) external nonReentrant returns (bool) {
        uint256 auctionId    = statement[0];
        uint256 treeNumber   = statement[1];
        uint256 merkleRoot   = statement[2];
        uint256 nullifier    = statement[3];
        uint256 commitA      = statement[4];
        uint256 commitB      = statement[5];
        uint256 revertCommit = statement[6];

        if (_auctions[auctionId].state != AuctionState.BIDDING) revert AuctionStateMismatch();
        if (block.timestamp >= _auctions[auctionId].deadline)   revert BiddingClosed();
        if (_bids[auctionId][commitA].active)                   revert BidAlreadyExists();

        // Verify the AuctionBid ZK proof.
        _verifyProof(VK_BID, proof, _toArr7(statement));

        // Ensure the Merkle root matches a root the USDC vault accepts.
        if (!ICoinVault(_erc20Vault).verifyRoot(treeNumber, merkleRoot))
            revert InvalidMerkleRoot();

        // Lock the bidder's USDC note so they cannot double-spend it while the
        // auction is live. The note is permanently spent (unlock + nullify) at
        // settlement, withdrawal, or reclaim time — never at submit time.
        ICoinVault(_erc20Vault).lockCoin(treeNumber, nullifier);

        _bids[auctionId][commitA] = BidData({
            active:       true,
            claimed:      false,
            batched:      false,
            commitB:      commitB,
            revertCommit: revertCommit,
            nullifier:    nullifier,
            treeNumber:   treeNumber
        });
        _auctions[auctionId].bidCount++;

        // Publish the ML-KEM capsule and encrypted bid data on-chain so the
        // auctioneer can monitor events and decrypt without trusting off-chain relays.
        emit BidSubmitted(auctionId, commitA, commitB, revertCommit, ctxt1, ctxt2);
        return true;
    }

    // -----------------------------------------------------------------------
    // Phase 0c — Bidder withdraws their bid before the deadline
    // -----------------------------------------------------------------------
    //
    // AuctionWithdraw public statement (2 elements):
    //   [0] StAuctionId — must match an open BIDDING auction
    //   [1] StCommitA   — the bid commitment being withdrawn
    //
    // The ZK proof proves the caller knows the preimage of commitA (i.e., knows
    // saltA), authenticating them as the original bidder. The contract then
    // permanently spends the locked USDC note and inserts the pre-committed
    // revertCommit into the USDC tree under a fresh, unlinkable salt.
    // The bid is marked inactive so submitBatch will reject it.
    //
    // Withdrawal is only permitted while state == BIDDING and before the deadline.
    // After the deadline the auctioneer has already started decrypting bids, and
    // allowing withdrawal at that point would let bidders selectively retract after
    // observing partial batch results.

    function withdrawBid(
        uint256[8] calldata proof,
        uint256[2] calldata statement
    ) external nonReentrant returns (bool) {
        uint256 auctionId = statement[0];
        uint256 commitA   = statement[1];

        // Access state sequentially without caching storage refs to avoid
        // simultaneous live variables exceeding the 16-slot EVM stack limit.
        if (_auctions[auctionId].state != AuctionState.BIDDING) revert AuctionStateMismatch();
        if (block.timestamp >= _auctions[auctionId].deadline)   revert WithdrawalDeadlinePassed();
        if (!_bids[auctionId][commitA].active)                  revert BidNotFound();
        if (_bids[auctionId][commitA].claimed)                  revert BidAlreadyClaimed();

        uint256[] memory inputs = new uint256[](2);
        inputs[0] = auctionId;
        inputs[1] = commitA;
        _verifyProofDyn(VK_WITHDRAW, proof, inputs);

        // Spend the bid note and return funds via revertCommit. Extracted to a
        // helper to bound stack depth (same pattern as _settleUsdcVault).
        _withdrawBidFunds(auctionId, commitA);
        _auctions[auctionId].bidCount--;

        emit BidWithdrawn(auctionId, commitA);
        return true;
    }

    function _withdrawBidFunds(uint256 auctionId, uint256 commitA) internal {
        BidData storage bid = _bids[auctionId][commitA];
        ICoinVault(_erc20Vault).unlockCoin(bid.treeNumber, bid.nullifier);
        ICoinVault(_erc20Vault).nullifyCoin(bid.treeNumber, bid.nullifier);
        uint256[] memory commits = new uint256[](1);
        commits[0] = bid.revertCommit;
        ICoinVault(_erc20Vault).registerCoins(commits);
        bid.active  = false;
        bid.claimed = true;
    }

    // -----------------------------------------------------------------------
    // Phase 1 — Auctioneer opens a batch of up to 100 bids — only after deadline
    // -----------------------------------------------------------------------
    //
    // AuctionBatch public statement (104 elements):
    //   [0]       StAuctionId
    //   [1..100]  StBidCommitA[0..99]  — on-chain commitA values (0 = inactive slot)
    //   [101]     StBatchWinnerCommit  — commitA of the batch winner
    //   [102]     StBatchWinnerPk      — spend public key of the batch winner
    //   [103]     StBatchWinnerAmount  — bid amount of the batch winner
    //
    // The contract verifies every non-zero StBidCommitA[i] was previously
    // registered by submitBid, preventing the auctioneer from including phantom bids.

    function submitBatch(
        uint256      batchId,
        uint256[8]   calldata proof,
        uint256[104] calldata statement
    ) external nonReentrant onlyRole(AUCTIONEER_ROLE) returns (bool) {
        uint256 auctionId = statement[0];

        if (_auctions[auctionId].state != AuctionState.BIDDING) revert AuctionStateMismatch();
        if (block.timestamp < _auctions[auctionId].deadline)    revert BidsStillOpen();
        if (_batches[auctionId][batchId].submitted)              revert BatchAlreadySubmitted();

        // Every active slot must reference a real, unbatched on-chain bid.
        // Marking each bid as batched here prevents the same commitA from
        // appearing in a second submitBatch call (cross-batch duplicate).
        for (uint256 i = 0; i < 100; i++) {
            uint256 commitA = statement[1 + i];
            if (commitA == 0) continue;
            BidData storage b = _bids[auctionId][commitA];
            if (!b.active)  revert UnknownBid(commitA);
            if (b.batched)  revert BidAlreadyBatched(commitA);
            b.batched = true;
        }

        // Verify the AuctionBatch ZK proof.
        uint256[] memory inputs = new uint256[](104);
        for (uint256 i = 0; i < 104; i++) inputs[i] = statement[i];
        _verifyProofDyn(VK_BATCH, proof, inputs);

        uint256 batchWinnerCommit = statement[101];
        uint256 batchWinnerPk    = statement[102];
        uint256 batchWinnerAmount = statement[103];

        _batches[auctionId][batchId] = BatchResult({
            submitted:         true,
            batchWinnerCommit: batchWinnerCommit,
            batchWinnerPk:     batchWinnerPk,
            batchWinnerAmount: batchWinnerAmount
        });

        if (_auctions[auctionId].batchCount >= 10) revert MaxBatchesExceeded();
        _auctions[auctionId].batchCount++;

        emit BatchProcessed(auctionId, batchId, batchWinnerCommit, batchWinnerAmount);
        return true;
    }

    // -----------------------------------------------------------------------
    // Phase 2 — ZK-optimistic settlement: claim now, verify later (or never)
    //
    // AuctionFinal public statement (38 elements):
    //   [0]       StAuctionId
    //   [1..10]   StBatchWinnerCommit[0..9]   — from Phase-1 proofs
    //   [11..20]  StBatchWinnerPk[0..9]
    //   [21..30]  StBatchWinnerAmount[0..9]
    //   [31]      StOverallWinnerCommit        — the winning commitA
    //   [32]      StWinnerPk                   — winner's spend public key
    //   [33]      StWinnerCommitB              — USDC payout for the seller
    //   [34]      StWinnerNftCommit            — new NFT commitment for winner
    //   [35]      StNftTokenId
    //   [36]      StWinningAmount
    //   [37]      StFloorPrice
    // -----------------------------------------------------------------------

    /// @notice Auctioneer posts a settlement claim without verifying the proof
    ///         on-chain. Everything that CAN be checked for free against
    ///         already-verified Phase-1 data (winner selection) is checked
    ///         immediately; only the cryptographic construction of
    ///         StWinnerCommitB / StWinnerNftCommit is deferred.
    function settleOptimistic(
        uint256[8]  calldata proof,
        uint256[38] calldata statement,
        uint256[10] calldata batchIds
    ) external nonReentrant onlyRole(AUCTIONEER_ROLE) returns (bool) {
        uint256 auctionId = statement[0];

        if (_auctions[auctionId].state != AuctionState.BIDDING) revert AuctionStateMismatch();
        if (block.timestamp < _auctions[auctionId].deadline)    revert DeadlineNotReached();

        // Verify all submitted batches are present and no batchId is duplicated.
        // Without this an auctioneer could omit the batch containing the highest
        // bidder or repeat a lower batch to fill all 10 Phase-2 slots.
        //
        // Additionally enforce that batchIds[i] and statement[1+i] agree on
        // zero/non-zero: a phantom slot (batchIds[i] != 0 but statement[1+i] == 0)
        // passes the count check but is silently skipped by _crossCheckBatches,
        // allowing the auctioneer to hide a high-bidding batch from the circuit.
        uint256 nonZeroCount;
        for (uint256 i = 0; i < 10; i++) {
            bool hasId     = batchIds[i] != 0;
            bool hasCommit = statement[1 + i] != 0;
            if (hasId != hasCommit) revert BatchSlotMismatch(i);
            if (!hasId) continue;
            for (uint256 j = 0; j < i; j++) {
                require(batchIds[j] != batchIds[i], "EnygmaAuction: duplicate batchId");
            }
            nonZeroCount++;
        }
        require(
            nonZeroCount == _auctions[auctionId].batchCount,
            "EnygmaAuction: not all batches included"
        );

        _crossCheckBatches(auctionId, statement, batchIds);

        // Ensure the overall winner (statement[31]) is one of the verified batch
        // winners (statement[1..10]).  Without this check the auctioneer could name
        // an arbitrary address as winner in Phase-2 and have it pass unchallenged if
        // no watchdog calls challengeSettlement() within the window.
        bool winnerFound = false;
        for (uint256 i = 0; i < 10; i++) {
            if (statement[1 + i] != 0 && statement[1 + i] == statement[31]) {
                winnerFound = true;
                break;
            }
        }
        require(winnerFound, "EnygmaAuction: overall winner not in any verified batch");

        if (statement[35] != _auctions[auctionId].nftTokenId) revert InvalidNftTokenId();
        if (statement[37] != _auctions[auctionId].floorPrice) revert FloorPriceMismatch();

        OptimisticClaim storage claim = _claims[auctionId];
        claim.pending           = true;
        claim.auctioneer        = msg.sender;
        claim.challengeDeadline = block.timestamp + challengeWindow;
        claim.proof             = proof;
        claim.batchIds          = batchIds;
        claim.statement         = statement;

        _auctions[auctionId].state = AuctionState.PENDING_SETTLEMENT;

        emit AuctionSettlementProposed(
            auctionId, msg.sender, statement[31], statement[36], claim.challengeDeadline
        );
        return true;
    }

    /// @notice Anyone can force real verification of a pending claim's proof
    ///         within the challenge window. Resolves immediately and deterministically
    ///         — Groth16 verification is a one-shot check. Invalid proof voids the
    ///         claim and reopens bidding; valid proof finalizes settlement.
    // Minimum gas that must remain before entering the verifyProof call inside
    // challengeSettlement. Prevents a griefer from supplying exactly enough gas
    // to reach the try block but not enough for verifyProof to complete: Solidity
    // try/catch catches OOG the same as an explicit false return, so an OOG would
    // void a valid optimistic claim and reopen bidding.
    // Set conservatively above the ~200k–400k BN254 Groth16 pairing cost.
    uint256 private constant CHALLENGE_VERIFY_GAS = 600_000;

    function challengeSettlement(uint256 auctionId) external nonReentrant returns (bool) {
        if (_auctions[auctionId].state != AuctionState.PENDING_SETTLEMENT) revert NoPendingSettlement();
        OptimisticClaim storage claim = _claims[auctionId];
        if (block.timestamp >= claim.challengeDeadline) revert ChallengeWindowClosed();
        require(gasleft() >= CHALLENGE_VERIFY_GAS, "EnygmaAuction: insufficient gas for verification");

        uint256[] memory inputs = new uint256[](38);
        for (uint256 i = 0; i < 38; i++) inputs[i] = claim.statement[i];

        bool valid;
        try IVerifier(_verifier).verifyProof(VK_FINAL, _makeSnarkProof(claim.proof), inputs) returns (bool ok) {
            valid = ok;
        } catch {
            valid = false;
        }

        if (!valid) {
            // Auctioneer's claim was fraudulent (or malformed) — void the claim
            // and reopen bidding so the auctioneer can resubmit via settleOptimistic().
            delete _claims[auctionId];
            _auctions[auctionId].state = AuctionState.BIDDING;
            emit AuctionSettlementChallenged(auctionId, msg.sender, true);
            return true;
        }

        // Claim is valid — finalize settlement now that validity is proven.
        emit AuctionSettlementChallenged(auctionId, msg.sender, false);
        uint256[38] memory finalStatement = claim.statement;
        delete _claims[auctionId];
        _applySettlement(auctionId, finalStatement);
        return true;
    }

    /// @notice Anyone can finalize an unchallenged claim once the challenge window
    ///         has elapsed. Trusts the claim without ever invoking the verifier.
    function finalizeSettlement(uint256 auctionId) external nonReentrant returns (bool) {
        if (_auctions[auctionId].state != AuctionState.PENDING_SETTLEMENT) revert NoPendingSettlement();
        OptimisticClaim storage claim = _claims[auctionId];
        if (block.timestamp < claim.challengeDeadline) revert ChallengeWindowOpen();

        uint256[38] memory finalStatement = claim.statement;
        delete _claims[auctionId];

        _applySettlement(auctionId, finalStatement);
        return true;
    }

    // -----------------------------------------------------------------------
    // Phase 3 — Bob cancels the auction at any time (before or after deadline)
    // -----------------------------------------------------------------------
    //
    // AuctionRevert public statement (4 elements):
    //   [0] StAuctionId
    //   [1] StCommitLocked   — must match auctions[id].commitLocked
    //   [2] StNftTokenId     — must match auctions[id].nftTokenId
    //   [3] StRevertedCommit — fresh commitment, re-inserted into the NFT tree
    //
    // Bob may cancel at any time while the auction is in BIDDING state. The ZK
    // proof proves he owns the locked note; no timing restriction is needed.
    // Bidders reclaim their USDC via reclaimBid() once the auction is CANCELED.

    function revertAuction(
        uint256[8] calldata proof,
        uint256[4] calldata statement
    ) external nonReentrant returns (bool) {
        uint256 auctionId      = statement[0];
        uint256 commitLocked   = statement[1];
        uint256 nftTokenId     = statement[2];
        uint256 revertedCommit = statement[3];

        AuctionCore storage a = _auctions[auctionId];
        if (a.state != AuctionState.BIDDING)   revert AuctionStateMismatch();
        if (block.timestamp >= a.deadline)     revert BiddingClosed();
        if (commitLocked != a.commitLocked)    revert CommitLockedMismatch();
        if (nftTokenId    != a.nftTokenId)     revert InvalidNftTokenId();

        uint256[] memory inputs = new uint256[](4);
        for (uint256 i = 0; i < 4; i++) inputs[i] = statement[i];
        _verifyProofDyn(VK_REVERT, proof, inputs);

        // Spend Bob's locked NFT note: unlock then permanently nullify — the
        // same vault calls as settlement, so a chain observer cannot distinguish
        // "sold" from "canceled" at the vault-spend level.
        ICoinVault(_erc721Vault).unlockCoin(a.nftTreeNumber, a.nftNullifier);
        ICoinVault(_erc721Vault).nullifyCoin(a.nftTreeNumber, a.nftNullifier);

        // Insert the freshly-salted commitment — no public link back to commitLocked.
        uint256[] memory nftCommits = new uint256[](1);
        nftCommits[0] = revertedCommit;
        ICoinVault(_erc721Vault).registerCoins(nftCommits);

        a.state = AuctionState.CANCELED;

        emit AuctionReverted(auctionId, revertedCommit);
        return true;
    }

    // -----------------------------------------------------------------------
    // Phase 3b — Timeout recovery: anyone triggers after settlement deadline
    // -----------------------------------------------------------------------
    //
    // If the auctioneer has not settled by settlementDeadline, anyone can call
    // recoverAuction() to return Bob's NFT via the pre-committed revert commitment
    // that was proven well-formed at initAuction() time. No new ZK proof required.
    //
    // After this call, individual losing bidders reclaim their USDC via reclaimBid().

    function recoverAuction(uint256 auctionId) external nonReentrant returns (bool) {
        AuctionCore storage a = _auctions[auctionId];
        if (a.state != AuctionState.BIDDING)              revert AuctionStateMismatch();
        if (block.timestamp < a.settlementDeadline)        revert SettlementDeadlineNotReached();

        // Spend Bob's locked NFT note: unlock then permanently nullify.
        ICoinVault(_erc721Vault).unlockCoin(a.nftTreeNumber, a.nftNullifier);
        ICoinVault(_erc721Vault).nullifyCoin(a.nftTreeNumber, a.nftNullifier);

        // Insert the pre-committed revert commitment — no ZK proof needed because
        // AuctionLock circuit already proved it is well-formed and unlinkable from
        // commitLocked (different salt, AssertIsDifferent enforced in-circuit).
        uint256[] memory nftCommits = new uint256[](1);
        nftCommits[0] = a.revertCommit;
        ICoinVault(_erc721Vault).registerCoins(nftCommits);

        a.state = AuctionState.CANCELED;

        emit AuctionRecovered(auctionId, a.revertCommit);
        return true;
    }

    // -----------------------------------------------------------------------
    // Phase 4 — Bidders reclaim their locked USDC
    // -----------------------------------------------------------------------
    //
    // No ZK proof is required. The contract checks:
    //   1. Auction is SETTLED or CANCELED.
    //   2. commitA was registered by submitBid.
    //   3. commitA is not the overall winner's commitment (always false when CANCELED).
    //   4. commitA has not already been reclaimed.
    //
    // On success, the bidder's pre-committed revertCommit is inserted into the
    // ERC-20 Merkle tree. Using revertCommit (not commitA) breaks the on-chain
    // link between the public bid and the returned note, preserving privacy.

    function reclaimBid(
        uint256 auctionId,
        uint256 commitA
    ) external nonReentrant returns (bool) {
        AuctionState state = _auctions[auctionId].state;
        if (state != AuctionState.SETTLED && state != AuctionState.CANCELED)
            revert AuctionStateMismatch();
        if (!_bids[auctionId][commitA].active)                  revert BidNotFound();
        if (_bids[auctionId][commitA].claimed)                  revert BidAlreadyClaimed();
        if (commitA == _auctions[auctionId].overallWinnerCommit) revert WinnerCannotReclaim();

        // Permanently spend the bidder's locked USDC note and insert the
        // pre-committed revert commitment. Extracted to avoid stack-depth issues.
        _withdrawBidFunds(auctionId, commitA);

        emit BidReclaimed(auctionId, commitA);
        return true;
    }

    // -----------------------------------------------------------------------
    // View helpers
    // -----------------------------------------------------------------------

    function getAuctionState(uint256 auctionId) external view returns (AuctionState) {
        return _auctions[auctionId].state;
    }

    function getAuctionCore(uint256 auctionId) external view returns (
        AuctionState state,
        uint256 nftTokenId,
        uint256 commitLocked,
        uint256 revertCommit,
        uint256 overallWinnerCommit,
        uint256 batchCount,
        uint256 deadline,
        uint256 settlementDeadline,
        uint256 floorPrice,
        uint256 bidCount
    ) {
        AuctionCore storage a = _auctions[auctionId];
        return (
            a.state, a.nftTokenId, a.commitLocked, a.revertCommit,
            a.overallWinnerCommit, a.batchCount, a.deadline,
            a.settlementDeadline, a.floorPrice, a.bidCount
        );
    }

    function getBid(
        uint256 auctionId,
        uint256 commitA
    ) external view returns (BidData memory) {
        return _bids[auctionId][commitA];
    }

    function getBatchResult(
        uint256 auctionId,
        uint256 batchId
    ) external view returns (BatchResult memory) {
        return _batches[auctionId][batchId];
    }

    function getOptimisticClaim(uint256 auctionId) external view returns (OptimisticClaim memory) {
        return _claims[auctionId];
    }

    // -----------------------------------------------------------------------
    // Internal helpers
    // -----------------------------------------------------------------------

    /// @dev Cross-references each active batch slot in an AuctionFinal statement
    ///      against the stored Phase-1 result. This is pure public-data checking
    ///      (no ZK proof needed) since BatchResults were already proven on-chain.
    function _crossCheckBatches(
        uint256 auctionId,
        uint256[38] calldata statement,
        uint256[10] calldata batchIds
    ) internal view {
        for (uint256 i = 0; i < 10; i++) {
            uint256 slotCommit = statement[1 + i];
            if (slotCommit == 0) continue;

            BatchResult storage br = _batches[auctionId][batchIds[i]];
            if (!br.submitted)                            revert BatchNotFound();
            if (br.batchWinnerCommit != slotCommit)       revert BatchResultMismatch(i);
            if (br.batchWinnerPk     != statement[11 + i]) revert BatchResultMismatch(i);
            if (br.batchWinnerAmount != statement[21 + i]) revert BatchResultMismatch(i);
        }
    }

    /// @dev Shared settlement application, invoked from a failed challenge
    ///      (proof turned out valid) and from finalizeSettlement(). This
    ///      is the single place that releases funds and flips state to SETTLED,
    ///      so every entry path re-checks nftTokenId/floorPrice consistency.
    ///      All vault interaction is delegated to focused helpers to stay within
    ///      the EVM 16-slot stack limit when viaIR + optimizer are in use.
    function _applySettlement(uint256 auctionId, uint256[38] memory statement) internal {
        _checkAndFinalizeState(auctionId, statement[31], statement[35], statement[37]);
        _settleNftVault(auctionId, statement[34]);
        _settleUsdcVault(auctionId, statement[31], statement[33]);
        emit AuctionSettled(auctionId, statement[31], statement[33], statement[34], statement[36]);
    }

    function _checkAndFinalizeState(
        uint256 auctionId,
        uint256 overallWinnerCommit,
        uint256 nftTokenId,
        uint256 floorPrice
    ) internal {
        AuctionCore storage a = _auctions[auctionId];
        if (nftTokenId != a.nftTokenId) revert InvalidNftTokenId();
        if (floorPrice  != a.floorPrice) revert FloorPriceMismatch();
        a.overallWinnerCommit = overallWinnerCommit;
        a.state               = AuctionState.SETTLED;
    }

    function _settleNftVault(uint256 auctionId, uint256 nftCommit) internal {
        AuctionCore storage a = _auctions[auctionId];
        ICoinVault(_erc721Vault).unlockCoin(a.nftTreeNumber, a.nftNullifier);
        ICoinVault(_erc721Vault).nullifyCoin(a.nftTreeNumber, a.nftNullifier);
        uint256[] memory commits = new uint256[](1);
        commits[0] = nftCommit;
        ICoinVault(_erc721Vault).registerCoins(commits);
    }

    function _settleUsdcVault(
        uint256 auctionId,
        uint256 winnerCommit,
        uint256 payoutCommit
    ) internal {
        BidData storage winnerBid = _bids[auctionId][winnerCommit];
        // Bind settlement to exactly the commitB Alice published at submitBid time.
        // Without this check the AuctionFinal circuit's private WtPkBob witness
        // lets the auctioneer substitute an arbitrary recipient for the USDC payout.
        require(payoutCommit == winnerBid.commitB, "EnygmaAuction: payout commitment mismatch");
        ICoinVault(_erc20Vault).unlockCoin(winnerBid.treeNumber, winnerBid.nullifier);
        ICoinVault(_erc20Vault).nullifyCoin(winnerBid.treeNumber, winnerBid.nullifier);
        winnerBid.claimed = true;
        uint256[] memory commits = new uint256[](1);
        commits[0] = payoutCommit;
        ICoinVault(_erc20Vault).registerCoins(commits);
    }

    function _verifyProof(
        uint256    vkId,
        uint256[8] calldata p,
        uint256[]  memory   inputs
    ) internal view {
        bool ok = IVerifier(_verifier).verifyProof(vkId, _makeSnarkProof(p), inputs);
        require(ok, "EnygmaAuction: invalid proof");
    }

    function _verifyProofDyn(
        uint256    vkId,
        uint256[8] memory   p,
        uint256[]  memory   inputs
    ) internal view {
        bool ok = IVerifier(_verifier).verifyProof(vkId, _makeSnarkProof(p), inputs);
        require(ok, "EnygmaAuction: invalid proof");
    }

    /// @dev Converts the flat [ax,ay,bx1,bx0,by1,by0,cx,cy] array emitted by
    ///      the gnark server into the SnarkProof struct expected by IVerifier.
    ///      gnark's G2 element: X.A1 = p[2], X.A0 = p[3]; Y.A1 = p[4], Y.A0 = p[5].
    function _makeSnarkProof(
        uint256[8] memory p
    ) internal pure returns (SnarkProof memory) {
        return SnarkProof({
            a: G1Point(p[0], p[1]),
            b: G2Point([p[2], p[3]], [p[4], p[5]]),
            c: G1Point(p[6], p[7])
        });
    }

    function _toArr6(uint256[6] calldata s) internal pure returns (uint256[] memory a) {
        a = new uint256[](6);
        for (uint256 i = 0; i < 6; i++) a[i] = s[i];
    }

    function _toArr7(uint256[7] calldata s) internal pure returns (uint256[] memory a) {
        a = new uint256[](7);
        for (uint256 i = 0; i < 7; i++) a[i] = s[i];
    }
}
