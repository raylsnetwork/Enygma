package core

import (
	"fmt"
	"math/big"
	"strings"
	"testing"
)

// testDepth keeps the trees small: 8 slots, the last of which a vault
// never fills, so a batch reaching index 7 goes into the next tree.
const testDepth = 3

// leaves is n distinct leaves from first on.
func leaves(first, n int64) []*big.Int {
	out := make([]*big.Int, n)
	for i := range out {
		out[i] = big.NewInt(first + int64(i))
	}
	return out
}

// position is a leaf and the (tree, index) a LeafInserted event gives it.
type position struct {
	tree, index int
	leaf        *big.Int
}

// positionsOf replays calls, each one insertLeaves call of the vault, and
// returns where each leaf went: a call that would reach a tree's last slot
// goes whole into the next tree.
func positionsOf(depth int, calls ...[]*big.Int) []position {
	var out []position
	tree, next := 0, 0
	for _, call := range calls {
		if next+len(call) >= 1<<depth {
			tree, next = tree+1, 0
		}
		for _, l := range call {
			out = append(out, position{tree, next, l})
			next++
		}
	}
	return out
}

// insertAll places every position with InsertAt, failing the test on a refusal.
func insertAll(t *testing.T, mt *MerkleTree, ps []position) {
	t.Helper()
	for _, p := range ps {
		if err := mt.InsertAt(p.tree, p.index, p.leaf); err != nil {
			t.Fatalf("InsertAt(%d, %d): %v", p.tree, p.index, err)
		}
	}
}

// foldProof hashes p's element up with its siblings, by the bits of its
// index, to the root the proof claims.
func foldProof(p *MerkleProof) *big.Int {
	node := p.Element
	for level, sib := range p.Elements {
		if p.Indices.Bit(level) == 1 {
			node = HashLeftRight(sib, node)
		} else {
			node = HashLeftRight(node, sib)
		}
	}
	return node
}

// An empty batch, which a vault refuses, leaves the tree as it was: an empty
// tree keeps its root, and a tree holding leaves its leaves and root.
func TestInsertLeavesTakesAnEmptyBatchAsNothing(t *testing.T) {
	mt := NewMerkleTree(testDepth)
	empty := mt.Root()
	mt.InsertLeaves(nil)
	if len(mt.GetLeaves()) != 0 || mt.Root().Cmp(empty) != 0 {
		t.Fatalf("an empty batch on an empty tree: %d leaves, root %s; want none and the empty root %s", len(mt.GetLeaves()), mt.Root(), empty)
	}
	mt.InsertLeaves(leaves(1, 3))
	root := mt.Root()
	mt.InsertLeaves([]*big.Int{})
	if mt.TreeNumber() != 0 || len(mt.GetLeaves()) != 3 || mt.Root().Cmp(root) != 0 {
		t.Errorf("an empty batch after 3 leaves: tree %d, %d leaves, root %s; want tree 0, 3 leaves, root %s", mt.TreeNumber(), len(mt.GetLeaves()), mt.Root(), root)
	}
}

// insertBatch is what InsertLeaves of a batch of n leaves into mt panics
// with, nil if it takes the batch.
func insertBatch(mt *MerkleTree, n int64) (panicked any) {
	defer func() { panicked = recover() }()
	mt.InsertLeaves(leaves(1, n))
	return nil
}

