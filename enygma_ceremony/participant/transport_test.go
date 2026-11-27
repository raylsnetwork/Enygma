package participant

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestPlainHTTPOnlyToThisMachine (audit R-055): a coordinator is reached
// over https; plain http only to one on this machine, where nobody is in
// between.
func TestPlainHTTPOnlyToThisMachine(t *testing.T) {
	for addr, ok := range map[string]bool{
		"ceremony.example.org":          true, // becomes https://
		"https://ceremony.example.org":  true,
		"http://ceremony.example.org":   false,
		"http://192.0.2.1:8080":         false,
		"http://127.0.0.1:8080":         true,
		"http://localhost:8080":         true,
		"http://[::1]:8080":             true,
		"ftp://ceremony.example.org":    false,
		"https://":                      false,
	} {
		c := &Client{URL: addr}
		if err := c.checkURL(); (err == nil) != ok {
			t.Errorf("%s: %v, want accepted %v", addr, err, ok)
		}
	}
}

// TestFollowsNoRedirect (audit R-056): the protocol has no redirects, and
// following one could carry the session token elsewhere; the program stops.
func TestFollowsNoRedirect(t *testing.T) {
	var elsewhere atomic.Bool
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		elsewhere.Store(true)
	}))
	defer other.Close()
	redirecting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	defer redirecting.Close()

	c := &Client{URL: redirecting.URL, Patience: time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := c.Run(ctx); err == nil || !strings.Contains(err.Error(), "never redirects") {
		t.Fatalf("Run against a redirecting coordinator: %v, want it refused", err)
	}
	if elsewhere.Load() {
		t.Fatal("the program followed the redirect")
	}
}

// TestPacingIsBounded (audit R-054): however long a coordinator says to
// wait, the program waits within bounds, and never overflows.
func TestPacingIsBounded(t *testing.T) {
	var h *httpError
	if err := replyError(http.StatusTooManyRequests, []byte(`{"error":"full"}`), "99999999999"); !errors.As(err, &h) || h.retryAfter != maxRetryAfter {
		t.Fatalf("Retry-After 99999999999: %v, want a wait of %s", err, maxRetryAfter)
	}
	for seconds, want := range map[float64]time.Duration{
		0.5:          500 * time.Millisecond,
		0:            minPoll,
		-1:           minPoll,
		math.NaN():   minPoll,
		math.Inf(1):  maxPoll,
		1e300:        maxPoll,
		600:          maxPoll,
	} {
		if got := pollFor(seconds); got != want {
			t.Errorf("pollFor(%v) = %s, want %s", seconds, got, want)
		}
	}
}
