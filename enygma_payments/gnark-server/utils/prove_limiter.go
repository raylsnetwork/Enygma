package utils

import (
	"os"
	"runtime"
	"strconv"
	"time"
)

// ProveAcquireTimeout bounds how long a request will wait for a free
// ProveLimiter slot before the handler gives up and answers 503 rather
// than queuing indefinitely.
const ProveAcquireTimeout = 10 * time.Second

// ProveLimiter bounds how many groth16.Prove calls run concurrently across
// the whole server.
//
// Fix M-08: a single proof costs ~220-240ms of CPU (measured), and
// nothing previously limited how many could run at once — "no semaphore
// and no timeouts so the queue grows without bound." Every accepted
// request queued behind however many were already proving, with no cap
// and no way for the server to shed load under sustained traffic.
// Defaults to GOMAXPROCS (proving is CPU-bound; running more of them
// concurrently than there are cores to run them on only adds queuing
// delay, not throughput), overridable via GNARK_MAX_CONCURRENT_PROOFS for
// operators who want to tune it.
var ProveLimiter = newProveLimiter()

type limiter chan struct{}

func newProveLimiter() limiter {
	n := runtime.GOMAXPROCS(0)
	if v := os.Getenv("GNARK_MAX_CONCURRENT_PROOFS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			n = parsed
		}
	}
	return make(limiter, n)
}

// Acquire blocks until a slot is free or ProveAcquireTimeout elapses,
// returning false on timeout. Callers must call Release exactly once for
// every Acquire that returns true (typically via defer).
func (l limiter) Acquire() bool {
	select {
	case l <- struct{}{}:
		return true
	case <-time.After(ProveAcquireTimeout):
		return false
	}
}

// Release frees a slot acquired by Acquire.
func (l limiter) Release() { <-l }
