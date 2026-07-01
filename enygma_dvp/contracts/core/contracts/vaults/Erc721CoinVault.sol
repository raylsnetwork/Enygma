// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;
// pragma abicoder v2;

import {IERC721} from "@openzeppelin/contracts/token/ERC721/IERC721.sol";
import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";

import {IEnygmaDvp} from "../../interfaces/IEnygmaDvp.sol";
import {IPoseidonWrapper} from "../../interfaces/IPoseidonWrapper.sol";
import {IVerifier} from "../../interfaces/IVerifier.sol";
import {AbstractCoinVault} from "./AbstractCoinVault.sol";

contract Erc721CoinVault is AbstractCoinVault {
    ///////////////////////////////////////////////
    //              Constants
    //////////////////////////////////////////////

    uint256 public constant VK_ID_ERC721_1 = 1;
    // DvP Destination circuit: circuit id=25 in enygmadvp.config.json → VK slot 24 (0-indexed)
    uint256 public constant VK_ID_DVP_DESTINATION = 24;
    ///////////////////////////////////////////////
    //              Constructor
    //////////////////////////////////////////////

    constructor(
        address zkDvpContractAddress
    ) AbstractCoinVault(zkDvpContractAddress) {
        _name = "ZkDvp - ERC20 Coin Vault";
        // _setupRole(DEFAULT_OWNER_ROLE, msg.sender);
    }

    // C-1 fix: commitment is now computed on-chain from the caller-supplied key material
    // so the actual deposited tokenId is binding.  Previously the caller could pass any
    // commitment encoding a different tokenId, allowing theft of any other NFT in the vault.
    //
    // params[0] = tokenId to deposit
    // params[1] = pkSpend — recipient's BabyJubJub spend public key (x-coordinate)
    // params[2] = salt    — random field element chosen by the depositor
    // Commitment = Poseidon4(pkSpend, salt, 1, tokenId) — matches Erc721Commitment circuit formula.
    // Amount is always 1 for non-fungible tokens.
    function deposit(uint256[] memory params) public override returns (bool) {
        uint256 tokenId = params[0];
        uint256 pkSpend = params[1];
        uint256 salt    = params[2];

        IERC721(_assetContractAddress).transferFrom(
            msg.sender,
            address(this),
            tokenId
        );

        // Commitment = Poseidon4(pkSpend, salt, 1, tokenId) — amount=1 for NFTs.
        uint256 commitment = IPoseidonWrapper(_hashContractAddress).poseidon4(
            [pkSpend, salt, 1, tokenId]
        );

        uint256[] memory commitments = new uint256[](1);
        commitments[0] = commitment;

        insertLeaves(commitments);

        emit Commitment(_vaultId, commitment);

        return true;
    }

    function transfer(
        IEnygmaDvp.ProofReceipt memory receipt
    ) public override nonReentrant returns (bool) {
        checkReceiptConditions(receipt);
        // NEW-1 fix: nullify inputs before inserting outputs (CEI).
        // Inserting first opened a reentrancy window via the Poseidon precompile
        // inside insertLeaves(); a re-entrant call would find the input notes
        // un-nullified and could double-spend them.
        _nullifyFromReceipt(receipt);
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

        // Interaction: transfer token after all state changes
        IERC721(_assetContractAddress).transferFrom(
            address(this),
            recipient,
            amount
        );

        return true;
    }

    function verifyOwnership(
        uint256[] memory params_,
        IEnygmaDvp.ProofReceipt memory receipt_
    ) public returns (bool) {
        // params:
        // 0: nftId
        // 1: challenge
        uint256 nftId = params_[0];

        // receipt.statement:
        // 0 challenge;
        // 1 treeNumber;
        // 2 merkleRoot;
        // 3 nullifier;
        // 4 commitment;
        uint256 challenge = receipt_.statement[0];

        IEnygmaDvp(_zkDvpContractAddress).checkAndRegisterChallenge(challenge);

        uint256[] memory uparams = new uint256[](1);
        uparams[0] = nftId;
        // regenerating uniqueId and commitment to verify
        uint256 uid = generateUniqueId(uparams);

        // re-computing commitment to verify
        uint256 commitment = IPoseidonWrapper(_hashContractAddress).poseidon(
            [uid, uint256(uint160(0))]
        );

        // verifying the corectness of the commitment
        if (receipt_.statement[4] != commitment) {
            revert InvalidOpening();
        }

        // checking generic conditions of Ownership receipt.

        checkReceiptConditions(receipt_);
        emit OwnershipVerificationReceipt(
            challenge,
            _vaultId,
            nftId,
            1 // the amount of Nft is always 1
        );
        return true;
    }
    ///////////////////////////////////////////////
    //       Generic functions
    //////////////////////////////////////////////
    function generateUniqueId(
        uint256[] memory params
    ) public view override returns (uint256) {
        uint256 nftId = params[0];
        return
            IPoseidonWrapper(_hashContractAddress).poseidon(
                [uint256(uint160(_assetContractAddress)), nftId]
            );
    }

    function checkReceiptConditions(
        IEnygmaDvp.ProofReceipt memory receipt
    ) public view override returns (bool) {
        // Statement layout (ERC721 Ownership and DvP Destination share the same 5-element structure):
        // 0 message;
        // 1 treeNumber;
        // 2 merkleRoot;
        // 3 nullifier;
        // 4 commitment (commitA for DvP Destination);

        if (!isValidRoot(receipt.statement[1], receipt.statement[2])) {
            revert InvalidMerkleRoot();
        }

        if (isValidNullifier(receipt.statement[1], receipt.statement[3])) {
            revert InvalidNullifier();
        }

        // HIGH-3 fix: use StMessage (statement[0]) as a proof-type discriminator
        // instead of try/catch VK fallthrough.
        // StMessage == 0  → standalone ERC721 transfer (retail circuit).
        // StMessage != 0  → DvP Destination proof (swap_id is non-zero by construction).
        // The try/catch pattern silently accepted cross-circuit proofs: a DvP Destination
        // proof that failed VK_ID_ERC721_1 would be re-verified against VK_ID_DVP_DESTINATION,
        // allowing semantic confusion between the two circuit types.
        if (receipt.statement[0] == 0) {
            IVerifier(_verifierContractAddress).verifyProof(
                VK_ID_ERC721_1,
                receipt.proof,
                receipt.statement
            );
        } else {
            IVerifier(_verifierContractAddress).verifyProof(
                VK_ID_DVP_DESTINATION,
                receipt.proof,
                receipt.statement
            );
        }
        return true;
    }
}
