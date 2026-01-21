// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;

interface IZkDvp {
    ///////////////////////////////////////////////
    //                  Structs
    //////////////////////////////////////////////
    struct G1Point {
        uint256 x;
        uint256 y;
    }
    struct G2Point {
        uint256[2] x;
        uint256[2] y;
    }

    struct VerifyingKey {
        G1Point alpha1;
        G2Point beta2;
        G2Point gamma2;
        G2Point delta2;
        G1Point[] ic;
    }

    struct SnarkProof {
        G1Point a;
        G2Point b;
        G1Point c;
    }

    struct ZkPrivateMintProof {
        uint256[8] proof;
        uint256[4] public_signal;
    }

    // struct including the Ownership circuit
    // statement arguments + the proof
    struct ProofReceipt {
        SnarkProof proof;
        uint256[] statement;
        uint256 numberOfInputs;
        uint256 numberOfOutputs;
    }

    struct TransactionMetadata {
        uint256 vaultId;
        uint256 groupId;
        uint256 targetReceiptId; // the uniqueId of the proofReceipt of the other leg's transaction
        uint256 proofHash; // reserved to mitigate potential replay attacks
    }

    enum AuctionStateEnum {
        AUCTION_INACTIVE,
        AUCTION_BIDDING,
        AUCTION_OPENNING,
        AUCTION_DECLARE_WINNER,
        AUCTION_CONCLUDED,
        AUCTION_REVERTED // TODO:: add the logic
    }

    enum BidStateEnum {
        BID_INACTIVE,
        BID_SEALED,
        BID_OPENED_PUBLICLY,
        BID_OPENED_PRIVATELY
    }

    struct AuctionData {
        uint256 auctionId;
        AuctionStateEnum auctionState;
        uint256[] uniqueIdParams;
        uint256 vaultId;
        uint256 bidVaultId;
        uint256 groupId;
        uint256 bidGroupId;
        address assetAddress;
        uint256 auctioneerItemPublicKey;
        uint256 sellerFundPublicKey;
        uint256 auctionEndsAtblock;
        IZkDvp.ProofReceipt itemProof;
        uint256 numberOfSubmittedBids;
        uint256 numberOfOpenedBids;
        mapping(uint256 => AuctionBidData) bids;
    }

    struct AuctionBidData {
        BidStateEnum bidState;
        uint256 blindedBid;
        uint256 bidAmount;
        uint256 bidRandom;
        uint256 bidBlockNumber;
        uint256[2] bidCommitments;
        uint256[2] bidTreeNumbers;
        uint256[2] bidNullifiers;
        uint256 receivingPublicKey;
    }

    struct AuditorData {
        uint256 auditorOffchainId;
        uint256 auditorGroupId; // in case of having independent rings of auditors
        uint256[2] auditorPublicKey;
    }
    // TODO:: add other desired attributes

    error AuctionIdMismatch();
    error BlindedBidMismatch();
    error WinningBidOpeningMismatch();
    error NotWinningBidsCountMismatch();
    error BidStateMismatch();
    error RottenChallenge();
    error InvalidOpening();
    error InvalidChallenge();
    error InvalidPaymentMessage();
    error InvalidDeliveryMessage();
    error JoinSplitWithSameCommitments();
    error InvalidMerkleRoot();
    error InvalidNullifier();
    error InvalidNumberOfInputs();
    error InvalidNumberOfOutputs();
    error NotImplemented();
    error NonFungiblePaymentVault();
    error FungibleDeliveryVault();
    error AuctionAlreadyExists();
    error AuctionStateMismatch();

    error GroupMembershipMismatch();
    error GroupFungibilityMismatch();
    error GroupIdOutOfRange();
    error GroupPairAlreadyRegistered();
    error InvalidSwapGroupPair();
    error InvalidExchangeGroupPair();

    error InvalidPartialProofReceipt();

