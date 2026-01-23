// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;

import {IEnygmaDvp} from "./IEnygmaDvp.sol";
interface IEnygmaAuction {
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
        IEnygmaDvp.ProofReceipt itemProof;
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

    struct AuctioneerData {
        uint256 auctioneerOffchainId;
        uint256 auctioneerGroupId; // in case of having independent rings of auctioneers
        uint256[2] auctioneerPublicKey;
    }
    // TODO:: add other desired attributes

    error AuctionAlreadyExists();
    error AuctionStateMismatch();
    error AuctionIdMismatch();
    error BlindedBidMismatch();
    error WinningBidOpeningMismatch();
    error NotWinningBidsCountMismatch();
    error BidStateMismatch();
    error RottenChallenge();
    error InvalidOpening();
    error InvalidChallenge();

    error InvalidNumberOfInputs();
    error InvalidNumberOfOutputs();

    error NotImplemented();

    error NonFungiblePaymentVault();
    error FungibleDeliveryVault();

    error InvalidStatementSize();
    error GroupMembershipMismatch();

    error InvalidBidMerkleRoot();

    error AuctioneerAlreadyRegistered(uint256, uint256);
    error AuctioneerNotRegistered(uint256);

    ///////////////////////////////////////////////
    //                  Events
    //////////////////////////////////////////////

    event AuctionInitialized(
        uint256 indexed auctionId,
        uint256 indexed vaultId,
        uint256 indexed bidVaultId,
        uint256 itemUniqueId,
        uint256 sellerFundCoinPublicKey
    );
    event AuctionConcluded(
        uint256 indexed auctionId,
        uint256 indexed winningBlindedBid,
        uint256 indexed winningBlockNumber,
        uint256 winningBid,
        uint256 winningRandom,
        uint256[] outCommitments
    );

    event AuctionBidSubmitted(uint256 indexed auctionId, uint256 blindedBid);

    event AuctionBidOpenedPublicly(
        uint256 indexed auctionId,
        uint256 indexed blindedBid,
        uint256 indexed bidAmount,
        uint256 bidRandom
    );

    event AuctionBidOpenedPrivately(
        uint256 indexed auctionId,
        uint256 indexed blindedBid
    );

    event AuctioneerRegistered(
        uint256 indexed onchainId,
        uint256 indexed offchainId,
        uint256 indexed groupId,
        uint256[2] publicKey
    );

    event AuctioneerUnregistered(uint256 indexed onchainId);

    ///////////////////////////////////////////////
    //              Getters
    //////////////////////////////////////////////

    function name() external view returns (string memory);

    function hashContractAddress() external view returns (address);

    function verifierContractAddress() external view returns (address);

    function enygmaDvpContractAddress() external view returns (address);

    ///////////////////////////////////////////////
    //         Initialization  Functions
    //////////////////////////////////////////////

    function initializeEnygmaAuction(
        address verifierAddress,
        address hashContractAddress
    ) external returns (bool);

    function registerAuctioneer(
        uint256 auctioneerOffchainId,
        uint256 auctioneerGroupId,
        uint256[2] memory auctioneerPublicKey
    ) external returns (bool);

    ///////////////////////////////////////////////
    //          Random Oracle functions
    //////////////////////////////////////////////
    function checkAndRegisterChallenge(
        uint256 challenge
    ) external returns (bool);

    ///////////////////////////////////////////////
    //           Auction Functions
    //////////////////////////////////////////////

    function newAuction(
        uint256[] memory uniqueIdParams,
        uint256 itemVaultId,
        uint256 bidVaultId,
        uint256 itemGroupId,
        uint256 bidGroupId,
        uint256 sellerFundCoinPublicKey,
        IEnygmaDvp.ProofReceipt memory auctionInitReceipt
    ) external returns (bool);

    // Called by bidder through RELAYER
    function submitBid(
        IEnygmaDvp.ProofReceipt memory bidProof,
        uint256 receivingPublicKey
    ) external returns (bool);

    // Called by bidder through RELAYER
    function publicOpeningReceipt(
        uint256 auctionId,
        uint256 bidAmount,
        uint256 bidRandom
    ) external returns (bool);

    function privateOpeningReceipt(
        IEnygmaDvp.ProofReceipt memory openingProof
    ) external returns (bool);

    // Called by auctioneer (EnygmaDvp owner)
    function declareWinner(
        uint256 auctionId,
        uint256 winningBid,
        uint256 winningRandom,
        IEnygmaDvp.ProofReceipt[] memory notWinningBidProofs
    ) external returns (bool);

    function getBid(
        uint256 auctionId,
        uint256 blindedBid
    ) external view returns (AuctionBidData memory);
}
