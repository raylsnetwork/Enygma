// Package adversarial holds soundness tests that run against the REAL
// Define() of the payments circuits rather than a reduced copy of one
// mechanism (the *_repro_test.go files next to each circuit isolate a single
// gadget). They answer two questions the reduced tests cannot: does the
// shipped circuit actually reject the attack, and does it accept everything
// an honest prover produces.
package adversarial

import (
	"math/big"
	"testing"

	burn "enygma_payments/gnark-server/pkg/circuits/burn"
	enygma "enygma_payments/gnark-server/pkg/circuits/enygma"
	usdr "enygma_payments/gnark-server/pkg/circuits/usdr"
	utils "enygma_payments/gnark-server/utils"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/test"
	"github.com/iden3/go-iden3-crypto/babyjub"
	iden3poseidon "github.com/iden3/go-iden3-crypto/poseidon"
)

// The generators must lie in the prime-order subgroup: every "mod P"
// argument in the C-02/C-03 fixes (Com(b,r) == Com(b+P,r)) assumes it.
func TestGeneratorsAreInThePrimeOrderSubgroup(t *testing.T) {
	one := big.NewInt(1)
	for name, pt := range map[string]*babyjub.Point{"G": utils.CircuitGBabyJub, "H": utils.HBabyJub} {
		if !pt.InCurve() {
			t.Fatalf("%s is not on the curve", name)
		}
		pg := babyjub.NewPoint().Mul(utils.P, pt)
		if pg.X.Sign() != 0 || pg.Y.Cmp(one) != 0 {
			t.Errorf("P*%s is not the identity: %s is outside the prime-order subgroup", name, name)
		}
		for _, m := range []int64{1, 2, 4, 8} {
			q := babyjub.NewPoint().Mul(big.NewInt(m), pt)
			if q.X.Sign() == 0 && q.Y.Cmp(one) == 0 {
				t.Errorf("%d*%s is the identity: %s has small order", m, name, name)
			}
		}
	}
	if utils.CircuitGBabyJub.X.Cmp(utils.HBabyJub.X) == 0 && utils.CircuitGBabyJub.Y.Cmp(utils.HBabyJub.Y) == 0 {
		t.Error("G == H")
	}
}

// Every public wire must appear in at least one constraint. A public input
// that no constraint touches is malleable: Groth16 verifies the same proof
// for any value of it (its verifying-key term is zero).
func TestEveryPublicWireIsConstrained(t *testing.T) {
	solver.RegisterHint(utils.ModHint)
	k := 6
	fp := func() [][]frontend.Variable {
		m := make([][]frontend.Variable, k)
		for i := range m {
			m[i] = make([]frontend.Variable, k)
		}
		return m
	}
	check := func(name string, c frontend.Circuit) {
		ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, c)
		if err != nil {
			t.Fatalf("%s compile: %v", name, err)
		}
		seen := map[int]bool{}
		for _, r1c := range ccs.(interface{ GetR1Cs() []constraint.R1C }).GetR1Cs() {
			for _, le := range []constraint.LinearExpression{r1c.L, r1c.R, r1c.O} {
				for _, term := range le {
					seen[term.WireID()] = true
				}
			}
		}
		for w := 1; w < ccs.GetNbPublicVariables(); w++ {
			if !seen[w] {
				t.Errorf("%s: public wire %d appears in no constraint", name, w)
			}
		}
	}
	check("enygma", &enygma.EnygmaCircuit{Config: enygma.EnygmaCircuitConfig{NCommitment: k},
		FingerPrintofSharedSecrets: fp(), PublicKey: make([]frontend.Variable, k),
		PreviousCommit: make([][2]frontend.Variable, k), TxCommit: make([][2]frontend.Variable, k),
		AnonymitySet: make([]frontend.Variable, k), SharedSecrets: make([]frontend.Variable, k),
		MessageTags: make([]frontend.Variable, k), TxValues: make([]frontend.Variable, k),
		TxRandomValues: make([]frontend.Variable, k)})
	check("usdr", &usdr.USDrCircuit{Config: usdr.USDrCircuitConfig{NCommitment: k},
		FingerPrintofSharedSecrets: fp(), PublicKey: make([]frontend.Variable, k),
		PreviousCommit: make([][2]frontend.Variable, k), TxCommit: make([][2]frontend.Variable, k),
		AnonymitySet: make([]frontend.Variable, k), SharedSecrets: make([]frontend.Variable, k),
		MessageTags: make([]frontend.Variable, k), TxValues: make([]frontend.Variable, k),
		TxRandomValues: make([]frontend.Variable, k)})
	check("burn", &burn.BurnCircuit{})
}

