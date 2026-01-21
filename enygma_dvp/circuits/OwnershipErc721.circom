pragma circom 2.0.0;
include "./templates/Erc721Template.circom";

component main {
                public [
                    st_message, 
                    st_treeNumbers, 
                    st_merkleRoots, 
                    st_nullifiers, 
                    st_commitmentsOut
                ]
            } =  Erc721Template(1,8);