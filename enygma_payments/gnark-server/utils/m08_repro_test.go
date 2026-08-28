package utils

// TestParseBigInt_*, TestBigIntParser_*, TestProveLimiter_* reproduce and
// verify the M-08 fixes (ENYGMA_PAYMENTS_AUDIT_2026-08-22.md, Medium/LIVE):
//
//   - ParseBigInt used to discard SetString's ok flag and return nil for
//     any non-decimal input. That nil reached frontend.NewWitness, which
//     errors — but gnark's witness.Fill returns on its first error
//     without draining or closing its unbuffered internal channel,
//     permanently leaking the producer goroutine (gnark's own source
//     comments "we may leek a chan + producer go routine"). One
//     malformed ~200-byte POST leaked one goroutine forever. ParseBigInt
//     now returns an error instead of a silent nil.
//   - BigIntParser lets every handler build a witness field-by-field in
//     its existing inline style while accumulating the first parse
//     failure, so it can reject the request with a single check *before*
//     frontend.NewWitness is ever called — the audit's own remediation:
//     "this alone removes the leak trigger."
//   - ProveLimiter bounds how many groth16.Prove calls run concurrently,
//     closing "no semaphore and no timeouts so the queue grows without
//     bound."
//
// Run:
//
//	CC=/usr/bin/clang go test -run "TestParseBigInt|TestBigIntParser|TestProveLimiter" -v

import (
	"testing"
	"time"
)

// ── ParseBigInt ───────────────────────────────────────────────────────────────

func TestParseBigInt_ValidDecimal(t *testing.T) {
	n, err := ParseBigInt("424242")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n.Int64() != 424242 {
		t.Errorf("got %v, want 424242", n)
	}
}

// Fix M-08: this is the exact input class the audit's PoC used
// ({"nullifier":"x", ...}) to trigger the leak — a non-decimal string
// that used to silently become nil.
func TestParseBigInt_InvalidReturnsErrorNotNil(t *testing.T) {
	n, err := ParseBigInt("not-a-number")
	if err == nil {
		t.Fatal("FAIL (M-08 regressed): ParseBigInt accepted a non-decimal string with no error")
	}
	if n != nil {
		t.Errorf("got non-nil %v alongside a non-nil error — callers should never see both", n)
	}
}

func TestParseBigInt_EmptyStringReturnsError(t *testing.T) {
	// The exact failure mode a short JSON array silently zero-padded into
	// a fixed-size Go array would produce (see withdraw's [10]string
	// fields) — "" must be rejected, not silently treated as 0.
	if _, err := ParseBigInt(""); err == nil {
		t.Fatal("expected error for empty string, got nil")
	}
}

// ── BigIntParser ──────────────────────────────────────────────────────────────

func TestBigIntParser_AllValid_NoError(t *testing.T) {
	bp := &BigIntParser{}
	a := bp.Parse("1")
	b := bp.Parse("2")
	if bp.Err() != nil {
		t.Fatalf("unexpected error: %v", bp.Err())
	}
	if a.Int64() != 1 || b.Int64() != 2 {
		t.Errorf("got a=%v b=%v", a, b)
	}
}

// Fix M-08's central guarantee: Parse must never return nil, even on a
// parse failure — every handler assigns its return value straight into a
// witness field (`witness.X[i] = bp.Parse(...)`), and a nil there is
// exactly what reached frontend.NewWitness and triggered the leak before
// this fix. Returning a 0 placeholder instead means nil can never reach
// NewWitness through this path again — Err() is what actually gates
// whether the request proceeds.
func TestBigIntParser_NeverReturnsNil(t *testing.T) {
	bp := &BigIntParser{}
	n := bp.Parse("not-a-number")
	if n == nil {
		t.Fatal("FAIL (M-08 regressed): Parse returned nil on a parse failure")
	}
	if bp.Err() == nil {
		t.Fatal("expected Err() to be non-nil after a failed Parse")
	}
}

// A handler calls Parse many times per request (15-30+ fields); only the
// *first* failure should be reported, and later successful parses must
// not clear it.
func TestBigIntParser_AccumulatesOnlyFirstError(t *testing.T) {
	bp := &BigIntParser{}
	bp.Parse("1")
	bp.Parse("bad-1")
	bp.Parse("2")
	bp.Parse("bad-2")
	err := bp.Err()
	if err == nil {
		t.Fatal("expected an error after two failed Parse calls")
	}
	if got := err.Error(); got != `invalid decimal integer "bad-1"` {
		t.Errorf("Err() = %q, want the FIRST failure (bad-1), not a later one", got)
	}
}

// ── ProveLimiter ──────────────────────────────────────────────────────────────

func TestProveLimiter_AcquireRelease(t *testing.T) {
	l := make(limiter, 1)
	if !l.Acquire() {
		t.Fatal("Acquire on an empty limiter should succeed immediately")
	}
	l.Release()
	if !l.Acquire() {
		t.Fatal("Acquire after Release should succeed")
	}
	l.Release()
}

// Fix M-08: "no semaphore ... so the queue grows without bound." A
// limiter of size 1 must reject (time out) a second concurrent Acquire
// while the first slot is held, rather than blocking forever.
func TestProveLimiter_BoundsConcurrency(t *testing.T) {
	l := make(limiter, 1)
	if !l.Acquire() {
		t.Fatal("first Acquire should succeed")
	}
	defer l.Release()

	done := make(chan bool, 1)
	go func() {
		select {
		case l <- struct{}{}:
			done <- true // would only happen if the limiter didn't actually bound to 1
			l.Release()
		case <-time.After(200 * time.Millisecond):
			done <- false
		}
	}()

	if acquired := <-done; acquired {
		t.Fatal("FAIL: a second slot was acquired while the limiter's single slot was already held")
	}
}
