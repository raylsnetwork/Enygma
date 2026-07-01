// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;
// pragma abicoder v2;

import {IERC1155} from "@openzeppelin/contracts/token/ERC1155/IERC1155.sol";
import {IRaylsERC1155} from "../../../erc1155/interfaces/IRaylsERC1155.sol";
import "@openzeppelin/contracts/token/ERC1155/utils/ERC1155Holder.sol";

import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";

import {IEnygmaDvp} from "../../interfaces/IEnygmaDvp.sol";
import {IPoseidonWrapper} from "../../interfaces/IPoseidonWrapper.sol";
import {IVerifier} from "../../interfaces/IVerifier.sol";
import {AbstractCoinVault} from "./AbstractCoinVault.sol";
// import {IFungibilityMerkle} from "../../interfaces/vaults/IFungibilityMerkle.sol";

contract Erc1155CoinVault is AbstractCoinVault, ERC1155Holder {
    ///////////////////////////////////////////////
    //              Constants
    //////////////////////////////////////////////

    uint256 public constant VK_ID_ERC1155_1 = 2; // nonfungible
    uint256 public constant VK_ID_ERC1155_FUNG_1 = 3;
    uint256 public constant VK_ID_ERC1155_2 = 4; // fungible joinSplit
    uint256 public constant VK_ID_ERC1155_10 = 5; // non-fungible batch
    uint256 public constant VK_ID_ERC1155_FUNG_AUDITOR = 15;
    uint256 public constant VK_ID_ERC1155_NON_FUNG_AUDITOR = 16;
    ///////////////////////////////////////////////
    //              Constructor
    //////////////////////////////////////////////

    constructor(
        address zkDvpContractAddress
    ) AbstractCoinVault(zkDvpContractAddress) {
        _name = "ZkDvp - ERC1155 Coin Vault";
    }

    //////////////////////////////////////////////
    // Overwrite function :  Access Control and ERC155Holder
    //////////////////////////////////////////////
    function supportsInterface(
        bytes4 interfaceId
    )
        public
        view
        virtual
        override(ERC1155Receiver, AccessControl)
        returns (bool)
    {
        return
            ERC1155Receiver.supportsInterface(interfaceId) ||
            AccessControl.supportsInterface(interfaceId);
    }

    //////////////////////////////////////////////
    // Vault functions
    //////////////////////////////////////////////
    // C-1 fix: commitment is now computed on-chain from caller-supplied key material
    // so the actual transferred amount is binding.  Previously a caller could pass any
    // commitment encoding a different amount, enabling vault drain.
    //
    // Single deposit (params.length == 4):
    //   params[0] = amountOrOne
    //   params[1] = tokenId
    //   params[2] = pkSpend  — recipient's BabyJubJub spend public key (x-coordinate)
    //   params[3] = salt     — random field element chosen by the depositor
    //   Commitment = Poseidon4(tokenId, amountOrOne, pkSpend, salt)
    //
    // Batch deposit (params.length is a multiple of 4):
    //   params[0..N-1]       = tokenIds
    //   params[N..2N-1]      = amounts
    //   params[2N..3N-1]     = pkSpeends (one per token)
    //   params[3N..4N-1]     = salts     (one per token)
    function deposit(uint256[] memory params) public override returns (bool) {
        if (params.length == 4) {
            uint256 amountOrOne = params[0];
            uint256 tokenId     = params[1];
            uint256 pkSpend     = params[2];
            uint256 salt        = params[3];
            bytes memory data   = "";

            IERC1155(_assetContractAddress).safeTransferFrom(
                msg.sender,
                address(this),
                tokenId,
                amountOrOne,
                data
            );

            // Commitment = Poseidon4(pkSpend, salt, amountOrOne, tokenId) — matches circuit
            // Erc1155Commitment(tokenId, amount, pk, salt) = Erc20CommitmentV2(pk, salt, amount, tokenId).
            uint256 commitment = IPoseidonWrapper(_hashContractAddress).poseidon4(
                [pkSpend, salt, amountOrOne, tokenId]
            );

            uint256[] memory commitments = new uint256[](1);
            commitments[0] = commitment;

            insertLeaves(commitments);

            emit Commitment(_vaultId, commitment);
        } else {
            // batch mode: params layout [tokenIds | amounts | pkSpeends | salts]
            uint256 tokenCount = params.length / 4;

            uint256[] memory tokenIds = new uint256[](tokenCount);
            uint256[] memory amounts  = new uint256[](tokenCount);

            for (uint i = 0; i < tokenCount; i++) {
                tokenIds[i] = params[i];
                amounts[i]  = params[i + tokenCount];
            }

            IERC1155(_assetContractAddress).safeBatchTransferFrom(
                msg.sender,
                address(this),
                tokenIds,
                amounts,
                ""
            );

            for (uint i = 0; i < tokenCount; i++) {
                uint256 pkSpend = params[i + tokenCount * 2];
                uint256 salt    = params[i + tokenCount * 3];

                uint256 commitment = IPoseidonWrapper(_hashContractAddress).poseidon4(
                    [pkSpend, salt, amounts[i], tokenIds[i]]
                );

                uint256[] memory commitments = new uint256[](1);
                commitments[0] = commitment;

                insertLeaves(commitments);

                emit Commitment(_vaultId, commitment);
            }
        }

        return true;
    }

    function transfer(
        IEnygmaDvp.ProofReceipt memory receipt
    ) public override nonReentrant returns (bool) {
        // H-4 fix: added nonReentrant guard and corrected CEI ordering.
        // Previously outputs were inserted before nullifiers were spent, opening a
        // reentrancy window via the Poseidon precompile inside insertLeaves(). A
        // re-entrant call would find the input notes un-nullified and could double-spend.
        checkReceiptConditions(receipt);
        // Effects: nullify inputs FIRST so re-entrant calls find them already spent.
        _nullifyFromReceipt(receipt);
        // Then insert outputs.
        _insertCommitmentsFromReceipt(receipt);
        return true;
    }

    function withdraw(
        uint256[] memory withdrawParams,
        address recipient,
        IEnygmaDvp.ProofReceipt memory receipt
    ) public override nonReentrant returns (bool) {
        //     receipt.statement;
        //     message;
        //     treeNumbers[numberOfInputs];
        //     merkleRoots[numberOfInputs];
        //     nullifiers[numberOfInputs];
        //     commitments[numberOfOutputs];

        // TODO:: check the params.length and receipt.numberOfInputs matches
        // if(withdrawParams.length != receipt.numberOfOutputs * _numberOfIdentifiers){
        //     revert WrongNumberOfIdentifiers();
        // }

        // check and verify generic Erc1155 proof receipt
        checkReceiptConditions(receipt);

        uint256 merkleRootsIndex = 1 + receipt.numberOfInputs;
        uint256 nullifiersIndex = merkleRootsIndex + receipt.numberOfInputs;
        uint256 commitmentsIndex = nullifiersIndex + receipt.numberOfInputs;

        // Checks + Effects: verify commitments and record all nullifiers before
        // any external transfer (checks-effects-interactions pattern).
        for (uint256 i = 0; i < receipt.numberOfOutputs; i++) {
            uint256 amountOrOne = withdrawParams[i * 2];
            uint256 tokenId = withdrawParams[i * 2 + 1];

            if (amountOrOne != 0 || tokenId != 0) {
                uint256[] memory assetParams = new uint256[](2);
                assetParams[0] = amountOrOne;
                assetParams[1] = tokenId;
                uint256 uid = generateUniqueId(assetParams);
                uint256 commitment = IPoseidonWrapper(_hashContractAddress)
                    .poseidon([uid, uint256(uint160(recipient))]);

                if (receipt.statement[commitmentsIndex + i] != commitment) {
                    revert InvalidOpening();
                }
            }

            if (receipt.statement[nullifiersIndex + i] != 0) {
                setNullifier(
                    receipt.statement[1 + i],
                    receipt.statement[nullifiersIndex + i]
                );
                emit Nullifier(
                    _vaultId,
                    receipt.statement[1 + i],
                    receipt.statement[nullifiersIndex + i]
                );
            }
        }

        // Interactions: external transfers after all state updates.
        for (uint256 i = 0; i < receipt.numberOfOutputs; i++) {
            uint256 amountOrOne = withdrawParams[i * 2];
            uint256 tokenId = withdrawParams[i * 2 + 1];

            if (amountOrOne != 0 || tokenId != 0) {
                IERC1155(_assetContractAddress).safeTransferFrom(
                    address(this),
                    recipient,
                    tokenId,
                    amountOrOne,
                    ""
                );
            }
        }

        return true;
    }

    function verifyOwnership(
        uint256[] memory params_,
        IEnygmaDvp.ProofReceipt memory receipt_
    ) public override returns (bool) {
        // params:
        // 0: tokenId
        // 1: amountOrOne

        // receipt.statement:
        // 0 challenge;
        // 1 treeNumber;
        // 2 merkleRoot;
        // 3 nullifier;
        // 4 commitment;
        uint256 amountOrOne = params_[0];
        uint256 tokenId = params_[1];
        uint256 challenge = receipt_.statement[0];

        // to avoid replay-attack
        IEnygmaDvp(_zkDvpContractAddress).checkAndRegisterChallenge(challenge);

        uint256[] memory uparams = new uint256[](2);
        uparams[0] = amountOrOne;
        uparams[1] = tokenId;
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
            tokenId,
            amountOrOne
        );

        return true;
    }

    function generateUniqueId(
        uint256[] memory params
    ) public view override returns (uint256) {
        uint256 amountOrOne = params[0];
        uint256 tokenId = params[1];

        uint256 uid1 = IPoseidonWrapper(_hashContractAddress).poseidon(
            [uint256(uint160(_assetContractAddress)), tokenId]
        );
        uint256 uid2 = IPoseidonWrapper(_hashContractAddress).poseidon(
            [uid1, amountOrOne]
        );

        return uid2;
    }

    function getEncryptionSizeForFungible(
        uint numberOfInputs,
        uint numberOfOutputs
    ) public pure returns (uint) {
        uint plainLength = numberOfInputs + numberOfInputs + 2;
        uint decLength = plainLength;
        while (decLength % 3 != 0) {
            decLength += 1;
        }
        return (decLength + 1);
    }
    function getEncryptionSizeForNonFungible(
        uint numberOfTokens
    ) public pure returns (uint) {
        uint plainLength = numberOfTokens * 2 + 1;
        uint decLength = plainLength;
        while (decLength % 3 != 0) {
            decLength += 1;
        }
        return (decLength + 1);
    }

    function checkReceiptConditions(
        IEnygmaDvp.ProofReceipt memory receipt
    ) public view override returns (bool) {
        // TODO:: In this function, type of receipt
        // is being determined by the size of the statement
        // it is not secure. Fix it.

        uint256 receiptType;

        uint256 expectedBatchSize = 1 + 6 * receipt.numberOfInputs;
        uint256 expectedNormalSize = 3 +
            3 *
            receipt.numberOfInputs +
            receipt.numberOfOutputs;

        uint256 expectedFungibleAuditorSize = expectedNormalSize +
            5 +
            getEncryptionSizeForFungible(
                receipt.numberOfInputs,
                receipt.numberOfOutputs
            );
        uint256 expectedNonFungibleAuditorSize = expectedNormalSize +
            5 +
            getEncryptionSizeForNonFungible(receipt.numberOfInputs);

        if (receipt.statement.length == expectedNormalSize) {
            receiptType = 1;

            if (receipt.numberOfInputs != receipt.numberOfOutputs) {
                revert InvalidNumberOfInputs();
            }
            // TODO:: when adding new normal circuits,
            // you need to add the numberOfInputs and outputs here
            if (
                receipt.numberOfInputs != 1 &&
                receipt.numberOfInputs != 2 &&
                receipt.numberOfInputs != 10
            ) {
                revert InvalidNumberOfInputs();
            }

            if (
                receipt.numberOfInputs != 1 &&
                receipt.numberOfOutputs != 2 &&
                receipt.numberOfOutputs != 10
            ) {
                revert InvalidNumberOfOutputs();
            }
        } else if (receipt.statement.length == expectedBatchSize) {
            receiptType = 2;
        } else if (receipt.statement.length == expectedFungibleAuditorSize) {
            receiptType = 4;
        } else if (receipt.statement.length == expectedNonFungibleAuditorSize) {
            receiptType = 5;
            // if(receipt.numberOfInputs != 2){
            //     revert InvalidNumberOfInputs();
            // }

            // if(receipt.numberOfOutputs != 2){
            //     revert InvalidNumberOfOutputs();
            // }
        } else {
            revert InvalidStatmentSize();
        }

        // jsReceipt.inputs;

        uint jInputSize = receipt.numberOfInputs;
        uint jTreeNumbersIndex = 1;
        uint jRootsIndex = jTreeNumbersIndex + jInputSize;
        uint jNullifiersIndex = jRootsIndex + jInputSize;
        uint jCommitmentsIndex = jNullifiersIndex + jInputSize;

        // TODO:: check all pairs of commitment to be different
        // Adding this require to original ones from the Aegis repo
        // to avoid entering the same coins' commitments in the
        // two input slots.
        if (jInputSize == 2) {
            if (
                receipt.statement[jCommitmentsIndex] ==
                receipt.statement[jCommitmentsIndex + 1]
            ) {
                revert JoinSplitWithSameCommitments();
            }
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

        if (receiptType == 5) {
            verifyAuditorPublicKey(receipt);
            IVerifier(_verifierContractAddress).verifyProof(
                VK_ID_ERC1155_NON_FUNG_AUDITOR,
                receipt.proof,
                receipt.statement
            );
        } else if (receiptType == 4) {
            // Auditor-enabled Proof

            verifyAuditorPublicKey(receipt);
            IVerifier(_verifierContractAddress).verifyProof(
                VK_ID_ERC1155_FUNG_AUDITOR,
                receipt.proof,
                receipt.statement
            );
        } else if (receiptType == 1) {
            // it is normal receipt
            if (jInputSize == 1) {
                IVerifier(_verifierContractAddress).verifyProof(
                    VK_ID_ERC1155_1,
                    receipt.proof,
                    receipt.statement
                );
            } else if (jInputSize == 2) {
                IVerifier(_verifierContractAddress).verifyProof(
                    VK_ID_ERC1155_2,
                    receipt.proof,
                    receipt.statement
                );
            } else {
                revert InvalidStatmentSize();
            }
        } else if (receiptType == 2) {
            if (jInputSize == 10) {
                IVerifier(_verifierContractAddress).verifyProof(
                    VK_ID_ERC1155_10,
                    receipt.proof,
                    receipt.statement
                );
            } else {
                revert InvalidStatmentSize();
            }
        }

        return true;
    }

    function verifyAuditorPublicKey(
        IEnygmaDvp.ProofReceipt memory receipt
    ) internal view returns (bool) {
        uint256[2] memory auditorPublicKey;

        uint256 pkIndex = receipt.numberOfInputs *
            3 +
            receipt.numberOfOutputs +
            1;
        auditorPublicKey[0] = receipt.statement[pkIndex];
        auditorPublicKey[1] = receipt.statement[pkIndex + 1];

        if (
            !(
                IEnygmaDvp(_zkDvpContractAddress).isAuditorRegistered(
                    auditorPublicKey
                )
            )
        ) {
            revert InvalidAuditorPublicKey();
        }

        return true;
    }
}
