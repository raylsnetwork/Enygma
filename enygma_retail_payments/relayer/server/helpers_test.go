package server

import (
	"bytes"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"enygma_relayer/config"
	"enygma_relayer/store"

	"github.com/gin-gonic/gin"
)

// ── bumpGasPrice ──────────────────────────────────────────────────────────────

func TestBumpGasPrice_AlwaysStrictlyGreater(t *testing.T) {
	// Covers the full range where the 112/100 integer-division bump alone
	// would be a no-op (1..8 wei), the boundary where it starts working on
	// its own (9 wei), and realistic higher values.
	for _, gasPrice := range []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 100, 1_000_000_000} {
		gp := big.NewInt(gasPrice)
		bumped := bumpGasPrice(gp)
		if bumped.Cmp(gp) <= 0 {
			t.Errorf("bumpGasPrice(%d) = %v, want strictly greater than %d", gasPrice, bumped, gasPrice)
		}
	}
}

func TestBumpGasPrice_LowPriceFallsBackToPlusOne(t *testing.T) {
	// At gasPrice=1, 1*112/100 truncates to 1 (no-op) — must fall back to
	// exactly gasPrice+1, not silently stay at gasPrice.
	got := bumpGasPrice(big.NewInt(1))
	if got.Int64() != 2 {
		t.Errorf("bumpGasPrice(1) = %v, want 2 (gasPrice+1 fallback)", got)
	}
}

func TestBumpGasPrice_HighPriceUsesPercentageBump(t *testing.T) {
	// At realistic gas prices the 12% bump dominates the +1 floor.
	got := bumpGasPrice(big.NewInt(1_000_000_000))
	want := big.NewInt(1_120_000_000)
	if got.Cmp(want) != 0 {
		t.Errorf("bumpGasPrice(1e9) = %v, want %v", got, want)
	}
}

// ── RelayPaymentFee: MinFee enforcement ──────────────────────────────────────

// stubVerifier always accepts a proof — these tests exercise the
// fee-threshold logic, not the cryptography, so a real Groth16 proof isn't
// needed to reach it.
type stubVerifier struct{}

func (stubVerifier) Verify(proof [8]*big.Int, publicSignal []*big.Int) error { return nil }

// newTestHandler builds a Handler with just enough wired up to reach
// RelayPaymentFee's MinFee check: a real (temp-file) store, since dedup runs
// before the fee check, and a stub verifier so local Groth16 verification
// always passes. dvpABI/vaultABI/client are left zero-valued — fine, because
// every assertion here is about a response returned before those fields
// would ever be touched.
func newTestHandler(t *testing.T, minFee int64) *Handler {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return &Handler{
		cfg: &config.Config{
			MinFee:         big.NewInt(minFee),
			DedupStaleness: time.Minute,
		},
		vRelayerFee: stubVerifier{},
		store:       st,
	}
}

// postPaymentFee invokes RelayPaymentFee directly (no HTTP server, no
// routing/auth/rate-limit middleware) with a syntactically valid request
// whose only fee-relevant field is publicSignal[8] (StFee).
func postPaymentFee(h *Handler, fee string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	body, _ := json.Marshal(RelayPaymentFeeRequest{
		VaultId: "1",
		Proof:   [8]string{"1", "2", "3", "4", "5", "6", "7", "8"},
		PublicSignal: [9]string{
			"0", "1", "2", "3", "4", "5", "6", "7", fee,
		},
		CipherText: "0x",
		EncTxData:  "0x",
	})
	req := httptest.NewRequest(http.MethodPost, "/relay/payment_fee", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	h.RelayPaymentFee(c)
	return w
}

func TestRelayPaymentFee_BelowMinFeeRejectedWith402(t *testing.T) {
	h := newTestHandler(t, 100)
	w := postPaymentFee(h, "50")
	if w.Code != http.StatusPaymentRequired {
		t.Fatalf("got status %d, want %d; body=%s", w.Code, http.StatusPaymentRequired, w.Body.String())
	}
}

func TestRelayPaymentFee_AtMinFeeIsAccepted(t *testing.T) {
	h := newTestHandler(t, 100)
	w := postPaymentFee(h, "100")
	if w.Code == http.StatusPaymentRequired {
		t.Fatalf("fee equal to MinFee should not be rejected with 402: %s", w.Body.String())
	}
}

func TestRelayPaymentFee_MinFeeDisabledAllowsZeroFee(t *testing.T) {
	// MinFee=0 disables enforcement entirely — a zero fee must pass the
	// fee-threshold check. The request still fails past that point (no real
	// chain behind h.client in this test), which is expected; the only
	// thing under test is that the failure isn't a 402.
	h := newTestHandler(t, 0)
	w := postPaymentFee(h, "0")
	if w.Code == http.StatusPaymentRequired {
		t.Fatalf("MinFee=0 should not reject on fee; got 402: %s", w.Body.String())
	}
}