// DomainId (Fix L-01) is constrained only by AssertIsEqual(x, x). This pins
// that Groth16 still binds such an input: a proof made for one value must not
// verify for another, or the cross-deployment replay defence is void.
type selfEqCircuit struct {
	X frontend.Variable `gnark:",public"`
	D frontend.Variable `gnark:",public"`
	Y frontend.Variable
}

func (c *selfEqCircuit) Define(api frontend.API) error {
	api.AssertIsEqual(api.Mul(c.Y, c.Y), c.X)
	api.AssertIsEqual(c.D, c.D)
	return nil
}

func TestSelfEqualityPublicInputIsBoundByGroth16(t *testing.T) {
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &selfEqCircuit{})
	if err != nil {
		t.Fatal(err)
	}
	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		t.Fatal(err)
	}
	f := ecc.BN254.ScalarField()
	full, _ := frontend.NewWitness(&selfEqCircuit{X: 9, D: 1, Y: 3}, f)
	proof, err := groth16.Prove(ccs, pk, full)
	if err != nil {
		t.Fatal(err)
	}
	pub, _ := full.Public()
	if err := groth16.Verify(proof, vk, pub); err != nil {
		t.Fatalf("honest proof rejected: %v", err)
	}
	other, _ := frontend.NewWitness(&selfEqCircuit{X: 9, D: 2, Y: 3}, f)
	otherPub, _ := other.Public()
	if err := groth16.Verify(proof, vk, otherPub); err == nil {
		t.Fatal("a proof verified against a different value of an AssertIsEqual(x,x) public input: DomainId would be malleable")
	}
}

// ── native helpers ──────────────────────────────────────────────────────────

func ph(in ...*big.Int) *big.Int {
	h, err := iden3poseidon.Hash(in)
	if err != nil {
		panic(err)
	}
	return h
}

func modP(x *big.Int) *big.Int { return new(big.Int).Mod(x, utils.P) }

func com(v, r *big.Int) *babyjub.Point {
	vG := babyjub.NewPoint().Mul(v, utils.CircuitGBabyJub)
	rH := babyjub.NewPoint().Mul(r, utils.HBabyJub)
	return babyjub.NewPoint().Projective().Add(vG.Projective(), rH.Projective()).Affine()
}

func bi(n int64) *big.Int { return big.NewInt(n) }

const k = 6

// tx describes one transfer. values[i] is the credit to slot i (the sender's
// entry is ignored and derived). domainTag/randTag select the main (12, 21) or
// USDr (120, 210) domain-separation constants.
type tx struct {
	labels             [k]*big.Int
	sender             int
	sk, prevBal, prevR *big.Int
	block              *big.Int
	values             [k]*big.Int
	spend              *big.Int
	domainTag, randTag int64
	claimedPrevBal     *big.Int // what the prover claims as PreviousSenderBalance (defaults to prevBal)
	recipient          int      // USDr only: the slot whose key is the FeeRecipientKey
}

func defaultTx() tx {
	t := tx{sender: 0, sk: bi(424242), prevBal: bi(500), prevR: bi(31337), block: bi(60), spend: bi(100),
		domainTag: 12, randTag: 21, recipient: 5}
	for i := 0; i < k; i++ {
		t.labels[i] = bi(int64(i))
		t.values[i] = bi(0)
	}
	t.values[1], t.values[2] = bi(60), bi(40)
	return t
}

