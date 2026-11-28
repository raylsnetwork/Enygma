package coordinator

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestQueuedAddressMove (audit R-015, L2): a queued participant who asks
// from another address is counted there only if it has room, so moving
// between two addresses never puts more than MaxPerAddress at either; and
// asking from a full one is refused, rather than keeping the place alive
// where it was queued, so one request from a borrowed address cannot park
// a place there for good.
func TestQueuedAddressMove(t *testing.T) {
	h := newHarness(t, Config{Auth: AuthOpen, MaxPerAddress: 1})
	for name, addr := range map[string]string{"holder": "192.0.2.9", "alice": "192.0.2.1", "carol": "192.0.2.1", "dave": "192.0.2.2"} {
		h.addrs[name] = addr
		h.register(name, Registration{Name: name})
	}
	wantStatus(t, "holder", h.ask("holder"), Go, 0)
	wantStatus(t, "alice", h.ask("alice"), Queued, 1) // from A
	h.addrs["alice"] = "192.0.2.2"
	wantStatus(t, "alice", h.ask("alice"), Queued, 1) // moved to B, which had room
	wantStatus(t, "carol", h.ask("carol"), Queued, 2) // A has room again
	h.addrs["alice"] = "192.0.2.1"
	if code, body := h.do("alice", http.MethodPost, "/v1/turn", nil); code != http.StatusTooManyRequests { // A is full
		t.Fatalf("alice from A, full: %d %s, want 429", code, body)
	}
	h.addrs["alice"] = "192.0.2.2"
	wantStatus(t, "alice", h.ask("alice"), Queued, 1) // still counted at B
	if code, body := h.do("dave", http.MethodPost, "/v1/turn", nil); code != http.StatusTooManyRequests {
		t.Fatalf("dave from B, where alice is counted: %d %s, want 429", code, body)
	}
}

// TestIPv6Prefix (audit R-016): an IPv6 /64 is one host's, so it counts as
// one network address for the limits.
func TestIPv6Prefix(t *testing.T) {
	h := newHarness(t, Config{Auth: AuthOpen, MaxPerAddress: 1})
	for name, addr := range map[string]string{"a": "2001:db8::1", "b": "2001:db8::2", "c": "2001:db8:0:1::1"} {
		h.addrs[name] = addr
		h.register(name, Registration{Name: name})
	}
	wantStatus(t, "a", h.ask("a"), Go, 0)
	if code, body := h.do("b", http.MethodPost, "/v1/turn", nil); code != http.StatusTooManyRequests {
		t.Fatalf("b from a's /64: %d %s, want 429", code, body)
	}
	wantStatus(t, "c", h.ask("c"), Queued, 1)
}

// TestAddress (audit R-017, R-018): X-Forwarded-For says where a request
// comes from only when a trusted proxy sent it, and only as an address,
// parsed; anything else counts as the connection's address.
func TestAddress(t *testing.T) {
	h := newHarness(t, Config{Auth: AuthOpen}) // trusts 127.0.0.0/8 and ::1
	for _, tc := range []struct{ remote, forwarded, want string }{
		{"192.0.2.5:1234", "198.51.100.1", "192.0.2.5"},    // not from the proxy: its own address
		{"127.0.0.1:1234", "198.51.100.1", "198.51.100.1"}, // from the proxy: what it added
		{"127.0.0.1:1234", "203.0.113.7, 198.51.100.1", "198.51.100.1"},
		{"127.0.0.1:1234", "not-an-address", "127.0.0.1"}, // the proxy's own: all such count as one
		{"127.0.0.1:1234", strings.Repeat("9", 1<<16), "127.0.0.1"},
		{"127.0.0.1:1234", "2001:db8::1234", "2001:db8::/64"},
		{"[2001:db8::7]:1234", "", "2001:db8::/64"},
		{"127.0.0.1:1234", "::ffff:198.51.100.1", "198.51.100.1"},
	} {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = tc.remote
		if tc.forwarded != "" {
			r.Header.Set("X-Forwarded-For", tc.forwarded)
		}
		if got := h.srv.address(r); got != tc.want {
			t.Errorf("from %s, forwarded for %.40q: %q, want %q", tc.remote, tc.forwarded, got, tc.want)
		}
	}
}

