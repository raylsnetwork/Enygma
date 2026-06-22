package auctionLock

import "math/big"

// AuctionLockRequest is the JSON body accepted by POST /proof/auctionLock.
//
// Fixed config: Merkle depth 8.
//
// Public statement returned: [stAuctionId, stTreeNumber, stMerkleRoot, stNullifier, stCommitLocked, stNftTokenId, stRevertCommit]
type AuctionLockRequest struct {
	StAuctionId    string    `json:"stAuctionId"    binding:"required"`
	StTreeNumber   string    `json:"stTreeNumber"   binding:"required"`
	StMerkleRoot   string    `json:"stMerkleRoot"   binding:"required"`
	StNullifier    string    `json:"stNullifier"    binding:"required"`
	StCommitLocked string    `json:"stCommitLocked" binding:"required"`
	StNftTokenId   string    `json:"stNftTokenId"   binding:"required"`
	StRevertCommit string    `json:"stRevertCommit" binding:"required"`

	WtSpendKey     string    `json:"wtSpendKey"     binding:"required"`
	WtTokenId      string    `json:"wtTokenId"      binding:"required"`
	WtSaltIn       string    `json:"wtSaltIn"       binding:"required"`
	WtPathElements [8]string `json:"wtPathElements" binding:"required"`
	WtPathIndex    string    `json:"wtPathIndex"    binding:"required"`
	WtSaltLocked   string    `json:"wtSaltLocked"   binding:"required"`
	WtSaltRevert   string    `json:"wtSaltRevert"   binding:"required"`
}

// AuctionLockOutput is the JSON response from POST /proof/auctionLock.
type AuctionLockOutput struct {
	Proof        []*big.Int `json:"proof"`
	PublicSignal []*big.Int `json:"publicSignal"`
}
