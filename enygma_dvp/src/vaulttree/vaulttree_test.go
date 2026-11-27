package vaulttree

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/raylsnetwork/enygma_dvp/src/core"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
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

// treeOf is the trees calls build, one InsertLeaves batch per call, as the
// vault builds them.
func treeOf(depth int, calls ...[]*big.Int) *core.MerkleTree {
	mt := core.NewMerkleTree(depth)
	for _, call := range calls {
		mt.InsertLeaves(call)
	}
	return mt
}

var testVault = common.HexToAddress("0xa11")

// leafLog is the LeafInserted log of leaf at (tree, index), in block.
func leafLog(tree, index int, leaf *big.Int, block uint64) types.Log {
	data := make([]byte, 64)
	big.NewInt(int64(tree)).FillBytes(data[:32])
	big.NewInt(int64(index)).FillBytes(data[32:])
	return types.Log{Address: testVault, BlockNumber: block, Data: data,
		Topics: []common.Hash{LeafInsertedTopic, common.BigToHash(big.NewInt(7)), common.BigToHash(leaf)}}
}

// logsOf is the LeafInserted logs of ps, one block each from block 1.
func logsOf(ps []position) []types.Log {
	out := make([]types.Log, len(ps))
	for i, p := range ps {
		out[i] = leafLog(p.tree, p.index, p.leaf, uint64(i+1))
	}
	return out
}

func TestParse(t *testing.T) {
	good := leafLog(3, 5, big.NewInt(77), 1)
	p, err := Parse(good)
	if err != nil || p.Tree != 3 || p.Index != 5 || p.Leaf.Int64() != 77 {
		t.Fatalf("Parse = %+v, %v; want tree 3, index 5, leaf 77", p, err)
	}

	commitment := good
	commitment.Topics = []common.Hash{common.HexToHash("0x01"), good.Topics[1], good.Topics[2]}
	short := good
	short.Data = good.Data[:63]
	twoTopics := good
	twoTopics.Topics = good.Topics[:2]
	huge := good
	huge.Data = append(common.LeftPadBytes([]byte{1, 0, 0, 0, 0}, 32), good.Data[32:]...)
	for name, l := range map[string]types.Log{
		"another event":         commitment,
		"63 data bytes":         short,
		"two topics":            twoTopics,
		"a tree number of 2^32": huge,
	} {
		if p, err := Parse(l); err == nil {
			t.Errorf("%s: Parse = %+v, want an error", name, p)
		}
	}
}

func TestRebuild(t *testing.T) {
	calls := [][]*big.Int{leaves(1, 6), leaves(100, 1), leaves(200, 2)}
	ps := positionsOf(testDepth, calls...)
	mt, err := Rebuild(testDepth, logsOf(ps))
	if err != nil {
		t.Fatal(err)
	}
	want := treeOf(testDepth, calls...)
	if mt.TreeNumber() != 1 || mt.Root().Cmp(want.Root()) != 0 || mt.RootOfPrevTree(0).Cmp(want.RootOfPrevTree(0)) != 0 {
		t.Errorf("rebuilt tree %d root %s, want tree 1 root %s", mt.TreeNumber(), mt.Root(), want.Root())
	}

	gap := logsOf(ps)
	gap = append(gap[:3], gap[4:]...)
	if _, err := Rebuild(testDepth, gap); err == nil || !strings.Contains(err.Error(), "block 5") {
		t.Errorf("Rebuild with a log missing = %v, want an error naming the block of the leaf after the gap", err)
	}
}

// fakeVaultChain serves LeafInserted logs and a vault's (treeNumber,
// currentRoot), recording the block each query asks for; with empty set,
// the views return no data, as an address with no code does.
type fakeVaultChain struct {
	head       uint64
	logs       []types.Log
	tree       int
	root       *big.Int
	empty      bool
	queries    []ethereum.FilterQuery
	callBlocks []*big.Int
}

func (f *fakeVaultChain) BlockNumber(context.Context) (uint64, error) { return f.head, nil }

func (f *fakeVaultChain) FilterLogs(_ context.Context, q ethereum.FilterQuery) ([]types.Log, error) {
	f.queries = append(f.queries, q)
	return f.logs, nil
}