// TestTurnsWithoutContributing (audit R-019): a turn holder whose program
// does not start downloading the state loses the turn, heartbeats or not,
// and an identity given MaxTurns turns without contributing gets no more.
func TestTurnsWithoutContributing(t *testing.T) {
	t.Run("a holder that does not download loses the turn", func(t *testing.T) {
		h := open(t, "alice", "bob")
		wantStatus(t, "alice", h.ask("alice"), Go, 0)
		wantStatus(t, "bob", h.ask("bob"), Queued, 1)
		h.pass(3*time.Minute, "alice", "bob") // alice's heartbeats, but no download
		wantStatus(t, "bob", h.ask("bob"), Go, 0)
	})
	t.Run("no more turns after MaxTurns without contributing", func(t *testing.T) {
		h := open(t, "alice")
		for range 3 {
			wantStatus(t, "alice", h.ask("alice"), Go, 0)
			h.tick(3 * time.Minute) // the turn passes on: no heartbeat, no download
		}
		if code, body := h.do("alice", http.MethodPost, "/v1/turn", nil); code != http.StatusForbidden {
			t.Fatalf("a fourth turn without contributing: %d %s, want 403", code, body)
		}
	})
}

// TestAbsentKeepsPlace (audit R-020): a queued participant not asking just
// when their turn comes is passed over, and keeps their place.
func TestAbsentKeepsPlace(t *testing.T) {
	h := open(t, "alice", "bob", "carol", "dave")
	h.ask("alice")
	h.ask("bob")
	h.ask("carol")
	h.ask("dave")
	h.do("alice", http.MethodGet, "/v1/state", nil)
	h.pass(presence+time.Minute, "alice", "carol", "dave") // bob goes quiet, for less than away
	h.mustContribute("alice")
	wantStatus(t, "carol", h.ask("carol"), Go, 0)
	wantStatus(t, "bob", h.ask("bob"), Queued, 1) // still ahead of dave
	wantStatus(t, "dave", h.ask("dave"), Queued, 2)
}

// TestRegistrationFromAProgram (audit R-021): a registration is JSON from a
// participant's program; a page elsewhere cannot make its visitors' browsers
// spend their address's registrations.
func TestRegistrationFromAProgram(t *testing.T) {
	h := newHarness(t, Config{Auth: AuthOpen})
	for _, headers := range []map[string]string{
		{"Content-Type": "text/plain"},
		{},
		{"Content-Type": "application/json", "Sec-Fetch-Site": "cross-site"},
		{"Content-Type": "application/json", "Origin": "https://elsewhere.example"},
	} {
		req, err := http.NewRequest(http.MethodPost, h.http.URL+"/v1/register", bytes.NewReader([]byte(`{"name":"x"}`)))
		if err != nil {
			t.Fatal(err)
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnsupportedMediaType && resp.StatusCode != http.StatusForbidden {
			t.Errorf("registration with %v: %d, want it refused", headers, resp.StatusCode)
		}
	}
	h.register("x", Registration{Name: "x"}) // as a program sends it
}

// TestNamesAreVisible (audit R-048): a name shows as what it is.
func TestNamesAreVisible(t *testing.T) {
	for name, ok := range map[string]bool{
		"Jane Smith": true,
		"Zoë":        true,
		"李雷":         true,
		"R2-D2 🤖":    true,
		"\U000E0835": false, // a default-ignorable, shown as nothing
		"alice​":     false, // zero-width space
		"alice️":     false, // a variation selector
		"bob⁠":       false, // word joiner
		"-_-":        false, // nothing to read
		"ㅤ":          false, // Hangul filler, shown blank
	} {
		if _, err := cleanName(name); (err == nil) != ok {
			t.Errorf("cleanName(%q): %v, want accepted %v", name, err, ok)
		}
	}
}

// TestTimingAndLimits (audit R-059, R-061): settings that would silently
// stall the queue, or turn a limit off, are refused; the turn holder's
// program is told how often to send heartbeats.
func TestTimingAndLimits(t *testing.T) {
	for _, cfg := range []Config{
		{Presence: time.Second, Poll: 5 * time.Second},
		{Away: time.Second, Presence: 30 * time.Second},
		{MaxPerAddress: -1},
		{GitHubMinAge: -time.Hour},
		{TurnPresence: -time.Second},
		{IPv6Prefix: 129},
	} {
		cfg.Dir, cfg.Auth = t.TempDir(), AuthOpen
		if _, err := New(cfg); err == nil {
			t.Errorf("New(%+v): no error", cfg)
		}
	}
	h := newHarness(t, Config{Auth: AuthOpen, TurnPresence: 6 * time.Second})
	if got := h.srv.heartbeat(); got != 2*time.Second {
		t.Fatalf("heartbeat for a TurnPresence of 6 s: %s, want 2 s", got)
	}
}
