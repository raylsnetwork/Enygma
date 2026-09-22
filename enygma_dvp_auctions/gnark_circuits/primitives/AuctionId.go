package primitives

import (
	pos "enygma_dvp_auctions/gnark_circuits/poseidon"
	"github.com/consensys/gnark/frontend"
)

func AuctionId(api frontend.API, commitment frontend.Variable) frontend.Variable {
	return pos.Poseidon(api, []frontend.Variable{commitment})
}
