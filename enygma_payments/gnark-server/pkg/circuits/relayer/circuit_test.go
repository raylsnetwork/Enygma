package relayer

// Unit tests for RelayerCircuit (verification plan step 2 in
// /Users/stephenyang/.claude/plans/atomic-churning-micali.md).
//
// These use a toy 80-public-signal inner circuit rather than the real
// EnygmaCircuit: no valid-witness builder exists anywhere in this repo for
// the transfer circuit's current field layout (the only pre-existing test,
// main_test.go, uses a stale/different struct shape — see the Step 0 spike
// history). RelayerCircuit's own logic (the recursive pairing check +
// native/emulated binding check) only depends on the inner proof's *shape*
// (80 public signals, BN254 Groth16 proof) not its domain semantics, so a
// toy inner circuit exercises RelayerCircuit itself faithfully.
//
// These use test.IsSolved (witness-satisfiability check only), NOT a full
// groth16.Setup()+Prove() on the outer circuit — Step 0 measured that at
// ~8 minutes, far too slow to pay per test case.

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/std/algebra/emulated/sw_bn254"
	"github.com/consensys/gnark/std/math/emulated"
	stdgroth16 "github.com/consensys/gnark/std/recursion/groth16"
	"github.com/consensys/gnark/test"
)

type toyInnerCircuit struct {
	Priv   [NPublicSignals]frontend.Variable
	Public [NPublicSignals]frontend.Variable `gnark:",public"`
}

func (c *toyInnerCircuit) Define(api frontend.API) error {
	for i := 0; i < NPublicSignals; i++ {
		api.AssertIsEqual(c.Priv[i], c.Public[i])
	}
	return nil
}

// setupToyInner compiles+sets up+proves the toy inner circuit once, with
// public values 1..NPublicSignals. Reused across sub-tests.
func setupToyInner(assert *test.Assert) (innerVK groth16.VerifyingKey, innerProof groth16.Proof, publicValues [NPublicSignals]*big.Int) {
	return setupToyInnerWithValues(assert, sequentialPublicValues())
}

// sequentialPublicValues returns 1..NPublicSignals — no zeros, used by most
// test cases above.
func sequentialPublicValues() [NPublicSignals]*big.Int {
	var v [NPublicSignals]*big.Int
	for i := range v {
		v[i] = big.NewInt(int64(i + 1))
	}
	return v
}

// zeroHeavyPublicValues mirrors the real EnygmaCircuit's actual public
// signal shape (see enygma/circuit.go's FingerPrintofSharedSecrets doc):
// indices 0-35 are the 6x6 FingerPrint matrix, mostly zero by design (5
// zeros then 1 nonzero, repeated 6x — diagonal/off-sender-column entries are
// explicitly skipped); indices 36-79 are ordinary nonzero values.
func zeroHeavyPublicValues() [NPublicSignals]*big.Int {
	var v [NPublicSignals]*big.Int
	for i := range v {
		if i < 36 && i%6 != 5 {
			v[i] = big.NewInt(0)
		} else {
			v[i] = big.NewInt(int64(i + 1))
		}
	}
	return v
}

func setupToyInnerWithValues(assert *test.Assert, publicValues [NPublicSignals]*big.Int) (innerVK groth16.VerifyingKey, innerProof groth16.Proof, out [NPublicSignals]*big.Int) {
	field := ecc.BN254.ScalarField()
	innerCcs, err := frontend.Compile(field, r1cs.NewBuilder, &toyInnerCircuit{})
	assert.NoError(err)

	innerPK, innerVK, err := groth16.Setup(innerCcs)
	assert.NoError(err)

	var assignment toyInnerCircuit
	for i := 0; i < NPublicSignals; i++ {
		assignment.Priv[i] = publicValues[i]
		assignment.Public[i] = publicValues[i]
	}
	innerWitnessFull, err := frontend.NewWitness(&assignment, field)
	assert.NoError(err)

	innerProof, err = groth16.Prove(innerCcs, innerPK, innerWitnessFull)
	assert.NoError(err)

	innerPubWitness, err := innerWitnessFull.Public()
	assert.NoError(err)
	assert.NoError(groth16.Verify(innerProof, innerVK, innerPubWitness))

	return innerVK, innerProof, publicValues
}