// built holds every value both circuits need.
type built struct {
	fp                   [k][k]*big.Int
	pk                   [k]*big.Int
	prevCommit, txCommit [k]*babyjub.Point
	tags                 [k]*big.Int
	nullifier            *big.Int
	shared               [k]*big.Int
	txValues, txRand     [k]*big.Int
	recipientKey         *big.Int // USDr FeeRecipientKey
	t                    tx
}

func build(t tx) built {
	b := built{t: t}
	sk := t.sk
	// The circuits hash the blinding factor REDUCED mod P (nullifier canonicality).
	secretRemain := modP(ph(modP(t.prevR), sk))
	for i := 0; i < k; i++ {
		b.shared[i] = bi(int64(5000 + i))
		skI := bi(int64(1000 + i))
		b.pk[i] = modP(ph(skI, skI))
		b.prevCommit[i] = com(bi(int64(10*i+1)), bi(int64(77+i)))
	}
	b.pk[t.sender] = modP(ph(sk, sk))
	b.recipientKey = b.pk[t.recipient]
	b.shared[t.sender] = secretRemain
	b.prevCommit[t.sender] = com(t.prevBal, t.prevR)
	for i := 0; i < k; i++ {
		for j := 0; j < k; j++ {
			b.fp[i][j] = bi(0)
		}
	}
	for i := 0; i < k; i++ {
		if i != t.sender {
			b.fp[i][t.sender] = modP(ph(b.shared[i]))
		}
	}
	b.nullifier = ph(secretRemain, t.block)

	hashTag := ph(bi(t.domainTag))
	hashRand := ph(bi(t.randTag))
	senderID := t.labels[t.sender]
	rf := [k]*big.Int{}
	sumRecv := new(big.Int)
	for i := 0; i < k; i++ {
		dir := ph(senderID, t.labels[i])
		nonce := ph(b.nullifier, dir)
		b.tags[i] = modP(ph(hashTag, b.shared[i], nonce))
		rf[i] = modP(ph(hashRand, b.shared[i], nonce))
		if i != t.sender {
			sumRecv.Add(sumRecv, rf[i])
		}
	}
	for i := 0; i < k; i++ {
		if i == t.sender {
			b.txValues[i] = modP(new(big.Int).Sub(utils.P, t.spend)) // (P - v) mod P: 0 when v == 0
			b.txRand[i] = modP(sumRecv)
		} else {
			b.txValues[i] = new(big.Int).Set(t.values[i])
			b.txRand[i] = new(big.Int).Sub(utils.P, rf[i])
		}
		b.txCommit[i] = com(b.txValues[i], b.txRand[i])
	}
	return b
}

func (b *built) recommit() {
	for i := 0; i < k; i++ {
		b.txCommit[i] = com(b.txValues[i], b.txRand[i])
	}
}

func vars2(p [k]*babyjub.Point) [][2]frontend.Variable {
	o := make([][2]frontend.Variable, k)
	for i := range o {
		o[i] = [2]frontend.Variable{p[i].X, p[i].Y}
	}
	return o
}

func vec(v [k]*big.Int) []frontend.Variable {
	o := make([]frontend.Variable, k)
	for i := range o {
		o[i] = v[i]
	}
	return o
}

func (b built) labelsVec() []frontend.Variable { return vec(b.t.labels) }

func (b built) fpMatrix() [][]frontend.Variable {
	m := make([][]frontend.Variable, k)
	for i := range m {
		m[i] = vec(b.fp[i])
	}
	return m
}

func (b built) claimedPrev() *big.Int {
	if b.t.claimedPrevBal != nil {
		return b.t.claimedPrevBal
	}
	return b.t.prevBal
}

