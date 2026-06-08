package primitives

import (
	"github.com/consensys/gnark/frontend"
	pos "gnark_server/poseidon"
)

// Erc20CommitmentV2 computes the ERC20 commitment for the non-interactive flow.
//
//	C = Poseidon(pkSpend, saltBField, amount, tokenId)
//
// pkSpend    — recipient's spend public key = Poseidon(sk_spend)
// saltField  — commitment salt reduced mod SNARK_SCALAR_FIELD (KEM-derived for destination; random for change)
// amount     — token amount
// tokenId    — token identifier
func Erc20CommitmentV2(api frontend.API, pkSpend, saltBField, amount, tokenId frontend.Variable) frontend.Variable {
	return pos.Poseidon(api, []frontend.Variable{pkSpend, saltBField, amount, tokenId})
}
