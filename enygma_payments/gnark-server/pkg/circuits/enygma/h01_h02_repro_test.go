package enygma

// TestH01/TestH02 reproduce H-01 (mechanism 1: message-tag symmetry +
// epoch-constant anchor) and H-02 (epoch-constant, direction-symmetric
// blinding factors) at the KDF-formula level, mirroring the C-01..C-08
// tests' approach of validating the vulnerable mechanism in isolation
// rather than the full enygma circuit (which needs Poseidon/Pedersen
// witness data only go_client, a separate module, can build).
//
// Both findings share one root cause and one fix, applied identically to
// two Poseidon calls that differ only in a leading domain-separation
// constant (12 for message tags, 21 for blinding factors):
//
//	vulnerable: Poseidon(domain, SharedSecret, BlockNumber)
//	fixed:      Poseidon(domain, SharedSecret, computedNullifier, SenderId, AnonymitySet[i])
//
// BlockNumber is the epoch anchor (constant for every transaction in an
// epoch); computedNullifier is guaranteed fresh on every transaction (it
// is Poseidon(secretRemain, BlockNumber), and secretRemain depends on the
// sender's own previous-commitment blinding factor, which changes on every
// send). SharedSecret is direction-agnostic (agreement/manager.go's
// pairKey canonicalises min_max), so the vulnerable formula collides
// whichever of a pair sends; SenderId/AnonymitySet[i] mixed in as ordered
// (not summed) inputs break that even if the nonce were somehow reused.
//
// tagCollisionCircuit computes the same formula twice (as it would for two
// different transactions sharing a pairwise secret) and asserts the two
// outputs are equal. Solve succeeding therefore means "these two
// transactions produced an identical value" — the exact observable an
// audit's passive chain observer exploits — and Solve failing means the
// fix broke that collision. This is the inverse framing of C-01..C-08's
// "does the circuit reject a bad witness" tests because H-01/H-02 are
// privacy (linkability), not soundness, findings: there is no invalid
// witness to reject, only an unwanted equality between two valid ones.

import (
	"math/big"
	"testing"

	utils "enygma-server/utils"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/iden3/go-iden3-crypto/babyjub"
	"github.com/iden3/go-iden3-crypto/constants"
	iden3poseidon "github.com/iden3/go-iden3-crypto/poseidon"

	pos "enygma-server/poseidon"
)

// tagCollisionCircuit computes Poseidon(domain, secret, X, [senderId,
// receiverId]) twice — once per side — and asserts the two results are
// equal. withDirectionAndNonce toggles between the vulnerable formula
// (X only) and the fixed one (X plus ordered SenderId/AnonymitySet[i]).
type tagCollisionCircuit struct {
	Domain frontend.Variable // 12 (message tag) or 21 (random factor)

	SharedSecret frontend.Variable // same pairwise secret both sides — it's the constant that must NOT be enough on its own to link transactions

	X1, X2 frontend.Variable // BlockNumber (vulnerable) or computedNullifier (fixed)

	SenderId1, ReceiverId1 frontend.Variable
	SenderId2, ReceiverId2 frontend.Variable

	withDirectionAndNonce bool // compile-time: selects vulnerable vs fixed formula shape
}

