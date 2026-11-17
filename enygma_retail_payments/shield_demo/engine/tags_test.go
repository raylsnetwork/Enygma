package engine

import (
	"cmp"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	tags "github.com/raylsnetwork/enygma_retail_payments/private_tags/src"
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

var (
	testTagRegistry = common.HexToAddress("0x7a6")
	testVault       = common.HexToAddress("0x7a0")
)

// fakeNode answers the two RPCs the engine's log queries make:
// eth_blockNumber with tip, and eth_getLogs with the TagRegistry's logs or
// the vault's Commitment events in vault (those of the queried commitments,
// if the query names any), each logged in its block, within the queried
// range. vaultDown fails the vault's, all but the first
// vaultUpFor of them; vaultQueries counts them. With maxSpan set it rejects
// any eth_getLogs spanning more than maxSpan blocks, as hosted RPC
// providers do. queries records every eth_getLogs it was asked, rejected
// or not; any other contract's logs it answers with none.
type fakeNode struct {
	tip          uint64
	tagLogs      []ethtypes.Log
	vault        map[common.Hash]uint64
	vaultDown    bool
	vaultUpFor   int
	vaultQueries int
	maxSpan      uint64
	queries      []logQuery
}

// logQuery is one eth_getLogs a fakeNode was asked: its first address, its
// block range, and whether the node rejected the range.
type logQuery struct {
	addr     common.Address
	from, to uint64
	rejected bool
}

// block is a query's fromBlock or toBlock: a number, or latest for the tip.
func (f *fakeNode) block(arg string) (uint64, error) {
	if arg == "" || arg == "latest" {
		return f.tip, nil
	}
	if arg == "earliest" {
		return 0, nil
	}
	return hexutil.DecodeUint64(arg)
}

func (f *fakeNode) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     json.RawMessage   `json:"id"`
		Method string            `json:"method"`
		Params []json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resp := map[string]any{"jsonrpc": "2.0", "id": req.ID}
	switch req.Method {
	case "eth_blockNumber":
		resp["result"] = hexutil.Uint64(f.tip)
	case "eth_getLogs":
		var q struct {
			Address   []common.Address `json:"address"`
			Topics    [][]common.Hash  `json:"topics"`
			FromBlock string           `json:"fromBlock"`
			ToBlock   string           `json:"toBlock"`
		}
		if err := json.Unmarshal(req.Params[0], &q); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		from, err1 := f.block(q.FromBlock)
		to, err2 := f.block(q.ToBlock)
		if err1 != nil || err2 != nil {
			http.Error(w, fmt.Sprintf("block range %q to %q", q.FromBlock, q.ToBlock), http.StatusBadRequest)
			return
		}
		rejected := f.maxSpan > 0 && to-from+1 > f.maxSpan
		f.queries = append(f.queries, logQuery{addr: q.Address[0], from: from, to: to, rejected: rejected})
		if rejected {
			resp["error"] = map[string]any{"code": -32005, "message": fmt.Sprintf("query spans blocks %d to %d, over the %d-block limit", from, to, f.maxSpan)}
			break
		}
		logs := []ethtypes.Log{}
		switch q.Address[0] {
		case testTagRegistry:
			for _, l := range f.tagLogs {
				if l.BlockNumber >= from && l.BlockNumber <= to {
					logs = append(logs, l)
				}
			}
		case testVault:
			f.vaultQueries++
			if f.vaultDown && f.vaultUpFor == 0 {
				resp["error"] = map[string]any{"code": -32000, "message": "node unavailable"}
				break
			}
			if f.vaultUpFor > 0 {
				f.vaultUpFor--
			}
			// A query naming commitments gets only theirs.
			named := map[common.Hash]bool{}
			if len(q.Topics) >= 3 {
				for _, cm := range q.Topics[2] {
					named[cm] = true
				}
			}
			for cm, b := range f.vault {
				if (len(named) == 0 || named[cm]) && b >= from && b <= to {
					logs = append(logs, ethtypes.Log{Address: testVault, BlockNumber: b, Topics: []common.Hash{commitmentTopic, {}, cm}})
				}
			}
			slices.SortFunc(logs, func(a, b ethtypes.Log) int { return cmp.Compare(a.BlockNumber, b.BlockNumber) })
		}
		if resp["error"] == nil {
			resp["result"] = logs
		}
	default:
		resp["error"] = map[string]any{"code": -32601, "message": req.Method}
	}
	json.NewEncoder(w).Encode(resp) //nolint:errcheck
}

