// Package primitives provides cryptographic primitives for the EnygmaDvp protocol.
//
// Deprecated: This package is deprecated and will be removed in a future version.
// Use Erc20Commitment.go instead

package primitives 

import(
	"github.com/consensys/gnark/frontend"
	 pos "gnark_server/poseidon"
)

func Commitment(api frontend.API, uniqueId frontend.Variable, publicKey frontend.Variable) frontend.Variable {
	return pos.Poseidon(api, []frontend.Variable{uniqueId, publicKey})
}

func CommitmentNative(api frontend.API, uniqueId frontend.Variable,publicKey frontend.Variable)frontend.Variable{

	commit,_:= api.NewHint(PoseidonNative, 1, uniqueId,publicKey)
		
	return commit[0]

}