func TestRelayerCircuit_ValidProofAndBindingSucceed(t *testing.T) {
	assert := test.NewAssert(t)
	innerVK, innerProof, publicValues := setupToyInner(assert)

	field := ecc.BN254.ScalarField()
	innerCcs, err := frontend.Compile(field, r1cs.NewBuilder, &toyInnerCircuit{})
	assert.NoError(err)

	circuitVk, err := stdgroth16.ValueOfVerifyingKeyFixed[sw_bn254.G1Affine, sw_bn254.G2Affine, sw_bn254.GTEl](innerVK)
	assert.NoError(err)
	circuitProof, err := stdgroth16.ValueOfProof[sw_bn254.G1Affine, sw_bn254.G2Affine](innerProof)
	assert.NoError(err)

	// build the emulated inner witness directly from the known public values
	// (equivalent to ValueOfWitness on the native public witness).
	circuitWitness := stdgroth16.PlaceholderWitness[sw_bn254.ScalarField](innerCcs)
	for i := range publicValues {
		circuitWitness.Public[i] = emulated.ValueOf[sw_bn254.ScalarField](publicValues[i])
	}

	outerCircuit := &RelayerCircuit{
		InnerWitness: stdgroth16.PlaceholderWitness[sw_bn254.ScalarField](innerCcs),
		vk:           circuitVk,
	}
	outerAssignment := &RelayerCircuit{
		InnerWitness: circuitWitness,
		Proof:        circuitProof,
		vk:           circuitVk,
	}
	for i := range publicValues {
		outerAssignment.PublicSignal[i] = publicValues[i]
	}

	assert.NoError(test.IsSolved(outerCircuit, outerAssignment, field))
}

func TestRelayerCircuit_TamperedPublicSignalFails(t *testing.T) {
	assert := test.NewAssert(t)
	innerVK, innerProof, publicValues := setupToyInner(assert)

	field := ecc.BN254.ScalarField()
	innerCcs, err := frontend.Compile(field, r1cs.NewBuilder, &toyInnerCircuit{})
	assert.NoError(err)

	circuitVk, err := stdgroth16.ValueOfVerifyingKeyFixed[sw_bn254.G1Affine, sw_bn254.G2Affine, sw_bn254.GTEl](innerVK)
	assert.NoError(err)
	circuitProof, err := stdgroth16.ValueOfProof[sw_bn254.G1Affine, sw_bn254.G2Affine](innerProof)
	assert.NoError(err)

	circuitWitness := stdgroth16.PlaceholderWitness[sw_bn254.ScalarField](innerCcs)
	for i := range publicValues {
		circuitWitness.Public[i] = emulated.ValueOf[sw_bn254.ScalarField](publicValues[i])
	}

	outerCircuit := &RelayerCircuit{
		InnerWitness: stdgroth16.PlaceholderWitness[sw_bn254.ScalarField](innerCcs),
		vk:           circuitVk,
	}
	outerAssignment := &RelayerCircuit{
		InnerWitness: circuitWitness,
		Proof:        circuitProof,
		vk:           circuitVk,
	}
	for i := range publicValues {
		outerAssignment.PublicSignal[i] = publicValues[i]
	}
	// Tamper: publish a different value on the native PublicSignal than the
	// one actually bound in the (emulated) InnerWitness used for the pairing
	// check — this must be rejected by the binding check.
	outerAssignment.PublicSignal[0] = big.NewInt(999999)

	assert.Error(test.IsSolved(outerCircuit, outerAssignment, field))
}

