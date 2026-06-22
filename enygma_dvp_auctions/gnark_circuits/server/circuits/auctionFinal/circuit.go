package auctionFinal

import "math/big"

// AuctionFinalRequest is the JSON body for POST /proof/auctionFinal.
//
// The auctioneer submits the Phase-1 batch outputs (already verified on-chain)
// together with the settlement witnesses recovered from the winning bid's
// ML-KEM decryption.  Inactive batch slots must have StBatchWinnerCommit[i]="0"
// and WtBatchActive[i]="0".
//
// Public statement returned (38 elements):
//
//	[StAuctionId,
//	 StBatchWinnerCommit[0..9], StBatchWinnerPk[0..9], StBatchWinnerAmount[0..9],
//	 StOverallWinnerCommit, StWinnerPk, StWinnerCommitB,
//	 StWinnerNftCommit, StNftTokenId, StWinningAmount, StFloorPrice]
type AuctionFinalRequest struct {
	// public inputs — Phase-1 batch outputs
	StAuctionId         string      `json:"stAuctionId"         binding:"required"`
	StBatchWinnerCommit [10]string  `json:"stBatchWinnerCommit" binding:"required"` // 0 = inactive batch
	StBatchWinnerPk     [10]string  `json:"stBatchWinnerPk"     binding:"required"`
	StBatchWinnerAmount [10]string  `json:"stBatchWinnerAmount" binding:"required"`

	// public outputs — settlement
	StOverallWinnerCommit string `json:"stOverallWinnerCommit" binding:"required"` // overall winner's commitA
	StWinnerPk            string `json:"stWinnerPk"            binding:"required"` // overall winner's spend public key
	StWinnerCommitB       string `json:"stWinnerCommitB"       binding:"required"` // USDC payout commitment (Bob)
	StWinnerNftCommit     string `json:"stWinnerNftCommit"     binding:"required"` // NFT commitment (Alice)
	StNftTokenId          string `json:"stNftTokenId"          binding:"required"`
	StWinningAmount       string `json:"stWinningAmount"       binding:"required"`
	StFloorPrice          string `json:"stFloorPrice"          binding:"required"` // seller's reserve price

	// private witnesses
	WtBatchActive    [10]string `json:"wtBatchActive"    binding:"required"` // "0" or "1" per batch
	WtWinnerBatchIdx string     `json:"wtWinnerBatchIdx" binding:"required"` // which batch contains the winner
	WtPkBob          string     `json:"wtPkBob"          binding:"required"` // NFT seller's spend key (from decryption)
	WtSaltB          string     `json:"wtSaltB"          binding:"required"` // HKDF(ss_winner, "note salt")
	WtUsdcTokenId    string     `json:"wtUsdcTokenId"    binding:"required"`
	WtSaltAlice      string     `json:"wtSaltAlice"      binding:"required"` // fresh salt for winner's NFT commitment
	WtNftTokenId     string     `json:"wtNftTokenId"     binding:"required"` // must match StNftTokenId
}

// AuctionFinalOutput is the JSON response from POST /proof/auctionFinal.
type AuctionFinalOutput struct {
	Proof        []*big.Int `json:"proof"`
	PublicSignal []*big.Int `json:"publicSignal"`
}
