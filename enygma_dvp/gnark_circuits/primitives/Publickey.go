package primitives

import (
	pos "enygma_gnark_shared/poseidon"
	"github.com/consensys/gnark/frontend"
)

func PublicKey(api frontend.API, privateKey frontend.Variable) frontend.Variable {
	return pos.Poseidon(api, []frontend.Variable{privateKey})
}