// tagLog is the TagPublished log of ch's tag for block carrying payload.
func tagLog(t *testing.T, block uint64, ch tags.Channel, payload []byte) ethtypes.Log {
	t.Helper()
	tag, err := tags.DeriveTag(block, ch.PkSpend, ch.SharedSecret)
	if err != nil {
		t.Fatal(err)
	}
	data, err := abiBytes().Pack(payload)
	if err != nil {
		t.Fatal(err)
	}
	return ethtypes.Log{Address: testTagRegistry, BlockNumber: block, Data: data,
		Topics: []common.Hash{tagPublishedTopic, common.BigToHash(new(big.Int).SetUint64(block)), tag, {}}}
}

// sealedNote is ch's payload for block opening to a note of amount and salt.
func sealedNote(t *testing.T, block uint64, ch tags.Channel, amount, salt *big.Int) []byte {
	t.Helper()
	pt := make([]byte, 96)
	amount.FillBytes(pt[0:32])
	tokenId.FillBytes(pt[32:64])
	salt.FillBytes(pt[64:96])
	ct, err := tags.EncryptPayload(tags.DeriveBlockKey(ch.SharedSecret, block, ch.PkSpend), pt)
	if err != nil {
		t.Fatal(err)
	}
	return ct
}

func newChannel(t *testing.T, pk *big.Int) tags.Channel {
	t.Helper()
	ss := make([]byte, 32)
	if _, err := rand.Read(ss); err != nil {
		t.Fatal(err)
	}
	return tags.Channel{SharedSecret: ss, PkSpend: pk}
}

