package templates

import(
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/cmp"
	"gnark_server/primitives"
)

const AuctionOpeningRange = "1000000000000000000000000000000000000"


type AuctionPrivateOpeningCircuitConfig struct{
	TmRange frontend.Variable
}
 

type AuctionPrivateOpeningCircuit struct {
	Config 					 AuctionPrivateOpeningCircuitConfig
	StVaultId      		     frontend.Variable    `gnark:",public"`
	StBlindedBid   			 frontend.Variable    `gnark:",public"`	 

	WtBidAmount				 frontend.Variable    
	WtBidRandom    		     frontend.Variable    

	
}

func (circuit *AuctionPrivateOpeningCircuit) Define(api frontend.API) error{

	isValid0 := cmp.IsLess(api, circuit.WtBidAmount, circuit.Config.TmRange)
	api.AssertIsEqual(isValid0, 1)

	isValid1 := cmp.IsLessOrEqual(api, 0,circuit.WtBidAmount )
	api.AssertIsEqual(isValid1, 1)

	isValid2 := cmp.IsLessOrEqual(api, circuit.WtBidAmount,circuit.Config.TmRange )
	api.AssertIsEqual(isValid2, 1)

	isValid3 := cmp.IsLessOrEqual(api, 0,circuit.WtBidAmount)
	api.AssertIsEqual(isValid3, 1)
	
	pedersen := primitives.Pedersen(api,circuit.WtBidAmount, circuit.WtBidRandom)
	
	api.AssertIsEqual(pedersen,circuit.StBlindedBid)
	return nil

}