// A batch of 2^depth leaves or more, which a vault refuses, panics; one
// leaf fewer fits a fresh tree. A NewMerkleTreeStrict tree, which tracks an
// AuctionCoinVault, takes 2^depth.
func TestInsertLeavesRefusesABatchAVaultRefuses(t *testing.T) {
	fits := NewMerkleTree(testDepth)
	if r := insertBatch(fits, 1<<testDepth-1); r != nil || fits.TreeNumber() != 0 || len(fits.GetLeaves()) != 1<<testDepth-1 {
		t.Errorf("a batch of %d: panic %v, tree %d holds %d leaves; want tree 0 to hold them all", 1<<testDepth-1, r, fits.TreeNumber(), len(fits.GetLeaves()))
	}
	for _, n := range []int64{1 << testDepth, 1<<testDepth + 1} {
		mt := NewMerkleTree(testDepth)
		if r := insertBatch(mt, n); r == nil || !strings.Contains(fmt.Sprint(r), "which a vault refuses") {
			t.Errorf("a batch of %d leaves: panic %v, tree %d holds %d leaves; want a panic naming the vault's refusal", n, r, mt.TreeNumber(), len(mt.GetLeaves()))
		}
	}
	strict := NewMerkleTreeStrict(testDepth)
	if r := insertBatch(strict, 1<<testDepth); r != nil || strict.TreeNumber() != 0 || len(strict.GetLeaves()) != 1<<testDepth {
		t.Errorf("a strict tree's batch of %d: panic %v, tree %d holds %d leaves; want tree 0 to hold them all", 1<<testDepth, r, strict.TreeNumber(), len(strict.GetLeaves()))
	}
}

// One transaction making two insertLeaves calls on a vault, the second
// reaching the tree's last slot: the first call's leaf stays in tree 0, the
// second call's two start tree 1. Placed at their events' positions, the
// leaves give the trees the per-call batches give, while one batch per
// transaction (the second call's leaves moving the first's) does not.
func TestInsertAtRebuildsPerCallBatches(t *testing.T) {
	fill, first, second := leaves(1, 6), leaves(100, 1), leaves(200, 2)

	want := NewMerkleTree(testDepth)
	for _, call := range [][]*big.Int{fill, first, second} {
		want.InsertLeaves(call)
	}
	got := NewMerkleTree(testDepth)
	insertAll(t, got, positionsOf(testDepth, fill, first, second))

	if got.TreeNumber() != 1 || want.TreeNumber() != 1 {
		t.Fatalf("on tree %d, the per-call batches on tree %d, want 1", got.TreeNumber(), want.TreeNumber())
	}
	if got.Root().Cmp(want.Root()) != 0 || got.RootOfPrevTree(0).Cmp(want.RootOfPrevTree(0)) != 0 {
		t.Errorf("roots (tree 0 %s, tree 1 %s), the per-call batches' (%s, %s)", got.RootOfPrevTree(0), got.Root(), want.RootOfPrevTree(0), want.Root())
	}
	if n := len(got.prevTrees[0][0]); n != 7 {
		t.Errorf("tree 0 holds %d leaves, want 7: the first call's leaf stays at index 6", n)
	}

	perTx := NewMerkleTree(testDepth)
	perTx.InsertLeaves(fill)
	perTx.InsertLeaves(append(append([]*big.Int{}, first...), second...))
	if perTx.Root().Cmp(got.Root()) == 0 {
		t.Error("one batch per transaction gives the same tree 1: the calls do not split at the boundary")
	}
}

// InsertAt refuses every position but the current tree's next index and
// the next tree's index 0, and an index past the tree's capacity, and
// leaves the tree as it was.
func TestInsertAtRefusesOutOfSequencePositions(t *testing.T) {
	for _, tc := range []struct {
		name        string
		tree, index int
	}{
		{"a gap", 1, 3},
		{"a repeat", 1, 1},
		{"an earlier tree", 0, 6},
		{"a tree skipped", 3, 0},
		{"the next tree past index 0", 2, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mt := NewMerkleTree(testDepth)
			insertAll(t, mt, positionsOf(testDepth, leaves(1, 6), leaves(10, 2)))
			root := mt.Root()
			err := mt.InsertAt(tc.tree, tc.index, big.NewInt(99))
			if err == nil || !strings.Contains(err.Error(), "takes tree 1 index 2, or tree 2 index 0") {
				t.Fatalf("InsertAt(%d, %d) = %v, want a refusal naming tree 1 index 2 and tree 2 index 0", tc.tree, tc.index, err)
			}
			if mt.TreeNumber() != 1 || len(mt.GetLeaves()) != 2 || mt.Root().Cmp(root) != 0 {
				t.Errorf("after the refusal: tree %d, %d leaves, root changed %v; want tree 1, 2 leaves, the same root",
					mt.TreeNumber(), len(mt.GetLeaves()), mt.Root().Cmp(root) != 0)
			}
		})
	}

	t.Run("past the capacity", func(t *testing.T) {
		mt := NewMerkleTree(testDepth)
		for i := range 8 {
			if err := mt.InsertAt(0, i, big.NewInt(int64(i+1))); err != nil {
				t.Fatalf("InsertAt(0, %d): %v", i, err)
			}
		}
		root := mt.Root()
		err := mt.InsertAt(0, 8, big.NewInt(9))
		if err == nil || !strings.Contains(err.Error(), "a tree of depth 3 holds 8 leaves") {
			t.Fatalf("InsertAt(0, 8) = %v, want a refusal: index 8 is past a depth-3 tree", err)
		}
		if len(mt.GetLeaves()) != 8 || mt.Root().Cmp(root) != 0 {
			t.Errorf("after the refusal: %d leaves, want the 8 before it and the same root", len(mt.GetLeaves()))
		}
	})
}

