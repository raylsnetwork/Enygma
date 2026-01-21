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
include "../primitives/Blinder.circom";

// ERC1155 single token ownership transfer circuit
// prefixes : 
// st: statement
// wt: witness
// cp: computed/component
// lo: local variables

template Erc1155FungibleWithBrokerV1Template(nInputs, mOutputs, MerkleTreeDepth, range, assetGroupMerkleTreeDepth, maxPermittedCommissionRate, commissionRateDecimals) {

    // Statement
    signal input st_message; //public
    signal input st_treeNumbers[nInputs];
    signal input st_merkleRoots[nInputs];
    signal input st_nullifiers[nInputs];
    signal input st_commitmentsOut[mOutputs];
    signal input st_broker_blindedPublicKey;
    signal input st_broker_commissionRate;
    signal input st_assetGroup_treeNumber;
    signal input st_assetGroup_merkleRoot;

    // Witness
    signal input wt_privateKeys[nInputs];
    signal input wt_valuesIn[nInputs];
    signal input wt_pathElements[nInputs][MerkleTreeDepth];
    signal input wt_pathIndices[nInputs];
    signal input wt_erc1155ContractAddress;
    signal input wt_erc1155TokenId;

    signal input wt_recipientPK[mOutputs]; 
    signal input wt_valuesOut[mOutputs];
    
    // has been added to support assetGroup membership proof
    signal input wt_assetGroup_pathElements[assetGroupMerkleTreeDepth];
    signal input wt_assetGroup_pathIndices;

    var inputsTotal = 0;
    var outputsTotal = 0;

    // to compute publickey based on wt_privateKeys
    component cp_publicKeys[nInputs];
    component cp_commitments[nInputs];
    component cp_nullfiers[nInputs];
    component cp_merkle[nInputs];
    component cp_isDummyInputs[nInputs];
    component cp_checkEqualIfIsNotDummys[nInputs];

    component cp_uniqueIds[nInputs];
    component cp_outUniqueIds[mOutputs];
    component cp_outCommitments[mOutputs];

    component cp_assetGroup_merkle;
    component cp_assetGroup_uniqueId;

    component cp_broker_blinder;

    cp_broker_blinder = Blinder();
    cp_broker_blinder.in <== wt_recipientPK[2];
    cp_broker_blinder.out === st_broker_blindedPublicKey;


    // verifying merkleProof of membership of assetGroup
    // creating erc1155 uniqueId with amount = 0
    cp_assetGroup_uniqueId = Erc1155UniqueId();
    cp_assetGroup_uniqueId.erc1155ContractAddress <== wt_erc1155ContractAddress;
    cp_assetGroup_uniqueId.erc1155TokenId <== wt_erc1155TokenId;
    cp_assetGroup_uniqueId.amount <== 0;

    //verify merkleComp proof on the note commitment
    cp_assetGroup_merkle = MerkleProof(assetGroupMerkleTreeDepth);
    cp_assetGroup_merkle.leaf <== cp_assetGroup_uniqueId.out;
    cp_assetGroup_merkle.pathIndices <== wt_assetGroup_pathIndices;
    for(var j=0; j< assetGroupMerkleTreeDepth; j++) {
        cp_assetGroup_merkle.pathElements[j] <== wt_assetGroup_pathElements[j];
    }

    cp_assetGroup_merkle.root === st_assetGroup_merkleRoot;

    //verify input notes
    for(var i =0; i<nInputs; i++){

        assert(wt_valuesIn[i] < range);
        assert(0 <= wt_valuesIn[i]);

        // Generating UniqueId for the commitment
        cp_uniqueIds[i] = Erc1155UniqueId();
        cp_uniqueIds[i].erc1155ContractAddress <== wt_erc1155ContractAddress;
        cp_uniqueIds[i].erc1155TokenId <== wt_erc1155TokenId;
        cp_uniqueIds[i].amount <== wt_valuesIn[i];
                
        //derive pubkey from the privatekey
        cp_publicKeys[i] = PublicKey();
        cp_publicKeys[i].privateKey <== wt_privateKeys[i];

        //verify nullifier
        cp_nullfiers[i] = Nullifier();
        cp_nullfiers[i].privateKey <== wt_privateKeys[i];
        cp_nullfiers[i].pathIndex <== wt_pathIndices[i];
        cp_nullfiers[i].out === st_nullifiers[i];

        //compute input commitment
        cp_commitments[i] = Commitment();
        cp_commitments[i].uniqueId <== cp_uniqueIds[i].out;
        cp_commitments[i].publicKey <== cp_publicKeys[i].out;

        //verify merkleComp proof on the note commitment
        cp_merkle[i] = MerkleProof(MerkleTreeDepth);
        cp_merkle[i].leaf <== cp_commitments[i].out;
        cp_merkle[i].pathIndices <== wt_pathIndices[i];
        for(var j=0; j< MerkleTreeDepth; j++) {
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

    assert(st_broker_commissionRate <= maxPermittedCommissionRate);
    assert(
        wt_valuesOut[2] == (wt_valuesOut[0] * st_broker_commissionRate) \ ( 10 ** commissionRateDecimals)) ;


    //verify output notes
    for(var i =0; i<mOutputs; i++){
        assert(wt_valuesOut[i] < range);
        assert(0 <= wt_valuesOut[i]);

        // Generating UniqueId
        cp_outUniqueIds[i] = Erc1155UniqueId();
        cp_outUniqueIds[i].erc1155ContractAddress <== wt_erc1155ContractAddress;
        cp_outUniqueIds[i].erc1155TokenId <== wt_erc1155TokenId;
        cp_outUniqueIds[i].amount <== wt_valuesOut[i];

        //verify commitment of output note
        cp_outCommitments[i] = Commitment();
        
        cp_outCommitments[i].uniqueId <== cp_outUniqueIds[i].out;
        cp_outCommitments[i].publicKey <== wt_recipientPK[i];
        cp_outCommitments[i].out === st_commitmentsOut[i];

        //accumulates output amount
        outputsTotal += wt_valuesOut[i]; //no overflow as long as mOutputs is small e.g. 
    }

    //check that inputs and outputs amounts are equal
    inputsTotal === outputsTotal;
}
