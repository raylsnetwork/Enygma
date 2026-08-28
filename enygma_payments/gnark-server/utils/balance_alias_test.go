package utils

// TestBalanceAlias reproduces C-02 (ENYGMA_PAYMENTS_AUDIT_2026-08-22.md,
// Critical/LIVE) directly at the gadget level and confirms the fixed
// circuits reject it, mirroring the C-01 test's methodology.
//
// C-02: PedersenCommitment's scalar multiplications have order P (~251
// bits, order of the Baby Jubjub prime subgroup), so Com(b, r) ==
// Com(b+P, r) == Com(b+2P, r) exactly. The circuits' only bound on
// PreviousSenderBalance/SenderTxValue/Fee was api.ToBinary(v, 252) — and
// since 2^252/P ≈ 2.645, a witness claiming b+P or b+2P still fits in 252
// bits, opens the identical on-chain commitment, and is a value a solvency
// comparator (previousV >= spend) would read as real+P or real+2P: a
// near-zero real balance passes as almost-P worth of funds. The fix
// (this branch's circuit.go changes) narrows those ToBinary calls to 64
// bits, ample for any real amount and far below P, closing the alias.
//
// This tests the same mechanism the real circuits use — PedersenCommitment
// plus a width-bounded reconstruction of the same witness value — without
// needing the full enygma circuit's Poseidon/fingerprint witness data
// (which only go_client, a separate module, knows how to build). It
// mirrors C-01's approach: validate the vulnerable *gadget* in isolation.

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/test"
	"github.com/iden3/go-iden3-crypto/babyjub"
)

// balanceAliasCircuit mirrors the vulnerable pattern common to
// PreviousSenderBalance (enygma/enygma_fee/deposit) and Fee (enygma_fee):
// the same witness value is (1) opened as a Pedersen commitment against a
// public on-chain point, and (2) separately width-constrained via
// ToBinary — the two checks that, pre-fix, disagreed about whether the
// value had to be canonically < P.
type balanceAliasCircuit struct {
	ClaimedBalance frontend.Variable
	Randomness     frontend.Variable
	CommitX        frontend.Variable `gnark:",public"`
	CommitY        frontend.Variable `gnark:",public"`

	widthBits int // compile-time only; not a circuit wire
}

func (c *balanceAliasCircuit) Define(api frontend.API) error {
	commit := PedersenCommitment(api, c.ClaimedBalance, c.Randomness)
	api.AssertIsEqual(commit.X, c.CommitX)
	api.AssertIsEqual(commit.Y, c.CommitY)

	// The C-02 bound (fixed: 64 bits; vulnerable: 252 bits, set by the
	// test that wants to reproduce the original gadget).
	bits := api.ToBinary(c.ClaimedBalance, c.widthBits)
	_ = bits
	return nil
}

// nativeCommit computes Com(v, r) using the SAME generators the circuit's
// PedersenCommitment uses (CircuitGBabyJub / HBabyJub) — not the since-deleted
// GetPK/PedersenCommitmentBabyJub, which used the *different*, dead-code
// GBabyJub (Fix L-15).
func nativeCommit(v, r *big.Int) (*big.Int, *big.Int) {
	vG := babyjub.NewPoint().Mul(v, CircuitGBabyJub)
	rH := babyjub.NewPoint().Mul(r, HBabyJub)
	sum := AddPks(vG, rH)
	return sum.X, sum.Y
}

func TestBalanceAlias_FixedWidthRejectsInflatedClaim(t *testing.T) {
	realBalance := big.NewInt(0)
	randomness := big.NewInt(12345)
	commitX, commitY := nativeCommit(realBalance, randomness)

	twoP := new(big.Int).Mul(big.NewInt(2), P)
	claimed := new(big.Int).Add(realBalance, twoP) // real + 2P

	// Sanity: Com(real, r) == Com(real+2P, r) — the periodicity C-02
	// exploits is real, independent of the fix.
	aliasX, aliasY := nativeCommit(claimed, randomness)
	if aliasX.Cmp(commitX) != 0 || aliasY.Cmp(commitY) != 0 {
		t.Fatalf("test setup bug: Com(real,r) != Com(real+2P,r); the periodicity this test relies on doesn't hold")
	}

	circuit := &balanceAliasCircuit{widthBits: 64}
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	witness := &balanceAliasCircuit{
		ClaimedBalance: claimed,
		Randomness:     randomness,
		CommitX:        commitX,
		CommitY:        commitY,
	}
	w, err := frontend.NewWitness(witness, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("build witness: %v", err)
	}
	if _, err := ccs.Solve(w); err == nil {
		t.Fatal("FAIL (C-02 regressed): claiming ClaimedBalance = real+2P solved successfully at the fixed 64-bit width")
	} else {
		t.Logf("64-bit width correctly rejected real+2P: %v", err)
	}
}

// TestBalanceAlias_VulnerableWidthAcceptsInflatedClaim is the negative
// control: the SAME attack against the ORIGINAL 252-bit width succeeds,
// proving TestBalanceAlias_FixedWidthRejectsInflatedClaim above genuinely
// discriminates the fix rather than failing for an unrelated reason.
func TestBalanceAlias_VulnerableWidthAcceptsInflatedClaim(t *testing.T) {
	realBalance := big.NewInt(0)
	randomness := big.NewInt(12345)
	commitX, commitY := nativeCommit(realBalance, randomness)

	twoP := new(big.Int).Mul(big.NewInt(2), P)
	claimed := new(big.Int).Add(realBalance, twoP)

	circuit := &balanceAliasCircuit{widthBits: 252}
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	witness := &balanceAliasCircuit{
		ClaimedBalance: claimed,
		Randomness:     randomness,
		CommitX:        commitX,
		CommitY:        commitY,
	}
	w, err := frontend.NewWitness(witness, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("build witness: %v", err)
	}
	if _, err := ccs.Solve(w); err != nil {
		t.Fatalf("expected the original 252-bit width to accept real+2P (reproducing C-02), but it was rejected: %v", err)
	}
	t.Log("252-bit width (the original, vulnerable gadget) accepts real+2P — confirms the exploit, and that the fixed-width test above is discriminating, not incidentally broken")
}

func TestBalanceAlias_HonestClaimSucceedsAtFixedWidth(t *testing.T) {
	assert := test.NewAssert(t)
	realBalance := big.NewInt(1_000_000) // small, realistic amount
	randomness := big.NewInt(67890)
	commitX, commitY := nativeCommit(realBalance, randomness)

	circuit := &balanceAliasCircuit{widthBits: 64}
	witness := &balanceAliasCircuit{
		ClaimedBalance: realBalance,
		Randomness:     randomness,
		CommitX:        commitX,
		CommitY:        commitY,
	}
	assert.SolvingSucceeded(circuit, witness, test.WithCurves(ecc.BN254))
}
