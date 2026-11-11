// Regression tests for the prover-chosen value bound. Every Payment-family
// circuit used to range-check note values against Config.TmRange, a
// frontend.Variable in a config struct without `gnark:"-"`: gnark compiled it as
// a secret witness, so a prover set it to p-1 and spent a 100-unit note into a
// 10^30 note plus a change note of 100 - 10^30 mod p, or withdrew 10^30. The
// bound is now the fixed valuebound constant; these witnesses are the
// exploit's, and must no longer solve or prove.
package test

import (
	"math/big"
	"testing"

	"enygma_gnark_shared/circuitconfig"
	"enygma_gnark_shared/valuebound"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/test"
	"github.com/iden3/go-iden3-crypto/poseidon"

	"enygma_retail_payments/gnark_circuits/templates"
)

// boundHash is Poseidon over xs, failing t on error.
func boundHash(t *testing.T, xs ...*big.Int) *big.Int {
	t.Helper()
	r, err := poseidon.Hash(xs)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// spendOneNote returns a PaymentConfig witness spending a valueIn note of
// key sk into out0 for pkOut0 (salt salt0) and out1 back to sk's key (salt
// salt1), with message as StMessage.
func spendOneNote(t *testing.T, message int, sk, valueIn, pkOut0, out0, salt0, out1, salt1 *big.Int) *templates.PaymentCircuit {
	t.Helper()
	cfg := templates.PaymentConfig
	pkS := boundHash(t, sk)
	token := big.NewInt(7)
	saltIn := big.NewInt(111)
	root := boundHash(t, pkS, saltIn, valueIn, token)
	sibs := make([]frontend.Variable, cfg.TmMerkleTreeDepth)
	for i := range sibs {
		s := big.NewInt(int64(1000 + i))
		sibs[i] = s
		root = boundHash(t, root, s)
	}
	nf := boundHash(t, sk, big.NewInt(0))
	if message == templates.MessageWithdraw {
		salt1 = boundHash(t, sk, nf)
	}

	a := templates.NewPaymentCircuit(cfg)
	a.StMessage = message
	a.StTreeNumbers[0] = 0
	a.StMerkleRoots[0] = root
	a.StNullifiers[0] = nf
	a.StCommitmentsOut[0] = boundHash(t, pkOut0, salt0, out0, token)
	a.StCommitmentsOut[1] = boundHash(t, pkS, salt1, out1, token)
	a.StContractAddress = big.NewInt(0xABCD)
	a.WtPrivateKeysIn[0] = sk
	a.WtValuesIn[0] = valueIn
	a.WtSaltsIn[0] = saltIn
	a.WtPathElements[0] = sibs
	a.WtPathIndices[0] = 0
	a.WtTokenId = token
	a.WtSpendPublicKeysOut[0], a.WtSpendPublicKeysOut[1] = pkOut0, pkS
	a.WtValuesOut[0], a.WtValuesOut[1] = out0, out1
	a.WtSaltsOut[0], a.WtSaltsOut[1] = salt0, salt1
	return &a
}

// wrapped returns valueIn - out mod p: the change note that balances the sum
// in the field when out exceeds valueIn.
func wrapped(valueIn, out *big.Int) *big.Int {
	c := new(big.Int).Sub(valueIn, out)
	return c.Mod(c, ecc.BN254.ScalarField())
}

// TestValueBound_InflationDoesNotProve: the exploit's payment, a 100-unit
// note spent into 10^30 plus a wrapped change note, neither solves nor proves
// against the deployed Payment shape.
func TestValueBound_InflationDoesNotProve(t *testing.T) {
	sk := big.NewInt(12345)
	valueIn := big.NewInt(100)
	big30 := new(big.Int).Exp(big.NewInt(10), big.NewInt(30), nil)
	a := spendOneNote(t, 0, sk, valueIn, boundHash(t, sk), big30, big.NewInt(444), wrapped(valueIn, big30), big.NewInt(555))

	empty := templates.NewPaymentCircuit(templates.PaymentConfig)
	if err := test.IsSolved(&empty, a, ecc.BN254.ScalarField()); err == nil {
		t.Fatal("a 100-unit note spent into 10^30 plus a wrapped change note solved")
	}

	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &empty)
	if err != nil {
		t.Fatal(err)
	}
	pk, _, err := groth16.Setup(ccs)
	if err != nil {
		t.Fatal(err)
	}
	wit, err := frontend.NewWitness(a, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := groth16.Prove(ccs, pk, wit); err == nil {
		t.Fatal("a 100-unit note spent into 10^30 plus a wrapped change note proved")
	}
}

// TestValueBound_WithdrawDrainDoesNotSolve: the exploit's withdrawal, 10^30
// to a public recipient from a 100-unit note (output 0 as
// Erc20CoinVault._settlePartialWithdraw recomputes it), does not solve.
func TestValueBound_WithdrawDrainDoesNotSolve(t *testing.T) {
	sk := big.NewInt(4242)
	valueIn := big.NewInt(100)
	amount := new(big.Int).Exp(big.NewInt(10), big.NewInt(30), nil)
	recipient := big.NewInt(0xab)
	a := spendOneNote(t, templates.MessageWithdraw, sk, valueIn, recipient, amount, big.NewInt(0), wrapped(valueIn, amount), nil)

	empty := templates.NewPaymentCircuit(templates.PaymentConfig)
	if err := test.IsSolved(&empty, a, ecc.BN254.ScalarField()); err == nil {
		t.Fatal("a withdrawal of 10^30 from a 100-unit note solved")
	}
}

// TestValueBound_TopValueSolves: a note of 10^36 - 1, the largest value the
// bound allows, still spends; one of 10^36 does not.
func TestValueBound_TopValueSolves(t *testing.T) {
	sk := big.NewInt(777)
	top := new(big.Int).Sub(valuebound.Bound(), big.NewInt(1))
	empty := templates.NewPaymentCircuit(templates.PaymentConfig)

	a := spendOneNote(t, 0, sk, top, big.NewInt(99), top, big.NewInt(1), big.NewInt(0), big.NewInt(2))
	if err := test.IsSolved(&empty, a, ecc.BN254.ScalarField()); err != nil {
		t.Fatalf("a 10^36 - 1 note did not spend: %v", err)
	}

	over := valuebound.Bound()
	a = spendOneNote(t, 0, sk, over, big.NewInt(99), over, big.NewInt(1), big.NewInt(0), big.NewInt(2))
	if err := test.IsSolved(&empty, a, ecc.BN254.ScalarField()); err == nil {
		t.Fatal("a 10^36 note spent")
	}
}

// TestCircuitConfigs_HoldNoVariables fails if any circuit this module
// compiles keeps a frontend.Variable (or any other non-constant) in its
// config, which gnark would compile as a prover-chosen witness.
func TestCircuitConfigs_HoldNoVariables(t *testing.T) {
	for name, c := range map[string]any{
		"Payment":                 templates.PaymentCircuit{},
		"PaymentFee":              templates.PaymentFeeCircuit{},
		"PaymentRelayerFeePublic": templates.PaymentRelayerFeePublicCircuit{},
		"UsdrFee":                 templates.UsdrFeeCircuit{},
		"PrivateMint":             templates.PrivateMintCircuit{},
	} {
		if err := circuitconfig.Check(c); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}
