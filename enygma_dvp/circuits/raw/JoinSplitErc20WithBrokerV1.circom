pragma circom 2.0.0;
include "./templates/Erc20WithBrokerV1Template.circom";

component main {
            public [
                st_message, 
                st_treeNumbers, 
                st_merkleRoots, 
                st_nullifiers, 
                st_commitmentsOut, 
                st_broker_blindedPublicKey
            ]
        } =  Erc20WithBrokerV1Template(2,3,TREE_DEPTH, FUNGIBLE_RANGE, 10, 0);
