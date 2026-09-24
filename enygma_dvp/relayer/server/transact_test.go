package server

// transact() must simulate a submission before sending it. These tests run it
// against a mock JSON-RPC node that records which methods were called:
//
//   - a call the node says would revert is refused with errWouldRevert and no
//     transaction is sent (before, the relayer sent it with a fixed 8M gas limit
//     and paid for the revert);
//   - a call that simulates fine is sent, and the simulation ran at the same gas
//     cap the transaction uses, so a pass really means it fits.

import (
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

type mockNode struct {
	mu       sync.Mutex
	calls    []string
	revert   bool
	failWith string // if set, eth_call fails with this message instead of a revert
	callGas  uint64
	sentGas  uint64
}

func (m *mockNode) called(method string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.calls {
		if c == method {
			return true
		}
	}
	return false
}

func (m *mockNode) serve(t *testing.T) *httptest.Server {
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
		m.mu.Lock()
		m.calls = append(m.calls, req.Method)
		m.mu.Unlock()

		reply := func(result interface{}) {
			out, _ := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": req.ID, "result": result})
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(out)
		}
		fail := func(msg string) {
			out, _ := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": req.ID,
				"error": map[string]interface{}{"code": 3, "message": msg}})
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(out)
		}

		switch req.Method {
		case "eth_chainId":
			reply("0x539")
		case "eth_call":
			var msg struct {
				Gas string `json:"gas"`
			}
			_ = json.Unmarshal(req.Params[0], &msg)
			m.mu.Lock()
			m.callGas = new(big.Int).SetBytes(common.FromHex(msg.Gas)).Uint64()
			m.mu.Unlock()
			if m.failWith != "" {
				fail(m.failWith)
				return
			}
			if m.revert {
				fail("execution reverted: InvalidProof()")
				return
			}
			reply("0x")
		case "eth_getTransactionCount":
			reply("0x0")
		case "eth_gasPrice", "eth_maxPriorityFeePerGas":
			reply("0x1")
		case "eth_getBlockByNumber":
			reply(header)
		case "eth_sendRawTransaction":
			var raw string
			_ = json.Unmarshal(req.Params[0], &raw)
			var tx types.Transaction
			if err := tx.UnmarshalBinary(common.FromHex(raw)); err != nil {
				fail("bad tx: " + err.Error())
				return
			}
			m.mu.Lock()
			m.sentGas = tx.Gas()
			m.mu.Unlock()
			reply(tx.Hash().Hex())
		case "eth_getTransactionReceipt":
			reply(map[string]interface{}{
				"transactionHash": zero32, "transactionIndex": "0x0", "blockHash": zero32, "blockNumber": "0x2",
				"cumulativeGasUsed": "0x5208", "gasUsed": "0x5208", "status": "0x1",
				"logs": []interface{}{}, "logsBloom": "0x" + strings.Repeat("00", 256), "type": "0x2",
				"effectiveGasPrice": "0x1", "contractAddress": nil,
			})
		default:
			fail("mock node: unsupported method " + req.Method)
		}
	}))
}

func newTestHandler(t *testing.T, url string, maxGas uint64) *Handler {
	t.Helper()
	client, err := ethclient.Dial(url)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(1337))
	if err != nil {
		t.Fatal(err)
	}
	auth.GasLimit = maxGas
	dvpABI, err := abi.JSON(strings.NewReader(`[{"type":"function","name":"ping","inputs":[],"outputs":[],"stateMutability":"nonpayable"}]`))
	if err != nil {
		t.Fatal(err)
	}
	return &Handler{dvpABI: dvpABI, dvpAddr: common.HexToAddress("0x00000000000000000000000000000000000000d1"), auth: auth, client: client}
}

func TestTransact_DoesNotSendACallThatWouldRevert(t *testing.T) {
	node := &mockNode{revert: true}
	srv := node.serve(t)
	defer srv.Close()
	h := newTestHandler(t, srv.URL, 8_000_000)

	_, err := h.transact("ping")
	var wr *errWouldRevert
	if !errors.As(err, &wr) {
		t.Fatalf("expected errWouldRevert, got: %v", err)
	}
	if node.called("eth_sendRawTransaction") {
		t.Fatal("VULNERABLE: the relayer sent a transaction that the node's simulation said would revert")
	}
}

func TestTransact_SendsAPassingCallAndSimulatesAtTheGasCap(t *testing.T) {
	node := &mockNode{}
	srv := node.serve(t)
	defer srv.Close()
	h := newTestHandler(t, srv.URL, 8_000_000)

	if _, err := h.transact("ping"); err != nil {
		t.Fatalf("transact: %v", err)
	}
	if !node.called("eth_sendRawTransaction") {
		t.Fatal("a call that simulates fine was not sent")
	}
	if node.callGas != 8_000_000 {
		t.Fatalf("simulation ran with gas %d, want the cap 8000000", node.callGas)
	}
	if node.sentGas != 8_000_000 {
		t.Fatalf("sent gas limit = %d, want the cap 8000000", node.sentGas)
	}
}

func TestTransact_RunningOutOfGasInTheSimulationIsRefused(t *testing.T) {
	node := &mockNode{failWith: "out of gas"}
	srv := node.serve(t)
	defer srv.Close()
	h := newTestHandler(t, srv.URL, 8_000_000)

	_, err := h.transact("ping")
	var wr *errWouldRevert
	if !errors.As(err, &wr) {
		t.Fatalf("expected errWouldRevert, got: %v", err)
	}
	if node.called("eth_sendRawTransaction") {
		t.Fatal("sent a transaction whose simulation ran out of gas")
	}
}

func TestTransact_NodeFailureIsNotReportedAsARevert(t *testing.T) {
	// An unreachable node is the relayer's problem (500), not the caller's (422).
	node := &mockNode{}
	srv := node.serve(t)
	h := newTestHandler(t, srv.URL, 8_000_000)
	srv.Close()

	_, err := h.transact("ping")
	if err == nil {
		t.Fatal("expected an error")
	}
	var wr *errWouldRevert
	if errors.As(err, &wr) {
		t.Fatalf("a transport failure was reported as a revert: %v", err)
	}
}
