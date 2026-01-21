pragma circom 2.0.0;
include "../primitives/MerkleProof.circom";
include "../primitives/Commitment.circom";
include "../primitives/Nullifier.circom";
include "../primitives/PublicKey.circom";
include "../primitives/AuctionId.circom";
include "../primitives/UniqueIdMux.circom";
include "../circomlib/circuits/bitify.circom";

include "../circomlib/circuits/comparators.circom";

template AuctionInitTemplate(tm_numOfIdParams, tm_merkleTreeDepth, tm_assetGroup_merkleTreeDepth) {

    // Statement
    signal input st_beacon; 
    signal input st_vaultId;
    signal input st_auctionId;

    signal input st_treeNumber;
    signal input st_merkleRoot;
    signal input st_nullifier;

    signal input st_assetGroup_treeNumber;
    signal input st_assetGroup_merkleRoot;

    // Witness
    signal input wt_commitment;
    signal input wt_pathElements[tm_merkleTreeDepth];
    signal input wt_pathIndices;
    signal input wt_privateKey;
    signal input wt_idParams[tm_numOfIdParams];
    signal input wt_contractAddress;

    signal input wt_assetGroup_pathElements[tm_assetGroup_merkleTreeDepth];
    signal input wt_assetGroup_pathIndices;

    component cp_publicKey;
    component cp_commitment;
    component cp_nullifier;
    component cp_merkle;
    component cp_auctionId;
    component cp_uniqueId;

    component cp_assetGroup_merkle;
    component cp_assetGroup_isDummy;
    component cp_assetGroup_checkRoot;
    component cp_assetGroup_uniqueId;

    cp_uniqueId = UniqueIdMux(tm_numOfIdParams);
    cp_uniqueId.vaultId <== st_vaultId;
    cp_uniqueId.contractAddress <== wt_contractAddress;
    cp_uniqueId.idParams <== wt_idParams;

    //derive pubkey from the spending key
    cp_publicKey = PublicKey();
    cp_publicKey.privateKey <== wt_privateKey;

    //verify nullifier
    cp_nullifier = Nullifier();
    cp_nullifier.privateKey <== wt_privateKey;
    cp_nullifier.pathIndex <== wt_pathIndices;
    cp_nullifier.out === st_nullifier;

    //compute commitment
    cp_commitment = Commitment();
    cp_commitment.uniqueId <== cp_uniqueId.out;
    cp_commitment.publicKey <== cp_publicKey.out;
    cp_commitment.out === wt_commitment;

    //verify cp_merkle proof on the commitment
    cp_merkle = MerkleProof(tm_merkleTreeDepth);
    cp_merkle.leaf <== cp_commitment.out;
    cp_merkle.pathIndices <== wt_pathIndices;
    for(var j = 0; j < tm_merkleTreeDepth; j++) {
        cp_merkle.pathElements[j] <== wt_pathElements[j];
    }
    st_merkleRoot === cp_merkle.root;

    cp_auctionId = AuctionId();
    cp_auctionId.commitment <== wt_commitment;
    cp_auctionId.out === st_auctionId;

    // verifying assetGroup membership

    // if st_assetGroup_merkleRoot == 0 
    // the membership has not been mentioned
    // => it should be checked on-chain


    cp_assetGroup_uniqueId = UniqueIdMux(tm_numOfIdParams);
    cp_assetGroup_uniqueId.vaultId <== st_vaultId;
    cp_assetGroup_uniqueId.contractAddress <== wt_contractAddress;
    cp_assetGroup_uniqueId.idParams[0] <== 0;
    for(var j = 1; j < tm_numOfIdParams; j++) {
        cp_assetGroup_uniqueId.idParams[j] <== wt_idParams[j];
    }    


    cp_assetGroup_merkle = MerkleProof(tm_assetGroup_merkleTreeDepth);
    cp_assetGroup_merkle.leaf <== cp_assetGroup_uniqueId.out;
    cp_assetGroup_merkle.pathIndices <== wt_pathIndices;
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


}
