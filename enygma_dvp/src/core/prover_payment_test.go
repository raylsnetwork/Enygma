package core

import (
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
)

// proveServer is a gnark server stub that answers path with the given
// public signals, whatever the request.
func proveServer(t *testing.T, path string, signals []int64) *GnarkClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			t.Errorf("request to %s, want %s", r.URL.Path, path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"proof":        []int64{1, 2, 3, 4, 5, 6, 7, 8},
			"publicSignal": signals,
		})
	}))
	t.Cleanup(srv.Close)
	return NewGnarkClient(srv.URL)
}

// paymentArgs are the inputs common to BoundPaymentProof and
// BoundPaymentFeeProof: one input note of 100 paid as 60 to a recipient and
// 40 back to the sender.
type paymentArgs struct {
	keys      []KeyPair
	encapKeys [][]byte
	proofs    []*MerkleProof
}

func newPaymentArgs(t *testing.T) paymentArgs {
	t.Helper()
	spend, err := NewSpendKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	view, err := NewViewKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	path := make([]*big.Int, 8)
	for i := range path {
		path[i] = big.NewInt(0)
	}
	return paymentArgs{
		keys:      []KeyPair{{PrivateKey: spend.PrivateKey, PublicKey: spend.PublicKey}},
		encapKeys: [][]byte{view.EncapsKey, view.EncapsKey},
		proofs:    []*MerkleProof{{Elements: path, Indices: big.NewInt(0), Root: big.NewInt(9)}},
	}
}

func assertStatement(t *testing.T, r *PaymentResult, want []int64) {
	t.Helper()
	got := r.ContractStatement()
	if len(got) != len(want) {
		t.Fatalf("statement %v, want %v", got, want)
	}
	for i := range want {
		if got[i].Cmp(big.NewInt(want[i])) != 0 {
			t.Errorf("statement[%d] = %s, want %d", i, got[i], want[i])
		}
	}
}

// The statement a Payment proof is submitted with is the one the server
// proved, as it returned it, not one rebuilt from the request.
func TestBoundPaymentProof_StatementIsTheServersPublicSignals(t *testing.T) {
	signals := []int64{100, 101, 102, 103, 104, 105, 106, 107}
	c := proveServer(t, "/proof/payment", signals)
	a := newPaymentArgs(t)
	r, err := c.BoundPaymentProof(big.NewInt(0xABCD), big.NewInt(0),
		[]*big.Int{big.NewInt(100)}, a.keys, []*big.Int{big.NewInt(1)},
		[]*big.Int{big.NewInt(60), big.NewInt(40)}, []*big.Int{big.NewInt(5), a.keys[0].PublicKey}, a.encapKeys,
		8, a.proofs, []*big.Int{big.NewInt(0)}, big.NewInt(7))
	if err != nil {
		t.Fatal(err)
	}
	assertStatement(t, r, signals)
}

func TestBoundPaymentFeeProof_StatementIsTheServersPublicSignals(t *testing.T) {
	signals := []int64{100, 101, 102, 103, 104, 105, 106, 107, 108}
	c := proveServer(t, "/proof/paymentFee", signals)
	a := newPaymentArgs(t)
	r, err := c.BoundPaymentFeeProof(big.NewInt(0xABCD), big.NewInt(10), big.NewInt(0),
		[]*big.Int{big.NewInt(100)}, a.keys, []*big.Int{big.NewInt(1)},
		[]*big.Int{big.NewInt(50), big.NewInt(40)}, []*big.Int{big.NewInt(5), a.keys[0].PublicKey}, a.encapKeys,
		8, a.proofs, []*big.Int{big.NewInt(0)}, big.NewInt(7))
	if err != nil {
		t.Fatal(err)
	}
	assertStatement(t, r, signals)
}