// scanSetup is Bob with an attacker's channel and Alice's, an engine whose
// node serves node, and the commitment of the note Alice's tag pays him.
func scanSetup(t *testing.T, node *fakeNode) (*Engine, *Actor, *big.Int) {
	t.Helper()
	spend, err := rpcore.NewSpendKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	attacker, alice := newChannel(t, spend.PublicKey), newChannel(t, spend.PublicKey)
	amount, salt := big.NewInt(25), big.NewInt(424242)
	paid, err := rpcore.Erc20CommitmentV2(spend.PublicKey, salt, amount, tokenId)
	if err != nil {
		t.Fatal(err)
	}
	// Block 2: a junk tag. Block 3: a well-formed note not in the vault.
	// Block 4: Alice's real payment.
	node.tagLogs = []ethtypes.Log{
		tagLog(t, 2, attacker, []byte("junkjunkjunkjunkjunkjunkjunkjunk")),
		tagLog(t, 3, attacker, sealedNote(t, 3, attacker, big.NewInt(999999), big.NewInt(1))),
		tagLog(t, 4, alice, sealedNote(t, 4, alice, amount, salt)),
	}
	node.vault = map[common.Hash]uint64{common.BigToHash(paid): 4}
	srv := httptest.NewServer(node)
	t.Cleanup(srv.Close)
	client, err := ethclient.Dial(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	e := &Engine{chain: &chain{client: client, tagRegistryAddr: testTagRegistry, vaultAddr: testVault}}
	bob := &Actor{Name: "Bob", Spend: spend, tags: tagState{in: []tags.Channel{attacker, alice}, lastBlock: 1}}
	return e, bob, paid
}

// A tag channel anyone opened to Bob carries a junk tag and a note that is
// not in the vault, then Alice pays him: the scan skips both bad tags, finds
// Alice's note, and moves its cursor past all three, so later scans don't
// stop at them again.
func TestScanTagsSkipsBadTags(t *testing.T) {
	node := &fakeNode{tip: 5}
	e, bob, paid := scanSetup(t, node)

	found, checked, skipped, err := e.scanTags(context.Background(), bob)
	if err != nil {
		t.Fatalf("scan stopped at a bad tag: %v", err)
	}
	if len(found) != 1 || found[0].block != 4 || len(bob.notes) != 1 || bob.notes[0].commitment.Cmp(paid) != 0 {
		t.Fatalf("found %d notes, want only Alice's payment", len(found))
	}
	if checked != 3 || skipped != 2 {
		t.Errorf("checked %d tags and skipped %d, want 3 and 2", checked, skipped)
	}
	if bob.tags.lastBlock != 5 {
		t.Errorf("cursor at block %d, want 5", bob.tags.lastBlock)
	}

	node.tip = 6
	if _, _, _, err := e.scanTags(context.Background(), bob); err != nil || bob.tags.lastBlock != 6 {
		t.Fatalf("next scan: cursor %d, err %v", bob.tags.lastBlock, err)
	}
}

// An RPC error is not a bad tag: the scan stops and its cursor stays put, so
// the next scan reads Alice's tag again rather than skipping it.
func TestScanTagsKeepsCursorOnRPCError(t *testing.T) {
	node := &fakeNode{tip: 5, vaultDown: true}
	e, bob, _ := scanSetup(t, node)

	if _, _, _, err := e.scanTags(context.Background(), bob); err == nil {
		t.Fatal("scan succeeded with the vault's events unreadable")
	}
	if bob.tags.lastBlock != 1 {
		t.Fatalf("cursor moved to block %d on an RPC error, want 1", bob.tags.lastBlock)
	}

	node.vaultDown = false
	found, _, _, err := e.scanTags(context.Background(), bob)
	if err != nil || len(found) != 1 || bob.tags.lastBlock != 5 {
		t.Fatalf("retry found %d notes, cursor %d, err %v; want Alice's note and block 5", len(found), bob.tags.lastBlock, err)
	}
}

// Alice pays Bob twice, 2,500 blocks apart, and the vault's events become
// unreadable after the scan has read the first 2,000 blocks of them: the
// failed scan keeps neither note, so the retry finds both rather than only
// the one in the blocks it had read.
func TestScanTagsKeepsNoNotesOnRPCError(t *testing.T) {
	node := &fakeNode{tip: 4100, vaultDown: true, vaultUpFor: 1}
	e, bob, _ := scanSetup(t, node)
	alice := bob.tags.in[1]
	amount, salt := big.NewInt(30), big.NewInt(515151)
	paid, err := rpcore.Erc20CommitmentV2(bob.Spend.PublicKey, salt, amount, tokenId)
	if err != nil {
		t.Fatal(err)
	}
	node.tagLogs = append(node.tagLogs, tagLog(t, 2501, alice, sealedNote(t, 2501, alice, amount, salt)))
	node.vault[common.BigToHash(paid)] = 2500

	if _, _, _, err := e.scanTags(context.Background(), bob); err == nil {
		t.Fatal("scan succeeded with the vault's events unreadable")
	}
	if len(bob.notes) != 0 || bob.tags.lastBlock != 1 {
		t.Fatalf("failed scan kept %d notes and moved the cursor to %d, want 0 and 1", len(bob.notes), bob.tags.lastBlock)
	}

	node.vaultDown = false
	found, _, _, err := e.scanTags(context.Background(), bob)
	if err != nil || len(found) != 2 || len(bob.notes) != 2 || bob.tags.lastBlock != 4100 {
		t.Fatalf("retry found %d notes, kept %d, cursor %d, err %v; want both of Alice's and block 4100", len(found), len(bob.notes), bob.tags.lastBlock, err)
	}
}

// An attacker fills a channel to Bob with well-formed notes that are not in
// the vault: the scan confirms all its candidates in one vault query, not
// one per tag, and still skips every junk note and finds Alice's payment.
func TestScanTagsConfirmsInOneVaultQuery(t *testing.T) {
	node := &fakeNode{tip: 200}
	e, bob, _ := scanSetup(t, node)
	attacker := bob.tags.in[0]
	const junk = 150
	for i := range junk {
		b := uint64(10 + i)
		node.tagLogs = append(node.tagLogs, tagLog(t, b, attacker, sealedNote(t, b, attacker, big.NewInt(7), big.NewInt(int64(1000+i)))))
	}

	found, _, skipped, err := e.scanTags(context.Background(), bob)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0].block != 4 || skipped != junk+2 {
		t.Fatalf("found %d notes and skipped %d tags, want Alice's note and %d", len(found), skipped, junk+2)
	}
	if node.vaultQueries != 1 {
		t.Errorf("confirmed %d candidates in %d vault queries, want 1", junk+2, node.vaultQueries)
	}
}

