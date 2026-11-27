package coordinator

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"enygma_ceremony/ceremony"
	"enygma_ceremony/transcript"
)

const (
	turnTime = 10 * time.Minute
	presence = 30 * time.Second
	away     = 5 * time.Minute
)

// harness is a coordinator of a small Phase 1 behind an HTTP server, on a
// clock the test moves. Each participant may come from their own network
// address, which the coordinator reads from X-Forwarded-For.
type harness struct {
	t      *testing.T
	dir    string
	cfg    Config
	tokens map[string]string // participant -> token
	addrs  map[string]string // participant -> network address
	clock  atomic.Int64      // Unix nanoseconds
	srv    *Server
	http   *httptest.Server
}

// newHarness starts Phase 1 at domain 8 in a new directory and serves it
// with cfg, filled in with the harness's directory, clock and durations.
func newHarness(t *testing.T, cfg Config) *harness {
	t.Helper()
	h := &harness{t: t, dir: t.TempDir(), tokens: map[string]string{}, addrs: map[string]string{}}
	h.clock.Store(time.Date(2026, 12, 1, 12, 0, 0, 0, time.UTC).UnixNano())
	if _, err := ceremony.NewPhase1(filepath.Join(h.dir, "state-0"), 8); err != nil {
		t.Fatal(err)
	}
	cfg.Dir, cfg.Turn, cfg.Presence, cfg.Away, cfg.Poll = h.dir, turnTime, presence, away, time.Second
	// The harness's requests come from this machine, as from a proxy: their
	// X-Forwarded-For names the participant's address.
	cfg.TrustedProxies = []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("::1/128")}
	cfg.now = func() time.Time { return time.Unix(0, h.clock.Load()) }
	h.cfg = cfg
	h.restart()
	return h
}

// open starts an open phase and registers names, each from their own address.
func open(t *testing.T, names ...string) *harness {
	t.Helper()
	h := newHarness(t, Config{Auth: AuthOpen})
	for i, n := range names {
		h.addrs[n] = fmt.Sprintf("192.0.2.%d", i+1)
		h.register(n, Registration{Name: n})
	}
	return h
}

// restart serves the directory from a new coordinator, as after a stop.
func (h *harness) restart() {
	h.t.Helper()
	if h.http != nil {
		h.http.Close()
	}
	if h.srv != nil {
		h.srv.Release() // as the stopped process would
	}
	srv, err := New(h.cfg)
	if err != nil {
		h.t.Fatal(err)
	}
	h.srv, h.http = srv, httptest.NewServer(srv.Handler())
	h.t.Cleanup(h.http.Close)
}

// tick moves the clock on by d.
func (h *harness) tick(d time.Duration) { h.clock.Add(int64(d)) }

// pass moves the clock on by d, 20 seconds at a time, with each of asking
// asking at every step, as their programs do: a queued participant asking
// for their turn, a turn holder sending heartbeats.
func (h *harness) pass(d time.Duration, asking ...string) {
	h.t.Helper()
	for step := 20 * time.Second; d > 0; d -= step {
		h.tick(min(step, d))
		for _, name := range asking {
			h.ask(name)
		}
	}
}

