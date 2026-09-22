// Property-based regression tests for PaymentCircuit, using gnark's own
// test.ProverSucceeded/ProverFailed helpers instead of a manual
// Compile/Setup/Prove/Verify sequence.
//
// These exist specifically to lock in two real, previously-shipped bugs
// found this session by manually auditing every circuit's public inputs for
// "is this field actually referenced in Define()":
//
//   - StContractAddress was completely unconstrained (a prover could pick
//     any vault address at proof-generation time).
//   - StTreeNumbers[i] was completely unconstrained (a prover could pick any
//     tree number independent of which tree their note actually lives in).
//
// Both are now bound into the nullifier via NullifierBoundTree. Each
// negative test below tampers with exactly ONE field of an otherwise-valid
// witness and asserts the prover fails — if either binding regresses (e.g.
// someone "simplifies" the nullifier formula back to the plain Nullifier()),
// these tests catch it immediately instead of requiring a full Picus run or
// another manual audit pass.
package test

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/test"
	"github.com/iden3/go-iden3-crypto/poseidon"

	"enygma_retail_payments/gnark_circuits/templates"
)

// hashLeftRight matches primitives.MerkleProof's Poseidon-pair hashing.
func paymentTestHashLeftRight(l, r *big.Int) *big.Int {
	h, err := poseidon.Hash([]*big.Int{l, r})
	if err != nil {
		panic(err)
	}
	return h
}

// paymentTestWitness holds everything needed to build a valid PaymentCircuit
// witness (depth 2, 1 input, 2 outputs) plus the intermediate values, so
// individual tests can tamper with exactly one field.
type paymentTestWitness struct {
	sk, pk                            *big.Int
	tokenId, valueIn, saltIn          *big.Int
	treeNumber, pathIndices           *big.Int
	siblings                          []*big.Int
	root, contractAddress             *big.Int
	nullifier, inputCommitment        *big.Int
	pkOut0, saltOut0, valueOut0, cmt0 *big.Int
	pkOut1, saltOut1, valueOut1, cmt1 *big.Int
}

const paymentTestDepth = 2

func buildValidPaymentWitness(t *testing.T) *paymentTestWitness {
	t.Helper()
	w := &paymentTestWitness{}

	w.sk = big.NewInt(12345)
	pk, err := poseidon.Hash([]*big.Int{w.sk})
	if err != nil {
		t.Fatal(err)
	}
	w.pk = pk

	w.tokenId = big.NewInt(7)
	w.valueIn = big.NewInt(100)
	w.saltIn = big.NewInt(111)
	w.inputCommitment, err = poseidon.Hash([]*big.Int{w.pk, w.saltIn, w.valueIn, w.tokenId})
	if err != nil {
		t.Fatal(err)
	}

	w.treeNumber = big.NewInt(0)
	w.pathIndices = big.NewInt(0) // both path bits 0 → leaf always "left"
	w.siblings = []*big.Int{big.NewInt(222), big.NewInt(333)}
	root := w.inputCommitment
	for _, sib := range w.siblings {
		root = paymentTestHashLeftRight(root, sib)
	}
	w.root = root

	w.contractAddress = big.NewInt(0xABCD)

	// globalIdx = treeNumber*2^depth + pathIndices = 0*4 + 0 = 0
	globalIdx := big.NewInt(0)
	w.nullifier, err = poseidon.Hash([]*big.Int{w.sk, globalIdx, w.contractAddress})
	if err != nil {
		t.Fatal(err)
	}

	// Output 0 (Bob) — arbitrary recipient, no ownership constraint.
	otherSk := big.NewInt(999)
	w.pkOut0, err = poseidon.Hash([]*big.Int{otherSk})
	if err != nil {
		t.Fatal(err)
	}
	w.saltOut0 = big.NewInt(444)
	w.valueOut0 = big.NewInt(60)
	w.cmt0, err = poseidon.Hash([]*big.Int{w.pkOut0, w.saltOut0, w.valueOut0, w.tokenId})
	if err != nil {
		t.Fatal(err)
	}

	// Output 1 (change) — MUST be owned by the sender (WtPrivateKeysIn[0]'s pk).
	w.pkOut1 = w.pk
	w.saltOut1 = big.NewInt(555)
	w.valueOut1 = big.NewInt(40) // 60 + 40 == 100 == valueIn (conservation)
	w.cmt1, err = poseidon.Hash([]*big.Int{w.pkOut1, w.saltOut1, w.valueOut1, w.tokenId})
	if err != nil {
		t.Fatal(err)
	}

	return w
}