// Bob's node caps eth_getLogs at 10,000 blocks, as hosted RPC providers do,
// and the run is 25,000 blocks long when he scans: Alice paid him at its
// start and again near its tip. The scan finds both notes, never asking
// for more blocks in one query than the node allows, and still skips the
// bad tags. Its candidates take one vault query per 2000 blocks of the
// deployment's blocks 2 to 25000: 13.
func TestScanTagsBoundsEachQuerysBlockRange(t *testing.T) {
	node := &fakeNode{tip: 25_000, maxSpan: 10_000}
	e, bob, _ := scanSetup(t, node)
	e.chain.deployBlock = 2
	alice := bob.tags.in[1]
	amount, salt := big.NewInt(30), big.NewInt(515151)
	late, err := rpcore.Erc20CommitmentV2(bob.Spend.PublicKey, salt, amount, tokenId)
	if err != nil {
		t.Fatal(err)
	}
	node.tagLogs = append(node.tagLogs, tagLog(t, 24_001, alice, sealedNote(t, 24_001, alice, amount, salt)))
	node.vault[common.BigToHash(late)] = 24_000

	found, _, skipped, err := e.scanTags(context.Background(), bob)
	if err != nil {
		t.Fatalf("scan against a node capping eth_getLogs at %d blocks: %v", node.maxSpan, err)
	}
	if len(found) != 2 || found[0].cmBlock != 4 || found[1].cmBlock != 24_000 || skipped != 2 {
		t.Fatalf("found %d notes and skipped %d tags, want both of Alice's, confirmed in blocks 4 and 24000, and 2", len(found), skipped)
	}
	if bob.tags.lastBlock != 25_000 {
		t.Errorf("cursor at block %d, want 25000", bob.tags.lastBlock)
	}
	if node.vaultQueries != 13 {
		t.Errorf("confirmed the candidates in %d vault queries, want 13", node.vaultQueries)
	}
}

// Bob scans once 25,000 blocks into the run and again a block later, when
// Alice has paid him: the second scan reads the vault's new block, not
// the 25,000 the first one read, and finds her note.
func TestScanTagsReadsEachVaultBlockOnce(t *testing.T) {
	node := &fakeNode{tip: 25_000}
	e, bob, _ := scanSetup(t, node)
	e.chain.deployBlock = 2
	if _, _, _, err := e.scanTags(context.Background(), bob); err != nil {
		t.Fatal(err)
	}
	alice := bob.tags.in[1]
	amount, salt := big.NewInt(30), big.NewInt(515151)
	paid, err := rpcore.Erc20CommitmentV2(bob.Spend.PublicKey, salt, amount, tokenId)
	if err != nil {
		t.Fatal(err)
	}
	node.tip = 25_001
	node.tagLogs = append(node.tagLogs, tagLog(t, 25_001, alice, sealedNote(t, 25_001, alice, amount, salt)))
	node.vault[common.BigToHash(paid)] = 25_001
	node.queries = nil

	found, _, _, err := e.scanTags(context.Background(), bob)
	if err != nil || len(found) != 1 || found[0].cmBlock != 25_001 {
		t.Fatalf("second scan found %d notes, err %v; want Alice's, in block 25001", len(found), err)
	}
	for _, q := range node.queries {
		if q.addr == testVault && q.from != 25_001 {
			t.Errorf("second scan read the vault's blocks %d to %d, want only block 25001", q.from, q.to)
		}
	}
}
