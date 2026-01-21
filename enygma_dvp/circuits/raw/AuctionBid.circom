pragma circom 2.0.0;
include "./templates/AuctionBidTemplate.circom";

component main {
    public [
        st_beacon,
        st_auctionId, 
        st_blindedBid, 
        st_vaultId, 
        st_treeNumbers, 
        st_merkleRoots, 
        st_nullifiers, 
        st_commitmentsOut,
        st_assetGroup_treeNumber, 
        st_assetGroup_merkleRoot
    ]
} =  AuctionBidTemplate(2,2, 5, TREE_DEPTH,FUNGIBLE_RANGE, TREE_DEPTH);