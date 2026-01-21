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

// From paper:
// "It allows users to prove the correctness of
// transferring NFT ownership from an input coin to an output
// one. It checks (i) knowledge of the sender’s seed and randomness 
// for the input commitment, (ii) validity of Merkle
// proof of membership, (iii) correctness of the serial number,
// (iv) and correctness of the output coin’s commitment on the
// same NFT."

template Erc721Template(tm_numOfTokens, tm_merkleTreeDepth) {

    // Statement
    signal input st_message; 
    signal input st_treeNumbers[tm_numOfTokens]; 
    signal input st_merkleRoots[tm_numOfTokens];
    signal input st_nullifiers[tm_numOfTokens];
    signal input st_commitmentsOut[tm_numOfTokens];

    // Witness
    signal input wt_privateKeysIn[tm_numOfTokens];
    signal input wt_values[tm_numOfTokens];
    signal input wt_pathElements[tm_numOfTokens][tm_merkleTreeDepth];
    signal input wt_pathIndices[tm_numOfTokens];
    signal input wt_publicKeysOut[tm_numOfTokens]; 

    component cp_publicKeys[tm_numOfTokens];
    component cp_notesIn[tm_numOfTokens];
    component cp_nullfiers[tm_numOfTokens];
    component cp_merkles[tm_numOfTokens];
    component cp_isDummyInputs[tm_numOfTokens];
    component cp_checkEqualIfIsNotDummys[tm_numOfTokens];
    component cp_notesOut[tm_numOfTokens];

    //verify input notes
    for(var i = 0; i < tm_numOfTokens; i++){
        
        //derive pubkey from the spending key
        cp_publicKeys[i] = PublicKey();
        cp_publicKeys[i].privateKey <== wt_privateKeysIn[i];

        //verify nullifier
        cp_nullfiers[i] = Nullifier();
        cp_nullfiers[i].privateKey <== wt_privateKeysIn[i];
        cp_nullfiers[i].pathIndex <== wt_pathIndices[i];
        cp_nullfiers[i].out === st_nullifiers[i];

        //compute note commitment
        cp_notesIn[i] = Commitment();
        cp_notesIn[i].uniqueId <== wt_values[i];
        cp_notesIn[i].publicKey <== cp_publicKeys[i].out;

        //verify merkleComp proof on the note commitment
        cp_merkles[i] = MerkleProof(tm_merkleTreeDepth);
        cp_merkles[i].leaf <== cp_notesIn[i].out;
        cp_merkles[i].pathIndices <== wt_pathIndices[i];
        
        for(var j=0; j< tm_merkleTreeDepth; j++) {
            cp_merkles[i].pathElements[j] <== wt_pathElements[i][j];
        }

        //dummy note if value = 0
        cp_isDummyInputs[i] = IsZero();
        cp_isDummyInputs[i].in <== wt_values[i];

        //Check merkle proof verification if NOT isDummyInputComps
        cp_checkEqualIfIsNotDummys[i] = ForceEqualIfEnabled();
        cp_checkEqualIfIsNotDummys[i].enabled <== 1 - cp_isDummyInputs[i].out;
        cp_checkEqualIfIsNotDummys[i].in[0] <== st_merkleRoots[i];
        cp_checkEqualIfIsNotDummys[i].in[1] <== cp_merkles[i].root;

        //verify commitment of output note
        cp_notesOut[i] = Commitment();
        cp_notesOut[i].uniqueId <== wt_values[i];
        cp_notesOut[i].publicKey <== wt_publicKeysOut[i];
        cp_notesOut[i].out === st_commitmentsOut[i];

    }

}
