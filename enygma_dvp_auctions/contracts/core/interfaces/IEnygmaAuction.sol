// SPDX-License-Identifier: BUSL-1.1
pragma solidity ^0.8.20;

/// @title IEnygmaAuction
/// @notice Interface for the two-phase ZK sealed-bid NFT auction contract.
interface IEnygmaAuction {

    // -------------------------------------------------------------------------
    // Enums
    // -------------------------------------------------------------------------

    enum AuctionState {
        INACTIVE,           // auction does not exist
        BIDDING,             // open for bids and batch processing
        PENDING_SETTLEMENT,  // an optimistic settlement claim is awaiting its challenge window
        SETTLED,             // final result accepted; winner NFT and USDC distributed
        CANCELED             // reverted — no qualifying settlement; NFT returned to Bob
    }

    // -------------------------------------------------------------------------
    // Structs
    // -------------------------------------------------------------------------

    struct BidData {
        bool     active;        // bid exists on-chain
        bool     claimed;       // commitA/revertCommit has been released into the tree
        bool     batched;       // true once included in any submitBatch call; prevents cross-batch duplicates
        uint256  commitB;       // USDC payout destination if this bidder wins
        uint256  revertCommit;  // Erc20CommitmentV2(pk_A, saltRevert, amount, tokenId) — pre-committed recovery
        uint256  nullifier;     // nullifier of the bidder's input USDC note
        uint256  treeNumber;    // sub-tree the USDC input was in
    }

    struct BatchResult {
        bool     submitted;
        uint256  batchWinnerCommit;   // commitA of the batch winner
        uint256  batchWinnerPk;       // spend public key of the batch winner
        uint256  batchWinnerAmount;   // bid amount of the batch winner
    }

    /// @notice An auctioneer's optimistic settlement claim, pending challenge.
    struct OptimisticClaim {
        bool        pending;
        address     auctioneer;
        uint256     challengeDeadline;
        uint256[38] statement;
        uint256[10] batchIds;
        uint256[8]  proof;
    }

    // -------------------------------------------------------------------------
    // Events
    // -------------------------------------------------------------------------

    /// @notice Emitted when Bob locks his NFT and opens the bidding.
    event AuctionInitialized(
        uint256 indexed auctionId,
        uint256 commitLocked,
        uint256 revertCommit,
        uint256 nftTokenId,
        uint256 deadline,
        uint256 settlementDeadline,
        uint256 floorPrice
    );

    /// @notice Emitted when a bidder submits a sealed bid.
    /// @param ctxt1 ML-KEM-768 capsule (1 088 bytes) — only the auctioneer can decapsulate.
    /// @param ctxt2 AEAD ciphertext of (pkBob, tokenId, amount) with AAD = ctxt1.
    event BidSubmitted(
        uint256 indexed auctionId,
        uint256 indexed commitA,
        uint256 commitB,
        uint256 revertCommit,
        bytes   ctxt1,
        bytes   ctxt2
    );

    /// @notice Emitted when the auctioneer submits a Phase-1 batch result.
    event BatchProcessed(
        uint256 indexed auctionId,
        uint256 indexed batchId,
        uint256 batchWinnerCommit,
        uint256 batchWinnerAmount
    );

    /// @notice Emitted when the auctioneer posts an optimistic (unverified) settlement claim.
    event AuctionSettlementProposed(
        uint256 indexed auctionId,
        address indexed auctioneer,
        uint256 claimedOverallWinnerCommit,
        uint256 claimedWinningAmount,
        uint256 challengeDeadline
    );

    /// @notice Emitted when someone challenges a pending optimistic claim.
    /// @param success true if the proof was found invalid (claim voided, auctioneer slashed);
    ///                 false if the proof verified (claim stands, challenger slashed).
    event AuctionSettlementChallenged(
        uint256 indexed auctionId,
        address indexed challenger,
        bool    success
    );

    /// @notice Emitted when the auction is fully settled — via a failed challenge
    ///         (proof found valid) or an unchallenged optimistic claim finalizing.
    event AuctionSettled(
        uint256 indexed auctionId,
        uint256 overallWinnerCommit,
        uint256 winnerCommitB,
        uint256 winnerNftCommit,
        uint256 winningAmount
    );

    /// @notice Emitted when a bidder withdraws their bid before the deadline.
    event BidWithdrawn(uint256 indexed auctionId, uint256 indexed commitA);

    /// @notice Emitted when a losing bidder reclaims their locked USDC.
    event BidReclaimed(uint256 indexed auctionId, uint256 indexed commitA);

    /// @notice Emitted when the auction is canceled and the NFT reverted to Bob
    ///         under a fresh, unlinkable commitment.
    event AuctionReverted(uint256 indexed auctionId, uint256 revertedCommit);

    /// @notice Emitted when anyone triggers timeout recovery after the settlement
    ///         deadline, returning the NFT to Bob via the pre-committed revert commitment.
    event AuctionRecovered(uint256 indexed auctionId, uint256 revertCommit);

    // -------------------------------------------------------------------------
    // Errors
    // -------------------------------------------------------------------------

