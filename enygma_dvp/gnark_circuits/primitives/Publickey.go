package primitives 

import(
	"github.com/consensys/gnark/frontend"
	 pos "gnark_server/poseidon"
)


func PublicKey(api frontend.API, privateKey frontend.Variable) frontend.Variable {
	return pos.Poseidon(api, []frontend.Variable{privateKey})
}
