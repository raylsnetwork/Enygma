package enygma_test

// TestRelayTier2_* reproduces and verifies the Tier 2 relayer fixes
// (ENYGMA_PAYMENTS_AUDIT_2026-08-22.md):
//
//   H-06 — one shared bearer token authenticated every bank, so a request
//     could never be attributed to a specific bank, and revoking one bank
//     meant rotating the token and locking out every other bank too.
//     Now each bank gets its own token (server.bearerAuth looks up a
//     token -> bankID map); the matching bankID is attached to the request
//     context and used in the relayer's logs.
//   H-10 (mechanisms 2 and 4) — commitments/kIndex were unbounded (an
//     oversized, guaranteed-to-revert payload could still be signed and
//     broadcast, burning intrinsic gas before the on-chain revert) and
//     negative kIndex values silently became huge uint256s via ABI two's
//     complement. Now both are rejected with 400 before any transaction is
//     built. (Mechanisms 1 and 3 — auto gas estimation and releasing txMu
//     before WaitMined — aren't independently observable through the
//     mockContract/mockMiner harness these tests share with the rest of
//     the relayer test suite, so they're verified by code review plus the
//     unit tests in relayer/server/helpers_test.go instead.)
//   M-09 — /health returned a static {"status":"ok"} with no chain
//     liveness signal. It now reports chainReachable/latestBlock/
//     relayerBalanceWei when a real chain probe is wired in (nil, and
//     omitted, for the mock-backed Handler these tests use).
//
// Run:
//
//	cd enygma_payments/test && go test -run TestRelayTier2 -v

import (
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"enygma_payments_relayer/config"
	"enygma_payments_relayer/server"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/crypto"
)

// newMultiBankHandler is newTestHandler's multi-credential counterpart: a
// Handler backed by the same mocks, but with an explicit apiKeys map
// instead of the single-bank testAPIKeys used elsewhere in this suite.
func newMultiBankHandler(c *mockContract, m *mockMiner, apiKeys map[string]string) (*server.Handler, map[string]string) {
	privKey, _ := crypto.HexToECDSA(hardhatKey0)
	auth, _ := bind.NewKeyedTransactorWithChainID(privKey, big.NewInt(1337))
	cfg := &config.Config{
		APIKeys:  apiKeys,
		ChainID:  big.NewInt(1337),
		GasLimit: 300_000_000,
	}
	h := server.NewHandlerWithDeps(cfg, "0x1234567890123456789012345678901234567890", auth, m, c)
	return h, apiKeys
}

// ── H-06: per-bank attribution ───────────────────────────────────────────────

func TestRelayTier2_H06_DistinctBankTokensBothWork(t *testing.T) {
	tx := dummyTx()
	apiKeys := map[string]string{
		"token-bank-a": "bank-a",
		"token-bank-b": "bank-b",
	}
	h, _ := newMultiBankHandler(&mockContract{tx: tx}, &mockMiner{receipt: successReceipt(tx)}, apiKeys)
	r := server.NewWithHandler(apiKeys, h)

	for _, tok := range []string{"token-bank-a", "token-bank-b"} {
		w := serveHTTPPost(r, "/relay/transfer", tok, validTransferBody())
		if w.Code != http.StatusOK {
			t.Fatalf("bank token %q: got %d: %s", tok, w.Code, w.Body.String())
		}
	}
}

// A token issued to one bank must not be accepted alongside — or confused
// for — another bank's token; only exact matches against the issued set
// authenticate.
func TestRelayTier2_H06_UnknownTokenRejectedEvenWithSiblingBanksConfigured(t *testing.T) {
	apiKeys := map[string]string{
		"token-bank-a": "bank-a",
		"token-bank-b": "bank-b",
	}
	h, _ := newMultiBankHandler(&mockContract{}, &mockMiner{}, apiKeys)
	r := server.NewWithHandler(apiKeys, h)

	w := serveHTTPPost(r, "/relay/transfer", "token-bank-c", validTransferBody())
	if w.Code != http.StatusForbidden {
		t.Errorf("got %d, want 403", w.Code)
	}
}

// ── H-10: bounded commitments/kIndex ─────────────────────────────────────────

func TestRelayTier2_H10_OversizedParticipantCountRejected(t *testing.T) {
	r := server.NewWithHandler(testAPIKeys, newTestHandler(&mockContract{}, &mockMiner{}))

	body := validTransferBody()
	// Grow past the fixed on-chain participant count (6) with otherwise
	// well-formed entries — pre-fix, this would have been signed and
	// broadcast only to revert on-chain.
	body.Commitments = append(body.Commitments, []string{"13", "14"})
	body.KIndex = append(body.KIndex, 7)

	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, body)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestRelayTier2_H10_UndersizedParticipantCountRejected(t *testing.T) {
	r := server.NewWithHandler(testAPIKeys, newTestHandler(&mockContract{}, &mockMiner{}))

	body := validTransferBody()
	body.Commitments = body.Commitments[:5]
	body.KIndex = body.KIndex[:5]

	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, body)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

// ── H-10: negative kIndex rejected ───────────────────────────────────────────

func TestRelayTier2_H10_NegativeKIndexRejected(t *testing.T) {
	r := server.NewWithHandler(testAPIKeys, newTestHandler(&mockContract{}, &mockMiner{}))

	body := validTransferBody()
	body.KIndex[2] = -1 // would otherwise ABI-encode as 2^256-1

	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, body)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

// ── M-09: /health reports pendingTransactions even without a chain probe ────

func TestRelayTier2_M09_HealthReportsPendingTransactionsField(t *testing.T) {
	r := server.NewWithHandler(testAPIKeys, newTestHandler(&mockContract{}, &mockMiner{}))
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}
	var body server.HealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Status != "ok" {
		t.Errorf("status: got %q, want ok", body.Status)
	}
	// The mock-backed Handler has no chain probe wired in (Fix M-09 doc on
	// Handler.chain) — chainReachable must default false, not panic.
	if body.ChainReachable {
		t.Error("chainReachable: got true with no chain probe configured")
	}
	if body.PendingTransactions != 0 {
		t.Errorf("pendingTransactions: got %d, want 0 (no in-flight transfer)", body.PendingTransactions)
	}
}
