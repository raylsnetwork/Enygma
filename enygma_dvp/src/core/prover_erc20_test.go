package core

import (
	"math/big"
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
