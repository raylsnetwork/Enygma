pragma circom 2.0.0;
include "./templates/AuctionInitTemplate.circom";

component main {public [st_beacon, st_auctionId, st_uniqueId, st_treeNumber, st_merkleRoot, st_nullifier]} =  AuctionInitTemplate(8);
