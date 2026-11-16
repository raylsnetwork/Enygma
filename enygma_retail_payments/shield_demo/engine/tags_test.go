package engine

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
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

// fakeNode answers the two RPCs scanTags makes: eth_blockNumber with tip,
// and eth_getLogs with the TagRegistry's logs or the vault's Commitment
// events for the queried commitments in vault. vaultDown fails the vault's,
// all but the first vaultUpFor of them; vaultQueries counts them.
type fakeNode struct {
	tip          uint64
	tagLogs      []ethtypes.Log
	vault        map[common.Hash]bool
	vaultDown    bool
	vaultUpFor   int
	vaultQueries int
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
			Address []common.Address `json:"address"`
			Topics  [][]common.Hash  `json:"topics"`
		}
		if err := json.Unmarshal(req.Params[0], &q); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		logs := []ethtypes.Log{}
		switch q.Address[0] {
		case testTagRegistry:
			logs = f.tagLogs
		case testVault:
			f.vaultQueries++
			if f.vaultDown && f.vaultUpFor == 0 {
				resp["error"] = map[string]any{"code": -32000, "message": "node unavailable"}
				break
			}
			if f.vaultUpFor > 0 {
				f.vaultUpFor--
			}
			for _, cm := range q.Topics[2] {
				if f.vault[cm] {
					logs = append(logs, ethtypes.Log{Address: testVault, Topics: []common.Hash{commitmentTopic, {}, cm}})
				}
			}
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
	node.vault = map[common.Hash]bool{common.BigToHash(paid): true}
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

// Alice pays Bob twice, and the vault's events become unreadable after the
// scan has confirmed her first payment, one commitment per vault query: the
// failed scan keeps neither note, so the retry finds both rather than only
// the one it had not yet reached.
func TestScanTagsKeepsNoNotesOnRPCError(t *testing.T) {
	defer func(n int) { vaultQueryChunk = n }(vaultQueryChunk)
	vaultQueryChunk = 1
	node := &fakeNode{tip: 6, vaultDown: true, vaultUpFor: 2}
	e, bob, _ := scanSetup(t, node)
	alice := bob.tags.in[1]
	amount, salt := big.NewInt(30), big.NewInt(515151)
	paid, err := rpcore.Erc20CommitmentV2(bob.Spend.PublicKey, salt, amount, tokenId)
	if err != nil {
		t.Fatal(err)
	}
	node.tagLogs = append(node.tagLogs, tagLog(t, 5, alice, sealedNote(t, 5, alice, amount, salt)))
	node.vault[common.BigToHash(paid)] = true

	if _, _, _, err := e.scanTags(context.Background(), bob); err == nil {
		t.Fatal("scan succeeded with the vault's events unreadable")
	}
	if len(bob.notes) != 0 || bob.tags.lastBlock != 1 {
		t.Fatalf("failed scan kept %d notes and moved the cursor to %d, want 0 and 1", len(bob.notes), bob.tags.lastBlock)
	}

	node.vaultDown = false
	found, _, _, err := e.scanTags(context.Background(), bob)
	if err != nil || len(found) != 2 || len(bob.notes) != 2 || bob.tags.lastBlock != 6 {
		t.Fatalf("retry found %d notes, kept %d, cursor %d, err %v; want both of Alice's and block 6", len(found), len(bob.notes), bob.tags.lastBlock, err)
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

	// Rescanning the same blocks with 100 commitments a query takes two.
	defer func(n int) { vaultQueryChunk = n }(vaultQueryChunk)
	vaultQueryChunk = 100
	bob.tags.lastBlock, bob.notes, node.vaultQueries = 1, nil, 0
	if found, _, _, err := e.scanTags(context.Background(), bob); err != nil || len(found) != 1 || node.vaultQueries != 2 {
		t.Errorf("chunks of 100: found %d notes in %d vault queries, err %v; want Alice's in 2", len(found), node.vaultQueries, err)
	}
}