// do sends a request as participant name (no token when name is "") and
// returns the status and body.
func (h *harness) do(name, method, path string, body []byte) (int, []byte) {
	h.t.Helper()
	req, err := http.NewRequest(method, h.http.URL+path, bytes.NewReader(body))
	if err != nil {
		h.t.Fatal(err)
	}
	if method == http.MethodPost && body != nil {
		req.Header.Set("Content-Type", "application/json") // as a participant's program sends its JSON
	}
	if name != "" {
		token, ok := h.tokens[name]
		if !ok {
			token = "not-a-token"
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if addr, ok := h.addrs[name]; ok {
		req.Header.Set("X-Forwarded-For", addr)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		h.t.Fatal(err)
	}
	return resp.StatusCode, data
}

// tryRegister registers as name with reg and returns the status and body,
// keeping the token on success.
func (h *harness) tryRegister(name string, reg Registration) (int, []byte) {
	h.t.Helper()
	body, err := json.Marshal(reg)
	if err != nil {
		h.t.Fatal(err)
	}
	code, data := h.do(name, http.MethodPost, "/v1/register", body)
	if code == http.StatusOK {
		var s Session
		if err := json.Unmarshal(data, &s); err != nil {
			h.t.Fatal(err)
		}
		h.tokens[name] = s.Token
	}
	return code, data
}

// register registers as name with reg, failing on any status but 200.
func (h *harness) register(name string, reg Registration) Session {
	h.t.Helper()
	code, data := h.tryRegister(name, reg)
	if code != http.StatusOK {
		h.t.Fatalf("registering %s: %d %s", name, code, data)
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		h.t.Fatal(err)
	}
	return s
}

// ask asks for name's turn and returns the reply, failing on any status but 200.
func (h *harness) ask(name string) Reply {
	h.t.Helper()
	code, data := h.do(name, http.MethodPost, "/v1/turn", nil)
	if code != http.StatusOK {
		h.t.Fatalf("%s asking: %d %s", name, code, data)
	}
	var r Reply
	if err := json.Unmarshal(data, &r); err != nil {
		h.t.Fatal(err)
	}
	return r
}

// contribute takes name's turn as package participant would: downloads the
// latest state, contributes and uploads. It returns the upload's status and
// reply body, and the contribution.
func (h *harness) contribute(name string) (int, []byte, []byte) {
	h.t.Helper()
	code, state := h.do(name, http.MethodGet, "/v1/state", nil)
	if code != http.StatusOK {
		h.t.Fatalf("%s downloading: %d %s", name, code, state)
	}
	next := h.contribution(state)
	code, body := h.do(name, http.MethodPut, "/v1/contribution", next)
	return code, body, next
}

// mustContribute takes name's turn and fails unless the contribution is accepted.
func (h *harness) mustContribute(name string) []byte {
	h.t.Helper()
	code, body, next := h.contribute(name)
	if code != http.StatusOK {
		h.t.Fatalf("%s's upload: %d %s", name, code, body)
	}
	return next
}

// contribution returns a contribution on top of the state data.
func (h *harness) contribution(data []byte) []byte {
	h.t.Helper()
	dir := h.t.TempDir()
	in, out := filepath.Join(dir, "in"), filepath.Join(dir, "out")
	if err := os.WriteFile(in, data, 0o644); err != nil {
		h.t.Fatal(err)
	}
	if _, err := ceremony.Contribute(in, out); err != nil {
		h.t.Fatal(err)
	}
	next, err := os.ReadFile(out)
	if err != nil {
		h.t.Fatal(err)
	}
	return next
}

// transcript returns the coordinator's transcript.
func (h *harness) transcript() transcript.Transcript {
	h.t.Helper()
	code, data := h.do("", http.MethodGet, "/v1/transcript", nil)
	var tr transcript.Transcript
	if err := json.Unmarshal(data, &tr); code != http.StatusOK || err != nil {
		h.t.Fatalf("transcript: %d %s %v", code, data, err)
	}
	return tr
}

// status returns the coordinator's status.
func (h *harness) status() Status {
	h.t.Helper()
	code, data := h.do("", http.MethodGet, "/v1/status", nil)
	var st Status
	if err := json.Unmarshal(data, &st); code != http.StatusOK || err != nil {
		h.t.Fatalf("status: %d %s %v", code, data, err)
	}
	return st
}

// wantStatus fails unless r has status, and position when queued.
func wantStatus(t *testing.T, who string, r Reply, status string, position int) {
	t.Helper()
	if r.Status != status || r.Position != position {
		t.Fatalf("%s: %+v, want status %s position %d", who, r, status, position)
	}
}

// sha is the hex sha256 of data.
func sha(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// TestTurnsGoInJoiningOrder: participants take turns in the order they first
// asked; each contribution builds on the one before it, and the transcript
// records who made it.
func TestTurnsGoInJoiningOrder(t *testing.T) {
	h := open(t, "alice", "bob", "carol")
	start := readFile(t, filepath.Join(h.dir, "state-0"))
	wantStatus(t, "alice", h.ask("alice"), Go, 0)
	wantStatus(t, "bob", h.ask("bob"), Queued, 1)
	wantStatus(t, "carol", h.ask("carol"), Queued, 2)

	aliceState := h.mustContribute("alice")
	wantStatus(t, "bob", h.ask("bob"), Go, 0)
	wantStatus(t, "carol", h.ask("carol"), Queued, 1)
	h.mustContribute("bob")

	tr := h.transcript()
	if tr.Start != sha(start) || len(tr.Contributions) != 2 {
		t.Fatalf("transcript %+v", tr)
	}
	if e := tr.Contributions[0]; e.Index != 1 || e.Participant != "alice" || !strings.HasPrefix(e.Identity, "open:") ||
		e.SHA256 != sha(aliceState) || e.Prev != sha(start) {
		t.Fatalf("first entry %+v", e)
	}
	if e := tr.Contributions[1]; e.Index != 2 || e.Participant != "bob" || e.Prev != sha(aliceState) {
		t.Fatalf("second entry %+v", e)
	}
	if err := ceremony.Check(filepath.Join(h.dir, "state-1"), filepath.Join(h.dir, "state-2")); err != nil {
		t.Fatal(err)
	}
}

// TestEachParticipantContributesOnce: once a participant's contribution is
// in, asking again tells them so rather than queueing them.
func TestEachParticipantContributesOnce(t *testing.T) {
	h := open(t, "alice")
	h.ask("alice")
	state := h.mustContribute("alice")
	r := h.ask("alice")
	if r.Status != Done || r.Index != 1 || r.SHA256 != sha(state) {
		t.Fatalf("alice asking again: %+v, want done with contribution 1, %s", r, sha(state))
	}
}

// TestRefuses: the coordinator answers only registered participants, serves
// and takes states only from the turn holder, and checks every upload before
// building on it, ending the turn of one it refuses.
func TestRefuses(t *testing.T) {
	t.Run("an unknown token", func(t *testing.T) {
		h := open(t, "alice")
		if code, body := h.do("mallory", http.MethodPost, "/v1/turn", nil); code != http.StatusUnauthorized {
			t.Fatalf("%d %s, want 401", code, body)
		}
	})
	t.Run("a download or upload out of turn", func(t *testing.T) {
		h := open(t, "alice", "bob")
		h.ask("alice")
		h.ask("bob")
		if code, _ := h.do("bob", http.MethodGet, "/v1/state", nil); code != http.StatusConflict {
			t.Fatalf("bob's download: %d, want 409", code)
		}
		_, state := h.do("alice", http.MethodGet, "/v1/state", nil)
		if code, _ := h.do("bob", http.MethodPut, "/v1/contribution", h.contribution(state)); code != http.StatusConflict {
			t.Fatalf("bob's upload: %d, want 409", code)
		}
	})
	t.Run("a contribution that does not check, passing the turn on", func(t *testing.T) {
		h := open(t, "alice", "bob")
		h.ask("alice")
		h.ask("bob")
		_, state := h.do("alice", http.MethodGet, "/v1/state", nil)
		twice := h.contribution(h.contribution(state)) // contribution 2, on top of state 0
		if code, body := h.do("alice", http.MethodPut, "/v1/contribution", twice); code != http.StatusUnprocessableEntity {
			t.Fatalf("alice's upload: %d %s, want 422", code, body)
		}
		wantStatus(t, "bob", h.ask("bob"), Go, 0)
		if n := len(h.transcript().Contributions); n != 0 {
			t.Fatalf("%d contributions after a refused one", n)
		}
	})
	t.Run("an upload larger than a contribution can be", func(t *testing.T) {
		h := open(t, "alice", "bob")
		h.ask("alice")
		h.ask("bob")
		_, state := h.do("alice", http.MethodGet, "/v1/state", nil)
		padded := append(h.contribution(state), make([]byte, 1<<16)...)
		if code, body := h.do("alice", http.MethodPut, "/v1/contribution", padded); code != http.StatusRequestEntityTooLarge {
			t.Fatalf("alice's upload: %d %s, want 413", code, body)
		}
		wantStatus(t, "bob", h.ask("bob"), Go, 0)
	})
	t.Run("a second upload while the first is coming in", func(t *testing.T) {
		h := open(t, "alice")
		h.ask("alice")
		_, state := h.do("alice", http.MethodGet, "/v1/state", nil)
		next := h.contribution(state)
		body, rest := io.Pipe()
		req, err := http.NewRequest(http.MethodPut, h.http.URL+"/v1/contribution", body)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+h.tokens["alice"])
		first := make(chan int, 1)
		go func() {
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				first <- 0
				return
			}
			resp.Body.Close()
			first <- resp.StatusCode
		}()
		if _, err := rest.Write(next[:len(next)/2]); err != nil {
			t.Fatal(err)
		}
		for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(5 * time.Millisecond) {
			if files, _ := filepath.Glob(filepath.Join(h.dir, "incoming", "upload-*")); len(files) > 0 {
				break // the first upload is coming in
			}
			if time.Now().After(deadline) {
				t.Fatal("the first upload never came in")
			}
		}
		if code, body := h.do("alice", http.MethodPut, "/v1/contribution", next); code != http.StatusConflict {
			t.Fatalf("a second upload during the first: %d %s, want 409", code, body)
		}
		if _, err := rest.Write(next[len(next)/2:]); err != nil {
			t.Fatal(err)
		}
		if err := rest.Close(); err != nil {
			t.Fatal(err)
		}
		if code := <-first; code != http.StatusOK {
			t.Fatalf("the first upload: %d, want 200", code)
		}
	})
}

// TestOneTokenAtOnce: requests with one session's token may come at once,
// as the turn holder's heartbeats do beside its download and upload, and
// from changing network addresses. Run with -race.
func TestOneTokenAtOnce(t *testing.T) {
	h := open(t, "alice")
	wantStatus(t, "alice", h.ask("alice"), Go, 0)
	var wg sync.WaitGroup
	for g := range 16 {
		wg.Go(func() {
			for range 200 {
				req, err := http.NewRequest(http.MethodPost, h.http.URL+"/v1/heartbeat", nil)
				if err != nil {
					t.Error(err)
					return
				}
				req.Header.Set("Authorization", "Bearer "+h.tokens["alice"])
				req.Header.Set("X-Forwarded-For", fmt.Sprintf("192.0.2.%d", 10+g))
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					t.Error(err)
					return
				}
				resp.Body.Close()
				if resp.StatusCode != http.StatusNoContent {
					t.Errorf("heartbeat: %d, want 204", resp.StatusCode)
					return
				}
			}
		})
	}
	wg.Wait()
}

