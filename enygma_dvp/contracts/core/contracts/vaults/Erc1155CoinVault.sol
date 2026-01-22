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
    uint256 public constant VK_ID_ERC1155_FUNG_BROKER = 14;
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
    function deposit(uint256[] memory params) public override returns (bool) {
        if (params.length == 3) {
            uint256 amountOrOne = params[0];
            uint256 tokenId = params[1];
            uint256 publicKey = params[2];
            bytes memory data = "";

            // transfering from user to ZkDvp smart contract
            IERC1155(_assetContractAddress).safeTransferFrom(
                msg.sender,
                address(this),
                tokenId,
                amountOrOne,
                data
            );

            uint256[] memory idParams = new uint256[](2);
            idParams[0] = amountOrOne;
            idParams[1] = tokenId;
            // generating the uniqueId of ERC1155 token
            uint256 uid = generateUniqueId(idParams);

            // generating commitment based on uniqueId and publickey
            uint256 commitment = IPoseidonWrapper(_hashContractAddress)
                .poseidon([uid, publicKey]);

            uint256[] memory commitments = new uint256[](1);
            commitments[0] = commitment;

            insertLeaves(commitments);

            emit Commitment(_vaultId, commitment);
        } else {
            // batch mode

            uint256 tokenCount = params.length / 3;
            uint256[] memory tokenIds = new uint256[](tokenCount);
            uint256[] memory amounts = new uint256[](tokenCount);
            uint256[] memory publicKeys = new uint256[](tokenCount);

            for (uint i = 0; i < tokenIds.length; i++) {
                // generating the uniqueId of ERC1155 token
                tokenIds[i] = params[i];
                amounts[i] = params[i + tokenCount];
                publicKeys[i] = params[i + (tokenCount * 2)];

                uint256[] memory params = new uint256[](1);
                params[0] = tokenIds[i];

                // checking if token is fungible
                if (
                    IRaylsERC1155(_assetContractAddress).isFungible(tokenIds[i])
                ) {
                    //  TODO:: check tokenId exists in fungibilityMerkle
                    // TODO:: check range of fungible [[IMPORTANT]]

                    uint256 uid1 = IPoseidonWrapper(_hashContractAddress)
                        .poseidon(
                            [
                                uint256(uint160(_assetContractAddress)),
                                tokenIds[i]
                            ]
                        );
                    // IZkDvp(_zkDvpContractAddress).insertToFungibleAssetGroup(uid1);
                }
            }

            IERC1155(_assetContractAddress).safeBatchTransferFrom(
                msg.sender,
                address(this),
                tokenIds,
                amounts,
                ""
            );

            for (uint i = 0; i < tokenIds.length; i++) {
                // generating the uniqueId of ERC1155 token

                uint256[] memory idParams = new uint256[](2);
                idParams[0] = amounts[i];
                idParams[1] = tokenIds[i];
                uint256 uid = generateUniqueId(idParams);

                // generating commitment based on uniqueId and publickey to verify
                uint256 commitment = IPoseidonWrapper(_hashContractAddress)
                    .poseidon([uid, publicKeys[i]]);

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
    ) public override returns (bool) {
        // receipt.inputs;
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

        // TODO:: check the params.length and receipt.numberOfInputs matches
        // if(withdrawParams.length != receipt.numberOfOutputs * _numberOfIdentifiers){
        //     revert WrongNumberOfIdentifiers();
        // }

        // check and verify generic Erc1155 proof receipt
        checkReceiptConditions(receipt);

        uint256 merkleRootsIndex = 1 + receipt.numberOfInputs;
        uint256 nullifiersIndex = merkleRootsIndex + receipt.numberOfInputs;
        uint256 commitmentsIndex = nullifiersIndex + receipt.numberOfInputs;

        for (uint256 i = 0; i < receipt.numberOfOutputs; i++) {
            uint256 amountOrOne = withdrawParams[i * 2];
            uint256 tokenId = withdrawParams[i * 2 + 1];

            if (amountOrOne != 0 || tokenId != 0) {
                uint256[] memory assetParams = new uint256[](2);
                assetParams[0] = amountOrOne;
                assetParams[1] = tokenId;
                // generating uniqueId for ERC20 token
                uint256 uid = generateUniqueId(assetParams);

                uint addrUint = uint256(uint160(recipient));

                // generating commitment based on uniqueId and publicKey
                uint256 commitment = IPoseidonWrapper(_hashContractAddress)
                    .poseidon([uid, addrUint]);

                // checking if the computed commitment
                // matches the first commitment in the proof.

                if (receipt.statement[commitmentsIndex + i] != commitment) {
                    revert InvalidOpening();
                }
                // Transfering the tokens from ZkDvp to recipient
                IERC1155(_assetContractAddress).safeTransferFrom(
                    address(this),
                    recipient,
                    tokenId,
                    amountOrOne,
                    ""
                );
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

        // size of normal statement: 1 + 3 * numberOfInputs + numberOfOutputs + 2
        // size broker-enabled statement: 3 + 3 * numberOfInputs + numberOfOutputs + 4
        uint256 expectedBatchSize = 1 + 6 * receipt.numberOfInputs;
        uint256 expectedNormalSize = 3 +
            3 *
            receipt.numberOfInputs +
            receipt.numberOfOutputs;
        // uint256 expectedBrokerSize = 5 + 3 * receipt.numberOfInputs + receipt.numberOfOutputs;
        // size broker-enabled statement: 3 + 3 * numberOfInputs + numberOfOutputs + 4

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
        }
        // else if(receipt.statement.length == expectedBrokerSize){
        //     receiptType = 3;

        //     // TODO:: when adding new broker circuits,
        //     // you need to add the numberOfInputs and outputs here
        //     if(receipt.numberOfInputs != 2){
        //         revert InvalidNumberOfInputs();
        //     }

        //     if(receipt.numberOfOutputs != 3){
        //         revert InvalidNumberOfOutputs();
        //     }

        // }
        else if (receipt.statement.length == expectedFungibleAuditorSize) {
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
        } else if (receiptType == 3) {
            // it is a broker-enabled receipt
            if (jInputSize == 2) {
                // JoinSplit broker-enabled
                IVerifier(_verifierContractAddress).verifyProof(
                    VK_ID_ERC1155_FUNG_BROKER,
                    receipt.proof,
                    receipt.statement
                );
            } else {
                revert InvalidStatmentSize();
            }
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
