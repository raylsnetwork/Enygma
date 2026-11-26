// Package proofslots limits concurrent proving requests.
package proofslots

import (
	"context"
	"os"
	"runtime"
	"strconv"
	"time"
)

// Wait is the maximum time a proving request waits for a free slot.
const Wait = 30 * time.Second

// Limiter holds active requests until their handlers finish.
type Limiter chan struct{}

// New uses GNARK_MAX_CONCURRENT_PROOFS when it is set to a positive integer.
// The default is GOMAXPROCS.
func New() Limiter {
	n, err := strconv.Atoi(os.Getenv("GNARK_MAX_CONCURRENT_PROOFS"))
	if err != nil || n < 1 {
		n = runtime.GOMAXPROCS(0)
	}
	return make(Limiter, n)
}

// Acquire waits up to wait for a slot while the request is active.
func (l Limiter) Acquire(ctx context.Context, wait time.Duration) bool {
	ctx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	select {
	case l <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

// Release frees an acquired slot.
func (l Limiter) Release() { <-l }
