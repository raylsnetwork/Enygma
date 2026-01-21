package templates

import(

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/cmp"
	"gnark_server/primitives"

)

const RangeAuctionNotWinning = "1000000000000000000000000000000000000"


type AuctionNotWinningCircuitConfig struct{
	TmRange frontend.Variable
}
 
type AuctionNotWinningCircuit struct {
	
	Config 					 AuctionNotWinningCircuitConfig
	StAuctionId      		 frontend.Variable    `gnark:",public"` 
	StBlindedBidDifference   frontend.Variable    `gnark:",public"` 
	StBidBlockNumber  		 frontend.Variable    `gnark:",public"` 
	StWinningBidBlockNumber  frontend.Variable	  `gnark:",public"` 
	
	WtBidAmount				 frontend.Variable
	WtBidRandom    		     frontend.Variable
	WtWinningBidAmount    	 frontend.Variable
	WtWinningBidRandom    	 frontend.Variable
	
}

func (circuit *AuctionNotWinningCircuit) Define(api frontend.API) error{


	isValid0 := cmp.IsLess(api, circuit.WtBidAmount,circuit.Config.TmRange)
	api.AssertIsEqual(isValid0, 1)

	isValid1 := cmp.IsLess(api, 0,circuit.WtBidAmount )
	api.AssertIsEqual(isValid1, 1)

	isValid2 := cmp.IsLess(api, circuit.WtWinningBidAmount,circuit.Config.TmRange )
	api.AssertIsEqual(isValid2, 1)

	isValid3 := cmp.IsLess(api, 0,circuit.WtWinningBidAmount)
	api.AssertIsEqual(isValid3, 1)
	
	pedersen := primitives.Pedersen(api,circuit.WtBidAmount, circuit.WtBidRandom)
	pedersenWinning := primitives.Pedersen(api,circuit.WtWinningBidAmount, circuit.WtWinningBidRandom)

	api.AssertIsEqual(circuit.StBlindedBidDifference,api.Sub(pedersenWinning,pedersen))

	isValid04 := cmp.IsLessOrEqual(api, circuit.WtBidAmount,circuit.WtWinningBidAmount )
	api.AssertIsEqual(isValid04,1)
	return nil
}

