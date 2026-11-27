package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/raylsnetwork/enygma_dvp/src/vaulttree"
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
)

// leafLog is a leaf a vault inserted, as its LeafInserted event logged it:
// the tree and index it went to, and the transaction that inserted it.
type leafLog struct {
	vaulttree.Position
	tx common.Hash
}

// vaultLog is the leaves a vault logged before block next, oldest first.
type vaultLog struct {
	next   uint64
	leaves []leafLog
}

// vaultLeaves is every leaf the vault at vaultAddr has inserted since the
// deployment, oldest first, from its LeafInserted events. It reads each
// block once: a later call asks only for the blocks mined since. Then it
// checks the leaves against the vault at the tip: rebuilt, they must end on
// the vault's treeNumber(), with its currentRoot().
//
// The engine assumes one local node, whose blocks are final once mined and
// whose logs keep up with its head: it reads up to the head, not to a
// finalized block, and keeps no block hashes. On another node a block
// already read could be replaced, or the logs could lag the head, and the
// cached leaves would be wrong; so on a mismatch it reads every block
// again, once, and fails if the leaves still differ. On an error it keeps
// none of the new blocks' leaves, so the next call asks for them again.
func (c *chain) vaultLeaves(ctx context.Context, vaultAddr common.Address) ([]leafLog, error) {
	tip, err := c.client.BlockNumber(ctx)
	if err != nil {
		return nil, fmt.Errorf("vault LeafInserted events: %w", err)
	}
	v, ok := c.leaves[vaultAddr]
	if !ok {
		v = &vaultLog{next: c.deployBlock}
	}
	read, err := c.readLeaves(ctx, vaultAddr, v, tip)
	if err != nil {
		return nil, err
	}
	tree, root, err := vaulttree.Current(ctx, c.client, vaultAddr, new(big.Int).SetUint64(tip))
	if err != nil {
		return nil, err
	}
	if !c.endsAt(read.leaves, tree, root) {
		if read, err = c.readLeaves(ctx, vaultAddr, &vaultLog{next: c.deployBlock}, tip); err != nil {
			return nil, err
		}
		if !c.endsAt(read.leaves, tree, root) {
			return nil, fmt.Errorf("the %s vault is on tree %d with root 0x%s at block %d, but its LeafInserted events, read twice, rebuild other trees",
				c.contractName(vaultAddr), tree, root.Text(16), tip)
		}
	}
	if c.leaves == nil {
		c.leaves = map[common.Address]*vaultLog{}
	}
	c.leaves[vaultAddr] = read
	return read.leaves, nil
}

// readLeaves is v and the leaves the vault at vaultAddr logged in the
// blocks from v.next to tip, leaving v as it was.
func (c *chain) readLeaves(ctx context.Context, vaultAddr common.Address, v *vaultLog, tip uint64) (*vaultLog, error) {
	if tip < v.next {
		return v, nil
	}
	logs, err := c.logsBetween(ctx, v.next, tip, ethereum.FilterQuery{
		Addresses: []common.Address{vaultAddr},
		Topics:    [][]common.Hash{{vaulttree.LeafInsertedTopic}},
	})
	if err != nil {
		return nil, fmt.Errorf("vault LeafInserted events: %w", err)
	}
	read := &vaultLog{next: tip + 1, leaves: v.leaves}
	for _, l := range logs {
		p, err := vaulttree.Parse(l)
		if err != nil {
			return nil, err
		}
		read.leaves = append(read.leaves, leafLog{Position: p, tx: l.TxHash})
	}
	return read, nil
}

// endsAt says whether leaves, each placed where its event says, end on tree
// tree with root root. Leaves out of sequence (a gap or a repeat) end
// nowhere.
func (c *chain) endsAt(leaves []leafLog, tree int, root *big.Int) bool {
	mt := rpcore.NewMerkleTree(c.depth)
	for _, l := range leaves {
		if mt.InsertAt(l.Tree, l.Index, l.Leaf) != nil {
			return false
		}
	}
	return mt.TreeNumber() == tree && mt.Root().Cmp(root) == 0
}

