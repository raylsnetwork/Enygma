package auctionBatch

import "math/big"

// AuctionBatchRequest is the JSON body for POST /proof/auctionBatch.
//
// Only the auctioneer calls this endpoint; they supply the decrypted preimage
// of every bid in the batch.  Inactive slots must have WtActive[i]="0" and
// StBidCommitA[i]="0".
//
// Public statement returned (104 elements):
//
//	[StAuctionId, StBidCommitA[0..99], StBatchWinnerCommit, StBatchWinnerPk, StBatchWinnerAmount]
type AuctionBatchRequest struct {
	// public inputs
	StAuctionId         string      `json:"stAuctionId"         binding:"required"`
	StBidCommitA        [100]string `json:"stBidCommitA"        binding:"required"` // on-chain commitA values; 0 = inactive
	StBatchWinnerCommit string      `json:"stBatchWinnerCommit" binding:"required"`
	StBatchWinnerPk     string      `json:"stBatchWinnerPk"     binding:"required"`
	StBatchWinnerAmount string      `json:"stBatchWinnerAmount" binding:"required"`

	// private witnesses
	WtAuctionId string      `json:"wtAuctionId" binding:"required"` // must equal StAuctionId

	// private witnesses — auctioneer's decrypted bid data
	WtActive    [100]string `json:"wtActive"    binding:"required"` // "0" or "1" per slot
	WtPkBidder  [100]string `json:"wtPkBidder"  binding:"required"`
	WtSaltA     [100]string `json:"wtSaltA"     binding:"required"`
	WtAmount    [100]string `json:"wtAmount"    binding:"required"`
	WtTokenId   [100]string `json:"wtTokenId"   binding:"required"`
	WtWinnerIdx string      `json:"wtWinnerIdx" binding:"required"`
}

// AuctionBatchOutput is the JSON response from POST /proof/auctionBatch.
type AuctionBatchOutput struct {
	Proof        []*big.Int `json:"proof"`
	PublicSignal []*big.Int `json:"publicSignal"`
}
