package coordinator

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"enygma_ceremony/transcript"
)

// TestDeny (audit H3): a network denied takes no part from then on: its
// queued participants leave the queue, a turn held from it ends, and its
// requests to take part are refused, while anyone may still follow the
// phase. Lifting it lets it take part again.
func TestDeny(t *testing.T) {
	h := newHarness(t, Config{Auth: AuthOpen})
	for n, a := range map[string]string{"holder": "192.0.2.1", "neighbour": "192.0.2.2", "next": "198.51.100.1", "mover": "203.0.113.5"} {
		h.addrs[n] = a
		h.register(n, Registration{})
	}
	wantStatus(t, "holder", h.ask("holder"), Go, 0)
	h.ask("neighbour")
	h.ask("next")
	h.ask("mover")
	h.addrs["mover"] = "192.0.2.50" // queued from one network, asking now from the one denied
	h.ask("mover")
	if dropped := h.srv.Deny([]netip.Prefix{netip.MustParsePrefix("192.0.2.77/24")}); dropped != 2 {
		t.Fatalf("Deny dropped %d from the queue, want neighbour and mover", dropped)
	}
	wantStatus(t, "next", h.ask("next"), Go, 0) // holder's turn ended
	if got := h.status().Wasted; got != 1 {
		t.Fatalf("turns without a contribution: %d, want holder's", got)
	}
	for _, route := range []struct{ method, path string }{
		{http.MethodPost, "/v1/turn"}, {http.MethodPost, "/v1/heartbeat"},
		{http.MethodGet, "/v1/state"}, {http.MethodPut, "/v1/contribution"},
	} {
		if code, body := h.do("holder", route.method, route.path, nil); code != http.StatusForbidden || !strings.Contains(string(body), "denied") {
			t.Errorf("%s %s from a network denied: %d %s, want 403", route.method, route.path, code, body)
		}
	}
	h.addrs["newcomer"] = "192.0.2.9"
	if code, body := h.tryRegister("newcomer", Registration{}); code != http.StatusForbidden {
		t.Fatalf("registering from a network denied: %d %s, want 403", code, body)
	}
	if code, body := h.do("holder", http.MethodGet, "/v1/status", nil); code != http.StatusOK {
		t.Fatalf("following the phase from a network denied: %d %s, want 200", code, body)
	}
	h.srv.Deny(nil)
	if r := h.ask("holder"); r.Status != Queued {
		t.Fatalf("holder, the deny lifted: %+v, want queued", r)
	}

	h = newHarness(t, Config{Auth: AuthOpen, Deny: []netip.Prefix{netip.MustParsePrefix("2001:db8::/32")}})
	h.addrs["v6"] = "2001:db8:1:2::3"
	if code, body := h.tryRegister("v6", Registration{}); code != http.StatusForbidden {
		t.Fatalf("registering from a network denied at the start: %d %s, want 403", code, body)
	}
}

