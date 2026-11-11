package privateMint

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark/backend/groth16"

	ht "enygma_retail_payments/gnark_circuits/server/circuits/internal/handlertest"
	"enygma_retail_payments/gnark_circuits/templates"
)

// TestHandler_ReturnsTheProofsPublicWitness drives the privateMint prove
// handler with throwaway keys: the public signals it returns must be the
// proof's own public witness, and the proof must verify against them.
func TestHandler_ReturnsTheProofsPublicWitness(t *testing.T) {
	pkPath, vkPath, vk := keys(t)
	c := privateMintCase(t, 111)
	proof, signals := ht.Post(t, NewHandler(pkPath, vkPath), c.Request)
	ht.AssertProvedWitness(t, c.Assignment, vk, proof, signals)
}

// TestHandler_ProvesConcurrentRequests sends distinct mints to one handler
// at once: each must come back proving its own.
func TestHandler_ProvesConcurrentRequests(t *testing.T) {
	pkPath, vkPath, vk := keys(t)
	var cases []ht.Case
	for salt := int64(1); salt <= 6; salt++ {
		cases = append(cases, privateMintCase(t, salt))
	}
	ht.AssertConcurrentProofs(t, NewHandler(pkPath, vkPath), vk, cases)
}

func keys(t *testing.T) (pkPath, vkPath string, vk groth16.VerifyingKey) {
	t.Helper()
	empty := templates.PrivateMintCircuit{}
	return ht.Keys(t, &empty)
}

// privateMintCase mints 100 of token 7 to one key, the note salted with
// noteSalt.
func privateMintCase(t *testing.T, noteSalt int64) ht.Case {
	t.Helper()
	pk := ht.Hash(t, big.NewInt(12345))
	salt, amount, tokenId := big.NewInt(noteSalt), big.NewInt(100), big.NewInt(7)
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
	w := templates.PrivateMintCircuit{Commitment: cmt, ContractAddress: ht.Vault, TokenId: tokenId, CipherText: tag}
	return ht.Case{Request: request, Assignment: &w}
}
