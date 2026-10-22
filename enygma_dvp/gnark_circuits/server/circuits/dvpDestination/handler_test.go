package dvpDestination

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark/frontend"

	ht "enygma_dvp/gnark_circuits/server/circuits/internal/handlertest"
	"enygma_dvp/gnark_circuits/templates"
	"enygma_dvp/layout"
)

// TestHandler_ReturnsTheProofsPublicWitness drives the dvpDestination prove
// handler with throwaway keys: the public signals it returns must be the
// proof's own public witness, and the proof must verify against them. A
// request whose public values the witness does not satisfy gets an error.
func TestHandler_ReturnsTheProofsPublicWitness(t *testing.T) {
	pkPath, vkPath, vk := ht.Keys(t, newTestCircuit())
	handler := NewHandler(pkPath, vkPath)

	in := ht.NewNote(t, 12345, 100, 7, merkleDepth)
	alicePk := ht.Hash(t, big.NewInt(999))
	saltA, saltB := big.NewInt(444), big.NewInt(333)
	valueAlice, tokenIdAlice := big.NewInt(50), big.NewInt(9)
	commitA := ht.Commitment(t, alicePk, saltA, in.Value, in.TokenId)
	commitB := ht.Commitment(t, in.Pk, saltB, valueAlice, tokenIdAlice)

	var path [8]string
	copy(path[:], ht.Strings(in.Siblings...))
	request := DvPDestinationRequest{
		StMessage:      commitB.String(),
		StTreeNumber:   "0",
		StMerkleRoot:   in.Root.String(),
		StNullifier:    in.Nullifier.String(),
		StCommitA:      commitA.String(),
		StCounterVault: ht.VaultHex,
		WtSpendKeyIn:   in.Sk.String(),
		WtValueIn:      in.Value.String(),
		WtSaltIn:       in.Salt.String(),
		WtTokenIdIn:    in.TokenId.String(),
		WtPathElements: path,
		WtPathIndex:    "0",
		WtSpendPkAlice: alicePk.String(),
		WtSaltA:        saltA.String(),
		WtSaltB:        saltB.String(),
		WtValueAlice:   valueAlice.String(),
		WtTokenIdAlice: tokenIdAlice.String(),
	}
	proof, signals := ht.Post(t, handler, request)

	w := newTestCircuit()
	w.StMessage, w.StTreeNumber, w.StMerkleRoot, w.StNullifier = commitB, 0, in.Root, in.Nullifier
	w.StCommitA, w.StCounterVault = commitA, ht.Vault
	ht.AssertStatementLen(t, signals, layout.DvpDestinationLen)
	ht.AssertProvedWitness(t, w, vk, proof, signals)

	request.StNullifier = "1"
	ht.PostRejected(t, handler, request)
}

func newTestCircuit() *templates.DvPDestinationCircuit {
	return &templates.DvPDestinationCircuit{
		Config: templates.DvPDestinationCircuitConfig{
			TmMerkleTreeDepth: merkleDepth,
			TmRange:           frontend.Variable("1000000000000000000000000000000000000"),
		},
		WtPathElements: make([]frontend.Variable, merkleDepth),
	}
}
