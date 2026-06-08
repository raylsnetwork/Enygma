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

    // 1-input / 2-output retail payment circuit (VK slot 0).
    uint256 public constant VK_ID_ERC20_JOINSPLIT = 0;
    // 2-input / 2-output consolidation/payment circuit (VK slot 1).
    // VULN-2 fix: 1-in and 2-in circuits have different R1CS constraint counts and
    // therefore different Groth16 VKs.  They MUST NOT share the same VK slot.
    uint256 public constant VK_ID_ERC20_JOINSPLIT_2INPUT = 1;
    uint256 public constant VK_ID_ERC20_10INPUT = 6;
    // DvP Initiator circuit: circuit id=24 in enygmadvp.config.json → VK slot 23 (0-indexed)
    uint256 public constant VK_ID_DVP_INITIATOR = 23;

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
    // params[0] = amount to deposit
    // params[1] = commitment computed off-chain as
    //             Poseidon(pk_spend, saltB_field, amount, tokenId)  (V2, non-interactive flow)
    function deposit(uint256[] memory params) public override returns (bool) {
        uint256 amount = params[0];
        uint256 commitment = params[1];

        IERC20(_assetContractAddress).transferFrom(
            msg.sender,
            address(this),
            amount
        );

        uint256[] memory commitments = new uint256[](1);
        commitments[0] = commitment;

        insertLeaves(commitments);

        emit Commitment(_vaultId, commitment);

        return true;
    }

    // depositV2 — non-interactive flow.
    // Same as deposit but also emits EncryptedNote so that the recipient can
    // scan the chain and discover notes sent to them without prior interaction.
    //
    // params[0]    = amount to deposit
    // params[1]    = commitment = Poseidon(pk_spend, saltB_field, amount, tokenId)
    // ciphertextI  = ML-KEM-768 capsule (1088 bytes)
    // ciphertextII = ChaCha20-Poly1305 ciphertext of tokenId||amount
    function depositV2(
        uint256[] memory params,
        bytes calldata ciphertextI,
        bytes calldata ciphertextII
    ) public returns (bool) {
        uint256 amount = params[0];
        uint256 commitment = params[1];

        IERC20(_assetContractAddress).transferFrom(
            msg.sender,
            address(this),
            amount
        );

        uint256[] memory commitments = new uint256[](1);
        commitments[0] = commitment;

        insertLeaves(commitments);

        emit Commitment(_vaultId, commitment);
        emit EncryptedNote(_vaultId, commitment, ciphertextI, ciphertextII);

        return true;
    }

    function transfer(
        IEnygmaDvp.ProofReceipt memory receipt
    ) public override returns (bool) {
        checkReceiptConditions(receipt);
        _insertCommitmentsFromReceipt(receipt);
        _nullifyFromReceipt(receipt);
        return true;
    }

    // transferV2 — non-interactive flow.
    // Same as transfer but also emits one EncryptedNote per output commitment
    // so that recipients can scan for notes sent to them.
    //
    // ciphertextI[i]  — ML-KEM capsule for output note i
    // ciphertextII[i] — AEAD ciphertext of tokenId||amount for output note i
    //
    // Statement layout (mirrors checkReceiptConditions):
    //   [0]           message
    //   [1..nIn]      treeNumbers
    //   [1+nIn..nIn]  merkleRoots
    //   [1+2nIn..]    nullifiers
    //   [1+3nIn..]    commitments  ← EncryptedNote emitted for each
    function transferV2(
        IEnygmaDvp.ProofReceipt memory receipt,
        bytes[] calldata ciphertextI,
        bytes[] calldata ciphertextII
    ) public returns (bool) {
        require(
            ciphertextI.length == receipt.numberOfOutputs &&
            ciphertextII.length == receipt.numberOfOutputs,
            "Erc20CoinVault: ciphertext length mismatch"
        );

        checkReceiptConditions(receipt);
        _insertCommitmentsFromReceipt(receipt);
        _nullifyFromReceipt(receipt);

        // Emit EncryptedNote for each output so recipients can scan
        uint256 commitmentsIndex = 1 + 3 * receipt.numberOfInputs;
        for (uint256 i = 0; i < receipt.numberOfOutputs; i++) {
            uint256 commitment = receipt.statement[commitmentsIndex + i];
            emit EncryptedNote(_vaultId, commitment, ciphertextI[i], ciphertextII[i]);
        }

        return true;
    }

    // withdrawV2 — non-interactive flow.
    // Output commitment encodes the recipient as pk_spend with salt=0:
    //   commitment = Poseidon4(uint160(recipient), 0, amount, tokenId)
    //
    // withdrawParams[0] = amount
    // withdrawParams[1] = tokenId (must match the value used when input notes were created)
    function withdrawV2(
        uint256[] memory withdrawParams,
        address recipient,
        IEnygmaDvp.ProofReceipt memory receipt
    ) public nonReentrant returns (bool) {
        uint256 amount  = withdrawParams[0];
        uint256 tokenId = withdrawParams[1];

        uint256 commitmentsIndex = 1 + 3 * receipt.numberOfInputs;

        uint256 commitment = IPoseidonWrapper(_hashContractAddress).poseidon4(
            [uint256(uint160(recipient)), 0, amount, tokenId]
        );

        if (receipt.statement[commitmentsIndex] != commitment) {
            revert InvalidOpening();
        }

        checkReceiptConditions(receipt);

        // Effects: nullify input coins before external transfer (checks-effects-interactions)
        uint256 treeNumbersIndex = 1;
        uint256 nullifiersIndex  = 1 + 2 * receipt.numberOfInputs;
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

        // Interaction: transfer tokens after all state changes
        IERC20(_assetContractAddress).transfer(recipient, amount);

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

        uint256 amount = withdrawParams[0];

        uint256 treeNumbersIndex = 1;
        uint256 merkleRootsIndex = 1 + receipt.numberOfInputs;
        uint256 nullifiersIndex = merkleRootsIndex + receipt.numberOfInputs;
        uint256 commitmentsIndex = nullifiersIndex + receipt.numberOfInputs;

        uint256[] memory assetParams = new uint256[](2);
        assetParams[0] = amount;
        assetParams[1] = uint256(uint160(_assetContractAddress));
        uint256 uid = generateUniqueId(assetParams);

        uint256 commitment = IPoseidonWrapper(_hashContractAddress).poseidon(
            [uid, uint256(uint160(recipient))]
        );

        if (receipt.statement[commitmentsIndex] != commitment) {
            revert InvalidOpening();
        }

        checkReceiptConditions(receipt);

        // Effects: nullify input coins before external transfer (checks-effects-interactions)
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

        // Interaction: transfer tokens after all state changes
        IERC20(_assetContractAddress).transfer(recipient, amount);

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

        // VULN-5 fix: verify the proof is bound to THIS vault contract.
        // statement layout: [...commitments, contractAddress] — contractAddress is last.
        // Only retail payment proofs (1-in/2-out and 2-in/2-out) carry StContractAddress.
        // DvP Initiator proofs (numberOfOutputs == 1) do not include it.
        if (receipt.numberOfOutputs == 2) {
            uint contractAddrIdx = jCommitmentsIndex + receipt.numberOfOutputs;
            require(
                receipt.statement[contractAddrIdx] == uint256(uint160(address(this))),
                "proof not bound to this vault"
            );
        }

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

        if (receipt.numberOfInputs != 1 && receipt.numberOfInputs != 2 && receipt.numberOfInputs != 10) {
            revert InvalidNumberOfInputs();
        }
        if (receipt.numberOfInputs == 1 && receipt.numberOfOutputs == 2) {
            // Retail Payment circuit: 1-input/2-output, VK registered at slot 0.
            IVerifier(_verifierContractAddress).verifyProof(
                VK_ID_ERC20_JOINSPLIT,
                receipt.proof,
                receipt.statement
            );
        } else if (receipt.numberOfInputs == 1) {
            // DvP Initiator: 1-input/3-output circuit with 7-element statement.
            // numberOfOutputs is reported as 1 on-chain so only statement[4]=commitB
            // is inserted into the vault; the full 7-element statement is used for VK verification.
            IVerifier(_verifierContractAddress).verifyProof(
                VK_ID_DVP_INITIATOR,
                receipt.proof,
                receipt.statement
            );
        } else if (receipt.numberOfInputs == 2) {
            // 2-input/2-output circuit uses its own VK slot — VULN-2 fix.
            IVerifier(_verifierContractAddress).verifyProof(
                VK_ID_ERC20_JOINSPLIT_2INPUT,
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
