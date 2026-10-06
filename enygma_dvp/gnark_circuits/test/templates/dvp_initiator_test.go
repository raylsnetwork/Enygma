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
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
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
	counterVault                         *big.Int
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

	// Any non-zero value; on-chain it is the counterparty vault's address.
	w.counterVault = new(big.Int).SetBytes([]byte{0xa3, 0x4b, 0x0b, 0xb5, 0xf2, 0xd8, 0xc1, 0x67})

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
		StCounterVault:  w.counterVault,

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

// TestDvpInitiatorCircuit_ZeroCounterVault_Fails: the expected counterparty
// vault must be set.
func TestDvpInitiatorCircuit_ZeroCounterVault_Fails(t *testing.T) {
	assert := test.NewAssert(t)
	w := buildValidDvpInitiatorWitness(t)
	wc := w.circuit()
	wc.Config = dvpInitiatorTestConfig()
	wc.StCounterVault = big.NewInt(0)
	assert.ProverFailed(emptyDvpInitiatorCircuit(), wc, test.WithCurves(ecc.BN254))
}

// TestDvpInitiatorCircuit_CounterVaultIsBoundByTheProof: a Groth16 proof made
// for one counterparty vault must not verify for another. A public input that
// appeared in no constraint would verify for any value; this is what the
// AssertIsDifferent constraint in Define guarantees against.
func TestDvpInitiatorCircuit_CounterVaultIsBoundByTheProof(t *testing.T) {
	w := buildValidDvpInitiatorWitness(t)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, emptyDvpInitiatorCircuit())
	if err != nil {
		t.Fatal(err)
	}
	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		t.Fatal(err)
	}
	wc := w.circuit()
	wc.Config = dvpInitiatorTestConfig()
	full, err := frontend.NewWitness(wc, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatal(err)
	}
	proof, err := groth16.Prove(ccs, pk, full)
	if err != nil {
		t.Fatal(err)
	}
	public := func(counterVault *big.Int) *templates.DvPInitiatorCircuit {
		c := w.circuit()
		c.Config = dvpInitiatorTestConfig()
		c.StCounterVault = counterVault
		return c
	}
	honest, err := frontend.NewWitness(public(w.counterVault), ecc.BN254.ScalarField(), frontend.PublicOnly())
	if err != nil {
		t.Fatal(err)
	}
	if err := groth16.Verify(proof, vk, honest); err != nil {
		t.Fatalf("proof does not verify with its own counterparty vault: %v", err)
	}
	other, err := frontend.NewWitness(public(new(big.Int).Add(w.counterVault, big.NewInt(1))), ecc.BN254.ScalarField(), frontend.PublicOnly())
	if err != nil {
		t.Fatal(err)
	}
	if err := groth16.Verify(proof, vk, other); err == nil {
		t.Fatal("VULNERABLE: the proof verified for a different counterparty vault")
	}
}
