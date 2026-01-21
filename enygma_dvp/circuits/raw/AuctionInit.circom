pragma circom 2.0.0;
include "./templates/AuctionInitTemplate.circom";

component main {
    public [
        st_beacon, 
        st_vaultId, 
        st_auctionId, 
        st_treeNumber, 
        st_merkleRoot, 
        st_nullifier,
        st_assetGroup_treeNumber, 
        st_assetGroup_merkleRoot
    ]
} =  AuctionInitTemplate(5, TREE_DEPTH, TREE_DEPTH);
