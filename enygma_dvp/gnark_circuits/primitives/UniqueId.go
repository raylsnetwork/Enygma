package primitives 

import(
	"github.com/consensys/gnark/frontend"
	 pos "gnark_server/poseidon"
)


func UniqueId(api frontend.API, contractAddress frontend.Variable,amount frontend.Variable)frontend.Variable{

	hasher:= pos.Poseidon(api, []frontend.Variable{contractAddress,amount})

	hasherOut,_ := api.NewHint(ModHint, 2,hasher)
	
	return hasherOut[0]

}