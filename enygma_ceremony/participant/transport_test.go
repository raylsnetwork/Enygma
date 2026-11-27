package participant

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
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
		"ceremony.example.org":         true, // becomes https://
		"https://ceremony.example.org": true,
		"http://ceremony.example.org":  false,
		"http://192.0.2.1:8080":        false,
		"http://127.0.0.1:8080":        true,
		"http://localhost:8080":        true,
		"http://[::1]:8080":            true,
		"ftp://ceremony.example.org":   false,
		"https://":                     false,
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

// TestStateSizeIsBounded (audit R-037): the program downloads at most
// MaxState bytes as a state, whether the coordinator announces a larger
// length or streams one, so a coordinator cannot make it decode, and
// allocate for, a state larger than the ceremony plans.
func TestStateSizeIsBounded(t *testing.T) {
	const limit = 100
	for _, c := range []struct {
		name    string
		size    int
		chunked bool
		ok      bool
	}{
		{"at the bound", limit, false, true},
		{"announced past it", limit + 1, false, false},
		{"streamed past it", limit + 1, true, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			state := []byte(strings.Repeat("s", c.size))
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if c.chunked {
					w.WriteHeader(http.StatusOK)
					w.(http.Flusher).Flush() // no Content-Length: the length is known only at the end
				} else {
					w.Header().Set("Content-Length", fmt.Sprint(len(state)))
				}
				_, _ = w.Write(state)
			}))
			defer srv.Close()
			cl := &Client{URL: srv.URL, HTTP: srv.Client(), MaxState: limit}
			got, err := cl.download(context.Background(), sha256.Sum256(state))
			if c.ok && (err != nil || len(got) != c.size) {
				t.Fatalf("a state of %d bytes, at most %d: %d bytes, %v", c.size, limit, len(got), err)
			}
			if !c.ok && (err == nil || !strings.Contains(err.Error(), "-max-state")) {
				t.Fatalf("a state of %d bytes, at most %d: %v, want it refused", c.size, limit, err)
			}
		})
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
		0.5:         500 * time.Millisecond,
		0:           minPoll,
		-1:          minPoll,
		math.NaN():  minPoll,
		math.Inf(1): maxPoll,
		1e300:       maxPoll,
		600:         maxPoll,
	} {
		if got := pollFor(seconds); got != want {
			t.Errorf("pollFor(%v) = %s, want %s", seconds, got, want)
		}
	}
}
