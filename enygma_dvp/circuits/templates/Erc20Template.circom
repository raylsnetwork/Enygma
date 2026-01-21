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
include "../primitives/UniqueId.circom";

// From paper:
// "It allows users to prove the correctness of
// transferring NFT ownership from an input coin to an output
// one. It checks (i) knowledge of the sender’s seed and randomness 
// for the input commitment, (ii) validity of Merkle
// proof of membership, (iii) correctness of the serial number,
// (iv) and correctness of the output coin’s commitment on the
// same NFT."

// First Erc20 JoinSplit implementation that supports the mix of
// tokens from the same ERC20 contract

template Erc20Template(tm_nInputs, tm_mOutputs, tm_merkleTreeDepth, tm_range) {

    // Statement
    signal input st_message; //public
    signal input st_treeNumbers[tm_nInputs]; //public
    signal input st_merkleRoots[tm_nInputs];
    signal input st_nullifiers[tm_nInputs];
    signal input st_commitmentsOut[tm_mOutputs];

    // Witness
    signal input wt_privateKeysIn[tm_nInputs];
    signal input wt_valuesIn[tm_nInputs];
    signal input wt_pathElements[tm_nInputs][tm_merkleTreeDepth];
    signal input wt_pathIndices[tm_nInputs];
    signal input wt_erc20ContractAddress;

    signal input wt_publicKeysOut[tm_mOutputs]; 
    signal input wt_valuesOut[tm_mOutputs];
    
    var inputsTotal = 0;
    var outputsTotal = 0;

    component cp_publicKeys[tm_nInputs];
    component cp_notesIn[tm_nInputs];
    component cp_nullfiers[tm_nInputs];
    component cp_merkles[tm_nInputs];
    component cp_isDummyInputs[tm_nInputs];
    component cp_checkEqualIfIsNotDummys[tm_nInputs];

    component cp_uniqueIdsIn[tm_nInputs];

    component cp_notesOut[tm_mOutputs];
    component cp_uniqueIdsOut[tm_mOutputs];

    //verify input notes
    for(var i =0; i<tm_nInputs; i++){

        // asserting valuesIn[i] to be in range
        assert(wt_valuesIn[i] < tm_range);
        assert(0 <= wt_valuesIn[i]);

        // Generating UniqueId
        cp_uniqueIdsIn[i] = UniqueId();
        cp_uniqueIdsIn[i].contractAddress <== wt_erc20ContractAddress;
        cp_uniqueIdsIn[i].amount <== wt_valuesIn[i];
                
        //derive pubkey from the spending key
        cp_publicKeys[i] = PublicKey();
        cp_publicKeys[i].privateKey <== wt_privateKeysIn[i];

        //verify nullifier
        cp_nullfiers[i] = Nullifier();
        cp_nullfiers[i].privateKey <== wt_privateKeysIn[i];
        cp_nullfiers[i].pathIndex <== wt_pathIndices[i];
        cp_nullfiers[i].out === st_nullifiers[i];

        //compute input commitment
        cp_notesIn[i] = Commitment();
        cp_notesIn[i].uniqueId <== cp_uniqueIdsIn[i].out;
        cp_notesIn[i].publicKey <== cp_publicKeys[i].out;

        //verify merkleComp proof on the input commitment
        cp_merkles[i] = MerkleProof(tm_merkleTreeDepth);
        cp_merkles[i].leaf <== cp_notesIn[i].out;
        cp_merkles[i].pathIndices <== wt_pathIndices[i];
        for(var j=0; j< tm_merkleTreeDepth; j++) {
            cp_merkles[i].pathElements[j] <== wt_pathElements[i][j];
        }

        //dummy note if value = 0
        cp_isDummyInputs[i] = IsZero();
        cp_isDummyInputs[i].in <== wt_valuesIn[i];

        //Check merkle proof verification if NOT cp_isDummyInputs
        cp_checkEqualIfIsNotDummys[i] = ForceEqualIfEnabled();
        cp_checkEqualIfIsNotDummys[i].enabled <== 1 - cp_isDummyInputs[i].out;
        cp_checkEqualIfIsNotDummys[i].in[0] <== st_merkleRoots[i];
        cp_checkEqualIfIsNotDummys[i].in[1] <== cp_merkles[i].root;

        inputsTotal += wt_valuesIn[i];
    }


    //verify output notes
    for(var i =0; i< tm_mOutputs; i++){
        assert(wt_valuesOut[i] < tm_range);
        assert(0 <= wt_valuesOut[i]);
        
        // Generating UniqueId
        cp_uniqueIdsOut[i] = UniqueId();
        cp_uniqueIdsOut[i].contractAddress <== wt_erc20ContractAddress;
        cp_uniqueIdsOut[i].amount <== wt_valuesOut[i];

        //verify commitment of output note
        cp_notesOut[i] = Commitment();

        // generating output uniqueId
        cp_notesOut[i].uniqueId <== cp_uniqueIdsOut[i].out;
        cp_notesOut[i].publicKey <== wt_publicKeysOut[i];
        cp_notesOut[i].out === st_commitmentsOut[i];

        //accumulates output amount
        outputsTotal += wt_valuesOut[i]; //no overflow as long as mOutputs is small e.g. 3
    }

    //check that inputs and outputs amounts are equal
    inputsTotal === outputsTotal;
}
