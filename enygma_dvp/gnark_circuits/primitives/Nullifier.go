package primitives 

import(
	"github.com/consensys/gnark/frontend"
	 pos "gnark_server/poseidon"
)

func Nullifier(api frontend.API, privateKey frontend.Variable,pathIndex frontend.Variable)frontend.Variable{

	hasher:= pos.Poseidon(api, []frontend.Variable{privateKey,pathIndex})
	nullifier,_ := api.NewHint(ModHint, 2,hasher)
	return nullifier[0]

}