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
	"strings"
	"testing"
	"time"

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
	if body.MinFee != "0" {
		t.Errorf("MinFee: got %q, want \"0\"", body.MinFee)
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
	body.PublicSignal = make([]string, 81)
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

func TestRelayHandler_Transfer_LocalVerifyFails(t *testing.T) {
	tx := dummyTx()
	c := &mockContract{tx: tx}
	failV := &mockVerifier{err: fmt.Errorf("groth16: invalid proof")}
	h := newTestHandlerWithVerifiers(c, &mockMiner{receipt: successReceipt(tx)}, big.NewInt(0), failV, &mockVerifier{})
	r := server.NewWithHandler(testAPIKey, h)
	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, validTransferBody())
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
	if c.simulateCalled {
		t.Error("SimulateTransfer() was called despite a failed local proof verification — should short-circuit first, no network call")
	}
	if c.transferCalled {
		t.Error("Transfer() was broadcast despite a failed local proof verification")
	}
}

func TestRelayHandler_Transfer_SimulateReverts(t *testing.T) {
	tx := dummyTx()
	c := &mockContract{tx: tx, simErr: fmt.Errorf("execution reverted: InvalidPublicInputs()")}
	r := server.NewWithHandler(testAPIKey, newTestHandler(c, &mockMiner{receipt: successReceipt(tx)}))
	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, validTransferBody())
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
	if c.transferCalled {
		t.Error("Transfer() was broadcast despite a failed dry-run — gas would have been spent on a doomed tx")
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
	h, st := newTestHandlerWithStore(&mockContract{tx: tx}, &mockMiner{receipt: successReceipt(tx)}, 5*time.Minute)
	r := server.NewWithHandler(testAPIKey, h)

	body := validTransferBody()
	// Pre-seed a fresh Pending record to simulate a concurrent identical request.
	if _, _, err := st.TryBeginPending("transfer:"+body.Proof[0], 5*time.Minute); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, body)
	if w.Code != http.StatusConflict {
		t.Errorf("got %d, want 409", w.Code)
	}
}

// ── /relay/transfer_fee ───────────────────────────────────────────────────────

