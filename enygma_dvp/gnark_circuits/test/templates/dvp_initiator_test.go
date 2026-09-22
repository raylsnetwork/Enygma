// Property-based regression tests for DvPInitiatorCircuit, mirroring
// payment_test.go's approach. This directly validates the StTreeNumber fix
// (dd821c5) on the DvP swap side, where the bug was originally found: the
// plain Nullifier(sk, pathIndex) formula left StTreeNumber completely
// unconstrained, letting a prover claim any tree number for their input
// note independent of which tree it actually lives in. NullifierTree now
// binds StTreeNumber into the nullifier.
package test

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/test"
	"github.com/iden3/go-iden3-crypto/poseidon"

	"enygma_dvp/gnark_circuits/templates"
)

const dvpInitiatorTestDepth = 2

type dvpInitiatorTestWitness struct {
	skAlice, pkAlice                     *big.Int
	valueIn, saltIn, tokenIdIn           *big.Int
	treeNumber, pathIndex                *big.Int
	siblings                             []*big.Int
	root, nullifier                      *big.Int
	spendPkBob, saltB, commitB           *big.Int
	saltA, valueBob, tokenIdBob, commitA *big.Int
	revertSalt, revertCommitA            *big.Int
}

func buildValidDvpInitiatorWitness(t *testing.T) *dvpInitiatorTestWitness {
	t.Helper()
	w := &dvpInitiatorTestWitness{}

	w.skAlice = big.NewInt(111)
	pkAlice, err := poseidon.Hash([]*big.Int{w.skAlice})
	if err != nil {
		t.Fatal(err)
	}
	w.pkAlice = pkAlice

	w.valueIn = big.NewInt(100)
	w.saltIn = big.NewInt(222)
	w.tokenIdIn = big.NewInt(7)
	commitIn, err := poseidon.Hash([]*big.Int{w.pkAlice, w.saltIn, w.valueIn, w.tokenIdIn})
	if err != nil {
		t.Fatal(err)
	}

	w.treeNumber = big.NewInt(0)
	w.pathIndex = big.NewInt(0) // both path bits 0 → leaf always "left"
	w.siblings = []*big.Int{big.NewInt(888), big.NewInt(999)}
	root := commitIn
	for _, sib := range w.siblings {
		root = paymentTestHashLeftRight(root, sib)
	}
	w.root = root

	// globalIdx = treeNumber*2^depth + pathIndex = 0*4 + 0 = 0
	globalIdx := big.NewInt(0)
	w.nullifier, err = poseidon.Hash([]*big.Int{w.skAlice, globalIdx})
	if err != nil {
		t.Fatal(err)
	}

	skBob := big.NewInt(222222)
	spendPkBob, err := poseidon.Hash([]*big.Int{skBob})
	if err != nil {
		t.Fatal(err)
	}
	w.spendPkBob = spendPkBob
	w.saltB = big.NewInt(333)
	w.commitB, err = poseidon.Hash([]*big.Int{w.spendPkBob, w.saltB, w.valueIn, w.tokenIdIn})
	if err != nil {
		t.Fatal(err)
	}

	w.saltA = big.NewInt(444)
	w.valueBob = big.NewInt(50)
	w.tokenIdBob = big.NewInt(9)
	w.commitA, err = poseidon.Hash([]*big.Int{w.pkAlice, w.saltA, w.valueBob, w.tokenIdBob})
	if err != nil {
		t.Fatal(err)
	}

	w.revertSalt = big.NewInt(555)
	w.revertCommitA, err = poseidon.Hash([]*big.Int{w.pkAlice, w.revertSalt, w.valueIn, w.tokenIdIn})
	if err != nil {
		t.Fatal(err)
	}

	return w
}

func (w *dvpInitiatorTestWitness) circuit() *templates.DvPInitiatorCircuit {
	return &templates.DvPInitiatorCircuit{
		StMessage:       w.commitA, // circuit requires StMessage == StCommitA
		StTreeNumber:    w.treeNumber,
		StMerkleRoot:    w.root,
		StNullifier:     w.nullifier,
		StCommitB:       w.commitB,
		StCommitA:       w.commitA,
		StRevertCommitA: w.revertCommitA,

		WtSpendKeyIn:   w.skAlice,
		WtValueIn:      w.valueIn,
		WtSaltIn:       w.saltIn,
		WtTokenIdIn:    w.tokenIdIn,
		WtPathElements: []frontend.Variable{w.siblings[0], w.siblings[1]},
		WtPathIndex:    w.pathIndex,

		WtSpendPkBob: w.spendPkBob,
		WtSaltB:      w.saltB,
		WtValueBob:   w.valueBob,
		WtTokenIdBob: w.tokenIdBob,
		WtSaltA:      w.saltA,
		WtRevertSalt: w.revertSalt,
	}
}

func dvpInitiatorTestConfig() templates.DvPInitiatorCircuitConfig {
	return templates.DvPInitiatorCircuitConfig{
		TmMerkleTreeDepth: dvpInitiatorTestDepth,
		TmRange:           frontend.Variable("1000000000000000000000000000000000000"),
	}
}

func emptyDvpInitiatorCircuit() *templates.DvPInitiatorCircuit {
	cfg := dvpInitiatorTestConfig()
	return &templates.DvPInitiatorCircuit{
		Config:         cfg,
		WtPathElements: make([]frontend.Variable, cfg.TmMerkleTreeDepth),
	}
}

func TestDvpInitiatorCircuit_ValidWitness_Succeeds(t *testing.T) {
	assert := test.NewAssert(t)
	w := buildValidDvpInitiatorWitness(t)
	wc := w.circuit()
	wc.Config = dvpInitiatorTestConfig()
	assert.ProverSucceeded(emptyDvpInitiatorCircuit(), wc, test.WithCurves(ecc.BN254))
}

// TestDvpInitiatorCircuit_TamperedTreeNumber_Fails locks in the StTreeNumber
// fix: changing StTreeNumber alone, without recomputing the nullifier to
// match, must fail — before the fix, StTreeNumber was never referenced
// anywhere in Define() and this witness would have satisfied the circuit
// regardless.
func TestDvpInitiatorCircuit_TamperedTreeNumber_Fails(t *testing.T) {
	assert := test.NewAssert(t)
	w := buildValidDvpInitiatorWitness(t)
	wc := w.circuit()
	wc.Config = dvpInitiatorTestConfig()
	wc.StTreeNumber = big.NewInt(1) // nullifier still bound to treeNumber=0
	assert.ProverFailed(emptyDvpInitiatorCircuit(), wc, test.WithCurves(ecc.BN254))
}
