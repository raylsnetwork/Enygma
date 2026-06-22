package auctionRevert

import "math/big"

// AuctionRevertRequest is the JSON body for POST /proof/auctionRevert.
//
// Used when an auction is canceled (deadline passed with no settlement, or
// zero bids ever submitted). Bob proves ownership of the locked NFT note and
// mints it back to himself under a fresh salt — never reusing StCommitLocked
// itself, which is already public, to avoid linking the reverted note back to
// this specific auction.
//
// Public statement returned (4 elements):
//
//	[StAuctionId, StCommitLocked, StNftTokenId, StRevertedCommit]
type AuctionRevertRequest struct {
	StAuctionId      string `json:"stAuctionId"      binding:"required"`
	StCommitLocked   string `json:"stCommitLocked"   binding:"required"`
	StNftTokenId     string `json:"stNftTokenId"     binding:"required"`
	StRevertedCommit string `json:"stRevertedCommit" binding:"required"`

	WtSpendKey   string `json:"wtSpendKey"   binding:"required"`
	WtTokenId    string `json:"wtTokenId"    binding:"required"`
	WtSaltLocked string `json:"wtSaltLocked" binding:"required"` // original salt from initAuction()
	WtSaltOut    string `json:"wtSaltOut"    binding:"required"` // fresh salt for the reverted commitment
}

// AuctionRevertOutput is the JSON response from POST /proof/auctionRevert.
type AuctionRevertOutput struct {
	Proof        []*big.Int `json:"proof"`
	PublicSignal []*big.Int `json:"publicSignal"`
}
