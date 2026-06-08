package primitives 

import(
	"github.com/consensys/gnark/frontend"
	 pos "gnark_server/poseidon"
)

func Pedersen(api frontend.API, amount frontend.Variable, random frontend.Variable) frontend.Variable {
	return pos.Poseidon(api, []frontend.Variable{amount, random})
}