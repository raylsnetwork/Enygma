package core

import (
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// A PrivateMint result names each field of the statement the server
// returned.
func TestErc20PrivateMintProof_AccessorsNameTheStatementsFields(t *testing.T) {
	// commitment, contract address, token id, cipher text
	signals := []int64{700, 0xABCD, 9, 701}
	c := proveServer(t, "/proof/privateMint", signals)
	r, err := c.Erc20PrivateMintProof(big.NewInt(5), big.NewInt(6), big.NewInt(100), big.NewInt(9), big.NewInt(0xABCD))
	if err != nil {
		t.Fatal(err)
	}
	assertInts(t, "ContractStatement", r.ContractStatement(), signals...)
	assertInts(t, "Commitment", []*big.Int{r.Commitment()}, 700)
	assertInts(t, "ContractAddress", []*big.Int{r.ContractAddress()}, 0xABCD)
	assertInts(t, "TokenId", []*big.Int{r.TokenId()}, 9)
	assertInts(t, "CipherText", []*big.Int{r.CipherText()}, 701)
	if len(r.Proof) != 8 {
		t.Errorf("Proof has %d elements, want 8", len(r.Proof))
	}
}

func TestErc20PrivateMintProof_RejectsAStatementOfTheWrongLength(t *testing.T) {
	c := proveServer(t, "/proof/privateMint", []int64{700, 0xABCD, 9})
	if _, err := c.Erc20PrivateMintProof(big.NewInt(5), big.NewInt(6), big.NewInt(100), big.NewInt(9), big.NewInt(0xABCD)); err == nil {
		t.Fatal("a 3-signal PrivateMint statement was accepted")
	}
}

// A mint's salt is never defaulted. PrivateMintProof refuses inputs whose
// salt is absent, unreadable or 0, and Erc20PrivateMintProof refuses nil and
// 0, each before any request to the gnark server: with salt 0, anyone could
// compute the mint's tag for every registered spend key.
func TestPrivateMintProof_RefusesAMissingOrZeroSalt(t *testing.T) {
	var mu sync.Mutex
	var bodies []map[string]any
	sent := func() []map[string]any {
		mu.Lock()
		defer mu.Unlock()
		return append([]map[string]any(nil), bodies...)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		mu.Lock()
		bodies = append(bodies, body)
		mu.Unlock()
		json.NewEncoder(w).Encode(map[string]any{
			"proof":        []string{"1", "2", "3", "4", "5", "6", "7", "8"},
			"publicSignal": []string{"700", "43981", "9", "701"},
		})
	}))
	t.Cleanup(srv.Close)
	c := NewGnarkClient(srv.URL)

	// mapInputs is a map-based mint request, with salt under "salt" unless
	// salt is absent.
	type absent struct{}
	mapInputs := func(salt any) map[string]interface{} {
		in := map[string]interface{}{
			"commitment": "700", "contractAddress": "43981", "tokenId": "9",
			"amount": "100", "publicKey": "5", "cipherText": "701",
		}
		if _, ok := salt.(absent); !ok {
			in["salt"] = salt
		}
		return in
	}

	for name, salt := range map[string]any{
		"no salt":       absent{},
		"a nil salt":    nil,
		"an empty salt": "",
		"salt \"0\"":    "0",
		"salt \"0x0\"":  "0x0",
		"a big.Int 0":   big.NewInt(0),
		"an unreadable": "salt",
	} {
		if _, err := c.PrivateMintProof(mapInputs(salt)); err == nil {
			t.Errorf("PrivateMintProof with %s: no error", name)
		}
	}
	for _, salt := range []*big.Int{big.NewInt(0), nil} {
		if _, err := c.Erc20PrivateMintProof(big.NewInt(5), salt, big.NewInt(100), big.NewInt(9), big.NewInt(0xABCD)); err == nil {
			t.Errorf("Erc20PrivateMintProof with salt %v: no error", salt)
		}
	}
	if got := sent(); len(got) != 0 {
		t.Fatalf("%d proof requests sent for refused salts, want none: %v", len(got), got)
	}

	// A salt given is passed on as given. A string salt is read as the gnark
	// server reads it, decimal unless it has a 0x prefix: "09" is 9, not a
	// bad octal number.
	for _, salt := range []string{"0x2a", "09"} {
		if _, err := c.PrivateMintProof(mapInputs(salt)); err != nil {
			t.Fatalf("PrivateMintProof with salt %s: %v", salt, err)
		}
	}
	if _, err := c.Erc20PrivateMintProof(big.NewInt(5), big.NewInt(6), big.NewInt(100), big.NewInt(9), big.NewInt(0xABCD)); err != nil {
		t.Fatalf("Erc20PrivateMintProof with salt 6: %v", err)
	}
	if got := sent(); len(got) != 3 || got[0]["salt"] != "0x2a" || got[1]["salt"] != "09" || got[2]["salt"] != "6" {
		t.Fatalf("requests sent: %v, want salts 0x2a, 09 and 6", got)
	}
}
