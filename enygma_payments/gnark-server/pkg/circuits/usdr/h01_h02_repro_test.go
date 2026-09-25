package usdr

// TestH01/TestH02 reproduce, for USDrCircuit, the same message-tag/
// blinding-factor epoch-collision found and fixed in enygma/circuit.go
// (that package's own h01_h02_repro_test.go) — usdr/circuit.go was added
// as a "near-verbatim copy" of enygma/circuit.go but initially missed the
// fix. Mirrors enygma/h01_h02_repro_test.go's approach: validate the
// vulnerable/fixed formula at the field level in isolation rather than the
// full USDrCircuit (which needs Poseidon/fingerprint witness data only
// go_client, a separate module, can build).
//
//	vulnerable: Poseidon(domain, SharedSecret, BlockNumber)
//	fixed:      Poseidon(domain, SharedSecret, computedNullifier, SenderId, AnonymitySet[i])
//
// usdr/circuit.go uses domain constants 120 (message tags) and 210
// (blinding factors) — deliberately different from enygma/circuit.go's
// 12/21 so a USDr proof and a main-asset proof for the same transaction
// (which reuse SharedSecrets/BlockNumber) don't produce correlated tags —
// but the vulnerable/fixed formula SHAPE and the collision mechanism are
// identical, so this test is otherwise a straight port.

import (
	"math/big"
	"testing"

	utils "enygma_payments/gnark-server/utils"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/iden3/go-iden3-crypto/babyjub"
	"github.com/iden3/go-iden3-crypto/constants"
	iden3poseidon "github.com/iden3/go-iden3-crypto/poseidon"

	pos "enygma_payments/gnark-server/poseidon"
)

type tagCollisionCircuit struct {
	Domain frontend.Variable

	SharedSecret frontend.Variable

	X1, X2 frontend.Variable // BlockNumber (vulnerable) or computedNullifier (fixed)

	SenderId1, ReceiverId1 frontend.Variable
	SenderId2, ReceiverId2 frontend.Variable

	withDirectionAndNonce bool // compile-time: selects vulnerable vs fixed formula shape
}

func (c *tagCollisionCircuit) Define(api frontend.API) error {
	hashDomain := pos.Poseidon(api, []frontend.Variable{c.Domain})

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
	usdrTagDomain    = 120 // H-01: message tags
	usdrRandomDomain = 210 // H-02: Pedersen blinding factors
)

func TestH01_VulnerableFormula_SameEpochTagsCollide(t *testing.T) {
	w := &tagCollisionCircuit{
		Domain:       big.NewInt(usdrTagDomain),
		SharedSecret: big.NewInt(999111),
		X1:           big.NewInt(3000),
		X2:           big.NewInt(3000),
		SenderId1:    big.NewInt(0), ReceiverId1: big.NewInt(0),
		SenderId2: big.NewInt(0), ReceiverId2: big.NewInt(0),
	}
	if err := solveCollision(t, w); err != nil {
		t.Fatalf("expected the vulnerable formula to collide across two same-epoch USDr proofs, reproducing H-01, but it did not: %v", err)
	}
	t.Log("vulnerable formula: two same-epoch USDr proofs produce an identical message tag for the same pair — confirms H-01 in usdr/circuit.go")
}

func TestH02_VulnerableFormula_SameEpochRandomFactorsCollideAndLeakAmount(t *testing.T) {
	w := &tagCollisionCircuit{
		Domain:       big.NewInt(usdrRandomDomain),
		SharedSecret: big.NewInt(999111),
		X1:           big.NewInt(3000),
		X2:           big.NewInt(3000),
		SenderId1:    big.NewInt(0), ReceiverId1: big.NewInt(0),
		SenderId2: big.NewInt(0), ReceiverId2: big.NewInt(0),
	}
	if err := solveCollision(t, w); err != nil {
		t.Fatalf("expected the vulnerable formula to collide across two same-epoch USDr proofs, reproducing H-02, but it did not: %v", err)
	}

	hashRandom, _ := iden3poseidon.Hash([]*big.Int{big.NewInt(usdrRandomDomain)})
	secret := big.NewInt(999111)
	blockHash := big.NewInt(3000)
	r, _ := iden3poseidon.Hash([]*big.Int{hashRandom, secret, blockHash})
	r.Mod(r, utils.P)

	v1, v2 := big.NewInt(10), big.NewInt(0)
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

	expected := babyjub.NewPoint().Mul(new(big.Int).Sub(v1, v2), utils.CircuitGBabyJub)
	if diff.X.Cmp(expected.X) != 0 || diff.Y.Cmp(expected.Y) != 0 {
		t.Fatalf("H component did not cancel: got (%s,%s), want (v1-v2)*G = (%s,%s)", diff.X, diff.Y, expected.X, expected.Y)
	}
	t.Log("vulnerable formula: Com(10,r) - Com(0,r) = 10*G exactly, H cancelled — confirms H-02's amount-recovery evidence in usdr/circuit.go")
}

func TestFixedFormula_PerTxNonceBreaksCollision(t *testing.T) {
	for _, domain := range []int64{usdrTagDomain, usdrRandomDomain} {
		w := &tagCollisionCircuit{
			Domain:                big.NewInt(domain),
			SharedSecret:          big.NewInt(999111),
			X1:                    big.NewInt(111111),
			X2:                    big.NewInt(222222),
			SenderId1:             big.NewInt(0),
			ReceiverId1:           big.NewInt(1),
			SenderId2:             big.NewInt(0),
			ReceiverId2:           big.NewInt(1),
			withDirectionAndNonce: true,
		}
		if err := solveCollision(t, w); err == nil {
			t.Fatalf("FAIL (domain %d): fixed formula still collided across two proofs with different nonces", domain)
		} else {
			t.Logf("domain %d: fixed formula correctly rejected the cross-proof collision (different nullifier): %v", domain, err)
		}
	}
}

func TestFixedFormula_DirectionSwapBreaksCollision(t *testing.T) {
	for _, domain := range []int64{usdrTagDomain, usdrRandomDomain} {
		w := &tagCollisionCircuit{
			Domain:                big.NewInt(domain),
			SharedSecret:          big.NewInt(999111),
			X1:                    big.NewInt(555555),
			X2:                    big.NewInt(555555),
			SenderId1:             big.NewInt(0),
			ReceiverId1:           big.NewInt(1),
			SenderId2:             big.NewInt(1),
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

func TestFixedFormula_SameTransactionRoundTrips(t *testing.T) {
	for _, domain := range []int64{usdrTagDomain, usdrRandomDomain} {
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
			t.Fatalf("FAIL (domain %d): re-deriving the same USDr proof's tag/random-factor twice should match, but it didn't: %v", domain, err)
		}
	}
	t.Log("fixed formula is still deterministic for a legitimate receiver re-deriving their own credit")
}

// TestUSDrCircuit_CompilesWithH01H02Fix compiles the real, full
// USDrCircuit (not the isolated gadget above) with k=6, the size every
// deployment actually uses — the test that would have caught a >3-input
// Poseidon call (enygma-server/poseidon's S-box table only covers up to 3
// inputs per call; frontend.Compile panics on more, which package-level
// `go build` cannot catch).
func TestUSDrCircuit_CompilesWithH01H02Fix(t *testing.T) {
	k := 6
	fp := make([][]frontend.Variable, k)
	for i := range fp {
		fp[i] = make([]frontend.Variable, k)
	}
	circuit := USDrCircuit{
		Config:                     USDrCircuitConfig{NCommitment: k},
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