func (b built) mainWitness() *enygma.EnygmaCircuit {
	return &enygma.EnygmaCircuit{
		Config:                     enygma.EnygmaCircuitConfig{NCommitment: k},
		FingerPrintofSharedSecrets: b.fpMatrix(), PublicKey: vec(b.pk),
		PreviousCommit: vars2(b.prevCommit), TxCommit: vars2(b.txCommit),
		BlockNumber: b.t.block, AnonymitySet: b.labelsVec(), MessageTags: vec(b.tags),
		Nullifier: b.nullifier, DomainId: bi(1337),
		SenderId: b.t.labels[b.t.sender], SharedSecrets: vec(b.shared), SecretKey: b.t.sk,
		PreviousSenderBalance: b.claimedPrev(), PreviousSenderRandomValue: b.t.prevR,
		TxValues: vec(b.txValues), TxRandomValues: vec(b.txRand), SenderTxValue: b.t.spend,
	}
}

func mainTemplate() *enygma.EnygmaCircuit {
	fp := make([][]frontend.Variable, k)
	for i := range fp {
		fp[i] = make([]frontend.Variable, k)
	}
	return &enygma.EnygmaCircuit{Config: enygma.EnygmaCircuitConfig{NCommitment: k},
		FingerPrintofSharedSecrets: fp, PublicKey: make([]frontend.Variable, k),
		PreviousCommit: make([][2]frontend.Variable, k), TxCommit: make([][2]frontend.Variable, k),
		AnonymitySet: make([]frontend.Variable, k), SharedSecrets: make([]frontend.Variable, k),
		MessageTags: make([]frontend.Variable, k), TxValues: make([]frontend.Variable, k),
		TxRandomValues: make([]frontend.Variable, k)}
}

func (b built) usdrWitness() *usdr.USDrCircuit {
	return &usdr.USDrCircuit{
		Config:                     usdr.USDrCircuitConfig{NCommitment: k},
		FingerPrintofSharedSecrets: b.fpMatrix(), PublicKey: vec(b.pk),
		PreviousCommit: vars2(b.prevCommit), TxCommit: vars2(b.txCommit),
		BlockNumber: b.t.block, AnonymitySet: b.labelsVec(), MessageTags: vec(b.tags),
		Nullifier: b.nullifier, FeeAmount: b.t.spend, DomainId: bi(1337), FeeRecipientKey: b.recipientKey,
		SenderId: b.t.labels[b.t.sender], SharedSecrets: vec(b.shared), SecretKey: b.t.sk,
		PreviousSenderBalance: b.claimedPrev(), PreviousSenderRandomValue: b.t.prevR,
		TxValues: vec(b.txValues), TxRandomValues: vec(b.txRand),
	}
}

func usdrTemplate() *usdr.USDrCircuit {
	fp := make([][]frontend.Variable, k)
	for i := range fp {
		fp[i] = make([]frontend.Variable, k)
	}
	return &usdr.USDrCircuit{Config: usdr.USDrCircuitConfig{NCommitment: k},
		FingerPrintofSharedSecrets: fp, PublicKey: make([]frontend.Variable, k),
		PreviousCommit: make([][2]frontend.Variable, k), TxCommit: make([][2]frontend.Variable, k),
		AnonymitySet: make([]frontend.Variable, k), SharedSecrets: make([]frontend.Variable, k),
		MessageTags: make([]frontend.Variable, k), TxValues: make([]frontend.Variable, k),
		TxRandomValues: make([]frontend.Variable, k)}
}

// ── adversarial matrix ──────────────────────────────────────────────────────

type attack struct {
	name string
	// mutateTx edits the transfer description before witness construction.
	mutateTx func(*tx)
	// mutateBuilt edits derived values after construction.
	mutateBuilt func(*built)
	// wantSolved is the expectation for the main circuit; usdrSolved, when
	// set, overrides it for the USDr circuit (which additionally pins the fee
	// recipient and split).
	wantSolved bool
	usdrSolved *bool
}

func yes() *bool { v := true; return &v }
func no() *bool  { v := false; return &v }

