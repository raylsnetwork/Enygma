package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
)

// batch is the commitments one transaction inserted into a vault, in order.
type batch struct {
	tx     common.Hash
	leaves []*big.Int
}

// commitmentBatches reads a vault's Commitment events as the batches its
// transactions inserted, oldest first. (The event names the vault, not the
// tree.) No path the demo uses inserts into one vault twice in a transaction.
func (c *chain) commitmentBatches(ctx context.Context, vaultAddr common.Address) ([]batch, error) {
	logs, err := c.client.FilterLogs(ctx, ethereum.FilterQuery{
		Addresses: []common.Address{vaultAddr},
		Topics:    [][]common.Hash{{commitmentTopic}},
	})
	if err != nil {
		return nil, fmt.Errorf("vault Commitment events: %w", err)
	}
	var out []batch
	for _, l := range logs {
		if len(l.Topics) < 3 {
			continue
		}
		if len(out) == 0 || out[len(out)-1].tx != l.TxHash {
			out = append(out, batch{tx: l.TxHash})
		}
		last := &out[len(out)-1]
		last.leaves = append(last.leaves, l.Topics[2].Big())
	}
	return out, nil
}

// vaultTree rebuilds a vault's Merkle trees from its Commitment events. The
// vault inserts each transaction's commitments as one batch and starts a new
// tree, numbered from 0, when that batch would fill the current one, so the
// commitments are inserted a transaction at a time.
func (c *chain) vaultTree(ctx context.Context, vaultAddr common.Address) (*rpcore.MerkleTree, error) {
	batches, err := c.commitmentBatches(ctx, vaultAddr)
	if err != nil {
		return nil, err
	}
	tree := rpcore.NewMerkleTree(merkleDepth)
	for _, b := range batches {
		tree.InsertLeaves(b.leaves)
	}
	return tree, nil
}

// emptyRoot is the root of a tree with no leaves.
var emptyRoot = rpcore.NewMerkleTree(merkleDepth).Root()

// leavesIn is the leavesIn hop detail of the leaves tx inserted into the
// vault at vaultAddr (node ID vault), rebuilt from the vault's Commitment
// events. owners names the actor owning each leaf, by commitment.
func (c *chain) leavesIn(ctx context.Context, vault string, vaultAddr common.Address, tx common.Hash, owners map[string]string) (HopDetail, error) {
	batches, err := c.commitmentBatches(ctx, vaultAddr)
	if err != nil {
		return HopDetail{}, err
	}
	tree := rpcore.NewMerkleTree(merkleDepth)
	for _, b := range batches {
		if b.tx != tx {
			tree.InsertLeaves(b.leaves)
			continue
		}
		before, beforeTree := tree.Root(), tree.TreeNumber()
		tree.InsertLeaves(b.leaves)
		d := HopDetail{Kind: DetailLeavesIn, Vault: vault, Tree: tree.TreeNumber(), TxHash: tx.Hex(),
			RootBefore: fieldHex(before), RootAfter: fieldHex(tree.Root()), TreeStarted: tree.TreeNumber() != beforeTree}
		if d.TreeStarted {
			d.RootBefore = fieldHex(emptyRoot)
		}
		first := len(tree.GetLeaves()) - len(b.leaves)
		for i, l := range b.leaves {
			d.Leaves = append(d.Leaves, Leaf{Index: first + i, Commitment: fieldHex(l), Owner: owners[l.String()]})
		}
		return d, nil
	}
	return HopDetail{}, fmt.Errorf("%s inserted no commitments in tx %s", vault, tx.Hex())
}

// treeSnapshot is the vault at vaultAddr's trees as its Commitment events
// leave them: the current tree's leaves and root, and each full tree's.
func (c *chain) treeSnapshot(ctx context.Context, vault string, vaultAddr common.Address) (TreeSnapshot, error) {
	batches, err := c.commitmentBatches(ctx, vaultAddr)
	if err != nil {
		return TreeSnapshot{}, err
	}
	snap := TreeSnapshot{Vault: vault, FullTrees: []FullTree{}}
	tree := rpcore.NewMerkleTree(merkleDepth)
	for _, b := range batches {
		num, count, root := tree.TreeNumber(), len(tree.GetLeaves()), tree.Root()
		tree.InsertLeaves(b.leaves)
		if tree.TreeNumber() != num {
			snap.FullTrees = append(snap.FullTrees, FullTree{Tree: num, LeafCount: count, Root: fieldHex(root)})
		}
	}
	snap.Tree, snap.Root, snap.LeafCount = tree.TreeNumber(), fieldHex(tree.Root()), len(tree.GetLeaves())
	snap.Leaves = make([]Leaf, len(tree.GetLeaves()))
	for i, l := range tree.GetLeaves() {
		snap.Leaves[i] = Leaf{Index: i, Commitment: fieldHex(l)}
	}
	return snap, nil
}

// publishTrees sends both vaults' tree snapshots as the "trees" protocol field.
func (e *Engine) publishTrees(ctx context.Context, s *step) error {
	c := e.chain
	var snaps []TreeSnapshot
	for _, v := range []struct {
		node string
		addr common.Address
	}{{NodeVault, c.vaultAddr}, {NodeUsdrVault, c.usdrVaultAddr}} {
		snap, err := c.treeSnapshot(ctx, v.node, v.addr)
		if err != nil {
			return err
		}
		snaps = append(snaps, snap)
		s.log("key", "%s: tree %d holds %d leaves before this run, root 0x%s…", v.node, snap.Tree, snap.LeafCount, snap.Root[2:14])
	}
	data, err := json.Marshal(snaps)
	if err != nil {
		return err
	}
	s.proto("trees", string(data))
	return nil
}