    error BrokerAlreadyRegistered();
    error InvalidStatementSize();
    error InvalidVaultId();
    error InvalidPublicKey();
    error InvalidSalt();
    error PrivateMintVerifierNotRegistered();
    error PublicSignalMismatch();

    error AuditorAlreadyRegistered(uint256, uint256);
    error AuditorNotRegistered(uint256);

    ///////////////////////////////////////////////
    //                  Events
    //////////////////////////////////////////////

    // Getting fired on a successful
    // VerifyOwnership functioncall
    event VerifyOwnershipReceipt(
        uint256 indexed challenge,
        uint256 indexed assetId,
        uint256 indexed tokenId,
        uint256 amountOrOne,
        address assetAddress,
        address senderAddress
    );

    event CoinLocked(
        uint256 indexed assetId,
        uint256 indexed treeNumber,
        uint256 indexed nullifier
    );

    event CoinUnlocked(
        uint256 indexed assetId,
        uint256 indexed treeNumber,
        uint256 indexed nullifier
    );

    event TokenAddedToGroup(
        uint256 indexed vaultId,
        uint256 indexed tokenUniqueId,
        uint256 indexed groupId
    );

    event VaultAddedToGroup(uint256 indexed vaultId, uint256 indexed groupId);

    // event AuctionInitialized(
    //     uint256 indexed auctionId,
    //     uint256 indexed vaultId,
    //     uint256 indexed bidVaultId,
    //     uint256 itemUniqueId,
    //     uint256 sellerFundCoinPublicKey
    // );
    // event AuctionConcluded(
    //     uint256 indexed auctionId,
    //     uint256 indexed winningBlindedBid,
    //     uint256 indexed winningBlockNumber,
    //     uint256 winningBid,
    //     uint256 winningRandom,
    //     uint256 bidCommitment1,
    //     uint256 bidCommitment2,
    //     uint256 itemCommitment
    // );

    // event AuctionBidSubmitted(
    //     uint256 indexed auctionId,
    //     uint256 blindedBid
    // );

    // event AuctionBidOpenedPublicly(
    //     uint256 indexed auctionId,
    //     uint256 indexed blindedBid,
    //     uint256 indexed bidAmount,
    //     uint256 bidRandom
    // );

    // event AuctionBidOpenedPrivately(
    //     uint256 indexed auctionId,
    //     uint256 indexed blindedBid
    // );

    event BrokerRegistered(
        uint256 indexed vaultId,
        uint256 indexed blindedBrokerPublicKey
    );

    event LegitBrokerReceipt(
        uint256 indexed beacon,
        uint256 indexed blindedBrokerPublicKey
    );

    event PendingProofAddedToVault(
        uint256 indexed vaultId,
        uint256 indexed groupId,
        uint256 indexed targetReceiptId,
        IZkDvp.ProofReceipt pendingProof
    );

    event Settled(uint256 indexed receiptId1, uint256 indexed receiptId2);

    event AuditorRegistered(
        uint256 indexed onchainId,
        uint256 indexed offchainId,
        uint256 indexed groupId,
        uint256[2] publicKey
    );

    event AuditorUnregistered(uint256 indexed onchainId);

    event PrivateMint(
        uint256 indexed vaultId,
        uint256 indexed commitment,
        uint256 indexed cipherText
    );
    ///////////////////////////////////////////////
    //              Getters
    //////////////////////////////////////////////

    function erc721Vault() external view returns (address);

    function erc20Vault() external view returns (address);

    function erc1155Vault() external view returns (address);

    function enygmaVault() external view returns (address);

    function vaultById(uint256) external view returns (address);

    function name() external view returns (string memory);

    function hashContractAddress() external view returns (address);

    function verifierContractAddress() external view returns (address);

    ///////////////////////////////////////////////
    //         Initialization  Functions
    //////////////////////////////////////////////

    function initializeDvp(address verifierAddress) external returns (bool);

    function registerVault(
        address vaultContractAddress,
        address assetContractAddress,
        uint256 vaultIdentifiersCount,
        uint256 treeDepth
    ) external returns (bool);

