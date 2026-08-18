package server

import (
	"context"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

// NonceManager tracks the next nonce to use for the relayer's own address.
// Fetched once at startup, then incremented locally after each claim —
// rather than calling PendingNonceAt fresh on every request. All claims are
// already serialized by Handler.txMu, so this is really just a local counter
// with an explicit release path for the one case that matters: a claim that
// never resulted in a broadcast tx (so nothing on-chain is using it yet).
//
// Exported (and NewNonceManagerFromStart provided) so external test packages
// can inject one via HandlerDeps without dialing a real chain.
type NonceManager struct {
	mu   sync.Mutex
	next uint64
}

// NewNonceManagerFromStart creates a NonceManager beginning at start —
// for tests that don't have a real chain to fetch a starting nonce from.
func NewNonceManagerFromStart(start uint64) *NonceManager {
	return &NonceManager{next: start}
}

// pendingNonceAt is the subset of the chain client needed to seed the
// manager — satisfied by *ethclient.Client and by mocks in tests.
type pendingNonceAt interface {
	PendingNonceAt(ctx context.Context, account common.Address) (uint64, error)
}

func newNonceManager(ctx context.Context, client pendingNonceAt, addr common.Address) (*NonceManager, error) {
	n, err := client.PendingNonceAt(ctx, addr)
	if err != nil {
		return nil, fmt.Errorf("fetch starting nonce: %w", err)
	}
	return &NonceManager{next: n}, nil
}

// take claims the next nonce.
func (nm *NonceManager) take() uint64 {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	n := nm.next
	nm.next++
	return n
}

// release gives back a claimed nonce that was never used in a broadcast tx
// (the contract call itself errored before anything was sent) — but only if
// it's still the most recently claimed one, so a release can never rewind
// past a nonce another goroutine has since built on. Everything is already
// serialized by Handler.txMu in practice, so this condition should always
// hold; the check is a safety net, not load-bearing.
func (nm *NonceManager) release(n uint64) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	if n == nm.next-1 {
		nm.next = n
	}
}