// TestTurnRunsOut: a turn holder who has not uploaded by the deadline loses
// the turn to the next in the queue, and their late upload is refused.
func TestTurnRunsOut(t *testing.T) {
	h := open(t, "alice", "bob")
	wantStatus(t, "alice", h.ask("alice"), Go, 0)
	_, state := h.do("alice", http.MethodGet, "/v1/state", nil)
	h.pass(turnTime-20*time.Second, "alice", "bob") // alice's program is still at it, short of the deadline
	wantStatus(t, "alice", h.ask("alice"), Go, 0)
	h.pass(21*time.Second, "bob")
	wantStatus(t, "bob", h.ask("bob"), Go, 0)
	if code, _ := h.do("alice", http.MethodPut, "/v1/contribution", h.contribution(state)); code != http.StatusConflict {
		t.Fatalf("alice's late upload: %d, want 409", code)
	}
	wantStatus(t, "alice", h.ask("alice"), Queued, 1)
}

// TestTurnHolderHeartbeats: a turn holder's program keeps telling the
// coordinator it is still at it; one that goes silent, as a closed laptop
// does, loses the turn long before its deadline. A heartbeat never queues:
// one sent on by a program still contributing after its turn has passed
// on is refused.
func TestTurnHolderHeartbeats(t *testing.T) {
	t.Run("passes the turn on when its holder goes silent", func(t *testing.T) {
		h := open(t, "alice", "bob")
		h.ask("alice")
		_, state := h.do("alice", http.MethodGet, "/v1/state", nil)
		h.pass(2*time.Minute+time.Second, "bob") // alice's program says nothing
		wantStatus(t, "bob", h.ask("bob"), Go, 0)
		if code, _ := h.do("alice", http.MethodPut, "/v1/contribution", h.contribution(state)); code != http.StatusConflict {
			t.Fatalf("alice's upload after going silent: %d, want 409", code)
		}
	})
	t.Run("keeps the turn of a holder sending heartbeats", func(t *testing.T) {
		h := open(t, "alice", "bob")
		first := h.ask("alice")
		_, state := h.do("alice", http.MethodGet, "/v1/state", nil)
		for range 15 { // 5 minutes: a slow contribution, its program still at it
			h.pass(20*time.Second, "bob")
			if code, body := h.do("alice", http.MethodPost, "/v1/heartbeat", nil); code != http.StatusNoContent {
				t.Fatalf("alice's heartbeat: %d %s, want 204", code, body)
			}
		}
		if r := h.ask("alice"); r.Status != Go || !r.Deadline.Equal(first.Deadline) {
			t.Fatalf("alice: %+v; want the turn she was handed, until %s", r, first.Deadline)
		}
		if code, body := h.do("alice", http.MethodPut, "/v1/contribution", h.contribution(state)); code != http.StatusOK {
			t.Fatalf("alice's upload: %d %s", code, body)
		}
	})
	t.Run("keeps the turn of a holder asking for it", func(t *testing.T) {
		h := open(t, "alice", "bob")
		first := h.ask("alice")
		_, state := h.do("alice", http.MethodGet, "/v1/state", nil)
		h.pass(5*time.Minute, "alice", "bob")
		if r := h.ask("alice"); r.Status != Go || !r.Deadline.Equal(first.Deadline) {
			t.Fatalf("alice: %+v; want the turn she was handed, until %s", r, first.Deadline)
		}
		if code, body := h.do("alice", http.MethodPut, "/v1/contribution", h.contribution(state)); code != http.StatusOK {
			t.Fatalf("alice's upload: %d %s", code, body)
		}
	})
	t.Run("never queues a holder whose turn has passed on", func(t *testing.T) {
		h := open(t, "alice", "bob")
		h.ask("alice")
		_, state := h.do("alice", http.MethodGet, "/v1/state", nil)
		for range turnTime / (20 * time.Second) { // her program, at it to the deadline
			h.pass(20*time.Second, "bob")
			if code, body := h.do("alice", http.MethodPost, "/v1/heartbeat", nil); code != http.StatusNoContent {
				t.Fatalf("alice's heartbeat in her turn: %d %s, want 204", code, body)
			}
		}
		h.pass(time.Second, "bob") // and on, past it
		wantStatus(t, "bob", h.ask("bob"), Go, 0)
		if code, body := h.do("alice", http.MethodPost, "/v1/heartbeat", nil); code != http.StatusConflict {
			t.Fatalf("alice's heartbeat after her turn: %d %s, want 409", code, body)
		}
		h.mustContribute("bob")
		if st := h.status(); st.Queue != 0 || st.Contributing != "" {
			t.Fatalf("status %+v: alice's heartbeat queued her", st)
		}
		if code, _ := h.do("alice", http.MethodPut, "/v1/contribution", h.contribution(state)); code != http.StatusConflict {
			t.Fatalf("alice's late upload: %d, want 409", code)
		}
	})
}

