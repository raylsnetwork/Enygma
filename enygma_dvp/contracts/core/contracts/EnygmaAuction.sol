// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;
// pragma abicoder v2;

import {IERC721} from "@openzeppelin/contracts/token/ERC721/IERC721.sol";
import {IERC1155} from "@openzeppelin/contracts/token/ERC1155/IERC1155.sol";
import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";

import {IEnygmaAuction} from "../interfaces/IEnygmaAuction.sol";
import {IEnygmaDvp} from "../interfaces/IEnygmaDvp.sol";
import {IAbstractCoinVault} from "../interfaces/vaults/IAbstractCoinVault.sol";
import {IMerkle} from "../interfaces/vaults/IMerkle.sol";
import {IAssetGroup} from "../interfaces/vaults/IAssetGroup.sol";
import {IPoseidonWrapper} from "../interfaces/IPoseidonWrapper.sol";
import {IVerifier} from "../interfaces/IVerifier.sol";

contract EnygmaAuction is IEnygmaAuction, AccessControl {
    ///////////////////////////////////////////////
    //              Constants
    //////////////////////////////////////////////

    uint256 constant SNARK_SCALAR_FIELD =
        21888242871839275222246405745257275088548364400416034343698204186575808495617;

    // Hard-corded asset Ids
    uint256 constant VAULT_ID_ERC20 = 0;
    uint256 constant VAULT_ID_ERC721 = 1;
    uint256 constant VAULT_ID_ERC1155 = 2;
    uint256 constant VAULT_ID_ENYGMA = 3;
    uint256 constant MAX_NUMBER_OF_VAULTS = 1000;

    uint256 constant GROUP_ID_FUNGIBLES = 0;
    uint256 constant GROUP_ID_NON_FUNGIBLES = 1;

    uint256 constant MAX_NUMBER_OF_GROUPS = 1000;

    // Index of verification keys that has been
    // used directly in ZkdDvp
    uint256 public constant VK_ID_AUCTION_INIT = 6;
    uint256 public constant VK_ID_AUCTION_BID = 7;
    uint256 public constant VK_ID_AUCTION_NOT_WINNING_BID = 9;
    uint256 public constant VK_ID_AUCTION_PRIVATE_OPENING = 10;
    uint256 public constant VK_ID_BROKER_REGISTRATION = 11;
    uint256 public constant VK_ID_LEGIT_BROKER = 12;
    uint256 public constant VK_ID_AUCTION_INIT_AUDITOR = 21;
    uint256 public constant VK_ID_AUCTION_BID_AUDITOR = 22;

    bytes32 public constant DEFAULT_DVP_ROLE =
        keccak256(abi.encodePacked("DvpRole"));

    bytes32 public constant DEFAULT_OWNER_ROLE =
        keccak256(abi.encodePacked("ownerRole"));
    ///////////////////////////////////////////////
    //           Private attributes
    //////////////////////////////////////////////

    // name identifier for ZkDvp smart contract
    string private _name;

    // the address of PoseidonWrapper smart contract
    address private _hashContractAddress;
    // address of non-generic verifier that pack the proofs
    // and utilizes the generic Gorth16 verifier smart contract
    address private _verifierContractAddress;

    address private _zkDvpContractAddress;

    mapping(uint256 => AuctionData) private _auctions;
    mapping(uint256 => bool) private _rottenChallenges;

    mapping(uint256 => AuctioneerData) private _registeredAuctioneers;

    ///////////////////////////////////////////////
    //              Constructor
    //////////////////////////////////////////////

    // hashContractAddress: poseidon Wrapper contract address
    // genericVerifierContractAddress: Groth16 generic verifier address.
    // TODO:: some form of verification is needed
    constructor(address zkDvpContractAddress) AccessControl() {
        _name = "ZkAuction smart contract";
        _zkDvpContractAddress = zkDvpContractAddress;
        _setupRole(DEFAULT_OWNER_ROLE, msg.sender);
        _setupRole(DEFAULT_DVP_ROLE, zkDvpContractAddress);
    }

    ///////////////////////////////////////////////
    //              Public Getters
    //////////////////////////////////////////////
    function name() public view returns (string memory) {
        return _name;
    }

    function zkDvpContractAddress() public view returns (address) {
        return _zkDvpContractAddress;
    }

    function hashContractAddress() public view returns (address) {
        return _hashContractAddress;
    }

    function verifierContractAddress() public view returns (address) {
        return _verifierContractAddress;
    }

    ///////////////////////////////////////////////
    //       initialization functions
    //////////////////////////////////////////////
    function initializeZkAuction(
        address hashContractAddress,
        address verifierContractAddress
    ) public onlyRole(DEFAULT_DVP_ROLE) returns (bool) {
        // registering the verifier smart contract address
        _hashContractAddress = hashContractAddress;
        _verifierContractAddress = verifierContractAddress;
        _zkDvpContractAddress = msg.sender;

        return true;
    }

    // TODO:: add accessControl
    function registerAuctioneer(
        uint256 auctioneerOffchainId,
        uint256 auctioneerGroupId,
        uint256[2] memory auctioneerPublicKey
    ) public returns (bool) {
        // TODO:: check auctioneerPublicKey be on curve.

        uint256 auctioneerOnchainId = uint256(
            keccak256(
                abi.encodePacked(auctioneerPublicKey[0], auctioneerPublicKey[1])
            )
        );

        // Auditor has not been registered
        if (
            _registeredAuctioneers[auctioneerOnchainId].auctioneerPublicKey[
                0
            ] == 0
        ) {
            AuctioneerData memory newAuctioneer;
            newAuctioneer.auctioneerOffchainId = auctioneerOffchainId;
            newAuctioneer.auctioneerGroupId = auctioneerGroupId;
            newAuctioneer.auctioneerPublicKey = auctioneerPublicKey;
            _registeredAuctioneers[auctioneerOnchainId] = newAuctioneer;

            emit AuctioneerRegistered(
                auctioneerOnchainId,
                auctioneerOffchainId,
                auctioneerGroupId,
                auctioneerPublicKey
            );
        } else {
            revert AuctioneerAlreadyRegistered(
                auctioneerOffchainId,
                auctioneerGroupId
            );
        }

        return true;
    }

    // function unregisterAuctioneer(uint256 auditorOnchainId
    // ) public onlyRole(DEFAULT_OWNER_ROLE) returns(bool){
    //     if(_registeredAuditors[auditorOnchainId].auditorPublicKey[0] == 0){
    //         revert AuditorNotRegistered(auditorOnchainId);
    //     }
    //     else{
    //         delete _registeredAuditors[auditorOnchainId];
    //         emit AuditorUnregistered(auditorOnchainId);
    //     }

    //     return true;

    // }

    ///////////////////////////////////////////////
    //          Auction functions
    //////////////////////////////////////////////

    // TODO:: this can later check the freshness of the challenge
    // with Random Oracle
    function checkAndRegisterChallenge(
        uint256 challenge_
    ) public returns (bool) {
        bool isRotten = _rottenChallenges[challenge_];

        if (isRotten) {
            revert RottenChallenge();
        }

        // TODO:: Require to check that challenge != valid address

        _rottenChallenges[challenge_] = true;

        return true;
    }

    function newAuction(
        uint256[] memory uniqueIdParams,
        uint256 itemVaultId,
        uint256 bidVaultId,
        uint256 itemGroupId,
        uint256 bidGroupId,
        uint256 sellerFundCoinPublicKey,
        IEnygmaDvp.ProofReceipt memory auctionInitReceipt
    ) public returns (bool) {
        // TODO:: check proof conditions
        // if itemVaultId == ERC1155 then the last statement can not be zero
        // TODO:: check the size of the statement

        uint256 auctionId = auctionInitReceipt.statement[1];

        if (
            _auctions[auctionId].auctionState !=
            AuctionStateEnum.AUCTION_INACTIVE
        ) {
            revert AuctionAlreadyExists();
        }

        uint256 nullifier = auctionInitReceipt.statement[5];
        uint256 treeNumber = auctionInitReceipt.statement[3];
        IAbstractCoinVault itemVault = IAbstractCoinVault(
            IEnygmaDvp(_zkDvpContractAddress).vaultById(itemVaultId)
        );

        address assetAddress = itemVault.getAssetContractAddress();

        uint256 itemUniqueId = itemVault.generateUniqueId(uniqueIdParams);

        IVerifier(_verifierContractAddress).verifyProof(
            VK_ID_AUCTION_INIT_AUDITOR,
            auctionInitReceipt.proof,
            auctionInitReceipt.statement
        );

        itemVault.lockCoin(treeNumber, nullifier);

        _checkItemGroupMembership(uniqueIdParams, itemVaultId, itemGroupId);

        _auctions[auctionId].bidVaultId = bidVaultId;
        _auctions[auctionId].vaultId = itemVaultId;

        _auctions[auctionId].bidGroupId = bidGroupId;
        _auctions[auctionId].groupId = itemGroupId;

        _auctions[auctionId].uniqueIdParams = uniqueIdParams;
        _auctions[auctionId].assetAddress = assetAddress;
        _auctions[auctionId].sellerFundPublicKey = sellerFundCoinPublicKey;
        _auctions[auctionId].auctionState = AuctionStateEnum.AUCTION_BIDDING;
        _auctions[auctionId].itemProof.statement = auctionInitReceipt.statement;

        emit AuctionInitialized(
            auctionId,
            itemVaultId,
            bidVaultId,
            itemUniqueId,
            sellerFundCoinPublicKey
        );

        return true;
    }

    function _checkItemGroupMembership(
        uint256[] memory uniqueIdParams,
        uint256 itemVaultId,
        uint256 itemGroupId
    ) internal returns (bool) {
        // checking assetGroups
        if (
            !IEnygmaDvp(_zkDvpContractAddress).isVaultMemberOf(
                itemVaultId,
                itemGroupId
            )
        ) {
            if (
                !IEnygmaDvp(_zkDvpContractAddress).isTokenMemberOf(
                    itemVaultId,
                    uniqueIdParams,
                    itemGroupId
                )
            ) {
                revert GroupMembershipMismatch();
            }
        }

        return true;
    }

    // Called by bidder through RELAYER
    function submitBid(
        IEnygmaDvp.ProofReceipt memory bidReceipt,
        uint256 receivingPublicKey
    ) public returns (bool) {
        // order of receipt.statement
        // signal input st_beacon;
        // signal input st_auctionId;
        // signal input st_blindedBid;
        // signal input st_vaultId;
        // signal input st_treeNumbers[nInputs];
        // signal input st_merkleRoots[nInputs];
        // signal input st_nullifiers[nInputs];
        // signal input st_commitmentsOut[nInputs];
        // signal input st_assetGroup_merkleRoot;

        uint256 auctionId = bidReceipt.statement[1];
        uint256 blindedBid = bidReceipt.statement[2];
        uint256 bidVaultIdFromStatement = bidReceipt.statement[3];
        // uint256 treeNumbersIndex = 3 , 4;

        // bid should not exist
        if (
            _auctions[auctionId].bids[blindedBid].bidState !=
            BidStateEnum.BID_INACTIVE
        ) {
            revert BidStateMismatch();
        }
        // auction must be in BIDDING state
        if (
            _auctions[auctionId].auctionState !=
            AuctionStateEnum.AUCTION_BIDDING
        ) {
            revert AuctionStateMismatch();
        }
        uint256 vaultId = _auctions[auctionId].vaultId;
        uint256 bidVaultId = _auctions[auctionId].bidVaultId;

        // verifying the proof
        IVerifier(_verifierContractAddress).verifyProof(
            VK_ID_AUCTION_BID_AUDITOR,
            bidReceipt.proof,
            bidReceipt.statement
        );

        IAbstractCoinVault bidVault = IAbstractCoinVault(
            IEnygmaDvp(_zkDvpContractAddress).vaultById(bidVaultId)
        );
        for (uint i = 0; i < 2; i++) {
            if (bidReceipt.statement[6 + i] != 0) {
                // checking roots
                if (
                    !bidVault.verifyRoot(
                        bidReceipt.statement[4 + i],
                        bidReceipt.statement[6 + i]
                    )
                ) {
                    revert InvalidBidMerkleRoot();
                }
            }
        }

        // locking the input coin's nullifiers
        bidVault.lockCoin(bidReceipt.statement[4], bidReceipt.statement[8]);
        bidVault.lockCoin(bidReceipt.statement[5], bidReceipt.statement[9]);

        // checking assetGroups
        if (
            !IEnygmaDvp(_zkDvpContractAddress).isVaultMemberOf(
                bidVaultIdFromStatement,
                _auctions[auctionId].bidGroupId
            )
        ) {
            if (
                !IEnygmaDvp(_zkDvpContractAddress).isMemberOfFromProofReceipt(
                    bidVaultIdFromStatement,
                    bidReceipt,
                    _auctions[auctionId].bidGroupId
                )
            ) {
                revert GroupMembershipMismatch();
            }
        }

        _auctions[auctionId].bids[blindedBid].blindedBid = blindedBid;
        _auctions[auctionId].bids[blindedBid].bidState = BidStateEnum
            .BID_SEALED;
        _auctions[auctionId].bids[blindedBid].bidCommitments[0] = bidReceipt
            .statement[10];
        _auctions[auctionId].bids[blindedBid].bidCommitments[1] = bidReceipt
            .statement[11];
        _auctions[auctionId].bids[blindedBid].bidTreeNumbers[0] = bidReceipt
            .statement[4];
        _auctions[auctionId].bids[blindedBid].bidTreeNumbers[1] = bidReceipt
            .statement[5];
        _auctions[auctionId].bids[blindedBid].bidNullifiers[0] = bidReceipt
            .statement[8];
        _auctions[auctionId].bids[blindedBid].bidNullifiers[1] = bidReceipt
            .statement[9];
        _auctions[auctionId]
            .bids[blindedBid]
            .receivingPublicKey = receivingPublicKey;

        _auctions[auctionId].numberOfSubmittedBids++;

        emit AuctionBidSubmitted(auctionId, blindedBid);

        return true;
    }

    function getBid(
        uint256 auctionId,
        uint256 blindedBid
    ) public view returns (AuctionBidData memory) {
        AuctionBidData memory bidData;
        bidData.bidState = _auctions[auctionId].bids[blindedBid].bidState;
        bidData.bidAmount = _auctions[auctionId].bids[blindedBid].bidAmount;
        bidData.bidRandom = _auctions[auctionId].bids[blindedBid].bidRandom;
        bidData.blindedBid = _auctions[auctionId].bids[blindedBid].blindedBid;

        return bidData;
    }

    // Called by bidder through RELAYER
    function publicOpeningReceipt(
        uint256 auctionId,
        uint256 bidAmount,
        uint256 bidRandom
    ) public returns (bool) {
        uint256 blindedBid = IPoseidonWrapper(_hashContractAddress).poseidon(
            [bidAmount, bidRandom]
        );

        if (
            _auctions[auctionId].bids[blindedBid].bidState !=
            BidStateEnum.BID_SEALED
        ) {
            revert BidStateMismatch();
        }

        _auctions[auctionId].bids[blindedBid].bidAmount = bidAmount;
        _auctions[auctionId].bids[blindedBid].bidRandom = bidRandom;
        _auctions[auctionId].bids[blindedBid].bidState = BidStateEnum
            .BID_OPENED_PUBLICLY;

        _auctions[auctionId].numberOfOpenedBids++;
        // emit the event
        emit AuctionBidOpenedPublicly(
            auctionId,
            blindedBid,
            bidAmount,
            bidRandom
        );

        return true;
    }

    // only callable by zkDvp owner / auctioneer
    // TODO:: needs accessControl
    function privateOpeningReceipt(
        IEnygmaDvp.ProofReceipt memory openingReceipt
    ) public returns (bool) {
        //     uint256 auctionId 0;
        //     uint256 blindedBid 1;
        uint256 auctionId = openingReceipt.statement[0];
        uint256 blindedBid = openingReceipt.statement[1];

        // TODO:: check auction state to be in OPENING

        // check the bid's state is in sealed
        if (
            _auctions[auctionId].bids[blindedBid].bidState !=
            BidStateEnum.BID_SEALED
        ) {
            revert BidStateMismatch();
        }

        // check the proof

        // verifies the validity of the proof
        IVerifier(_verifierContractAddress).verifyProof(
            VK_ID_AUCTION_PRIVATE_OPENING,
            openingReceipt.proof,
            openingReceipt.statement
        );

        // update the bid data
        _auctions[auctionId].bids[blindedBid].bidState = BidStateEnum
            .BID_OPENED_PRIVATELY;

        _auctions[auctionId].numberOfOpenedBids++;

        // emit the event
        emit AuctionBidOpenedPrivately(auctionId, blindedBid);

        return true;
    }

    // Called by auctioneer (ZkDvp owner)
    function declareWinner(
        uint256 auctionId,
        uint256 winningBid,
        uint256 winningRandom,
        IEnygmaDvp.ProofReceipt[] memory notWinningBidProofs
    ) public returns (bool) {
        // VERIFICATION

        // TODO:: check auctions[auctionId].state
        // TODO:: add time constraints
        // require(_auctions[auctionId].auctionState == IZkDvp.AuctionStateEnum.AUCTION_DECLARE_WINNER, "ZkDvp: Invalid auction state");

        // compute winningBlindedBid
        uint256 winningBlindedBid = IPoseidonWrapper(_hashContractAddress)
            .poseidon([winningBid, winningRandom]);

        BidStateEnum winningBidState = _auctions[auctionId]
            .bids[winningBlindedBid]
            .bidState;
        // check winningBlindedBid is in auctions.blindedBids
        // if(winningBidState != BidStateEnum.BID_OPENED_PUBLICLY &&
        //             winningBidState != BidStateEnum.BID_OPENED_PRIVATELY){
        //     revert WinningBidOpeningMismatch();
        // }

        _verifyNotWinningProofs(
            auctionId,
            winningBid,
            winningRandom,
            notWinningBidProofs
        );

        // SETTLEMENT
        AuctionBidData memory bidData = _auctions[auctionId].bids[
            winningBlindedBid
        ];

        _unlockAndNullifyWinnerBidCoins(auctionId, bidData);

        _unlockAndNullifyAuctionItemCoin(auctionId);

        uint256[] memory outCommitments = new uint256[](3);
        uint256[] memory bidCommitments = _registerBidDataCommitments(
            auctionId,
            bidData
        );

        outCommitments[0] = bidCommitments[0];
        outCommitments[1] = bidCommitments[1];
        outCommitments[2] = _registerItemCommitment(
            auctionId,
            bidData.receivingPublicKey
        );

        // unlocking loser coins:
        _unlockLosersBids(auctionId, winningBlindedBid, notWinningBidProofs);

        _auctions[auctionId].auctionState = AuctionStateEnum.AUCTION_CONCLUDED;

        emit AuctionConcluded(
            auctionId,
            winningBlindedBid,
            0, // TODO:: fix it
            winningBid,
            winningRandom,
            outCommitments
        );
        return true;
    }

    // TODO:: needs accessControl
    function _unlockAuctionBidCoins(
        uint256 auctionId,
        uint256 blindedBid
    ) internal returns (bool) {
        uint256 vaultId = _auctions[auctionId].bidVaultId;
        IAbstractCoinVault vault = IAbstractCoinVault(
            IEnygmaDvp(_zkDvpContractAddress).vaultById(vaultId)
        );

        uint256 treeNumber1 = _auctions[auctionId]
            .bids[blindedBid]
            .bidTreeNumbers[0];
        uint256 nullifier1 = _auctions[auctionId]
            .bids[blindedBid]
            .bidNullifiers[0];
        vault.unlockCoin(treeNumber1, nullifier1);

        uint256 treeNumber2 = _auctions[auctionId]
            .bids[blindedBid]
            .bidTreeNumbers[1];
        uint256 nullifier2 = _auctions[auctionId]
            .bids[blindedBid]
            .bidNullifiers[1];
        vault.unlockCoin(treeNumber2, nullifier2);
        return true;
    }

    function _registerItemCommitment(
        uint256 auctionId,
        uint256 receivingPublicKey
    ) internal returns (uint256) {
        uint256 itemVaultId = _auctions[auctionId].vaultId;
        IAbstractCoinVault itemVault = IAbstractCoinVault(
            IEnygmaDvp(_zkDvpContractAddress).vaultById(itemVaultId)
        );

        // Generating uniqueId for item coin
        uint256[] memory params = _auctions[auctionId].uniqueIdParams;
        uint256 itemUniqueId = itemVault.generateUniqueId(params);

        // generating commitment based on the uniqueId and the publicKey
        uint256[] memory params2 = new uint256[](1);

        params2[0] = _getCommitment(itemUniqueId, receivingPublicKey);

        // registering item coin for the winner.
        itemVault.registerCoins(params2);

        return params2[0];
    }

    function _unlockLosersBids(
        uint256 auctionId,
        uint256 winningBlindedBid,
        IEnygmaDvp.ProofReceipt[] memory notWinningBidProofs
    ) internal returns (bool) {
        for (uint i = 0; i < notWinningBidProofs.length; i++) {
            // compute computedBlindedBid[i] = winningBlindedBid - blindedDifferenceBid;
            uint256 computedBlindedBid = _recomputeLoserBid(
                winningBlindedBid,
                notWinningBidProofs[i].statement[1]
            );

            _unlockAuctionBidCoins(auctionId, computedBlindedBid);
        }
    }

    function _unlockAndNullifyWinnerBidCoins(
        uint256 auctionId,
        AuctionBidData memory bidData
    ) internal returns (bool) {
        uint256 bidVaultId = _auctions[auctionId].bidVaultId;

        IAbstractCoinVault bidVault = IAbstractCoinVault(
            IEnygmaDvp(_zkDvpContractAddress).vaultById(bidVaultId)
        );

        for (uint i = 0; i < 2; i++) {
            bidVault.unlockCoin(
                bidData.bidTreeNumbers[i],
                bidData.bidNullifiers[i]
            );
            bidVault.nullifyCoin(
                bidData.bidTreeNumbers[i],
                bidData.bidNullifiers[i]
            );
        }
    }

    function _registerBidDataCommitments(
        uint256 auctionId,
        AuctionBidData memory bidData
    ) internal returns (uint256[] memory) {
        uint256 bidVaultId = _auctions[auctionId].bidVaultId;
        IAbstractCoinVault bidVault = IAbstractCoinVault(
            IEnygmaDvp(_zkDvpContractAddress).vaultById(bidVaultId)
        );

        // registering new bid coins
        uint256[] memory commitments = new uint256[](2);
        commitments[0] = bidData.bidCommitments[0];
        commitments[1] = bidData.bidCommitments[1];
        bidVault.registerCoins(commitments);

        return commitments;
    }

    function _verifyNotWinningProofs(
        uint256 auctionId,
        uint256 winningBid,
        uint256 winningRandom,
        IEnygmaDvp.ProofReceipt[] memory notWinningBidProofs
    ) internal returns (bool) {
        uint256 winningBlindedBid = IPoseidonWrapper(_hashContractAddress)
            .poseidon([winningBid, winningRandom]);

        BidStateEnum winningBidState = _auctions[auctionId]
            .bids[winningBlindedBid]
            .bidState;
        // check winningBlindedBid is in auctions.blindedBids
        // if(winningBidState != BidStateEnum.BID_OPENED_PUBLICLY &&
        //             winningBidState != BidStateEnum.BID_OPENED_PRIVATELY){
        //     revert WinningBidOpeningMismatch();
        // }

        // uint256 winningBlockNumber = _auctions[auctionId].bids[winningBlindedBid].bidBlockNumber;
        // The number of not winning proofs must be
        // the number of openedBids - 1
        // if(_auctions[auctionId].numberOfOpenedBids != notWinningBidProofs.length + 1){
        //     revert NotWinningBidsCountMismatch();
        // }

        // uint256 auctionId;
        // uint256 blindedBidDifference;
        // uint256 bidBlockNumber;
        // uint256 winningBidBlockNumber;
        for (uint i = 0; i < notWinningBidProofs.length; i++) {
            if (notWinningBidProofs[i].statement[0] != auctionId) {
                revert AuctionIdMismatch();
            }

            // compute computedBlindedBid[i] = winningBlindedBid - blindedDifferenceBid;
            uint256 computedBlindedBid = (SNARK_SCALAR_FIELD +
                winningBlindedBid -
                notWinningBidProofs[i].statement[1]) % SNARK_SCALAR_FIELD;
            BidStateEnum computedBidState = _auctions[auctionId]
                .bids[computedBlindedBid]
                .bidState;

            // verify computedBlindedBid[i] is in auction[auctionId].blindedBids;
            if (
                computedBidState != BidStateEnum.BID_OPENED_PUBLICLY &&
                computedBidState != BidStateEnum.BID_OPENED_PRIVATELY &&
                computedBidState != BidStateEnum.BID_SEALED
            ) {
                revert BlindedBidMismatch();
            }

            // verifies the validity of the notWinningProof\
            IVerifier(_verifierContractAddress).verifyProof(
                VK_ID_AUCTION_NOT_WINNING_BID,
                notWinningBidProofs[i].proof,
                notWinningBidProofs[i].statement
            );
        }

        return true;
    }

    function _unlockAndNullifyAuctionItemCoin(
        uint256 auctionId
    ) internal returns (bool) {
        uint256 itemVaultId = _auctions[auctionId].vaultId;
        IAbstractCoinVault itemVault = IAbstractCoinVault(
            IEnygmaDvp(_zkDvpContractAddress).vaultById(itemVaultId)
        );

        itemVault.unlockCoin(
            _auctions[auctionId].itemProof.statement[3],
            _auctions[auctionId].itemProof.statement[5]
        );
        itemVault.nullifyCoin(
            _auctions[auctionId].itemProof.statement[3],
            _auctions[auctionId].itemProof.statement[5]
        );

        return true;
    }

    function _getCommitment(
        uint256 uniqueId,
        uint256 publicKey
    ) internal returns (uint256) {
        return
            IPoseidonWrapper(_hashContractAddress).poseidon(
                [uniqueId, publicKey]
            );
    }

    function _recomputeLoserBid(
        uint winningBlindedBid,
        uint bidDiff
    ) internal returns (uint256) {
        // TODO:: needs audit
        return
            (SNARK_SCALAR_FIELD + winningBlindedBid - bidDiff) %
            SNARK_SCALAR_FIELD;
    }
}
