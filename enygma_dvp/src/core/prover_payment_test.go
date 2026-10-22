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
	signals := []int64{100, 101, 102, 103, 104, 105, 106}
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
	signals := []int64{100, 101, 102, 103, 104, 105, 106, 107}
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
	// message, tree, root, nullifier, commitments out ×2, contract address, fee
	c := proveServer(t, "/proof/paymentFee", []int64{0, 3, 900, 901, 902, 903, 0xABCD, 10})
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
	c := proveServer(t, "/proof/payment", []int64{0, 3, 900, 901, 902, 903, 0xABCD})
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
	c := proveServer(t, "/proof/payment", []int64{0, 3, 900, 901, 902, 903})
	a := newPaymentArgs(t)
	_, err := c.BoundPaymentProof(big.NewInt(0xABCD), big.NewInt(0),
		[]*big.Int{big.NewInt(100)}, a.keys, []*big.Int{big.NewInt(1)},
		[]*big.Int{big.NewInt(60), big.NewInt(40)}, []*big.Int{big.NewInt(5), a.keys[0].PublicKey}, a.encapKeys,
		8, a.proofs, []*big.Int{big.NewInt(3)}, big.NewInt(7))
	if err == nil {
		t.Fatal("a 6-signal Payment statement was accepted")
	}
}
