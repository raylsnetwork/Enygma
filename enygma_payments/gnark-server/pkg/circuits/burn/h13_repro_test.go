package burn

// TestBurnCircuit_* reproduce H-13 (ENYGMA_PAYMENTS_AUDIT_2026-08-22.md,
// High/LIVE) and confirm the fix.
//
// H-13: burn() was plaintext arithmetic on a hidden Pedersen-committed
// balance — Com(balance,r) + Com(P-amount,0) — with no way for anyone,
// including the contract's own owner, to verify amount <= balance, because
// the balance is hidden by design. Over-burning silently wrapped the
// committed value to something just under P, and the audit demonstrated
// that value was then spendable through the transfer circuit's (at the
// time) 252-bit range check.
//
// Unlike C-01..C-08/H-01/H-02, this isn't a fix to an existing circuit's
// formula — it's a brand new circuit, so these tests exercise the real
// BurnCircuit directly (compile + Solve, no groth16.Prove — Solve alone
// proves witness satisfiability, which is what these tests need) rather
// than an isolated gadget mirroring it.
//
// Note on the "spendable" half of H-13's evidence: this repo's C-02/C-08
// fixes (earlier in this branch) already tightened the transfer/withdraw
// circuits' balance range checks from 252 to 64 bits, which independently
// closes the specific propagation path the audit demonstrated (a ~2^250
// wrapped balance can no longer pass a 64-bit ToBinary decomposition
// either). That doesn't make this fix redundant: burn() itself still had
// zero verification of its own arithmetic, which is silently exploitable
// admin behavior and breaks the totalSupply invariant independent of
// whether the resulting balance later happens to be spendable anywhere
// else — TestBurnCircuit_WrappedBalanceRejected demonstrates the wrapped
// value doesn't even satisfy the burn circuit's own range check.

import (
	"math/big"
	"testing"

	utils "enygma-server/utils"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/iden3/go-iden3-crypto/babyjub"
	iden3poseidon "github.com/iden3/go-iden3-crypto/poseidon"
)

func pedersenCommit(v, r *big.Int) *babyjub.Point {
	vG := babyjub.NewPoint().Mul(v, utils.CircuitGBabyJub)
	rH := babyjub.NewPoint().Mul(r, utils.HBabyJub)
	return babyjub.NewPoint().Projective().Add(vG.Projective(), rH.Projective()).Affine()
}

// burnWitness builds a full, internally-consistent BurnCircuit witness for
// the given (previousBalance, amount). NewCommit reuses prevR as its
// blinding factor, not an independently-chosen one — required, not a
// simplification: see circuit.go's comment on computedNewCommitment for
// why (Enygma.sol's totalSupply tracking only stays consistent with
// Σ(balances) if a burn leaves the account's blinding factor unchanged).
// Every field is derived exactly as circuit.go's Define requires,
// mirroring what a real client (go_client) would compute.
func burnWitness(t *testing.T, sk, prevR, previousBalance, amount, blockNumber *big.Int) *BurnCircuit {
	t.Helper()

	pkRaw, err := iden3poseidon.Hash([]*big.Int{sk, sk})
	if err != nil {
		t.Fatalf("pk hash: %v", err)
	}
	publicKey := new(big.Int).Mod(pkRaw, utils.P)

	prevCommit := pedersenCommit(previousBalance, prevR)

	newBalance := new(big.Int).Sub(previousBalance, amount)
	newCommit := pedersenCommit(newBalance, prevR)

	secretRemainRaw, err := iden3poseidon.Hash([]*big.Int{prevR, sk})
	if err != nil {
		t.Fatalf("secretRemain hash: %v", err)
	}
	secretRemain := new(big.Int).Mod(secretRemainRaw, utils.P)

	nullifier, err := iden3poseidon.Hash([]*big.Int{secretRemain, blockNumber})
	if err != nil {
		t.Fatalf("nullifier hash: %v", err)
	}

	w := &BurnCircuit{
		PublicKey:           publicKey,
		Amount:              amount,
		BlockNumber:         blockNumber,
		Nullifier:           nullifier,
		SecretKey:           sk,
		PreviousBalance:     previousBalance,
		PreviousRandomValue: prevR,
		DomainId:            big.NewInt(1), // Fix L-01: arbitrary fixed value, unconstrained by Define()
	}
	w.PreviousCommit[0], w.PreviousCommit[1] = prevCommit.X, prevCommit.Y
	w.NewCommit[0], w.NewCommit[1] = newCommit.X, newCommit.Y
	return w
}

