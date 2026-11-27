package tests

// 35_v2_vault_leaf_positions_test.go
//
// A vault decides when to start a new tree once per insertLeaves call, and
// logs where each leaf went in a LeafInserted event. Rebuilt from those
// positions, the trees have the vault's roots, including when one
// transaction inserts into the vault twice and the second insert starts a
// tree, and when one insert of two leaves starts a tree: there, grouping
// the Commitment events by transaction, or replaying them leaf by leaf,
// gives roots the vault never had.
//
// Runs on go-ethereum's simulated backend (lockEnv) with a real
// Erc20CoinVault and the circomlibjs Poseidon; needs `npx hardhat compile`.

import (
	"context"
	"math/big"
	"testing"

	"github.com/raylsnetwork/enygma_dvp/src/core"
	"github.com/raylsnetwork/enygma_dvp/src/vaulttree"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// leafPosDepth is the depth the vault is initialized with: 256 slots, the
// last never filled.
const leafPosDepth = 8

// leafPosEnv is lockEnv's chain holding a vault whose DvP and auction
// contract is a TwoCallForwarder, so the test makes the vault's inserts
// through it, one or two in a transaction.
type leafPosEnv struct {
	*lockEnv
	vaultABI abi.ABI
	vault    common.Address
	fwdABI   abi.ABI
	fwd      common.Address
}

func newLeafPosEnv(t *testing.T) *leafPosEnv {
	t.Helper()
	e := &leafPosEnv{lockEnv: newLockEnv(t)}
	fABI, fBin := loadArtifact(t, "test/TwoCallForwarder.sol/TwoCallForwarder.json")
	e.fwdABI, e.fwd = fABI, e.deploy(fABI, fBin)
	vABI, vBin := loadArtifact(t, "core/contracts/vaults/Erc20CoinVault.sol/Erc20CoinVault.json")
	e.vaultABI, e.vault = vABI, e.deploy(vABI, vBin, e.fwd)
	e.mine("forward", e.vault, e.pack("initializeVault", big.NewInt(1), big.NewInt(1), common.HexToAddress("0x1"),
		big.NewInt(leafPosDepth), e.wrapper, e.verifier, e.fwd))
	return e
}

// pack is the vault calldata of method with args.
func (e *leafPosEnv) pack(method string, args ...interface{}) []byte {
	e.t.Helper()
	data, err := e.vaultABI.Pack(method, args...)
	if err != nil {
		e.t.Fatalf("pack %s: %v", method, err)
	}
	return data
}

// mine sends the forwarder's method with args in one transaction, mines it,
// and returns its receipt.
func (e *leafPosEnv) mine(method string, args ...interface{}) *types.Receipt {
	e.t.Helper()
	tx, err := bind.NewBoundContract(e.fwd, e.fwdABI, nil, e.backend.Client(), nil).Transact(e.auth, method, args...)
	if err != nil {
		e.t.Fatalf("%s: %v", method, err)
	}
	e.backend.Commit()
	r, err := bind.WaitMined(context.Background(), e.backend.Client(), tx)
	if err != nil || r.Status != types.ReceiptStatusSuccessful {
		e.t.Fatalf("%s: mined %v, status %v", method, err, r.Status)
	}
	return r
}

// register inserts n new leaves from first on into the vault, in calls of at
// most 64, one transaction each.
func (e *leafPosEnv) register(first, n int64) {
	e.t.Helper()
	for n > 0 {
		k := min(n, 64)
		e.mine("forward", e.vault, e.pack("registerCoins", leafPosLeaves(first, k)))
		first, n = first+k, n-k
	}
}

// hadRoot is the vault's isValidRoot(tree, root).
func (e *leafPosEnv) hadRoot(tree int, root *big.Int) bool {
	e.t.Helper()
	return e.call(e.vaultABI, e.vault, "isValidRoot", big.NewInt(int64(tree)), root).(bool)
}

// commitmentLogs is every Commitment event the vault logged, in order.
func (e *leafPosEnv) commitmentLogs() []types.Log {
	e.t.Helper()
	logs, err := e.backend.Client().FilterLogs(context.Background(), ethereum.FilterQuery{
		FromBlock: big.NewInt(0), Addresses: []common.Address{e.vault},
		Topics: [][]common.Hash{{e.vaultABI.Events["Commitment"].ID}},
	})
	if err != nil {
		e.t.Fatal(err)
	}
	return logs
}

// leafPosLeaves is n distinct leaves from first on.
func leafPosLeaves(first, n int64) []*big.Int {
	out := make([]*big.Int, n)
	for i := range out {
		out[i] = big.NewInt(first + int64(i))
	}
	return out
}

// positionsIn is where r's LeafInserted events put each leaf, as
// [tree, index] by leaf.
func positionsIn(t *testing.T, r *types.Receipt) map[int64][2]int {
	t.Helper()
	out := map[int64][2]int{}
	for _, l := range r.Logs {
		if len(l.Topics) == 0 || l.Topics[0] != vaulttree.LeafInsertedTopic {
			continue
		}
		p, err := vaulttree.Parse(*l)
		if err != nil {
			t.Fatal(err)
		}
		out[p.Leaf.Int64()] = [2]int{p.Tree, p.Index}
	}
	return out
}

// perTransaction rebuilds trees from Commitment logs, one batch per
// transaction; leafByLeaf, one leaf at a time. Neither knows the calls.
func perTransaction(logs []types.Log) *core.MerkleTree {
	mt := core.NewMerkleTree(leafPosDepth)
	for i := 0; i < len(logs); {
		var batch []*big.Int
		for j := i; j < len(logs) && logs[j].TxHash == logs[i].TxHash; j++ {
			batch = append(batch, logs[j].Topics[2].Big())
		}
		mt.InsertLeaves(batch)
		i += len(batch)
	}
	return mt
}

func leafByLeaf(logs []types.Log) *core.MerkleTree {
	mt := core.NewMerkleTree(leafPosDepth)
	for _, l := range logs {
		mt.InsertLeaf(l.Topics[2].Big())
	}
	return mt
}

// The subtests run in order on one vault: the second starts where the
// first left it.
func TestV2Vault_LeafPositionsRebuildTreesAcrossBoundaries(t *testing.T) {
	e := newLeafPosEnv(t)
	ctx := context.Background()
	x, y, z := big.NewInt(1_000), big.NewInt(1_001), big.NewInt(1_002)
	u, v := big.NewInt(2_000), big.NewInt(2_001)

	// proves checks that mt proves leaf at (tree, index) with a root the
	// vault has had in that tree.
	proves := func(name string, mt *core.MerkleTree, leaf *big.Int, tree, index int) {
		t.Helper()
		p, err := mt.GenerateProof(leaf)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if p.TreeNumber != tree || p.Indices.Int64() != int64(index) || !e.hadRoot(p.TreeNumber, p.Root) {
			t.Errorf("%s: tree %d index %s, root known %v; want tree %d index %d, a root the vault has had",
				name, p.TreeNumber, p.Indices, e.hadRoot(p.TreeNumber, p.Root), tree, index)
		}
	}
	// strays checks that a rebuild by another rule proves leaf with a root
	// the vault never had: the boundary is one that rule gets wrong.
	strays := func(rule string, mt *core.MerkleTree, leaf *big.Int) {
		t.Helper()
		p, err := mt.GenerateProof(leaf)
		if err != nil {
			t.Fatalf("%s: %v", rule, err)
		}
		if e.hadRoot(p.TreeNumber, p.Root) {
			t.Errorf("rebuilt %s, leaf %s's root in tree %d is one the vault has had: the boundary does not test the positions", rule, leaf, p.TreeNumber)
		}
	}

	t.Run("one transaction, two inserts, the second starting a tree", func(t *testing.T) {
		e.register(1, 253) // tree 0, indices 0 to 252
		r := e.mine("forwardTwice", e.vault, e.pack("registerCoins", []*big.Int{x}), e.pack("registerCoins", []*big.Int{y, z}))
		got := positionsIn(t, r)
		if got[x.Int64()] != [2]int{0, 253} || got[y.Int64()] != [2]int{1, 0} || got[z.Int64()] != [2]int{1, 1} {
			t.Fatalf("LeafInserted put x, y, z at %v, %v, %v; want [0 253], [1 0], [1 1]", got[x.Int64()], got[y.Int64()], got[z.Int64()])
		}

		mt, err := vaulttree.Load(ctx, e.backend.Client(), e.vault, leafPosDepth, nil)
		if err != nil {
			t.Fatal(err)
		}
		proves("x", mt, x, 0, 253)
		proves("y", mt, y, 1, 0)
		proves("z", mt, z, 1, 1)
		strays("one batch per transaction", perTransaction(e.commitmentLogs()), y)
		strays("leaf by leaf", leafByLeaf(e.commitmentLogs()), y)
	})

	t.Run("one insert of two leaves starting a tree", func(t *testing.T) {
		e.register(3_000, 252) // tree 1, indices 2 to 253
		got := positionsIn(t, e.mine("forward", e.vault, e.pack("registerCoins", []*big.Int{u, v})))
		if got[u.Int64()] != [2]int{2, 0} || got[v.Int64()] != [2]int{2, 1} {
			t.Fatalf("LeafInserted put u, v at %v, %v; want [2 0], [2 1]", got[u.Int64()], got[v.Int64()])
		}

		mt, err := vaulttree.Load(ctx, e.backend.Client(), e.vault, leafPosDepth, nil)
		if err != nil {
			t.Fatal(err)
		}
		proves("x", mt, x, 0, 253)
		proves("z", mt, z, 1, 1)
		proves("u", mt, u, 2, 0)
		proves("v", mt, v, 2, 1)
		strays("leaf by leaf", leafByLeaf(e.commitmentLogs()), u)
	})
}