func attacks() []attack {
	return []attack{
		{name: "honest transfer", wantSolved: true},
		{name: "honest zero-value transfer", wantSolved: true,
			mutateTx: func(t *tx) {
				t.spend = bi(0)
				for i := range t.values {
					t.values[i] = bi(0)
				}
			}},
		{name: "spend more than the balance",
			mutateTx: func(t *tx) { t.prevBal = new(big.Int).Div(t.spend, bi(2)) }},
		{name: "claim balance+P to hide an overspend (C-02)",
			mutateTx: func(t *tx) {
				t.prevBal = new(big.Int).Div(t.spend, bi(2))
				t.claimedPrevBal = new(big.Int).Add(t.prevBal, utils.P)
			}},
		{name: "hidden debit in a non-sender slot (C-03)",
			mutateTx: func(t *tx) { t.spend = bi(0) },
			mutateBuilt: func(b *built) {
				b.txValues[b.t.sender] = bi(0)
				b.txValues[1] = bi(400)
				b.txValues[2] = new(big.Int).Sub(utils.P, bi(400))
				b.recommit()
			}},
		{name: "non-sender credits exceed the declared spend",
			mutateBuilt: func(b *built) { b.txValues[1] = new(big.Int).Add(b.txValues[1], bi(1)); b.recommit() }},
		{name: "forged message tag",
			mutateBuilt: func(b *built) { b.tags[1] = new(big.Int).Add(b.tags[1], bi(1)) }},
		{name: "wrong secret key",
			mutateBuilt: func(b *built) { b.t.sk = bi(1) }},
		{name: "receiver blinding factor tampered (recommitted consistently)",
			mutateBuilt: func(b *built) { b.txRand[1] = new(big.Int).Add(b.txRand[1], bi(1)); b.recommit() }},
		{name: "sender label duplicated",
			mutateTx: func(t *tx) { t.labels[2] = new(big.Int).Set(t.labels[t.sender]) }},
		{name: "nullifier not derived from the state",
			mutateBuilt: func(b *built) { b.nullifier = new(big.Int).Add(b.nullifier, bi(1)) }},
		{name: "previous commitment does not open to the claimed balance",
			mutateBuilt: func(b *built) { b.t.claimedPrevBal = new(big.Int).Sub(b.t.prevBal, bi(1)) }},
		{name: "wrong fingerprint in the sender column",
			mutateBuilt: func(b *built) { b.fp[1][b.t.sender] = new(big.Int).Add(b.fp[1][b.t.sender], bi(1)) }},
		{name: "nullifier hashed from a lifted (unreduced) blinding factor",
			mutateBuilt: func(b *built) {
				// the pre-fix formula: Poseidon(prevR + P, sk), which the circuit no longer computes
				lifted := new(big.Int).Add(b.t.prevR, utils.P)
				legacy := modP(ph(lifted, b.t.sk))
				b.nullifier = ph(legacy, b.t.block)
			}},

		// Who receives the credits, and in what split.
		// The main circuit only requires the non-sender credits to add up to the
		// spend. The USDr circuit pins the whole fee to the FeeRecipientKey slot.
		{name: "spend split unevenly across two non-sender slots", wantSolved: true, usdrSolved: no(),
			mutateTx: func(t *tx) {
				t.values[1], t.values[2], t.values[5] = new(big.Int).Sub(t.spend, bi(0)), bi(0), bi(0)
				if t.spend.Sign() > 0 {
					t.values[1] = bi(1)
					t.values[2] = new(big.Int).Sub(t.spend, bi(1))
				}
			}},
		{name: "entire spend credited to one slot that is not the fee recipient", wantSolved: true, usdrSolved: no(),
			mutateTx: func(t *tx) {
				for i := range t.values {
					t.values[i] = bi(0)
				}
				t.values[1] = new(big.Int).Set(t.spend)
			}},
		{name: "fee split between the recipient and another slot", wantSolved: true, usdrSolved: no(),
			mutateTx: func(t *tx) {
				for i := range t.values {
					t.values[i] = bi(0)
				}
				t.values[t.recipient] = new(big.Int).Sub(t.spend, bi(1))
				t.values[1] = bi(1)
			}},

		// Accepted by the circuit; must be caught (or are harmless) elsewhere.
		{name: "arbitrary anonymity-set labels", wantSolved: true,
			mutateTx: func(t *tx) {
				for i := 0; i < k; i++ {
					t.labels[i] = bi(int64(9000 + 7*i))
				}
			}},
		{name: "garbage in fingerprint cells outside the sender column", wantSolved: true,
			mutateBuilt: func(b *built) { b.fp[2][3] = bi(123456789) }},
	}
}