// Two input notes (60 + 50 merged into 70 paid and 40 back) are proved by the
// Payment2in circuit, so the request goes to its own route.
func TestBoundPaymentProof_TwoInputsUsePayment2in(t *testing.T) {
	signals := []int64{100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110}
	c := proveServer(t, "/proof/payment2in", signals)
	a := newPaymentArgs(t)
	r, err := c.BoundPaymentProof(big.NewInt(0xABCD), big.NewInt(0),
		[]*big.Int{big.NewInt(60), big.NewInt(50)}, []KeyPair{a.keys[0], a.keys[0]}, []*big.Int{big.NewInt(1), big.NewInt(2)},
		[]*big.Int{big.NewInt(70), big.NewInt(40)}, []*big.Int{big.NewInt(5), a.keys[0].PublicKey}, a.encapKeys,
		8, []*MerkleProof{a.proofs[0], a.proofs[0]}, []*big.Int{big.NewInt(0), big.NewInt(0)}, big.NewInt(7))
	if err != nil {
		t.Fatal(err)
	}
	if r.NumberOfInputs != 2 {
		t.Errorf("NumberOfInputs = %d, want 2", r.NumberOfInputs)
	}
	assertStatement(t, r, signals)
	assertInts(t, "Nullifiers", r.Nullifiers(), 105, 106)
	assertInts(t, "CommitmentsOut", r.CommitmentsOut(), 107, 108)
	assertInts(t, "ContractAddress", []*big.Int{r.ContractAddress()}, 109)
}

func assertInts(t *testing.T, name string, got []*big.Int, want ...int64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
	for i := range want {
		if got[i] == nil || got[i].Cmp(big.NewInt(want[i])) != 0 {
			t.Errorf("%s[%d] = %v, want %d", name, i, got[i], want[i])
		}
	}
}

// A PaymentFee result names each field of the statement the server returned.
func TestBoundPaymentFeeProof_AccessorsNameTheStatementsFields(t *testing.T) {
	// message, tree, root, nullifier, commitments out ×2, contract address, ciphertext hash, fee
	c := proveServer(t, "/proof/paymentFee", []int64{0, 3, 900, 901, 902, 903, 0xABCD, 10, 0xC1F7})
	a := newPaymentArgs(t)
	r, err := c.BoundPaymentFeeProof(big.NewInt(0xABCD), big.NewInt(10), big.NewInt(0),
		[]*big.Int{big.NewInt(100)}, a.keys, []*big.Int{big.NewInt(1)},
		[]*big.Int{big.NewInt(50), big.NewInt(40)}, []*big.Int{big.NewInt(5), a.keys[0].PublicKey}, a.encapKeys,
		8, a.proofs, []*big.Int{big.NewInt(3)}, big.NewInt(7))
	if err != nil {
		t.Fatal(err)
	}
	assertInts(t, "Message", []*big.Int{r.Message()}, 0)
	assertInts(t, "TreeNumbers", r.TreeNumbers(), 3)
	assertInts(t, "MerkleRoots", r.MerkleRoots(), 900)
	assertInts(t, "Nullifiers", r.Nullifiers(), 901)
	assertInts(t, "CommitmentsOut", r.CommitmentsOut(), 902, 903)
	assertInts(t, "ContractAddress", []*big.Int{r.ContractAddress()}, 0xABCD)
	assertInts(t, "Fee", []*big.Int{r.Fee()}, 10)
}

// A Payment result has no fee.
func TestBoundPaymentProof_HasNoFee(t *testing.T) {
	c := proveServer(t, "/proof/payment", []int64{0, 3, 900, 901, 902, 903, 0xABCD, 0xC1F7})
	a := newPaymentArgs(t)
	r, err := c.BoundPaymentProof(big.NewInt(0xABCD), big.NewInt(0),
		[]*big.Int{big.NewInt(100)}, a.keys, []*big.Int{big.NewInt(1)},
		[]*big.Int{big.NewInt(60), big.NewInt(40)}, []*big.Int{big.NewInt(5), a.keys[0].PublicKey}, a.encapKeys,
		8, a.proofs, []*big.Int{big.NewInt(3)}, big.NewInt(7))
	if err != nil {
		t.Fatal(err)
	}
	assertInts(t, "CommitmentsOut", r.CommitmentsOut(), 902, 903)
	assertInts(t, "ContractAddress", []*big.Int{r.ContractAddress()}, 0xABCD)
	if r.Fee() != nil {
		t.Errorf("Fee = %v, want nil", r.Fee())
	}
}