func (c *tagCollisionCircuit) Define(api frontend.API) error {
	hashDomain := pos.Poseidon(api, []frontend.Variable{c.Domain})

	// Mirrors circuit.go exactly, including the reason for folding
	// SenderId/ReceiverId/X into one perSlotNonce via two chained 2-input
	// calls: enygma-server/poseidon's S-box constant table only covers
	// state size t = len(inputs)+1 ∈ {2,3,4}, i.e. at most 3 inputs per
	// call, so a direct 4-input call panics inside frontend.Compile.
	var raw1, raw2 frontend.Variable
	if c.withDirectionAndNonce {
		dir1 := pos.Poseidon(api, []frontend.Variable{c.SenderId1, c.ReceiverId1})
		dir2 := pos.Poseidon(api, []frontend.Variable{c.SenderId2, c.ReceiverId2})
		nonce1 := pos.Poseidon(api, []frontend.Variable{c.X1, dir1})
		nonce2 := pos.Poseidon(api, []frontend.Variable{c.X2, dir2})
		raw1 = pos.Poseidon(api, []frontend.Variable{hashDomain, c.SharedSecret, nonce1})
		raw2 = pos.Poseidon(api, []frontend.Variable{hashDomain, c.SharedSecret, nonce2})
	} else {
		raw1 = pos.Poseidon(api, []frontend.Variable{hashDomain, c.SharedSecret, c.X1})
		raw2 = pos.Poseidon(api, []frontend.Variable{hashDomain, c.SharedSecret, c.X2})
	}

	v1 := utils.ReduceModP(api, raw1)
	v2 := utils.ReduceModP(api, raw2)
	api.AssertIsEqual(v1, v2)
	return nil
}

func solveCollision(t *testing.T, w *tagCollisionCircuit) error {
	t.Helper()
	solver.RegisterHint(utils.ModHint)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &tagCollisionCircuit{withDirectionAndNonce: w.withDirectionAndNonce})
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

const (
	tagDomain    = 12 // H-01: message tags
	randomDomain = 21 // H-02: Pedersen blinding factors
)

// TestH01_VulnerableFormula_SameEpochTagsCollide reproduces the audit's
// core evidence: bank 0 sends two transfers in the same epoch
// (identical BlockNumber). Under the vulnerable formula, the message tag
// for any non-sender slot depends only on (SharedSecret, BlockNumber) —
// both identical across the two transactions — so it is byte-identical
// both times, the "5 of 6 slots match, 1 differs" signature that singles
// out the sender.
func TestH01_VulnerableFormula_SameEpochTagsCollide(t *testing.T) {
	w := &tagCollisionCircuit{
		Domain:       big.NewInt(tagDomain),
		SharedSecret: big.NewInt(999111), // s_{bank0,bank1} — same secret, same pair, both transactions
		X1:           big.NewInt(3000),   // BlockNumber (epoch anchor) — tx1
		X2:           big.NewInt(3000),   // BlockNumber — tx2, same epoch: IDENTICAL
		// SenderId/ReceiverId unused by the vulnerable (withDirectionAndNonce=false)
		// formula, but every exported field still needs a witness assignment.
		SenderId1: big.NewInt(0), ReceiverId1: big.NewInt(0),
		SenderId2: big.NewInt(0), ReceiverId2: big.NewInt(0),
	}
	if err := solveCollision(t, w); err != nil {
		t.Fatalf("expected the vulnerable formula to collide across two same-epoch transactions, reproducing H-01, but it did not: %v", err)
	}
	t.Log("vulnerable formula: two same-epoch transactions produce an identical message tag for the same pair — confirms H-01 mechanism 1")
}

