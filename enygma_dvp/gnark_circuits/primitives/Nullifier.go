package primitives

import (
	"github.com/consensys/gnark/frontend"
	pos "gnark_server/poseidon"
)

func Nullifier(api frontend.API, privateKey frontend.Variable, pathIndex frontend.Variable) frontend.Variable {
	return pos.Poseidon(api, []frontend.Variable{privateKey, pathIndex})
}

// NullifierBound computes a contract-address-bound nullifier:
//
//	Poseidon(privateKey, pathIndex, contractAddress)
//
// Matches core.GetNullifierBound's Go-side formula exactly. Binds the proof
// to a specific vault contract — without this, a circuit's StContractAddress
// public input (the vault address) is completely unconstrained, letting a
// prover retarget an otherwise-identical proof at a different vault
// deployment (the same bug class as PrivateMintCircuit's pre-fix
// ContractAddress, "Vuln 11").
func NullifierBound(api frontend.API, privateKey frontend.Variable, pathIndex frontend.Variable, contractAddress frontend.Variable) frontend.Variable {
	return pos.Poseidon(api, []frontend.Variable{privateKey, pathIndex, contractAddress})
}