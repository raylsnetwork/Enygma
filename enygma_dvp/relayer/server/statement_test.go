package server

// Each route reads its receipts as one circuit's statement, so a publicSignal
// that is not that statement's length is refused with a 400 before the relayer
// touches the chain: not a vault lookup, not a root check, not a simulation.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"enygma_dvp/layout"

	"github.com/gin-gonic/gin"
)

func signals(n int) []string {
	s := make([]string, n)
	for i := range s {
		s[i] = strconv.Itoa(i + 1)
	}
	return s
}

func receiptOf(n, nIn, nOut int) ReceiptPayload {
	return ReceiptPayload{
		Proof:        [8]string{"1", "2", "3", "4", "5", "6", "7", "8"},
		PublicSignal: signals(n), NumberOfInputs: nIn, NumberOfOutputs: nOut,
	}
}

// postWithoutChain posts body to path on a handler whose node records every
// RPC call, and returns the response and whether the node was called at all.
func postWithoutChain(t *testing.T, path string, handle func(*Handler) gin.HandlerFunc, body interface{}) (*httptest.ResponseRecorder, []string) {
	t.Helper()
	node := &mockNode{}
	srv := node.serve(t)
	t.Cleanup(srv.Close)
	h := newTestHandler(t, srv.URL, 8_000_000)
	// newTestHandler dials and builds a transactor, which may query the chain
	// id; only calls made while handling the request count.
	node.mu.Lock()
	node.calls = nil
	node.mu.Unlock()

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST(path, handle(h))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw)))
	node.mu.Lock()
	defer node.mu.Unlock()
	return w, append([]string(nil), node.calls...)
}

func TestRelayPayment_RefusesASignalThatIsNotThePaymentStatementLength(t *testing.T) {
	for _, tc := range []struct {
		name   string
		n, nIn int
	}{
		{"Payment one short", layout.PaymentLen - 1, 1},
		{"Payment one long", layout.PaymentLen + 1, 1},
		{"Payment2in one short", layout.Payment2inLen - 1, 2},
		{"Payment2in signals with one input", layout.Payment2inLen, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, calls := postWithoutChain(t, "/relay/payment", func(h *Handler) gin.HandlerFunc { return h.RelayPayment },
				RelayPaymentRequest{VaultId: "0", Receipt: receiptOf(tc.n, tc.nIn, 2), CipherText: "0x00", EncTxData: "0x00"})
			if w.Code != http.StatusBadRequest {
				t.Fatalf("got %d, want 400: %s", w.Code, w.Body.String())
			}
			if len(calls) != 0 {
				t.Fatalf("the relayer called the chain before refusing the request: %v", calls)
			}
		})
	}
}

func TestRelaySwap_RefusesALegThatIsNotItsDvpStatementLength(t *testing.T) {
	for _, tc := range []struct {
		name         string
		payment, del int
	}{
		{"initiator one short", layout.DvpInitiatorLen - 1, layout.DvpDestinationLen},
		{"initiator one long", layout.DvpInitiatorLen + 1, layout.DvpDestinationLen},
		{"destination one short", layout.DvpInitiatorLen, layout.DvpDestinationLen - 1},
		{"destination one long", layout.DvpInitiatorLen, layout.DvpDestinationLen + 1},
		{"legs swapped", layout.DvpDestinationLen, layout.DvpInitiatorLen},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, calls := postWithoutChain(t, "/relay/swap", func(h *Handler) gin.HandlerFunc { return h.RelaySwap },
				RelaySwapRequest{
					PaymentReceipt: receiptOf(tc.payment, 1, 1), DeliveryReceipt: receiptOf(tc.del, 1, 1),
					PaymentVaultId: "0", DeliveryVaultId: "1",
				})
			if w.Code != http.StatusBadRequest {
				t.Fatalf("got %d, want 400: %s", w.Code, w.Body.String())
			}
			if len(calls) != 0 {
				t.Fatalf("the relayer called the chain before refusing the request: %v", calls)
			}
		})
	}
}
