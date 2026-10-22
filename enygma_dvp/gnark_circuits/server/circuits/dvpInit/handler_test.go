package dvpInit

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark/frontend"

	ht "enygma_dvp/gnark_circuits/server/circuits/internal/handlertest"
	"enygma_dvp/gnark_circuits/templates"
	"enygma_dvp/layout"
)

// TestHandler_ReturnsTheProofsPublicWitness drives the dvpInitiator prove
// handler with throwaway keys: the public signals it returns must be the
// proof's own public witness, and the proof must verify against them. A
// request whose public values the witness does not satisfy gets an error.
func TestHandler_ReturnsTheProofsPublicWitness(t *testing.T) {
	pkPath, vkPath, vk := ht.Keys(t, newTestCircuit())
	handler := NewHandler(pkPath, vkPath)

	in := ht.NewNote(t, 12345, 100, 7, merkleDepth)
	bobPk := ht.Hash(t, big.NewInt(999))
	saltB, saltA, revertSalt := big.NewInt(333), big.NewInt(444), big.NewInt(555)
	valueBob, tokenIdBob := big.NewInt(50), big.NewInt(9)
	commitB := ht.Commitment(t, bobPk, saltB, in.Value, in.TokenId)
	commitA := ht.Commitment(t, in.Pk, saltA, valueBob, tokenIdBob)
	revertCommitA := ht.Commitment(t, in.Pk, revertSalt, in.Value, in.TokenId)

	var path [8]string
	copy(path[:], ht.Strings(in.Siblings...))
	request := DvPInitiatorRequest{
		StMessage:       commitA.String(),
		StTreeNumber:    "0",
		StMerkleRoot:    in.Root.String(),
		StNullifier:     in.Nullifier.String(),
		StCommitB:       commitB.String(),
		StCommitA:       commitA.String(),
		StRevertCommitA: revertCommitA.String(),
		StCounterVault:  ht.VaultHex,
		WtSpendKeyIn:    in.Sk.String(),
		WtValueIn:       in.Value.String(),
		WtSaltIn:        in.Salt.String(),
		WtTokenIdIn:     in.TokenId.String(),
		WtPathElements:  path,
		WtPathIndex:     "0",
		WtSpendPkBob:    bobPk.String(),
		WtSaltB:         saltB.String(),
		WtValueBob:      valueBob.String(),
		WtTokenIdBob:    tokenIdBob.String(),
		WtSaltA:         saltA.String(),
		WtRevertSalt:    revertSalt.String(),
	}
	proof, signals := ht.Post(t, handler, request)

	w := newTestCircuit()
	w.StMessage, w.StTreeNumber, w.StMerkleRoot, w.StNullifier = commitA, 0, in.Root, in.Nullifier
	w.StCommitB, w.StCommitA, w.StRevertCommitA, w.StCounterVault = commitB, commitA, revertCommitA, ht.Vault
	ht.AssertStatementLen(t, signals, layout.DvpInitiatorLen)
	ht.AssertProvedWitness(t, w, vk, proof, signals)

	request.StNullifier = "1"
	ht.PostRejected(t, handler, request)
}

func newTestCircuit() *templates.DvPInitiatorCircuit {
	return &templates.DvPInitiatorCircuit{
		Config: templates.DvPInitiatorCircuitConfig{
			TmMerkleTreeDepth: merkleDepth,
			TmRange:           frontend.Variable("1000000000000000000000000000000000000"),
		},
		WtPathElements: make([]frontend.Variable, merkleDepth),
	}
}
