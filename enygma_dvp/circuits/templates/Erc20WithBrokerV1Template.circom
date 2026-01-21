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
include "../primitives/Blinder.circom";

template Erc20WithBrokerV1Template(
    tm_nInputs, 
    tm_mOutputs, 
    tm_merkleTreeDepth, 
    tm_range, 
    tm_maxCommissionPercentage, 
    tm_commissionPercentageDecimals
) {

    // Statement
    signal input st_message;
    signal input st_treeNumbers[tm_nInputs];
    signal input st_merkleRoots[tm_nInputs];
    signal input st_nullifiers[tm_nInputs];
    signal input st_commitmentsOut[tm_mOutputs];
    signal input st_broker_blindedPublicKey;
    signal input st_broker_commissionPercentage;


    // st_brokerageFee <= (BrokeragePercentagetm_range * first coin's value) \ 100

    // Witness
    signal input wt_privateKeys[tm_nInputs];
    signal input wt_valuesIn[tm_nInputs];
    signal input wt_pathElements[tm_nInputs][tm_merkleTreeDepth];
    signal input wt_pathIndices[tm_nInputs];
    signal input wt_erc20ContractAddress;

    signal input wt_publicKeysOut[tm_mOutputs]; 
    signal input wt_valuesOut[tm_mOutputs];
    

    // we need at least three output coins for broker-enabled settlement.
    assert(tm_mOutputs == 3);

    var inputsTotal = 0;
    var outputsTotal = 0;

    component cp_publicKeys[tm_nInputs];
    component cp_uniqueIdsIn[tm_nInputs];
    component cp_commitmentsIn[tm_nInputs];
    component cp_nullfiers[tm_nInputs];
    component cp_merkles[tm_nInputs];
    component cp_isDummyInputs[tm_nInputs];
    component cp_checkEqualIfIsNotDummys[tm_nInputs];

    component cp_uniqueIdsOut[tm_mOutputs];
    component cp_commitmentsOut[tm_mOutputs];

    component cp_broker_blinder;

    // checking the third output to match with broker's publicKey
    cp_broker_blinder = Blinder();
    cp_broker_blinder.in <== wt_publicKeysOut[2];
    cp_broker_blinder.out === st_broker_blindedPublicKey;


    // checking the brokerage fee < delegator's coin's value
    assert(wt_valuesOut[2] < wt_valuesOut[0]);

    // verifying that brokerageFee is the same as the first idParams
    //st_brokerageFee === wt_valuesOut[2];
    // assert(st_brokerageFee <= (brokeragePercentagetm_range * wt_delegator_idParams[0]) \ 100;

    //verify input notes
    for(var i =0; i<tm_nInputs; i++){

        // asserting valuesIn[i] to be in tm_range
        assert(wt_valuesIn[i] < tm_range);
        assert(0 <= wt_valuesIn[i]);

        // Generating UniqueId
        cp_uniqueIdsIn[i] = UniqueId();
        cp_uniqueIdsIn[i].contractAddress <== wt_erc20ContractAddress;
        cp_uniqueIdsIn[i].amount <== wt_valuesIn[i];
                
        //derive pubkey from the spending key
        cp_publicKeys[i] = PublicKey();
        cp_publicKeys[i].privateKey <== wt_privateKeys[i];

        //verify nullifier
        cp_nullfiers[i] = Nullifier();
        cp_nullfiers[i].privateKey <== wt_privateKeys[i];
        cp_nullfiers[i].pathIndex <== wt_pathIndices[i];
        cp_nullfiers[i].out === st_nullifiers[i];

        //compute input commitment
        cp_commitmentsIn[i] = Commitment();
        cp_commitmentsIn[i].uniqueId <== cp_uniqueIdsIn[i].out;
        cp_commitmentsIn[i].publicKey <== cp_publicKeys[i].out;

        //verify merkleComp proof on the input commitment
        cp_merkles[i] = MerkleProof(tm_merkleTreeDepth);
        cp_merkles[i].leaf <== cp_commitmentsIn[i].out;
        cp_merkles[i].pathIndices <== wt_pathIndices[i];
        for(var j=0; j< tm_merkleTreeDepth; j++) {
            cp_merkles[i].pathElements[j] <== wt_pathElements[i][j];
        }

        //dummy note if value = 0
        cp_isDummyInputs[i] = IsZero();
        cp_isDummyInputs[i].in <== wt_valuesIn[i];

        //Check merkle proof verification if NOT isDummyInputComps
        cp_checkEqualIfIsNotDummys[i] = ForceEqualIfEnabled();
        cp_checkEqualIfIsNotDummys[i].enabled <== 1-cp_isDummyInputs[i].out;
        cp_checkEqualIfIsNotDummys[i].in[0] <== st_merkleRoots[i];
        cp_checkEqualIfIsNotDummys[i].in[1] <== cp_merkles[i].root;

        inputsTotal += wt_valuesIn[i];
    }


    assert(st_broker_commissionPercentage <= tm_maxCommissionPercentage);
    assert(
        wt_valuesOut[2] == (wt_valuesOut[0] * st_broker_commissionPercentage) \ ( 10 ** (2+tm_commissionPercentageDecimals))) ;


    //verify output notes
    for(var i =0; i<tm_mOutputs; i++){
        assert(wt_valuesOut[i] < tm_range);
        assert(0 <= wt_valuesOut[i]);
        
        // Generating UniqueId
        cp_uniqueIdsOut[i] = UniqueId();
        cp_uniqueIdsOut[i].contractAddress <== wt_erc20ContractAddress;
        cp_uniqueIdsOut[i].amount <== wt_valuesOut[i];

        //verify commitment of output note
        cp_commitmentsOut[i] = Commitment();

        // generating output uniqueId
        cp_commitmentsOut[i].uniqueId <== cp_uniqueIdsOut[i].out;
        cp_commitmentsOut[i].publicKey <== wt_publicKeysOut[i];
        cp_commitmentsOut[i].out === st_commitmentsOut[i];

        //accumulates output amount
        outputsTotal += wt_valuesOut[i]; //no overflow as long as tm_mOutputs is small e.g. 3
    }

    //check that inputs and outputs amounts are equal
    inputsTotal === outputsTotal;
}
