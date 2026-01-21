pragma circom 2.0.0;
include "./templates/Erc1155FungibleWithBrokerV1Template.circom";

// this circuit should only be used for fungible ERC1155 token.
component main {
                public [
                    st_message, 
                    st_treeNumbers, 
                    st_merkleRoots, 
                    st_nullifiers, 
                    st_commitmentsOut, 
                    st_broker_blindedPublicKey, 
                    st_broker_commissionRate, 
                    st_assetGroup_treeNumber, 
                    st_assetGroup_merkleRoot
                ]
            } =  Erc1155FungibleWithBrokerV1Template(
                    2,
                    3,
                    TREE_DEPTH,
                    FUNGIBLE_RANGE, 
                    TREE_DEPTH, 
                    10, 
                    2
                );