// vaultTree rebuilds a vault's Merkle trees, numbered from 0, from its
// LeafInserted events, each leaf where its event says. The vault decides
// when to start a new tree once per insertLeaves call, and one transaction
// can make several calls, so no grouping of its Commitment events gives
// the trees. sizes[i] is how many leaves tree i holds: a spent note's
// crowd, the leaves its proof hides it among.
func (c *chain) vaultTree(ctx context.Context, vaultAddr common.Address) (tree *rpcore.MerkleTree, sizes []int, err error) {
	leaves, err := c.vaultLeaves(ctx, vaultAddr)
	if err != nil {
		return nil, nil, err
	}
	tree = rpcore.NewMerkleTree(c.depth)
	sizes = []int{0}
	for _, l := range leaves {
		if err := tree.InsertAt(l.Tree, l.Index, l.Leaf); err != nil {
			return nil, nil, err
		}
		if l.Tree == len(sizes) {
			sizes = append(sizes, 0)
		}
		sizes[l.Tree]++
	}
	return tree, sizes, nil
}

// leavesIn is the leavesIn hop detail of the leaves tx inserted into the
// vault at vaultAddr (node ID vault), where the vault's LeafInserted events
// put them. owners names the actor owning each leaf, by commitment. The
// detail shows one tree: a transaction whose leaves went into two (two
// inserts, the second starting a tree) is an error, and the engine sends no
// transaction that inserts into a vault twice.
func (c *chain) leavesIn(ctx context.Context, vault string, vaultAddr common.Address, tx common.Hash, owners map[string]string) (HopDetail, error) {
	leaves, err := c.vaultLeaves(ctx, vaultAddr)
	if err != nil {
		return HopDetail{}, err
	}
	tree := rpcore.NewMerkleTree(c.depth)
	var d *HopDetail
	for _, l := range leaves {
		if l.tx != tx {
			if d != nil {
				break // a transaction's leaves are consecutive
			}
			if err := tree.InsertAt(l.Tree, l.Index, l.Leaf); err != nil {
				return HopDetail{}, err
			}
			continue
		}
		if d == nil {
			d = &HopDetail{Kind: DetailLeavesIn, Vault: vault, Tree: l.Tree, TxHash: tx.Hex(),
				RootBefore: fieldHex(tree.Root()), TreeStarted: l.Tree != tree.TreeNumber()}
			if d.TreeStarted {
				d.RootBefore = fieldHex(rpcore.NewMerkleTree(c.depth).Root())
			}
		}
		if l.Tree != d.Tree {
			return HopDetail{}, fmt.Errorf("%s: tx %s inserted leaves into trees %d and %d, and the hop detail shows one tree", vault, tx.Hex(), d.Tree, l.Tree)
		}
		if err := tree.InsertAt(l.Tree, l.Index, l.Leaf); err != nil {
			return HopDetail{}, err
		}
		d.Leaves = append(d.Leaves, Leaf{Index: l.Index, Commitment: fieldHex(l.Leaf), Owner: owners[l.Leaf.String()]})
		d.RootAfter = fieldHex(tree.Root())
	}
	if d == nil {
		return HopDetail{}, fmt.Errorf("%s inserted no commitments in tx %s", vault, tx.Hex())
	}
	return *d, nil
}

// commitmentIn is the commitment hop detail of the note whose commitment
// tx deposited into the vault at vaultAddr (node ID vault), owned by owner.
func (c *chain) commitmentIn(ctx context.Context, vault string, vaultAddr common.Address, tx common.Hash, commitment *big.Int, owner string) (HopDetail, error) {
	d, err := c.leavesIn(ctx, vault, vaultAddr, tx, map[string]string{commitment.String(): owner})
	if err != nil {
		return HopDetail{}, err
	}
	if len(d.Leaves) != 1 || d.Leaves[0].Commitment != fieldHex(commitment) {
		return HopDetail{}, fmt.Errorf("%s: deposit tx %s inserted %d leaves, want its one note 0x%s", vault, tx.Hex(), len(d.Leaves), commitment.Text(16))
	}
	d.Kind, d.Commitment, d.Opening = DetailCommitment, fieldHex(commitment), noteOpening
	return d, nil
}

// treeSnapshot is the vault at vaultAddr's trees as its LeafInserted
// events leave them: the current tree's leaves and root, and each full tree's.
func (c *chain) treeSnapshot(ctx context.Context, vault string, vaultAddr common.Address) (TreeSnapshot, error) {
	leaves, err := c.vaultLeaves(ctx, vaultAddr)
	if err != nil {
		return TreeSnapshot{}, err
	}
	snap := TreeSnapshot{Vault: vault, FullTrees: []FullTree{}}
	tree := rpcore.NewMerkleTree(c.depth)
	for _, l := range leaves {
		num, count, root := tree.TreeNumber(), len(tree.GetLeaves()), tree.Root()
		if err := tree.InsertAt(l.Tree, l.Index, l.Leaf); err != nil {
			return TreeSnapshot{}, err
		}
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
