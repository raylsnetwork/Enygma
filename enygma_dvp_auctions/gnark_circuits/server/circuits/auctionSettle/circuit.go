package auctionSettle

import "math/big"

// AuctionSettleRequest is the JSON body accepted by POST /proof/auctionSettle.
//
// Only the auctioneer can call this endpoint — they hold sk_auction (ML-KEM
// decapsulation key) which is needed to recover WtSaltB from the winning bid.
//
// Public statement returned: [stAuctionId, stCommitB, stCommitWinnerNft, stNftTokenId, stWinningAmount]
type AuctionSettleRequest struct {
	StAuctionId       string `json:"stAuctionId"       binding:"required"`
	StCommitB         string `json:"stCommitB"         binding:"required"` // winning bid's USDC payout for Bob
	StCommitWinnerNft string `json:"stCommitWinnerNft" binding:"required"` // new NFT commitment for Alice
	StNftTokenId      string `json:"stNftTokenId"      binding:"required"` // NFT token ID
	StWinningAmount   string `json:"stWinningAmount"   binding:"required"` // public winning amount — threshold for losers

	// winning bid data — recovered via ML-KEM.Decapsulate(sk_auction, ctxt_1)
	// then HKDF and AEAD.Dec(k, ctxt_2, ctxt_1)
	WtPkBob       string `json:"wtPkBob"       binding:"required"`
	WtSaltB       string `json:"wtSaltB"       binding:"required"` // HKDF(ss, "note salt")
	WtBidAmount   string `json:"wtBidAmount"   binding:"required"`
	WtUsdcTokenId string `json:"wtUsdcTokenId" binding:"required"`

	// NFT transfer data
	WtPkAlice    string `json:"wtPkAlice"    binding:"required"` // Alice's spend pk (from decrypted bid)
	WtSaltAlice  string `json:"wtSaltAlice"  binding:"required"` // fresh random salt for Alice's NFT
	WtNftTokenId string `json:"wtNftTokenId" binding:"required"` // must match StNftTokenId
}

// AuctionSettleOutput is the JSON response from POST /proof/auctionSettle.
type AuctionSettleOutput struct {
	Proof        []*big.Int `json:"proof"`
	PublicSignal []*big.Int `json:"publicSignal"`
}
