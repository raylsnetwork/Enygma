package core

import (
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
)

// dvpInitiatorServer is a gnark server stub for /proof/dvpInitiator that
// records the wtRevertSalt and stRevertCommitA of every request.
func dvpInitiatorServer(t *testing.T) (*GnarkClient, *[]map[string]any) {
	t.Helper()
	var reqs []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/proof/dvpInitiator" {
			t.Errorf("request to %s, want /proof/dvpInitiator", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		reqs = append(reqs, body)
		json.NewEncoder(w).Encode(map[string]any{"proof": []int64{1, 2, 3, 4, 5, 6, 7, 8}})
	}))
	t.Cleanup(srv.Close)
	return NewGnarkClient(srv.URL), &reqs
}

// The initiator's revert note must be openable after claimSwapTimeout, which
// spends the swap input: every initiator prover returns the salt it proved
// with, and the wallet can rebuild it from the spend key and the nullifier.
func TestDvPInitiatorRevertSaltIsReturnedAndDerivable(t *testing.T) {
	c, reqs := dvpInitiatorServer(t)
	a := newPaymentArgs(t)
	alice := a.keys[0]
	bob, err := NewSpendKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	view, err := NewViewKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	value, tokenId := big.NewInt(100), big.NewInt(7)
	treeNumber, depth, counterVault := big.NewInt(0), 8, big.NewInt(0xbeef)
	nf, err := GetNullifierWithTree(alice.PrivateKey, treeNumber, a.proofs[0].Indices, depth)
	if err != nil {
		t.Fatal(err)
	}
	wantSalt, wantCommit, err := SwapRevertNote(alice.PrivateKey, nf, value, tokenId)
	if err != nil {
		t.Fatal(err)
	}

	kem, err := c.DvPInitiatorProof(alice, big.NewInt(1), value, tokenId, bob.PublicKey, view.EncapsKey,
		big.NewInt(1), big.NewInt(25), treeNumber, a.proofs[0], depth, counterVault)
	if err != nil {
		t.Fatalf("DvPInitiatorProof: %v", err)
	}
	fromSalts, err := c.DvPInitiatorProofFromSalts(alice, big.NewInt(1), value, tokenId, bob.PublicKey,
		big.NewInt(2), big.NewInt(1), big.NewInt(25), big.NewInt(3), treeNumber, a.proofs[0], depth, counterVault)
	if err != nil {
		t.Fatalf("DvPInitiatorProofFromSalts: %v", err)
	}
	zk, err := c.ZkDvpInitiateSwap(alice, big.NewInt(1), value, tokenId, bob.PublicKey, view.EncapsKey,
		big.NewInt(25), big.NewInt(1), depth, a.proofs[0], treeNumber)
	if err != nil {
		t.Fatalf("ZkDvpInitiateSwap: %v", err)
	}

	results := []struct {
		name         string
		salt, commit *big.Int
	}{
		{"DvPInitiatorProof", kem.RevertSalt, kem.RevertCommitA},
		{"DvPInitiatorProofFromSalts", fromSalts.RevertSalt, fromSalts.RevertCommitA},
		{"ZkDvpInitiateSwap", zk.RevertSalt, zk.RevertCommitA},
	}
	for i, r := range results {
		if r.salt == nil || r.salt.Cmp(wantSalt) != 0 {
			t.Errorf("%s: RevertSalt %v, want SwapRevertSalt(sk, nf) %s", r.name, r.salt, wantSalt)
		}
		if r.commit == nil || r.commit.Cmp(wantCommit) != 0 {
			t.Errorf("%s: RevertCommitA %v, want SwapRevertNote's %s", r.name, r.commit, wantCommit)
		}
		if got := (*reqs)[i]["wtRevertSalt"]; got != wantSalt.String() {
			t.Errorf("%s: proved with wtRevertSalt %v, want %s", r.name, got, wantSalt)
		}
		if got := (*reqs)[i]["stRevertCommitA"]; got != wantCommit.String() {
			t.Errorf("%s: proved with stRevertCommitA %v, want %s", r.name, got, wantCommit)
		}
	}

	changeSalt, err := WithdrawChangeSalt(alice.PrivateKey, nf)
	if err != nil {
		t.Fatal(err)
	}
	if changeSalt.Cmp(wantSalt) == 0 {
		t.Error("SwapRevertSalt equals WithdrawChangeSalt for the same nullifier")
	}
}