func TestRelayHandler_TransferFee_Success(t *testing.T) {
	tx := dummyTx()
	r := server.NewWithHandler(testAPIKey, newTestHandler(&mockContract{tx: tx}, &mockMiner{receipt: successReceipt(tx)}))
	w := serveHTTPPost(r, "/relay/transfer_fee", testAPIKey, validTransferFeeBody())
	if w.Code != http.StatusOK {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	var resp server.RelayResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.TxHash == "" {
		t.Error("TxHash is empty")
	}
}

func TestRelayHandler_TransferFee_InvalidJSON(t *testing.T) {
	r := server.NewWithHandler(testAPIKey, newTestHandler(&mockContract{}, &mockMiner{}))
	req := httptest.NewRequest(http.MethodPost, "/relay/transfer_fee", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestRelayHandler_TransferFee_WrongSignalCount(t *testing.T) {
	body := validTransferFeeBody()
	body.PublicSignal = body.PublicSignal[:53] // fee circuit requires exactly 54
	r := server.NewWithHandler(testAPIKey, newTestHandler(&mockContract{}, &mockMiner{}))
	w := serveHTTPPost(r, "/relay/transfer_fee", testAPIKey, body)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestRelayHandler_TransferFee_LocalVerifyFails(t *testing.T) {
	tx := dummyTx()
	c := &mockContract{tx: tx}
	failV := &mockVerifier{err: fmt.Errorf("groth16: invalid proof")}
	h := newTestHandlerWithVerifiers(c, &mockMiner{receipt: successReceipt(tx)}, big.NewInt(0), &mockVerifier{}, failV)
	r := server.NewWithHandler(testAPIKey, h)
	w := serveHTTPPost(r, "/relay/transfer_fee", testAPIKey, validTransferFeeBody())
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
	if c.simulateCalled {
		t.Error("SimulateTransferWithFee() was called despite a failed local proof verification — should short-circuit first, no network call")
	}
	if c.transferCalled {
		t.Error("TransferWithFee() was broadcast despite a failed local proof verification")
	}
}

func TestRelayHandler_TransferFee_SimulateReverts(t *testing.T) {
	tx := dummyTx()
	c := &mockContract{tx: tx, simErr: fmt.Errorf("execution reverted: TreasuryNotSet()")}
	r := server.NewWithHandler(testAPIKey, newTestHandler(c, &mockMiner{receipt: successReceipt(tx)}))
	w := serveHTTPPost(r, "/relay/transfer_fee", testAPIKey, validTransferFeeBody())
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
	if c.transferCalled {
		t.Error("TransferWithFee() was broadcast despite a failed dry-run — gas would have been spent on a doomed tx")
	}
}

func TestRelayHandler_TransferFee_ContractError(t *testing.T) {
	r := server.NewWithHandler(testAPIKey, newTestHandler(
		&mockContract{err: fmt.Errorf("nonce too low")},
		&mockMiner{},
	))
	w := serveHTTPPost(r, "/relay/transfer_fee", testAPIKey, validTransferFeeBody())
	if w.Code != http.StatusInternalServerError {
		t.Errorf("got %d, want 500", w.Code)
	}
}

func TestRelayHandler_TransferFee_TxReverted(t *testing.T) {
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
	w := serveHTTPPost(r, "/relay/transfer_fee", testAPIKey, validTransferFeeBody())
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400 (reverted)", w.Code)
	}
}

func TestRelayHandler_TransferFee_Deduplication(t *testing.T) {
	tx := dummyTx()
	h, st := newTestHandlerWithStore(&mockContract{tx: tx}, &mockMiner{receipt: successReceipt(tx)}, 5*time.Minute)
	r := server.NewWithHandler(testAPIKey, h)

	body := validTransferFeeBody()
	if _, _, err := st.TryBeginPending("transfer_fee:"+body.Proof[0], 5*time.Minute); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	w := serveHTTPPost(r, "/relay/transfer_fee", testAPIKey, body)
	if w.Code != http.StatusConflict {
		t.Errorf("got %d, want 409", w.Code)
	}
}

// ── /relay/transfer_fee minimum-fee policy ──────────────────────────────────────

func TestRelayHandler_TransferFee_BelowMinimum(t *testing.T) {
	tx := dummyTx()
	h := newTestHandlerWithMinFee(&mockContract{tx: tx}, &mockMiner{receipt: successReceipt(tx)}, big.NewInt(21))
	r := server.NewWithHandler(testAPIKey, h)

	body := validTransferFeeBody() // fee = 20, below the minimum of 21
	w := serveHTTPPost(r, "/relay/transfer_fee", testAPIKey, body)
	if w.Code != http.StatusPaymentRequired {
		t.Errorf("got %d, want 402", w.Code)
	}
}

func TestRelayHandler_TransferFee_ExactlyMinimum(t *testing.T) {
	tx := dummyTx()
	h := newTestHandlerWithMinFee(&mockContract{tx: tx}, &mockMiner{receipt: successReceipt(tx)}, big.NewInt(20))
	r := server.NewWithHandler(testAPIKey, h)

	body := validTransferFeeBody() // fee = 20, equal to the minimum — should pass
	w := serveHTTPPost(r, "/relay/transfer_fee", testAPIKey, body)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
}

func TestRelayHandler_TransferFee_AboveMinimum(t *testing.T) {
	tx := dummyTx()
	h := newTestHandlerWithMinFee(&mockContract{tx: tx}, &mockMiner{receipt: successReceipt(tx)}, big.NewInt(5))
	r := server.NewWithHandler(testAPIKey, h)

	body := validTransferFeeBody() // fee = 20, above the minimum of 5
	w := serveHTTPPost(r, "/relay/transfer_fee", testAPIKey, body)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
}

func TestRelayHandler_TransferFee_MinFeeDisabledAcceptsZero(t *testing.T) {
	tx := dummyTx()
	// newTestHandler defaults to MinFee=0 (disabled) — even a zero fee must pass.
	h := newTestHandler(&mockContract{tx: tx}, &mockMiner{receipt: successReceipt(tx)})
	r := server.NewWithHandler(testAPIKey, h)

	body := validTransferFeeBody()
	body.PublicSignal[50] = "0"
	w := serveHTTPPost(r, "/relay/transfer_fee", testAPIKey, body)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
}

// ── Rate limiting ────────────────────────────────────────────────────────────

func TestRelayHandler_RateLimit_AllowsWithinBurst(t *testing.T) {
	tx := dummyTx()
	c := &mockContract{tx: tx}
	h := newTestHandlerWithRateLimit(c, &mockMiner{receipt: successReceipt(tx)}, big.NewInt(0),
		&mockVerifier{}, &mockVerifier{}, 1000, 5) // high rps, burst=5
	r := server.NewWithHandler(testAPIKey, h)

	for i := 0; i < 5; i++ {
		w := serveHTTPPost(r, "/relay/transfer", testAPIKey, validTransferBody())
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: got %d, want 200: %s", i, w.Code, w.Body.String())
		}
	}
}

func TestRelayHandler_RateLimit_BlocksOverBurst(t *testing.T) {
	tx := dummyTx()
	c := &mockContract{tx: tx}
	// rps effectively zero (no meaningful refill during the test) — the
	// bucket starts full at burst=2 and never refills within this test's
	// runtime, so the 3rd request deterministically has nothing left.
	h := newTestHandlerWithRateLimit(c, &mockMiner{receipt: successReceipt(tx)}, big.NewInt(0),
		&mockVerifier{}, &mockVerifier{}, 0.0001, 2)
	r := server.NewWithHandler(testAPIKey, h)

	for i := 0; i < 2; i++ {
		w := serveHTTPPost(r, "/relay/transfer", testAPIKey, validTransferBody())
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: got %d, want 200: %s", i, w.Code, w.Body.String())
		}
	}

	callsBefore := c.transferCalls
	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, validTransferBody())
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("got %d, want 429", w.Code)
	}
	if c.transferCalls != callsBefore {
		t.Error("Transfer() was called for a request that should have been rate-limited before reaching the handler")
	}
}