// A response whose statement is not the circuit's statement length is an
// error, not a result whose accessors read the wrong signals.
func TestBoundPaymentProof_RejectsAStatementOfTheWrongLength(t *testing.T) {
	c := proveServer(t, "/proof/payment", []int64{0, 3, 900, 901, 902, 903, 0xABCD})
	a := newPaymentArgs(t)
	_, err := c.BoundPaymentProof(big.NewInt(0xABCD), big.NewInt(0),
		[]*big.Int{big.NewInt(100)}, a.keys, []*big.Int{big.NewInt(1)},
		[]*big.Int{big.NewInt(60), big.NewInt(40)}, []*big.Int{big.NewInt(5), a.keys[0].PublicKey}, a.encapKeys,
		8, a.proofs, []*big.Int{big.NewInt(3)}, big.NewInt(7))
	if err == nil {
		t.Fatal("a 7-signal Payment statement was accepted")
	}
}

// PaymentCiphertextHash must equal CiphertextHash.sol's value, which the
// contracts compare with the proof's StCiphertextHash. The vectors are
// keccak256(abi.encode(bytes[], bytes[])) mod the scalar field, computed
// with ethers' ABI coder: no ciphertexts (Erc20CoinVault.transfer), one
// note (EnygmaDvp.payment*) and two (transferV2).
func TestPaymentCiphertextHash_MatchesTheContractsEncoding(t *testing.T) {
	for _, tc := range []struct {
		name      string
		ctI, ctII [][]byte
		want      string
	}{
		{"none", nil, nil, "2399156551665389805367252676811131530831779407996746425235550023578295295939"},
		{"one note", [][]byte{{1, 2}}, [][]byte{{3}}, "21386765348705937713341287917350094816637447784635915768830309461782184974402"},
		{"two notes", [][]byte{{1}, {2}}, [][]byte{{3}, {4}}, "13835726498560739053917206011880702979312706352419410452261044024720295753759"},
	} {
		if got := PaymentCiphertextHash(tc.ctI, tc.ctII); got.String() != tc.want {
			t.Errorf("%s: %s, want %s", tc.name, got, tc.want)
		}
	}
}

// The proof request binds the hash of exactly the ciphertexts the result
// hands back for EnygmaDvp.payment*.
func TestBoundPaymentProof_BindsItsCiphertexts(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		got, _ = req["stCiphertextHash"].(string)
		json.NewEncoder(w).Encode(map[string]any{
			"proof":        []int64{1, 2, 3, 4, 5, 6, 7, 8},
			"publicSignal": []int64{0, 3, 900, 901, 902, 903, 0xABCD, 0xC1F7},
		})
	}))
	defer srv.Close()
	a := newPaymentArgs(t)
	r, err := NewGnarkClient(srv.URL).BoundPaymentProof(big.NewInt(0xABCD), big.NewInt(0),
		[]*big.Int{big.NewInt(100)}, a.keys, []*big.Int{big.NewInt(1)},
		[]*big.Int{big.NewInt(60), big.NewInt(40)}, []*big.Int{big.NewInt(5), a.keys[0].PublicKey}, a.encapKeys,
		8, a.proofs, []*big.Int{big.NewInt(3)}, big.NewInt(7))
	if err != nil {
		t.Fatal(err)
	}
	if want := PaymentCiphertextHash([][]byte{r.CipherText}, [][]byte{r.EncTxData}).String(); got != want {
		t.Errorf("stCiphertextHash %q, want %s", got, want)
	}
}