// usdrDefaults makes the default transfer a valid USDr fee payment: a fee of
// 10 credited entirely to the recipient slot (5).
func usdrDefaults(d *tx) {
	d.domainTag, d.randTag = 120, 210
	d.spend = bi(10)
	for i := range d.values {
		d.values[i] = bi(0)
	}
	d.values[d.recipient] = bi(10)
}

func runMatrix(t *testing.T, name string, isUsdr bool, tmpl func() frontend.Circuit, witness func(built) frontend.Circuit) {
	solver.RegisterHint(utils.ModHint)
	f := ecc.BN254.ScalarField()
	for _, a := range attacks() {
		a := a
		t.Run(name+"/"+a.name, func(t *testing.T) {
			d := defaultTx()
			if isUsdr {
				usdrDefaults(&d)
			}
			if a.mutateTx != nil {
				a.mutateTx(&d)
			}
			b := build(d)
			if a.mutateBuilt != nil {
				a.mutateBuilt(&b)
			}
			want := a.wantSolved
			if isUsdr && a.usdrSolved != nil {
				want = *a.usdrSolved
			}
			err := test.IsSolved(tmpl(), witness(b), f)
			if want && err != nil {
				t.Fatalf("expected the circuit to accept this, but it was rejected: %v", err)
			}
			if !want && err == nil {
				t.Fatalf("SOUNDNESS: the circuit accepted %q", a.name)
			}
		})
	}
}

func TestEnygmaCircuitAdversarial(t *testing.T) {
	runMatrix(t, "enygma", false,
		func() frontend.Circuit { return mainTemplate() },
		func(b built) frontend.Circuit { return b.mainWitness() })
}

func TestUsdrCircuitAdversarial(t *testing.T) {
	runMatrix(t, "usdr", true,
		func() frontend.Circuit { return usdrTemplate() },
		func(b built) frontend.Circuit { return b.usdrWitness() })
}

// The fee-recipient binding, tested directly on the USDr circuit.
func TestUsdrFeeRecipientBinding(t *testing.T) {
	solver.RegisterHint(utils.ModHint)
	f := ecc.BN254.ScalarField()
	cases := []struct {
		name   string
		mutate func(*built)
		solved bool
	}{
		{"fee paid in full to the recipient's slot", func(*built) {}, true},
		{"recipient key is the SENDER's own key (a bank cannot relay itself)", func(b *built) {
			b.recipientKey = b.pk[b.t.sender]
		}, false},
		{"recipient key belongs to no participant", func(b *built) {
			b.recipientKey = bi(777)
		}, false},
		{"two slots carry the recipient's key", func(b *built) {
			b.pk[1] = b.recipientKey
		}, false},
		{"recipient key names a different participant than the one credited", func(b *built) {
			b.recipientKey = b.pk[3]
		}, false},
		{"recipient credited nothing (fee paid to nobody)", func(b *built) {
			b.txValues[b.t.recipient] = bi(0)
			b.txValues[3] = bi(10)
			b.recommit()
		}, false},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			d := defaultTx()
			usdrDefaults(&d)
			b := build(d)
			c.mutate(&b)
			err := test.IsSolved(usdrTemplate(), b.usdrWitness(), f)
			if c.solved && err != nil {
				t.Fatalf("expected accepted, got: %v", err)
			}
			if !c.solved && err == nil {
				t.Fatalf("SOUNDNESS: the USDr circuit accepted %q", c.name)
			}
		})
	}
}