// TestQuietParticipants: a queued participant may go quiet for a while (a
// laptop asleep, a network down) and keep their place, but loses it if they
// are not asking when their turn comes, or stay away too long; asking again
// queues them last.
func TestQuietParticipants(t *testing.T) {
	t.Run("keeps the place of one who comes back before their turn", func(t *testing.T) {
		h := open(t, "alice", "bob", "carol")
		h.ask("alice")
		h.ask("bob")
		h.ask("carol")
		h.pass(away-time.Second, "alice", "carol")    // bob goes quiet; carol keeps asking
		wantStatus(t, "bob", h.ask("bob"), Queued, 1) // still ahead of carol
		h.mustContribute("alice")
		wantStatus(t, "bob", h.ask("bob"), Go, 0)
	})
	t.Run("passes over one not asking when their turn comes", func(t *testing.T) {
		h := open(t, "alice", "bob", "carol")
		h.ask("alice")
		h.ask("bob")
		h.ask("carol")
		h.tick(presence + time.Second)
		h.ask("carol") // carol still asks; bob has gone quiet
		h.mustContribute("alice")
		wantStatus(t, "carol", h.ask("carol"), Go, 0)
		wantStatus(t, "bob", h.ask("bob"), Queued, 1)
	})
	t.Run("drops one away too long", func(t *testing.T) {
		h := open(t, "alice", "bob", "carol")
		h.ask("alice")
		h.ask("bob")
		h.ask("carol")
		h.pass(away+time.Second, "alice", "carol") // alice holds the turn; bob goes quiet for too long
		if code, _ := h.do("bob", http.MethodPost, "/v1/turn", nil); code != http.StatusUnauthorized {
			t.Fatalf("bob after too long away: %d, want 401: his registration is forgotten", code)
		}
		h.register("bob", Registration{Name: "bob"}) // as his program does
		wantStatus(t, "bob", h.ask("bob"), Queued, 2)
	})
}

// TestCarriesOnAfterRestart: a coordinator restarted on the phase's directory
// knows every contribution so far, and the next one builds on the latest. It
// forgets the registrations, which participants' programs make again.
func TestCarriesOnAfterRestart(t *testing.T) {
	h := open(t, "alice", "bob")
	h.ask("alice")
	state := h.mustContribute("alice")
	h.restart()
	if code, _ := h.do("bob", http.MethodPost, "/v1/turn", nil); code != http.StatusUnauthorized {
		t.Fatalf("bob's old registration after a restart: %d, want 401", code)
	}
	h.register("bob", Registration{Name: "bob"})
	wantStatus(t, "bob", h.ask("bob"), Go, 0)
	h.mustContribute("bob")
	if got := h.srv.States(); len(got) != 2 {
		t.Fatalf("States() = %v, want 2", got)
	}
	if e := h.transcript().Contributions[1]; e.Prev != sha(state) {
		t.Fatalf("contribution 2 builds on %s, want alice's %s", e.Prev, sha(state))
	}
}

