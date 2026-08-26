package enygma_test

// End-to-end integration tests for the relayer HTTP server.
// Uses a real TCP listener (httptest.NewServer) and a mock Ethereum backend.
// No running chain or gnark server required.
//
// Run:
//
//	cd enygma_payments/test && go test -run TestRelayIntegration -v

import (
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"enygma_payments_relayer/server"
)

// newIntegrationServer starts a real TCP server backed by mock contract + miner.
func newIntegrationServer(t *testing.T, c *mockContract, m *mockMiner) *httptest.Server {
	t.Helper()
	h := newTestHandler(c, m)
	r := server.NewWithHandler(testAPIKeys, h)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

func getURL(t *testing.T, url string) *http.Response {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	return resp
}

func postURL(t *testing.T, url, apiKey string, body interface{}) *http.Response {
	t.Helper()
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(string(data)))
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

func readAll(t *testing.T, r *http.Response) []byte {
	t.Helper()
	defer r.Body.Close()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return b
}

// ── Health check ──────────────────────────────────────────────────────────────

func TestRelayIntegration_Health(t *testing.T) {
	srv := newIntegrationServer(t, &mockContract{}, &mockMiner{})

	resp := getURL(t, srv.URL+"/health")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got %d, want 200", resp.StatusCode)
	}
	var body map[string]string
	json.Unmarshal(readAll(t, resp), &body)
	if body["status"] != "ok" {
		t.Errorf("status: got %q, want ok", body["status"])
	}
}

// ── Info endpoint ─────────────────────────────────────────────────────────────

func TestRelayIntegration_Info(t *testing.T) {
	srv := newIntegrationServer(t, &mockContract{}, &mockMiner{})

	resp := getURL(t, srv.URL+"/relay/info")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got %d, want 200", resp.StatusCode)
	}
	var info server.InfoResponse
	json.Unmarshal(readAll(t, resp), &info)
	if info.ContractAddr == "" {
		t.Error("ContractAddr is empty")
	}
	if info.RelayerAddr == "" {
		t.Error("RelayerAddr is empty")
	}
	if info.ChainID != 1337 {
		t.Errorf("ChainID: got %d, want 1337", info.ChainID)
	}
}

// ── Auth enforcement ──────────────────────────────────────────────────────────

func TestRelayIntegration_Auth(t *testing.T) {
	srv := newIntegrationServer(t, &mockContract{}, &mockMiner{})

	t.Run("no_header", func(t *testing.T) {
		resp := postURL(t, srv.URL+"/relay/transfer", "", validTransferBody())
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("got %d, want 401", resp.StatusCode)
		}
		resp.Body.Close()
	})
	t.Run("wrong_token", func(t *testing.T) {
		resp := postURL(t, srv.URL+"/relay/transfer", "bad-token", validTransferBody())
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("got %d, want 403", resp.StatusCode)
		}
		resp.Body.Close()
	})
}

// ── Transfer ──────────────────────────────────────────────────────────────────

func TestRelayIntegration_Transfer(t *testing.T) {
	tx := dummyTx()
	receipt := successReceipt(tx)
	srv := newIntegrationServer(t, &mockContract{tx: tx}, &mockMiner{receipt: receipt})

	resp := postURL(t, srv.URL+"/relay/transfer", testAPIKey, validTransferBody())
	body := readAll(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("transfer: got %d: %s", resp.StatusCode, body)
	}
	var txResp server.RelayResponse
	json.Unmarshal(body, &txResp)
	if txResp.TxHash == "" {
		t.Error("TxHash is empty")
	}
	if txResp.BlockNumber != 42 {
		t.Errorf("BlockNumber: got %d, want 42", txResp.BlockNumber)
	}
	if txResp.GasUsed != 21000 {
		t.Errorf("GasUsed: got %d, want 21000", txResp.GasUsed)
	}
}

// ── Deduplication ─────────────────────────────────────────────────────────────

func TestRelayIntegration_Deduplication(t *testing.T) {
	tx := dummyTx()
	h := newTestHandler(&mockContract{tx: tx}, &mockMiner{receipt: successReceipt(tx)})
	srv := httptest.NewServer(server.NewWithHandler(testAPIKeys, h))
	t.Cleanup(srv.Close)

	body := validTransferBody()
	// Mark the dedup key as in-flight to simulate a concurrent duplicate.
	// Fix H-10: the dedup key is now a hash of the whole body, not just
	// "transfer:"+proof[0] — server.DedupKey computes it the same way
	// RelayTransfer does.
	dedupKey, err := server.DedupKey("transfer", body)
	if err != nil {
		t.Fatalf("DedupKey: %v", err)
	}
	h.SetInFlight(dedupKey, struct{}{})
	defer h.DeleteInFlight(dedupKey)

	resp := postURL(t, srv.URL+"/relay/transfer", testAPIKey, body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("got %d, want 409", resp.StatusCode)
	}
}

// ── Public endpoints need no auth ─────────────────────────────────────────────

func TestRelayIntegration_PublicEndpointsNoAuth(t *testing.T) {
	srv := newIntegrationServer(t, &mockContract{}, &mockMiner{})

	for _, path := range []string{"/health", "/relay/info"} {
		resp := getURL(t, srv.URL+path)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: got %d, want 200", path, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

// ── PublicSignal zero-padding ─────────────────────────────────────────────────

// TestRelayIntegration_TransferPublicSignalRejectsShort reproduces L-05
// over the real HTTP stack (not just the in-process handler test in
// relayer_handler_test.go): a short publicSignal must be rejected, not
// silently zero-padded and forwarded as a signed, broadcast transaction.
func TestRelayIntegration_TransferPublicSignalRejectsShort(t *testing.T) {
	srv := newIntegrationServer(t, &mockContract{}, &mockMiner{})

	body := validTransferBody()
	body.PublicSignal = []string{big.NewInt(999).String()} // only 1 signal, want exactly transferPublicSignalLen

	resp := postURL(t, srv.URL+"/relay/transfer", testAPIKey, body)
	b := readAll(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("FAIL (L-05 regressed): got %d, want 400 (a short publicSignal must be rejected, not zero-padded): %s", resp.StatusCode, b)
	}
}
