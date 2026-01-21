pragma circom 2.0.0;
include "./templates/Erc1155NonFungibleWithAuditorTemplate.circom";

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
                st_assetGroup_treeNumbers, 
                st_assetGroup_merkleRoots
            ]
} =  Erc1155NonFungibleWithAuditorTemplate(1,8, 8);