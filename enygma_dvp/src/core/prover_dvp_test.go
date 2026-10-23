package core

import (
	"math/big"
	"testing"
)

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
