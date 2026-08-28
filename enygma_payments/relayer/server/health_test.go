package server

// White-box tests for Health (package server, not server_test) — needed to
// inject a fake chainProbe directly into Handler.chain, an unexported field
// with no public setter (by design: only NewHandler, the production path,
// ever populates it — see handler.go).

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// fakeChainProbe implements chainProbe without dialing a real chain.
type fakeChainProbe struct {
	block    uint64
	blockErr error
	balance  *big.Int
	balErr   error
}

func (f *fakeChainProbe) BlockNumber(ctx context.Context) (uint64, error) {
	return f.block, f.blockErr
}
func (f *fakeChainProbe) BalanceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (*big.Int, error) {
	return f.balance, f.balErr
}

func healthTestHandler(t *testing.T, chain chainProbe) *Handler {
	t.Helper()
	// Any valid private key works — Health only reads h.auth.From.
	privKey, err := crypto.HexToECDSA("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	if err != nil {
		t.Fatalf("HexToECDSA: %v", err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(privKey, big.NewInt(1337))
	if err != nil {
		t.Fatalf("NewKeyedTransactorWithChainID: %v", err)
	}
	h := newHandlerDeps(nil, "0xdeadbeef", auth, nil, nil)
	h.chain = chain
	return h
}

func doHealth(h *Handler) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	// Health is a gin.HandlerFunc method; drive it through a minimal gin
	// context rather than a full engine, matching how the rest of this
	// package's tests exercise handlers directly where convenient.
	r := NewWithHandler(map[string]string{}, h)
	r.ServeHTTP(w, req)
	return w
}

func TestHealth_ChainReachable(t *testing.T) {
	h := healthTestHandler(t, &fakeChainProbe{block: 12345, balance: big.NewInt(7_000_000_000_000_000_000)})
	w := doHealth(h)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}
	var resp HealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.ChainReachable {
		t.Error("ChainReachable: got false, want true")
	}
	if resp.LatestBlock != 12345 {
		t.Errorf("LatestBlock: got %d, want 12345", resp.LatestBlock)
	}
	if resp.RelayerBalanceWei != "7000000000000000000" {
		t.Errorf("RelayerBalanceWei: got %q", resp.RelayerBalanceWei)
	}
	if resp.ChainError != "" {
		t.Errorf("ChainError: got %q, want empty", resp.ChainError)
	}
}

func TestHealth_ChainUnreachable(t *testing.T) {
	h := healthTestHandler(t, &fakeChainProbe{blockErr: context.DeadlineExceeded})
	w := doHealth(h)
	// Fix M-09: the endpoint itself still answers 200 — it is the process
	// that is healthy — but ChainReachable:false is the signal monitoring
	// should key off instead of a static {"status":"ok"} that never fires.
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}
	var resp HealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.ChainReachable {
		t.Error("ChainReachable: got true, want false")
	}
	if resp.ChainError == "" {
		t.Error("ChainError: got empty, want the underlying dial/probe error")
	}
	if resp.LatestBlock != 0 {
		t.Errorf("LatestBlock: got %d, want 0 (unreachable)", resp.LatestBlock)
	}
}

func TestHealth_NoChainProbeConfigured(t *testing.T) {
	h := healthTestHandler(t, nil)
	w := doHealth(h)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}
	var resp HealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.ChainReachable {
		t.Error("ChainReachable: got true with no chain probe configured")
	}
}
