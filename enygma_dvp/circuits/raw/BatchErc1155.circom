pragma circom 2.0.0;
include "./templates/Erc1155NonFungibleTemplate.circom";

component main {
    public 
    [
        st_message, 
        st_treeNumbers, 
        st_merkleRoots, 
        st_nullifiers, 
        st_commitmentsOut, 
        st_assetGroup_treeNumbers, 
        st_assetGroup_merkleRoots
    ]
} =  Erc1155NonFungibleTemplate(10,TREE_DEPTH, TREE_DEPTH);