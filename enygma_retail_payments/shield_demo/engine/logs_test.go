package engine

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"math/big"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

var (
	testDvp       = common.HexToAddress("0x7b0")
	testUsdrVault = common.HexToAddress("0x7b1")
	testRegistry  = common.HexToAddress("0x7b2")
)

// The node caps eth_getLogs at 10,000 blocks, as hosted RPC providers do,
// and the chain is 25,000 blocks long: the contracts were deployed at block
// 100 and the run started at block 5,000. Each of the engine's log queries
// asks for every block from the first one holding an event it needs (the
// run's first for the auditor's, the deployment's for one that needs the
// contracts' whole history) to the tip, with no query the node rejects.
func TestEveryLogQueryStaysWithinTheNodesBlockLimit(t *testing.T) {
	const deployed, started, tip = 100, 5_000, 25_000
	node := &fakeNode{tip: tip, maxSpan: 10_000}
	srv := httptest.NewServer(node)
	t.Cleanup(srv.Close)
	client, err := ethclient.Dial(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	c := &chain{client: client, deployBlock: deployed, dvpAddr: testDvp, vaultAddr: testVault, usdrVaultAddr: testUsdrVault,
		registryAddr: testRegistry, tagRegistryAddr: testTagRegistry}
	e := &Engine{chain: c, startBlock: started, auditUsers: []auditUser{}}
	spend := testSpendKey(t)
	owner := &Actor{Name: "Bob", Spend: spend}
	ctx := context.Background()

	for _, tc := range []struct {
		name string
		addr common.Address
		from uint64
		run  func() error
	}{
		{"Audit's note events", testDvp, started, func() error { _, err := e.Audit(ctx); return err }},
		{"the auditor's tag query", testTagRegistry, started, func() error {
			_, err := e.tagPayers(ctx, []auditChannel{{Channel: newChannel(t, spend.PublicKey)}})
			return err
		}},
		{"noteEvents", testDvp, deployed, func() error { _, err := c.noteEvents(ctx, testVault, erc20VaultId); return err }},
		{"commitmentBatches", testVault, deployed, func() error { _, err := c.commitmentBatches(ctx, testVault); return err }},
		{"registration", testRegistry, deployed, func() error { _, err := c.registration(ctx, common.HexToAddress("0xb0b")); return err }},
		{"the note encryption card", testDvp, deployed, func() error {
			_, err := c.noteEncryptionFormula(ctx, owner, big.NewInt(7))
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			node.queries = nil
			err := tc.run()
			next := tc.from
			for _, q := range node.queries {
				if q.rejected {
					t.Fatalf("asked for blocks %d to %d, over the node's %d-block limit (err %v)", q.from, q.to, node.maxSpan, err)
				}
				if q.addr != tc.addr {
					continue
				}
				if q.from != next {
					t.Fatalf("asked for blocks %d to %d, want a query from block %d", q.from, q.to, next)
				}
				next = q.to + 1
			}
			if next != tip+1 {
				t.Fatalf("asked for blocks %d to %d of the chain, want %d to %d", tc.from, next-1, tc.from, tip)
			}
		})
	}
}

func testSpendKey(t *testing.T) *rpcore.SpendKeyPair {
	t.Helper()
	spend, err := rpcore.NewSpendKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	return spend
}

// Every eth_getLogs the engine sends goes through logsBetween, which keeps
// each query within logQueryBlocks: a FilterLogs call anywhere else in the
// engine's code could ask for more blocks than a hosted node allows.
func TestOnlyLogsBetweenCallsFilterLogs(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == "logsBetween" {
				return false
			}
			if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "FilterLogs" {
				t.Errorf("%s calls FilterLogs; ask c.logsBetween or c.logsSince for the logs instead", fset.Position(sel.Pos()))
			}
			return true
		})
	}
}