// TestRestartRefusesABrokenTranscript: a directory whose entries do not chain
// from state-0, whose state-0 is no phase's start, or whose latest state is
// not the one its entry names, is not a phase to carry on.
func TestRestartRefusesABrokenTranscript(t *testing.T) {
	// contributed is a phase directory holding alice's contribution.
	contributed := func(t *testing.T) *harness {
		h := open(t, "alice")
		h.ask("alice")
		h.mustContribute("alice")
		return h
	}
	refused := func(t *testing.T, h *harness, why string) {
		t.Helper()
		h.srv.Release() // refused for the transcript, not for the lock
		if _, err := New(h.cfg); err == nil || strings.Contains(err.Error(), "another coordinator") {
			t.Fatalf("New: %v on %s, want it refused", err, why)
		}
	}
	t.Run("entries that do not chain", func(t *testing.T) {
		h := contributed(t)
		entry := filepath.Join(h.dir, "state-1.json")
		data := bytes.Replace(readFile(t, entry), []byte(`"index": 1`), []byte(`"index": 2`), 1)
		if err := os.WriteFile(entry, data, 0o644); err != nil {
			t.Fatal(err)
		}
		refused(t, h, "an entry numbered 2 for state-1")
	})
	t.Run("a state-0 that is a contribution", func(t *testing.T) {
		h := contributed(t)
		if err := os.Rename(filepath.Join(h.dir, "state-1"), filepath.Join(h.dir, "state-0")); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Join(h.dir, "state-1.json")); err != nil {
			t.Fatal(err)
		}
		refused(t, h, "contribution 1 as state-0")
	})
	t.Run("a state-0 that is a sealed SRS", func(t *testing.T) {
		h := contributed(t)
		srs := filepath.Join(t.TempDir(), "phase1.srs")
		if _, err := ceremony.SealPhase1(srs, bytes.Repeat([]byte{7}, ceremony.MinBeacon), []string{filepath.Join(h.dir, "state-1")}); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"state-1", "state-1.json"} {
			if err := os.Remove(filepath.Join(h.dir, name)); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.Rename(srs, filepath.Join(h.dir, "state-0")); err != nil {
			t.Fatal(err)
		}
		refused(t, h, "an SRS as state-0")
	})
	t.Run("a latest state other than its entry names", func(t *testing.T) {
		h := contributed(t)
		latest := filepath.Join(h.dir, "state-1")
		data := readFile(t, latest)
		data[len(data)-1] ^= 1
		if err := os.WriteFile(latest, data, 0o644); err != nil {
			t.Fatal(err)
		}
		refused(t, h, "a state-1 changed after its entry")
	})
}

// TestClosing: a closed phase hands out no more turns, but the contribution
// under way still counts. It closes when told, or at its scheduled time.
func TestClosing(t *testing.T) {
	t.Run("when told, letting the turn finish", func(t *testing.T) {
		h := open(t, "alice", "bob")
		h.ask("alice")
		h.ask("bob")
		h.srv.Close()
		if !h.srv.Busy() {
			t.Fatal("not busy during alice's turn")
		}
		h.mustContribute("alice")
		wantStatus(t, "bob", h.ask("bob"), Closed, 0)
		if h.srv.Busy() {
			t.Fatal("busy after the last turn")
		}
	})
	t.Run("at its time", func(t *testing.T) {
		h := newHarness(t, Config{Auth: AuthOpen, Notice: "Phase 2 opens on Monday"})
		h.cfg.CloseAt = time.Unix(0, h.clock.Load()).Add(time.Hour)
		h.restart()
		h.register("alice", Registration{Name: "alice"})
		wantStatus(t, "alice", h.ask("alice"), Go, 0)
		h.tick(time.Hour)
		h.register("bob", Registration{Name: "bob"})
		r := h.ask("bob")
		if r.Status != Closed || r.Notice != "Phase 2 opens on Monday" || !h.srv.Closed() {
			t.Fatalf("bob after the close: %+v", r)
		}
	})
}

// TestStatesAreServed: anyone can download every state of the transcript.
func TestStatesAreServed(t *testing.T) {
	h := open(t, "alice")
	h.ask("alice")
	state := h.mustContribute("alice")
	for i, want := range [][]byte{readFile(t, filepath.Join(h.dir, "state-0")), state} {
		code, got := h.do("", http.MethodGet, fmt.Sprintf("/v1/states/%d", i), nil)
		if code != http.StatusOK || !bytes.Equal(got, want) {
			t.Fatalf("state %d: %d, %d bytes, want %d", i, code, len(got), len(want))
		}
	}
	if code, _ := h.do("", http.MethodGet, "/v1/states/2", nil); code != http.StatusNotFound {
		t.Fatalf("state 2: %d, want 404", code)
	}
}

