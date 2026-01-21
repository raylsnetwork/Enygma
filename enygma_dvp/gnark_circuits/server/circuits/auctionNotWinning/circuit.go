package auctionNotWinning


import(
	"math/big"
)
type AuctionNotWinningRequest struct {
	
	StAuctionId        			string 		`json:"stVaultId" binding:"required"`	
	StBlindedBidDifference      string      `json:"stBlindedBidDifference" binding:"required"`
	StBidBlockNumber     		string 		`json:"stBidBlockNumber" binding:"required"`
	StWinningBidBlockNumber     string      `json:"stWinningBidBlockNumber" binding:"required"`
	
	WtBidAmount      string 		`json:"wtBidAmount" binding:"required"`
	WtBidRandom      string 		`json:"wtBidRandom" binding:"required"`
	WtWinningBidAmount      string 		`json:"wtWinningBidAmount" binding:"required"`
	WtWinningBidRandom      string 		`json:"wtWinningBidRandom" binding:"required"`
	
}

type AuctionNotWinningOutput struct{
	Proof 			[]*big.Int `json:"proof"`
	PublicSignal    []*big.Int `json:"publicSignal"`
}
