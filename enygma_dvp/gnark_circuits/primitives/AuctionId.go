package primitives 

import(
	"github.com/consensys/gnark/frontend"
	 pos "gnark_server/poseidon"
)


func AuctionId(api frontend.API, commitment frontend.Variable)frontend.Variable{

	idInter:= pos.Poseidon(api, []frontend.Variable{commitment})
	id,_ := api.NewHint(ModHint, 2,idInter)
	return id[0]

}