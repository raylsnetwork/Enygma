package engine

import (
	"context"
	"math/big"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/raylsnetwork/enygma_dvp/src/vaulttree"
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// treeTestDepth keeps the trees small: 8 slots, the last never filled, so
// a call that would reach index 7 goes into the next tree.
const treeTestDepth = 3

// txHash is the hash of the test's i-th transaction.
func txHash(i int) common.Hash { return common.BigToHash(big.NewInt(int64(1000 + i))) }

// leafInserted is the vault's LeafInserted log of leaf at (tree, index),
// mined in block by tx.
func leafInserted(tree, index int, leaf *big.Int, block uint64, tx common.Hash) ethtypes.Log {
	data := make([]byte, 64)
	big.NewInt(int64(tree)).FillBytes(data[:32])
	big.NewInt(int64(index)).FillBytes(data[32:])
	return ethtypes.Log{Address: testVault, BlockNumber: block, TxHash: tx, Data: data,
		Topics: []common.Hash{vaulttree.LeafInsertedTopic, common.BigToHash(erc20VaultId), common.BigToHash(leaf)}}
}

// vaultTxs is the LeafInserted logs of txs, transaction i mined in block
// i + 2 as txHash(i), each a list of insertLeaves calls, placed as the vault
// places them: a call that would reach a tree's last slot goes whole into
// the next tree. want is the trees the calls build, one InsertLeaves batch
// per call.
func vaultTxs(txs ...[][]*big.Int) (logs []ethtypes.Log, want *rpcore.MerkleTree) {
	want = rpcore.NewMerkleTree(treeTestDepth)
	tree, next := 0, 0
	for i, calls := range txs {
		for _, call := range calls {
			want.InsertLeaves(call)
			if next+len(call) >= 1<<treeTestDepth {
				tree, next = tree+1, 0
			}
			for _, l := range call {
				logs = append(logs, leafInserted(tree, next, l, uint64(i+2), txHash(i)))
				next++
			}
		}
	}
	return logs, want
}

// leafRange is n distinct leaves from first on.
func leafRange(first, n int64) []*big.Int {
	out := make([]*big.Int, n)
	for i := range out {
		out[i] = big.NewInt(first + int64(i))
	}
	return out
}

// treeChain is a chain whose node is node, with the vault's trees at
// treeTestDepth and its deployment at block 1.
func treeChain(t *testing.T, node *fakeNode) *chain {
	t.Helper()
	node.depth = treeTestDepth
	srv := httptest.NewServer(node)
	t.Cleanup(srv.Close)
	client, err := ethclient.Dial(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	return &chain{client: client, deployBlock: 1, depth: treeTestDepth, vaultAddr: testVault}
}

// The vault's events put each leaf where the vault did: transaction 1
// makes two inserts, the first's leaf filling index 5 of tree 0 and the
// second's two starting tree 1, which no grouping of the Commitment events
// by transaction (or leaf by leaf) gives. Transaction 4's leaf starts tree 2.
func TestVaultTreesPlaceLeavesWhereTheirEventsSay(t *testing.T) {
	x, y, z, w, u := big.NewInt(100), big.NewInt(101), big.NewInt(102), big.NewInt(103), big.NewInt(104)
	logs, want := vaultTxs(
		[][]*big.Int{leafRange(1, 5)},
		[][]*big.Int{{x}, {y, z}},
		[][]*big.Int{{w}},
		[][]*big.Int{leafRange(10, 4)},
		[][]*big.Int{{u}},
	)
	c := treeChain(t, &fakeNode{tip: 7, leaves: logs})
	ctx := context.Background()

	tree, sizes, err := c.vaultTree(ctx, testVault)
	if err != nil {
		t.Fatal(err)
	}
	if tree.TreeNumber() != 2 || tree.Root().Cmp(want.Root()) != 0 || !slices.Equal(sizes, []int{6, 7, 1}) {
		t.Fatalf("tree %d root %s sizes %v, want tree 2 root %s sizes [6 7 1]", tree.TreeNumber(), tree.Root(), sizes, want.Root())
	}
	for i := range 2 {
		if tree.RootOfPrevTree(i).Cmp(want.RootOfPrevTree(i)) != 0 {
			t.Errorf("tree %d's root %s, want %s", i, tree.RootOfPrevTree(i), want.RootOfPrevTree(i))
		}
	}
	if p, err := tree.GenerateProof(x); err != nil || p.TreeNumber != 0 || p.Indices.Int64() != 5 {
		t.Errorf("the first insert's leaf is at %+v (err %v), want tree 0 index 5", p, err)
	}

	snap, err := c.treeSnapshot(ctx, NodeVault, testVault)
	if err != nil {
		t.Fatal(err)
	}
	wantFull := []FullTree{{0, 6, fieldHex(want.RootOfPrevTree(0))}, {1, 7, fieldHex(want.RootOfPrevTree(1))}}
	if !slices.Equal(snap.FullTrees, wantFull) || snap.Tree != 2 || snap.LeafCount != 1 || snap.Root != fieldHex(want.Root()) {
		t.Errorf("snapshot %+v, want full trees %+v and tree 2 holding one leaf", snap, wantFull)
	}

	for _, tc := range []struct {
		name    string
		tx      int
		tree    int
		indices []int
		started bool
	}{
		{"a fresh tree's first leaves", 0, 0, []int{0, 1, 2, 3, 4}, false},
		{"a leaf after a new tree's first", 2, 1, []int{2}, false},
		{"a leaf starting a tree", 4, 2, []int{0}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, err := c.leavesIn(ctx, NodeVault, testVault, txHash(tc.tx), nil)
			if err != nil {
				t.Fatal(err)
			}
			var indices []int
			for _, l := range d.Leaves {
				indices = append(indices, l.Index)
			}
			if d.Tree != tc.tree || !slices.Equal(indices, tc.indices) || d.TreeStarted != tc.started {
				t.Errorf("leaves %v in tree %d, started %v; want %v in tree %d, started %v", indices, d.Tree, d.TreeStarted, tc.indices, tc.tree, tc.started)
			}
			if d.RootAfter == d.RootBefore {
				t.Errorf("the root before and after is %s, want the leaves to change it", d.RootAfter)
			}
		})
	}
	if d, err := c.leavesIn(ctx, NodeVault, testVault, txHash(1), nil); err == nil || !strings.Contains(err.Error(), "trees 0 and 1") {
		t.Errorf("leavesIn of the transaction inserting into two trees = %+v, %v; want an error naming trees 0 and 1", d, err)
	}
}

// A reorg replaces a block the engine has read: the cached leaves no longer
// end where the vault does, so the next call reads every block again and
// returns the leaves the chain now holds.
func TestVaultLeavesReadEveryBlockAgainAfterAReorg(t *testing.T) {
	a, b, b2 := big.NewInt(1), big.NewInt(2), big.NewInt(3)
	node := &fakeNode{tip: 5, leaves: []ethtypes.Log{leafInserted(0, 0, a, 2, txHash(0)), leafInserted(0, 1, b, 3, txHash(1))}}
	c := treeChain(t, node)
	ctx := context.Background()
	if got, err := c.vaultLeaves(ctx, testVault); err != nil || len(got) != 2 {
		t.Fatalf("vaultLeaves = %d leaves, %v; want 2", len(got), err)
	}

	node.leaves[1] = leafInserted(0, 1, b2, 3, txHash(2))
	node.tip = 6
	node.queries = nil
	got, err := c.vaultLeaves(ctx, testVault)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1].Leaf.Cmp(b2) != 0 || got[1].tx != txHash(2) {
		t.Fatalf("after the reorg, leaves %+v, want block 3's new leaf %s second", got, b2)
	}
	if !slices.ContainsFunc(node.queries, func(q logQuery) bool { return q.from == 1 }) {
		t.Errorf("queries %+v, want the blocks read again from the deployment's, 1", node.queries)
	}
}