func TestRelayHandler_RateLimit_DisabledAllowsUnlimited(t *testing.T) {
	tx := dummyTx()
	// newTestHandler defaults to RateLimitRPS=0 (disabled).
	h := newTestHandler(&mockContract{tx: tx}, &mockMiner{receipt: successReceipt(tx)})
	r := server.NewWithHandler(testAPIKey, h)

	for i := 0; i < 20; i++ {
		w := serveHTTPPost(r, "/relay/transfer", testAPIKey, validTransferBody())
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: got %d, want 200 (rate limit should be disabled)", i, w.Code)
		}
	}
}

func TestRelayHandler_RateLimit_UnauthenticatedDoesNotConsumeBudget(t *testing.T) {
	tx := dummyTx()
	c := &mockContract{tx: tx}
	h := newTestHandlerWithRateLimit(c, &mockMiner{receipt: successReceipt(tx)}, big.NewInt(0),
		&mockVerifier{}, &mockVerifier{}, 0.0001, 1) // burst=1
	r := server.NewWithHandler(testAPIKey, h)

	// Several failed-auth attempts with the wrong key must not burn the
	// legitimate caller's single token — auth runs before rate limiting.
	for i := 0; i < 5; i++ {
		w := serveHTTPPost(r, "/relay/transfer", "wrong-token", validTransferBody())
		if w.Code != http.StatusForbidden {
			t.Fatalf("unauthenticated request %d: got %d, want 403", i, w.Code)
		}
	}

	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, validTransferBody())
	if w.Code != http.StatusOK {
		t.Fatalf("legitimate request: got %d, want 200: %s", w.Code, w.Body.String())
	}
}

// TestRelayHandler_RateLimit_SchemeCaseDoesNotMultiplyBudget guards against
// keying the limiter on the raw Authorization header: bearerAuth accepts the
// "Bearer" scheme case-insensitively, so if the limiter keyed on the header
// string as a whole, the same caller could get a fresh bucket per case
// variant ("Bearer", "bearer", "BEARER", ...) and multiply its effective
// rate limit. It must key on the validated token instead, so every case
// variant shares one bucket.
func TestRelayHandler_RateLimit_SchemeCaseDoesNotMultiplyBudget(t *testing.T) {
	tx := dummyTx()
	c := &mockContract{tx: tx}
	h := newTestHandlerWithRateLimit(c, &mockMiner{receipt: successReceipt(tx)}, big.NewInt(0),
		&mockVerifier{}, &mockVerifier{}, 0.0001, 1) // burst=1, negligible refill
	r := server.NewWithHandler(testAPIKey, h)

	postWithScheme := func(scheme string) *httptest.ResponseRecorder {
		data, _ := json.Marshal(validTransferBody())
		req := httptest.NewRequest(http.MethodPost, "/relay/transfer", bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", scheme+" "+testAPIKey)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	if w := postWithScheme("Bearer"); w.Code != http.StatusOK {
		t.Fatalf("first request (Bearer): got %d, want 200: %s", w.Code, w.Body.String())
	}
	// Burst is exhausted — a different scheme casing must still hit the same
	// bucket and be rejected, not get its own fresh allowance.
	if w := postWithScheme("bearer"); w.Code != http.StatusTooManyRequests {
		t.Errorf("second request (bearer, lowercase): got %d, want 429 — scheme case must not grant a fresh rate-limit bucket", w.Code)
	}
	if w := postWithScheme("BEARER"); w.Code != http.StatusTooManyRequests {
		t.Errorf("third request (BEARER, uppercase): got %d, want 429 — scheme case must not grant a fresh rate-limit bucket", w.Code)
	}
}

// ── Readiness ─────────────────────────────────────────────────────────────────

func TestRelayHandler_Ready_AllChecksPass(t *testing.T) {
	m := &mockMiner{blockNum: 42, balance: big.NewInt(1_000_000_000_000_000_000)}
	h := newTestHandler(&mockContract{}, m)
	r := server.NewWithHandler(testAPIKey, h)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200: %s", w.Code, w.Body.String())
	}
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if ready, _ := body["ready"].(bool); !ready {
		t.Errorf("ready: got %v, want true: %s", body["ready"], w.Body.String())
	}
}

