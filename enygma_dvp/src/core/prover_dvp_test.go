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
		// Answer with the statement the request claims, in DvpInitiator
		// layout order, as a server proving that witness would.
		var signals []json.Number
		for _, k := range []string{"stMessage", "stTreeNumber", "stMerkleRoot", "stNullifier",
			"stCommitB", "stCommitA", "stRevertCommitA", "stCounterVault"} {
			v, _ := body[k].(string)
			if v == "" {
				v = "0"
			}
			signals = append(signals, json.Number(v))
		}
		json.NewEncoder(w).Encode(map[string]any{"proof": []int64{1, 2, 3, 4, 5, 6, 7, 8}, "publicSignal": signals})
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
		{"DvPInitiatorProof", kem.RevertSalt, kem.RevertCommitA()},
		{"DvPInitiatorProofFromSalts", fromSalts.RevertSalt, fromSalts.RevertCommitA()},
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

// dvpArgs are the inputs common to the DvP prove functions: a spend key and
// a one-note Merkle proof.
func dvpArgs(t *testing.T) (KeyPair, *MerkleProof) {
	t.Helper()
	a := newPaymentArgs(t)
	return a.keys[0], a.proofs[0]
}

// A DvpInitiator result names each field of the statement the server
// returned, not one rebuilt from the request.
func TestDvPInitiatorProof_AccessorsNameTheStatementsFields(t *testing.T) {
	// message, tree, root, nullifier, commitB, commitA, revertCommitA, counterVault
	signals := []int64{800, 3, 900, 901, 902, 800, 903, 0xBEEF}
	c := proveServer(t, "/proof/dvpInitiator", signals)
	key, mp := dvpArgs(t)
	r, err := c.DvPInitiatorProofFromSalts(key, big.NewInt(1), big.NewInt(100), big.NewInt(0),
		big.NewInt(5), big.NewInt(6), big.NewInt(1), big.NewInt(1), big.NewInt(7),
		big.NewInt(3), mp, 8, big.NewInt(0xBEEF))
	if err != nil {
		t.Fatal(err)
	}
	assertInts(t, "ContractStatement", r.ContractStatement(), signals...)
	assertInts(t, "Message", []*big.Int{r.Message()}, 800)
	assertInts(t, "TreeNumber", []*big.Int{r.TreeNumber()}, 3)
	assertInts(t, "MerkleRoot", []*big.Int{r.MerkleRoot()}, 900)
	assertInts(t, "Nullifier", []*big.Int{r.Nullifier()}, 901)
	assertInts(t, "CommitB", []*big.Int{r.CommitB()}, 902)
	assertInts(t, "CommitA", []*big.Int{r.CommitA()}, 800)
	assertInts(t, "RevertCommitA", []*big.Int{r.RevertCommitA()}, 903)
	assertInts(t, "CounterVault", []*big.Int{r.CounterVault()}, 0xBEEF)
}

// The statement is never truncated: CounterVault, the last signal, is
// submitted even though the receipt reports one output.
func TestDvPInitiatorProof_ContractStatementKeepsTheCounterVault(t *testing.T) {
	c := proveServer(t, "/proof/dvpInitiator", []int64{800, 3, 900, 901, 902, 800, 903, 0xBEEF})
	key, mp := dvpArgs(t)
	r, err := c.DvPInitiatorProofFromSalts(key, big.NewInt(1), big.NewInt(100), big.NewInt(0),
		big.NewInt(5), big.NewInt(6), big.NewInt(1), big.NewInt(1), big.NewInt(7),
		big.NewInt(3), mp, 8, big.NewInt(0xBEEF))
	if err != nil {
		t.Fatal(err)
	}
	st := r.ContractStatement()
	if st[len(st)-1].Cmp(big.NewInt(0xBEEF)) != 0 {
		t.Errorf("ContractStatement %v does not end with the counter vault", st)
	}
}

func TestDvPInitiatorProof_RejectsAStatementOfTheWrongLength(t *testing.T) {
	c := proveServer(t, "/proof/dvpInitiator", []int64{800, 3, 900, 901, 902, 800, 903})
	key, mp := dvpArgs(t)
	_, err := c.DvPInitiatorProofFromSalts(key, big.NewInt(1), big.NewInt(100), big.NewInt(0),
		big.NewInt(5), big.NewInt(6), big.NewInt(1), big.NewInt(1), big.NewInt(7),
		big.NewInt(3), mp, 8, big.NewInt(0xBEEF))
	if err == nil {
		t.Fatal("a 7-signal DvpInitiator statement was accepted")
	}
}

// A DvpDestination result names each field of the statement the server
// returned.
func TestDvPDestinationProof_AccessorsNameTheStatementsFields(t *testing.T) {
	// message, tree, root, nullifier, commitA, counterVault
	signals := []int64{802, 4, 910, 911, 800, 0xCAFE}
	c := proveServer(t, "/proof/dvpDestination", signals)
	key, mp := dvpArgs(t)
	r, err := c.DvPDestinationProof(key, big.NewInt(1), big.NewInt(1), big.NewInt(1),
		big.NewInt(5), big.NewInt(7), big.NewInt(6), big.NewInt(100), big.NewInt(0), big.NewInt(800),
		big.NewInt(4), mp, 8, big.NewInt(0xCAFE))
	if err != nil {
		t.Fatal(err)
	}
	assertInts(t, "ContractStatement", r.ContractStatement(), signals...)
	assertInts(t, "Message", []*big.Int{r.Message()}, 802)
	assertInts(t, "TreeNumber", []*big.Int{r.TreeNumber()}, 4)
	assertInts(t, "MerkleRoot", []*big.Int{r.MerkleRoot()}, 910)
	assertInts(t, "Nullifier", []*big.Int{r.Nullifier()}, 911)
	assertInts(t, "CommitA", []*big.Int{r.CommitA()}, 800)
	assertInts(t, "CounterVault", []*big.Int{r.CounterVault()}, 0xCAFE)
}

func TestDvPDestinationProof_RejectsAStatementOfTheWrongLength(t *testing.T) {
	c := proveServer(t, "/proof/dvpDestination", []int64{802, 4, 910, 911, 800, 0xCAFE, 1})
	key, mp := dvpArgs(t)
	_, err := c.DvPDestinationProof(key, big.NewInt(1), big.NewInt(1), big.NewInt(1),
		big.NewInt(5), big.NewInt(7), big.NewInt(6), big.NewInt(100), big.NewInt(0), big.NewInt(800),
		big.NewInt(4), mp, 8, big.NewInt(0xCAFE))
	if err == nil {
		t.Fatal("a 7-signal DvpDestination statement was accepted")
	}
}
