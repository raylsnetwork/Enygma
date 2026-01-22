// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;
// pragma abicoder v2;

import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";

import {IEnygmaDvp} from "../../interfaces/IEnygmaDvp.sol";
import {IPoseidonWrapper} from "../../interfaces/IPoseidonWrapper.sol";
import {IVerifier} from "../../interfaces/IVerifier.sol";
import {AbstractCoinVault} from "./AbstractCoinVault.sol";

contract Erc20CoinVault is AbstractCoinVault {
    ///////////////////////////////////////////////
    //              Constants
    //////////////////////////////////////////////

    uint256 public constant VK_ID_ERC20_JOINSPLIT = 0;
    uint256 public constant VK_ID_ERC20_10INPUT = 6;

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

    // Standards that are currently supported: ERC20, ERC721, ERC1155
    function deposit(uint256[] memory params) public override returns (bool) {
        // Transferring the ERC20 tokens from User to ZkDvp
        uint256 amount = params[0];
        uint256 publicKey = params[1];
        IERC20(_assetContractAddress).transferFrom(
            msg.sender,
            address(this),
            amount
        );

        uint256[] memory assetParams = new uint256[](2);
        assetParams[0] = amount;
        assetParams[1] = uint256(uint160(_assetContractAddress));
        // Generating uniqueId for the ERC20 token
        uint256 uid = generateUniqueId(assetParams);

        // Generating the commitment based on the ERC20 uniqueId and the publickey
        uint256 commitment = IPoseidonWrapper(_hashContractAddress).poseidon(
            [uid, publicKey]
        );

        uint256[] memory commitments = new uint256[](1);
        commitments[0] = commitment;

        insertLeaves(commitments);

        emit Commitment(_vaultId, commitment);

        return true;
    }

    function transfer(
        IEnygmaDvp.ProofReceipt memory receipt
    ) public override returns (bool) {
        // jsReceipt.inputs;
        // message;
        // treeNumbers[numberOfInputs];
        // merkleRoots[numberOfInputs];
        // nullifiers[numberOfInputs];
        // commitments[numberOfOutputs];

        uint jInputSize = receipt.numberOfInputs;
        uint jTreeNumbersIndex = 1 + jInputSize;
        uint jNullifiersIndex = jTreeNumbersIndex + (2 * jInputSize);
        uint jCommitmentsIndex = jNullifiersIndex + jInputSize;

        // checking the proof

        checkReceiptConditions(receipt);

        _insertCommitmentsFromReceipt(receipt);

        // Nullifying the old coins
        _nullifyFromReceipt(receipt);

        return true;
    }

    function withdraw(
        uint256[] memory withdrawParams,
        address recipient,
        IEnygmaDvp.ProofReceipt memory receipt
    ) public override returns (bool) {
        //     receipt.statement;
        //     message;
        //     treeNumbers[numberOfInputs];
        //     merkleRoots[numberOfInputs];
        //     nullifiers[numberOfInputs];
        //     commitments[numberOfOutputs];

        uint256 amount = withdrawParams[0];

        uint256 treeNumbersIndex = 1;
        uint256 merkleRootsIndex = 1 + receipt.numberOfInputs;
        uint256 nullifiersIndex = merkleRootsIndex + receipt.numberOfInputs;
        uint256 commitmentsIndex = nullifiersIndex + receipt.numberOfInputs;

        uint256[] memory assetParams = new uint256[](2);
        assetParams[0] = amount;
        assetParams[1] = uint256(uint160(_assetContractAddress));
        // generating uniqueId for ERC20 token
        uint256 uid = generateUniqueId(assetParams);

        // generating commitment based on uniqueId and publicKey
        uint256 commitment = IPoseidonWrapper(_hashContractAddress).poseidon(
            [uid, uint256(uint160(recipient))]
        );

        // checking if the computed commitment
        // matches the first commitment in the proof.

        if (receipt.statement[commitmentsIndex] != commitment) {
            revert InvalidOpening();
        }

        // checking generic JoinSplit proof conditions

        checkReceiptConditions(receipt);

        // Transfering the tokens from ZkDvp to User
        IERC20(_assetContractAddress).transfer(recipient, amount);

        // Nullifying the input coins
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
        IEnygmaDvp.ProofReceipt memory receipt
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

    function verifyOwnership(
        uint256[] memory params_,
        IEnygmaDvp.ProofReceipt memory receipt_
    ) public override returns (bool) {
        // params:
        // 0: amount
        uint256 amount = params_[0];

        // receipt.statement:
        // 0 challenge;
        // 1 treeNumber;
        // 2 merkleRoot;
        // 3 nullifier;
        // 4 commitment;
        uint256 challenge = receipt_.statement[0];

        IEnygmaDvp(_zkDvpContractAddress).checkAndRegisterChallenge(challenge);

        uint256[] memory uparams = new uint256[](2);
        uparams[0] = amount;
        // regenerating uniqueId and commitment to verify
        uint256 uid = generateUniqueId(uparams);

        uint256 commitment = IPoseidonWrapper(_hashContractAddress).poseidon(
            [uid, uint256(uint160(0))]
        );

        if (receipt_.statement[4] != commitment) {
            revert InvalidOpening();
        }

        // checking generic conditions of Ownership receipt.

        checkReceiptConditions(receipt_);
        // firing the receipt event
        emit OwnershipVerificationReceipt(
            challenge,
            _vaultId,
            0, // there is no token Id for erc20
            amount
        );

        return true;
    }
}
