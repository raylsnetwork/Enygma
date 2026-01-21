pragma circom 2.0.0;
include "./templates/AuctionBidTemplate.circom";

component main {public [st_auctionId, st_blindedBid, st_treeNumbers, st_merkleRoots, st_nullifiers, st_commitmentsOut]} =  AuctionBidTemplate(2,2,8);