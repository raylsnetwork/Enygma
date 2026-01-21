package primitives 

import(
	"github.com/consensys/gnark/frontend"
	 pos "gnark_server/poseidon"
)


func Blinder(api frontend.API, in frontend.Variable)frontend.Variable{

	hasherInter:= pos.Poseidon(api, []frontend.Variable{in})

	hash,_ := api.NewHint(ModHint, 2,hasherInter)

	return hash[0]

}

