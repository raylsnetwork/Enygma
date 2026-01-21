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
include "../primitives/AuditorAccess.circom";
include "Erc721Template.circom";
// From paper:
// "It allows users to prove the correctness of
// transferring NFT ownership from an input coin to an output
// one. It checks (i) knowledge of the sender’s seed and randomness 
// for the input commitment, (ii) validity of Merkle
// proof of membership, (iii) correctness of the serial number,
// (iv) and correctness of the output coin’s commitment on the
// same NFT."

template Erc721WithAuditorTemplate(tm_numOfTokens, tm_merkleTreeDepth) {
    var plainLength = tm_numOfTokens;
    var decLength = plainLength;
    while(decLength  % 3 != 0) {
        decLength += 1;
    }
    var encLength = decLength + 1;
    // Statement
    signal input st_message; 
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


    // Witness
    signal input wt_privateKeysIn[tm_numOfTokens];
    signal input wt_values[tm_numOfTokens];
    signal input wt_pathElements[tm_numOfTokens][tm_merkleTreeDepth];
    signal input wt_pathIndices[tm_numOfTokens];
    signal input wt_publicKeysOut[tm_numOfTokens]; 


    // connecting wires to Erc1155Fungible Proof
    component cp_coinProof = Erc721Template(
        tm_numOfTokens, 
        tm_merkleTreeDepth 
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

    component cp_auditorAccess;
    cp_auditorAccess = AuditorAccess(tm_numOfTokens);
    cp_auditorAccess.st_publicKey <== st_auditor_publicKey;
    cp_auditorAccess.st_nonce <== st_auditor_nonce;
    cp_auditorAccess.st_encryptedValues <== st_auditor_encryptedValues;
    cp_auditorAccess.wt_random <== wt_auditor_random;

    cp_auditorAccess.wt_values <== wt_values;
    
}
