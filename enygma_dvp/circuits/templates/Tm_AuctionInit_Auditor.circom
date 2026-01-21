pragma circom 2.0.0;
include "../circomlib/circuits/bitify.circom";
include "../circomlib/circuits/comparators.circom";

include "../primitives/MerkleProof.circom";
include "../primitives/Commitment.circom";
include "../primitives/Nullifier.circom";
include "../primitives/PublicKey.circom";
include "../primitives/AuctionId.circom";
include "../primitives/UniqueIdMux.circom";
include "../primitives/AuditorAccess.circom";

include "AuctionInitTemplate.circom";


template Tm_AuctionInit_Auditor(tm_numOfIdParams, tm_merkleTreeDepth, tm_assetGroup_merkleTreeDepth) {

    // encrypted values = idParams[], contractAddress
    var plainLength = tm_numOfIdParams + 1;
    var decLength = plainLength;
    while(decLength  % 3 != 0) {
        decLength += 1;
    }
    var encLength = decLength + 1;

    // Statement
    signal input st_beacon; 
    signal input st_auctionId;
    signal input st_vaultId;
    signal input st_treeNumber;
    signal input st_merkleRoot;
    signal input st_nullifier;

    // Auditor Signals
    signal input st_auditor_publicKey[2];
    signal input st_auditor_authKey[2];
    signal input st_auditor_nonce;
    signal input st_auditor_encryptedValues[encLength];
    signal input wt_auditor_random;


    signal input st_assetGroup_treeNumber;
    signal input st_assetGroup_merkleRoot;


    // Witness
    signal input wt_commitment;
    signal input wt_pathElements[tm_merkleTreeDepth];
    signal input wt_pathIndices;
    signal input wt_privateKey;
    signal input wt_idParams[tm_numOfIdParams];
    signal input wt_contractAddress;

    signal input wt_assetGroup_pathElements[tm_assetGroup_merkleTreeDepth];
    signal input wt_assetGroup_pathIndices;

    // connecting wires to Erc1155Fungible Proof
    component cp_initProof = AuctionInitTemplate(
        tm_numOfIdParams,
        tm_merkleTreeDepth, 
        tm_assetGroup_merkleTreeDepth
    );

    cp_initProof.st_beacon <== st_beacon;
    cp_initProof.st_vaultId <== st_vaultId;
    cp_initProof.st_auctionId <== st_auctionId;
    cp_initProof.st_treeNumber <== st_treeNumber;
    cp_initProof.st_merkleRoot <== st_merkleRoot;
    cp_initProof.st_nullifier <== st_nullifier;
    cp_initProof.wt_privateKey <== wt_privateKey;
    cp_initProof.wt_idParams <== wt_idParams;
    cp_initProof.wt_pathIndices <== wt_pathIndices;
    cp_initProof.wt_pathElements <== wt_pathElements;
    cp_initProof.wt_commitment <== wt_commitment;

    cp_initProof.st_assetGroup_treeNumber <== st_assetGroup_treeNumber;
    cp_initProof.st_assetGroup_merkleRoot <== st_assetGroup_merkleRoot;

    cp_initProof.wt_contractAddress <== wt_contractAddress;

    cp_initProof.wt_assetGroup_pathElements <== wt_assetGroup_pathElements;
    cp_initProof.wt_assetGroup_pathIndices <== wt_assetGroup_pathIndices;


    component cp_auditorAccess;
    cp_auditorAccess = AuditorAccess(plainLength);
    cp_auditorAccess.st_publicKey <== st_auditor_publicKey;
    cp_auditorAccess.st_nonce <== st_auditor_nonce;
    cp_auditorAccess.st_encryptedValues <== st_auditor_encryptedValues;
    cp_auditorAccess.wt_random <== wt_auditor_random;

    for(var j = 0; j< tm_numOfIdParams; j++) {
        cp_auditorAccess.wt_values[j] <== wt_idParams[j];
    }
    cp_auditorAccess.wt_values[tm_numOfIdParams] <== wt_contractAddress;  
}
