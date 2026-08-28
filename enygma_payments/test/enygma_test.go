package enygma_test

// Standalone unit tests for Enygma cryptographic invariants.
//
// These tests verify the mathematical properties that the gnark circuit
// enforces as constraints, and the on-chain checks in Enygma.sol.
// No running chain or gnark server is required.
//
// Run:
//
//	cd enygma_payments/test && go mod tidy && go test ./... -v

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/iden3/go-iden3-crypto/babyjub"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

// ── Curve parameters (mirrors go_client/internal/curve/curve.go) ─────────────

var (
	// G: NUMS hash-to-curve derivation, seed "2" (H-11 fix). Reproduce with:
	// cd gnark-server && go run ./cmd/derive_generator
	gx, _ = new(big.Int).SetString("12337812418750581066638756637363471856433191340622504180842886595232027947307", 10)
	gy, _ = new(big.Int).SetString("15225366398330386329633463986700597127113326976080712967801565482915963669722", 10)
	G     = &babyjub.Point{X: gx, Y: gy}

	hx, _ = new(big.Int).SetString("10100005861917718053548237064487763771145251762383025193119768015180892676690", 10)
	hy, _ = new(big.Int).SetString("7512830269827713629724023825249861327768672768516116945507944076335453576011", 10)
	H     = &babyjub.Point{X: hx, Y: hy}

	// Baby Jubjub subgroup order
	P, _ = new(big.Int).SetString("2736030358979909402780800718157159386076813972158567259200215660948447373041", 10)
)

// pedersenCommitment computes Com(v, r) = v*G + r*H on Baby Jubjub.
func pedersenCommitment(v, r *big.Int) *babyjub.Point {
	vG := babyjub.NewPoint().Mul(v, G)
	rH := babyjub.NewPoint().Mul(r, H)
	return babyjub.NewPoint().Projective().Add(vG.Projective(), rH.Projective()).Affine()
}

// getNegative returns P - x (additive inverse mod P).
func getNegative(x *big.Int) *big.Int {
	return new(big.Int).Sub(P, new(big.Int).Mod(x, P))
}

// identity is the Baby Jubjub neutral element (0, 1).
func identity() *babyjub.Point {
	return &babyjub.Point{X: big.NewInt(0), Y: big.NewInt(1)}
}

func pointAdd(a, b *babyjub.Point) *babyjub.Point {
	return babyjub.NewPoint().Projective().Add(a.Projective(), b.Projective()).Affine()
}

func pointsEqual(a, b *babyjub.Point) bool {
	return a.X.Cmp(b.X) == 0 && a.Y.Cmp(b.Y) == 0
}

// ── 1. Field arithmetic ───────────────────────────────────────────────────────

// GetNegative(v) is the additive inverse mod P: v + GetNegative(v) ≡ 0 (mod P)
func TestGetNegative(t *testing.T) {
	cases := []string{"1", "100", "999999", P.String()}
	for _, s := range cases {
		v := new(big.Int)
		v.SetString(s, 10)
		neg := getNegative(v)
		sum := new(big.Int).Add(v, neg)
		sum.Mod(sum, P)
		if sum.Sign() != 0 {
			t.Errorf("v + GetNegative(v) mod P = %s (want 0) for v=%s", sum, s)
		}
	}
}

// ── 2. Pedersen commitment binding ───────────────────────────────────────────

// Two different (v, r) pairs must produce different commitments — binding.
func TestPedersenCommitmentBinding(t *testing.T) {
	c1 := pedersenCommitment(big.NewInt(100), big.NewInt(42))
	c2 := pedersenCommitment(big.NewInt(101), big.NewInt(42)) // different value
	c3 := pedersenCommitment(big.NewInt(100), big.NewInt(43)) // different randomness

	if pointsEqual(c1, c2) {
		t.Error("Com(100,42) == Com(101,42): commitment is not value-binding")
	}
	if pointsEqual(c1, c3) {
		t.Error("Com(100,42) == Com(100,43): commitment is not randomness-binding")
	}
}

// Com(0, r) for any r should not equal the identity — hiding property.
func TestPedersenCommitmentHiding(t *testing.T) {
	id := identity()
	r := big.NewInt(999)
	c := pedersenCommitment(big.NewInt(0), r)
	if pointsEqual(c, id) {
		t.Error("Com(0, r) equals identity — commitment is not hiding")
	}
}

