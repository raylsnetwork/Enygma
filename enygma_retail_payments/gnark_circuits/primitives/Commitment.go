package primitives

import (
	pos "enygma_retail_payments/gnark_circuits/poseidon"
	"github.com/consensys/gnark/frontend"
)

func Commitment(api frontend.API, uniqueId frontend.Variable, publicKey frontend.Variable) frontend.Variable {
	return pos.Poseidon(api, []frontend.Variable{uniqueId, publicKey})
}
