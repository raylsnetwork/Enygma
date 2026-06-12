package primitives

import (
	"github.com/consensys/gnark/frontend"
)

// Erc721Commitment computes the ERC721 commitment using the unified V2 formula:
//
//	C = Poseidon(pk_spend, salt, 1, tokenId)
//
// The amount is always 1 for non-fungible tokens.
func Erc721Commitment(api frontend.API, tokenId, publicKey, salt frontend.Variable) frontend.Variable {
	return Erc20CommitmentV2(api, publicKey, salt, frontend.Variable(1), tokenId)
}