// TestH02_VulnerableFormula_SameEpochRandomFactorsCollideAndLeakAmount goes
// one step further than the tag test, reproducing H-02's actual headline
// evidence: two same-epoch transactions using the same blinding-factor
// formula produce the same r for a given pair, so subtracting their
// Pedersen commitments at that slot cancels the H term exactly — leaving
// (v1-v2)*G in the clear, solvable by table lookup since the protocol
// requires small amounts.
func TestH02_VulnerableFormula_SameEpochRandomFactorsCollideAndLeakAmount(t *testing.T) {
	w := &tagCollisionCircuit{
		Domain:       big.NewInt(randomDomain),
		SharedSecret: big.NewInt(999111),
		X1:           big.NewInt(3000),
		X2:           big.NewInt(3000),
		SenderId1:    big.NewInt(0), ReceiverId1: big.NewInt(0),
		SenderId2: big.NewInt(0), ReceiverId2: big.NewInt(0),
	}
	if err := solveCollision(t, w); err != nil {
		t.Fatalf("expected the vulnerable formula to collide across two same-epoch transactions, reproducing H-02, but it did not: %v", err)
	}

	// Confirm the actual attack, off-circuit: with r1 == r2 (just proven
	// above), Com(v1,r) - Com(v2,r) = (v1-v2)*G + (r-r)*H = (v1-v2)*G.
	// Uses CircuitGBabyJub/HBabyJub directly (the generators the real
	// circuit's utils.PedersenCommitment gadget uses via utils.G/H) rather
	// than the package's since-deleted PedersenCommitmentBabyJub/GetPK
	// helpers, which built on the *standard* iden3 base point and had no
	// live caller anywhere — see H-11's commit message and Fix L-15.
	hashRandom, _ := iden3poseidon.Hash([]*big.Int{big.NewInt(randomDomain)})
	secret := big.NewInt(999111)
	blockHash := big.NewInt(3000)
	r, _ := iden3poseidon.Hash([]*big.Int{hashRandom, secret, blockHash})
	r.Mod(r, utils.P)

	v1, v2 := big.NewInt(60), big.NewInt(0) // tx1 credits this receiver 60; tx2 pads the slot with 0 (audit's "known-plaintext padding" observation)
	pedersenCommit := func(v, rr *big.Int) *babyjub.Point {
		vG := babyjub.NewPoint().Mul(v, utils.CircuitGBabyJub)
		rH := babyjub.NewPoint().Mul(rr, utils.HBabyJub)
		return babyjub.NewPoint().Projective().Add(vG.Projective(), rH.Projective()).Affine()
	}
	negatePoint := func(p *babyjub.Point) *babyjub.Point {
		return &babyjub.Point{X: new(big.Int).Sub(constants.Q, p.X), Y: new(big.Int).Set(p.Y)}
	}

	c1 := pedersenCommit(v1, r)
	c2 := pedersenCommit(v2, r)
	diff := babyjub.NewPoint().Projective().Add(c1.Projective(), negatePoint(c2).Projective()).Affine()

	expected := babyjub.NewPoint().Mul(new(big.Int).Sub(v1, v2), utils.CircuitGBabyJub) // (v1-v2)*G, computed independently
	if diff.X.Cmp(expected.X) != 0 || diff.Y.Cmp(expected.Y) != 0 {
		t.Fatalf("H component did not cancel: got (%s,%s), want (v1-v2)*G = (%s,%s)", diff.X, diff.Y, expected.X, expected.Y)
	}
	t.Log("vulnerable formula: Com(60,r) - Com(0,r) = 60*G exactly, H cancelled — confirms H-02's amount-recovery evidence")
}

// TestFixedFormula_PerTxNonceBreaksCollision confirms the fix's first half:
// even holding SharedSecret, SenderId and AnonymitySet[i] fixed, two
// transactions with different computedNullifier values no longer collide.
// Applies to both domains — the formula shape is identical.
func TestFixedFormula_PerTxNonceBreaksCollision(t *testing.T) {
	for _, domain := range []int64{tagDomain, randomDomain} {
		w := &tagCollisionCircuit{
			Domain:                big.NewInt(domain),
			SharedSecret:          big.NewInt(999111),
			X1:                    big.NewInt(111111), // computedNullifier, tx1
			X2:                    big.NewInt(222222), // computedNullifier, tx2 — different, as guaranteed by nullifier freshness
			SenderId1:             big.NewInt(0),
			ReceiverId1:           big.NewInt(1),
			SenderId2:             big.NewInt(0),
			ReceiverId2:           big.NewInt(1),
			withDirectionAndNonce: true,
		}
		if err := solveCollision(t, w); err == nil {
			t.Fatalf("FAIL (domain %d): fixed formula still collided across two transactions with different nonces", domain)
		} else {
			t.Logf("domain %d: fixed formula correctly rejected the cross-transaction collision (different nullifier): %v", domain, err)
		}
	}
}

