// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;

import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";

import {IEnygmaDvp} from "../../interfaces/IEnygmaDvp.sol";
import {
    IAbstractCoinVault
} from "../../interfaces/vaults/IAbstractCoinVault.sol";
import {Merkle} from "./Merkle.sol";

abstract contract AbstractCoinVault is
    IAbstractCoinVault,
    Merkle,
    AccessControl
{
    bytes32 public constant DEFAULT_DVP_ROLE =
        keccak256(abi.encodePacked("DvpRole"));

    bytes32 public constant DEFAULT_AUCTION_ROLE =
        keccak256(abi.encodePacked("AuctionRole"));

    bytes32 public constant DEFAULT_OWNER_ROLE =
        keccak256(abi.encodePacked("ownerRole"));
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

    address internal _assetContractAddress;

    address internal _zkAuctionContractAddress;

    uint256 internal _vaultId;

    uint256 internal _numberOfIdentifiers;

    // utxo's uniqueId to proofReceipt
    mapping(uint256 => IEnygmaDvp.ProofReceipt) _pendingProofReceipts;

    function getVaultId() public view returns (uint256) {
        return _vaultId;
    }
    function getAssetContractAddress() public view returns (address) {
        return _assetContractAddress;
    }
    function getHashContractAddress() public view returns (address) {
        return _hashContractAddress;
    }
    function getVerifierContractAddress() public view returns (address) {
        return _verifierContractAddress;
    }

    function getNumberOfAssetIdentifiers() public view returns (uint256) {
        return _numberOfIdentifiers;
    }

    function getRoot() public view returns (uint256 root) {
        return currentRoot();
    }
    function verifyRoot(
        uint256 treeNumber,
        uint256 root
    ) public view returns (bool) {
        return isValidRoot(treeNumber, root);
    }

    constructor(address zkDvpAddress) Merkle() AccessControl() {
        // _hashContractAddress = hashContractAddress;
        _setupRole(DEFAULT_OWNER_ROLE, msg.sender);
        _setupRole(DEFAULT_DVP_ROLE, zkDvpAddress);
    }

    function initializeVault(
        uint256 vaultId,
        uint256 numberOfAssetIdentifiers,
        address assetContractAddress,
        uint256 treeDepth,
        address hashContractAddress,
        address verifierContractAddress,
        address zkAuctionContractAddress
    ) public onlyRole(DEFAULT_DVP_ROLE) returns (bool) {
        _vaultId = vaultId;
        _zkDvpContractAddress = msg.sender;
        _hashContractAddress = hashContractAddress;
        _verifierContractAddress = verifierContractAddress;
        _assetContractAddress = assetContractAddress;
        _zkAuctionContractAddress = zkAuctionContractAddress;
        _numberOfIdentifiers = numberOfAssetIdentifiers;

        // TODO:: give AUCTION_ROLE to zkAuction
        // and add access in the function to AUCTION_ROLE
        _setupRole(DEFAULT_DVP_ROLE, _zkAuctionContractAddress);

        initializeMerkle(treeDepth, _vaultId, _hashContractAddress);

        return true;
    }

    function _insertCommitmentsFromReceipt(
        IEnygmaDvp.ProofReceipt memory receipt
    ) internal returns (bool) {
        uint256 inputSize = receipt.numberOfInputs;
        uint256 outputSize = receipt.numberOfOutputs;
        uint nullifiersIndex = 1 + (2 * inputSize);
        uint commitmentsIndex = nullifiersIndex + inputSize;

        uint numberOfCommitments = 0;
        for (uint256 i = 0; i < outputSize; i++) {
            if (receipt.statement[commitmentsIndex + i] != 0) {
                numberOfCommitments++;
            }
        }

        uint256[] memory commitments = new uint256[](numberOfCommitments);

        for (uint256 i = 0; i < numberOfCommitments; i++) {
            commitments[i] = (receipt.statement[commitmentsIndex + i]);
            emit Commitment(_vaultId, commitments[i]);
        }

        insertLeaves(commitments);

        return true;
    }

    // publicly only accessible by ZkDvp
    function insertCommitmentsFromReceipt(
        IEnygmaDvp.ProofReceipt memory receipt
    ) public onlyRole(DEFAULT_DVP_ROLE) returns (bool) {
        return _insertCommitmentsFromReceipt(receipt);
    }

    function _nullifyFromReceipt(
        IEnygmaDvp.ProofReceipt memory receipt
    ) internal returns (bool) {
        uint256 inputSize = receipt.numberOfInputs;

        uint rootsIndex = 1 + (1 * inputSize);
        uint nullifiersIndex = 1 + (2 * inputSize);
        for (uint i = 0; i < inputSize; i++) {
            uint256 treeNumber = receipt.statement[1 + i];
            uint256 nullifier = receipt.statement[nullifiersIndex + i];
            if (receipt.statement[rootsIndex + i] != 0) {
                if (!isLocked(treeNumber, nullifier)) {
                    setNullifier(treeNumber, nullifier);
                    emit Nullifier(_vaultId, treeNumber, nullifier);
                } else {
                    revert CantSpendLockedCoin();
                }
            }
        }
    }

    function _unlockFromReceipt(
        IEnygmaDvp.ProofReceipt memory receipt
    ) internal returns (bool) {
        uint256 inputSize = receipt.numberOfInputs;

        uint rootsIndex = 1 + (1 * inputSize);
        uint nullifiersIndex = 1 + (2 * inputSize);
        for (uint i = 0; i < inputSize; i++) {
            uint256 treeNumber = receipt.statement[1 + i];
            uint256 nullifier = receipt.statement[nullifiersIndex + i];
            if (receipt.statement[rootsIndex + i] != 0) {
                if (isLocked(treeNumber, nullifier)) {
                    unlockCoin(treeNumber, nullifier);
                    emit CoinUnlocked(_vaultId, treeNumber, nullifier);
                } else {
                    revert CoinAlreadyUnlocked();
                }
            }
        }
    }

    // public access is only allowed for ZkDvp
    function nullifyFromReceipt(
        IEnygmaDvp.ProofReceipt memory receipt
    ) public onlyRole(DEFAULT_DVP_ROLE) returns (bool) {
        return _nullifyFromReceipt(receipt);
    }

    function unlockFromReceipt(
        IEnygmaDvp.ProofReceipt memory receipt
    ) public onlyRole(DEFAULT_DVP_ROLE) returns (bool) {
        return _unlockFromReceipt(receipt);
    }

    function lockCoin(
        uint256 treeNumber,
        uint256 nullifier
    ) public onlyRole(DEFAULT_DVP_ROLE) returns (bool) {
        lock(treeNumber, nullifier);
        emit CoinLocked(_vaultId, treeNumber, nullifier);

        return true;
    }

    function unlockCoin(
        uint256 treeNumber,
        uint256 nullifier
    ) public onlyRole(DEFAULT_DVP_ROLE) returns (bool) {
        unlock(treeNumber, nullifier);
        emit CoinUnlocked(_vaultId, treeNumber, nullifier);

        return true;
    }

    function nullifyCoin(
        uint256 treeNumber,
        uint256 nullifier
    ) public onlyRole(DEFAULT_DVP_ROLE) returns (bool) {
        setNullifier(treeNumber, nullifier);
        emit Nullifier(_vaultId, treeNumber, nullifier);

        return true;
    }

    function registerCoins(
        uint256[] memory commitments
    ) public onlyRole(DEFAULT_DVP_ROLE) returns (bool) {
        uint numberOfCommitments = 0;
        for (uint256 i = 0; i < commitments.length; i++) {
            if (commitments[i] != 0) {
                numberOfCommitments++;
            }
        }

        uint256[] memory commitmentsToInsert = new uint256[](
            numberOfCommitments
        );

        for (uint256 i = 0; i < numberOfCommitments; i++) {
            if (commitments[i] != 0) {
                commitmentsToInsert[i] = (commitments[i]);
                emit Commitment(_vaultId, commitmentsToInsert[i]);
            }
        }

        insertLeaves(commitmentsToInsert);

        return true;
    }

    function addPendingProofReceipt(
        IEnygmaDvp.ProofReceipt memory receipt
    ) public onlyRole(DEFAULT_DVP_ROLE) returns (bool) {
        uint256 inputSize = receipt.numberOfInputs;
        uint256 outputSize = receipt.numberOfOutputs;

        uint treeNumbersIndex = 1;
        uint nullifiersIndex = 1 + (2 * inputSize);
        uint commitmentsIndex = nullifiersIndex + inputSize;

        uint256 utxoUniqueId = receipt.statement[commitmentsIndex];

        // TODO:: you can check being empty but proof parameters being zero
        if (_pendingProofReceipts[utxoUniqueId].statement.length != 0) {
            // proofReceipt has been already added to the vault
            revert ProofReceiptAlreadyAdded();
        } else {
            _pendingProofReceipts[utxoUniqueId] = receipt;

            // lock the nullifiers to avoid potential front-running cases

            for (uint256 i = 0; i < receipt.numberOfInputs; i++) {
                lockCoin(
                    receipt.statement[treeNumbersIndex + i],
                    receipt.statement[nullifiersIndex + i]
                );
            }
        }

        return true;
    }

    function getPendingProofReceipt(
        uint256 proofUniqueId
    ) public returns (IEnygmaDvp.ProofReceipt memory proofReceipt) {
        return _pendingProofReceipts[proofUniqueId];
    }

    function checkRegisterBrokerProofConditions(
        IEnygmaDvp.ProofReceipt memory receipt
    ) public returns (bool) {
        // signal input st_beacon;
        // signal input st_vaultId;
        // signal input st_groupId;
        // signal input st_delegator_treeNumbers[tm_numOfInputs];
        // signal input st_delegator_merkleRoots[tm_numOfInputs];
        // signal input st_delegator_nullifiers[tm_numOfInputs];
        // signal input st_broker_blindedPublicKey;

        // signal input st_assetGroup_treeNumber;
        // signal input st_assetGroup_merkleRoot;

        uint jInputSize = receipt.numberOfInputs;
        uint jTreeNumbersIndex = 3;
        uint jRootsIndex = jTreeNumbersIndex + jInputSize;
        uint jNullifiersIndex = jRootsIndex + jInputSize;

        for (uint i = 0; i < jInputSize; i++) {
            if (receipt.statement[jRootsIndex + i] != 0) {
                if (
                    !isValidRoot(
                        receipt.statement[jTreeNumbersIndex + i],
                        receipt.statement[jRootsIndex + i]
                    )
                ) {
                    revert InvalidMerkleRoot();
                }

                if (
                    isValidNullifier(
                        receipt.statement[jTreeNumbersIndex + i],
                        receipt.statement[jNullifiersIndex + i]
                    )
                ) {
                    revert InvalidNullifier();
                }

                // locking the coins for later settlement
                lockCoin(
                    receipt.statement[jTreeNumbersIndex + i],
                    receipt.statement[jNullifiersIndex + i]
                );
            }
        }

        return true;
    }
}
