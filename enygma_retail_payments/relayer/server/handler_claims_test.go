package server

// In-flight claims on the payment routes: one claim per note (scoped by vault,
// shared by every route), all-or-nothing for a two-note request, and held
// past a request's timeout until the transaction is actually mined.

import (
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
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