// Each leaf InsertAt adds rehashes only its path, and gives the root a
// whole rebuild of the same leaves does, at every count.
func TestInsertAtMatchesAWholeRebuild(t *testing.T) {
	mt := NewMerkleTree(testDepth)
	all := leaves(7, 8)
	for i, l := range all {
		if err := mt.InsertAt(0, i, l); err != nil {
			t.Fatal(err)
		}
		whole := NewMerkleTreeStrict(testDepth)
		whole.InsertLeaves(all[:i+1])
		if mt.Root().Cmp(whole.Root()) != 0 {
			t.Fatalf("after %d leaves the root is %s, a whole rebuild's %s", i+1, mt.Root(), whole.Root())
		}
	}
}

// A vault takes the same commitment twice as two notes. GenerateProof
// reaches only one of them, the current tree's; GenerateProofAt proves
// either, each against its own tree's root, and refuses a position the
// trees do not hold.
func TestGenerateProofAt(t *testing.T) {
	dup := big.NewInt(42)
	mt := NewMerkleTree(testDepth)
	fill := leaves(1, 5)
	insertAll(t, mt, positionsOf(testDepth, []*big.Int{fill[0], dup}, fill[1:], []*big.Int{dup, big.NewInt(43)}))
	if mt.TreeNumber() != 1 {
		t.Fatalf("on tree %d, want the third call to start tree 1", mt.TreeNumber())
	}

	first, err := mt.GenerateProof(dup)
	if err != nil {
		t.Fatal(err)
	}
	if first.TreeNumber != 1 || first.Indices.Int64() != 0 {
		t.Errorf("GenerateProof found tree %d index %s, want the current tree's copy, tree 1 index 0", first.TreeNumber, first.Indices)
	}
	for _, at := range []struct {
		tree, index int
		root        *big.Int
	}{{0, 1, mt.RootOfPrevTree(0)}, {1, 0, mt.Root()}, {0, 5, mt.RootOfPrevTree(0)}} {
		p, err := mt.GenerateProofAt(at.tree, at.index)
		if err != nil {
			t.Fatalf("GenerateProofAt(%d, %d): %v", at.tree, at.index, err)
		}
		if p.TreeNumber != at.tree || p.Indices.Int64() != int64(at.index) || p.Root.Cmp(at.root) != 0 {
			t.Errorf("GenerateProofAt(%d, %d) proves tree %d index %s root %s, want root %s", at.tree, at.index, p.TreeNumber, p.Indices, p.Root, at.root)
		}
		if got := foldProof(p); got.Cmp(p.Root) != 0 {
			t.Errorf("GenerateProofAt(%d, %d): the path folds to %s, not its root %s", at.tree, at.index, got, p.Root)
		}
	}
	if p, _ := mt.GenerateProofAt(0, 1); p.Element.Cmp(dup) != 0 {
		t.Errorf("GenerateProofAt(0, 1) proves %s, want the first copy %s", p.Element, dup)
	}

	for _, at := range [][2]int{{2, 0}, {-1, 0}, {0, 6}, {1, 2}, {0, -1}} {
		if p, err := mt.GenerateProofAt(at[0], at[1]); err == nil {
			t.Errorf("GenerateProofAt(%d, %d) = %+v, want an error: the trees hold no such leaf", at[0], at[1], p)
		}
	}
}
