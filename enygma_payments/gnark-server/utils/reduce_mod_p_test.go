package utils

// TestReduceModP_RejectsTamperedHint is the regression test the audit's C-01
// remediation asked for directly: "Add a regression test asserting that
// ccs.Solve with solver.OverrideHint(solver.GetHintID(utils.ModHint),
// tamperedHint) fails — that test fails today."
//
// C-01: every ModHint call site implemented modular reduction as
// q·P + r == value (mod Fr) plus r < P, with q completely unbounded. Because
// P is invertible in the BN254 scalar field Fr, a satisfying q exists for
// ANY r a prover's hint chooses to return — the "reduction" proved nothing.
// ReduceModP fixes this by additionally constraining q to 3 bits (Fr is
// ~254 bits, P is ~251 bits, so floor(Fr/P) == 7 — any honest reduction's
// quotient is in [0,7], and a forged r that doesn't match the true
// value mod P needs a q reached only via modular inverse, which lands far
// outside that range).
//
// This tests ReduceModP directly rather than a full enygma/enygma_fee/
// deposit/withdraw circuit: those need real Poseidon/Pedersen-derived
// witness data that only go_client (a separate Go module) knows how to
// build. Testing the gadget in isolation is what the audit's own Control
// A/B methodology did first too, before the full end-to-end mint demo —
// and it's the actual unit under fix: ReduceModP itself, not any one of
// its 28 call sites.

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/test"
)

// reduceModPCircuit exposes ReduceModP directly: Value in, Expect out.
type reduceModPCircuit struct {
	Value  frontend.Variable
	Expect frontend.Variable
}

func (c *reduceModPCircuit) Define(api frontend.API) error {
	r := ReduceModP(api, c.Value)
	api.AssertIsEqual(r, c.Expect)
	return nil
}

// evilModHint is a forged hint for the C-01 attack: for the one specific
// input value it's told to target, it returns an attacker-chosen r (e.g. a
// "minted" amount) together with whatever q the modular-inverse trick
// requires to satisfy q·P + r == value (mod Fr) — the field equation the
// unbounded original gadget only ever checked. Every other input is
// reduced honestly, mirroring the audit's own repro ("exactly one of the
// hint invocations is overridden; the rest reduce honestly").
func evilModHint(targetValue, forgedR *big.Int) func(mod *big.Int, inputs []*big.Int, res []*big.Int) error {
	return func(mod *big.Int, inputs []*big.Int, res []*big.Int) error {
		value := inputs[0]
		if value.Cmp(targetValue) == 0 {
			// Solve q = (value - r) * P^-1 (mod Fr) — a field-equation
			// solution, not an integer quotient. This is exactly the
			// freedom C-01 left unconstrained.
			diff := new(big.Int).Sub(value, forgedR)
			diff.Mod(diff, mod)
			pInv := new(big.Int).ModInverse(P, mod)
			if pInv == nil {
				panic("P not invertible mod Fr — test assumption broken")
			}
			q := new(big.Int).Mul(diff, pInv)
			q.Mod(q, mod)
			res[0] = new(big.Int).Mod(forgedR, mod)
			res[1] = q
			return nil
		}
		return ModHint(mod, inputs, res)
	}
}

func TestReduceModP_HonestReductionSucceeds(t *testing.T) {
	solver.RegisterHint(ModHint)
	assert := test.NewAssert(t)
	circuit := &reduceModPCircuit{}
	witness := &reduceModPCircuit{Value: P, Expect: 0} // P mod P == 0
	assert.SolvingSucceeded(circuit, witness, test.WithCurves(ecc.BN254))
}

// TestReduceModP_RejectsTamperedHint is the C-01 regression test. It fails
// on the pre-fix gadget (bare hint + q·P+r==value + r<P, no bound on q) and
// passes on ReduceModP.
func TestReduceModP_RejectsTamperedHint(t *testing.T) {
	forgedR := new(big.Int)
	forgedR.SetString("1000000000000000000000000", 10) // an arbitrary "minted" amount
	hint := evilModHint(P, forgedR)

	circuit := &reduceModPCircuit{}
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	// The attacker's own witness: they claim ReduceModP(P) == forgedR.
	witness := &reduceModPCircuit{Value: P, Expect: frontend.Variable(forgedR)}
	w, err := frontend.NewWitness(witness, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("build witness: %v", err)
	}

	_, err = ccs.Solve(w, solver.OverrideHint(solver.GetHintID(ModHint), hint))
	if err == nil {
		t.Fatal("FAIL (C-01 regressed): ccs.Solve succeeded with a tampered hint claiming ReduceModP(P) == " +
			forgedR.String() + " — the quotient bound is not being enforced")
	}
	t.Logf("tampered-hint solve correctly rejected: %v", err)
}

// TestReduceModP_MatchedHonestControl proves the rejection above is
// specific to the forged value, not incidental breakage from registering
// any hint override at all — mirrors the audit's own Control B.
func TestReduceModP_MatchedHonestControl(t *testing.T) {
	// A hint override that behaves identically to ModHint for every input,
	// registered through the exact same solver.OverrideHint mechanism.
	honestOverride := func(mod *big.Int, inputs []*big.Int, res []*big.Int) error {
		return ModHint(mod, inputs, res)
	}

	circuit := &reduceModPCircuit{}
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	witness := &reduceModPCircuit{Value: P, Expect: 0}
	w, err := frontend.NewWitness(witness, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("build witness: %v", err)
	}

	if _, err := ccs.Solve(w, solver.OverrideHint(solver.GetHintID(ModHint), honestOverride)); err != nil {
		t.Fatalf("FAIL: an honest hint override was rejected — the failure above isn't specific to the forged value: %v", err)
	}
	t.Log("honest hint override (identical behavior, same OverrideHint mechanism) solves fine")
}

// TestReduceModP_EndToEndProof proves and verifies a real Groth16 proof
// through the fixed gadget, confirming the fix doesn't just affect Solve
// but the full prove/verify path a client and the on-chain verifier
// actually exercise.
func TestReduceModP_EndToEndProof(t *testing.T) {
	solver.RegisterHint(ModHint)
	circuit := &reduceModPCircuit{}
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	witness := &reduceModPCircuit{Value: P, Expect: 0}
	w, err := frontend.NewWitness(witness, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("witness: %v", err)
	}
	proof, err := groth16.Prove(ccs, pk, w)
	if err != nil {
		t.Fatalf("prove: %v", err)
	}
	pub, err := w.Public()
	if err != nil {
		t.Fatalf("public witness: %v", err)
	}
	if err := groth16.Verify(proof, vk, pub); err != nil {
		t.Fatalf("verify: %v", err)
	}
	t.Log("honest end-to-end Groth16 prove+verify through the fixed ReduceModP succeeds")
}
