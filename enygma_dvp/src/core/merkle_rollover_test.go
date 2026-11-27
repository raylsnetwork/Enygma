package core

import (
	"math/big"
	"testing"
)

// A vault (Merkle.sol) starts a new tree when a transaction's leaves would
// reach its last slot, nextLeafIndex + count >= 2^depth, and keeps accepting
// the roots of the trees it closed. The wallet's tree must roll over at the
// same point and prove a closed tree's leaf against that tree's last root,
// under its own tree number, or its proofs name roots the vault never had.
// A depth-3 tree (8 slots) stands in for the deployed depth, which no test
// fills.
func TestMerkleTreeRollsOverWhereTheVaultDoes(t *testing.T) {
	const depth = 3
	slots := 1 << depth
	mt := NewMerkleTree(depth)
	leaf := func(i int) *big.Int { return big.NewInt(int64(1000 + i)) }

	// One leaf a transaction: slots 0 to 6 of tree 0, then the next starts
	// tree 1, so slot 7 is never used.
	for i := 0; i < slots-1; i++ {
		mt.InsertLeaves([]*big.Int{leaf(i)})
	}
	if mt.TreeNumber() != 0 || len(mt.GetLeaves()) != slots-1 {
		t.Fatalf("after %d one-leaf inserts: tree %d with %d leaves, want tree 0 with %d", slots-1, mt.TreeNumber(), len(mt.GetLeaves()), slots-1)
	}
	closedRoot := mt.Root()
	mt.InsertLeaves([]*big.Int{leaf(slots)})
	if mt.TreeNumber() != 1 || len(mt.GetLeaves()) != 1 {
		t.Fatalf("the insert that would fill tree 0's last slot left tree %d with %d leaves, want tree 1 with 1", mt.TreeNumber(), len(mt.GetLeaves()))
	}

	// A closed tree's leaf proves against that tree's last root.
	p, err := mt.GenerateProof(leaf(3))
	if err != nil {
		t.Fatalf("GenerateProof for a leaf of the closed tree: %v", err)
	}
	if p.TreeNumber != 0 || p.Root.Cmp(closedRoot) != 0 || p.Indices.Int64() != 3 {
		t.Errorf("proof names tree %d, root %s, index %s; want tree 0, its last root %s, index 3", p.TreeNumber, p.Root, p.Indices, closedRoot)
	}
	node := p.Element
	for level, sibling := range p.Elements {
		if p.Indices.Bit(level) == 0 {
			node = HashLeftRight(node, sibling)
		} else {
			node = HashLeftRight(sibling, node)
		}
	}
	if node.Cmp(closedRoot) != 0 {
		t.Errorf("the closed tree's path hashes to %s, want its last root %s", node, closedRoot)
	}

	// A transaction's leaves go in together: two leaves with slots-2 slots
	// used would reach the last slot, so both open tree 2.
	for i := 1; i < slots-2; i++ {
		mt.InsertLeaves([]*big.Int{leaf(slots + i)})
	}
	if mt.TreeNumber() != 1 || len(mt.GetLeaves()) != slots-2 {
		t.Fatalf("tree %d with %d leaves, want tree 1 with %d before the two-leaf insert", mt.TreeNumber(), len(mt.GetLeaves()), slots-2)
	}
	mt.InsertLeaves([]*big.Int{leaf(2 * slots), leaf(2*slots + 1)})
	if mt.TreeNumber() != 2 || len(mt.GetLeaves()) != 2 {
		t.Errorf("the two-leaf insert left tree %d with %d leaves, want tree 2 with both", mt.TreeNumber(), len(mt.GetLeaves()))
	}
}
