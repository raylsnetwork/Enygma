pragma circom 2.0.0;
include "./templates/AuctionPrivateOpeningTemplate.circom";

component main {
    public [
        st_auctionId, 
        st_blindedBid
    ]
} =  AuctionPrivateOpeningTemplate(FUNGIBLE_RANGE);
