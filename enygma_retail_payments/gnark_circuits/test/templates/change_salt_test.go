// Regression tests for the change salt rule (assertChangeSalt). A payment's
// change salt used to be random, so the change note could be opened only with
// the salt the wallet stored: if the receipt was settled first by someone
// else with replaced ciphertexts, the sender's own transaction reverted while
// the input was spent, and a wallet that then dropped the payment's local
// data lost the change note. Every Payment-family proof, a payment as well as
// a withdrawal, now has to salt its change note (output[1]) with
// Poseidon(sk, nullifier0), which the owner can rebuild from the key and the
// spent nullifier on chain.
package test

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/test"

	"enygma_retail_payments/gnark_circuits/templates"
)

// randomChangeSalt is a change salt a wallet could have picked at random.
var randomChangeSalt = big.NewInt(555)

// withChangeSalt returns the change commitment that salt gives the change
// note (owner pk, value and token as in the valid witness).
func withChangeSalt(t *testing.T, pk, value, token frontend.Variable, salt *big.Int) *big.Int {
	t.Helper()
	return boundHash(t, pk.(*big.Int), salt, value.(*big.Int), token.(*big.Int))
}

// TestPaymentCircuit_RandomChangeSalt_Fails: a payment (StMessage 0) whose
// change note has a random salt, its commitment recomputed to match, no
// longer proves; the valid witness, with the derived salt, does
// (TestPaymentCircuit_ValidWitness_Succeeds).
func TestPaymentCircuit_RandomChangeSalt_Fails(t *testing.T) {
	assert := test.NewAssert(t)
	w := buildValidPaymentWitness(t)
	wc := w.circuit()
	wc.Config = paymentTestConfig()
	wc.WtSaltsOut = []frontend.Variable{w.saltOut0, randomChangeSalt}
	wc.StCommitmentsOut = []frontend.Variable{w.cmt0, withChangeSalt(t, w.pkOut1, w.valueOut1, w.tokenId, randomChangeSalt)}
	assert.ProverFailed(emptyPaymentCircuit(), wc, test.WithCurves(ecc.BN254))
}

// TestPaymentCircuit_DerivedChangeSaltOfAnotherNullifier_Fails: the salt must
// be derived from this proof's own first nullifier, not any nullifier of sk.
func TestPaymentCircuit_DerivedChangeSaltOfAnotherNullifier_Fails(t *testing.T) {
	assert := test.NewAssert(t)
	w := buildValidPaymentWitness(t)
	other := boundHash(t, w.sk, boundHash(t, w.sk, big.NewInt(1)))
	wc := w.circuit()
	wc.Config = paymentTestConfig()
	wc.WtSaltsOut = []frontend.Variable{w.saltOut0, other}
	wc.StCommitmentsOut = []frontend.Variable{w.cmt0, withChangeSalt(t, w.pkOut1, w.valueOut1, w.tokenId, other)}
	assert.ProverFailed(emptyPaymentCircuit(), wc, test.WithCurves(ecc.BN254))
}

// TestPaymentCircuit_RandomWithdrawChangeSalt_Fails keeps the withdrawal rule
// the payment rule grew out of: a withdrawal's change salt is derived too.
func TestPaymentCircuit_RandomWithdrawChangeSalt_Fails(t *testing.T) {
	sk := big.NewInt(4242)
	empty := templates.NewPaymentCircuit(templates.PaymentConfig)
	recipient := big.NewInt(0xab)
	derived := spendOneNote(t, templates.MessageWithdraw, sk, big.NewInt(100), recipient, big.NewInt(60), big.NewInt(0), big.NewInt(40), nil)
	if err := test.IsSolved(&empty, derived, ecc.BN254.ScalarField()); err != nil {
		t.Fatalf("a withdrawal with the derived change salt did not solve: %v", err)
	}
	random := spendOneNote(t, templates.MessageWithdraw, sk, big.NewInt(100), recipient, big.NewInt(60), big.NewInt(0), big.NewInt(40), randomChangeSalt)
	if err := test.IsSolved(&empty, random, ecc.BN254.ScalarField()); err == nil {
		t.Fatal("a withdrawal with a random change salt solved")
	}
}

