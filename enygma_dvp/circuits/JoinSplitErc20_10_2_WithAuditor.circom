pragma circom 2.0.0;
include "./templates/Erc20WithAuditorTemplate.circom";

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
                    st_auditor_encryptedValues
                ]
            } =  Erc20WithAuditorTemplate(10,2,8,1000000000000000000000000000000000000);