// TestAppend (audit H3): a contribution made by hand on the latest state
// is added as the next state, with its entry, as an upload would be; one
// that is not on top of the latest, or nameless, is refused, and nothing
// is added without the directory's lock.
func TestAppend(t *testing.T) {
	h := newHarness(t, Config{Auth: AuthOpen})
	state0, err := os.ReadFile(filepath.Join(h.dir, "state-0"))
	if err != nil {
		t.Fatal(err)
	}
	in := filepath.Join(t.TempDir(), "by-hand")
	next := h.contribution(state0)
	if err := os.WriteFile(in, next, 0o600); err != nil {
		t.Fatal(err)
	}
	h.srv.Release() // as a stopped coordinator would: append opens the directory itself
	c, err := New(h.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Append(in, " "); err == nil {
		t.Fatal("appended under no name")
	}
	if _, err := c.Append(filepath.Join(h.dir, "state-0"), "Alice"); err == nil {
		t.Fatal("appended state-0 on top of itself")
	}
	e, err := c.Append(in, "Alice")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(next)
	if want := hex.EncodeToString(sum[:]); e.Index != 1 || e.SHA256 != want || e.Identity != "hand:"+want[:16] || e.Participant != "Alice" {
		t.Fatalf("entry %+v", e)
	}
	if _, err := c.Append(in, "Alice"); err == nil {
		t.Fatal("appended the same contribution twice")
	}
	if _, err := os.Stat(in); err != nil {
		t.Fatalf("the contribution given: %v, want it left as it was", err)
	}
	c.Release()
	h.restart() // a coordinator started on the directory carries on from it
	if _, err := h.srv.Append(in, "Alice"); err == nil || !strings.Contains(err.Error(), "serving") {
		t.Fatalf("Append on a coordinator serving the phase: %v, want it refused", err)
	}
	tr, err := transcript.Load(h.dir)
	if err != nil {
		t.Fatal(err)
	}
	if tr.Latest() != e.SHA256 || h.status().Contributions != 1 {
		t.Fatalf("transcript ends at %s with status %+v, want the contribution added", tr.Latest(), h.status())
	}
	if _, err := c.Append(in, "Alice"); err == nil || !strings.Contains(err.Error(), "released") {
		t.Fatalf("Append after Release: %v, want it refused", err)
	}
}

// uploadPausing uploads next as name's contribution, pausing after its
// first byte, once the coordinator has it coming in, to run during; it
// returns the answer's status.
func uploadPausing(h *harness, name string, next []byte, during func()) int {
	h.t.Helper()
	body, upload := io.Pipe()
	answered := make(chan int, 1)
	go func() {
		req, err := http.NewRequest(http.MethodPut, h.http.URL+"/v1/contribution", body)
		if err != nil {
			answered <- 0
			return
		}
		req.Header.Set("Authorization", "Bearer "+h.tokens[name])
		req.Header.Set("X-Forwarded-For", h.addrs[name])
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			answered <- 0
			return
		}
		resp.Body.Close()
		answered <- resp.StatusCode
	}()
	if _, err := upload.Write(next[:1]); err != nil {
		h.t.Fatal(err)
	}
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(time.Millisecond) {
		h.srv.mu.Lock()
		uploading := h.srv.turn != nil && h.srv.turn.uploading
		h.srv.mu.Unlock()
		if uploading {
			break
		}
		if time.Now().After(deadline) {
			h.t.Fatal("the upload never came in")
		}
	}
	during()
	if _, err := upload.Write(next[1:]); err != nil {
		h.t.Fatal(err)
	}
	if err := upload.Close(); err != nil {
		h.t.Fatal(err)
	}
	return <-answered
}

// holderWithNext gives holder the turn, with next queued, and returns the
// contribution holder will upload.
func holderWithNext(t *testing.T) (*harness, []byte) {
	h := newHarness(t, Config{Auth: AuthOpen})
	for n, a := range map[string]string{"holder": "192.0.2.1", "next": "198.51.100.1"} {
		h.addrs[n] = a
		h.register(n, Registration{})
	}
	wantStatus(t, "holder", h.ask("holder"), Go, 0)
	h.ask("next")
	code, state := h.do("holder", http.MethodGet, "/v1/state", nil)
	if code != http.StatusOK {
		t.Fatalf("downloading: %d", code)
	}
	return h, h.contribution(state)
}

