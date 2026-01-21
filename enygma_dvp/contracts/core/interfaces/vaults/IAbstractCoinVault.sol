// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;

import {IZkDvp} from "../IZkDvp.sol";

interface IAbstractCoinVault {


    // Getting fired whenever a new nullifier is set
    // treeId: the ID of the asset and degisnated merkleTree
    // treeNumber: the sub-tree number.
    // nullifier: the nullifier value that has been registered. 
    event Nullifier(
        uint256 indexed vaultId,
        uint256 indexed treeNumber,
        uint256 indexed nullifier
    );

    // Getting fired Whenever a new commitment
    // is generated and added to on-chain merkleTree
    event Commitment(
        uint256 indexed vaultId, 
        uint256 indexed commitment
    );

    event CoinLocked(
        uint256 indexed vaultId,
        uint256 indexed treeNumber,
        uint256 indexed nullifier
    );

    event CoinUnlocked(
        uint256 indexed vaultId,
        uint256 indexed treeNumber,
        uint256 indexed nullifier
    );

    event OwnershipVerificationReceipt(
        uint256 indexed challenge,
        uint256 indexed vaultId,
        uint256 indexed tokenId,
        uint256 amount
    );

    event PendingProofAdded(
        IZkDvp.ProofReceipt pendingProof
    );

    error RottenChallenge();
    error InvalidOpening();
    error InvalidErc721Transfer();
    error InvalidErc20Transfer();
    error InvalidErc1155Transfer();
    error InvalidErc1155BatchTransfer();
    error JoinSplitWithSameCommitments();
    error InvalidMerkleRoot();
    error InvalidNullifier();
    error InvalidNumberOfInputs();
    error InvalidNumberOfOutputs();
    error WrongNumberOfIdentifiers();
    error NotImplemented();
    error FungibilityMismatch();

    error ProofReceiptAlreadyAdded();
    error CantSpendLockedCoin();
    error CoinAlreadyUnlocked();
    error InvalidStatmentSize();

    error InvalidAuditorPublicKey();

    function getVaultId() external view returns (uint256);
    function getAssetContractAddress() external view returns (address);
    function getHashContractAddress() external view returns (address);
    function getVerifierContractAddress() external view returns (address);
    function getNumberOfAssetIdentifiers() external view returns (uint256);
    function getRoot() external view returns (uint256 root);
    function verifyRoot(uint256 treeNumber, uint256 root) external view returns(bool);

    function initializeVault(
        // string memory assetSymbol,
        // string memory assetStandard,
        uint256 vaultId, 
        uint256 numberOfAssetIdentifiers, 
        address assetContractAddress, 
        uint256 treeDepth,
        address hashContractAddress,
        address verifierContractAddress,
        address zkAuctionContractAddress
    ) external returns (bool);

    function nullifyFromReceipt(
        IZkDvp.ProofReceipt memory receipt
    ) external returns (bool); 

    function registerCoins(
        uint256[] memory commitments
    ) external returns (bool);

    function lockCoin(
        uint256 treeNumber,
        uint256 nullifier
    ) external returns (bool);

    function unlockCoin(
        uint256 treeNumber,
        uint256 nullifier
    ) external returns (bool);

    function nullifyCoin(
        uint256 treeNumber,
        uint256 nullifier
    )external returns (bool); 

    function insertCommitmentsFromReceipt(
        IZkDvp.ProofReceipt memory receipt
    ) external returns (bool); 

    function unlockFromReceipt(
        IZkDvp.ProofReceipt memory receipt
    ) external returns (bool);
    /////////////////////////////////
    // Virtual functions
    /////////////////////////////////

    function deposit(
        uint256[] memory depositParams
    ) external returns (bool); 

    function transfer(
        IZkDvp.ProofReceipt memory receipt
    ) external returns (bool); 

    function withdraw(
        uint256[] memory withdrawParams,
        address recipient,
        IZkDvp.ProofReceipt memory receipt
    ) external returns (bool);

    function generateUniqueId(
        uint256[] memory assetIdentifiers
    ) external view returns(uint256);
   
    function checkReceiptConditions(
        IZkDvp.ProofReceipt memory receipt
    ) external view returns (bool); 

    function verifyOwnership(
        uint256[] memory params,
        IZkDvp.ProofReceipt memory receipt
    ) external returns (bool);

    function addPendingProofReceipt(
        IZkDvp.ProofReceipt memory receipt
    ) external returns (bool);


    function getPendingProofReceipt(
        uint256 receiptUniqueId
    ) external returns (IZkDvp.ProofReceipt memory receipt);


    function checkRegisterBrokerProofConditions(
        IZkDvp.ProofReceipt memory receipt
    ) external returns (bool);

}
