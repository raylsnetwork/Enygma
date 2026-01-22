// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;

import {IEnygmaDvp} from "../IEnygmaDvp.sol";

interface IAssetGroup {
    error NotAMember();
    error InvalidMerkleRoot();
    error InvalidNumberOfInputs();
    error NotImplemented();
    error VaultAlreadyInserted();
    error TokenAlreadyInserted();

    function getHashContractAddress() external view returns (address);
    function getVerifierContractAddress() external view returns (address);
    function getRoot() external view returns (uint256 root);

    function initializeAssetGroup(
        // AccessControlled by the ZkDvp
        uint256 assetGroupId,
        string memory assetGroupName,
        bool isFungible,
        uint256 treeDepth
    ) external returns (bool);

    function isTokenMember(
        uint256 vaultId,
        uint256 assetUniqueId
    ) external view returns (bool);

    function isVaultMember(uint256 vaultId) external view returns (bool);

    function isMemberFromProofReceipt(
        uint256 vaultId,
        IEnygmaDvp.ProofReceipt memory receipt
    ) external view returns (bool);

    function insertTokenMember(
        // only accessible by the ZkDvp
        uint256 vaultId,
        uint256 assetUniqueId
    ) external returns (bool);

    function insertVaultMember(
        // only accessible by the ZkDvp
        uint256 vaultId
    ) external returns (bool);

    function isFungible() external view returns (bool);
}
