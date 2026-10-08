package primitives

import (
	"math/big"

	pos "enygma_gnark_shared/poseidon"
	"github.com/consensys/gnark/frontend"
)

func Nullifier(api frontend.API, privateKey frontend.Variable, pathIndex frontend.Variable) frontend.Variable {
	return pos.Poseidon(api, []frontend.Variable{privateKey, pathIndex})
}

// NullifierTree computes the tree-bound nullifier every circuit spending from a
// vault must use:
//
//	nf = Poseidon(sk, treeNumber*2^treeDepth + pathIndex)
//
// Matches core.GetNullifierWithTree and enygma_dvp's NullifierTree exactly. The
// vault records spends by nullifier value only, so every circuit that can
// spend a given commitment (Payment family, DvpInitiator, DvpDestination) must
// derive the same value from it: two formulas would give one note two unspent
// nullifiers and let it be spent once through each. The vault address is bound
// separately in each circuit (StContractAddress), not through the nullifier.
func NullifierTree(api frontend.API, privateKey, treeNumber, pathIndex frontend.Variable, treeDepth int) frontend.Variable {
	capacity := new(big.Int).Lsh(big.NewInt(1), uint(treeDepth))
	globalIdx := api.Add(api.Mul(treeNumber, capacity), pathIndex)
	return pos.Poseidon(api, []frontend.Variable{privateKey, globalIdx})
}