// Homomorphic property: Com(a,r1) + Com(b,r2) == Com(a+b, r1+r2)
func TestPedersenHomomorphic(t *testing.T) {
	a, b := big.NewInt(30), big.NewInt(70)
	r1, r2 := big.NewInt(11), big.NewInt(22)

	ca := pedersenCommitment(a, r1)
	cb := pedersenCommitment(b, r2)
	sum := pointAdd(ca, cb)

	ab := new(big.Int).Add(a, b)
	r12 := new(big.Int).Add(r1, r2)
	direct := pedersenCommitment(ab, r12)

	if !pointsEqual(sum, direct) {
		t.Error("Pedersen commitment is not homomorphic: Com(a)+Com(b) ≠ Com(a+b)")
	}
}

// ── 3. Value conservation (core circuit invariant) ───────────────────────────

// The circuit asserts: sum of all TxCommit points == identity (0,1).
// This means sum(txValues) == 0 AND sum(txRandoms) == 0 (mod P).
// Test a valid 6-party transfer: sender sends 100, split 60/40 to two receivers.
func TestValueConservation_ValidTransfer(t *testing.T) {
	// Sender (id=0) sends 100: TxValue[0] = -100 = P - 100
	// Receiver 1 (id=1) gets 60:  TxValue[1] = 60
	// Receiver 2 (id=2) gets 40:  TxValue[2] = 40
	// Others (id=3,4,5) get 0
	txValues := []*big.Int{
		getNegative(big.NewInt(100)),
		big.NewInt(60),
		big.NewInt(40),
		big.NewInt(0),
		big.NewInt(0),
		big.NewInt(0),
	}

	// Random factors must also sum to zero for the commitment sum to be identity.
	// Use r1=10, r2=6, r3=4, rest=0 so sender gets -(sum of others) = -20 → P-20.
	r := []*big.Int{
		getNegative(big.NewInt(20)), // sender: negation of sum of others
		big.NewInt(6),
		big.NewInt(4),
		big.NewInt(10),
		big.NewInt(0),
		big.NewInt(0),
	}

	sum := identity()
	for i := 0; i < 6; i++ {
		c := pedersenCommitment(txValues[i], r[i])
		sum = pointAdd(sum, c)
	}

	if !pointsEqual(sum, identity()) {
		t.Errorf("sum of TxCommit points ≠ identity: got (%s, %s)", sum.X, sum.Y)
	}
}

// An invalid transfer (values don't sum to zero) must NOT produce identity.
func TestValueConservation_InvalidTransfer(t *testing.T) {
	// Sender "forgets" to deduct — sends 60+40=100 but keeps their balance too.
	txValues := []*big.Int{
		big.NewInt(0), // sender keeps full balance (should be -100)
		big.NewInt(60),
		big.NewInt(40),
		big.NewInt(0),
		big.NewInt(0),
		big.NewInt(0),
	}
	r := []*big.Int{
		big.NewInt(0),
		big.NewInt(6),
		big.NewInt(4),
		big.NewInt(0),
		big.NewInt(0),
		big.NewInt(0),
	}

	sum := identity()
	for i := 0; i < 6; i++ {
		c := pedersenCommitment(txValues[i], r[i])
		sum = pointAdd(sum, c)
	}

	if pointsEqual(sum, identity()) {
		t.Error("invalid transfer (no deduction from sender) produced identity — value conservation violated")
	}
}

// ── 4. Nullifier properties (Fix C-2) ────────────────────────────────────────

// The nullifier formula is Poseidon(sk, blockNumber).
// We don't change this — we only add on-chain storage.
// These tests verify the properties the on-chain storage relies on.

// Same sk, different blocks → different nullifiers (no block replay).
func TestNullifier_DifferentBlocks(t *testing.T) {
	sk := big.NewInt(1234567890)
	block1 := big.NewInt(100)
	block2 := big.NewInt(101)

	n1, _ := poseidon.Hash([]*big.Int{sk, block1})
	n2, _ := poseidon.Hash([]*big.Int{sk, block2})

	if n1.Cmp(n2) == 0 {
		t.Error("Poseidon(sk, block1) == Poseidon(sk, block2): nullifier does not distinguish blocks")
	}
}

// Different sks, same block → different nullifiers (no cross-sender collision).
func TestNullifier_DifferentSks(t *testing.T) {
	sk1 := big.NewInt(1111)
	sk2 := big.NewInt(2222)
	block := big.NewInt(500)

	n1, _ := poseidon.Hash([]*big.Int{sk1, block})
	n2, _ := poseidon.Hash([]*big.Int{sk2, block})

	if n1.Cmp(n2) == 0 {
		t.Error("Poseidon(sk1, block) == Poseidon(sk2, block): different senders produce the same nullifier")
	}
}

