package enygma_test

// In-flight claims are keyed by the nullifiers a request consumes, so the same
// proof cannot be relayed twice at once by varying other request fields, and a
// claim outlives a request that timed out until its transaction is mined.

import (
	"context"
	"math/big"
	"net/http"
	"sync"
	"testing"
	"time"

	"enygma_payments/relayer/server"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestRelayHandler_Transfer_DuplicateNullifierRefusedWhateverElseDiffers(t *testing.T) {
	tx := dummyTx()
	h := newTestHandler(&mockContract{tx: tx}, &mockMiner{receipt: successReceipt(tx)})
	r := server.NewWithHandler(testAPIKeys, h)

	inFlight := validTransferBody()
	key, err := server.DedupKey("transfer", inFlight)
	if err != nil {
		t.Fatal(err)
	}
	h.SetInFlight(key, struct{}{})
	defer h.DeleteInFlight(key)

	// Same note (same nullifier), different proof bytes and commitments: the
	// old whole-body key let this through, and one of the two then reverted
	// on chain at the relayer's cost.
	dup := validTransferBody()
	dup.Proof[0] = "999"
	dup.Commitments[0] = []string{"101", "102"}
	if w := serveHTTPPost(r, "/relay/transfer", testAPIKey, dup); w.Code != http.StatusConflict {
		t.Fatalf("same nullifier with other fields changed: got %d, want 409: %s", w.Code, w.Body.String())
	}

	// A different note goes through.
	other := validTransferBody()
	other.PublicSignal[server.TransferPublicSignalLen-2] = "123456789"
	other.UsdrPublicSignal[79] = "987654321"
	if w := serveHTTPPost(r, "/relay/transfer", testAPIKey, other); w.Code != http.StatusOK {
		t.Fatalf("a different nullifier was refused: got %d: %s", w.Code, w.Body.String())
	}
}

// switchMiner reports no receipt until mined is set.
type switchMiner struct {
	mu      sync.Mutex
	receipt *types.Receipt
}

func (m *switchMiner) TransactionReceipt(_ context.Context, _ common.Hash) (*types.Receipt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.receipt == nil {
		return nil, ethereum.NotFound
	}
	return m.receipt, nil
}
func (m *switchMiner) CodeAt(_ context.Context, _ common.Address, _ *big.Int) ([]byte, error) {
	return []byte("ok"), nil
}
func (m *switchMiner) mine(r *types.Receipt) {
	m.mu.Lock()
	m.receipt = r
	m.mu.Unlock()
}

func TestRelayHandler_Transfer_TimeoutAnswersPendingAndHoldsClaim(t *testing.T) {
	defer server.SetTxTimeout(300 * time.Millisecond)()

	tx := dummyTx()
	miner := &switchMiner{}
	h := newTestHandlerWithBackend(&mockContract{tx: tx}, miner)
	r := server.NewWithHandler(testAPIKeys, h)
	body := validTransferBody()

	w := serveHTTPPost(r, "/relay/transfer", testAPIKey, body)
	if w.Code != http.StatusAccepted {
		t.Fatalf("unmined transaction: got %d, want 202: %s", w.Code, w.Body.String())
	}
	// The transaction may still land: the same note must stay claimed.
	if w := serveHTTPPost(r, "/relay/transfer", testAPIKey, body); w.Code != http.StatusConflict {
		t.Fatalf("retry while the first transaction is pending: got %d, want 409", w.Code)
	}

	miner.mine(successReceipt(tx)) // it lands
	deadline := time.Now().Add(5 * time.Second)
	for {
		key, _ := server.DedupKey("transfer", body)
		if h.InFlight(key) == false {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the note stayed claimed after its transaction was mined")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// A transfer proof binds every participant's current balance, so a second
// transfer touching an account of a pending one is refused (409) instead of
// being sent to revert on chain; it goes through once the account is free.
func TestRelayHandler_Transfer_ConflictingParticipantRefused(t *testing.T) {
	tx := dummyTx()
	h := newTestHandler(&mockContract{tx: tx}, &mockMiner{receipt: successReceipt(tx)})
	r := server.NewWithHandler(testAPIKeys, h)

	body := validTransferBody() // participants 1..6, nullifiers not in flight
	busy := server.ParticipantKey(body.KIndex[2])
	h.SetInFlight(busy, struct{}{}) // another transfer touching account 3 is pending

	if w := serveHTTPPost(r, "/relay/transfer", testAPIKey, body); w.Code != http.StatusConflict {
		t.Fatalf("transfer sharing a pending participant: got %d, want 409: %s", w.Code, w.Body.String())
	}
	// Nothing of the refused request stays claimed.
	key, _ := server.DedupKey("transfer", body)
	if h.InFlight(key) || h.InFlight(server.ParticipantKey(body.KIndex[0])) {
		t.Fatal("a refused request left claims behind")
	}

	h.DeleteInFlight(busy) // the pending transfer settled
	if w := serveHTTPPost(r, "/relay/transfer", testAPIKey, body); w.Code != http.StatusOK {
		t.Fatalf("after the account is free: got %d, want 200: %s", w.Code, w.Body.String())
	}
	for _, id := range body.KIndex {
		if h.InFlight(server.ParticipantKey(id)) {
			t.Fatalf("account %d still claimed after its transfer was mined", id)
		}
	}
}

// A repeated participant id is a malformed request (400), not a transient
// conflict with its own account claim (409).
func TestRelayHandler_Transfer_DuplicateParticipantIdIsBadRequest(t *testing.T) {
	tx := dummyTx()
	h := newTestHandler(&mockContract{tx: tx}, &mockMiner{receipt: successReceipt(tx)})
	r := server.NewWithHandler(testAPIKeys, h)
	body := validTransferBody()
	body.KIndex = []int64{1, 2, 3, 3, 5, 6}
	if w := serveHTTPPost(r, "/relay/transfer", testAPIKey, body); w.Code != http.StatusBadRequest {
		t.Fatalf("duplicate participant id: got %d, want 400: %s", w.Code, w.Body.String())
	}
}
