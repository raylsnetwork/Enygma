pragma circom 2.0.0;
include "./templates/Erc1155FungibleWithAuditorTemplate.circom";

component main {
                public [
                    st_message, 
                    st_treeNumbers, 
                    st_merkleRoots, 
                    st_nullifiers, 
                    st_commitmentsOut, 
                    st_auditor_publicKey, 
                    st_auditor_authKey,
                    st_auditor_nonce,
                    st_auditor_encryptedValues, 
                    st_assetGroup_treeNumber, 
                    st_assetGroup_merkleRoot 
                ]
            } =  Erc1155FungibleWithAuditorTemplate(
                    2,
                    2,
                    8, 
                    1000000000000000000000000000000000000,
                    8
                );