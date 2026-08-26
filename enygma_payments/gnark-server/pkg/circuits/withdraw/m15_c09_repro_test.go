package withdraw

// TestM15/TestC09 reproduce and verify the M-15 and C-09 fixes to
// withdraw/circuit.go (ENYGMA_PAYMENTS_AUDIT_2026-08-22.md), mirroring
// the C-01/C-02/C-03/C-08 tests' approach of testing the specific
// vulnerable mechanism in isolation via a minimal standalone circuit,
// rather than the full WithdrawEnygmaCircuit (which needs go_client's
// full Poseidon/Pedersen witness machinery to build a valid witness for
// — see h01_h02_repro_test.go's identical rationale in the enygma
// package).
//
//   M-15 (direction inversion): withdraw's sender-slot encoding used to
//     assert selected_v == +SenderTxValue (a CREDIT — the shielded
//     balance would INCREASE on a withdrawal, backwards, since
//     withdraw() removes value from the pool). Fixed to assert
//     selected_v == (P - SenderTxValue) mod P, the same debit idiom the
//     base transfer circuit uses for a sender.
//   C-09 (no external-leg binding): nothing related Σ VPerDeposit to
//     SenderTxValue, so a withdraw proof's shielded debit and its DvP
//     deposit-note value were independent, caller-chosen quantities.
//     Fixed by asserting Σ VPerDeposit == SenderTxValue and exposing the
//     sum as a new TotalDepositValue public signal Enygma.sol's
//     withdraw() binds against Σ depositParams[i].amount.
//
// Run:
//
//	go test -run "TestM15|TestC09" -v

