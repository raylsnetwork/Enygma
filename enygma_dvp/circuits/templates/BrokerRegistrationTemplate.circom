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
include "../primitives/UniqueIdMux.circom";
include "../primitives/Blinder.circom";

// Seller (delegator) generates a proof that proves 
// + seller has access to an unspent valid coin.
// + seller knows broker's publicKey to create new coin with that.
// ...................................
// The proof's application:
// + prevents the broker from Front-running or performing MITM
// + gives the broker a receipt that proves to the buyer that broker is seller's legit delegatee to do the negotiations.

template BrokerRegistrationTemplate(
    tm_numOfInputs, 
    tm_merkleTreeDepth, 
    tm_groupMerkleTreeDepth, 
    tm_range, 
    tm_maxPermittedCommissionRate, 
    tm_commissionRateDecimals
) {

    // Statement
    signal input st_beacon;
    signal input st_vaultId;
    signal input st_groupId;
    signal input st_delegator_treeNumbers[tm_numOfInputs];
    signal input st_delegator_merkleRoots[tm_numOfInputs];
    signal input st_delegator_nullifiers[tm_numOfInputs];
    signal input st_broker_blindedPublicKey;
    signal input st_broker_minCommissionRate;
    signal input st_broker_maxCommissionRate;

    signal input st_assetGroup_treeNumber;
    signal input st_assetGroup_merkleRoot;

    // Witness
    signal input wt_delegator_privateKeys[tm_numOfInputs];
    signal input wt_delegator_pathElements[tm_numOfInputs][tm_merkleTreeDepth];
    signal input wt_delegator_pathIndices[tm_numOfInputs];
    signal input wt_delegator_idParams[tm_numOfInputs][5];
    signal input wt_contractAddress;
    signal input wt_broker_publicKey;

    signal input wt_assetGroup_pathElements[tm_groupMerkleTreeDepth];
    signal input wt_assetGroup_pathIndices;

    // components
    component cp_delegator_publicKeys[tm_numOfInputs];
    component cp_delegator_uniqueIds[tm_numOfInputs];
    component cp_delegator_commitments[tm_numOfInputs];
    component cp_delegator_nullifiers[tm_numOfInputs];
    component cp_delegator_merkles[tm_numOfInputs];
    component cp_broker_blinder;

    component cp_assetGroup_merkle;
    component cp_assetGroup_isDummy;
    component cp_assetGroup_checkRoot;
    component cp_assetGroup_uniqueId;


    cp_broker_blinder = Blinder();
    cp_broker_blinder.in <== wt_broker_publicKey;
    cp_broker_blinder.out === st_broker_blindedPublicKey;

    var totalAmount = 0;
    // TODO:: check idParams[i][1..4] to be the same
    for(var i =0; i<tm_numOfInputs; i++){
        // checking the range of delegator's coin's value
        assert(wt_delegator_idParams[i][0] < tm_range);
        assert(0 <= wt_delegator_idParams[i][0]);

        //generating delegator's coin's uniqueId
        cp_delegator_uniqueIds[i] = UniqueIdMux(5);
        cp_delegator_uniqueIds[i].vaultId <== st_vaultId;
        cp_delegator_uniqueIds[i].contractAddress <== wt_contractAddress;
        cp_delegator_uniqueIds[i].idParams[0] <== wt_delegator_idParams[i][0];
        cp_delegator_uniqueIds[i].idParams[1] <== wt_delegator_idParams[i][1];
        cp_delegator_uniqueIds[i].idParams[2] <== wt_delegator_idParams[i][2];
        cp_delegator_uniqueIds[i].idParams[3] <== wt_delegator_idParams[i][3];
        cp_delegator_uniqueIds[i].idParams[4] <== wt_delegator_idParams[i][4];

        // generating Delegator's coin's publicKey
        cp_delegator_publicKeys[i] = PublicKey();
        cp_delegator_publicKeys[i].privateKey <== wt_delegator_privateKeys[i];

        // verifying Delegator's coin's commitment
        cp_delegator_commitments[i] = Commitment();
        cp_delegator_commitments[i].uniqueId <== cp_delegator_uniqueIds[i].out;
        cp_delegator_commitments[i].publicKey <== cp_delegator_publicKeys[i].out;


        //verify nullifier
        cp_delegator_nullifiers[i] = Nullifier();
        cp_delegator_nullifiers[i].privateKey <== wt_delegator_privateKeys[i];
        cp_delegator_nullifiers[i].pathIndex <== wt_delegator_pathIndices[i];
        cp_delegator_nullifiers[i].out === st_delegator_nullifiers[i];

        //verify cp_merkle proof on the commitment
        cp_delegator_merkles[i] = MerkleProof(tm_merkleTreeDepth);
        cp_delegator_merkles[i].leaf <== cp_delegator_commitments[i].out;
        cp_delegator_merkles[i].pathIndices <== wt_delegator_pathIndices[i];
        for(var j = 0; j < tm_merkleTreeDepth; j++) {
            cp_delegator_merkles[i].pathElements[j] <== wt_delegator_pathElements[i][j];
        }
        st_delegator_merkleRoots[i] === cp_delegator_merkles[i].root;

        totalAmount += wt_delegator_idParams[i][0];
    }

    cp_assetGroup_uniqueId = UniqueIdMux(5);
    cp_assetGroup_uniqueId.vaultId <== st_vaultId;
    cp_assetGroup_uniqueId.contractAddress <== wt_contractAddress;
    cp_assetGroup_uniqueId.idParams[0] <== 0;
    cp_assetGroup_uniqueId.idParams[1] <== wt_delegator_idParams[0][1];
    cp_assetGroup_uniqueId.idParams[2] <== wt_delegator_idParams[0][2];
    cp_assetGroup_uniqueId.idParams[3] <== wt_delegator_idParams[0][3];
    cp_assetGroup_uniqueId.idParams[4] <== wt_delegator_idParams[0][4];

    cp_assetGroup_merkle = MerkleProof(tm_groupMerkleTreeDepth);
    cp_assetGroup_merkle.leaf <== cp_assetGroup_uniqueId.out;
    cp_assetGroup_merkle.pathIndices <== wt_assetGroup_pathIndices;
    for(var j = 0; j < tm_groupMerkleTreeDepth; j++) {
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

    // checking commission percentage ranges
    assert(0 <= st_broker_minCommissionRate);
    assert(0 <= st_broker_maxCommissionRate);
    assert(st_broker_minCommissionRate <= st_broker_maxCommissionRate);
    var scaledMax = st_broker_maxCommissionRate \ (10 ** tm_commissionRateDecimals);
    assert(scaledMax <= tm_maxPermittedCommissionRate);
}
