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

// tm_numOfTokens: max number of tokens in the batch commitment mode.

template Erc1155NonFungibleTemplate(tm_numOfTokens, tm_merkleTreeDepth, tm_assetGroup_merkleTreeDepth) {

    // Statement
    signal input st_message; // message that has been used for swap/mix functionality
    signal input st_treeNumbers[tm_numOfTokens];
    signal input st_merkleRoots[tm_numOfTokens]; // merkleRoot of the input erc1155 token
    signal input st_nullifiers[tm_numOfTokens];
    signal input st_commitmentsOut[tm_numOfTokens];
    signal input st_assetGroup_treeNumbers[tm_numOfTokens];
    signal input st_assetGroup_merkleRoots[tm_numOfTokens];

    // Witness
    signal input wt_privateKeysIn[tm_numOfTokens];
    signal input wt_values[tm_numOfTokens];
    signal input wt_pathElements[tm_numOfTokens][tm_merkleTreeDepth];
    signal input wt_pathIndices[tm_numOfTokens];
    signal input wt_erc1155TokenIds[tm_numOfTokens];
    signal input wt_publicKeysOut[tm_numOfTokens]; 
    signal input wt_erc1155ContractAddress;

    signal input wt_assetGroup_pathElements[tm_numOfTokens][tm_assetGroup_merkleTreeDepth];
    signal input wt_assetGroup_pathIndices[tm_numOfTokens];

    component cp_outPublicKeys[tm_numOfTokens];
    component cp_inPublicKeys[tm_numOfTokens];
    component cp_uniqueIds[tm_numOfTokens];
    component cp_nullifiers[tm_numOfTokens];
    component cp_inCommitments[tm_numOfTokens];
    component cp_outCommitments[tm_numOfTokens];
    component cp_merkle[tm_numOfTokens];
    component cp_isDummyInputs[tm_numOfTokens];
    component cp_checkEqualIfIsNotDummys[tm_numOfTokens];
    component cp_checkEqualIfIsNotDummys2[tm_numOfTokens];
    component cp_checkEqualIfIsNotDummys3[tm_numOfTokens];

    component cp_assetGroup_merkle[tm_numOfTokens];
    component cp_assetGroup_checkDummys[tm_numOfTokens];
    component cp_assetGroup_uniqueIds[tm_numOfTokens];

    for(var i = 0; i < tm_numOfTokens; i++){
        cp_isDummyInputs[i] = IsZero();
        cp_isDummyInputs[i].in <== wt_values[i];

        // creating erc1155 uniqueId with amount = 0
        cp_assetGroup_uniqueIds[i] = Erc1155UniqueId();
        cp_assetGroup_uniqueIds[i].erc1155ContractAddress <== wt_erc1155ContractAddress;
        cp_assetGroup_uniqueIds[i].erc1155TokenId <== wt_erc1155TokenIds[i];
        cp_assetGroup_uniqueIds[i].amount <== 0;
        

        cp_assetGroup_merkle[i] = MerkleProof(tm_assetGroup_merkleTreeDepth);
        cp_assetGroup_merkle[i].leaf <== cp_assetGroup_uniqueIds[i].out;
        cp_assetGroup_merkle[i].pathIndices <== wt_assetGroup_pathIndices[i];
        for(var j = 0; j< tm_assetGroup_merkleTreeDepth; j++) {
            cp_assetGroup_merkle[i].pathElements[j] <== wt_assetGroup_pathElements[i][j];
        }

        // checking merkleRoot if the input value is not zero
        cp_assetGroup_checkDummys[i] = ForceEqualIfEnabled();
        cp_assetGroup_checkDummys[i].enabled <== 1 - cp_isDummyInputs[i].out;
        cp_assetGroup_checkDummys[i].in[0] <== cp_assetGroup_merkle[i].root;
        cp_assetGroup_checkDummys[i].in[1] <== st_assetGroup_merkleRoots[i];

        //derive pubkey from the spending secret key
        cp_inPublicKeys[i] = PublicKey();
        cp_inPublicKeys[i].privateKey <== wt_privateKeysIn[i];

        //verify nullifier
        cp_nullifiers[i] = Nullifier();
        cp_nullifiers[i].privateKey <== wt_privateKeysIn[i];
        cp_nullifiers[i].pathIndex <== wt_pathIndices[i];

        cp_checkEqualIfIsNotDummys[i] = ForceEqualIfEnabled();
        cp_checkEqualIfIsNotDummys[i].enabled <== 1 - cp_isDummyInputs[i].out;
        cp_checkEqualIfIsNotDummys[i].in[0] <== cp_nullifiers[i].out;
        cp_checkEqualIfIsNotDummys[i].in[1] <== st_nullifiers[i];

        //compute uniqueId per token
        cp_uniqueIds[i] = Erc1155UniqueId();
        cp_uniqueIds[i].erc1155ContractAddress <== wt_erc1155ContractAddress;
        cp_uniqueIds[i].erc1155TokenId <== wt_erc1155TokenIds[i];
        cp_uniqueIds[i].amount <== wt_values[i];

        // compute and verify input commitment
        cp_inCommitments[i] = Commitment();
        cp_inCommitments[i].uniqueId <== cp_uniqueIds[i].out;
        cp_inCommitments[i].publicKey <== cp_inPublicKeys[i].out;

        // verify merkleComp proof on the note commitment
        cp_merkle[i] = MerkleProof(tm_merkleTreeDepth);
        cp_merkle[i].leaf <== cp_inCommitments[i].out;
        cp_merkle[i].pathIndices <== wt_pathIndices[i];
        for(var j = 0; j< tm_merkleTreeDepth; j++) {
            cp_merkle[i].pathElements[j] <== wt_pathElements[i][j];
        }

        cp_checkEqualIfIsNotDummys2[i] = ForceEqualIfEnabled();
        cp_checkEqualIfIsNotDummys2[i].enabled <== 1 - cp_isDummyInputs[i].out;
        cp_checkEqualIfIsNotDummys2[i].in[0] <== cp_merkle[i].root;
        cp_checkEqualIfIsNotDummys2[i].in[1] <== st_merkleRoots[i];

        // compute and verifiy output commitment
        cp_outCommitments[i] = Commitment();
        cp_outCommitments[i].uniqueId <== cp_uniqueIds[i].out;
        cp_outCommitments[i].publicKey <== wt_publicKeysOut[i];

        cp_checkEqualIfIsNotDummys3[i] = ForceEqualIfEnabled();
        cp_checkEqualIfIsNotDummys3[i].enabled <== 1 - cp_isDummyInputs[i].out;
        cp_checkEqualIfIsNotDummys3[i].in[0] <== st_commitmentsOut[i];
        cp_checkEqualIfIsNotDummys3[i].in[1] <== cp_outCommitments[i].out;

    }


}