import (
	"math/big"
	"testing"

	utils "enygma-server/utils"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

// directionCircuit mirrors withdraw/circuit.go's sender-slot debit
// encoding exactly (P - v via ToBinary/FromBinary + ReduceModP, matching
// the real circuit's Fix C-01/C-02-safe idiom). SelectedV is a free
// witness — standing in for whatever TxValues[senderId] the prover
// picked — so this isolates only the encoding check itself.
type directionCircuit struct {
	SelectedV     frontend.Variable
	SenderTxValue frontend.Variable
}

func (c *directionCircuit) Define(api frontend.API) error {
	jubJubP := frontend.Variable(JubJubPrimeSubGroupStr)

	selectedVBits := api.ToBinary(c.SelectedV, 252)
	vBits := api.ToBinary(c.SenderTxValue, 64)
	pDiffBits := api.ToBinary(jubJubP, 252)

	selectedVConstrained := api.FromBinary(selectedVBits...)
	vConstrained := api.FromBinary(vBits...)
	pDiffConstrained := api.FromBinary(pDiffBits...)

	expectedTxValue := api.Sub(pDiffConstrained, vConstrained)
	expectedTxValueMod := utils.ReduceModP(api, expectedTxValue)

	api.AssertIsEqual(selectedVConstrained, expectedTxValueMod)
	return nil
}

func TestM15_WithdrawFixedCircuitRequiresDebitEncoding(t *testing.T) {
	solver.RegisterHint(utils.ModHint)
	circuit := &directionCircuit{}
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	amount := big.NewInt(400)
	debitEncoded := new(big.Int).Sub(utils.P, amount) // P - 400, the correct (fixed) encoding

	honest := &directionCircuit{SelectedV: debitEncoded, SenderTxValue: amount}
	w, err := frontend.NewWitness(honest, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("build witness: %v", err)
	}
	if _, err := ccs.Solve(w); err != nil {
		t.Fatalf("FAIL: the fixed circuit rejected a correctly debit-encoded (P-v) sender slot: %v", err)
	}
	t.Log("fixed circuit accepts the correct debit encoding (P - SenderTxValue) ✓")
}

// Fix M-15 regression guard: the OLD, wrong encoding (selected_v == +v,
// a credit) must now be REJECTED by the fixed constraint — this is
// exactly the direction inversion the audit found: a withdrawal that
// used to credit the sender instead of debiting them.
func TestM15_WithdrawFixedCircuitRejectsCreditEncoding(t *testing.T) {
	solver.RegisterHint(utils.ModHint)
	circuit := &directionCircuit{}
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	amount := big.NewInt(400)
	creditEncoded := amount // the OLD (wrong, pre-fix) encoding: selected_v == +v

	bad := &directionCircuit{SelectedV: creditEncoded, SenderTxValue: amount}
	w, err := frontend.NewWitness(bad, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("build witness: %v", err)
	}
	if _, err := ccs.Solve(w); err == nil {
		t.Fatal("FAIL (M-15 regressed): the fixed circuit accepted a credit-encoded (+v) sender slot — withdraw would still credit instead of debit")
	} else {
		t.Logf("fixed circuit correctly rejects the old credit encoding: %v", err)
	}
}

// depositValueBindingCircuit mirrors withdraw/circuit.go's C-09 fix: sum
// 10 VPerDeposit slots and assert the sum equals SenderTxValue, exposing
// the sum as TotalDepositValue.
type depositValueBindingCircuit struct {
	VPerDeposit       [10]frontend.Variable
	SenderTxValue     frontend.Variable
	TotalDepositValue frontend.Variable `gnark:",public"`
}

func (c *depositValueBindingCircuit) Define(api frontend.API) error {
	sum := frontend.Variable(0)
	for i := 0; i < 10; i++ {
		sum = api.Add(sum, c.VPerDeposit[i])
	}
	api.AssertIsEqual(sum, c.SenderTxValue)
	api.AssertIsEqual(c.TotalDepositValue, sum)
	return nil
}

func TestC09_HonestDepositValueSumAccepted(t *testing.T) {
	circuit := &depositValueBindingCircuit{}
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	witness := &depositValueBindingCircuit{SenderTxValue: big.NewInt(500), TotalDepositValue: big.NewInt(500)}
	witness.VPerDeposit[0] = big.NewInt(300)
	witness.VPerDeposit[1] = big.NewInt(200)
	for i := 2; i < 10; i++ {
		witness.VPerDeposit[i] = big.NewInt(0)
	}
	w, err := frontend.NewWitness(witness, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("build witness: %v", err)
	}
	if _, err := ccs.Solve(w); err != nil {
		t.Fatalf("FAIL: an honest Σ VPerDeposit (300+200=500) matching SenderTxValue was rejected: %v", err)
	}
	t.Log("fixed circuit accepts Σ VPerDeposit == SenderTxValue ✓")
}

// Fix C-09's actual security property: a withdraw proof debiting a small
// (or zero) SenderTxValue while claiming a much larger Σ VPerDeposit —
// exactly the audit's "unlimited unbacked value creation" construction —
// must be rejected by the circuit itself.
func TestC09_MismatchedDepositValueRejected(t *testing.T) {
	circuit := &depositValueBindingCircuit{}
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	// Debits SenderTxValue=1 (near-zero shielded debit) while claiming
	// VPerDeposit sums to 1,000,000 — the exact attack C-09 describes.
	witness := &depositValueBindingCircuit{SenderTxValue: big.NewInt(1), TotalDepositValue: big.NewInt(1_000_000)}
	witness.VPerDeposit[0] = big.NewInt(1_000_000)
	for i := 1; i < 10; i++ {
		witness.VPerDeposit[i] = big.NewInt(0)
	}
	w, err := frontend.NewWitness(witness, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("build witness: %v", err)
	}
	if _, err := ccs.Solve(w); err == nil {
		t.Fatal("FAIL (C-09 regressed): a proof debiting 1 while claiming Σ VPerDeposit=1,000,000 was accepted")
	} else {
		t.Logf("fixed circuit correctly rejects the mismatched deposit value: %v", err)
	}
}

// The public TotalDepositValue signal itself must also be pinned to the
// true sum — a prover cannot publish a different value than what the
// witness actually sums to (this is what lets Enygma.sol trust the
// signal for its own Σ depositParams[i].amount comparison).
func TestC09_TotalDepositValueSignalMustMatchSum(t *testing.T) {
	circuit := &depositValueBindingCircuit{}
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	// Sum genuinely is 500 (matches SenderTxValue), but the published
	// public signal claims 999 instead.
	witness := &depositValueBindingCircuit{SenderTxValue: big.NewInt(500), TotalDepositValue: big.NewInt(999)}
	witness.VPerDeposit[0] = big.NewInt(500)
	for i := 1; i < 10; i++ {
		witness.VPerDeposit[i] = big.NewInt(0)
	}
	w, err := frontend.NewWitness(witness, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("build witness: %v", err)
	}
	if _, err := ccs.Solve(w); err == nil {
		t.Fatal("FAIL (C-09 regressed): a public TotalDepositValue signal that does not match Σ VPerDeposit was accepted")
	} else {
		t.Logf("fixed circuit correctly rejects a forged public signal: %v", err)
	}
}
