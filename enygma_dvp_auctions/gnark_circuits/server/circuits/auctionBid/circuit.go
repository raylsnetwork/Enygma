package auctionBid

import "math/big"

// AuctionBidRequest is the JSON body accepted by POST /proof/auctionBid.
//
// Fixed config: Merkle depth 8, all-in bid (bidAmount == note amount).
//
// Public statement returned: [stAuctionId, stTreeNumber, stMerkleRoot, stNullifier, stCommitA, stCommitB]
type AuctionBidRequest struct {
	StAuctionId  string    `json:"stAuctionId"  binding:"required"`
	StTreeNumber string    `json:"stTreeNumber" binding:"required"`
	StMerkleRoot string    `json:"stMerkleRoot" binding:"required"`
	StNullifier  string    `json:"stNullifier"  binding:"required"`
	StCommitA    string    `json:"stCommitA"    binding:"required"` // Alice's locked bid
	StCommitB    string    `json:"stCommitB"    binding:"required"` // Bob's USDC payout destination

	WtSpendKey     string    `json:"wtSpendKey"     binding:"required"`
	WtAmount       string    `json:"wtAmount"       binding:"required"`
	WtTokenId      string    `json:"wtTokenId"      binding:"required"`
	WtSaltIn       string    `json:"wtSaltIn"       binding:"required"`
	WtPathElements [8]string `json:"wtPathElements" binding:"required"`
	WtPathIndex    string    `json:"wtPathIndex"    binding:"required"`
	WtSpendPkBob   string    `json:"wtSpendPkBob"   binding:"required"`
	WtSaltA        string    `json:"wtSaltA"        binding:"required"`
	WtSaltB        string    `json:"wtSaltB"        binding:"required"` // HKDF(ML-KEM ss, "note salt")
	WtBidAmount    string    `json:"wtBidAmount"    binding:"required"`
}

// AuctionBidOutput is the JSON response from POST /proof/auctionBid.
type AuctionBidOutput struct {
	Proof        []*big.Int `json:"proof"`
	PublicSignal []*big.Int `json:"publicSignal"`
}