    function registerNewVerificationKey(
        VerifyingKey memory vk
    ) external returns (bool);

    function registerZkAuction(
        address zkAuctionContractAddress
    ) external returns (bool);

    function registerPrivateMintVerifier(
        address privateMintVerifierAddress
    ) external returns (bool);

    ///////////////////////////////////////////////
    //         Auditor functions
    //////////////////////////////////////////////

    function registerAuditor(
        uint256 auditorOffchainId,
        uint256 auditorGroupId,
        uint256[2] memory auditorPublicKey
    ) external returns (bool);

    function unregisterAuditor(
        uint256 auditorOnchainId
    ) external returns (bool);

    function isAuditorRegistered(
        uint256[2] memory publicKey
    ) external view returns (bool);

    ///////////////////////////////////////////////
    //         Asset Group functions
    //////////////////////////////////////////////

    function registerAssetGroup(
        address assetGroupContractAddress,
        string memory assetGroupName,
        bool isAssetGroupFungible,
        uint256 treeDepth
    ) external returns (bool);

    function addTokenToGroup(
        uint256 vaultId, // id of the vault that the asset belongs to
        uint256[] memory uniqueIdParams, // params that is needed for uniqueId generation
        uint256 groupId
    ) external returns (bool);

    function addVaultToGroup(
        uint256 vaultId, // id of the vault
        uint256 groupId
    ) external returns (bool);

    function isTokenMemberOf(
        uint256 vaultId,
        uint256[] memory uniqueIdParams,
        uint256 groupId
    ) external view returns (bool);

    function isVaultMemberOf(
        uint256 vaultId,
        uint256 groupId
    ) external view returns (bool);

    function isMemberOfFromProofReceipt(
        uint256 vaultId,
        ProofReceipt memory receipt,
        uint256 groupId
    ) external view returns (bool);
    // TODO:: implement it
    ///////////////////////////////////////////////
    //          Random Oracle functions
    //////////////////////////////////////////////
    function checkAndRegisterChallenge(
        uint256 challenge
    ) external returns (bool);

    ///////////////////////////////////////////////
    //          Broker functions
    //////////////////////////////////////////////
    function registerBroker(
        ProofReceipt memory brokerRegistrationProof
    ) external returns (bool);

    function verifyLegitBrokerReceipt(
        ProofReceipt memory receipt
    ) external returns (bool);
    ///////////////////////////////////////////////
    //          Swap/mix functions
    //////////////////////////////////////////////
    function swapOnGroupPair(
        ProofReceipt memory paymentProof,
        ProofReceipt memory deliveryProof,
        uint256 paymentVaultId,
        uint256 deliveryVaultId,
        uint256 paymentGroupId,
        uint256 deliveryGroupId
    ) external returns (bool);

    function exchangeOnGroupPair(
        ProofReceipt memory paymentProof,
        ProofReceipt memory deliveryProof,
        uint256 paymentVaultId,
        uint256 deliveryVaultId,
        uint256 paymentGroupId,
        uint256 deliveryGroupId
    ) external returns (bool);

    function swap(
        ProofReceipt memory paymentProof,
        ProofReceipt memory deliveryProof,
        uint256 paymentVaultId,
        uint256 deliveryVaultId
    ) external returns (bool);

    function exchange(
        ProofReceipt memory paymentProof1,
        ProofReceipt memory paymentProof2,
        uint256 paymentVaultId1,
        uint256 paymentVaultId2
    ) external returns (bool);

    function submitPartialSettlement(
        ProofReceipt memory receipt,
        uint256 vaultId,
        uint256 groupId
    ) external returns (bool);

    ///////////////////////////////////////////////
    //          Private Mint functions
    //////////////////////////////////////////////
    function privateMint(
        uint256 vaultId,
        uint256 commitment,
        ZkPrivateMintProof calldata proof
    ) external returns (bool);
}
