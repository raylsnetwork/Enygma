package withdraw

// TestM03_* reproduce and verify the M-03 fix
// (ENYGMA_PAYMENTS_AUDIT_2026-08-22.md): handler.go never assigned
// witness.BlockNumber, witness.Nullifier, witness.PreviousSenderBalance,
// or witness.PreviousSenderRandomValue — despite WithdrawRequest carrying
// all four as required JSON fields and the circuit needing every one of
// them (BlockNumber/Nullifier are public signals fed straight into
// on-chain checks; PreviousSenderBalance/PreviousSenderRandomValue are the
// private opening of the sender's own commitment, load-bearing for the
// solvency comparator, C-08's fix, and the Pedersen equality that binds
// the whole witness together). Every real HTTP request therefore built an
// incomplete witness struct with those four fields left at their Go zero
// value (nil frontend.Variable), and frontend.NewWitness either rejected
// or panicked on it — the withdraw circuit's prover route was completely
// unreachable, independent of anything else about the request.
//
// Unlike C-08/M-15/C-09 (this package's other repro tests), M-03 isn't a
// circuit-logic bug — the fixed circuit's own Define() was already
// correct — so this is the first test in this package to build a FULLY
// VALID witness for the real WithdrawEnygmaCircuit and drive it through
// the real HTTP handler (mirroring what go_client's
// tagMessageGen/genCommitmentAndRandom — the "legacy",
// pre-nullifier-folding formula deposit/withdraw/enygma_fee still use —
// compute), rather than a minimal standalone mirror circuit. That's
// deliberate: the whole point of this fix is that a real, complete
// request now reaches a satisfiable witness — and a real proof — at all.
//
// Run:
//
//	CC=/usr/bin/clang go test ./pkg/circuits/withdraw/... -run TestM03 -v

