package auctionWithdraw

import "math/big"

// AuctionWithdrawRequest is the JSON body for POST /proof/auctionWithdraw.
//
// The bidder proves they own the bid commitment (commitA) by supplying its
// preimage. The contract uses this to authenticate the withdrawal before
// unlocking and spending the bidder's locked USDC note.
//
// Public statement returned (2 elements):
//
//	[StAuctionId, StCommitA]
type AuctionWithdrawRequest struct {
	StAuctionId string `json:"stAuctionId" binding:"required"`
	StCommitA   string `json:"stCommitA"   binding:"required"`

	WtSpendKey string `json:"wtSpendKey" binding:"required"` // bidder's spend secret key
	WtSaltA    string `json:"wtSaltA"    binding:"required"` // salt used to form commitA
	WtAmount   string `json:"wtAmount"   binding:"required"` // USDC amount in commitA
	WtTokenId  string `json:"wtTokenId"  binding:"required"` // USDC token ID
}

// AuctionWithdrawOutput is the JSON response from POST /proof/auctionWithdraw.
type AuctionWithdrawOutput struct {
	Proof        []*big.Int `json:"proof"`
	PublicSignal []*big.Int `json:"publicSignal"`
}
