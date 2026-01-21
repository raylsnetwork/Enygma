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

// ERC1155 single token ownership transfer circuit
// prefixes : 
// st: statement
// wt: witness
// cp: computed/component
// lo: local variables

template Erc1155FungibleTemplate(tm_nInputs, tm_mOutputs, tm_merkleTreeDepth, tm_range, tm_assetGroup_merkleTreeDepth) {

    // Statement
    signal input st_message; //public
    signal input st_treeNumbers[tm_nInputs];
    signal input st_merkleRoots[tm_nInputs];
    signal input st_nullifiers[tm_nInputs];
    signal input st_commitmentsOut[tm_mOutputs];
    signal input st_assetGroup_treeNumber;
    signal input st_assetGroup_merkleRoot;

    // Witness
    signal input wt_privateKeysIn[tm_nInputs];
    signal input wt_valuesIn[tm_nInputs];
    signal input wt_pathElements[tm_nInputs][tm_merkleTreeDepth];
    signal input wt_pathIndices[tm_nInputs];
    signal input wt_erc1155ContractAddress;
    signal input wt_erc1155TokenId;

    signal input wt_publicKeysOut[tm_mOutputs]; 
    signal input wt_valuesOut[tm_mOutputs];
    
    // has been added to support assetGroup membership proof
    signal input wt_assetGroup_pathElements[tm_assetGroup_merkleTreeDepth];
    signal input wt_assetGroup_pathIndices;

    var inputsTotal = 0;
    var outputsTotal = 0;

    // to compute publickey based on wt_privateKeysIn
    component cp_publicKeys[tm_nInputs];
    component cp_commitments[tm_nInputs];
    component cp_nullfiers[tm_nInputs];
    component cp_merkle[tm_nInputs];
    component cp_isDummyInputs[tm_nInputs];
    component cp_checkEqualIfIsNotDummys[tm_nInputs];

    component cp_uniqueIdsIn[tm_nInputs];
    component cp_uniqueIdsOut[tm_mOutputs];
    component cp_outCommitments[tm_mOutputs];

    component cp_assetGroup_merkle;
    component cp_assetGroup_uniqueId;

    // verifying merkleProof of membership of assetGroup


    // creating erc1155 uniqueId with amount = 0
    cp_assetGroup_uniqueId = Erc1155UniqueId();
    cp_assetGroup_uniqueId.erc1155ContractAddress <== wt_erc1155ContractAddress;
    cp_assetGroup_uniqueId.erc1155TokenId <== wt_erc1155TokenId;
    cp_assetGroup_uniqueId.amount <== 0;

    //verify merkleComp proof on the note commitment
    cp_assetGroup_merkle = MerkleProof(tm_assetGroup_merkleTreeDepth);
    cp_assetGroup_merkle.leaf <== cp_assetGroup_uniqueId.out;
    cp_assetGroup_merkle.pathIndices <== wt_assetGroup_pathIndices;
    for(var j=0; j< tm_assetGroup_merkleTreeDepth; j++) {
        cp_assetGroup_merkle.pathElements[j] <== wt_assetGroup_pathElements[j];
    }

    cp_assetGroup_merkle.root === st_assetGroup_merkleRoot;

    //verify input notes
    for(var i =0; i<tm_nInputs; i++){

        assert(wt_valuesIn[i] < tm_range);
        assert(0 <= wt_valuesIn[i]);

        // Generating UniqueId for the commitment
        cp_uniqueIdsIn[i] = Erc1155UniqueId();
        cp_uniqueIdsIn[i].erc1155ContractAddress <== wt_erc1155ContractAddress;
        cp_uniqueIdsIn[i].erc1155TokenId <== wt_erc1155TokenId;
        cp_uniqueIdsIn[i].amount <== wt_valuesIn[i];
                
        //derive pubkey from the privatekey
        cp_publicKeys[i] = PublicKey();
        cp_publicKeys[i].privateKey <== wt_privateKeysIn[i];

        //verify nullifier
        cp_nullfiers[i] = Nullifier();
        cp_nullfiers[i].privateKey <== wt_privateKeysIn[i];
        cp_nullfiers[i].pathIndex <== wt_pathIndices[i];
        cp_nullfiers[i].out === st_nullifiers[i];

        //compute input commitment
        cp_commitments[i] = Commitment();
        cp_commitments[i].uniqueId <== cp_uniqueIdsIn[i].out;
        cp_commitments[i].publicKey <== cp_publicKeys[i].out;

        //verify merkleComp proof on the note commitment
        cp_merkle[i] = MerkleProof(tm_merkleTreeDepth);
        cp_merkle[i].leaf <== cp_commitments[i].out;
        cp_merkle[i].pathIndices <== wt_pathIndices[i];
        for(var j=0; j< tm_merkleTreeDepth; j++) {
            cp_merkle[i].pathElements[j] <== wt_pathElements[i][j];
        }

        //dummy note if value = 0
        cp_isDummyInputs[i] = IsZero();
        cp_isDummyInputs[i].in <== wt_valuesIn[i];

        //Check merkle proof verification if NOT isDummyInputComps
        cp_checkEqualIfIsNotDummys[i] = ForceEqualIfEnabled();
        cp_checkEqualIfIsNotDummys[i].enabled <== 1-cp_isDummyInputs[i].out;
        cp_checkEqualIfIsNotDummys[i].in[0] <== st_merkleRoots[i];
        cp_checkEqualIfIsNotDummys[i].in[1] <== cp_merkle[i].root;

        inputsTotal += wt_valuesIn[i];
    }


    //verify output notes
    for(var i =0; i<tm_mOutputs; i++){
        assert(wt_valuesOut[i] < tm_range);
        assert(0 <= wt_valuesOut[i]);

        // Generating UniqueId
        cp_uniqueIdsOut[i] = Erc1155UniqueId();
        cp_uniqueIdsOut[i].erc1155ContractAddress <== wt_erc1155ContractAddress;
        cp_uniqueIdsOut[i].erc1155TokenId <== wt_erc1155TokenId;
        cp_uniqueIdsOut[i].amount <== wt_valuesOut[i];

        //verify commitment of output note
        cp_outCommitments[i] = Commitment();
        
        cp_outCommitments[i].uniqueId <== cp_uniqueIdsOut[i].out;
        cp_outCommitments[i].publicKey <== wt_publicKeysOut[i];
        cp_outCommitments[i].out === st_commitmentsOut[i];

        //accumulates output amount
        outputsTotal += wt_valuesOut[i]; //no overflow as long as tm_mOutputs is small e.g. 
    }

    //check that inputs and outputs amounts are equal
    inputsTotal === outputsTotal;
}
