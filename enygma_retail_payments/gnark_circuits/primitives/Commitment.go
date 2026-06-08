package primitives

import (
	"github.com/consensys/gnark/frontend"
	pos "gnark_server/poseidon"
)

func Commitment(api frontend.API, uniqueId frontend.Variable, publicKey frontend.Variable) frontend.Variable {
	return pos.Poseidon(api, []frontend.Variable{uniqueId, publicKey})
}