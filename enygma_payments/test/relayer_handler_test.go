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
	r := server.NewWithHandler(testAPIKeys, newTestHandler(&mockContract{}, &mockMiner{}))
	req := httptest.NewRequest(http.MethodPost, "/relay/transfer", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", w.Code)
	}
}

func TestRelayHandler_Auth_WrongToken(t *testing.T) {
	r := server.NewWithHandler(testAPIKeys, newTestHandler(&mockContract{}, &mockMiner{}))
	req := httptest.NewRequest(http.MethodPost, "/relay/transfer", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("got %d, want 403", w.Code)
	}
}

func TestRelayHandler_Auth_BadScheme(t *testing.T) {
	r := server.NewWithHandler(testAPIKeys, newTestHandler(&mockContract{}, &mockMiner{}))
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
	r := server.NewWithHandler(testAPIKeys, newTestHandler(&mockContract{}, &mockMiner{}))
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
	r := server.NewWithHandler(testAPIKeys, newTestHandler(&mockContract{}, &mockMiner{}))
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
	r := server.NewWithHandler(testAPIKeys, newTestHandler(&mockContract{tx: tx}, &mockMiner{receipt: successReceipt(tx)}))
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

// TestRelayHandler_Transfer_AttributesCallingBank reproduces the H-09
// (item 4) fix directly: the relayer must pass the authenticated caller's
// bank identity (Fix H-06's bankID, resolved from the bearer token) as
// Transfer()'s bankTag argument, not an empty string — this is what lets
// the chain's own RelayAttribution event carry per-bank attribution
// instead of relying solely on the relayer's off-chain logs.
func TestRelayHandler_Transfer_AttributesCallingBank(t *testing.T) {
	tx := dummyTx()
	mc := &mockContract{tx: tx}
	r := server.NewWithHandler(testAPIKeys, newTestHandler(mc, &mockMiner{receipt: successReceipt(tx)}))
	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, validTransferBody())
	if w.Code != http.StatusOK {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	const wantBankTag = "bank-test" // testAPIKeys[testAPIKey], see relayer_mocks_test.go
	if mc.gotBankTag != wantBankTag {
		t.Errorf("FAIL (H-09 regressed): Transfer() was called with bankTag %q, want %q", mc.gotBankTag, wantBankTag)
	}
}

func TestRelayHandler_Transfer_InvalidJSON(t *testing.T) {
	r := server.NewWithHandler(testAPIKeys, newTestHandler(&mockContract{}, &mockMiner{}))
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
	r := server.NewWithHandler(testAPIKeys, newTestHandler(&mockContract{}, &mockMiner{}))
	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, body)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestRelayHandler_Transfer_InvalidCommitment(t *testing.T) {
	body := validTransferBody()
	body.Commitments[1] = []string{"bad", "1"}
	r := server.NewWithHandler(testAPIKeys, newTestHandler(&mockContract{}, &mockMiner{}))
	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, body)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

// bn254Fr/bn254Fq mirror the moduli server.checkFieldElement validates
// against (Fix L-05) — duplicated here rather than exported from server,
// since only these tests need the literal values.
var (
	bn254Fr, _ = new(big.Int).SetString("21888242871839275222246405745257275088548364400416034343698204186575808495617", 10)
	bn254Fq, _ = new(big.Int).SetString("21888242871839275222246405745257275088696311157297823662689037894645226208583", 10)
)

// TestRelayHandler_Transfer_OutOfRangeProofElementRejected reproduces the
// L-05 field-order gap directly: pre-fix, every numeric field was parsed
// with plain big.Int.SetString and handed to abigen with no check against
// the field it's actually an element of. A proof coordinate at or above
// BN254's base field Fq is not a valid G1/G2 coordinate.
func TestRelayHandler_Transfer_OutOfRangeProofElementRejected(t *testing.T) {
	body := validTransferBody()
	body.Proof[0] = bn254Fq.String() // == Fq is already out of range ([0, Fq))
	r := server.NewWithHandler(testAPIKeys, newTestHandler(&mockContract{}, &mockMiner{}))
	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("FAIL (L-05 regressed): got %d, want 400 for a proof element == Fq: %s", w.Code, w.Body.String())
	}
}

