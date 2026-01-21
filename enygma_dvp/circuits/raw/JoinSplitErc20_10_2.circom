pragma circom 2.0.0;
include "./templates/Erc20Template.circom";

component main {
                public [
                    st_message, 
                    st_treeNumbers, 
                    st_merkleRoots, 
                    st_nullifiers, 
                    st_commitmentsOut
                ]
            } =  Erc20Template(10,2,TREE_DEPTH,FUNGIBLE_RANGE);