func (w *paymentTestWitness) circuit() *templates.PaymentCircuit {
	return &templates.PaymentCircuit{
		StMessage:            big.NewInt(0),
		StTreeNumbers:        []frontend.Variable{w.treeNumber},
		StMerkleRoots:        []frontend.Variable{w.root},
		StNullifiers:         []frontend.Variable{w.nullifier},
		StCommitmentsOut:     []frontend.Variable{w.cmt0, w.cmt1},
		StContractAddress:    w.contractAddress,
		WtPrivateKeysIn:      []frontend.Variable{w.sk},
		WtValuesIn:           []frontend.Variable{w.valueIn},
		WtSaltsIn:            []frontend.Variable{w.saltIn},
		WtPathElements:       [][]frontend.Variable{{w.siblings[0], w.siblings[1]}},
		WtPathIndices:        []frontend.Variable{w.pathIndices},
		WtTokenId:            w.tokenId,
		WtSpendPublicKeysOut: []frontend.Variable{w.pkOut0, w.pkOut1},
		WtValuesOut:          []frontend.Variable{w.valueOut0, w.valueOut1},
		WtSaltsOut:           []frontend.Variable{w.saltOut0, w.saltOut1},
	}
}

func paymentTestConfig() templates.PaymentCircuitConfig {
	return templates.PaymentCircuitConfig{
		TmNInputs:         1,
		TmMOutputs:        2,
		TmMerkleTreeDepth: paymentTestDepth,
		TmRange:           frontend.Variable("1000000000000000000000000000000000000"),
	}
}

// emptyPaymentCircuit returns a circuit skeleton with correctly-sized slices
// for frontend.Compile (values don't matter, only shape).
func emptyPaymentCircuit() *templates.PaymentCircuit {
	cfg := paymentTestConfig()
	c := &templates.PaymentCircuit{
		Config:               cfg,
		StTreeNumbers:        make([]frontend.Variable, cfg.TmNInputs),
		StMerkleRoots:        make([]frontend.Variable, cfg.TmNInputs),
		StNullifiers:         make([]frontend.Variable, cfg.TmNInputs),
		StCommitmentsOut:     make([]frontend.Variable, cfg.TmMOutputs),
		WtPrivateKeysIn:      make([]frontend.Variable, cfg.TmNInputs),
		WtValuesIn:           make([]frontend.Variable, cfg.TmNInputs),
		WtSaltsIn:            make([]frontend.Variable, cfg.TmNInputs),
		WtPathElements:       make([][]frontend.Variable, cfg.TmNInputs),
		WtPathIndices:        make([]frontend.Variable, cfg.TmNInputs),
		WtSpendPublicKeysOut: make([]frontend.Variable, cfg.TmMOutputs),
		WtValuesOut:          make([]frontend.Variable, cfg.TmMOutputs),
		WtSaltsOut:           make([]frontend.Variable, cfg.TmMOutputs),
	}
	for i := range c.WtPathElements {
		c.WtPathElements[i] = make([]frontend.Variable, cfg.TmMerkleTreeDepth)
	}
	return c
}

func TestPaymentCircuit_ValidWitness_Succeeds(t *testing.T) {
	assert := test.NewAssert(t)
	w := buildValidPaymentWitness(t)
	wc := w.circuit()
	wc.Config = paymentTestConfig()
	assert.ProverSucceeded(emptyPaymentCircuit(), wc, test.WithCurves(ecc.BN254))
}

