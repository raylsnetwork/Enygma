package core

import (
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
)

// paymentServer is a gnark server stub that answers path with the given
// public signals, whatever the request.
func paymentServer(t *testing.T, path string, signals []int64) *GnarkClient {
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
	c := paymentServer(t, "/proof/payment", signals)
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
	c := paymentServer(t, "/proof/paymentFee", signals)
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
	signals := []int64{100, 101, 102, 103, 104, 105, 106, 107, 108, 109}
	c := paymentServer(t, "/proof/payment2in", signals)
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
}