// The nullifier hashes PreviousSenderRandomValue REDUCED mod P. Every
// representative r + kP (k = 0..7, below Fr) of the same blinding factor
// opens the same on-chain commitment and must now yield the SAME nullifier;
// before the fix each yielded a different one.
func TestNullifierIsCanonicalAcrossBlindingLifts(t *testing.T) {
	solver.RegisterHint(utils.ModHint)
	f := ecc.BN254.ScalarField()
	base := defaultTx()
	b0 := build(base)
	if err := test.IsSolved(mainTemplate(), b0.mainWitness(), f); err != nil {
		t.Fatalf("honest witness rejected: %v", err)
	}
	accepted := 0
	for j := int64(1); j <= 8; j++ {
		lifted := base
		lifted.prevR = new(big.Int).Add(base.prevR, new(big.Int).Mul(bi(j), utils.P))
		if lifted.prevR.Cmp(f) >= 0 {
			break
		}
		b := build(lifted)
		b.prevCommit[base.sender] = b0.prevCommit[base.sender] // same on-chain commitment
		if err := test.IsSolved(mainTemplate(), b.mainWitness(), f); err != nil {
			t.Fatalf("lift %d of the blinding factor was rejected: %v", j, err)
		}
		if b.nullifier.Cmp(b0.nullifier) != 0 {
			t.Fatalf("lift %d produced a different nullifier: the nullifier is not canonical", j)
		}
		accepted++
	}
	t.Logf("%d lifts of the blinding factor: all accepted, all with the canonical nullifier", accepted)
	if accepted == 0 {
		t.Fatal("no lift was exercised")
	}
}

// Burn keeps the account's blinding factor unchanged (its total-supply
// accounting requires that), so its nullifier -- a function of blinding,
// secret key and epoch -- repeats for every burn of an account within one
// epoch. The contract consumes each nullifier once, so a second burn in the
// same epoch reverts NullifierAlreadyUsed. Before the canonicality fix a
// holder could dodge that by re-proving with a lifted blinding factor; that
// is no longer possible, so the limit is now firm: one burn per account per
// epoch.
func TestBurnNullifierRepeatsWithinAnEpoch(t *testing.T) {
	solver.RegisterHint(utils.ModHint)
	f := ecc.BN254.ScalarField()
	sk, pR, block := bi(12345), bi(777777), bi(60)

	mk := func(pB, amount, blinding *big.Int) *burn.BurnCircuit {
		pk := modP(ph(sk, sk))
		prev := com(pB, pR)
		next := com(new(big.Int).Sub(pB, amount), pR)
		return &burn.BurnCircuit{
			PublicKey: pk, PreviousCommit: [2]frontend.Variable{prev.X, prev.Y},
			NewCommit: [2]frontend.Variable{next.X, next.Y}, Amount: amount, BlockNumber: block,
			Nullifier: ph(modP(ph(modP(pR), sk)), block), DomainId: bi(1),
			SecretKey: sk, PreviousBalance: pB, PreviousRandomValue: blinding,
		}
	}
	first, second := mk(bi(100), bi(10), pR), mk(bi(90), bi(5), pR)
	for name, w := range map[string]*burn.BurnCircuit{"first": first, "second": second} {
		if err := test.IsSolved(&burn.BurnCircuit{}, w, f); err != nil {
			t.Fatalf("%s burn rejected: %v", name, err)
		}
	}
	if first.Nullifier.(*big.Int).Cmp(second.Nullifier.(*big.Int)) != 0 {
		t.Fatal("consecutive burns produce different nullifiers; the liveness note in the docs is stale")
	}

	// A lifted blinding factor opens the same commitments but no longer
	// yields a different nullifier.
	lifted := new(big.Int).Add(pR, utils.P)
	dodge := mk(bi(90), bi(5), lifted)
	if err := test.IsSolved(&burn.BurnCircuit{}, dodge, f); err != nil {
		t.Fatalf("a lifted blinding factor should still open the commitment: %v", err)
	}
	if dodge.Nullifier.(*big.Int).Cmp(first.Nullifier.(*big.Int)) != 0 {
		t.Fatal("the lifted witness carries a different nullifier: the dodge still works")
	}
	legacy := mk(bi(90), bi(5), lifted)
	legacy.Nullifier = ph(modP(ph(lifted, sk)), block) // pre-fix formula
	if err := test.IsSolved(&burn.BurnCircuit{}, legacy, f); err == nil {
		t.Fatal("SOUNDNESS: a nullifier hashed from the unreduced blinding factor was accepted")
	}
}