import (
	"bytes"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	utils "enygma-server/utils"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/gin-gonic/gin"
	"github.com/iden3/go-iden3-crypto/babyjub"
	iden3poseidon "github.com/iden3/go-iden3-crypto/poseidon"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func m03PedersenCommit(v, r *big.Int) *babyjub.Point {
	vG := babyjub.NewPoint().Mul(v, utils.CircuitGBabyJub)
	rH := babyjub.NewPoint().Mul(r, utils.HBabyJub)
	return babyjub.NewPoint().Projective().Add(vG.Projective(), rH.Projective()).Affine()
}

func m03NegMod(x *big.Int) *big.Int {
	return new(big.Int).Sub(utils.P, new(big.Int).Mod(x, utils.P))
}

func m03ReduceModP(x *big.Int) *big.Int {
	return new(big.Int).Mod(x, utils.P)
}

// m03Values holds every raw field a complete, honest withdraw witness
// needs, computed once and consumed two ways below: as a real
// frontend.Variable witness (m03Values.circuit) and as the decimal-string
// JSON body a real HTTP client would send (m03Values.requestJSON) — both
// must describe the exact same witness, so building them from one shared
// set of values is what makes the HTTP-level test meaningful rather than
// accidentally re-deriving a different (and differently buggy) witness.
type m03Values struct {
	k         int
	senderIdx int

	hashedSecrets []*big.Int
	pks           []*big.Int
	prevCommit    [][2]*big.Int
	txCommit      [][2]*big.Int
	blockNumber   *big.Int
	anonymitySet  []*big.Int
	messageTags   []*big.Int
	nullifier     *big.Int
	senderId      *big.Int
	secrets       []*big.Int
	sk            *big.Int
	prevBalance   *big.Int
	prevR         *big.Int
	txValues      []*big.Int
	txRandom      []*big.Int
	withdrawAmt   *big.Int
	depositAddr   *big.Int
	hashes        [10]*big.Int
	skDeposits    [10]*big.Int
	vPerDeposit   [10]*big.Int
	domainId      *big.Int
}

// m03BuildValues mirrors circuit.go's Define() field-by-field for a single
// honest withdrawal: sender (slot 0 of a 6-slot anonymity set) debits
// withdrawAmt from a previousBalance opening of (previousBalance, prevR),
// routed to one deposit slot (of 10) worth exactly withdrawAmt — using the
// same "legacy" (pre-nullifier-folding) MessageTags/RandomFactor formula
// deposit/withdraw/enygma_fee all still use: Poseidon(HashTag/HashRandom,
// secret, BlockNumber) — see
// go_client/enygma_test/transaction_test.go's legacyTagMessageGen/
// legacyGenCommitmentAndRandom for the equivalent construction this
// mirrors.
func m03BuildValues(t *testing.T, sk, prevR, previousBalance, withdrawAmt, blockNumber *big.Int) m03Values {
	t.Helper()
	const k = 6
	const senderIdx = 0

	sks := make([]*big.Int, k)
	sks[senderIdx] = sk
	for i := 1; i < k; i++ {
		sks[i] = big.NewInt(int64(1000 + i))
	}

	pks := make([]*big.Int, k)
	for i := 0; i < k; i++ {
		h, err := iden3poseidon.Hash([]*big.Int{sks[i], sks[i]})
		if err != nil {
			t.Fatalf("pk hash %d: %v", i, err)
		}
		pks[i] = m03ReduceModP(h)
	}

	senderSecretRaw, err := iden3poseidon.Hash([]*big.Int{prevR, sk})
	if err != nil {
		t.Fatalf("sender secret: %v", err)
	}
	secrets := make([]*big.Int, k)
	secrets[senderIdx] = m03ReduceModP(senderSecretRaw)
	for i := 1; i < k; i++ {
		secrets[i] = big.NewInt(int64(2000 + i))
	}

	hashedSecrets := make([]*big.Int, k)
	for i := 0; i < k; i++ {
		h, err := iden3poseidon.Hash([]*big.Int{secrets[i], secrets[i]})
		if err != nil {
			t.Fatalf("hashed secret %d: %v", i, err)
		}
		hashedSecrets[i] = m03ReduceModP(h)
	}

	anonymitySet := make([]*big.Int, k)
	for i := 0; i < k; i++ {
		anonymitySet[i] = big.NewInt(int64(i + 1))
	}
	senderId := anonymitySet[senderIdx]

	hashTag, err := iden3poseidon.Hash([]*big.Int{big.NewInt(12)})
	if err != nil {
		t.Fatalf("HashTag: %v", err)
	}
	hashRandom, err := iden3poseidon.Hash([]*big.Int{big.NewInt(21)})
	if err != nil {
		t.Fatalf("HashRandom: %v", err)
	}

	messageTags := make([]*big.Int, k)
	for i := 0; i < k; i++ {
		h, err := iden3poseidon.Hash([]*big.Int{hashTag, secrets[i], blockNumber})
		if err != nil {
			t.Fatalf("message tag %d: %v", i, err)
		}
		messageTags[i] = m03ReduceModP(h)
	}

	rValues := make([]*big.Int, k)
	rSum := big.NewInt(0)
	for i := 0; i < k; i++ {
		h, err := iden3poseidon.Hash([]*big.Int{hashRandom, secrets[i], blockNumber})
		if err != nil {
			t.Fatalf("random factor %d: %v", i, err)
		}
		rValues[i] = m03ReduceModP(h)
		if i != senderIdx {
			rSum = m03ReduceModP(new(big.Int).Add(rSum, rValues[i]))
		}
	}

	txValues := make([]*big.Int, k)
	txRandom := make([]*big.Int, k)
	txCommit := make([][2]*big.Int, k)
	for i := 0; i < k; i++ {
		if i == senderIdx {
			txValues[i] = m03NegMod(withdrawAmt)
			txRandom[i] = rSum
		} else {
			txValues[i] = big.NewInt(0)
			txRandom[i] = m03NegMod(rValues[i])
		}
		pt := m03PedersenCommit(txValues[i], txRandom[i])
		txCommit[i] = [2]*big.Int{pt.X, pt.Y}
	}

	prevCommit := make([][2]*big.Int, k)
	for i := 0; i < k; i++ {
		if i == senderIdx {
			pt := m03PedersenCommit(previousBalance, prevR)
			prevCommit[i] = [2]*big.Int{pt.X, pt.Y}
		} else {
			pt := m03PedersenCommit(big.NewInt(0), big.NewInt(int64(3000+i)))
			prevCommit[i] = [2]*big.Int{pt.X, pt.Y}
		}
	}

	nullifierRaw, err := iden3poseidon.Hash([]*big.Int{hashedSecrets[senderIdx], blockNumber})
	if err != nil {
		t.Fatalf("nullifier: %v", err)
	}

	const depositAddr = 777
	const depositSk = 555
	var hashes, skDeposits, vPerDeposit [10]*big.Int
	for i := 0; i < 10; i++ {
		hashes[i] = big.NewInt(0)
		skDeposits[i] = big.NewInt(0)
		vPerDeposit[i] = big.NewInt(0)
	}
	skDeposits[0] = big.NewInt(depositSk)
	vPerDeposit[0] = new(big.Int).Set(withdrawAmt)
	firstHash, err := iden3poseidon.Hash([]*big.Int{big.NewInt(depositAddr), vPerDeposit[0]})
	if err != nil {
		t.Fatalf("deposit firstHash: %v", err)
	}
	pkFromSk, err := iden3poseidon.Hash([]*big.Int{skDeposits[0]})
	if err != nil {
		t.Fatalf("deposit pkFromSk: %v", err)
	}
	secondHash, err := iden3poseidon.Hash([]*big.Int{firstHash, pkFromSk})
	if err != nil {
		t.Fatalf("deposit secondHash: %v", err)
	}
	hashes[0] = secondHash

	return m03Values{
		k: k, senderIdx: senderIdx,
		hashedSecrets: hashedSecrets, pks: pks, prevCommit: prevCommit, txCommit: txCommit,
		blockNumber: blockNumber, anonymitySet: anonymitySet, messageTags: messageTags,
		nullifier: nullifierRaw, senderId: senderId, secrets: secrets, sk: sk,
		prevBalance: previousBalance, prevR: prevR, txValues: txValues, txRandom: txRandom,
		withdrawAmt: withdrawAmt, depositAddr: big.NewInt(depositAddr),
		hashes: hashes, skDeposits: skDeposits, vPerDeposit: vPerDeposit,
		domainId: big.NewInt(1), // Fix L-01: arbitrary fixed value, unconstrained by Define()
	}
}

func (v m03Values) circuit() *WithdrawEnygmaCircuit {
	w := &WithdrawEnygmaCircuit{
		Config:              WithdrawEnygmaCircuitConfig{NCommitment: v.k},
		HashedSharedSecrets: toVars(v.hashedSecrets),
		PublicKey:           toVars(v.pks),
		PreviousCommit:      toVarPairs(v.prevCommit),
		TxCommit:            toVarPairs(v.txCommit),
		BlockNumber:         v.blockNumber,
		AnonymitySet:        toVars(v.anonymitySet),
		MessageTags:         toVars(v.messageTags),
		Nullifier:           v.nullifier,
		TotalDepositValue:   new(big.Int).Set(v.withdrawAmt),
		DomainId:            v.domainId,

		SenderId:                  v.senderId,
		SharedSecrets:             toVars(v.secrets),
		SecretKey:                 v.sk,
		PreviousSenderBalance:     v.prevBalance,
		PreviousSenderRandomValue: v.prevR,
		TxValues:                  toVars(v.txValues),
		TxRandomValues:            toVars(v.txRandom),
		SenderTxValue:             new(big.Int).Set(v.withdrawAmt),
		Address:                   v.depositAddr,
	}
	hashesVars := toVars(v.hashes[:])
	skDepositsVars := toVars(v.skDeposits[:])
	vPerDepositVars := toVars(v.vPerDeposit[:])
	copy(w.Hashes[:], hashesVars)
	copy(w.SkDeposits[:], skDepositsVars)
	copy(w.VPerDeposit[:], vPerDepositVars)
	return w
}

// requestJSON converts v into the exact map[string]interface{} a real
// client would POST to /proof/withdraw/6 — decimal strings throughout,
// matching WithdrawRequest's json tags precisely.
func (v m03Values) requestJSON() map[string]interface{} {
	strs := func(vals []*big.Int) []string {
		out := make([]string, len(vals))
		for i, x := range vals {
			out[i] = x.String()
		}
		return out
	}
	pairs := func(vals [][2]*big.Int) [][2]string {
		out := make([][2]string, len(vals))
		for i, p := range vals {
			out[i] = [2]string{p[0].String(), p[1].String()}
		}
		return out
	}
	arr10 := func(vals [10]*big.Int) [10]string {
		var out [10]string
		for i, x := range vals {
			out[i] = x.String()
		}
		return out
	}
	return map[string]interface{}{
		"hashed_shared_secrets":        strs(v.hashedSecrets),
		"public_keys":                  strs(v.pks),
		"previous_commits":             pairs(v.prevCommit),
		"tx_commits":                   pairs(v.txCommit),
		"block_number":                 v.blockNumber.String(),
		"anonymity_set":                strs(v.anonymitySet),
		"message_tags":                 strs(v.messageTags),
		"nullifier":                    v.nullifier.String(),
		"sender_id":                    v.senderId.String(),
		"shared_secrets":               strs(v.secrets),
		"secret_key":                   v.sk.String(),
		"previous_sender_balance":      v.prevBalance.String(),
		"previous_sender_random_value": v.prevR.String(),
		"tx_values":                    strs(v.txValues),
		"tx_random_values":             strs(v.txRandom),
		"sender_tx_value":              v.withdrawAmt.String(),
		"hashes":                       arr10(v.hashes),
		"sk_deposits":                  arr10(v.skDeposits),
		"v_per_deposit":                arr10(v.vPerDeposit),
		"address":                      v.depositAddr.String(),
		"domain_id":                    v.domainId.String(),
	}
}

func toVars(vals []*big.Int) []frontend.Variable {
	out := make([]frontend.Variable, len(vals))
	for i, v := range vals {
		out[i] = v
	}
	return out
}

func toVarPairs(pairs [][2]*big.Int) [][2]frontend.Variable {
	out := make([][2]frontend.Variable, len(pairs))
	for i, p := range pairs {
		out[i] = [2]frontend.Variable{p[0], p[1]}
	}
	return out
}

func m03Solve(t *testing.T, w *WithdrawEnygmaCircuit) error {
	t.Helper()
	solver.RegisterHint(utils.ModHint)
	template := createWithdrawCircuitTemplate(WithdrawEnygmaCircuitConfig{NCommitment: 6})
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	witness, err := frontend.NewWitness(w, ecc.BN254.ScalarField())
	if err != nil {
		// This is the exact M-03 failure mode: pre-fix, BlockNumber/
		// Nullifier/PreviousSenderBalance/PreviousSenderRandomValue were
		// nil frontend.Variable fields (never assigned by handler.go),
		// and frontend.NewWitness rejected the incomplete struct before a
		// proof could ever be attempted.
		t.Fatalf("build witness (this is the M-03 failure mode if it fires): %v", err)
	}
	_, err = ccs.Solve(witness)
	return err
}

// TestM03_HonestWithdrawWitnessSolves is the core M-03 regression at the
// witness level: a complete, honest withdraw witness — every field
// handler.go now populates, including the four it used to skip — builds
// successfully AND satisfies the real circuit. Pre-fix this never got far
// enough to even attempt Solve: the witness itself was incomplete.
func TestM03_HonestWithdrawWitnessSolves(t *testing.T) {
	v := m03BuildValues(t, big.NewInt(424242), big.NewInt(67890), big.NewInt(500), big.NewInt(100), big.NewInt(3000))
	if err := m03Solve(t, v.circuit()); err != nil {
		t.Fatalf("FAIL (M-03 regressed, or a witness-construction bug): honest withdraw witness rejected: %v", err)
	}
	t.Log("complete withdraw witness (BlockNumber/Nullifier/PreviousSenderBalance/PreviousSenderRandomValue all populated) solves ✓")
}

// TestM03_MissingPreviousSenderBalanceFieldFailsToBuild reproduces the
// exact pre-fix symptom directly: a witness struct with
// PreviousSenderBalance left nil (Go's zero value for an unassigned
// interface field, exactly what handler.go used to leave it at) must fail
// at frontend.NewWitness — proving this is a real, previously-live crash
// path, not a hypothetical.
func TestM03_MissingPreviousSenderBalanceFieldFailsToBuild(t *testing.T) {
	v := m03BuildValues(t, big.NewInt(424242), big.NewInt(67890), big.NewInt(500), big.NewInt(100), big.NewInt(3000))
	w := v.circuit()
	w.PreviousSenderBalance = nil // the exact pre-fix bug: handler.go never set this
	solver.RegisterHint(utils.ModHint)
	_, err := frontend.NewWitness(w, ecc.BN254.ScalarField())
	if err == nil {
		t.Fatal("expected frontend.NewWitness to reject a witness with a nil PreviousSenderBalance field")
	}
	t.Logf("nil PreviousSenderBalance correctly rejected at witness construction (this is what every pre-fix withdraw request hit): %v", err)
}

// TestM03_HandlerProvesRealRequest drives the ACTUAL HTTP handler — the
// exact code this fix changed — with a complete, honest request body,
// through a real groth16.Prove. This is the strongest form of the
// regression: pre-fix, this exact request would have panicked/errored
// inside frontend.NewWitness before Prove was ever reached; post-fix it
// returns a genuine proof over a genuinely satisfied witness.
//
// Needs the real withdraw proving/verifying keys (gnark-server/keys/zkdvp/
// WithdrawPk6.key, WithdrawVk6.key) — present after
// `go run ./keygen/generate_keys.go -circuit withdraw` (or -circuit all).
func TestM03_HandlerProvesRealRequest(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	keysDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "keys", "zkdvp")
	pkPath := filepath.Join(keysDir, "WithdrawPk6.key")
	vkPath := filepath.Join(keysDir, "WithdrawVk6.key")
	if _, err := os.Stat(pkPath); err != nil {
		t.Skipf("withdraw proving key not found at %s — run: cd gnark-server && go run ./keygen/generate_keys.go -circuit withdraw", pkPath)
	}

	handler := NewHandler(pkPath, vkPath)

	v := m03BuildValues(t, big.NewInt(424242), big.NewInt(67890), big.NewInt(500), big.NewInt(100), big.NewInt(3000))
	body, err := json.Marshal(v.requestJSON())
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/proof/withdraw/6", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler(c)

	if w.Code != http.StatusOK {
		t.Fatalf("FAIL (M-03 regressed): handler returned %d, want 200: %s", w.Code, w.Body.String())
	}

	var resp WithdrawOutput
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Proof) != 8 {
		t.Errorf("proof: got %d elements, want 8", len(resp.Proof))
	}
	if len(resp.PublicSignal) != 52 {
		t.Errorf("publicSignal: got %d elements, want 52", len(resp.PublicSignal))
	}
	t.Logf("POST /proof/withdraw/6 returned a real proof (8 elements) and %d public signals — the withdraw circuit's HTTP prover route is unblocked ✓", len(resp.PublicSignal))
}
