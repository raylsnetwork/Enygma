package engine

import (
	"math/big"
	"slices"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"
)

// rolloverDepth is a tree small enough to fill: 2^3 = 8 slots, of which a
// vault uses 7, as it starts a new tree when a batch would reach the last.
// The deployed depth's trees hold too many notes for a run to fill one.
const rolloverDepth = 3

// rolloverBatches are four transactions' batches into one vault: 3 leaves,
// then 4 that fill tree 0's 7 slots, then 1 that starts tree 1, and 2 more
// in tree 1. Transaction i's hash is common.Hash{i}.
func rolloverBatches() []batch {
	next := int64(100)
	b := func(tx byte, n int) batch {
		out := batch{tx: common.Hash{tx}}
		for range n {
			out.leaves = append(out.leaves, big.NewInt(next))
			next++
		}
		return out
	}
	return []batch{b(1, 3), b(2, 4), b(3, 1), b(4, 2)}
}

// The engine rebuilds a vault's trees as the vault fills them: when a
// batch would reach a tree's last slot, it starts the next tree, whose
// root before is the empty tree's, and the snapshot keeps the full tree's
// leaf count and last root. The crowd a spent note hides in is its own
// tree's leaves.
func TestTheEngineFollowsAVaultIntoItsNextTree(t *testing.T) {
	batches := rolloverBatches()
	emptyRoot := fieldHex(rpcore.NewMerkleTree(rolloverDepth).Root())

	tree, sizes := treesOf(rolloverDepth, batches)
	if tree.TreeNumber() != 1 || !slices.Equal(sizes, []int{7, 3}) {
		t.Errorf("treesOf: tree %d, crowds %v; want tree 1, crowds [7 3]", tree.TreeNumber(), sizes)
	}

	leaves := func(tx byte) HopDetail {
		t.Helper()
		d, err := leavesInOf(rolloverDepth, batches, NodeVault, common.Hash{tx}, nil)
		if err != nil {
			t.Fatalf("leavesInOf tx %d: %v", tx, err)
		}
		return d
	}
	indices := func(d HopDetail) []int {
		var out []int
		for _, l := range d.Leaves {
			out = append(out, l.Index)
		}
		return out
	}
	filled, started, after := leaves(2), leaves(3), leaves(4)
	if filled.TreeStarted || filled.Tree != 0 || !slices.Equal(indices(filled), []int{3, 4, 5, 6}) {
		t.Errorf("the batch that fills tree 0: started %v, tree %d, leaves %v; want tree 0, leaves [3 4 5 6]",
			filled.TreeStarted, filled.Tree, indices(filled))
	}
	if !started.TreeStarted || started.Tree != 1 || !slices.Equal(indices(started), []int{0}) || started.RootBefore != emptyRoot {
		t.Errorf("the batch after it: started %v, tree %d, leaves %v, root before %s; want tree 1 started, leaf [0], the empty tree's root %s",
			started.TreeStarted, started.Tree, indices(started), started.RootBefore, emptyRoot)
	}
	if after.TreeStarted || after.Tree != 1 || !slices.Equal(indices(after), []int{1, 2}) || after.RootBefore != started.RootAfter {
		t.Errorf("the next batch: started %v, tree %d, leaves %v, root before %s; want tree 1, leaves [1 2], root before %s",
			after.TreeStarted, after.Tree, indices(after), after.RootBefore, started.RootAfter)
	}

	snap := snapshotOf(rolloverDepth, NodeVault, batches)
	full := FullTree{Tree: 0, LeafCount: 7, Root: filled.RootAfter}
	if snap.Tree != 1 || snap.LeafCount != 3 || len(snap.Leaves) != 3 || snap.Root != fieldHex(tree.Root()) ||
		!slices.Equal(snap.FullTrees, []FullTree{full}) {
		t.Errorf("snapshotOf: tree %d with %d leaves (%d listed), root %s, full trees %+v; want tree 1 with 3, root %s, full trees [%+v]",
			snap.Tree, snap.LeafCount, len(snap.Leaves), snap.Root, snap.FullTrees, fieldHex(tree.Root()), full)
	}
}
