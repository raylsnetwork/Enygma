package enygma

// TestC03 reproduces C-03 (ENYGMA_PAYMENTS_AUDIT_2026-08-22.md,
// Critical/LIVE) directly at the conservation-check level and confirms the
// fixed circuit rejects it — mirroring the C-01/C-02 tests' approach of
// validating the vulnerable mechanism in isolation rather than the full
// enygma circuit (which needs Poseidon/fingerprint witness data only
// go_client, a separate module, can build).
//
// C-03: only the sender's own TxValues slot had any bound on its
// magnitude. Every other slot was constrained solely by the aggregate
// Σ TxValues[i] ≡ 0 (mod P) — and P - w (a debit of w) is a perfectly
// valid field element satisfying that congruence just as well as an
// honest 0. The audit's own evidence: sender spends 0, one account is
// credited +400 honestly, a second (uninvolved, non-consenting) account
// is "credited" P-400 — a silent debit of 400 with no accomplice and no
// on-chain trace, since Σ TxValues == 0 + 400 + (P-400) + 0+0+0 ≡ 0 (mod
// P) exactly like an honest all-zero transfer would.

import (
	"math/big"
	"testing"

	utils "enygma-server/utils"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/constraint/solver"
)

const c03NCommitment = 6

// conservationCircuit mirrors the exact mechanism enygma/circuit.go uses to
// enforce transfer conservation (Σ TxValues[i] ≡ 0 mod P via ReduceModP —
// the real circuit reaches the same congruence through a Pedersen point
// identity, but the vulnerability is about the missing per-slot range
// check and integer-sum assertion, not the curve arithmetic, so testing
// the field-level mechanism directly is faithful to what's actually being
// fixed). applyFix toggles the C-03 non-sender range check on or off, so
// the same circuit is both the vulnerable and the fixed version.
type conservationCircuit struct {
	SenderId      frontend.Variable
	AnonymitySet  [c03NCommitment]frontend.Variable
	TxValues      [c03NCommitment]frontend.Variable
	SenderTxValue frontend.Variable // what the sender claims to spend

	applyFix bool // compile-time only, not a circuit wire
}

func (c *conservationCircuit) Define(api frontend.API) error {
	sum := frontend.Variable(0)
	for i := 0; i < c03NCommitment; i++ {
		sum = api.Add(sum, c.TxValues[i])
	}
	sumMod := utils.ReduceModP(api, sum)
	api.AssertIsEqual(sumMod, frontend.Variable(0))

	if c.applyFix {
		sumNonSender := frontend.Variable(0)
		for i := 0; i < c03NCommitment; i++ {
			isSenderSlot := api.IsZero(api.Sub(c.AnonymitySet[i], c.SenderId))
			nonSenderValue := api.Select(isSenderSlot, frontend.Variable(0), c.TxValues[i])
			bits := api.ToBinary(nonSenderValue, 64)
			nonSenderConstrained := api.FromBinary(bits...)
			sumNonSender = api.Add(sumNonSender, nonSenderConstrained)
		}
		// Matches enygma/circuit.go's actual target: the non-sender
		// credits must sum to exactly what the sender claims to spend.
		api.AssertIsEqual(sumNonSender, c.SenderTxValue)
	}
	return nil
}

// attackWitness builds the audit's exact construction: sender (slot 0)
// spends nothing, slot 1 (an accomplice, or the attacker's own second
// account) is honestly credited +400, slot 2 (a victim who never
// consented) is "credited" P-400 — a hidden debit of 400.
func attackWitness() *conservationCircuit {
	w := &conservationCircuit{}
	for i := 0; i < c03NCommitment; i++ {
		w.AnonymitySet[i] = big.NewInt(int64(i + 1))
	}
	w.SenderId = big.NewInt(1)
	w.SenderTxValue = big.NewInt(0) // sender spends nothing — the audit's "free" construction
	w.TxValues[0] = big.NewInt(0)
	w.TxValues[1] = big.NewInt(400)
	w.TxValues[2] = new(big.Int).Sub(utils.P, big.NewInt(400)) // P - 400: hides a debit of 400
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
	t.Log("vulnerable circuit (no non-sender range check) accepts sender=0, +400 honest credit, P-400 hidden debit — confirms C-03")
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

func TestC03_FixedCircuitAcceptsHonestTransfer(t *testing.T) {
	solver.RegisterHint(utils.ModHint)
	circuit := &conservationCircuit{applyFix: true}
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	honest := &conservationCircuit{}
	for i := 0; i < c03NCommitment; i++ {
		honest.AnonymitySet[i] = big.NewInt(int64(i + 1))
	}
	honest.SenderId = big.NewInt(1)
	honest.SenderTxValue = big.NewInt(100)
	honest.TxValues[0] = new(big.Int).Sub(utils.P, big.NewInt(100)) // sender spends 100
	honest.TxValues[1] = big.NewInt(60)
	honest.TxValues[2] = big.NewInt(40)
	honest.TxValues[3] = big.NewInt(0)
	honest.TxValues[4] = big.NewInt(0)
	honest.TxValues[5] = big.NewInt(0)

	w, err := frontend.NewWitness(honest, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("build witness: %v", err)
	}
	if _, err := ccs.Solve(w); err != nil {
		t.Fatalf("FAIL: an honest transfer (sender spends 100, split 60/40 to two receivers) was rejected by the fixed circuit: %v", err)
	}
	t.Log("fixed circuit accepts an honest split transfer")
}
