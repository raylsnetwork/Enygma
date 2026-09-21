package primitives

import (
	"math/big"

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

// globalIdx computes treeNumber*2^treeDepth + pathIndex — the same folding
// core.GetNullifierWithTree already uses Go-side.
func globalIdx(api frontend.API, treeNumber, pathIndex frontend.Variable, treeDepth int) frontend.Variable {
	capacity := new(big.Int).Lsh(big.NewInt(1), uint(treeDepth))
	return api.Add(api.Mul(treeNumber, capacity), pathIndex)
}

// NullifierTree computes a tree-bound nullifier:
//
//	Poseidon(privateKey, treeNumber*2^treeDepth + pathIndex)
//
// Matches core.GetNullifierWithTree's Go-side formula exactly ("HIGH-1 fix":
// Poseidon(sk, pathIndex) alone is identical for the same slot in two
// different trees, enabling cross-tree double-spend). DvPInitiatorProof/
// DvPInitiatorProofFromSalts/DvPDestinationProof already compute nullifiers
// this way — this primitive is what makes DvPInitiatorCircuit/
// DvPDestinationCircuit's Define() actually check what those Go-side
// functions compute, instead of silently accepting the plain, non-tree-aware
// Nullifier() (which also left StTreeNumber completely unconstrained).
func NullifierTree(api frontend.API, privateKey, treeNumber, pathIndex frontend.Variable, treeDepth int) frontend.Variable {
	return pos.Poseidon(api, []frontend.Variable{privateKey, globalIdx(api, treeNumber, pathIndex, treeDepth)})
}

// NullifierBoundTree combines NullifierBound and NullifierTree:
//
//	Poseidon(privateKey, treeNumber*2^treeDepth + pathIndex, contractAddress)
//
// Matches core.GetNullifierBoundTree's Go-side formula exactly. Used by the
// Payment-family circuits, which need both bindings (StContractAddress and
// StTreeNumbers were both completely unconstrained before).
func NullifierBoundTree(api frontend.API, privateKey, treeNumber, pathIndex frontend.Variable, treeDepth int, contractAddress frontend.Variable) frontend.Variable {
	return pos.Poseidon(api, []frontend.Variable{privateKey, globalIdx(api, treeNumber, pathIndex, treeDepth), contractAddress})
}
