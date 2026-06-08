package primitives 

import(
	"github.com/consensys/gnark/frontend"
	 pos "gnark_server/poseidon"
)


func AuctionId(api frontend.API, commitment frontend.Variable) frontend.Variable {
	return pos.Poseidon(api, []frontend.Variable{commitment})
}