// Nullifier is deterministic: same inputs always produce the same output.
func TestNullifier_Deterministic(t *testing.T) {
	sk := big.NewInt(99887766)
	block := big.NewInt(42)

	n1, _ := poseidon.Hash([]*big.Int{sk, block})
	n2, _ := poseidon.Hash([]*big.Int{sk, block})

	if n1.Cmp(n2) != 0 {
		t.Error("Poseidon is not deterministic")
	}
}

// On-chain nullifier storage (Fix C-2): simulate the _consumeNullifier mapping.
// A nullifier used once must be rejected on re-use, regardless of proof validity.
func TestNullifier_OnChainStorage_PreventsReplay(t *testing.T) {
	consumed := make(map[string]bool)

	consumeNullifier := func(nullifier *big.Int) error {
		key := nullifier.String()
		if consumed[key] {
			return fmt.Errorf("NullifierAlreadyUsed")
		}
		consumed[key] = true
		return nil
	}

	sk := big.NewInt(555)
	block := big.NewInt(200)
	nullifier, _ := poseidon.Hash([]*big.Int{sk, block})

	// First use: should succeed.
	if err := consumeNullifier(nullifier); err != nil {
		t.Fatalf("first nullifier use failed: %v", err)
	}

	// Second use with identical proof: must be rejected.
	if err := consumeNullifier(nullifier); err == nil {
		t.Error("replay with same nullifier was accepted — double-spend not prevented")
	}

	// Different block → different nullifier → must succeed.
	block2 := big.NewInt(201)
	nullifier2, _ := poseidon.Hash([]*big.Int{sk, block2})
	if err := consumeNullifier(nullifier2); err != nil {
		t.Fatalf("new nullifier (different block) incorrectly rejected: %v", err)
	}
}

// ── 5. CommitmentDelta integrity (Fix C-1) ───────────────────────────────────

// The circuit proves TxCommit[i] = Com(txValue[i], txRandom[i]).
// Fix C-1: _verifyPublicInputs now enforces commitmentDeltas[i] == TxCommit from proof.
// Simulate both the honest case and the attack case.

type point struct{ x, y *big.Int }

func txCommitFromProof(txValues, txRandoms []*big.Int) []point {
	out := make([]point, len(txValues))
	for i := range txValues {
		c := pedersenCommitment(txValues[i], txRandoms[i])
		out[i] = point{new(big.Int).Set(c.X), new(big.Int).Set(c.Y)}
	}
	return out
}

// verifyCommitmentDeltas simulates the fixed _verifyPublicInputs check.
func verifyCommitmentDeltas(callerDeltas, proofTxCommit []point) bool {
	if len(callerDeltas) != len(proofTxCommit) {
		return false
	}
	for i := range callerDeltas {
		if callerDeltas[i].x.Cmp(proofTxCommit[i].x) != 0 ||
			callerDeltas[i].y.Cmp(proofTxCommit[i].y) != 0 {
			return false
		}
	}
	return true
}

// Honest caller: commitmentDeltas match TxCommit from the proof → accepted.
func TestCommitmentDelta_HonestCaller(t *testing.T) {
	txValues := []*big.Int{getNegative(big.NewInt(100)), big.NewInt(60), big.NewInt(40), big.NewInt(0), big.NewInt(0), big.NewInt(0)}
	txRandoms := []*big.Int{big.NewInt(10), big.NewInt(3), big.NewInt(7), big.NewInt(0), big.NewInt(0), big.NewInt(0)}

	proofTxCommit := txCommitFromProof(txValues, txRandoms)
	callerDeltas := proofTxCommit // honest: same values

	if !verifyCommitmentDeltas(callerDeltas, proofTxCommit) {
		t.Error("honest caller was incorrectly rejected by commitmentDelta check")
	}
}

