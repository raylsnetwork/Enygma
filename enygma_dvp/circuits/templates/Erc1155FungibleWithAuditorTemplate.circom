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
include "Erc1155FungibleTemplate.circom";

// ERC1155 single token ownership transfer circuit
// prefixes : 
// st: statement
// wt: witness
// cp: computed/component
// lo: local variables

template Erc1155FungibleWithAuditorTemplate(
    tm_nInputs, 
    tm_mOutputs, 
    tm_merkleTreeDepth, 
    tm_range, 
    tm_assetGroup_merkleTreeDepth
) {
    var plainLength = tm_nInputs + tm_mOutputs + 2;
    var decLength = plainLength;
    while(decLength  % 3 != 0) {
        decLength += 1;
    }
    var encLength = decLength + 1;

    // Statement
    signal input st_message; //public
    signal input st_treeNumbers[tm_nInputs];
    signal input st_merkleRoots[tm_nInputs];
    signal input st_nullifiers[tm_nInputs];
    signal input st_commitmentsOut[tm_mOutputs];

    // Auditor Signals
    signal input st_auditor_publicKey[2];
    signal input st_auditor_authKey[2];
    signal input st_auditor_nonce;
    signal input st_auditor_encryptedValues[encLength];
    signal input wt_auditor_random;


    // AssetGroup Signals
    signal input st_assetGroup_treeNumber;
    signal input st_assetGroup_merkleRoot;

    // Witness
    signal input wt_privateKeysIn[tm_nInputs];
    signal input wt_valuesIn[tm_nInputs];
    signal input wt_pathElements[tm_nInputs][tm_merkleTreeDepth];
    signal input wt_pathIndices[tm_nInputs];
    signal input wt_erc1155TokenId;
    signal input wt_erc1155ContractAddress;

    signal input wt_publicKeysOut[tm_mOutputs]; 
    signal input wt_valuesOut[tm_mOutputs];
    
    // has been added to support assetGroup membership proof
    signal input wt_assetGroup_pathElements[tm_assetGroup_merkleTreeDepth];
    signal input wt_assetGroup_pathIndices;
    

    // connecting wires to Erc1155Fungible Proof
    component cp_coinProof = Erc1155FungibleTemplate(
        tm_nInputs, 
        tm_mOutputs, 
        tm_merkleTreeDepth, 
        tm_range, 
        tm_assetGroup_merkleTreeDepth
    );

    cp_coinProof.st_message <== st_message;
    cp_coinProof.st_treeNumbers <== st_treeNumbers;
    cp_coinProof.st_merkleRoots <== st_merkleRoots;
    cp_coinProof.st_nullifiers <== st_nullifiers;
    cp_coinProof.wt_privateKeysIn <== wt_privateKeysIn;
    cp_coinProof.wt_valuesIn <== wt_valuesIn;
    cp_coinProof.wt_pathIndices <== wt_pathIndices;
    cp_coinProof.wt_pathElements <== wt_pathElements;

    cp_coinProof.st_commitmentsOut <== st_commitmentsOut;
    cp_coinProof.wt_publicKeysOut <== wt_publicKeysOut; 
    cp_coinProof.wt_valuesOut <== wt_valuesOut;

    cp_coinProof.st_assetGroup_treeNumber <== st_assetGroup_treeNumber;
    cp_coinProof.st_assetGroup_merkleRoot <== st_assetGroup_merkleRoot;

    cp_coinProof.wt_erc1155TokenId <== wt_erc1155TokenId;
    cp_coinProof.wt_erc1155ContractAddress <== wt_erc1155ContractAddress;

    cp_coinProof.wt_assetGroup_pathElements <== wt_assetGroup_pathElements;
    cp_coinProof.wt_assetGroup_pathIndices <== wt_assetGroup_pathIndices;


    component cp_auditorAccess;
    cp_auditorAccess = AuditorAccess(plainLength);
    cp_auditorAccess.st_publicKey <== st_auditor_publicKey;
    cp_auditorAccess.st_nonce <== st_auditor_nonce;
    cp_auditorAccess.st_encryptedValues <== st_auditor_encryptedValues;
    cp_auditorAccess.wt_random <== wt_auditor_random;

    for(var j = 0; j< tm_nInputs; j++) {
        cp_auditorAccess.wt_values[j] <== wt_valuesIn[j];
    }
    for(var j = 0; j< tm_mOutputs; j++) {
        cp_auditorAccess.wt_values[j + tm_nInputs] <== wt_valuesOut[j];
    }

    cp_auditorAccess.wt_values[tm_nInputs + tm_mOutputs] <== wt_erc1155TokenId;  
    cp_auditorAccess.wt_values[tm_nInputs + tm_mOutputs + 1] <== wt_erc1155ContractAddress;  


}
