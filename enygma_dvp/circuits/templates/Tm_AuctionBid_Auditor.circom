pragma circom 2.0.0;
include "../primitives/MerkleProof.circom";
include "../primitives/Commitment.circom";
include "../primitives/Nullifier.circom";
include "../primitives/PublicKey.circom";
include "../primitives/AuctionId.circom";
include "../primitives/UniqueIdMux.circom";
include "../circomlib/circuits/bitify.circom";

include "../circomlib/circuits/comparators.circom";
include "AuctionBidTemplate.circom";
include "../primitives/AuditorAccess.circom";


template Tm_AuctionBid_Auditor(
    tm_nInputs, 
    tm_mOutputs,
    tm_numOfIdParams,
    tm_merkleTreeDepth, 
    tm_range, 
    tm_assetGroup_merkleTreeDepth
) {

    // encrypted values = idParams[5] * (nInputs + mOutputs), contractAddress, bidValue, bidRandom
    var plainLength = tm_numOfIdParams * (tm_nInputs + tm_mOutputs) + 4;
    var decLength = plainLength;
    while(decLength  % 3 != 0) {
        decLength += 1;
    }
    var encLength = decLength + 1;

    var auctionPlainLength = 3; // encrypting only the bid amount
    var auctionDecLength = 3;
    var auctionEncLength = 4;


    // Statement
    signal input st_beacon;
    signal input st_auctionId; 
    signal input st_blindedBid;
    signal input st_vaultId; 
    signal input st_treeNumbers[tm_nInputs];
    signal input st_merkleRoots[tm_nInputs];
    signal input st_nullifiers[tm_nInputs];
    signal input st_commitmentsOut[tm_mOutputs];
    signal input st_assetGroup_treeNumber;
    signal input st_assetGroup_merkleRoot;


    // Auditor Signals
    signal input st_auctioneer_publicKey[2];
    signal input st_auctioneer_authKey[2];
    signal input st_auctioneer_nonce;
    signal input st_auctioneer_encryptedValues[auctionEncLength];
    signal input wt_auctioneer_random;


    // Auditor Signals
    signal input st_auditor_publicKey[2];
    signal input st_auditor_authKey[2];
    signal input st_auditor_nonce;
    signal input st_auditor_encryptedValues[encLength];
    signal input wt_auditor_random;

    // Witness  
    signal input wt_bidAmount;
    signal input wt_bidRandom;

    signal input wt_privateKeysIn[tm_nInputs];
    signal input wt_pathElements[tm_nInputs][tm_merkleTreeDepth];
    signal input wt_pathIndices[tm_nInputs];
    signal input wt_contractAddress;

    signal input wt_publicKeysOut[tm_mOutputs]; 

    // has been added to support assetGroup membership proof
    signal input wt_assetGroup_pathElements[tm_assetGroup_merkleTreeDepth];
    signal input wt_assetGroup_pathIndices;

    // for generic standard support
    signal input wt_idParamsIn[tm_nInputs][tm_numOfIdParams];
    signal input wt_idParamsOut[tm_mOutputs][tm_numOfIdParams];

    // connecting wires to Erc1155Fungible Proof
    component cp_bidProof = AuctionBidTemplate(
        tm_nInputs, 
        tm_mOutputs, 
        tm_numOfIdParams,
        tm_merkleTreeDepth, 
        tm_range, 
        tm_assetGroup_merkleTreeDepth
    );

    cp_bidProof.st_beacon <== st_beacon;
    cp_bidProof.st_vaultId <== st_vaultId;
    cp_bidProof.st_auctionId <== st_auctionId;
    cp_bidProof.st_blindedBid <== st_blindedBid;
    cp_bidProof.st_treeNumbers <== st_treeNumbers;
    cp_bidProof.st_merkleRoots <== st_merkleRoots;
    cp_bidProof.st_nullifiers <== st_nullifiers;
    cp_bidProof.st_commitmentsOut <== st_commitmentsOut;
    cp_bidProof.wt_privateKeysIn <== wt_privateKeysIn;
    cp_bidProof.wt_idParamsIn <== wt_idParamsIn;

    cp_bidProof.wt_idParamsOut <== wt_idParamsOut;
    cp_bidProof.wt_publicKeysOut <== wt_publicKeysOut;
    
    cp_bidProof.wt_pathIndices <== wt_pathIndices;
    cp_bidProof.wt_pathElements <== wt_pathElements;

    cp_bidProof.wt_bidRandom <== wt_bidRandom;
    cp_bidProof.wt_bidAmount <== wt_bidAmount;

    cp_bidProof.wt_contractAddress <== wt_contractAddress;

    cp_bidProof.st_assetGroup_treeNumber <== st_assetGroup_treeNumber;
    cp_bidProof.st_assetGroup_merkleRoot <== st_assetGroup_merkleRoot;
    cp_bidProof.wt_assetGroup_pathElements <== wt_assetGroup_pathElements;
    cp_bidProof.wt_assetGroup_pathIndices <== wt_assetGroup_pathIndices;


    component cp_auctioneer;
    cp_auctioneer = AuditorAccess(auctionPlainLength);
    cp_auctioneer.st_publicKey <== st_auctioneer_publicKey;
    cp_auctioneer.st_nonce <== st_auctioneer_nonce;
    cp_auctioneer.st_encryptedValues <== st_auctioneer_encryptedValues;
    cp_auctioneer.wt_random <== wt_auctioneer_random;
    cp_auctioneer.wt_values[0] <== wt_bidAmount;  
    cp_auctioneer.wt_values[1] <== wt_bidRandom;  
    cp_auctioneer.wt_values[2] <== 0;  

    component cp_auditorAccess;
    cp_auditorAccess = AuditorAccess(plainLength);
    cp_auditorAccess.st_publicKey <== st_auditor_publicKey;
    cp_auditorAccess.st_nonce <== st_auditor_nonce;
    cp_auditorAccess.st_encryptedValues <== st_auditor_encryptedValues;
    cp_auditorAccess.wt_random <== wt_auditor_random;

    for(var i = 0; i < tm_nInputs; i++) {
        for(var j = 0; j< tm_numOfIdParams; j++) {
            cp_auditorAccess.wt_values[j + i * tm_numOfIdParams] <== wt_idParamsIn[i][j];
        }
    }


    for(var i = 0; i < tm_mOutputs; i++) {
        for(var j = 0; j< tm_numOfIdParams; j++) {
            cp_auditorAccess.wt_values[j + (i * tm_numOfIdParams) + (tm_nInputs * tm_numOfIdParams)] <== wt_idParamsOut[i][j];
        }
    }

    cp_auditorAccess.wt_values[tm_numOfIdParams * (tm_nInputs + tm_mOutputs) ] <== wt_contractAddress;  
    cp_auditorAccess.wt_values[tm_numOfIdParams * (tm_nInputs + tm_mOutputs) + 1] <== wt_bidAmount;  
    cp_auditorAccess.wt_values[tm_numOfIdParams * (tm_nInputs + tm_mOutputs) + 2] <== wt_bidRandom;  
    cp_auditorAccess.wt_values[tm_numOfIdParams * (tm_nInputs + tm_mOutputs) + 3] <== 0;  
}