// TestFixedFormula_DirectionSwapBreaksCollision confirms the fix's second
// half in isolation: even if the nonce were somehow identical (a future
// regression, hypothetically), swapping sender/receiver order changes the
// hash preimage under the symmetric secret. Defense in depth for the
// direction-correlation half of both findings.
func TestFixedFormula_DirectionSwapBreaksCollision(t *testing.T) {
	for _, domain := range []int64{tagDomain, randomDomain} {
		w := &tagCollisionCircuit{
			Domain:                big.NewInt(domain),
			SharedSecret:          big.NewInt(999111), // s_{0,1} == s_{1,0}: same secret, both directions
			X1:                    big.NewInt(555555), // same nonce both sides — isolates the direction effect
			X2:                    big.NewInt(555555),
			SenderId1:             big.NewInt(0), // A -> B
			ReceiverId1:           big.NewInt(1),
			SenderId2:             big.NewInt(1), // B -> A (swapped)
			ReceiverId2:           big.NewInt(0),
			withDirectionAndNonce: true,
		}
		if err := solveCollision(t, w); err == nil {
			t.Fatalf("FAIL (domain %d): fixed formula collided across swapped sender/receiver order under a symmetric secret", domain)
		} else {
			t.Logf("domain %d: fixed formula correctly rejected the direction-swapped collision: %v", domain, err)
		}
	}
}

// TestFixedFormula_SameTransactionRoundTrips is the honest-path sanity
// check: evaluating the fixed formula twice with byte-identical inputs
// (i.e. re-deriving the same real transaction, which is exactly what a
// legitimate receiver does to recognise their own credit) must still
// succeed — the fix must not make the formula non-deterministic.
func TestFixedFormula_SameTransactionRoundTrips(t *testing.T) {
	for _, domain := range []int64{tagDomain, randomDomain} {
		w := &tagCollisionCircuit{
			Domain:                big.NewInt(domain),
			SharedSecret:          big.NewInt(999111),
			X1:                    big.NewInt(111111),
			X2:                    big.NewInt(111111),
			SenderId1:             big.NewInt(0),
			ReceiverId1:           big.NewInt(1),
			SenderId2:             big.NewInt(0),
			ReceiverId2:           big.NewInt(1),
			withDirectionAndNonce: true,
		}
		if err := solveCollision(t, w); err != nil {
			t.Fatalf("FAIL (domain %d): re-deriving the same transaction's tag/random-factor twice should match, but it didn't: %v", domain, err)
		}
	}
	t.Log("fixed formula is still deterministic for a legitimate receiver re-deriving their own credit")
}

// TestEnygmaCircuit_CompilesWithH01H02Fix compiles the real, full
// EnygmaCircuit (not the isolated gadget above) with k=6, the size every
// deployment actually uses. This is the test that would have caught the
// fix's first draft: enygma-server/poseidon's S-box constant table only
// covers up to 3 inputs per Poseidon call, and a first attempt at this fix
// used 4-5 (SenderId and AnonymitySet[i] appended directly to the existing
// HashTag/SharedSecrets[i]/computedNullifier call) — frontend.Compile
// panics on that, which frontend.Compile-free package-level `go build`
// cannot catch. If a future change reintroduces a >3-input Poseidon call
// anywhere in this circuit, this test is what fails.
func TestEnygmaCircuit_CompilesWithH01H02Fix(t *testing.T) {
	k := 6
	fp := make([][]frontend.Variable, k)
	for i := range fp {
		fp[i] = make([]frontend.Variable, k)
	}
	circuit := EnygmaCircuit{
		Config:                     EnygmaCircuitConfig{NCommitment: k},
		FingerPrintofSharedSecrets: fp,
		PublicKey:                  make([]frontend.Variable, k),
		PreviousCommit:             make([][2]frontend.Variable, k),
		TxCommit:                   make([][2]frontend.Variable, k),
		AnonymitySet:               make([]frontend.Variable, k),
		SharedSecrets:              make([]frontend.Variable, k),
		MessageTags:                make([]frontend.Variable, k),
		TxValues:                   make([]frontend.Variable, k),
		TxRandomValues:             make([]frontend.Variable, k),
	}
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	t.Logf("compiled OK: nbConstraints=%d nbPublic=%d nbSecret=%d", ccs.GetNbConstraints(), ccs.GetNbPublicVariables(), ccs.GetNbSecretVariables())
}