func solveBurn(t *testing.T, w *BurnCircuit) error {
	t.Helper()
	solver.RegisterHint(utils.ModHint)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &BurnCircuit{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	witness, err := frontend.NewWitness(w, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("build witness: %v", err)
	}
	_, err = ccs.Solve(witness)
	return err
}

func TestBurnCircuit_HonestBurnSucceeds(t *testing.T) {
	w := burnWitness(t,
		big.NewInt(424242), // sk
		big.NewInt(67890),  // prevR
		big.NewInt(500),    // previousBalance
		big.NewInt(100),    // amount
		big.NewInt(3000))   // blockNumber
	if err := solveBurn(t, w); err != nil {
		t.Fatalf("honest burn (balance=500, amount=100) rejected: %v", err)
	}
	t.Log("honest burn (500 -> 400, amount 100) accepted")
}

// TestBurnCircuit_OverBurnRejected is H-13's core scenario: an amount
// exceeding the account's actual balance. Pre-fix, burn() had no circuit
// at all and would have accepted this via plaintext point arithmetic,
// silently wrapping the result. Post-fix, the solvency comparator rejects
// it outright — there is no proof to submit.
func TestBurnCircuit_OverBurnRejected(t *testing.T) {
	w := burnWitness(t,
		big.NewInt(424242),
		big.NewInt(67890),
		big.NewInt(100), // previousBalance
		big.NewInt(200), // amount — exceeds balance
		big.NewInt(3000))
	if err := solveBurn(t, w); err == nil {
		t.Fatal("FAIL (H-13 regressed): over-burn (balance=100, amount=200) was accepted")
	} else {
		t.Logf("over-burn correctly rejected: %v", err)
	}
}

// TestBurnCircuit_WrappedBalanceRejected reproduces the audit's specific
// evidence: a balance claimed as a value "just under P" (the wrapped
// result of a plaintext over-burn in the old, proof-less code). It must
// fail the 64-bit range check before the solvency comparator is even
// reached — the circuit cannot be satisfied by a witness claiming a
// near-P balance no honest account will ever actually have.
func TestBurnCircuit_WrappedBalanceRejected(t *testing.T) {
	wrappedBalance := new(big.Int).Sub(utils.P, big.NewInt(1)) // P-1: "just under P"
	w := burnWitness(t,
		big.NewInt(424242),
		big.NewInt(67890),
		wrappedBalance,
		big.NewInt(100),
		big.NewInt(3000))
	if err := solveBurn(t, w); err == nil {
		t.Fatal("FAIL (H-13 regressed): a near-P wrapped balance was accepted as a valid witness")
	} else {
		t.Logf("wrapped (near-P) balance correctly rejected at the range check: %v", err)
	}
}

// TestBurnCircuit_WrongSecretKeyRejected confirms the authorization half
// of the fix: a proof cannot be produced without the account's own sk,
// even by someone who happens to know the account's actual balance and
// randomness (e.g. the contract owner, who — pre-fix — could burn
// unilaterally with no cooperation from the account at all).
func TestBurnCircuit_WrongSecretKeyRejected(t *testing.T) {
	w := burnWitness(t,
		big.NewInt(424242),
		big.NewInt(67890),
		big.NewInt(500),
		big.NewInt(100),
		big.NewInt(3000))
	// Tamper: claim a different PublicKey than the one sk=424242 actually
	// derives — simulates trying to bind the proof to an account whose sk
	// the prover doesn't hold.
	w.PublicKey = big.NewInt(999999999)
	if err := solveBurn(t, w); err == nil {
		t.Fatal("FAIL: a proof with a PublicKey not matching SecretKey was accepted")
	} else {
		t.Logf("mismatched PublicKey correctly rejected: %v", err)
	}
}

// TestBurnCircuit_TamperedNewCommitRejected confirms the "correct new
// commitment" half of the remediation: a prover cannot assert an
// arbitrary NewCommit — it must actually equal
// Com(previousBalance-amount, PreviousRandomValue).
func TestBurnCircuit_TamperedNewCommitRejected(t *testing.T) {
	w := burnWitness(t,
		big.NewInt(424242),
		big.NewInt(67890),
		big.NewInt(500),
		big.NewInt(100),
		big.NewInt(3000))
	// Tamper: claim the new commitment still encodes the full balance,
	// i.e. pretend nothing was burned.
	untouched := pedersenCommit(big.NewInt(500), big.NewInt(67890))
	w.NewCommit[0], w.NewCommit[1] = untouched.X, untouched.Y
	if err := solveBurn(t, w); err == nil {
		t.Fatal("FAIL: a NewCommit not equal to Com(previousBalance-amount, PreviousRandomValue) was accepted")
	} else {
		t.Logf("tampered NewCommit correctly rejected: %v", err)
	}
}

// TestBurnCircuit_IndependentBlindingFactorRejected confirms the specific
// bug caught while integration-testing this fix against the real
// contract: a NewCommit using a *fresh, independently-chosen* blinding
// factor (rather than reusing PreviousRandomValue) is mathematically a
// valid Pedersen commitment to the correct new balance, but it is NOT
// what this circuit accepts, because Enygma.sol's totalSupply bookkeeping
// requires the account's blinding factor to stay fixed across a burn (see
// circuit.go's comment on computedNewCommitment). A circuit that allowed
// this would compile and even solve for the *wrong* NewCommit, then break
// the on-chain invariant silently — exactly what happened before this
// test existed.
func TestBurnCircuit_IndependentBlindingFactorRejected(t *testing.T) {
	w := burnWitness(t,
		big.NewInt(424242),
		big.NewInt(67890),
		big.NewInt(500),
		big.NewInt(100),
		big.NewInt(3000))
	// Tamper: a mathematically valid commitment to the correct new
	// balance (400), but blinded with a fresh r instead of reusing prevR.
	freshR := big.NewInt(11111)
	independentlyBlinded := pedersenCommit(big.NewInt(400), freshR)
	w.NewCommit[0], w.NewCommit[1] = independentlyBlinded.X, independentlyBlinded.Y
	if err := solveBurn(t, w); err == nil {
		t.Fatal("FAIL: a NewCommit with an independently-chosen blinding factor was accepted")
	} else {
		t.Logf("independently-blinded NewCommit correctly rejected: %v", err)
	}
}

func TestBurnCircuit_CompilesCleanly(t *testing.T) {
	solver.RegisterHint(utils.ModHint)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &BurnCircuit{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	t.Logf("compiled OK: nbConstraints=%d nbPublic=%d nbSecret=%d", ccs.GetNbConstraints(), ccs.GetNbPublicVariables(), ccs.GetNbSecretVariables())
}