// TestOpenPhase: anyone may register, under a name of their choosing or
// none, and contribute once per registration.
func TestOpenPhase(t *testing.T) {
	t.Run("registers anyone and records the name they chose", func(t *testing.T) {
		h := open(t, "alice")
		wantStatus(t, "alice", h.ask("alice"), Go, 0)
		h.mustContribute("alice")
		e := h.transcript().Contributions[0]
		if e.Participant != "alice" || !strings.HasPrefix(e.Identity, "open:") {
			t.Fatalf("entry %+v, want alice, open:…", e)
		}
	})
	t.Run("names a participant who chose no name anonymous", func(t *testing.T) {
		h := open(t)
		if s := h.register("someone", Registration{}); s.Participant != "anonymous" {
			t.Fatalf("registered as %q, want anonymous", s.Participant)
		}
	})
	t.Run("records any name a person types, tidied", func(t *testing.T) {
		h := open(t)
		for typed, shown := range map[string]string{"  Jane   Smith ": "Jane Smith", "Zoë O'Brien-Lee": "Zoë O'Brien-Lee", "李雷": "李雷"} {
			if s := h.register("x", Registration{Name: typed}); s.Participant != shown {
				t.Errorf("registered %q as %q, want %q", typed, s.Participant, shown)
			}
		}
	})
	t.Run("refuses a name that could pass for something else", func(t *testing.T) {
		h := open(t)
		for _, name := range []string{"ok\x1b[2Jcleared", "evil\u202egnp.exe", strings.Repeat("a", 65)} {
			if code, body := h.tryRegister("x", Registration{Name: name}); code != http.StatusBadRequest {
				t.Errorf("%q: %d %s, want 400", name, code, body)
			}
		}
	})
	t.Run("forgets a registration unused for long", func(t *testing.T) {
		h := open(t, "alice")
		h.tick(away + time.Second)
		if code, body := h.do("alice", http.MethodPost, "/v1/turn", nil); code != http.StatusUnauthorized {
			t.Fatalf("%d %s, want 401", code, body)
		}
	})
}

// TestAddressLimits: one network address cannot fill an open phase's queue,
// or make registrations without end.
func TestAddressLimits(t *testing.T) {
	t.Run("bounds the participants queued from one address", func(t *testing.T) {
		h := newHarness(t, Config{Auth: AuthOpen, MaxPerAddress: 2})
		for _, n := range []string{"a", "b", "c"} {
			h.addrs[n] = "192.0.2.1"
			h.register(n, Registration{Name: n})
		}
		h.addrs["d"] = "192.0.2.2"
		h.register("d", Registration{Name: "d"})
		h.ask("a")
		h.ask("b")
		if code, body := h.do("c", http.MethodPost, "/v1/turn", nil); code != http.StatusTooManyRequests {
			t.Fatalf("a third from one address: %d %s, want 429", code, body)
		}
		wantStatus(t, "d", h.ask("d"), Queued, 2)
		h.mustContribute("a")
		wantStatus(t, "c", h.ask("c"), Queued, 2)
	})
	t.Run("bounds the registrations from one address an hour", func(t *testing.T) {
		h := newHarness(t, Config{Auth: AuthOpen})
		h.addrs["x"] = "192.0.2.1"
		for range registrationsPerHour {
			h.register("x", Registration{})
		}
		if code, body := h.tryRegister("x", Registration{}); code != http.StatusTooManyRequests {
			t.Fatalf("one registration too many: %d %s, want 429", code, body)
		}
		h.tick(time.Hour)
		h.register("x", Registration{})
	})
}

// The GitHub OAuth app of the tests' GitHub phases.
const (
	testClientID     = "Iv1.test"
	testClientSecret = "test-secret"
)