func TestRelayHandler_Ready_RPCDown(t *testing.T) {
	m := &mockMiner{blockErr: fmt.Errorf("connection refused"), balance: big.NewInt(1)}
	h := newTestHandler(&mockContract{}, m)
	r := server.NewWithHandler(testAPIKey, h)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d, want 503: %s", w.Code, w.Body.String())
	}
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if ready, _ := body["ready"].(bool); ready {
		t.Error("ready: got true, want false when RPC is down")
	}
}

func TestRelayHandler_Ready_BalanceBelowFloor(t *testing.T) {
	m := &mockMiner{blockNum: 42, balance: big.NewInt(5)}
	h := newTestHandlerWithMinBalance(&mockContract{}, m, big.NewInt(1_000_000))
	r := server.NewWithHandler(testAPIKey, h)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d, want 503: %s", w.Code, w.Body.String())
	}
}

func TestRelayHandler_Ready_BalanceCheckDisabledByDefault(t *testing.T) {
	// newTestHandler defaults MinBalanceWei to 0 (disabled) — a near-zero
	// balance must still pass readiness.
	m := &mockMiner{blockNum: 42, balance: big.NewInt(1)}
	h := newTestHandler(&mockContract{}, m)
	r := server.NewWithHandler(testAPIKey, h)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200: %s", w.Code, w.Body.String())
	}
}

// ── Idempotent persistence ───────────────────────────────────────────────────

func TestRelayHandler_Transfer_IdempotentReplayAfterSuccess(t *testing.T) {
	tx := dummyTx()
	c := &mockContract{tx: tx}
	h := newTestHandler(c, &mockMiner{receipt: successReceipt(tx)})
	r := server.NewWithHandler(testAPIKey, h)

	body := validTransferBody()

	w1 := serveHTTPPost(r, "/relay/transfer", testAPIKey, body)
	if w1.Code != http.StatusOK {
		t.Fatalf("first request: got %d: %s", w1.Code, w1.Body.String())
	}
	var resp1 server.RelayResponse
	json.Unmarshal(w1.Body.Bytes(), &resp1)

	w2 := serveHTTPPost(r, "/relay/transfer", testAPIKey, body)
	if w2.Code != http.StatusOK {
		t.Fatalf("replay: got %d, want 200 (cached replay): %s", w2.Code, w2.Body.String())
	}
	var resp2 server.RelayResponse
	json.Unmarshal(w2.Body.Bytes(), &resp2)

	if resp2.TxHash != resp1.TxHash {
		t.Errorf("replay txHash: got %q, want cached %q", resp2.TxHash, resp1.TxHash)
	}
	if c.transferCalls != 1 {
		t.Errorf("Transfer() called %d times, want 1 — replay should not resubmit", c.transferCalls)
	}
}

func TestRelayHandler_Transfer_StaleClaimAllowsRetry(t *testing.T) {
	tx := dummyTx()
	c := &mockContract{tx: tx}
	h, st := newTestHandlerWithStore(c, &mockMiner{receipt: successReceipt(tx)}, 10*time.Millisecond)
	r := server.NewWithHandler(testAPIKey, h)

	body := validTransferBody()
	if _, _, err := st.TryBeginPending("transfer:"+body.Proof[0], 10*time.Millisecond); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	time.Sleep(20 * time.Millisecond) // let the seeded claim go stale

	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, body)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200 (stale claim should be reclaimable): %s", w.Code, w.Body.String())
	}
}

