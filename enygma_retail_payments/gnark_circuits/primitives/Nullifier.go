package primitives

import(
	"github.com/consensys/gnark/frontend"
	 pos "gnark_server/poseidon"
)

func Nullifier(api frontend.API, privateKey frontend.Variable, pathIndex frontend.Variable) frontend.Variable {
	return pos.Poseidon(api, []frontend.Variable{privateKey, pathIndex})
}

// NullifierBound computes a nullifier bound to a specific contract deployment.
// Not currently used by any template — Payment.go, PaymentFee.go and
// PaymentRelayerFeePublic.go all constrain their nullifier with the plain
// Nullifier() above instead.
//
//	nf = Poseidon(sk, leafIndex, contractAddress)
//
// Including contractAddress prevents cross-deployment proof replay: a proof
// generated for vault A produces a different nullifier than the same note
// would produce for vault B — the property this primitive exists for.
func NullifierBound(api frontend.API, privateKey, pathIndex, contractAddress frontend.Variable) frontend.Variable {
	return pos.Poseidon(api, []frontend.Variable{privateKey, pathIndex, contractAddress})
}