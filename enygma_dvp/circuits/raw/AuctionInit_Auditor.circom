pragma circom 2.0.0;
include "./templates/Tm_AuctionInit_Auditor.circom";

component main {
    public [
        st_beacon, 
        st_vaultId, 
        st_auctionId, 
        st_treeNumber, 
        st_merkleRoot, 
        st_nullifier,
        st_auditor_publicKey, 
        st_auditor_authKey,
        st_auditor_nonce,
        st_auditor_encryptedValues,
        st_assetGroup_treeNumber, 
        st_assetGroup_merkleRoot
    ]
} =  Tm_AuctionInit_Auditor(5, TREE_DEPTH, TREE_DEPTH);
