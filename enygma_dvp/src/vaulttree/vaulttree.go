// Package vaulttree rebuilds a vault's Merkle trees from its LeafInserted
// events, each leaf at the tree and index the vault put it, and checks them
// against the vault's own tree number and root.
package vaulttree

import (
	"context"
	"fmt"
	"math"
	"math/big"

	"github.com/raylsnetwork/enygma_dvp/src/core"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// LeafInsertedTopic is the topic of the LeafInserted event a vault's
// insertLeaves logs for each leaf: the leaf, and the tree and index it went
// to. The vault decides when to start a new tree once per insertLeaves
// call, and one transaction can make several calls, so a leaf's position
// is read from this event, not worked out from its Commitment event.
var LeafInsertedTopic = crypto.Keccak256Hash([]byte("LeafInserted(uint256,uint256,uint256,uint256)"))

// Position is a leaf a vault inserted and where it went: the number of the
// tree and the leaf's index there.
type Position struct {
	Tree  int
	Index int
	Leaf  *big.Int
}

// Parse reads a LeafInserted log: its leaf (topic 2), tree number and leaf
// index (its data). A log of any other shape is an error.
func Parse(l types.Log) (Position, error) {
	if len(l.Topics) != 3 || l.Topics[0] != LeafInsertedTopic {
		return Position{}, fmt.Errorf("log %d of tx %s is not a LeafInserted event", l.Index, l.TxHash.Hex())
	}
	if len(l.Data) != 64 {
		return Position{}, fmt.Errorf("LeafInserted log %d of tx %s has %d data bytes, want 64", l.Index, l.TxHash.Hex(), len(l.Data))
	}
	tree, index := new(big.Int).SetBytes(l.Data[:32]), new(big.Int).SetBytes(l.Data[32:])
	if !tree.IsInt64() || tree.Int64() > math.MaxInt32 || !index.IsInt64() || index.Int64() > math.MaxInt32 {
		return Position{}, fmt.Errorf("LeafInserted log %d of tx %s names tree %s index %s", l.Index, l.TxHash.Hex(), tree, index)
	}
	return Position{Tree: int(tree.Int64()), Index: int(index.Int64()), Leaf: l.Topics[2].Big()}, nil
}

// Rebuild rebuilds a vault's Merkle trees, of depth depth, from its
// LeafInserted logs in chain order, each leaf at the position its event
// names (MerkleTree.InsertAt). It fails on a log it cannot read or a
// position out of sequence, which a missing or reordered log gives.
func Rebuild(depth int, logs []types.Log) (*core.MerkleTree, error) {
	mt := core.NewMerkleTree(depth)
	for _, l := range logs {
		p, err := Parse(l)
		if err != nil {
			return nil, err
		}
		if err := mt.InsertAt(p.Tree, p.Index, p.Leaf); err != nil {
			return nil, fmt.Errorf("block %d tx %s: %w", l.BlockNumber, l.TxHash.Hex(), err)
		}
	}
	return mt, nil
}

// Chain is what Load reads a vault through: its logs, its views and the
// chain's head. An ethclient.Client is one.
type Chain interface {
	ethereum.LogFilterer
	ethereum.ContractCaller
	ethereum.BlockNumberReader
}

var (
	treeNumberSelector  = crypto.Keccak256([]byte("treeNumber()"))[:4]
	currentRootSelector = crypto.Keccak256([]byte("currentRoot()"))[:4]
)

// Current reads the vault at address vault at block at (nil: the latest
// block): its treeNumber() and currentRoot(), the number and root of its
// current tree.
func Current(ctx context.Context, caller ethereum.ContractCaller, vault common.Address, at *big.Int) (int, *big.Int, error) {
	view := func(name string, selector []byte) (*big.Int, error) {
		out, err := caller.CallContract(ctx, ethereum.CallMsg{To: &vault, Data: selector}, at)
		if err != nil {
			return nil, fmt.Errorf("vault %s %s: %w", vault.Hex(), name, err)
		}
		if len(out) != 32 {
			return nil, fmt.Errorf("vault %s %s returned %d bytes, want 32", vault.Hex(), name, len(out))
		}
		return new(big.Int).SetBytes(out), nil
	}
	tree, err := view("treeNumber()", treeNumberSelector)
	if err != nil {
		return 0, nil, err
	}
	if !tree.IsInt64() || tree.Int64() > math.MaxInt32 {
		return 0, nil, fmt.Errorf("vault %s treeNumber() is %s", vault.Hex(), tree)
	}
	root, err := view("currentRoot()", currentRootSelector)
	if err != nil {
		return 0, nil, err
	}
	return int(tree.Int64()), root, nil
}

// Load rebuilds the Merkle trees, of depth depth, of the vault at vault
// from its LeafInserted events up to block at (nil: the chain's head), and
// checks them against the vault at that block: its treeNumber() and
// currentRoot() must be the rebuilt current tree's, so a rebuild that
// missed a leaf fails here rather than in a proof the vault refuses. It
// asks for every block from 0 in one eth_getLogs, as a local node allows.
func Load(ctx context.Context, chain Chain, vault common.Address, depth int, at *big.Int) (*core.MerkleTree, error) {
	if at == nil {
		head, err := chain.BlockNumber(ctx)
		if err != nil {
			return nil, fmt.Errorf("block number: %w", err)
		}
		at = new(big.Int).SetUint64(head)
	}
	logs, err := chain.FilterLogs(ctx, ethereum.FilterQuery{
		FromBlock: big.NewInt(0),
		ToBlock:   at,
		Addresses: []common.Address{vault},
		Topics:    [][]common.Hash{{LeafInsertedTopic}},
	})
	if err != nil {
		return nil, fmt.Errorf("vault %s LeafInserted events: %w", vault.Hex(), err)
	}
	mt, err := Rebuild(depth, logs)
	if err != nil {
		return nil, fmt.Errorf("vault %s: %w", vault.Hex(), err)
	}
	tree, root, err := Current(ctx, chain, vault, at)
	if err != nil {
		return nil, err
	}
	if tree != mt.TreeNumber() || root.Cmp(mt.Root()) != 0 {
		return nil, fmt.Errorf("vault %s at block %s is on tree %d with root %s, but its %d LeafInserted events rebuild tree %d with root %s",
			vault.Hex(), at, tree, root, len(logs), mt.TreeNumber(), mt.Root())
	}
	return mt, nil
}