// TestRelayHandler_Transfer_OutOfRangePublicSignalElementRejected: a
// public-signal element (the SNARK's own public input, an Fr element) at
// or above BN254's scalar field Fr is not a valid field element either.
func TestRelayHandler_Transfer_OutOfRangePublicSignalElementRejected(t *testing.T) {
	body := validTransferBody()
	body.PublicSignal[10] = bn254Fr.String()
	r := server.NewWithHandler(testAPIKeys, newTestHandler(&mockContract{}, &mockMiner{}))
	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("FAIL (L-05 regressed): got %d, want 400 for a publicSignal element == Fr: %s", w.Code, w.Body.String())
	}
}

// TestRelayHandler_Transfer_OutOfRangeCommitmentRejected: Baby Jubjub
// point coordinates (Commitments) are embedded inside BN254's scalar
// field Fr, so the same bound applies to them.
func TestRelayHandler_Transfer_OutOfRangeCommitmentRejected(t *testing.T) {
	body := validTransferBody()
	body.Commitments[0] = []string{bn254Fr.String(), "1"}
	r := server.NewWithHandler(testAPIKeys, newTestHandler(&mockContract{}, &mockMiner{}))
	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("FAIL (L-05 regressed): got %d, want 400 for a commitment coordinate == Fr: %s", w.Code, w.Body.String())
	}
}

// TestRelayHandler_Transfer_NegativeProofElementRejected: a negative
// value is not a valid field element either (checkFieldElement's Sign()
// check, not just the >= modulus one above).
func TestRelayHandler_Transfer_NegativeProofElementRejected(t *testing.T) {
	body := validTransferBody()
	body.Proof[0] = "-1"
	r := server.NewWithHandler(testAPIKeys, newTestHandler(&mockContract{}, &mockMiner{}))
	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400 for a negative proof element: %s", w.Code, w.Body.String())
	}
}

func TestRelayHandler_Transfer_TooManyPublicSignals(t *testing.T) {
	body := validTransferBody()
	// Fix L-05: exactly transferPublicSignalLen is now required — one
	// element over that must be rejected the same way one under it is
	// (TestRelayHandler_Transfer_EmptyPublicSignals).
	body.PublicSignal = make([]string, transferPublicSignalLen+1)
	for i := range body.PublicSignal {
		body.PublicSignal[i] = "1"
	}
	r := server.NewWithHandler(testAPIKeys, newTestHandler(&mockContract{}, &mockMiner{}))
	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, body)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestRelayHandler_Transfer_InvalidPublicSignal(t *testing.T) {
	body := validTransferBody()
	body.PublicSignal[3] = "not-a-number"
	r := server.NewWithHandler(testAPIKeys, newTestHandler(&mockContract{}, &mockMiner{}))
	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, body)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

// TestRelayHandler_Transfer_EmptyPublicSignals reproduces the core L-05
// finding directly: an empty (or any short) publicSignal used to be
// silently zero-padded up to transferPublicSignalLen and forwarded as a
// signed, broadcast transaction — a malformed/truncated request paid
// real gas for a payload that could only ever revert on chain (Groth16
// verification over the padded vector cannot match a proof generated for
// a different, non-zero-tail statement). It must now be rejected locally
// instead.
func TestRelayHandler_Transfer_EmptyPublicSignals(t *testing.T) {
	body := validTransferBody()
	body.PublicSignal = []string{}
	r := server.NewWithHandler(testAPIKeys, newTestHandler(&mockContract{}, &mockMiner{}))
	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("FAIL (L-05 regressed): got %d, want 400 (an empty publicSignal must be rejected, not zero-padded): %s", w.Code, w.Body.String())
	}
}

func TestRelayHandler_Transfer_ContractError(t *testing.T) {
	r := server.NewWithHandler(testAPIKeys, newTestHandler(
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
	r := server.NewWithHandler(testAPIKeys, newTestHandler(
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
	r := server.NewWithHandler(testAPIKeys, h)

	body := validTransferBody()
	// Pre-seed the in-flight map to simulate a concurrent identical request.
	// Fix H-10: the dedup key is now a hash of the whole body, not just
	// "transfer:"+proof[0] — server.DedupKey computes it the same way
	// RelayTransfer does.
	dedupKey, err := server.DedupKey("transfer", body)
	if err != nil {
		t.Fatalf("DedupKey: %v", err)
	}
	h.SetInFlight(dedupKey, struct{}{})
	defer h.DeleteInFlight(dedupKey)

	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, body)
	if w.Code != http.StatusConflict {
		t.Errorf("got %d, want 409", w.Code)
	}
}
