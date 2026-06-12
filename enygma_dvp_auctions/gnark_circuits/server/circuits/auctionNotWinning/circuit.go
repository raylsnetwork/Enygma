package auctionNotWinning

import "math/big"

// AuctionNotWinningRequest is the JSON body accepted by POST /proof/auctionNotWinning.
//
// Submitted by a losing bidder to prove their bid was strictly less than the
// winning amount and unlock their commitA (locked USDC).
//
// WtSaltA is derived from the bidder's personal ML-KEM encapsulation during
// the original AuctionBid step: saltA = HKDF(ss_personal, "bid salt A").
// Only the original bidder can produce it, preventing proxy submissions.
//
// Public statement returned: [stAuctionId, stCommitA, stWinningAmount]
type AuctionNotWinningRequest struct {
	StAuctionId     string `json:"stAuctionId"     binding:"required"`
	StCommitA       string `json:"stCommitA"       binding:"required"` // loser's locked bid commitment
	StWinningAmount string `json:"stWinningAmount" binding:"required"` // from AuctionSettle public signal

	WtPkBidder string `json:"wtPkBidder" binding:"required"` // loser's spend public key
	WtSaltA    string `json:"wtSaltA"    binding:"required"` // salt that opens commitA
	WtMyAmount string `json:"wtMyAmount" binding:"required"` // loser's bid amount (< StWinningAmount)
	WtTokenId  string `json:"wtTokenId"  binding:"required"` // USDC token ID
}

// AuctionNotWinningOutput is the JSON response from POST /proof/auctionNotWinning.
type AuctionNotWinningOutput struct {
	Proof        []*big.Int `json:"proof"`
	PublicSignal []*big.Int `json:"publicSignal"`
}