// recordingProveServer is proveServer that also records each request body.
func recordingProveServer(t *testing.T, path string, nSignals int) (*GnarkClient, *[]map[string]any) {
	t.Helper()
	var reqs []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			t.Errorf("request to %s, want %s", r.URL.Path, path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		reqs = append(reqs, body)
		json.NewEncoder(w).Encode(map[string]any{
			"proof":        []int64{1, 2, 3, 4, 5, 6, 7, 8},
			"publicSignal": make([]int64, nSignals),
		})
	}))
	t.Cleanup(srv.Close)
	return NewGnarkClient(srv.URL), &reqs
}

// A payment's change note must be recoverable without local state: if the
// receipt is settled first with replaced ciphertexts, the sender's own
// transaction reverts although the input is spent. Every payment prover
// salts the change with WithdrawChangeSalt(sk, nullifier0), so the wallet
// rebuilds the note from its spend key and the spent nullifier
// (WithdrawChangeNote), and the request proves exactly that note.
func TestBoundPaymentProofs_ChangeNoteIsRebuiltFromKeyAndNullifier(t *testing.T) {
	a := newPaymentArgs(t)
	sk, tree, token, change := a.keys[0].PrivateKey, big.NewInt(3), big.NewInt(7), big.NewInt(40)
	nf, err := GetNullifierWithTree(sk, tree, a.proofs[0].Indices, 8)
	if err != nil {
		t.Fatal(err)
	}
	wantSalt, wantCommit, err := WithdrawChangeNote(sk, nf, change, token)
	if err != nil {
		t.Fatal(err)
	}

	pay, payReqs := recordingProveServer(t, "/proof/payment", paymentStatement.length)
	payRes, err := pay.BoundPaymentProof(big.NewInt(0xABCD), big.NewInt(0),
		[]*big.Int{big.NewInt(100)}, a.keys, []*big.Int{big.NewInt(1)},
		[]*big.Int{big.NewInt(60), change}, []*big.Int{big.NewInt(5), a.keys[0].PublicKey}, a.encapKeys,
		8, a.proofs, []*big.Int{tree}, token)
	if err != nil {
		t.Fatalf("BoundPaymentProof: %v", err)
	}
	fee, feeReqs := recordingProveServer(t, "/proof/paymentFee", paymentFeeStatement.length)
	feeRes, err := fee.BoundPaymentFeeProof(big.NewInt(0xABCD), big.NewInt(10), big.NewInt(0),
		[]*big.Int{big.NewInt(100)}, a.keys, []*big.Int{big.NewInt(1)},
		[]*big.Int{big.NewInt(50), change}, []*big.Int{big.NewInt(5), a.keys[0].PublicKey}, a.encapKeys,
		8, a.proofs, []*big.Int{tree}, token)
	if err != nil {
		t.Fatalf("BoundPaymentFeeProof: %v", err)
	}

	for _, r := range []struct {
		name string
		res  *PaymentResult
		req  map[string]any
	}{
		{"BoundPaymentProof", payRes, (*payReqs)[0]},
		{"BoundPaymentFeeProof", feeRes, (*feeReqs)[0]},
	} {
		if r.res.SaltA == nil || r.res.SaltA.Cmp(wantSalt) != 0 {
			t.Errorf("%s: SaltA %v, want WithdrawChangeSalt(sk, nf) %s", r.name, r.res.SaltA, wantSalt)
		}
		if got := r.req["wtSaltsOut"].([]any)[1]; got != wantSalt.String() {
			t.Errorf("%s: proved with change salt %v, want %s", r.name, got, wantSalt)
		}
		if got := r.req["stCommitmentsOut"].([]any)[1]; got != wantCommit.String() {
			t.Errorf("%s: proved change commitment %v, want WithdrawChangeNote's %s", r.name, got, wantCommit)
		}
	}
}