    error AuctionAlreadyExists();
    error AuctionNotFound();
    error AuctionStateMismatch();
    error BidAlreadyExists();
    error BidNotFound();
    error BidAlreadyClaimed();
    error WinnerCannotReclaim();
    error BatchAlreadySubmitted();
    error MaxBatchesExceeded();
    error BatchNotFound();
    error UnknownBid(uint256 commitA);
    error BidAlreadyBatched(uint256 commitA);
    error BatchResultMismatch(uint256 slot);
    error BatchSlotMismatch(uint256 slot);
    error InvalidMerkleRoot();
    error InvalidNftTokenId();
    error CommitLockedMismatch();
    error InvalidDeadline();
    error InvalidSettlementDeadline();
    error BiddingClosed();
    error BidsStillOpen();
    error DeadlineNotReached();
    error SettlementDeadlineNotReached();
    error FloorPriceMismatch();
    error NoPendingSettlement();
    error ChallengeWindowClosed();
    error ChallengeWindowOpen();
    error WithdrawalDeadlinePassed();

    // -------------------------------------------------------------------------
    // Functions
    // -------------------------------------------------------------------------

    /// @notice Bob locks his NFT and creates the auction.
    /// @param proof               Groth16 proof [ax,ay,bx1,bx0,by1,by0,cx,cy]
    /// @param statement           [StAuctionId, StTreeNumber, StMerkleRoot, StNullifier, StCommitLocked, StNftTokenId, StRevertCommit]
    /// @param deadline            Unix timestamp after which bidding closes and settlement/revert may proceed.
    /// @param settlementDeadline  Unix timestamp after which anyone can trigger timeout recovery.
    ///                            Must be strictly greater than deadline.
    /// @param floorPrice          Seller's reserve price; enforced in-circuit by AuctionFinal.
    function initAuction(
        uint256[8]  calldata proof,
        uint256[7]  calldata statement,
        uint256     deadline,
        uint256     settlementDeadline,
        uint256     floorPrice
    ) external returns (bool);

    /// @notice A bidder locks their USDC and submits a sealed bid. Only before deadline.
    /// @param proof      Groth16 proof
    /// @param statement  [StAuctionId, StTreeNumber, StMerkleRoot, StNullifier, StCommitA, StCommitB, StRevertCommit]
    /// @param ctxt1      ML-KEM capsule (published so the auctioneer can decrypt)
    /// @param ctxt2      AEAD ciphertext of bid details
    function submitBid(
        uint256[8]  calldata proof,
        uint256[7]  calldata statement,
        bytes       calldata ctxt1,
        bytes       calldata ctxt2
    ) external returns (bool);

    /// @notice Auctioneer submits a Phase-1 batch result for up to 100 bids. Only after deadline.
    /// @param batchId    Caller-chosen identifier for this batch (unique per auction).
    /// @param proof      Groth16 proof
    /// @param statement  [StAuctionId, StBidCommitA[100], StBatchWinnerCommit, StBatchWinnerPk, StBatchWinnerAmount]
    function submitBatch(
        uint256     batchId,
        uint256[8]  calldata proof,
        uint256[104] calldata statement
    ) external returns (bool);

    /// @notice Auctioneer posts a settlement claim optimistically — the proof is stored
    ///         but NOT verified on-chain yet. Moves the auction to PENDING_SETTLEMENT
    ///         for the challenge window.
    function settleOptimistic(
        uint256[8]  calldata proof,
        uint256[38] calldata statement,
        uint256[10] calldata batchIds
    ) external returns (bool);

    /// @notice Anyone may challenge a pending optimistic claim within the challenge
    ///         window by forcing real verification of the stored proof.
    ///         Resolves immediately: invalid proof voids the claim and reopens bidding
    ///         (auctioneer must resubmit via settleOptimistic); valid proof finalizes
    ///         the auction as SETTLED.
    function challengeSettlement(uint256 auctionId) external returns (bool);

    /// @notice Anyone may finalize an optimistic claim once its challenge window has
    ///         elapsed with no challenge — trusts the claim without ever verifying it.
    function finalizeSettlement(uint256 auctionId) external returns (bool);

    /// @notice Bob cancels the auction and reclaims his NFT under a fresh, unlinkable
    ///         commitment. May be called at any time while the auction is in BIDDING
    ///         state — before or after the bidding deadline. After CANCELED, all bidders
    ///         may call reclaimBid() to recover their USDC via their pre-committed revertCommit.
    /// @param proof      Groth16 proof for AuctionRevertCircuit
    /// @param statement  [StAuctionId, StCommitLocked, StNftTokenId, StRevertedCommit]
    function revertAuction(
        uint256[8] calldata proof,
        uint256[4] calldata statement
    ) external returns (bool);

    /// @notice Anyone can trigger timeout recovery once the settlement deadline has
    ///         passed without a settled auction. Returns Bob's NFT via the pre-committed
    ///         revert commitment stored at initAuction time. No ZK proof required.
    function recoverAuction(uint256 auctionId) external returns (bool);

    /// @notice A bidder withdraws their sealed bid before the bidding deadline.
    ///         Requires a ZK proof proving ownership of commitA. The bidder's locked
    ///         USDC note is permanently spent and revertCommit is inserted into the
    ///         USDC tree. The bid is marked inactive so the auctioneer cannot batch it.
    ///         Only callable while state == BIDDING and block.timestamp < deadline.
    /// @param proof      Groth16 proof for AuctionWithdrawCircuit
    /// @param statement  [StAuctionId, StCommitA]
    function withdrawBid(
        uint256[8] calldata proof,
        uint256[2] calldata statement
    ) external returns (bool);

    /// @notice A bidder inserts their pre-committed revert commitment into the USDC
    ///         tree to reclaim funds. Identified by commitA (stored at submitBid time).
    ///         No ZK proof required — the contract checks commitA ≠ overallWinnerCommit.
    function reclaimBid(uint256 auctionId, uint256 commitA) external returns (bool);
}
