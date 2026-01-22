// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;

import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";

import {IEnygmaDvp} from "../../interfaces/IEnygmaDvp.sol";
import {
    IAbstractCoinVault
} from "../../interfaces/vaults/IAbstractCoinVault.sol";
import {IAssetGroup} from "../../interfaces/vaults/IAssetGroup.sol";
import {Merkle} from "./Merkle.sol";

contract AssetGroup is IAssetGroup, Merkle, AccessControl {
    uint256 constant GROUP_ID_OFFSET = 1000; // first 1000 ids have been reserved for vaults

    // Getting fired whenever a new nullifier is set
    // treeId: the ID of the asset and degisnated merkleTree
    // treeNumber: the sub-tree number.
    // nullifier: the nullifier value that has been registered.
    event MemberInserted(uint256 indexed vaultId, uint256 indexed tokenId);

    // Getting fired Whenever a new commitment
    // is generated and added to on-chain merkleTree
    event MemberRemoved(uint256 indexed vaultId, uint256 indexed tokenId);

    bytes32 public constant DEFAULT_DVP_ROLE =
        keccak256(abi.encodePacked("DvpRole"));

    bytes32 public constant DEFAULT_OWNER_ROLE =
        keccak256(abi.encodePacked("OwnerRole"));

    bytes32 public constant DEFAULT_VAULT_ROLE =
        keccak256(abi.encodePacked("VaultRole"));

    ///////////////////////////////////////////////
    //           Private attributes
    //////////////////////////////////////////////

    // name identifier for ZkDvp smart contract
    string internal _name;

    // the address of PoseidonWrapper smart contract
    address internal _hashContractAddress;

    // address of non-generic verifier that pack the proofs
    // and utilizes the generic Gorth16 verifier smart contract
    address internal _verifierContractAddress;

    address internal _zkDvpContractAddress;

    uint256 internal _assetGroupId;

    bool internal _isFungible;

    mapping(uint256 => bool) internal _vaultMembers;

    function getHashContractAddress() public view returns (address) {
        return _hashContractAddress;
    }
    function getVerifierContractAddress() public view returns (address) {
        return _verifierContractAddress;
    }

    function getRoot() public view returns (uint256 root) {
        return currentRoot();
    }

    constructor(address zkDvpAddress) Merkle() AccessControl() {
        // _hashContractAddress = hashContractAddress;
        _setupRole(DEFAULT_OWNER_ROLE, msg.sender);
        _setupRole(DEFAULT_DVP_ROLE, zkDvpAddress);

        _zkDvpContractAddress = zkDvpAddress;
    }

    function initializeAssetGroup(
        uint256 assetGroupId,
        string memory name,
        bool isFungible,
        uint256 treeDepth
    ) public onlyRole(DEFAULT_DVP_ROLE) returns (bool) {
        _assetGroupId = assetGroupId;
        _name = name;
        _isFungible = isFungible;

        _hashContractAddress = IEnygmaDvp(_zkDvpContractAddress)
            .hashContractAddress();
        _verifierContractAddress = IEnygmaDvp(_zkDvpContractAddress)
            .verifierContractAddress();

        initializeMerkle(
            treeDepth,
            GROUP_ID_OFFSET + _assetGroupId,
            _hashContractAddress
        );

        return true;
    }

    function isFungible() public view returns (bool) {
        return _isFungible;
    }

    function isVaultMember(uint256 vaultId) public view returns (bool) {
        return _vaultMembers[vaultId];
    }

    function isTokenMember(
        uint256 vaultId,
        uint256 assetUniqueId
    ) public view returns (bool) {
        return isValidNullifier(0, assetUniqueId);
    }

    function isMemberFromProofReceipt(
        uint256 vaultId,
        IEnygmaDvp.ProofReceipt memory receipt
    ) public view returns (bool) {
        bool isVaultAMember = _vaultMembers[vaultId];

        uint256 statementLength = receipt.statement.length;

        uint256 assetGroupMerkleRoot = receipt.statement[statementLength - 1];

        // TODO:: connect treeNumber
        bool isTokenAMember = isValidRoot(0, assetGroupMerkleRoot);

        return (isVaultAMember || isTokenAMember);
    }

    function insertTokenMember(
        // AccessControlled by ZkDvp
        uint256 vaultId,
        uint256 assetUniqueId
    ) public onlyRole(DEFAULT_DVP_ROLE) returns (bool) {
        // using zero for treeId
        // using nullifier to check whether the tokenId
        // has been inserted or not.
        if (!isValidNullifier(0, assetUniqueId)) {
            // if nullifier has not been registered
            uint256[] memory params = new uint256[](1);
            params[0] = assetUniqueId;
            insertLeaves(params);
            setNullifier(0, assetUniqueId);
        } else {
            revert TokenAlreadyInserted();
        }

        emit MemberInserted(_assetGroupId, assetUniqueId);
    }

    function insertVaultMember(
        uint256 vaultId
    ) public onlyRole(DEFAULT_DVP_ROLE) returns (bool) {
        if (_vaultMembers[vaultId]) {
            revert VaultAlreadyInserted();
        }

        _vaultMembers[vaultId] = true;

        return true;
    }
}
