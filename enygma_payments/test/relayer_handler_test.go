package enygma_test

// HTTP handler tests for the relayer server.
// Uses mock contract + mock miner injected via server.NewHandlerWithDeps.
// No running chain or gnark server required.
//
// Run:
//
//	cd enygma_payments/test && go test -run TestRelayHandler -v

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"enygma_payments_relayer/server"

	"github.com/ethereum/go-ethereum/core/types"
)

// ── bearerAuth middleware ─────────────────────────────────────────────────────

func TestRelayHandler_Auth_MissingHeader(t *testing.T) {
	r := server.NewWithHandler(testAPIKey, newTestHandler(&mockContract{}, &mockMiner{}))
	req := httptest.NewRequest(http.MethodPost, "/relay/transfer", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", w.Code)
	}
}

func TestRelayHandler_Auth_WrongToken(t *testing.T) {
	r := server.NewWithHandler(testAPIKey, newTestHandler(&mockContract{}, &mockMiner{}))
	req := httptest.NewRequest(http.MethodPost, "/relay/transfer", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("got %d, want 403", w.Code)
	}
}

func TestRelayHandler_Auth_BadScheme(t *testing.T) {
	r := server.NewWithHandler(testAPIKey, newTestHandler(&mockContract{}, &mockMiner{}))
	req := httptest.NewRequest(http.MethodPost, "/relay/transfer", nil)
	req.Header.Set("Authorization", "Basic "+testAPIKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", w.Code)
	}
}

// ── /health ───────────────────────────────────────────────────────────────────

func TestRelayHandler_Health(t *testing.T) {
	r := server.NewWithHandler(testAPIKey, newTestHandler(&mockContract{}, &mockMiner{}))
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != "ok" {
		t.Errorf("status: got %q, want ok", body["status"])
	}
}

// ── /relay/info ───────────────────────────────────────────────────────────────

func TestRelayHandler_Info(t *testing.T) {
	r := server.NewWithHandler(testAPIKey, newTestHandler(&mockContract{}, &mockMiner{}))
	req := httptest.NewRequest(http.MethodGet, "/relay/info", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}
	var body server.InfoResponse
	json.Unmarshal(w.Body.Bytes(), &body)
	if body.ContractAddr != "0x1234567890123456789012345678901234567890" {
		t.Errorf("ContractAddr: got %q", body.ContractAddr)
	}
	if body.ChainID != 1337 {
		t.Errorf("ChainID: got %d, want 1337", body.ChainID)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func serveHTTPPost(r interface{ ServeHTTP(http.ResponseWriter, *http.Request) }, path, apiKey string, body interface{}) *httptest.ResponseRecorder {
	data, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ── /relay/transfer ───────────────────────────────────────────────────────────

func TestRelayHandler_Transfer_Success(t *testing.T) {
	tx := dummyTx()
	r := server.NewWithHandler(testAPIKey, newTestHandler(&mockContract{tx: tx}, &mockMiner{receipt: successReceipt(tx)}))
	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, validTransferBody())
	if w.Code != http.StatusOK {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	var resp server.RelayResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.BlockNumber != 42 {
		t.Errorf("BlockNumber: got %d, want 42", resp.BlockNumber)
	}
	if resp.GasUsed != 21000 {
		t.Errorf("GasUsed: got %d, want 21000", resp.GasUsed)
	}
	if resp.TxHash == "" {
		t.Error("TxHash is empty")
	}
}

func TestRelayHandler_Transfer_InvalidJSON(t *testing.T) {
	r := server.NewWithHandler(testAPIKey, newTestHandler(&mockContract{}, &mockMiner{}))
	req := httptest.NewRequest(http.MethodPost, "/relay/transfer", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestRelayHandler_Transfer_InvalidProof(t *testing.T) {
	body := validTransferBody()
	body.Proof[2] = "not-a-number"
	r := server.NewWithHandler(testAPIKey, newTestHandler(&mockContract{}, &mockMiner{}))
	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, body)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestRelayHandler_Transfer_InvalidCommitment(t *testing.T) {
	body := validTransferBody()
	body.Commitments[1] = []string{"bad", "1"}
	r := server.NewWithHandler(testAPIKey, newTestHandler(&mockContract{}, &mockMiner{}))
	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, body)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestRelayHandler_Transfer_TooManyPublicSignals(t *testing.T) {
	body := validTransferBody()
	body.PublicSignal = make([]string, 51)
	for i := range body.PublicSignal {
		body.PublicSignal[i] = "1"
	}
	r := server.NewWithHandler(testAPIKey, newTestHandler(&mockContract{}, &mockMiner{}))
	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, body)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestRelayHandler_Transfer_InvalidPublicSignal(t *testing.T) {
	body := validTransferBody()
	body.PublicSignal[3] = "not-a-number"
	r := server.NewWithHandler(testAPIKey, newTestHandler(&mockContract{}, &mockMiner{}))
	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, body)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestRelayHandler_Transfer_EmptyPublicSignals(t *testing.T) {
	// Empty signal slice (encodes as []) should zero-pad to [50]*big.Int.
	tx := dummyTx()
	body := validTransferBody()
	body.PublicSignal = []string{}
	r := server.NewWithHandler(testAPIKey, newTestHandler(&mockContract{tx: tx}, &mockMiner{receipt: successReceipt(tx)}))
	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, body)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
}

func TestRelayHandler_Transfer_ContractError(t *testing.T) {
	r := server.NewWithHandler(testAPIKey, newTestHandler(
		&mockContract{err: fmt.Errorf("nonce too low")},
		&mockMiner{},
	))
	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, validTransferBody())
	if w.Code != http.StatusInternalServerError {
		t.Errorf("got %d, want 500", w.Code)
	}
}

func TestRelayHandler_Transfer_TxReverted(t *testing.T) {
	tx := dummyTx()
	r := server.NewWithHandler(testAPIKey, newTestHandler(
		&mockContract{tx: tx},
		&mockMiner{receipt: &types.Receipt{
			Status:      types.ReceiptStatusFailed,
			TxHash:      tx.Hash(),
			BlockNumber: big.NewInt(5),
			GasUsed:     21000,
		}},
	))
	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, validTransferBody())
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400 (reverted)", w.Code)
	}
}

// ── Deduplication ─────────────────────────────────────────────────────────────

func TestRelayHandler_Transfer_Deduplication(t *testing.T) {
	tx := dummyTx()
	h := newTestHandler(&mockContract{tx: tx}, &mockMiner{receipt: successReceipt(tx)})
	r := server.NewWithHandler(testAPIKey, h)

	body := validTransferBody()
	// Pre-seed the in-flight map to simulate a concurrent identical request.
	h.SetInFlight("transfer:"+body.Proof[0], struct{}{})
	defer h.DeleteInFlight("transfer:" + body.Proof[0])

	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, body)
	if w.Code != http.StatusConflict {
		t.Errorf("got %d, want 409", w.Code)
	}
}
