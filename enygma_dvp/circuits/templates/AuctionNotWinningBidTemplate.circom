pragma circom 2.0.0;

include "../circomlib/circuits/comparators.circom";

include "../primitives/MerkleProof.circom";
include "../primitives/Commitment.circom";
include "../primitives/Nullifier.circom";
include "../primitives/PublicKey.circom";
include "../primitives/Pedersen.circom";

template AuctionNotWinningBidTemplate(tm_range) {

    // Statement
    signal input st_auctionId; 
    signal input st_blindedBidDifference;
    signal input st_bidBlockNumber;
    signal input st_winningBidBlockNumber;

    // Witness
    signal input wt_bidAmount;
    signal input wt_bidRandom;
    signal input wt_winningBidAmount;
    signal input wt_winningBidRandom;

    // check bidAmount and winningBidAmount to be in tm_range
    assert(wt_bidAmount < tm_range);
    assert(0 < wt_bidAmount);
    assert(wt_winningBidAmount < tm_range);
    assert(0 < wt_winningBidAmount);

    component cp_pedersen;
    cp_pedersen = Pedersen();
    cp_pedersen.amount <== wt_bidAmount;
    cp_pedersen.random <== wt_bidRandom;

    component cp_pedersen2;
    cp_pedersen2 = Pedersen();
    cp_pedersen2.amount <== wt_winningBidAmount;
    cp_pedersen2.random <== wt_winningBidRandom;

    st_blindedBidDifference === (cp_pedersen2.out - cp_pedersen.out); 


    component cp_lessEq;
    cp_lessEq = LessEqThan(60);
    cp_lessEq.in[0] <== wt_bidAmount;
    cp_lessEq.in[1] <== wt_winningBidAmount;
    
    cp_lessEq.out === 1;

    // (blindedWinningBid - blindedBid) < blindedWinningBid
    // or bidAmount == winningBidAmount && bidBlockNumber < winningBidBlockNumber)
}
