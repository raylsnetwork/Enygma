package server

// In-flight claims on the payment routes: one claim per note (scoped by vault,
// shared by every route), all-or-nothing for a two-note request, and held
// past a request's timeout until the transaction is actually mined.

import (
	"bytes"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-gonic/gin"
)

func TestNullifierKey_PerVaultAndSharedAcrossRoutes(t *testing.T) {
	erc20 := common.HexToAddress("0x00000000000000000000000000000000000000a1")
	usdr := common.HexToAddress("0x00000000000000000000000000000000000000b2")
	tree, nf := big.NewInt(0), big.NewInt(42)
	if nullifierKey(erc20, tree, nf) != nullifierKey(erc20, tree, nf) {
		t.Fatal("the same note must map to the same claim on every route")
	}
	if nullifierKey(erc20, tree, nf) == nullifierKey(usdr, tree, nf) {
		t.Fatal("the same tree/nullifier in two vaults are different notes")
	}
}

func TestClaim_AllOrNothing(t *testing.T) {
	h := &Handler{}
	if !h.claim("a") {
		t.Fatal("first claim refused")
	}
	// The same note through another route (e.g. /relay/payment_relayer_fee).
	if h.claim("a") {
		t.Fatal("a note already in flight was claimed again")
	}
	// A two-note request whose second note is taken claims nothing.
	if h.claim("b", "a") {
		t.Fatal("claimed a pair whose second note is in flight")
	}
	if _, held := h.inFlight.Load("b"); held {
		t.Fatal("a refused two-note claim left its first note claimed")
	}
	h.release([]string{"a"})
	if !h.claim("b", "a") {
		t.Fatal("pair refused after its notes were released")
	}
}

// mockReceiptNode is a JSON-RPC node that reports no receipt until mined is set.
func mockReceiptNode(t *testing.T, mined *atomic.Bool) *httptest.Server {
	zero32 := "0x" + strings.Repeat("00", 32)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		_ = json.Unmarshal(body, &req)
		var result interface{}
		if req.Method == "eth_getTransactionReceipt" && mined.Load() {
			result = map[string]interface{}{
				"transactionHash": zero32, "transactionIndex": "0x0", "blockHash": zero32, "blockNumber": "0x2",
				"cumulativeGasUsed": "0x5208", "gasUsed": "0x5208", "status": "0x1",
				"logs": []interface{}{}, "logsBloom": "0x" + strings.Repeat("00", 256), "type": "0x0",
				"effectiveGasPrice": "0x1", "contractAddress": nil,
			}
		}
		out, _ := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": req.ID, "result": result})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(out)
	}))
}