func TestRelayHandler_Transfer_FailedSubmissionAllowsImmediateRetry(t *testing.T) {
	tx := dummyTx()
	c := &mockContract{tx: tx, err: fmt.Errorf("nonce too low")}
	h, _ := newTestHandlerWithStore(c, &mockMiner{receipt: successReceipt(tx)}, 5*time.Minute)
	r := server.NewWithHandler(testAPIKey, h)

	body := validTransferBody()
	w1 := serveHTTPPost(r, "/relay/transfer", testAPIKey, body)
	if w1.Code != http.StatusInternalServerError {
		t.Fatalf("first request: got %d, want 500", w1.Code)
	}

	// Fix the mock so the retry can succeed, then immediately retry the
	// identical request — MarkFailed should have cleared the key, so this
	// must NOT be blocked by the 5-minute staleness window.
	c.err = nil
	w2 := serveHTTPPost(r, "/relay/transfer", testAPIKey, body)
	if w2.Code != http.StatusOK {
		t.Fatalf("retry after failure: got %d, want 200 (failure should clear immediately, not wait out staleness): %s", w2.Code, w2.Body.String())
	}
}

// ── Nonce & gas-bump retry ───────────────────────────────────────────────────

func TestRelayHandler_Transfer_GasBumpRetrySucceeds(t *testing.T) {
	tx := dummyTx()
	c := &mockContract{tx: tx}
	// notFoundCalls=2: the first (unbumped) attempt polls once, gets
	// "not mined yet", then its short txTimeout fires before the next poll
	// — a genuine timeout, not an instant failure. submitWithRetry's
	// one-shot recheck (right before deciding to resubmit) consumes a
	// second "not mined yet", so it correctly falls through to the bumped
	// resubmission instead of short-circuiting as if the tx had actually
	// mined right at the deadline. The resubmission's first poll then finds
	// notFoundCalls exhausted and succeeds.
	m := &mockMiner{receipt: successReceipt(tx), notFoundCalls: 2}
	h := newTestHandlerWithTxTimeout(c, m, 600*time.Millisecond)
	r := server.NewWithHandler(testAPIKey, h)

	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, validTransferBody())
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200 (should recover via gas-bump retry): %s", w.Code, w.Body.String())
	}
	if c.transferCalls != 2 {
		t.Errorf("Transfer() called %d times, want 2 (initial + one gas-bump resubmit)", c.transferCalls)
	}
}

// TestRelayHandler_Transfer_MinedRightAtDeadlineSkipsResubmit covers the
// race submitWithRetry's one-shot recheck exists for: the tx actually mines
// in the same instant the wait context's deadline fires. Without the
// recheck, this would resubmit a same-nonce replacement for a transfer that
// already succeeded — the node would reject it (nonce already used) and the
// caller would see a spurious 500 for a request that, on-chain, worked.
func TestRelayHandler_Transfer_MinedRightAtDeadlineSkipsResubmit(t *testing.T) {
	tx := dummyTx()
	c := &mockContract{tx: tx}
	// notFoundCalls=1: the initial waitMined poll consumes it, so by the
	// time submitWithRetry's recheck runs, TransactionReceipt already has
	// the real receipt available — simulating the tx mining just after the
	// poll but before the deadline fired.
	m := &mockMiner{receipt: successReceipt(tx), notFoundCalls: 1}
	h := newTestHandlerWithTxTimeout(c, m, 600*time.Millisecond)
	r := server.NewWithHandler(testAPIKey, h)

	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, validTransferBody())
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200 (should recover via the mined-at-deadline recheck): %s", w.Code, w.Body.String())
	}
	if c.transferCalls != 1 {
		t.Errorf("Transfer() called %d times, want exactly 1 (no resubmit — the recheck should have found it already mined)", c.transferCalls)
	}
}

func TestRelayHandler_Transfer_StuckAfterGasBumpRetryFails(t *testing.T) {
	tx := dummyTx()
	c := &mockContract{tx: tx}
	m := &mockMiner{notFoundCalls: 1000} // never mines, on either attempt
	h := newTestHandlerWithTxTimeout(c, m, 600*time.Millisecond)
	r := server.NewWithHandler(testAPIKey, h)

	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, validTransferBody())
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500", w.Code)
	}
	if c.transferCalls != 2 {
		t.Errorf("Transfer() called %d times, want exactly 2 (one retry attempted, not an unbounded loop)", c.transferCalls)
	}
	if !strings.Contains(w.Body.String(), "stuck after gas-bump retry") {
		t.Errorf("expected error to explain the gas-bump retry also failed, got: %s", w.Body.String())
	}
}
