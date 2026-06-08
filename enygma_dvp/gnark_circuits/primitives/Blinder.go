package primitives 

import(
	"github.com/consensys/gnark/frontend"
	 pos "gnark_server/poseidon"
)


func Blinder(api frontend.API, in frontend.Variable) frontend.Variable {
	return pos.Poseidon(api, []frontend.Variable{in})
}

