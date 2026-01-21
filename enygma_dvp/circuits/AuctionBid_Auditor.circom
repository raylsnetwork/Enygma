pragma circom 2.0.0;
include "./templates/Tm_AuctionBid_Auditor.circom";

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
        st_auditor_publicKey,
        st_auditor_authKey,
        st_auditor_nonce,
        st_auditor_encryptedValues,
        st_auctioneer_publicKey,
        st_auctioneer_authKey,
        st_auctioneer_nonce,
        st_auctioneer_encryptedValues,
        st_assetGroup_treeNumber,
        st_assetGroup_merkleRoot
    ]
} =  Tm_AuctionBid_Auditor(2, 2, 5, 8, 1000000000000000000000000000000000000, 8);
