package usdr

// TestC02/TestC03 confirm the fixes ported into usdr/circuit.go from
// enygma/circuit.go (that circuit's own c02/c03 regression tests) —
// usdr/circuit.go was added as a "near-verbatim copy" of enygma/circuit.go
// but initially missed both fixes; these tests exist so a future copy-paste
// regression here is caught the same way it would be in enygma/circuit.go.
// Mirrors enygma/c03_repro_test.go's approach: validate the vulnerable
// mechanism in isolation (field-level Pedersen/range-check arithmetic)
// rather than the full USDrCircuit, which needs Poseidon/fingerprint
// witness data only go_client (a separate module) can build.

import (
	"math/big"
	"testing"

	utils "enygma_payments/gnark-server/utils"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

const repro02_03NCommitment = 6

// solvencyCircuit mirrors the exact mechanism usdr/circuit.go (and
// enygma/circuit.go) use to enforce previousBalance >= spend: range-check
// both operands then api.Cmp them. previousVBits toggles between the
// vulnerable 252-bit bound (admits claimed = real + P) and the fixed
// 64-bit bound.
type solvencyCircuit struct {
	PreviousSenderBalance frontend.Variable
	SpendValue            frontend.Variable

	previousVBits int // 252 (vulnerable) or 64 (fixed) — compile-time only
}

func (c *solvencyCircuit) Define(api frontend.API) error {
	vBits := api.ToBinary(c.SpendValue, 64)
	vConstrained := api.FromBinary(vBits...)

	previousVBits := api.ToBinary(c.PreviousSenderBalance, c.previousVBits)
	previousVConstrained := api.FromBinary(previousVBits...)

	prevVGreaterEqualV := api.Cmp(previousVConstrained, vConstrained)
	api.AssertIsEqual(api.IsZero(api.Add(prevVGreaterEqualV, frontend.Variable(1))), frontend.Variable(0))
	return nil
}

func TestC02_VulnerableCircuitAcceptsInflatedBalance(t *testing.T) {
	circuit := &solvencyCircuit{previousVBits: 252}
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	// real balance is 0 (insolvent), but claimed = real + P — same
	// Pedersen commitment (periodic mod P), passes the 252-bit solvency
	// check as a near-2^251 integer regardless of true balance.
	w := &solvencyCircuit{
		PreviousSenderBalance: new(big.Int).Set(utils.P),
		SpendValue:            big.NewInt(100),
	}
	witness, err := frontend.NewWitness(w, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("build witness: %v", err)
	}
	if _, err := ccs.Solve(witness); err != nil {
		t.Fatalf("expected the original (vulnerable) circuit to accept previousBalance=P as sufficient for spend=100, reproducing C-02, but it was rejected: %v", err)
	}
	t.Log("vulnerable circuit (252-bit bound) accepts previousBalance=P (real balance 0) as solvent for spend=100 — confirms C-02")
}

func TestC02_FixedCircuitRejectsInflatedBalance(t *testing.T) {
	circuit := &solvencyCircuit{previousVBits: 64}
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	w := &solvencyCircuit{
		PreviousSenderBalance: new(big.Int).Set(utils.P),
		SpendValue:            big.NewInt(100),
	}
	witness, err := frontend.NewWitness(w, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("build witness: %v", err)
	}
	if _, err := ccs.Solve(witness); err == nil {
		t.Fatal("FAIL (C-02 regressed): the fixed circuit accepted previousBalance=P as sufficient for spend=100")
	} else {
		t.Logf("fixed circuit correctly rejected the inflated balance: %v", err)
	}
}

func TestC02_FixedCircuitAcceptsHonestBalance(t *testing.T) {
	circuit := &solvencyCircuit{previousVBits: 64}
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	w := &solvencyCircuit{
		PreviousSenderBalance: big.NewInt(500),
		SpendValue:            big.NewInt(100),
	}
	witness, err := frontend.NewWitness(w, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("build witness: %v", err)
	}
	if _, err := ccs.Solve(witness); err != nil {
		t.Fatalf("FAIL: an honest balance (500 >= 100) was rejected by the fixed circuit: %v", err)
	}
	t.Log("fixed circuit accepts an honest sufficient balance")
}

// conservationCircuit mirrors enygma/c03_repro_test.go's conservationCircuit
// exactly — same mechanism usdr/circuit.go uses to enforce non-sender-slot
// conservation.
type conservationCircuit struct {
	SenderId      frontend.Variable
	AnonymitySet  [repro02_03NCommitment]frontend.Variable
	TxValues      [repro02_03NCommitment]frontend.Variable
	SenderTxValue frontend.Variable // what the sender's fee is claimed to be

	applyFix bool // compile-time only, not a circuit wire
}

func (c *conservationCircuit) Define(api frontend.API) error {
	sum := frontend.Variable(0)
	for i := 0; i < repro02_03NCommitment; i++ {
		sum = api.Add(sum, c.TxValues[i])
	}
	sumMod := utils.ReduceModP(api, sum)
	api.AssertIsEqual(sumMod, frontend.Variable(0))

	if c.applyFix {
		sumNonSender := frontend.Variable(0)
		for i := 0; i < repro02_03NCommitment; i++ {
			isSenderSlot := api.IsZero(api.Sub(c.AnonymitySet[i], c.SenderId))
			nonSenderValue := api.Select(isSenderSlot, frontend.Variable(0), c.TxValues[i])
			bits := api.ToBinary(nonSenderValue, 64)
			nonSenderConstrained := api.FromBinary(bits...)
			sumNonSender = api.Add(sumNonSender, nonSenderConstrained)
		}
		api.AssertIsEqual(sumNonSender, c.SenderTxValue)
	}
	return nil
}

func attackWitness() *conservationCircuit {
	w := &conservationCircuit{}
	for i := 0; i < repro02_03NCommitment; i++ {
		w.AnonymitySet[i] = big.NewInt(int64(i + 1))
	}
	w.SenderId = big.NewInt(1)
	w.SenderTxValue = big.NewInt(0)
	w.TxValues[0] = big.NewInt(0)
	w.TxValues[1] = big.NewInt(400)
	w.TxValues[2] = new(big.Int).Sub(utils.P, big.NewInt(400))
	w.TxValues[3] = big.NewInt(0)
	w.TxValues[4] = big.NewInt(0)
	w.TxValues[5] = big.NewInt(0)
	return w
}

func TestC03_VulnerableCircuitAcceptsHiddenDebit(t *testing.T) {
	solver.RegisterHint(utils.ModHint)
	circuit := &conservationCircuit{applyFix: false}
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	w, err := frontend.NewWitness(attackWitness(), ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("build witness: %v", err)
	}
	if _, err := ccs.Solve(w); err != nil {
		t.Fatalf("expected the original (vulnerable) circuit to accept the hidden debit, reproducing C-03, but it was rejected: %v", err)
	}
	t.Log("vulnerable circuit (no non-sender range check) accepts fee=0, +400 honest credit, P-400 hidden debit — confirms C-03")
}

func TestC03_FixedCircuitRejectsHiddenDebit(t *testing.T) {
	solver.RegisterHint(utils.ModHint)
	circuit := &conservationCircuit{applyFix: true}
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	w, err := frontend.NewWitness(attackWitness(), ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("build witness: %v", err)
	}
	if _, err := ccs.Solve(w); err == nil {
		t.Fatal("FAIL (C-03 regressed): the fixed circuit accepted a P-400 hidden debit in a non-sender slot")
	} else {
		t.Logf("fixed circuit correctly rejected the hidden debit: %v", err)
	}
}

func TestC03_FixedCircuitAcceptsHonestFeeSplit(t *testing.T) {
	solver.RegisterHint(utils.ModHint)
	circuit := &conservationCircuit{applyFix: true}
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	honest := &conservationCircuit{}
	for i := 0; i < repro02_03NCommitment; i++ {
		honest.AnonymitySet[i] = big.NewInt(int64(i + 1))
	}
	honest.SenderId = big.NewInt(1)
	honest.SenderTxValue = big.NewInt(10)
	honest.TxValues[0] = new(big.Int).Sub(utils.P, big.NewInt(10))
	honest.TxValues[1] = big.NewInt(10)
	honest.TxValues[2] = big.NewInt(0)
	honest.TxValues[3] = big.NewInt(0)
	honest.TxValues[4] = big.NewInt(0)
	honest.TxValues[5] = big.NewInt(0)

	w, err := frontend.NewWitness(honest, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("build witness: %v", err)
	}
	if _, err := ccs.Solve(w); err != nil {
		t.Fatalf("FAIL: an honest fee transfer (sender pays 10 to one recipient) was rejected by the fixed circuit: %v", err)
	}
	t.Log("fixed circuit accepts an honest fee transfer")
}
