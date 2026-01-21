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

// Broker generates a proof that checks 
// + broker has access to the coin with st_delegator_commitment
// + seller has wt_broker_publickey and generated st_broker_commitment with proper st_brokerageFee.
// + seller's coin's value is in valid range
// + st_brokerageFee is in valid brokeragePercentageRange of the seller's coin's value
// ...................................
// The proof's application:
// + corfirms on-chain that broker knows brokerageFee and her coin's privateKey
// + confirms on-chain that broker knows seller and buyer's coin's commitments and hence the settlement value.

template BrokerSettlementTemplate(maxCommissionPercentage, commissionPercentageDecimals) {

    // Statement
    signal input st_beacon;
    signal input st_vaultId;
    signal input st_broker_blindedPublicKey;

    // Witness
    signal input wt_contractAddress;
    signal input wt_broker_privateKey;
    signal input wt_broker_idParams[5];
    signal input wt_broker_publickey;
    
    // components
    component cp_broker_publicKey;
    component cp_broker_uniqueId;
    component cp_broker_commitment;
    component cp_broker_blinder;

    // checking the range of delegator's coin's value
    assert(wt_delegator_idParams[0] < range);
    assert(0 <= wt_delegator_idParams[0]);
    
    // checking 
    //generating delegator's coin's uniqueId
    cp_broker_uniqueId = UniqueIdMux();
    cp_broker_uniqueId.vaultId <== st_vaultId;
    cp_broker_uniqueId.contractAddress <== wt_contractAddress;
    cp_broker_uniqueId.idParams[0] <== wt_delegator_idParams[0];
    cp_broker_uniqueId.idParams[1] <== wt_delegator_idParams[1];
    cp_broker_uniqueId.idParams[2] <== wt_delegator_idParams[2];
    cp_broker_uniqueId.idParams[3] <== wt_delegator_idParams[3];
    cp_broker_uniqueId.idParams[4] <== wt_delegator_idParams[4];

    // generating Delegator's coin's publicKey
    cp_broker_publicKey = PublicKey();
    cp_broker_publicKey.privateKey <== wt_delegator_privateKey;

    // verifying Delegator's coin's commitment
    cp_broker_commitment = Commitment();
    cp_broker_commitment.uniqueId <== cp_broker_uniqueId.out;
    cp_broker_commitment.publicKey <== cp_broker_publicKey.out;
    cp_broker_commitment.out === st_delegator_commitment;

}
