// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;
// pragma abicoder v2;

import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";

import {IZkDvp} from "../../interfaces/IZkDvp.sol";
import {IPoseidonWrapper} from "../../interfaces/IPoseidonWrapper.sol";
import {IVerifier} from "../../interfaces/IVerifier.sol";
import {AbstractCoinVault} from "./AbstractCoinVault.sol";
import {IEnygmaErc20CoinVault} from "../../interfaces/vaults/IEnygmaErc20CoinVault.sol";

contract EnygmaErc20CoinVault is AbstractCoinVault, IEnygmaErc20CoinVault {
    ///////////////////////////////////////////////
    //              Constants
    //////////////////////////////////////////////

    uint256 public constant VK_ID_ERC20_JOINSPLIT = 0;
    uint256 public constant VK_ID_ERC20_10INPUT = 5;

    bytes32 public constant DEFAULT_ENYGMA_ROLE =
        keccak256(abi.encodePacked("EnygmaRole"));

    address private _enygmaAddress;
    ///////////////////////////////////////////////
    //              Constructor
    //////////////////////////////////////////////

    // hashContractAddress: poseidon Wrapper contract address
    // genericVerifierContractAddress: Groth16 generic verifier address.
    // TODO:: some form of verification is needed
    constructor(
        address zkDvpContractAddress
    ) AbstractCoinVault(zkDvpContractAddress) {
        _name = "ZkDvp - ERC20 Coin Vault";
    }

    function addEnygma(
        address enygmaAddress
    ) public onlyRole(DEFAULT_OWNER_ROLE) returns (bool) {
        _assetContractAddress = enygmaAddress;
        _enygmaAddress = enygmaAddress;
        _setupRole(DEFAULT_ENYGMA_ROLE, _enygmaAddress);
        return true;
    }

    function depositThroughEnygma(
        uint256[] memory depositParams
    ) public override onlyRole(DEFAULT_ENYGMA_ROLE) returns (bool, uint256) {
        // Generating uniqueId for Enygma tokens.
        // TODO:: here consider adding another id to differentiate
        // between the erc20 tokens id and Enygma tokens Id.

        uint256 amount = depositParams[0];
        uint256 publicKey = depositParams[1];

        uint256[] memory assetParams = new uint256[](1);
        assetParams[0] = amount;

        uint256 uid = generateUniqueId(assetParams);

        // Generating the commitment based on the ERC20 uniqueId and the publickey
        uint256 commitment = IPoseidonWrapper(_hashContractAddress).poseidon(
            [uid, publicKey]
        );

        uint256[] memory commitments = new uint256[](1);
        commitments[0] = commitment;

        insertLeaves(commitments);

        emit Commitment(_vaultId, commitment);

        return (true, commitment);
    }

    function withdrawThroughEnygma(
        IZkDvp.ProofReceipt memory receipt
    ) public override onlyRole(DEFAULT_ENYGMA_ROLE) returns (bool) {
        uint256 treeNumbersIndex = 1;
        uint256 merkleRootsIndex = 1 + receipt.numberOfInputs;
        uint256 nullifiersIndex = merkleRootsIndex + receipt.numberOfInputs;
        uint256 commitmentsIndex = nullifiersIndex + receipt.numberOfInputs;

        checkReceiptConditions(receipt);

        for (uint256 i = 0; i < receipt.numberOfInputs; i++) {
            if (receipt.statement[nullifiersIndex + i] != 0) {
                setNullifier(
                    receipt.statement[treeNumbersIndex + i],
                    receipt.statement[nullifiersIndex + i]
                );

                emit Nullifier(
                    _vaultId,
                    receipt.statement[treeNumbersIndex + i],
                    receipt.statement[nullifiersIndex + i]
                );
            }
        }

        return true;
    }

    // Standards that are currently supported: ERC20, ERC721, ERC1155
    function deposit(
        uint256[] memory params
    ) public override onlyRole(DEFAULT_ENYGMA_ROLE) returns (bool) {
        revert NotImplemented();
    }

    function transfer(
        IZkDvp.ProofReceipt memory receipt
    ) public override returns (bool) {
        revert NotImplemented();
    }

    function withdraw(
        uint256[] memory withdrawParams,
        address recipient,
        IZkDvp.ProofReceipt memory receipt
    ) public override returns (bool) {
        revert NotImplemented();

        return true;
    }

    function verifyOwnership(
        uint256[] memory params,
        IZkDvp.ProofReceipt memory receipt
    ) public override returns (bool) {
        revert NotImplemented();
    }

    ///////////////////////////////////////////////
    //       Generic functions
    //////////////////////////////////////////////
    function generateUniqueId(
        uint256[] memory params
    ) public view override returns (uint256) {
        uint256 amount = params[0];
        return
            IPoseidonWrapper(_hashContractAddress).poseidon(
                [uint256(uint160(_assetContractAddress)), amount]
            );
    }

    function checkReceiptConditions(
        IZkDvp.ProofReceipt memory receipt
    ) public view override returns (bool) {
        // jsReceipt.inputs;
        // message;
        // treeNumbers[numberOfInputs];
        // merkleRoots[numberOfInputs];
        // nullifiers[numberOfInputs];
        // commitments[numberOfOutputs];

        uint jInputSize = receipt.numberOfInputs;
        uint jTreeNumbersIndex = 1;
        uint jRootsIndex = jTreeNumbersIndex + jInputSize;
        uint jNullifiersIndex = jRootsIndex + jInputSize;
        uint jCommitmentsIndex = jNullifiersIndex + jInputSize;

        // TODO:: check all pairs of commitment to be different
        // Adding this require to original ones from the Aegis repo
        // to avoid entering the same coins' commitments in the
        // two input slots.

        if (
            receipt.statement[jCommitmentsIndex] ==
            receipt.statement[jCommitmentsIndex + 1]
        ) {
            revert JoinSplitWithSameCommitments();
        }

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
            }
        }

        if (receipt.numberOfInputs != 2 && receipt.numberOfInputs != 10) {
            revert InvalidNumberOfInputs();
        }
        if (receipt.numberOfInputs == 2) {
            IVerifier(_verifierContractAddress).verifyProof(
                VK_ID_ERC20_JOINSPLIT,
                receipt.proof,
                receipt.statement
            );
        } else {
            IVerifier(_verifierContractAddress).verifyProof(
                VK_ID_ERC20_10INPUT,
                receipt.proof,
                receipt.statement
            );
        }

        return true;
    }
}
