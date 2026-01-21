pragma circom 2.0.0;
include "./templates/AuctionNotWinningBidTemplate.circom";

component main {
    public [
        st_auctionId, 
        st_blindedBidDifference, 
        st_bidBlockNumber, 
        st_winningBidBlockNumber
    ]
} =  AuctionNotWinningBidTemplate(1000000000000000000000000000000000000);