// fakeGitHub answers GitHub's /user for the accounts it knows, by token, and
// GitHub's "check a token" for testClientID: every token it knows is the
// app's, but those starting "other-app-", which are another app's.
func fakeGitHub(t *testing.T, accounts map[string]githubAccount) string {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/applications/"+testClientID+"/token" {
			id, secret, ok := r.BasicAuth()
			if !ok || id != testClientID || secret != testClientSecret {
				http.Error(w, `{"message":"Requires authentication"}`, http.StatusUnauthorized)
				return
			}
			var body struct {
				AccessToken string `json:"access_token"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, `{"message":"Problems parsing JSON"}`, http.StatusBadRequest)
				return
			}
			if _, known := accounts[body.AccessToken]; !known || strings.HasPrefix(body.AccessToken, "other-app-") {
				http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
				return
			}
			fmt.Fprint(w, `{}`)
			return
		}
		a, ok := accounts[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")]
		if r.URL.Path != "/user" || !ok {
			http.Error(w, `{"message":"Bad credentials"}`, http.StatusUnauthorized)
			return
		}
		if err := json.NewEncoder(w).Encode(a); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(s.Close)
	return s.URL
}

// TestGitHubPhase: a GitHub phase admits a signed-in account old enough,
// under its login, and counts one contribution per account however often it
// registers.
func TestGitHubPhase(t *testing.T) {
	now := time.Date(2026, 12, 1, 12, 0, 0, 0, time.UTC)
	api := fakeGitHub(t, map[string]githubAccount{
		"old-token":           {Login: "octocat", ID: 583231, Created: now.AddDate(-5, 0, 0)},
		"young-token":         {Login: "newbie", ID: 99, Created: now.AddDate(0, 0, -3)},
		"other-app-old-token": {Login: "octocat", ID: 583231, Created: now.AddDate(-5, 0, 0)},
		"no-id-token":         {Login: "nobody", Created: now.AddDate(-5, 0, 0)},
		"no-age-token":        {Login: "ageless", ID: 7},
		"odd-login-token":     {Login: "octo‮cat", ID: 8, Created: now.AddDate(-5, 0, 0)},
	})
	github := func(t *testing.T) *harness {
		return newHarness(t, Config{Auth: AuthGitHub, GitHubClientID: testClientID, GitHubClientSecret: testClientSecret,
			GitHubMinAge: 30 * 24 * time.Hour, GitHubAPI: api})
	}

	t.Run("admits an account under its login, once", func(t *testing.T) {
		h := github(t)
		if s := h.register("octocat", Registration{GitHubToken: "old-token"}); s.Participant != "octocat" {
			t.Fatalf("registered as %q", s.Participant)
		}
		h.ask("octocat")
		h.mustContribute("octocat")
		if e := h.transcript().Contributions[0]; e.Participant != "octocat" || e.Identity != "github:583231" {
			t.Fatalf("entry %+v", e)
		}
		h.register("octocat", Registration{GitHubToken: "old-token"})
		if r := h.ask("octocat"); r.Status != Done {
			t.Fatalf("octocat registering again: %+v, want done", r)
		}
	})
	t.Run("remembers an account's contribution across a restart", func(t *testing.T) {
		h := github(t)
		h.register("octocat", Registration{GitHubToken: "old-token"})
		h.ask("octocat")
		h.mustContribute("octocat")
		h.restart()
		h.register("octocat", Registration{GitHubToken: "old-token"})
		if r := h.ask("octocat"); r.Status != Done || r.Index != 1 {
			t.Fatalf("octocat after a restart: %+v, want done with contribution 1", r)
		}
	})
	t.Run("refuses a sign-in GitHub does not accept", func(t *testing.T) {
		h := github(t)
		if code, body := h.tryRegister("x", Registration{GitHubToken: "forged"}); code != http.StatusUnauthorized {
			t.Fatalf("%d %s, want 401", code, body)
		}
	})
	t.Run("refuses an account younger than the minimum", func(t *testing.T) {
		h := github(t)
		if code, body := h.tryRegister("newbie", Registration{GitHubToken: "young-token"}); code != http.StatusForbidden {
			t.Fatalf("%d %s, want 403", code, body)
		}
	})
	// Audit R-022: a token /user accepts may be any app's.
	t.Run("refuses a token of another app", func(t *testing.T) {
		h := github(t)
		if code, body := h.tryRegister("x", Registration{GitHubToken: "other-app-old-token"}); code != http.StatusUnauthorized {
			t.Fatalf("%d %s, want 401", code, body)
		}
	})
	// Audit R-047: an answer missing a field admits no one.
	t.Run("refuses an answer without an id, a creation time, or a login that is a name", func(t *testing.T) {
		h := github(t)
		for _, token := range []string{"no-id-token", "no-age-token", "odd-login-token"} {
			if code, body := h.tryRegister("x", Registration{GitHubToken: token}); code != http.StatusBadGateway {
				t.Errorf("%s: %d %s, want 502", token, code, body)
			}
		}
	})
	// Audit R-049: a token that cannot be one is refused before GitHub is asked.
	t.Run("refuses a token holding a byte no token has", func(t *testing.T) {
		var logged bytes.Buffer
		h := newHarness(t, Config{Auth: AuthGitHub, GitHubClientID: testClientID, GitHubClientSecret: testClientSecret,
			GitHubAPI: api, Log: log.New(&logged, "", 0)})
		for _, token := range []string{"gho_SECRETPART\x00x", "gho_SECRETPART\nx", "gho SECRETPART", strings.Repeat("g", 256)} {
			if code, body := h.tryRegister("x", Registration{GitHubToken: token}); code != http.StatusBadRequest {
				t.Errorf("%q: %d %s, want 400", token, code, body)
			}
		}
		if strings.Contains(logged.String(), "SECRETPART") {
			t.Errorf("a token reached the log: %q", logged.String())
		}
	})
	t.Run("tells participants' programs to sign in with its app", func(t *testing.T) {
		h := github(t)
		_, data := h.do("", http.MethodGet, "/v1/config", nil)
		var c ClientConfig
		if err := json.Unmarshal(data, &c); err != nil || c.Auth != AuthGitHub || c.GitHubClientID != testClientID ||
			strings.Contains(string(data), testClientSecret) {
			t.Fatalf("config %s, %v", data, err)
		}
	})
}

// TestWaitIsEstimated: a queued participant is told about how long they will
// wait, from how long the last turns took, and the page how long a check
// takes, from the checks of the last accepted contributions.
func TestWaitIsEstimated(t *testing.T) {
	h := open(t, "alice", "bob", "carol")
	h.ask("alice")
	h.ask("bob")
	h.ask("carol")
	h.pass(2*time.Minute, "alice", "bob", "carol")
	h.mustContribute("alice")
	if r := h.ask("carol"); r.Position != 1 || r.Wait != 120 {
		t.Fatalf("carol: %+v, want position 1, a wait of 120 s", r)
	}
	if st := h.status(); st.AverageTurn != 120 || st.Contributing != "bob" || st.AverageCheck <= 0 {
		t.Fatalf("status %+v", st)
	}

	t.Run("after a restart, from the transcript", func(t *testing.T) {
		h.do("bob", http.MethodGet, "/v1/state", nil) // bob's program starts downloading at once, as programs do
		h.pass(3*time.Minute, "bob", "carol")
		h.mustContribute("bob") // 3 minutes after alice's
		h.restart()
		if st := h.status(); st.AverageTurn != 180 {
			t.Fatalf("average turn %v s after a restart, want 180", st.AverageTurn)
		}
	})
	t.Run("after a restart, leaving out time with no turn under way", func(t *testing.T) {
		h := open(t, "alice")
		h.ask("alice")
		h.pass(2*time.Minute, "alice")
		h.mustContribute("alice")
		h.tick(time.Hour) // nobody asking
		for i, name := range []string{"bob", "carol"} {
			h.addrs[name] = fmt.Sprintf("192.0.2.%d", 20+i)
			h.register(name, Registration{Name: name})
			h.ask(name)
			h.pass(3*time.Minute, name)
			h.mustContribute(name)
		}
		h.restart()
		if st := h.status(); st.AverageTurn != 180 {
			t.Fatalf("average turn %v s after a restart, want 180: the hour without a turn counted", st.AverageTurn)
		}
	})
	t.Run("from accepted contributions' checks only", func(t *testing.T) {
		h := open(t, "alice")
		h.ask("alice")
		if code, body := h.do("alice", http.MethodPut, "/v1/contribution", []byte("not a state")); code != http.StatusUnprocessableEntity {
			t.Fatalf("alice's upload: %d %s, want 422", code, body)
		}
		if st := h.status(); st.AverageCheck != 0 {
			t.Fatalf("average check %v s from a refused upload alone, want none", st.AverageCheck)
		}
	})
}

// TestPage: the phase's page is served to anyone, runs nothing but its own
// script, and points to the ceremony's release; the coordinator serves no
// participant code, so it never chooses the code that draws a participant's
// randomness.
func TestPage(t *testing.T) {
	const release = "https://github.com/example/Enygma/releases/tag/ceremony-v1"
	h := newHarness(t, Config{Auth: AuthOpen, ReleaseURL: release})

	resp, body := h.get("/", nil)
	if resp.StatusCode != http.StatusOK || !bytes.Contains(body, []byte("<title>Enygma setup ceremony</title>")) {
		t.Fatalf("page: %d, %d bytes", resp.StatusCode, len(body))
	}
	// Names on the page are participants' own: nothing but its own script, by its hash, and
	// requests to this coordinator may run (audit R-051); it starts no workers, and is never
	// framed (R-052).
	_, after, _ := bytes.Cut(body, []byte("<script>"))
	script, _, _ := bytes.Cut(after, []byte("</script>"))
	sum := sha256.Sum256(script)
	scriptHash := "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
	csp := resp.Header.Get("Content-Security-Policy")
	for _, want := range []string{"default-src 'none'", "connect-src 'self'", "script-src " + scriptHash, "frame-ancestors 'none'", "base-uri 'none'", "form-action 'none'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("page served with Content-Security-Policy %q, without %q", csp, want)
		}
	}
	if strings.Contains(csp, "unsafe-inline") || strings.Contains(csp, "worker-src") {
		t.Errorf("page served with Content-Security-Policy %q", csp)
	}
	if got := resp.Header.Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("page served with X-Frame-Options %q, want DENY", got)
	}
	if got := h.status().ReleaseURL; got != release {
		t.Fatalf("status gives release %q, want %q", got, release)
	}
	for _, path := range []string{"/web/worker.js", "/web/heartbeat.js", "/web/ceremony.wasm", "/download/ceremony-linux-amd64", "/download/SHA256SUMS"} {
		if resp, _ := h.get(path, nil); resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s: %d, want 404: the coordinator serves no participant code", path, resp.StatusCode)
		}
	}
}

// TestScheduledCloseStaysClosed (audit R-033, R-071): a phase closed at its
// scheduled time stays closed when its coordinator restarts, even without
// CloseAt, so it can be served read-only; a phase interrupted (Close), as
// for a restart, carries on.
func TestScheduledCloseStaysClosed(t *testing.T) {
	h := newHarness(t, Config{Auth: AuthOpen, CloseAt: time.Date(2026, 12, 1, 13, 0, 0, 0, time.UTC)})
	if h.status().Closed {
		t.Fatal("closed before its time")
	}
	h.tick(2 * time.Hour)
	if !h.status().Closed {
		t.Fatal("open after its time")
	}
	h.cfg.CloseAt = time.Time{}
	h.restart()
	if !h.status().Closed {
		t.Fatal("a restart without CloseAt reopened a phase closed at its time")
	}

	t.Run("an interrupted phase carries on after a restart", func(t *testing.T) {
		h := newHarness(t, Config{Auth: AuthOpen})
		h.srv.Close()
		h.restart()
		if h.status().Closed {
			t.Fatal("an interrupted phase stayed closed after a restart")
		}
	})
}

// TestReleaseURL: the release address the page links to must be https.
func TestReleaseURL(t *testing.T) {
	for _, u := range []string{"http://github.com/example/Enygma/releases", "javascript:alert(1)", "https://", "github.com/example"} {
		if _, err := New(Config{Dir: t.TempDir(), Auth: AuthOpen, ReleaseURL: u}); err == nil || !strings.Contains(err.Error(), "ReleaseURL") {
			t.Errorf("ReleaseURL %q: %v, want refused", u, err)
		}
	}
}

// get fetches path from h's coordinator with headers, and returns the
// response and its body.
func (h *harness) get(path string, headers map[string]string) (*http.Response, []byte) {
	h.t.Helper()
	req, err := http.NewRequest(http.MethodGet, h.http.URL+path, nil)
	if err != nil {
		h.t.Fatal(err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		h.t.Fatal(err)
	}
	return resp, body
}

// TestNoGitHubRelay (audit R-023): the coordinator relays nothing to GitHub
// for anyone: a participant's program signs in with GitHub itself. A GitHub
// phase needs its app's client secret, to admit only the app's tokens.
func TestNoGitHubRelay(t *testing.T) {
	h := newHarness(t, Config{Auth: AuthGitHub, GitHubClientID: testClientID, GitHubClientSecret: testClientSecret})
	for _, path := range []string{"/github/login/device/code", "/github/login/oauth/access_token"} {
		resp, err := http.Post(h.http.URL+path, "application/x-www-form-urlencoded", strings.NewReader("client_id="+testClientID))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("POST %s: %d, want 404", path, resp.StatusCode)
		}
	}
	if _, err := New(Config{Dir: t.TempDir(), Auth: AuthGitHub, GitHubClientID: testClientID}); err == nil || !strings.Contains(err.Error(), "secret") {
		t.Fatalf("a GitHub phase without its app's secret: %v, want it refused", err)
	}
}

// readFile reads path or fails the test.
func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