// TestPaymentCircuit_TamperedContractAddress_Fails locks in the
// StContractAddress fix: changing StContractAddress alone, without
// recomputing the nullifier to match, must fail — before the fix,
// StContractAddress was never referenced anywhere in Define() and this
// witness would have satisfied the circuit regardless.
func TestPaymentCircuit_TamperedContractAddress_Fails(t *testing.T) {
	assert := test.NewAssert(t)
	w := buildValidPaymentWitness(t)
	wc := w.circuit()
	wc.Config = paymentTestConfig()
	wc.StContractAddress = big.NewInt(0xDEAD) // nullifier still bound to 0xABCD
	assert.ProverFailed(emptyPaymentCircuit(), wc, test.WithCurves(ecc.BN254))
}

// TestPaymentCircuit_TamperedTreeNumber_Fails locks in the StTreeNumbers fix:
// changing StTreeNumbers[0] alone, without recomputing the nullifier to
// match, must fail — before the fix, StTreeNumbers was never referenced
// anywhere in Define() and this witness would have satisfied the circuit
// regardless.
func TestPaymentCircuit_TamperedTreeNumber_Fails(t *testing.T) {
	assert := test.NewAssert(t)
	w := buildValidPaymentWitness(t)
	wc := w.circuit()
	wc.Config = paymentTestConfig()
	wc.StTreeNumbers = []frontend.Variable{big.NewInt(1)} // nullifier still bound to treeNumber=0
	assert.ProverFailed(emptyPaymentCircuit(), wc, test.WithCurves(ecc.BN254))
}

// TestPaymentCircuit_TamperedChangeOwnership_Fails locks in a pre-existing
// (not this-session) security property: the change output (index >= 1)
// must be owned by the sender's own key. Recomputes StCommitmentsOut[1] to
// match the new (wrong) owner, so the commitment check itself would pass —
// isolating the failure specifically to the ownership constraint.
func TestPaymentCircuit_TamperedChangeOwnership_Fails(t *testing.T) {
	assert := test.NewAssert(t)
	w := buildValidPaymentWitness(t)

	attackerSk := big.NewInt(31337)
	attackerPk, err := poseidon.Hash([]*big.Int{attackerSk})
	if err != nil {
		t.Fatal(err)
	}
	cmt1, err := poseidon.Hash([]*big.Int{attackerPk, w.saltOut1, w.valueOut1, w.tokenId})
	if err != nil {
		t.Fatal(err)
	}

	wc := w.circuit()
	wc.Config = paymentTestConfig()
	wc.WtSpendPublicKeysOut = []frontend.Variable{w.pkOut0, attackerPk}
	wc.StCommitmentsOut = []frontend.Variable{w.cmt0, cmt1}
	assert.ProverFailed(emptyPaymentCircuit(), wc, test.WithCurves(ecc.BN254))
}

// TestPaymentCircuit_TamperedConservation_Fails: total outputs must equal
// total inputs. Recomputes StCommitmentsOut[1] to match the new (wrong)
// value, so the commitment check itself would pass — isolating the failure
// specifically to the conservation constraint.
func TestPaymentCircuit_TamperedConservation_Fails(t *testing.T) {
	assert := test.NewAssert(t)
	w := buildValidPaymentWitness(t)

	badValueOut1 := big.NewInt(30) // 60 + 30 = 90 != 100 (valueIn)
	cmt1, err := poseidon.Hash([]*big.Int{w.pkOut1, w.saltOut1, badValueOut1, w.tokenId})
	if err != nil {
		t.Fatal(err)
	}

	wc := w.circuit()
	wc.Config = paymentTestConfig()
	wc.WtValuesOut = []frontend.Variable{w.valueOut0, badValueOut1}
	wc.StCommitmentsOut = []frontend.Variable{w.cmt0, cmt1}
	assert.ProverFailed(emptyPaymentCircuit(), wc, test.WithCurves(ecc.BN254))
}
