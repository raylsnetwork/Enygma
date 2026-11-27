package engine

import (
	"bytes"
	"cmp"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/raylsnetwork/enygma_dvp/src/vaulttree"
	tags "github.com/raylsnetwork/enygma_retail_payments/private_tags/src"
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

var testVault = common.HexToAddress("0x7a0")

// fakeNode answers the RPCs the engine's log queries make:
// eth_blockNumber with tip, and eth_getLogs with the vault's Commitment
// events in vault (those of the queried commitments, if the query names
// any), its LeafInserted events in leaves (for a query naming that event),
// or EnygmaDvp's ChannelPayment events in payments (those whose block_n
// and tag topics the query names, if it names any), each logged in its
// block, within the queried range. It answers the vault's treeNumber() and
// currentRoot() at a block with the trees its leaves up to that block build
// at depth (merkleDepth if unset), and at latest with all of leaves: a leaf
// in a block past tip is in a block mined after the head was read. It adds
// treeShift to the tree number it answers, as from a vault on a later tree
// than its leaves build, with the same root. While
// lagging is above zero, a LeafInserted query gets all of leaves but the
// last, as from a log backend behind its head, and lagging counts down;
// with repeat set, it gets the last of leaves twice. dvpDown fails the
// ChannelPayment queries, all but the first dvpUpFor of them; dvpQueries
// counts them. With maxSpan set it rejects any eth_getLogs spanning more
// than maxSpan blocks, as hosted RPC providers do. queries records every
// eth_getLogs it was asked, rejected or not; any other contract's logs it
// answers with none.
type fakeNode struct {
	tip        uint64
	payments   []ethtypes.Log
	vault      map[common.Hash]uint64
	leaves     []ethtypes.Log
	depth      int
	treeShift  int
	lagging    int
	repeat     bool
	dvpDown    bool
	dvpUpFor   int
	dvpQueries int
	maxSpan    uint64
	queries    []logQuery
}

// vaultTree is the number and root of the tree f's LeafInserted events in
// blocks up to at end on, in trees of depth depth (merkleDepth if unset).
func (f *fakeNode) vaultTree(at uint64) (int, *big.Int, error) {
	var leaves []ethtypes.Log
	for _, l := range f.leaves {
		if l.BlockNumber <= at {
			leaves = append(leaves, l)
		}
	}
	mt, err := vaulttree.Rebuild(cmp.Or(f.depth, merkleDepth), leaves)
	if err != nil {
		return 0, nil, err
	}
	return mt.TreeNumber(), mt.Root(), nil
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

// matches says whether l's topics are among the ones q names at each
// position (a position q names none of matches any topic).
func matches(l ethtypes.Log, topics [][]common.Hash) bool {
	for i, want := range topics {
		if len(want) == 0 {
			continue
		}
		if i >= len(l.Topics) || !slices.Contains(want, l.Topics[i]) {
			return false
		}
	}
	return true
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
	case "eth_call":
		// ethclient sends the calldata as input; older clients as data.
		var call struct {
			To    common.Address `json:"to"`
			Input hexutil.Bytes  `json:"input"`
			Data  hexutil.Bytes  `json:"data"`
		}
		if err := json.Unmarshal(req.Params[0], &call); err != nil || call.To != testVault {
			http.Error(w, fmt.Sprintf("eth_call %s: %v", req.Params[0], err), http.StatusBadRequest)
			return
		}
		if len(call.Input) > 0 {
			call.Data = call.Input
		}
		at := uint64(math.MaxUint64)
		var arg string
		if len(req.Params) > 1 {
			if err := json.Unmarshal(req.Params[1], &arg); err != nil {
				http.Error(w, fmt.Sprintf("eth_call block %s: %v", req.Params[1], err), http.StatusBadRequest)
				return
			}
		}
		if arg != "" && arg != "latest" {
			n, err := hexutil.DecodeUint64(arg)
			if err != nil {
				http.Error(w, fmt.Sprintf("eth_call block %q: %v", arg, err), http.StatusBadRequest)
				return
			}
			at = n
		}
		tree, root, err := f.vaultTree(at)
		if err != nil {
			resp["error"] = map[string]any{"code": -32000, "message": err.Error()}
			break
		}
		switch common.Bytes2Hex(call.Data) {
		case common.Bytes2Hex(crypto.Keccak256([]byte("treeNumber()"))[:4]):
			resp["result"] = hexutil.Bytes(common.BigToHash(big.NewInt(int64(tree + f.treeShift))).Bytes())
		case common.Bytes2Hex(crypto.Keccak256([]byte("currentRoot()"))[:4]):
			resp["result"] = hexutil.Bytes(common.BigToHash(root).Bytes())
		default:
			resp["error"] = map[string]any{"code": -32000, "message": "execution reverted"}
		}
	case "eth_getLogs":
		var q struct {
			Address   []common.Address `json:"address"`
			Topics    [][]common.Hash  `json:"topics"`
			FromBlock string           `json:"fromBlock"`
			ToBlock   string           `json:"toBlock"`
		}
		if err := json.Unmarshal(req.Params[0], &q); err != nil {
			// A single topic or address comes as a string, not a list.
			var one struct {
				Address   common.Address `json:"address"`
				Topics    []any          `json:"topics"`
				FromBlock string         `json:"fromBlock"`
				ToBlock   string         `json:"toBlock"`
			}
			if err := json.Unmarshal(req.Params[0], &one); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			q.Address, q.FromBlock, q.ToBlock = []common.Address{one.Address}, one.FromBlock, one.ToBlock
			for _, t := range one.Topics {
				var at []common.Hash
				switch v := t.(type) {
				case string:
					at = []common.Hash{common.HexToHash(v)}
				case []any:
					for _, h := range v {
						at = append(at, common.HexToHash(h.(string)))
					}
				}
				q.Topics = append(q.Topics, at)
			}
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
		case testDvp:
			f.dvpQueries++
			if f.dvpDown && f.dvpUpFor == 0 {
				resp["error"] = map[string]any{"code": -32000, "message": "node unavailable"}
				break
			}
			if f.dvpUpFor > 0 {
				f.dvpUpFor--
			}
			for _, l := range f.payments {
				if l.BlockNumber >= from && l.BlockNumber <= to && matches(l, q.Topics) {
					logs = append(logs, l)
				}
			}
		case testVault:
			if len(q.Topics) > 0 && slices.Contains(q.Topics[0], vaulttree.LeafInsertedTopic) {
				served := f.leaves
				if f.lagging > 0 {
					f.lagging--
					served = f.leaves[:len(f.leaves)-1]
				}
				if f.repeat {
					served = append(slices.Clip(served), served[len(served)-1])
				}
				for _, l := range served {
					if l.BlockNumber >= from && l.BlockNumber <= to {
						logs = append(logs, l)
					}
				}
				break
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

// paymentLog is the ChannelPayment log of a payment over ch tagged for
// blockN, settled in block mined, publishing commitment cm and ctxt.
func paymentLog(t *testing.T, blockN, mined uint64, ch tags.Channel, cm *big.Int, ctxt []byte) ethtypes.Log {
	t.Helper()
	tag, err := tags.ChannelTag(ch.SharedSecret, blockN, ch.PkSpend)
	if err != nil {
		t.Fatal(err)
	}
	data, err := channelPaymentData().Pack(cm, ctxt)
	if err != nil {
		t.Fatal(err)
	}
	return ethtypes.Log{Address: testDvp, BlockNumber: mined, Data: data, TxHash: common.BigToHash(new(big.Int).SetUint64(blockN)),
		Topics: []common.Hash{channelPaymentTopic, common.BigToHash(erc20VaultId), common.BigToHash(new(big.Int).SetUint64(blockN)), tag}}
}

// sealedNote is ch's sealed note for block opening to a note of amount and salt.
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

// pay is a payment of amount with salt to ch's payee, tagged for blockN and
// settled in that block: its commitment and log.
func pay(t *testing.T, blockN uint64, ch tags.Channel, amount, salt *big.Int) (*big.Int, ethtypes.Log) {
	t.Helper()
	cm, err := rpcore.Erc20CommitmentV2(ch.PkSpend, salt, amount, tokenId)
	if err != nil {
		t.Fatal(err)
	}
	return cm, paymentLog(t, blockN, blockN, ch, cm, sealedNote(t, blockN, ch, amount, salt))
}

// scanSetup is Bob with an attacker's channel and Alice's, an engine whose
// node serves node, and the commitment of the note Alice pays him.
func scanSetup(t *testing.T, node *fakeNode) (*Engine, *Actor, *big.Int) {
	t.Helper()
	spend, err := rpcore.NewSpendKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	attacker, alice := newChannel(t, spend.PublicKey), newChannel(t, spend.PublicKey)
	paid, real := pay(t, 4, alice, big.NewInt(25), big.NewInt(424242))
	// Block 2: junk sealed under no key. Block 3: a well-formed note that
	// opens to another commitment than the one the payment settled.
	// Block 4: Alice's real payment.
	_, other := pay(t, 3, attacker, big.NewInt(999999), big.NewInt(1))
	other.Data, _ = channelPaymentData().Pack(big.NewInt(12345), sealedNote(t, 3, attacker, big.NewInt(999999), big.NewInt(1)))
	node.payments = []ethtypes.Log{
		paymentLog(t, 2, 2, attacker, big.NewInt(1), []byte("junkjunkjunkjunkjunkjunkjunkjunk")),
		other,
		real,
	}
	srv := httptest.NewServer(node)
	t.Cleanup(srv.Close)
	client, err := ethclient.Dial(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	e := &Engine{chain: &chain{client: client, dvpAddr: testDvp, vaultAddr: testVault}}
	bob := &Actor{Name: "Bob", Spend: spend, tags: tagState{in: []tags.Channel{attacker, alice}, lastBlock: 1}}
	return e, bob, paid
}

// A tag channel anyone opened to Bob carries a junk note and a note that
// opens to another commitment than it settled, then Alice pays him: the scan
// skips both bad payments, finds Alice's note, and moves its cursor past all
// three, so later scans don't stop at them again.
func TestScanTagsSkipsBadPayments(t *testing.T) {
	node := &fakeNode{tip: 5}
	e, bob, paid := scanSetup(t, node)

	found, computed, skipped, err := e.scanTags(context.Background(), bob)
	if err != nil {
		t.Fatalf("scan stopped at a bad payment: %v", err)
	}
	if len(found) != 1 || found[0].block != 4 || len(bob.notes) != 1 || bob.notes[0].commitment.Cmp(paid) != 0 {
		t.Fatalf("found %d notes, want only Alice's payment", len(found))
	}
	// Two channels' tags for each of blocks 2 to 5.
	if computed != 8 || skipped != 2 {
		t.Errorf("computed %d tags and skipped %d payments, want 8 and 2", computed, skipped)
	}
	if bob.tags.lastBlock != 5 {
		t.Errorf("cursor at block %d, want 5", bob.tags.lastBlock)
	}

	node.tip = 6
	if _, _, _, err := e.scanTags(context.Background(), bob); err != nil || bob.tags.lastBlock != 6 {
		t.Fatalf("next scan: cursor %d, err %v", bob.tags.lastBlock, err)
	}
}

// Someone with a channel to Bob pays him over it into another vault (the
// USDr vault's notes are token 0 too): his scan skips it as no RaylsERC20
// note, though it opens.
func TestScanTagsSkipsAnotherVaultsPayment(t *testing.T) {
	node := &fakeNode{tip: 5}
	e, bob, paid := scanSetup(t, node)
	_, usdr := pay(t, 5, bob.tags.in[1], big.NewInt(40), big.NewInt(777))
	usdr.Topics[1] = common.BigToHash(usdrVaultId)
	node.payments = append(node.payments, usdr)

	found, _, skipped, err := e.scanTags(context.Background(), bob)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || bob.notes[0].commitment.Cmp(paid) != 0 || skipped != 3 {
		t.Fatalf("found %d notes and skipped %d payments, want only Alice's RaylsERC20 note and 3", len(found), skipped)
	}
}

// An RPC error is not a bad payment: the scan stops and its cursor stays
// put, so the next scan reads Alice's payment again rather than skipping it.
func TestScanTagsKeepsCursorOnRPCError(t *testing.T) {
	node := &fakeNode{tip: 5, dvpDown: true}
	e, bob, _ := scanSetup(t, node)

	if _, _, _, err := e.scanTags(context.Background(), bob); err == nil {
		t.Fatal("scan succeeded with EnygmaDvp's events unreadable")
	}
	if bob.tags.lastBlock != 1 {
		t.Fatalf("cursor moved to block %d on an RPC error, want 1", bob.tags.lastBlock)
	}

	node.dvpDown = false
	found, _, _, err := e.scanTags(context.Background(), bob)
	if err != nil || len(found) != 1 || bob.tags.lastBlock != 5 {
		t.Fatalf("retry found %d notes, cursor %d, err %v; want Alice's note and block 5", len(found), bob.tags.lastBlock, err)
	}
}

// Alice pays Bob twice, and EnygmaDvp's events become unreadable after the
// scan has read the blocks of the first: the failed scan keeps neither
// note, so the retry finds both rather than only the one it had read.
func TestScanTagsKeepsNoNotesOnRPCError(t *testing.T) {
	node := &fakeNode{tip: 10, dvpDown: true, dvpUpFor: 4}
	e, bob, _ := scanSetup(t, node)
	_, late := pay(t, 9, bob.tags.in[1], big.NewInt(30), big.NewInt(515151))
	node.payments = append(node.payments, late)

	if _, _, _, err := e.scanTags(context.Background(), bob); err == nil {
		t.Fatal("scan succeeded with EnygmaDvp's events unreadable")
	}
	if len(bob.notes) != 0 || bob.tags.lastBlock != 1 {
		t.Fatalf("failed scan kept %d notes and moved the cursor to %d, want 0 and 1", len(bob.notes), bob.tags.lastBlock)
	}

	node.dvpDown = false
	found, _, _, err := e.scanTags(context.Background(), bob)
	if err != nil || len(found) != 2 || len(bob.notes) != 2 || bob.tags.lastBlock != 10 {
		t.Fatalf("retry found %d notes, kept %d, cursor %d, err %v; want both of Alice's and block 10", len(found), len(bob.notes), bob.tags.lastBlock, err)
	}
}

// An attacker fills a channel to Bob with payments of junk: the scan asks
// the node once per block_n, however many payments a block holds, never
// once per payment, and still skips every junk note and finds Alice's.
func TestScanTagsAsksOncePerBlock(t *testing.T) {
	node := &fakeNode{tip: 200}
	e, bob, _ := scanSetup(t, node)
	attacker := bob.tags.in[0]
	const junk = 150
	for i := range junk {
		b := uint64(10 + i%20)
		node.payments = append(node.payments, paymentLog(t, b, b, attacker, big.NewInt(int64(1000+i)), sealedNote(t, b, attacker, big.NewInt(7), big.NewInt(int64(1000+i)))))
	}

	found, _, skipped, err := e.scanTags(context.Background(), bob)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0].block != 4 || skipped != junk+2 {
		t.Fatalf("found %d notes and skipped %d payments, want Alice's note and %d", len(found), skipped, junk+2)
	}
	if node.dvpQueries != 199 {
		t.Errorf("scanned blocks 2 to 200 in %d queries, want one per block, 199", node.dvpQueries)
	}
}

// A payment lands up to tags.ChannelPaymentWindow blocks after its block_n:
// Bob's scan of a block_n looks that far, and never further, so no query
// spans more blocks than a node capping eth_getLogs at a window allows. A
// payment that landed late is found under its block_n, not where it landed.
func TestScanTagsLooksOneWindowPastEachBlock(t *testing.T) {
	node := &fakeNode{tip: 400, maxSpan: tags.ChannelPaymentWindow + 1}
	e, bob, _ := scanSetup(t, node)
	alice := bob.tags.in[1]
	amount, salt := big.NewInt(30), big.NewInt(515151)
	cm, err := rpcore.Erc20CommitmentV2(bob.Spend.PublicKey, salt, amount, tokenId)
	if err != nil {
		t.Fatal(err)
	}
	node.payments = append(node.payments, paymentLog(t, 200, 200+tags.ChannelPaymentWindow, alice, cm, sealedNote(t, 200, alice, amount, salt)))

	found, _, _, err := e.scanTags(context.Background(), bob)
	if err != nil {
		t.Fatalf("scan against a node capping eth_getLogs at %d blocks: %v", node.maxSpan, err)
	}
	if len(found) != 2 || found[1].block != 200 || found[1].cmBlock != 200+tags.ChannelPaymentWindow {
		t.Fatalf("found %d notes, want Alice's two, the late one under block_n 200", len(found))
	}
	for _, q := range node.queries {
		if q.rejected {
			t.Fatalf("asked for blocks %d to %d, over a window", q.from, q.to)
		}
	}
}

// Bob scans, then scans again a block later, when Alice has paid him: the
// second scan asks only for the new block_n, not the ones the first scan
// read, and finds her note.
func TestScanTagsReadsEachBlockOnce(t *testing.T) {
	node := &fakeNode{tip: 300}
	e, bob, _ := scanSetup(t, node)
	if _, _, _, err := e.scanTags(context.Background(), bob); err != nil {
		t.Fatal(err)
	}
	_, next := pay(t, 301, bob.tags.in[1], big.NewInt(30), big.NewInt(515151))
	node.tip = 301
	node.payments = append(node.payments, next)
	node.queries = nil

	found, _, _, err := e.scanTags(context.Background(), bob)
	if err != nil || len(found) != 1 || found[0].block != 301 {
		t.Fatalf("second scan found %d notes, err %v; want Alice's, for block 301", len(found), err)
	}
	for _, q := range node.queries {
		if q.addr == testDvp && q.from != 301 {
			t.Errorf("second scan read block_n %d, want only block 301", q.from)
		}
	}
}

// A node whose UserRegistry cannot be read, as a dropped connection would
// leave it: Bob's channel scan and the auditor's fail rather than scan with
// no users, which would skip every record and move Bob's cursor past them.
func TestChannelScansFailWithoutTheUserCount(t *testing.T) {
	getUserCount := crypto.Keccak256([]byte("getUserCount()"))[:4]
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Params []struct {
				Data  hexutil.Bytes `json:"data"`
				Input hexutil.Bytes `json:"input"`
			} `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Params) == 0 {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		resp := map[string]any{"jsonrpc": "2.0", "id": req.ID}
		if call := append(req.Params[0].Input, req.Params[0].Data...); bytes.HasPrefix(call, getUserCount) {
			resp["error"] = map[string]any{"code": -32000, "message": "node unavailable"}
		} else {
			resp["result"] = hexutil.Bytes(common.LeftPadBytes([]byte{0}, 32))
		}
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	t.Cleanup(srv.Close)
	client, err := ethclient.Dial(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	view, err := rpcore.NewViewKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	e := &Engine{
		chain:      &chain{client: client, registryAddr: common.HexToAddress("0x5e"), channelRegistryAddr: common.HexToAddress("0xc4")},
		auditUsers: []auditUser{{name: "Bob", view: view.DecapsKey}},
	}

	if _, err := e.scanChannels(context.Background(), nil, &Actor{Name: "Bob", View: view}); err == nil {
		t.Error("Bob's channel scan succeeded with UserRegistry unreadable")
	}
	if _, err := e.auditChannels(); err == nil {
		t.Error("the auditor's channel scan succeeded with UserRegistry unreadable")
	}
}
