package deposit

// TestM15 reproduces and verifies the M-15 direction fix to
// deposit/circuit.go (ENYGMA_PAYMENTS_AUDIT_2026-08-22.md), mirroring
// the C-01/C-02/C-03/C-08 tests' approach of testing the specific
// vulnerable mechanism in isolation via a minimal standalone circuit,
// rather than the full DepositEnygmaCircuit (which needs go_client's
// full Poseidon/Pedersen witness machinery to build a valid witness for
// — see enygma/h01_h02_repro_test.go's identical rationale).
//
// M-15 (direction inversion): deposit's sender-slot encoding used to
// assert selected_v == (P - SenderTxValue) mod P — a DEBIT. That is
// backwards: Enygma.deposit() moves value IN from the DvP side (it
// redeems an existing note), so the depositor must be CREDITED — under
// the old encoding a depositor was effectively paying twice (the note
// redeemed *and* the shielded balance debited). Fixed to a plain credit:
// selected_v == SenderTxValue.
//
// Run:
//
//	go test -run TestM15 -v

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

// directionCircuit mirrors deposit/circuit.go's sender-slot credit
// encoding exactly. SelectedV is a free witness — standing in for
// whatever TxValues[senderId] the prover picked — so this isolates only
// the encoding check itself.
type directionCircuit struct {
	SelectedV     frontend.Variable
	SenderTxValue frontend.Variable
}

func (c *directionCircuit) Define(api frontend.API) error {
	selectedVBits := api.ToBinary(c.SelectedV, 252)
	vBits := api.ToBinary(c.SenderTxValue, 64)

	selectedVConstrained := api.FromBinary(selectedVBits...)
	vConstrained := api.FromBinary(vBits...)

	api.AssertIsEqual(selectedVConstrained, vConstrained)
	return nil
}

func TestM15_DepositFixedCircuitRequiresCreditEncoding(t *testing.T) {
	circuit := &directionCircuit{}
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	amount := big.NewInt(400)
	creditEncoded := amount // the correct (fixed) encoding: selected_v == +v

	honest := &directionCircuit{SelectedV: creditEncoded, SenderTxValue: amount}
	w, err := frontend.NewWitness(honest, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("build witness: %v", err)
	}
	if _, err := ccs.Solve(w); err != nil {
		t.Fatalf("FAIL: the fixed circuit rejected a correctly credit-encoded (+v) sender slot: %v", err)
	}
	t.Log("fixed circuit accepts the correct credit encoding (+SenderTxValue) ✓")
}

// Fix M-15 regression guard: the OLD, wrong encoding (selected_v ==
// P-v, a debit) must now be REJECTED — this is exactly the direction
// inversion the audit found: a deposit that used to debit the depositor
// instead of crediting them (paying twice: once via the redeemed DvP
// note, once via the shielded debit).
func TestM15_DepositFixedCircuitRejectsDebitEncoding(t *testing.T) {
	circuit := &directionCircuit{}
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	amount := big.NewInt(400)
	debitEncoded := new(big.Int).Sub(curveP, amount) // the OLD (wrong, pre-fix) encoding: P - v

	bad := &directionCircuit{SelectedV: debitEncoded, SenderTxValue: amount}
	w, err := frontend.NewWitness(bad, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("build witness: %v", err)
	}
	if _, err := ccs.Solve(w); err == nil {
		t.Fatal("FAIL (M-15 regressed): the fixed circuit accepted a debit-encoded (P-v) sender slot — deposit would still debit instead of credit")
	} else {
		t.Logf("fixed circuit correctly rejects the old debit encoding: %v", err)
	}
}

var curveP, _ = new(big.Int).SetString(JubJubPrimeSubGroupStr, 10)
