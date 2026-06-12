package primitives

import (
	"github.com/consensys/gnark/frontend"
	pos "gnark_server/poseidon"
)

// Erc20Commitment computes the legacy ERC20 commitment (interactive flow).
// Deprecated: use Erc20CommitmentV2 for the non-interactive flow.
func Erc20Commitment(api frontend.API, contractAddress, amount, publicKey, salt frontend.Variable) frontend.Variable {
	return pos.Poseidon(api, []frontend.Variable{contractAddress, amount, publicKey, salt})
}

// Erc20CommitmentV2 computes the ERC20 commitment for the non-interactive flow.
//
//	C = Poseidon(pkSpend, saltBField, amount, tokenId)
//
// pkSpend    — recipient's spend public key = Poseidon(sk_spend)
// saltBField — KEM-derived shared secret reduced mod SNARK_SCALAR_FIELD
// amount     — token amount
// tokenId    — token identifier
func Erc20CommitmentV2(api frontend.API, pkSpend, saltBField, amount, tokenId frontend.Variable) frontend.Variable {
	return pos.Poseidon(api, []frontend.Variable{pkSpend, saltBField, amount, tokenId})
}