func (f *fakeVaultChain) SubscribeFilterLogs(context.Context, ethereum.FilterQuery, chan<- types.Log) (ethereum.Subscription, error) {
	panic("not used")
}

func (f *fakeVaultChain) CallContract(_ context.Context, call ethereum.CallMsg, block *big.Int) ([]byte, error) {
	f.callBlocks = append(f.callBlocks, block)
	if f.empty {
		return nil, nil
	}
	switch string(call.Data) {
	case string(treeNumberSelector):
		return common.LeftPadBytes(big.NewInt(int64(f.tree)).Bytes(), 32), nil
	case string(currentRootSelector):
		return common.LeftPadBytes(f.root.Bytes(), 32), nil
	}
	panic("unexpected call")
}

// Load reads the vault's LeafInserted events up to one block and returns
// their trees only when the vault, at that block, is on their last tree
// with their root.
func TestLoad(t *testing.T) {
	calls := [][]*big.Int{leaves(1, 6), leaves(100, 1), leaves(200, 2)}
	ps := positionsOf(testDepth, calls...)
	want := treeOf(testDepth, calls...)
	ctx := context.Background()

	t.Run("matching", func(t *testing.T) {
		f := &fakeVaultChain{head: 40, logs: logsOf(ps), tree: 1, root: want.Root()}
		mt, err := Load(ctx, f, testVault, testDepth, big.NewInt(30))
		if err != nil {
			t.Fatal(err)
		}
		if mt.TreeNumber() != 1 || mt.Root().Cmp(want.Root()) != 0 {
			t.Errorf("tree %d root %s, want tree 1 root %s", mt.TreeNumber(), mt.Root(), want.Root())
		}
		q := f.queries[0]
		if len(f.queries) != 1 || q.FromBlock.Sign() != 0 || q.ToBlock.Int64() != 30 || len(q.Addresses) != 1 || q.Addresses[0] != testVault ||
			len(q.Topics) != 1 || len(q.Topics[0]) != 1 || q.Topics[0][0] != LeafInsertedTopic {
			t.Errorf("queries %+v, want one for the vault's LeafInserted events from block 0 to 30", f.queries)
		}
		for _, b := range f.callBlocks {
			if b == nil || b.Int64() != 30 {
				t.Errorf("read the vault at block %v, want block 30, the logs' last", b)
			}
		}
	})

	t.Run("at the head", func(t *testing.T) {
		f := &fakeVaultChain{head: 40, logs: logsOf(ps), tree: 1, root: want.Root()}
		if _, err := Load(ctx, f, testVault, testDepth, nil); err != nil {
			t.Fatal(err)
		}
		if f.queries[0].ToBlock.Int64() != 40 || f.callBlocks[0].Int64() != 40 {
			t.Errorf("logs to block %s, vault read at %s, want both at the head, 40", f.queries[0].ToBlock, f.callBlocks[0])
		}
	})

	for name, tc := range map[string]struct {
		f    *fakeVaultChain
		want string
	}{
		"a leaf missing at the end": {&fakeVaultChain{head: 40, logs: logsOf(ps)[:len(ps)-1], tree: 1, root: want.Root()}, "LeafInserted events rebuild tree 1"},
		"another tree number":       {&fakeVaultChain{head: 40, logs: logsOf(ps), tree: 2, root: want.Root()}, "LeafInserted events rebuild tree 1"},
		"another root":              {&fakeVaultChain{head: 40, logs: logsOf(ps), tree: 1, root: big.NewInt(5)}, "LeafInserted events rebuild tree 1"},
		"no vault there":            {&fakeVaultChain{head: 40, logs: logsOf(ps), empty: true}, "treeNumber() returned 0 bytes, want 32"},
		"a tree number past 2^31":   {&fakeVaultChain{head: 40, logs: logsOf(ps), tree: 1 << 40, root: want.Root()}, "treeNumber() is 1099511627776"},
	} {
		t.Run(name, func(t *testing.T) {
			mt, err := Load(ctx, tc.f, testVault, testDepth, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Load = tree %v, %v; want an error naming %q", mt, err, tc.want)
			}
		})
	}
}