func TestBurnCircuitAdversarial(t *testing.T) {
	solver.RegisterHint(utils.ModHint)
	f := ecc.BN254.ScalarField()
	sk, pB, pR, block := bi(12345), bi(100), bi(777777), bi(60)
	honest := func() *burn.BurnCircuit {
		pk := modP(ph(sk, sk))
		prev := com(pB, pR)
		next := com(bi(90), pR)
		return &burn.BurnCircuit{
			PublicKey: pk, PreviousCommit: [2]frontend.Variable{prev.X, prev.Y},
			NewCommit: [2]frontend.Variable{next.X, next.Y}, Amount: bi(10), BlockNumber: block,
			Nullifier: ph(modP(ph(modP(pR), sk)), block), DomainId: bi(1),
			SecretKey: sk, PreviousBalance: pB, PreviousRandomValue: pR,
		}
	}
	cases := []struct {
		name   string
		mutate func(*burn.BurnCircuit)
		solved bool
	}{
		{"honest burn", func(*burn.BurnCircuit) {}, true},
		{"burn more than the balance", func(w *burn.BurnCircuit) {
			w.Amount = bi(101)
			n := com(new(big.Int).Sub(bi(100), bi(101)), pR) // negative balance, mod P
			w.NewCommit = [2]frontend.Variable{n.X, n.Y}
		}, false},
		{"amount aliased by +P", func(w *burn.BurnCircuit) {
			w.Amount = new(big.Int).Add(bi(10), utils.P)
		}, false},
		{"new commitment not equal to previous minus amount", func(w *burn.BurnCircuit) {
			n := com(bi(50), pR)
			w.NewCommit = [2]frontend.Variable{n.X, n.Y}
		}, false},
		{"new commitment re-blinded", func(w *burn.BurnCircuit) {
			n := com(bi(90), bi(1))
			w.NewCommit = [2]frontend.Variable{n.X, n.Y}
		}, false},
		{"wrong secret key", func(w *burn.BurnCircuit) { w.SecretKey = bi(1) }, false},
		{"public key of another account", func(w *burn.BurnCircuit) { w.PublicKey = bi(5) }, false},
		{"previous commitment of another balance", func(w *burn.BurnCircuit) {
			p := com(bi(999), pR)
			w.PreviousCommit = [2]frontend.Variable{p.X, p.Y}
		}, false},
		{"balance aliased by +P", func(w *burn.BurnCircuit) {
			w.PreviousBalance = new(big.Int).Add(pB, utils.P)
		}, false},
		{"stale nullifier", func(w *burn.BurnCircuit) { w.Nullifier = bi(1) }, false},
		{"zero-amount burn (state unchanged)", func(w *burn.BurnCircuit) {
			w.Amount = bi(0)
			n := com(pB, pR)
			w.NewCommit = [2]frontend.Variable{n.X, n.Y}
		}, true},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			w := honest()
			c.mutate(w)
			err := test.IsSolved(&burn.BurnCircuit{}, w, f)
			if c.solved && err != nil {
				t.Fatalf("expected accepted, got: %v", err)
			}
			if !c.solved && err == nil {
				t.Fatalf("SOUNDNESS: the burn circuit accepted %q", c.name)
			}
		})
	}
}
