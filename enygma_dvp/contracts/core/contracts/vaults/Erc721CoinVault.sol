// Copyright 2024-2025, Parity Holding Ltd.
// SPDX-License-Identifier: BUSL-1.1

pragma solidity ^0.8.0;
// pragma abicoder v2;

import {IERC721} from "@openzeppelin/contracts/token/ERC721/IERC721.sol";
import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";

import {IZkDvp} from "../../interfaces/IZkDvp.sol";
import {IPoseidonWrapper} from "../../interfaces/IPoseidonWrapper.sol";
import {IVerifier} from "../../interfaces/IVerifier.sol";
import {AbstractCoinVault} from "./AbstractCoinVault.sol";


contract Erc721CoinVault is AbstractCoinVault {

    ///////////////////////////////////////////////
    //              Constants
    //////////////////////////////////////////////

    uint256 constant public VK_ID_ERC721_1 = 1;
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
        // _setupRole(DEFAULT_OWNER_ROLE, msg.sender);
    }

    // Standards that are currently supported: ERC20, ERC721, ERC1155
    function deposit(
        uint256[] memory params
    ) public override returns(bool) {


        // Transferring the ERC20 tokens from User to ZkDvp
        uint256 tokenId = params[0];
        uint256 publicKey = params[1];
        IERC721(_assetContractAddress).transferFrom(msg.sender, address(this), tokenId);

        uint256[] memory assetParams = new uint256[](2);
        assetParams[0] = tokenId;
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
        IZkDvp.ProofReceipt memory receipt
    ) public override returns (bool){

        
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
        IZkDvp.ProofReceipt memory receipt
    ) public override returns(bool) {
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

        if(receipt.statement[commitmentsIndex] != commitment){
            revert InvalidOpening();
        }

        // checking generic JoinSplit proof conditions

        checkReceiptConditions(receipt);
        
        // Transfering the tokens from ZkDvp to User
        IERC721(_assetContractAddress).transferFrom(address(this), recipient, amount);

        // Nullifying the input coins
        for(uint256 i = 0;i< receipt.numberOfInputs; i++){
            if (receipt.statement[nullifiersIndex + i] != 0) {
                setNullifier(
                    receipt.statement[treeNumbersIndex + i], 
                    receipt.statement[nullifiersIndex + i]
                );
                 emit Nullifier(_vaultId, 
                            receipt.statement[treeNumbersIndex + i], 
                            receipt.statement[nullifiersIndex + i]);
            }

        }

        return true;
    }

    function verifyOwnership(
        uint256[] memory params_,
        IZkDvp.ProofReceipt memory receipt_
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

        IZkDvp(_zkDvpContractAddress).checkAndRegisterChallenge(challenge);
        

        uint256[] memory uparams = new uint256[](1);
        uparams[0] = nftId;
        // regenerating uniqueId and commitment to verify
        uint256 uid = generateUniqueId(uparams);

        // re-computing commitment to verify
        uint256 commitment = IPoseidonWrapper(_hashContractAddress).poseidon(
            [uid, uint256(uint160(0))]
        );

        // verifying the corectness of the commitment
        if(receipt_.statement[4] != commitment){
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
    ) public override view returns(uint256) {
        uint256 nftId = params[0];
        return
            IPoseidonWrapper(_hashContractAddress).poseidon(
                [uint256(uint160(_assetContractAddress)), nftId]
            );
    }

    function checkReceiptConditions(
        IZkDvp.ProofReceipt memory receipt
    ) public override view returns(bool) {

        // ownReceipt.inputs:
        // 0 message;
        // 1 treeNumber;
        // 2 merkleRoot;
        // 3 nullifier;
        // 4 commitment;

        if(!isValidRoot(
                        receipt.statement[1],
                        receipt.statement[2]
                    )){
            revert InvalidMerkleRoot();
        }

        if(isValidNullifier(
                receipt.statement[1],
                receipt.statement[3]
            )){
            revert InvalidNullifier();
        }

        IVerifier(_verifierContractAddress).verifyProof(
            VK_ID_ERC721_1, receipt.proof, receipt.statement
        );
        return true;
    }


}
