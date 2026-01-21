pragma circom 2.0.0;

include "../primitives/Pedersen.circom";

template AuctionPrivateOpeningTemplate(tm_range) {
    // Statement
    signal input st_auctionId; 
    signal input st_blindedBid;

    // Witness
    signal input wt_bidAmount;
    signal input wt_bidRandom;

    // check bidAmount to be in tm_range
    assert(wt_bidAmount < tm_range);
    assert(0 < wt_bidAmount);

    component cp_pedersen;
    cp_pedersen = Pedersen();
    cp_pedersen.amount <== wt_bidAmount;
    cp_pedersen.random <== wt_bidRandom;
    cp_pedersen.out === st_blindedBid;
}