func TestRelayerCircuit_WrongVerifyingKeyFails(t *testing.T) {
	assert := test.NewAssert(t)
	_, innerProof, publicValues := setupToyInner(assert)
	// Independent Setup() run on the identical circuit description — same
	// shape, genuinely different (wrong) VK due to Setup's randomness.
	wrongVK, _, _ := setupToyInner(assert)

	field := ecc.BN254.ScalarField()
	innerCcs, err := frontend.Compile(field, r1cs.NewBuilder, &toyInnerCircuit{})
	assert.NoError(err)

	circuitVk, err := stdgroth16.ValueOfVerifyingKeyFixed[sw_bn254.G1Affine, sw_bn254.G2Affine, sw_bn254.GTEl](wrongVK)
	assert.NoError(err)
	circuitProof, err := stdgroth16.ValueOfProof[sw_bn254.G1Affine, sw_bn254.G2Affine](innerProof)
	assert.NoError(err)

	circuitWitness := stdgroth16.PlaceholderWitness[sw_bn254.ScalarField](innerCcs)
	for i := range publicValues {
		circuitWitness.Public[i] = emulated.ValueOf[sw_bn254.ScalarField](publicValues[i])
	}

	outerCircuit := &RelayerCircuit{
		InnerWitness: stdgroth16.PlaceholderWitness[sw_bn254.ScalarField](innerCcs),
		vk:           circuitVk,
	}
	outerAssignment := &RelayerCircuit{
		InnerWitness: circuitWitness,
		Proof:        circuitProof, // proof made against the *right* VK's PK, checked against the *wrong* VK
		vk:           circuitVk,
	}
	for i := range publicValues {
		outerAssignment.PublicSignal[i] = publicValues[i]
	}

	assert.Error(test.IsSolved(outerCircuit, outerAssignment, field))
}

// TestRelayerCircuit_ZeroHeavyPublicSignalsSucceed is a regression test: the
// real EnygmaCircuit's public signal legitimately contains many exact-zero
// values (FingerPrint diagonal entries — see zeroHeavyPublicValues above).
// AssertProof's default MultiScalarMul strategy (jointScalarMulGLVUnsafe)
// divides by zero when it internally collides two points while combining
// zero scalars — reproduced directly against a real EnygmaCircuit-shaped
// zero pattern via /proof/relayer during end-to-end testing. Fixed by
// passing stdgroth16.WithCompleteArithmetic() in circuit.go's AssertProof
// call; this test guards against that fix being lost.
func TestRelayerCircuit_ZeroHeavyPublicSignalsSucceed(t *testing.T) {
	assert := test.NewAssert(t)
	innerVK, innerProof, publicValues := setupToyInnerWithValues(assert, zeroHeavyPublicValues())

	field := ecc.BN254.ScalarField()
	innerCcs, err := frontend.Compile(field, r1cs.NewBuilder, &toyInnerCircuit{})
	assert.NoError(err)

	circuitVk, err := stdgroth16.ValueOfVerifyingKeyFixed[sw_bn254.G1Affine, sw_bn254.G2Affine, sw_bn254.GTEl](innerVK)
	assert.NoError(err)
	circuitProof, err := stdgroth16.ValueOfProof[sw_bn254.G1Affine, sw_bn254.G2Affine](innerProof)
	assert.NoError(err)

	circuitWitness := stdgroth16.PlaceholderWitness[sw_bn254.ScalarField](innerCcs)
	for i := range publicValues {
		circuitWitness.Public[i] = emulated.ValueOf[sw_bn254.ScalarField](publicValues[i])
	}

	outerCircuit := &RelayerCircuit{
		InnerWitness: stdgroth16.PlaceholderWitness[sw_bn254.ScalarField](innerCcs),
		vk:           circuitVk,
	}
	outerAssignment := &RelayerCircuit{
		InnerWitness: circuitWitness,
		Proof:        circuitProof,
		vk:           circuitVk,
	}
	for i := range publicValues {
		outerAssignment.PublicSignal[i] = publicValues[i]
	}

	assert.NoError(test.IsSolved(outerCircuit, outerAssignment, field))
}