// TestPaymentCircuit_PaddingFirstInput_Fails: nullifier0 must be a real
// note's, unique and on chain, or the derived salt would repeat across
// proofs (Poseidon(sk, 0) for every one): a proof whose input 0 is
// zero-value padding fails even with that salt.
func TestPaymentCircuit_PaddingFirstInput_Fails(t *testing.T) {
	sk := big.NewInt(12345)
	pk := boundHash(t, sk)
	token := big.NewInt(7)
	zero := big.NewInt(0)
	salt1 := boundHash(t, sk, zero)

	cfg := templates.PaymentConfig
	a := templates.NewPaymentCircuit(cfg)
	a.StMessage = 0
	a.StTreeNumbers[0], a.StMerkleRoots[0], a.StNullifiers[0] = zero, zero, zero
	a.StCommitmentsOut[0] = boundHash(t, pk, big.NewInt(444), zero, token)
	a.StCommitmentsOut[1] = boundHash(t, pk, salt1, zero, token)
	a.StContractAddress = big.NewInt(0xABCD)
	a.WtPrivateKeysIn[0], a.WtValuesIn[0], a.WtSaltsIn[0] = sk, zero, zero
	sibs := make([]frontend.Variable, cfg.TmMerkleTreeDepth)
	for i := range sibs {
		sibs[i] = zero
	}
	a.WtPathElements[0], a.WtPathIndices[0] = sibs, zero
	a.WtTokenId = token
	a.WtSpendPublicKeysOut[0], a.WtSpendPublicKeysOut[1] = pk, pk
	a.WtValuesOut[0], a.WtValuesOut[1] = zero, zero
	a.WtSaltsOut[0], a.WtSaltsOut[1] = big.NewInt(444), salt1

	empty := templates.NewPaymentCircuit(cfg)
	if err := test.IsSolved(&empty, &a, ecc.BN254.ScalarField()); err == nil {
		t.Fatal("a payment whose first input is padding solved")
	}
}

// The fee circuits carry the same rule for their change note, output[1].

func TestPaymentFeeCircuit_RandomChangeSalt_Fails(t *testing.T) {
	assert := test.NewAssert(t)
	wc, _ := buildValidPaymentFeeWitness(t)
	wc.StCommitmentsOut[1] = withChangeSalt(t, wc.WtSpendPublicKeysOut[1], wc.WtValuesOut[1], wc.WtTokenId, randomChangeSalt)
	wc.WtSaltsOut[1] = randomChangeSalt
	assert.ProverFailed(emptyPaymentFeeCircuit(), wc, test.WithCurves(ecc.BN254))
}

func TestPaymentRelayerFeePublicCircuit_RandomChangeSalt_Fails(t *testing.T) {
	assert := test.NewAssert(t)
	wc := buildValidPaymentRelayerFeePublicWitness(t)
	wc.StCommitmentsOut[1] = withChangeSalt(t, wc.WtSpendPublicKeysOut[1], wc.WtValuesOut[1], wc.WtTokenId, randomChangeSalt)
	wc.WtSaltsOut[1] = randomChangeSalt
	assert.ProverFailed(emptyPaymentRelayerFeePublicCircuit(), wc, test.WithCurves(ecc.BN254))
}

func TestUsdrFeeCircuit_RandomChangeSalt_Fails(t *testing.T) {
	assert := test.NewAssert(t)
	wc := buildValidUsdrFeeWitness(t)
	wc.StCommitmentsOut[1] = withChangeSalt(t, wc.WtSpendPublicKeysOut[1], wc.WtValuesOut[1], wc.StTokenId, randomChangeSalt)
	wc.WtSaltsOut[1] = randomChangeSalt
	assert.ProverFailed(emptyUsdrFeeCircuit(), wc, test.WithCurves(ecc.BN254))
}
