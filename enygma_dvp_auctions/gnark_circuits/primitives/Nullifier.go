package primitives

import (
	pos "enygma_gnark_shared/poseidon"
	"github.com/consensys/gnark/frontend"
)

func Nullifier(api frontend.API, privateKey frontend.Variable, pathIndex frontend.Variable) frontend.Variable {
	return pos.Poseidon(api, []frontend.Variable{privateKey, pathIndex})
}