// TestDenyWhileUploading: a turn whose upload is coming in when its network
// is denied is not ended under it, which would let the upload take the next
// holder's turn: the upload is refused before it is committed, and the turn
// passes on.
func TestDenyWhileUploading(t *testing.T) {
	h, next := holderWithNext(t)
	code := uploadPausing(h, "holder", next, func() {
		h.srv.Deny([]netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")})
	})
	if code != http.StatusForbidden {
		t.Fatalf("an upload from a network denied while it came in: %d, want 403", code)
	}
	if tr, err := transcript.Load(h.dir); err != nil || len(tr.Contributions) != 0 {
		t.Fatalf("transcript %+v, %v: want nothing committed", tr, err)
	}
	wantStatus(t, "next", h.ask("next"), Go, 0)
}

// TestCommitOnlyOnItsTurn: an upload is committed only while its turn is
// the one under way, on top of the state it was checked on: were anything
// to change either while it came in, it would take another's place.
func TestCommitOnlyOnItsTurn(t *testing.T) {
	for name, change := range map[string]func(s *Server){
		"another turn":  func(s *Server) { other := *s.turn; s.turn = &other },
		"another state": func(s *Server) { s.latest.hash[0] ^= 1 },
	} {
		t.Run(name, func(t *testing.T) {
			h, next := holderWithNext(t)
			code := uploadPausing(h, "holder", next, func() {
				h.srv.mu.Lock()
				defer h.srv.mu.Unlock()
				change(h.srv)
			})
			if code != http.StatusConflict {
				t.Fatalf("upload: %d, want 409", code)
			}
			if tr, err := transcript.Load(h.dir); err != nil || len(tr.Contributions) != 0 {
				t.Fatalf("transcript %+v, %v: want nothing committed", tr, err)
			}
		})
	}
}

// TestDenyFollowsTheHolder: Deny matches a turn's holder by the address it
// asks from now, not the one it was given the turn at.
func TestDenyFollowsTheHolder(t *testing.T) {
	for _, route := range []string{"/v1/heartbeat", "/v1/turn"} {
		h := newHarness(t, Config{Auth: AuthOpen})
		for n, a := range map[string]string{"holder": "198.51.100.1", "next": "203.0.113.1"} {
			h.addrs[n] = a
			h.register(n, Registration{})
		}
		wantStatus(t, "holder", h.ask("holder"), Go, 0)
		h.ask("next")
		h.addrs["holder"] = "192.0.2.7"
		if code, body := h.do("holder", http.MethodPost, route, nil); code >= 300 {
			t.Fatalf("%s from the holder's new address: %d %s", route, code, body)
		}
		h.srv.Deny([]netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")})
		if r := h.ask("next"); r.Status != Go {
			t.Fatalf("after %s from a network then denied: next %+v, want the holder's turn ended", route, r)
		}
	}
}

// TestDeniedAfterAdmit: a request admitted just before its network is
// denied is refused under the lock: not queued, its place gone, and a turn
// its holder asks from there ends.
func TestDeniedAfterAdmit(t *testing.T) {
	for _, tc := range []struct {
		name, who, path string
		queued          bool
	}{
		{"queueing", "late", "/v1/turn", false}, {"queued, a heartbeat", "late", "/v1/heartbeat", true},
		{"holding, asking", "holder", "/v1/turn", false}, {"holding, a heartbeat", "holder", "/v1/heartbeat", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, Config{Auth: AuthOpen})
			for n, a := range map[string]string{"holder": "203.0.113.1", "late": "198.51.100.5", "next": "198.18.0.1"} {
				h.addrs[n] = a
				h.register(n, Registration{})
			}
			wantStatus(t, "holder", h.ask("holder"), Go, 0)
			h.ask("next")
			if tc.queued {
				h.ask(tc.who)
			}
			h.srv.Deny([]netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")})
			req := httptest.NewRequest(http.MethodPost, tc.path, nil) // from the network denied, as admit had let it in
			req.RemoteAddr = "192.0.2.5:4242"
			req.Header.Set("Authorization", "Bearer "+h.tokens[tc.who])
			rec := httptest.NewRecorder()
			if tc.path == "/v1/turn" {
				h.srv.handleTurn(rec, req)
			} else {
				h.srv.handleHeartbeat(rec, req)
			}
			if rec.Code != http.StatusForbidden {
				t.Fatalf("%s: %d %s, want 403", tc.who, rec.Code, rec.Body)
			}
			if tc.who == "late" && h.status().Queue != 1 {
				t.Fatalf("queue %d, want next alone", h.status().Queue)
			}
			if tc.who == "holder" {
				wantStatus(t, "next", h.ask("next"), Go, 0) // holder's turn ended
			}
		})
	}
}

// TestDeniedContributorIsToldDone: one who has contributed, asking from a
// network denied since, is told they are done, as before the deny.
func TestDeniedContributorIsToldDone(t *testing.T) {
	h := newHarness(t, Config{Auth: AuthOpen})
	h.addrs["alice"] = "192.0.2.10"
	h.register("alice", Registration{})
	wantStatus(t, "alice", h.ask("alice"), Go, 0)
	h.mustContribute("alice")
	h.srv.Deny([]netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")})
	if r := h.ask("alice"); r.Status != Done || r.Index != 1 {
		t.Fatalf("alice: %+v, want done, at 1", r)
	}
}

// TestMovingIntoADeniedNetwork: a participant who comes to a network
// already denied is refused there, and loses their place, or their turn,
// as Deny's would have them.
func TestMovingIntoADeniedNetwork(t *testing.T) {
	h := newHarness(t, Config{Auth: AuthOpen, Deny: []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}})
	for n, a := range map[string]string{"holder": "198.51.100.1", "queued": "203.0.113.1", "next": "198.18.0.1"} {
		h.addrs[n] = a
		h.register(n, Registration{})
	}
	wantStatus(t, "holder", h.ask("holder"), Go, 0)
	h.ask("queued")
	h.ask("next")
	h.addrs["queued"], h.addrs["holder"] = "192.0.2.8", "192.0.2.9"
	if code, body := h.do("queued", http.MethodPost, "/v1/turn", nil); code != http.StatusForbidden {
		t.Fatalf("queued, from the network denied: %d %s, want 403", code, body)
	}
	if code, body := h.do("holder", http.MethodPost, "/v1/heartbeat", nil); code != http.StatusForbidden {
		t.Fatalf("holder, from the network denied: %d %s, want 403", code, body)
	}
	if r := h.ask("next"); r.Status != Go {
		t.Fatalf("next: %+v, want the turn: holder's ended, and queued's place went", r)
	}
}
