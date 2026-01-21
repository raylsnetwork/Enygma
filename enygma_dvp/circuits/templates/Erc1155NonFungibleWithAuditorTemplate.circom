pragma circom 2.0.0;
include "../circomlib/circuits/comparators.circom";
include "../circomlib/circuits/babyjub.circom";
include "../circomlib/circuits/poseidon.circom";
include "../circomlib/circuits/gates.circom";
include "../circomlib/circuits/mux1.circom";
include "../primitives/MerkleProof.circom";
include "../primitives/Commitment.circom";
include "../primitives/Nullifier.circom";
include "../primitives/PublicKey.circom";
include "../primitives/Erc1155UniqueId.circom";
include "../primitives/AuditorAccess.circom";
include "Erc1155NonFungibleTemplate.circom";

// ERC1155 single token ownership transfer circuit
// prefixes : 
// st: statement
// wt: witness
// cp: computed/component
// lo: local variables

template Erc1155NonFungibleWithAuditorTemplate(tm_numOfTokens, tm_merkleTreeDepth, tm_assetGroup_merkleTreeDepth) {
    var plainLength = tm_numOfTokens * 2 + 1;
    var decLength = plainLength;
    while(decLength  % 3 != 0) {
        decLength += 1;
    }
    var encLength = decLength + 1;

    // Statement
    signal input st_message; //public
    signal input st_treeNumbers[tm_numOfTokens];
    signal input st_merkleRoots[tm_numOfTokens];
    signal input st_nullifiers[tm_numOfTokens];
    signal input st_commitmentsOut[tm_numOfTokens];

    // Auditor Signals
    signal input st_auditor_publicKey[2];
    signal input st_auditor_authKey[2];
    signal input st_auditor_nonce;
    signal input st_auditor_encryptedValues[encLength];
    signal input wt_auditor_random;

    // AssetGroup Signals
    signal input st_assetGroup_treeNumbers[tm_numOfTokens];
    signal input st_assetGroup_merkleRoots[tm_numOfTokens];

    // Witness
    signal input wt_privateKeysIn[tm_numOfTokens];
    signal input wt_values[tm_numOfTokens];
    signal input wt_pathElements[tm_numOfTokens][tm_merkleTreeDepth];
    signal input wt_pathIndices[tm_numOfTokens];
    signal input wt_erc1155TokenIds[tm_numOfTokens];
    signal input wt_erc1155ContractAddress;

    signal input wt_publicKeysOut[tm_numOfTokens]; 
    
    // has been added to support assetGroup membership proof
    signal input wt_assetGroup_pathElements[tm_numOfTokens][tm_assetGroup_merkleTreeDepth];
    signal input wt_assetGroup_pathIndices[tm_numOfTokens];
    

    // connecting wires to Erc1155Fungible Proof
    component cp_coinProof = Erc1155NonFungibleTemplate(
        tm_numOfTokens, 
        tm_merkleTreeDepth, 
        tm_assetGroup_merkleTreeDepth
    );

    cp_coinProof.st_message <== st_message;
    cp_coinProof.st_treeNumbers <== st_treeNumbers;
    cp_coinProof.st_merkleRoots <== st_merkleRoots;
    cp_coinProof.st_nullifiers <== st_nullifiers;
    cp_coinProof.wt_privateKeysIn <== wt_privateKeysIn;
    cp_coinProof.wt_values <== wt_values;
    cp_coinProof.wt_pathIndices <== wt_pathIndices;
    cp_coinProof.wt_pathElements <== wt_pathElements;

    cp_coinProof.st_commitmentsOut <== st_commitmentsOut;
    cp_coinProof.wt_publicKeysOut <== wt_publicKeysOut; 

    cp_coinProof.st_assetGroup_treeNumbers <== st_assetGroup_treeNumbers;
    cp_coinProof.st_assetGroup_merkleRoots <== st_assetGroup_merkleRoots;

    cp_coinProof.wt_erc1155ContractAddress <== wt_erc1155ContractAddress;
    cp_coinProof.wt_erc1155TokenIds <== wt_erc1155TokenIds;

    cp_coinProof.wt_assetGroup_pathElements <== wt_assetGroup_pathElements;
    cp_coinProof.wt_assetGroup_pathIndices <== wt_assetGroup_pathIndices;


    component cp_auditorAccess;
    cp_auditorAccess = AuditorAccess(plainLength);
    cp_auditorAccess.st_publicKey <== st_auditor_publicKey;
    cp_auditorAccess.st_nonce <== st_auditor_nonce;
    cp_auditorAccess.st_encryptedValues <== st_auditor_encryptedValues;
    cp_auditorAccess.wt_random <== wt_auditor_random;

    for(var j = 0; j< tm_numOfTokens; j++) {
        cp_auditorAccess.wt_values[j] <== wt_values[j];
    }
    for(var j = 0; j< tm_numOfTokens; j++) {
        cp_auditorAccess.wt_values[j + tm_numOfTokens] <== wt_erc1155TokenIds[j];
    }

    cp_auditorAccess.wt_values[tm_numOfTokens * 2] <== wt_erc1155ContractAddress;    
}
