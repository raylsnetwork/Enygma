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

// A tag publication still unmined when the request times out answers 202 and
// keeps the tag claimed until it is mined: released earlier, a retry would
// publish the same tag again and revert (TagAlreadyExists) at the relayer's cost.
func TestRelayTag_TimeoutAnswersPendingAndHoldsClaim(t *testing.T) {
	var mined atomic.Bool
	srv := txNode(t, &mined)
	defer srv.Close()
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
	h := &Handler{
		client: client, auth: auth,
		tagRegistryAddr: common.HexToAddress("0x00000000000000000000000000000000000000c3"),
		tagRegistryABI:  tagABI,
	}
	saved := txTimeout
	txTimeout = 300 * time.Millisecond
	defer func() { txTimeout = saved }()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/relay/tag", h.RelayTag)
	tag := "0x" + strings.Repeat("ab", 32)
	post := func() *httptest.ResponseRecorder {
		body, _ := json.Marshal(RelayTagRequest{Tag: tag, Ctxt: "0x01"})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/relay/tag", bytes.NewReader(body)))
		return w
	}

	if w := post(); w.Code != http.StatusAccepted {
		t.Fatalf("unmined publication: got %d, want 202: %s", w.Code, w.Body.String())
	}
	if w := post(); w.Code != http.StatusConflict {
		t.Fatalf("retry while the first publication is pending: got %d, want 409", w.Code)
	}
	mined.Store(true)
	deadline := time.Now().Add(5 * time.Second)
	for !h.claim(tag) {
		if time.Now().After(deadline) {
			t.Fatal("the tag stayed claimed after its transaction was mined")
		}
		time.Sleep(100 * time.Millisecond)
	}
}
