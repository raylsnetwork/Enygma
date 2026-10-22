package privateMint

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark/frontend"

	ht "enygma_dvp/gnark_circuits/server/circuits/internal/handlertest"
	"enygma_dvp/gnark_circuits/templates"
	"enygma_dvp/layout"
)

// TestHandler_ReturnsTheProofsPublicWitness drives the privateMint prove
// handler with throwaway keys: the public signals it returns must be the
// proof's own public witness, and the proof must verify against them. A
// request whose public values the witness does not satisfy gets an error.
func TestHandler_ReturnsTheProofsPublicWitness(t *testing.T) {
	empty := templates.PrivateMintCircuit{
		Config: templates.PrivateMintConfig{TmRange: frontend.Variable("1000000000000000000000000000000000000")},
	}
	pkPath, vkPath, vk := ht.Keys(t, &empty)
	handler := NewHandler(pkPath, vkPath)

	pk := ht.Hash(t, big.NewInt(12345))
	salt, amount, tokenId := big.NewInt(111), big.NewInt(100), big.NewInt(7)
	cmt := ht.Commitment(t, pk, salt, amount, tokenId)
	tag := ht.Hash(t, pk, salt, ht.Vault)

	request := PrivateMintRequest{
		Commitment:      cmt.String(),
		ContractAddress: ht.VaultHex,
		TokenId:         tokenId.String(),
		Salt:            salt.String(),
		Amount:          amount.String(),
		PublicKey:       pk.String(),
		CipherText:      tag.String(),
	}
	proof, signals := ht.Post(t, handler, request)

	w := templates.PrivateMintCircuit{Commitment: cmt, ContractAddress: ht.Vault, TokenId: tokenId, CipherText: tag}
	ht.AssertStatementLen(t, signals, layout.PrivateMintLen)
	ht.AssertProvedWitness(t, &w, vk, proof, signals)

	request.Commitment = "1"
	ht.PostRejected(t, handler, request)
}
