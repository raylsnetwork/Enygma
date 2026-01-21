pragma circom 2.0.0;
include "../primitives/MerkleProof.circom";
include "../primitives/Commitment.circom";
include "../primitives/Nullifier.circom";
include "../primitives/PublicKey.circom";
include "../primitives/AuctionId.circom";
include "../primitives/UniqueIdMux.circom";
include "../primitives/Pedersen.circom";

template AuctionBidTemplate(
    tm_nInputs, 
    tm_mOutputs, 
    tm_numOfIdParams,
    tm_merkleTreeDepth, 
    tm_range, 
    tm_assetGroup_merkleTreeDepth
) {

    // Statement
    signal input st_beacon;
    signal input st_auctionId; 
    signal input st_blindedBid;
    signal input st_vaultId; 
    signal input st_treeNumbers[tm_nInputs];
    signal input st_merkleRoots[tm_nInputs];
    signal input st_nullifiers[tm_nInputs];
    signal input st_commitmentsOut[tm_nInputs];
    signal input st_assetGroup_treeNumber;
    signal input st_assetGroup_merkleRoot;

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

    // Checking BidAmount is in fungible tm_range
    assert(wt_bidAmount < tm_range);
    assert(0 < wt_bidAmount);

    var inputsTotal = 0;
    var outputsTotal = 0;

    component cp_pedersen;

    cp_pedersen = Pedersen();
    cp_pedersen.amount <== wt_bidAmount;
    cp_pedersen.random <== wt_bidRandom;
    cp_pedersen.out === st_blindedBid;

    component cp_publicKeys[tm_nInputs];
    component cp_commitmentsIn[tm_nInputs];
    component cp_nullifiers[tm_nInputs];
    component cp_merkles[tm_nInputs];

    // TODO:: connect dummy input circuits
    component cp_isDummyInputs[tm_nInputs];
    component cp_checkEqualIfIsNotDummys[tm_nInputs];

    component cp_inUniqueIds[tm_nInputs];
    component cp_outUniqueIds[tm_mOutputs];

    component cp_assetGroup_uniqueId;
    component cp_assetGroup_merkle;
    component cp_assetGroup_isDummy;
    component cp_assetGroup_checkRoot;

    cp_assetGroup_uniqueId = UniqueIdMux(tm_numOfIdParams);
    cp_assetGroup_uniqueId.vaultId <== st_vaultId;
    cp_assetGroup_uniqueId.contractAddress <== wt_contractAddress;
    cp_assetGroup_uniqueId.idParams[0] <== 0;
    for(var j = 1; j < tm_numOfIdParams; j++) {
        cp_assetGroup_uniqueId.idParams[j] <== wt_idParamsIn[0][j];
    }

    cp_assetGroup_merkle = MerkleProof(tm_assetGroup_merkleTreeDepth);
    cp_assetGroup_merkle.leaf <== cp_assetGroup_uniqueId.out;
    cp_assetGroup_merkle.pathIndices <== wt_assetGroup_pathIndices;
    for(var j = 0; j < tm_assetGroup_merkleTreeDepth; j++) {
        cp_assetGroup_merkle.pathElements[j] <== wt_assetGroup_pathElements[j];
    }

    //dummy note if value = 0
    cp_assetGroup_isDummy = IsZero();
    cp_assetGroup_isDummy.in <== st_assetGroup_merkleRoot;

    //Check merkle proof verification if NOT isDummyInputComps
    cp_assetGroup_checkRoot = ForceEqualIfEnabled();
    cp_assetGroup_checkRoot.enabled <== 1 - cp_assetGroup_isDummy.out;
    cp_assetGroup_checkRoot.in[0] <== cp_assetGroup_merkle.root;
    cp_assetGroup_checkRoot.in[1] <== st_assetGroup_merkleRoot;


    //verify input commitments
    for(var i = 0; i < tm_nInputs; i++){

        assert(0 < wt_idParamsIn[i][0]);
        assert(wt_idParamsIn[i][0] < tm_range);

        // Generating UniqueId
        cp_inUniqueIds[i] = UniqueIdMux(tm_numOfIdParams);
        cp_inUniqueIds[i].vaultId <== st_vaultId;
        cp_inUniqueIds[i].contractAddress <== wt_contractAddress;
        for(var j = 0; j < tm_numOfIdParams; j++) {
            cp_inUniqueIds[i].idParams[j] <== wt_idParamsIn[i][j];
        }
                
        //derive pubkey from the spending key
        cp_publicKeys[i] = PublicKey();
        cp_publicKeys[i].privateKey <== wt_privateKeysIn[i];

        //verify nullifier
        cp_nullifiers[i] = Nullifier();
        cp_nullifiers[i].privateKey <== wt_privateKeysIn[i];
        cp_nullifiers[i].pathIndex <== wt_pathIndices[i];
        cp_nullifiers[i].out === st_nullifiers[i];

        //compute input commitment
        cp_commitmentsIn[i] = Commitment();
        cp_commitmentsIn[i].uniqueId <== cp_inUniqueIds[i].out;
        cp_commitmentsIn[i].publicKey <== cp_publicKeys[i].out;

        //verify cp_merkles proof on the input commitment
        cp_merkles[i] = MerkleProof(tm_merkleTreeDepth);
        cp_merkles[i].leaf <== cp_commitmentsIn[i].out;
        cp_merkles[i].pathIndices <== wt_pathIndices[i];
        for(var j = 0; j < tm_merkleTreeDepth; j++) {
            cp_merkles[i].pathElements[j] <== wt_pathElements[i][j];
        }

        cp_merkles[i].root === st_merkleRoots[i];
        
        inputsTotal += wt_idParamsIn[i][0];
    }

    // Value of the first output coin must be equal to bidAmount
    wt_idParamsOut[0][0] === wt_bidAmount;

    component cp_notesOut[tm_mOutputs];

    //verify output notes
    for(var i = 0; i < tm_mOutputs; i++){
        assert(0 <= wt_idParamsOut[i][0]);
        assert(wt_idParamsOut[i][0] < tm_range);

        // Generating UniqueId
        cp_outUniqueIds[i] = UniqueIdMux(tm_numOfIdParams);
        cp_outUniqueIds[i].contractAddress <== wt_contractAddress;
        cp_outUniqueIds[i].vaultId <== st_vaultId;

        for(var j = 0; j < tm_numOfIdParams; j++) {
            cp_outUniqueIds[i].idParams[j] <== wt_idParamsOut[i][j];
        }
        //verify commitment of output note
        cp_notesOut[i] = Commitment();

        // generating output uniqueId
        cp_notesOut[i].uniqueId <== cp_outUniqueIds[i].out;
        cp_notesOut[i].publicKey <== wt_publicKeysOut[i];
        cp_notesOut[i].out === st_commitmentsOut[i];

        //accumulates output amount
        outputsTotal += wt_idParamsOut[i][0]; //no overflow as long as tm_mOutputs is small e.g. 3
    }

    //check that inputs and outputs amounts are equal
    inputsTotal === outputsTotal;

}