// While the node's logs lag its head, the leaves never end where the vault
// does: the call fails rather than return them. Once the logs catch up, the
// next call returns every leaf.
func TestVaultLeavesFailWhileTheLogsLag(t *testing.T) {
	node := &fakeNode{tip: 5, lagging: 2, leaves: []ethtypes.Log{
		leafInserted(0, 0, big.NewInt(1), 2, txHash(0)),
		leafInserted(0, 1, big.NewInt(2), 3, txHash(1)),
		leafInserted(0, 2, big.NewInt(3), 4, txHash(2)),
	}}
	c := treeChain(t, node)
	ctx := context.Background()
	got, err := c.vaultLeaves(ctx, testVault)
	if err == nil || !strings.Contains(err.Error(), "read twice, rebuild other trees") {
		t.Fatalf("vaultLeaves with lagging logs = %d leaves, %v; want an error", len(got), err)
	}
	if got, err := c.vaultLeaves(ctx, testVault); err != nil || len(got) != 3 {
		t.Fatalf("vaultLeaves once the logs caught up = %d leaves, %v; want all 3", len(got), err)
	}
}

// A block mined after the engine read the head holds a leaf. Read at that
// head, the vault's tree and root are those of the leaves the engine read
// up to it, so the call returns them rather than fail on the newer state.
func TestVaultLeavesReadTheVaultAtTheHeadTheLogsEndAt(t *testing.T) {
	node := &fakeNode{tip: 5, leaves: []ethtypes.Log{
		leafInserted(0, 0, big.NewInt(1), 2, txHash(0)),
		leafInserted(0, 1, big.NewInt(2), 3, txHash(1)),
		leafInserted(0, 2, big.NewInt(3), 6, txHash(2)),
	}}
	c := treeChain(t, node)
	if got, err := c.vaultLeaves(context.Background(), testVault); err != nil || len(got) != 2 {
		t.Fatalf("vaultLeaves = %d leaves, %v; want the 2 in blocks up to the head, 5", len(got), err)
	}
}

// A vault on a later tree than its leaves build, with the same root, is no
// match either: the call reads every block again, then fails rather than
// return leaves that end on another tree.
func TestVaultLeavesCompareTheTreeNumber(t *testing.T) {
	node := &fakeNode{tip: 5, treeShift: 1, leaves: []ethtypes.Log{leafInserted(0, 0, big.NewInt(1), 2, txHash(0))}}
	c := treeChain(t, node)
	if got, err := c.vaultLeaves(context.Background(), testVault); err == nil || !strings.Contains(err.Error(), "read twice, rebuild other trees") {
		t.Fatalf("vaultLeaves with the vault one tree on = %d leaves, %v; want an error", len(got), err)
	}
}

// A node that returns a LeafInserted log twice gives leaves out of
// sequence: they rebuild no tree, so the call fails rather than return them,
// even though the leaves it does place end where the vault does.
func TestVaultLeavesRefuseARepeatedLog(t *testing.T) {
	node := &fakeNode{tip: 5, repeat: true, leaves: []ethtypes.Log{
		leafInserted(0, 0, big.NewInt(1), 2, txHash(0)),
		leafInserted(0, 1, big.NewInt(2), 3, txHash(1)),
	}}
	c := treeChain(t, node)
	if got, err := c.vaultLeaves(context.Background(), testVault); err == nil || !strings.Contains(err.Error(), "read twice, rebuild other trees") {
		t.Fatalf("vaultLeaves with a log repeated = %d leaves, %v; want an error", len(got), err)
	}
}