// Attack case (C-1 pre-fix): caller supplies fraudulent deltas that don't match
// the proof's TxCommit — the fixed check must reject them.
func TestCommitmentDelta_FraudulentCaller_Rejected(t *testing.T) {
	txValues := []*big.Int{getNegative(big.NewInt(100)), big.NewInt(60), big.NewInt(40), big.NewInt(0), big.NewInt(0), big.NewInt(0)}
	txRandoms := []*big.Int{big.NewInt(10), big.NewInt(3), big.NewInt(7), big.NewInt(0), big.NewInt(0), big.NewInt(0)}

	proofTxCommit := txCommitFromProof(txValues, txRandoms)

	// Attacker supplies deltas that would drain everyone's balance to themselves.
	drainValue := big.NewInt(1_000_000)
	fraudulentDeltas := make([]point, 6)
	for i := range fraudulentDeltas {
		c := pedersenCommitment(drainValue, big.NewInt(0))
		fraudulentDeltas[i] = point{new(big.Int).Set(c.X), new(big.Int).Set(c.Y)}
	}

	if verifyCommitmentDeltas(fraudulentDeltas, proofTxCommit) {
		t.Error("fraudulent commitmentDeltas were accepted — Fix C-1 is broken")
	}
}

// ── 6. Range checks ──────────────────────────────────────────────────────────

// The circuit enforces: previousBalance > txAmount > 0.
// These tests verify the intended semantics of those constraints.

func TestRangeCheck_PositiveAmount(t *testing.T) {
	previousV := big.NewInt(200)
	txAmount := big.NewInt(100)

	// previousV > txAmount (circuit: Cmp returns 1 when a > b)
	if previousV.Cmp(txAmount) != 1 {
		t.Error("previousV should be > txAmount")
	}
	// txAmount > 0
	if txAmount.Cmp(big.NewInt(0)) != 1 {
		t.Error("txAmount should be > 0")
	}
}

func TestRangeCheck_ZeroAmountRejected(t *testing.T) {
	txAmount := big.NewInt(0)
	// Circuit: api.AssertIsEqual(api.Cmp(V, 0), 1) — V must be strictly > 0
	if txAmount.Cmp(big.NewInt(0)) == 1 {
		t.Error("zero amount should not pass the range check")
	}
}

func TestRangeCheck_OverspendRejected(t *testing.T) {
	previousV := big.NewInt(50)
	txAmount := big.NewInt(100) // trying to send more than balance
	// Circuit: api.AssertIsEqual(api.Cmp(previousV, V), 1) — previousV must be > V
	if previousV.Cmp(txAmount) == 1 {
		t.Error("overspend should not pass the range check")
	}
}

// ── 7. Spend key authentication ───────────────────────────────────────────────

// Circuit enforces: pk_sender == sk * G (knowledge of sk).
// Verify that sk * G produces the correct public key and that a wrong sk fails.
func TestSpendKeyAuthentication(t *testing.T) {
	sk := big.NewInt(424242)
	pk := babyjub.NewPoint().Mul(sk, G)

	wrongSk := big.NewInt(424243)
	wrongPk := babyjub.NewPoint().Mul(wrongSk, G)

	if pointsEqual(pk, wrongPk) {
		t.Error("different secret keys produced the same public key")
	}

	// Re-derive to confirm determinism
	pk2 := babyjub.NewPoint().Mul(sk, G)
	if !pointsEqual(pk, pk2) {
		t.Error("sk * G is not deterministic")
	}
}

// ── 8. Hash-array generation (shared secrets) ─────────────────────────────────

// HashArrayGen: hashArray[j] = Poseidon(secrets[j], secrets[j])
// Must be deterministic and unique per secret.
func TestHashArrayGen_Determinism(t *testing.T) {
	secrets := []*big.Int{big.NewInt(111), big.NewInt(222), big.NewInt(333)}

	hash := func(s []*big.Int) []*big.Int {
		out := make([]*big.Int, len(s))
		for j, v := range s {
			h, _ := poseidon.Hash([]*big.Int{v, v})
			h.Mod(h, P)
			out[j] = h
		}
		return out
	}

	h1 := hash(secrets)
	h2 := hash(secrets)

	for i := range h1 {
		if h1[i].Cmp(h2[i]) != 0 {
			t.Errorf("HashArrayGen[%d] is not deterministic", i)
		}
	}
}

func TestHashArrayGen_UniquenessPerSecret(t *testing.T) {
	s1 := big.NewInt(111)
	s2 := big.NewInt(112)

	h1, _ := poseidon.Hash([]*big.Int{s1, s1})
	h2, _ := poseidon.Hash([]*big.Int{s2, s2})
	h1.Mod(h1, P)
	h2.Mod(h2, P)

	if h1.Cmp(h2) == 0 {
		t.Error("different secrets produced the same hash — hash array is not unique per secret")
	}
}
