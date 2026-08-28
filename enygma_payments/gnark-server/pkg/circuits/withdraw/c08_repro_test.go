package withdraw

// TestC08 reproduces C-08 (ENYGMA_PAYMENTS_AUDIT_2026-08-22.md, High/BROKEN
// today, Critical the day the ZkDvp bridge is wired up) directly at the
// solvency-comparator level, mirroring the C-01/C-02/C-03 tests' approach.
//
// C-08: withdraw/circuit.go was the only one of the four circuits with no
// previousBalance >= amount check at all — only the vacuous
// api.Cmp(v, 0) >= 0, which is a pre-existing no-op in all four circuits
// (Cmp against the constant 0 can never take the "less than" branch for
// any field element). The audit demonstrated an account with balance 0
// crediting itself an arbitrary amount and instructing the DvP payout leg
// to pay it out, with no debit and no rejection.

import (
	"math/big"
	"testing"

	utils "enygma-server/utils"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

// solvencyCircuit mirrors withdraw/circuit.go's actual comparator mechanism
// (api.Cmp(previousVConstrained, vConstrained) != -1, both operands
// bit-decomposed to 64 bits first). applyFix toggles whether the
// previousV >= v check is present at all.
type solvencyCircuit struct {
	PreviousSenderBalance frontend.Variable
	SenderTxValue         frontend.Variable

	applyFix bool // compile-time only, not a circuit wire
}

func (c *solvencyCircuit) Define(api frontend.API) error {
	previousVBits := api.ToBinary(c.PreviousSenderBalance, 64)
	previousVConstrained := api.FromBinary(previousVBits...)
	vBits := api.ToBinary(c.SenderTxValue, 64)
	vConstrained := api.FromBinary(vBits...)

	if c.applyFix {
		prevVGreaterEqualV := api.Cmp(previousVConstrained, vConstrained)
		api.AssertIsEqual(api.IsZero(api.Add(prevVGreaterEqualV, frontend.Variable(1))), frontend.Variable(0))
	}

	// The pre-existing vacuous check present in all four circuits today —
	// included in both variants since it's not what C-08 is about, and
	// removing it isn't part of this fix.
	vGreaterEqualZero := api.Cmp(vConstrained, frontend.Variable(0))
	api.AssertIsEqual(api.IsZero(api.Add(vGreaterEqualZero, frontend.Variable(1))), frontend.Variable(0))
	return nil
}

func TestC08_VulnerableCircuitAcceptsOverWithdraw(t *testing.T) {
	circuit := &solvencyCircuit{applyFix: false}
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	// The audit's exact construction: balance 0, withdraw 1e18.
	witness := &solvencyCircuit{
		PreviousSenderBalance: big.NewInt(0),
		SenderTxValue:         new(big.Int).SetInt64(1_000_000_000_000_000_000),
	}
	w, err := frontend.NewWitness(witness, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("build witness: %v", err)
	}
	if _, err := ccs.Solve(w); err != nil {
		t.Fatalf("expected the original (vulnerable) circuit to accept a zero-balance over-withdraw, reproducing C-08, but it was rejected: %v", err)
	}
	t.Log("vulnerable circuit (no solvency comparator) accepts balance=0 withdrawing 1e18 — confirms C-08")
}

func TestC08_FixedCircuitRejectsOverWithdraw(t *testing.T) {
	circuit := &solvencyCircuit{applyFix: true}
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	witness := &solvencyCircuit{
		PreviousSenderBalance: big.NewInt(0),
		SenderTxValue:         new(big.Int).SetInt64(1_000_000_000_000_000_000),
	}
	w, err := frontend.NewWitness(witness, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("build witness: %v", err)
	}
	if _, err := ccs.Solve(w); err == nil {
		t.Fatal("FAIL (C-08 regressed): the fixed circuit accepted withdrawing 1e18 from a zero balance")
	} else {
		t.Logf("fixed circuit correctly rejected the over-withdraw: %v", err)
	}
}

func TestC08_FixedCircuitAcceptsHonestWithdraw(t *testing.T) {
	circuit := &solvencyCircuit{applyFix: true}
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	witness := &solvencyCircuit{
		PreviousSenderBalance: big.NewInt(1000),
		SenderTxValue:         big.NewInt(400),
	}
	w, err := frontend.NewWitness(witness, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("build witness: %v", err)
	}
	if _, err := ccs.Solve(w); err != nil {
		t.Fatalf("FAIL: an honest withdraw (balance 1000, withdraw 400) was rejected by the fixed circuit: %v", err)
	}
	t.Log("fixed circuit accepts an honest, solvent withdraw")
}

func TestC08_FixedCircuitRejectsBalanceAlias(t *testing.T) {
	// Sanity: confirms the 64-bit PreviousSenderBalance bound added
	// alongside the comparator (needed the moment the comparator exists,
	// same reasoning as C-02) — real+2P must still be rejected outright,
	// not merely fail the solvency comparison.
	circuit := &solvencyCircuit{applyFix: true}
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	twoP := new(big.Int).Mul(big.NewInt(2), utils.P)
	claimed := new(big.Int).Add(big.NewInt(0), twoP)
	witness := &solvencyCircuit{
		PreviousSenderBalance: claimed,
		SenderTxValue:         big.NewInt(400),
	}
	w, err := frontend.NewWitness(witness, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("build witness: %v", err)
	}
	if _, err := ccs.Solve(w); err == nil {
		t.Fatal("FAIL: claiming PreviousSenderBalance = real+2P solved successfully")
	} else {
		t.Logf("real+2P correctly rejected: %v", err)
	}
}
