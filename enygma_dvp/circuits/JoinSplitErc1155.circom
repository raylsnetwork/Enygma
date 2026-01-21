pragma circom 2.0.0;
include "./templates/Erc1155FungibleTemplate.circom";

// this circuit should only be used for fungible ERC1155 token.
component main {
                public [
                    st_message, 
                    st_treeNumbers, 
                    st_merkleRoots, 
                    st_nullifiers, 
                    st_commitmentsOut, 
                    st_assetGroup_treeNumber, 
                    st_assetGroup_merkleRoot
                ]
            } =  Erc1155FungibleTemplate(2,2,8,1000000000000000000000000000000000000, 8);