func TestWaitMined_TimeoutAnswersPendingAndHoldsClaims(t *testing.T) {
	var mined atomic.Bool
	srv := mockReceiptNode(t, &mined)
	defer srv.Close()
	client, err := ethclient.Dial(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	h := &Handler{client: client}

	saved := txTimeout
	txTimeout = 300 * time.Millisecond
	defer func() { txTimeout = saved }()

	keys := []string{"nf:vault:0:42"}
	if !h.claim(keys...) {
		t.Fatal("claim refused")
	}
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	tx := types.NewTx(&types.LegacyTx{Nonce: 7, Gas: 21000, GasPrice: big.NewInt(1)})

	if _, ok := h.waitMined(c, tx, keys); ok {
		t.Fatal("waitMined reported a receipt for an unmined transaction")
	}
	if w.Code != http.StatusAccepted || !strings.Contains(w.Body.String(), tx.Hash().Hex()) {
		t.Fatalf("timeout response: %d %s, want 202 with the tx hash", w.Code, w.Body.String())
	}
	time.Sleep(200 * time.Millisecond)
	if h.claim(keys...) {
		t.Fatal("the note was released while its transaction is still pending")
	}

	mined.Store(true) // the transaction lands
	deadline := time.Now().Add(5 * time.Second)
	for !h.claim(keys...) {
		if time.Now().After(deadline) {
			t.Fatal("the note was not released after its transaction was mined")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestWaitMined_MinedReleasesClaims(t *testing.T) {
	var mined atomic.Bool
	mined.Store(true)
	srv := mockReceiptNode(t, &mined)
	defer srv.Close()
	client, err := ethclient.Dial(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	h := &Handler{client: client}
	keys := []string{"nf:vault:0:43"}
	h.claim(keys...)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	if _, ok := h.waitMined(c, types.NewTx(&types.LegacyTx{Nonce: 8}), keys); !ok {
		t.Fatal("waitMined did not return the receipt of a mined transaction")
	}
	if !h.claim(keys...) {
		t.Fatal("the note stayed claimed after its transaction was mined")
	}
}

// sentTxData holds the calldata of the last transaction txNode accepted.
var sentTxData atomic.Value

// txNode is a JSON-RPC node that accepts transactions and reports no receipt
// until mined is set.
func txNode(t *testing.T, mined *atomic.Bool) *httptest.Server {
	zero32 := "0x" + strings.Repeat("00", 32)
	header := map[string]interface{}{
		"parentHash": zero32, "sha3Uncles": zero32, "miner": "0x" + strings.Repeat("00", 20),
		"stateRoot": zero32, "transactionsRoot": zero32, "receiptsRoot": zero32,
		"logsBloom": "0x" + strings.Repeat("00", 256), "difficulty": "0x0", "number": "0x1",
		"gasLimit": "0x1c9c380", "gasUsed": "0x0", "timestamp": "0x1", "extraData": "0x",
		"baseFeePerGas": "0x1", "hash": zero32,
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			ID     json.RawMessage   `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		_ = json.Unmarshal(body, &req)
		var result interface{}
		switch req.Method {
		case "eth_chainId":
			result = "0x539"
		case "eth_blockNumber":
			result = "0x1"
		case "eth_getTransactionCount":
			result = "0x0"
		case "eth_gasPrice", "eth_maxPriorityFeePerGas":
			result = "0x1"
		case "eth_getBlockByNumber":
			result = header
		case "eth_sendRawTransaction":
			var raw string
			_ = json.Unmarshal(req.Params[0], &raw)
			var tx types.Transaction
			_ = tx.UnmarshalBinary(common.FromHex(raw))
			sentTxData.Store(tx.Data())
			result = tx.Hash().Hex()
		case "eth_getTransactionReceipt":
			if mined.Load() {
				result = map[string]interface{}{
					"transactionHash": zero32, "transactionIndex": "0x0", "blockHash": zero32, "blockNumber": "0x2",
					"cumulativeGasUsed": "0x5208", "gasUsed": "0x5208", "status": "0x1",
					"logs": []interface{}{}, "logsBloom": "0x" + strings.Repeat("00", 256), "type": "0x2",
					"effectiveGasPrice": "0x1", "contractAddress": nil,
				}
			}
		}
		out, _ := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": req.ID, "result": result})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(out)
	}))
}

// tagHandler builds a handler whose registries sit on a node that accepts
// transactions and reports no receipt until mined is set.
func tagHandler(t *testing.T, mined *atomic.Bool) (*Handler, *gin.Engine) {
	t.Helper()
	srv := txNode(t, mined)
	t.Cleanup(srv.Close)
	client, err := ethclient.Dial(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(1337))
	if err != nil {
		t.Fatal(err)
	}
	auth.GasLimit = 1_000_000
	tagABI, err := loadABIFromFile("../../private_tags/contracts/TagRegistry.json")
	if err != nil {
		t.Fatal(err)
	}
	chanABI, err := loadABIFromFile("../../private_tags/contracts/TagChannelRegistry.json")
	if err != nil {
		t.Fatal(err)
	}
	h := &Handler{
		client: client, auth: auth,
		tagRegistryAddr:        common.HexToAddress("0x00000000000000000000000000000000000000c3"),
		tagRegistryABI:         tagABI,
		tagChannelRegistryAddr: common.HexToAddress("0x00000000000000000000000000000000000000c4"),
		tagChannelRegistryABI:  chanABI,
	}
	saved := txTimeout
	txTimeout = 300 * time.Millisecond
	t.Cleanup(func() { txTimeout = saved })
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/relay/tag", h.RelayTag)
	r.POST("/relay/channel", h.RelayChannel)
	return h, r
}

func postJSON(r *gin.Engine, path string, v interface{}) *httptest.ResponseRecorder {
	body, _ := json.Marshal(v)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body)))
	return w
}

// A tag publication still unmined when the request times out answers 202 and
// keeps the tag claimed until it is mined: TagRegistry only rejects a repeated
// tag within one block, so a retry in another block would publish it again.
// The claim is on the decoded tag, so another hex spelling of it collides.
func TestRelayTag_TimeoutAnswersPendingAndHoldsClaim(t *testing.T) {
	var mined atomic.Bool
	h, r := tagHandler(t, &mined)
	hexTag := strings.Repeat("ab", 32)

	if w := postJSON(r, "/relay/tag", RelayTagRequest{Tag: "0x" + hexTag, Ctxt: "0x01"}); w.Code != http.StatusAccepted {
		t.Fatalf("unmined publication: got %d, want 202: %s", w.Code, w.Body.String())
	}
	for _, spelling := range []string{"0x" + hexTag, hexTag, "0x" + strings.ToUpper(hexTag)} {
		if w := postJSON(r, "/relay/tag", RelayTagRequest{Tag: spelling, Ctxt: "0x01"}); w.Code != http.StatusConflict {
			t.Fatalf("retry as %q while the first is pending: got %d, want 409", spelling, w.Code)
		}
	}
	mined.Store(true)
	var tag [32]byte
	copy(tag[:], common.FromHex(hexTag))
	deadline := time.Now().Add(5 * time.Second)
	for !h.claim(tagKey(tag)) {
		if time.Now().After(deadline) {
			t.Fatal("the tag stayed claimed after its transaction was mined")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Window mode claims every tag of the window: a shifted window that still
// covers a tag being published collides.
func TestRelayTag_WindowClaimsEveryTag(t *testing.T) {
	var mined atomic.Bool
	_, r := tagHandler(t, &mined)
	tagA, tagB := "0x"+strings.Repeat("a1", 32), "0x"+strings.Repeat("b2", 32)

	// The node reports block 1, so the publication targets block 2 = startBlock.
	if w := postJSON(r, "/relay/tag", RelayTagRequest{Tags: []string{tagA, tagB}, StartBlock: 2, Ctxts: []string{"0x01", "0x02"}}); w.Code != http.StatusAccepted {
		t.Fatalf("unmined window publication: got %d, want 202: %s", w.Code, w.Body.String())
	}
	other := "0x" + strings.Repeat("c3", 32)
	if w := postJSON(r, "/relay/tag", RelayTagRequest{Tags: []string{other, tagB}, StartBlock: 1, Ctxts: []string{"0x01", "0x02"}}); w.Code != http.StatusConflict {
		t.Fatalf("shifted window sharing a pending tag: got %d, want 409", w.Code)
	}
}

// A window tag is decoded and validated before anything is claimed, so a
// caller cannot use it to name (and hold) another claim, such as a note's.
func TestRelayTag_RejectsMalformedWindowTagBeforeClaiming(t *testing.T) {
	var mined atomic.Bool
	h, r := tagHandler(t, &mined)
	noteKey := "nf:0x00000000000000000000000000000000000000a1:0:42"
	w := postJSON(r, "/relay/tag", RelayTagRequest{Tags: []string{noteKey, "0x" + strings.Repeat("ab", 32)}, StartBlock: 3, Ctxts: []string{"0x01", "0x02"}})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("window with a malformed tag: got %d, want 400: %s", w.Code, w.Body.String())
	}
	if _, held := h.inFlight.Load(noteKey); held {
		t.Fatal("a malformed window tag claimed another key")
	}
}

// A channel setup still unmined at the timeout keeps its c1 claimed; the claim
// is on the decoded c1, so another hex spelling of it collides.
func TestRelayChannel_TimeoutHoldsClaimForAnySpelling(t *testing.T) {
	var mined atomic.Bool
	_, r := tagHandler(t, &mined)
	c1 := strings.Repeat("cd", 1088)
	req := RelayChannelRequest{C1: "0x" + c1, C2: "0x01", Bitmap: "0x01"}
	if w := postJSON(r, "/relay/channel", req); w.Code != http.StatusAccepted {
		t.Fatalf("unmined channel setup: got %d, want 202: %s", w.Code, w.Body.String())
	}
	req.C1 = strings.ToUpper(c1)
	if w := postJSON(r, "/relay/channel", req); w.Code != http.StatusConflict {
		t.Fatalf("same c1 in another spelling while pending: got %d, want 409", w.Code)
	}
}

// The payload is under a per-block key, so in window mode the relayer must
// publish the ctxt for the same block as the tag it picks: ctxts[i] with tags[i].
func TestRelayTag_WindowPublishesTheCtxtOfTheChosenBlock(t *testing.T) {
	var mined atomic.Bool
	h, r := tagHandler(t, &mined)
	tags := []string{"0x" + strings.Repeat("a1", 32), "0x" + strings.Repeat("b2", 32), "0x" + strings.Repeat("c3", 32)}
	ctxts := []string{"0x" + strings.Repeat("11", 40), "0x" + strings.Repeat("22", 40), "0x" + strings.Repeat("33", 40)}

	// The node reports block 1, so the publication targets block 2: index 1.
	if w := postJSON(r, "/relay/tag", RelayTagRequest{Tags: tags, StartBlock: 1, Ctxts: ctxts}); w.Code != http.StatusAccepted {
		t.Fatalf("window publication: got %d, want 202: %s", w.Code, w.Body.String())
	}
	data, _ := sentTxData.Load().([]byte)
	args, err := h.tagRegistryABI.Methods["publishTag"].Inputs.Unpack(data[4:])
	if err != nil {
		t.Fatalf("decode publishTag call: %v", err)
	}
	gotTag, gotCtxt := args[0].([32]byte), args[1].([]byte)
	if common.Bytes2Hex(gotTag[:]) != strings.Repeat("b2", 32) {
		t.Fatalf("published tag %x, want tags[1]", gotTag)
	}
	if common.Bytes2Hex(gotCtxt) != strings.Repeat("22", 40) {
		t.Fatalf("published ctxt %x, want ctxts[1] (the chosen block's)", gotCtxt)
	}
}

// Window mode needs one ctxt per tag; single mode takes exactly one ctxt.
func TestRelayTag_CtxtsMustMatchTheMode(t *testing.T) {
	var mined atomic.Bool
	_, r := tagHandler(t, &mined)
	tagA, tagB := "0x"+strings.Repeat("a1", 32), "0x"+strings.Repeat("b2", 32)
	for name, req := range map[string]RelayTagRequest{
		"window with a single ctxt":      {Tags: []string{tagA, tagB}, StartBlock: 2, Ctxt: "0x01"},
		"window with too few ctxts":      {Tags: []string{tagA, tagB}, StartBlock: 2, Ctxts: []string{"0x01"}},
		"window with a malformed ctxt":   {Tags: []string{tagA, tagB}, StartBlock: 2, Ctxts: []string{"0x01", "zz"}},
		"single tag with ctxts":          {Tag: tagA, Ctxts: []string{"0x01"}},
		"single tag with no ctxt at all": {Tag: tagA},
	} {
		if w := postJSON(r, "/relay/tag", req); w.Code != http.StatusBadRequest {
			t.Errorf("%s: got %d, want 400: %s", name, w.Code, w.Body.String())
		}
	}
}
