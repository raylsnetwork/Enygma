package auctionPrivateOpening


import(
	"math/big"
)

type AuctionPrivateOpeningRequest struct {
	
	StVaultId        			string 		`json:"stVaultId" binding:"required"`	
	StBlindedBid      			string      `json:"stBlindedBid" binding:"required"`
	
	WtBidAmount      string 		`json:"wtBidAmount" binding:"required"`
	WtBidRandom      string 		`json:"wtBidRandom" binding:"required"`

	
}

type AuctionPrivateOpeningROutput struct{
	Proof 			[]*big.Int `json:"proof"`
	PublicSignal    []*big.Int `json:"publicSignal"`
}
