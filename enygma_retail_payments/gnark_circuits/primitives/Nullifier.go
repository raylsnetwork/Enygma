package primitives

import(
	"math/big"

	"github.com/consensys/gnark/frontend"
	 pos "gnark_server/poseidon"
)

func Nullifier(api frontend.API, privateKey frontend.Variable, pathIndex frontend.Variable) frontend.Variable {
	return pos.Poseidon(api, []frontend.Variable{privateKey, pathIndex})
}

// NullifierBound computes a nullifier bound to a specific contract deployment.
// Used by Payment.go, PaymentFee.go, PaymentRelayerFeePublic.go and
// UsdrFee.go — their StContractAddress public input was previously
// completely unconstrained (same bug class as PrivateMintCircuit's pre-fix
// ContractAddress, "Vuln 11") since they constrained their nullifier with
// the plain Nullifier() above instead of this one.
//
//	nf = Poseidon(sk, leafIndex, contractAddress)
//
// Including contractAddress prevents cross-deployment proof replay: a proof
// generated for vault A produces a different nullifier than the same note
// would produce for vault B — the property this primitive exists for.
func NullifierBound(api frontend.API, privateKey, pathIndex, contractAddress frontend.Variable) frontend.Variable {
	return pos.Poseidon(api, []frontend.Variable{privateKey, pathIndex, contractAddress})
}

// NullifierBoundTree additionally binds StTreeNumbers (also previously
// completely unconstrained in Payment.go/PaymentFee.go/
// PaymentRelayerFeePublic.go/UsdrFee.go):
//
//	nf = Poseidon(sk, treeNumber*2^treeDepth + pathIndex, contractAddress)
//
// Matches core.GetNullifierBoundTree's Go-side formula exactly — the same
// treeNumber*capacity+pathIndex folding core.GetNullifierWithTree already
// uses ("HIGH-1 fix": Poseidon(sk, pathIndex) alone is identical for the
// same slot in two different trees, enabling cross-tree double-spend).
func NullifierBoundTree(api frontend.API, privateKey, treeNumber, pathIndex frontend.Variable, treeDepth int, contractAddress frontend.Variable) frontend.Variable {
	capacity := new(big.Int).Lsh(big.NewInt(1), uint(treeDepth))
	globalIdx := api.Add(api.Mul(treeNumber, capacity), pathIndex)
	return pos.Poseidon(api, []frontend.Variable{privateKey, globalIdx, contractAddress})
}