package participant

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"enygma_ceremony/ceremony"
	"enygma_ceremony/coordinator"
)

// phase is a coordinator of a small Phase 1, serving over HTTP. Its
// handler can be swapped for a restarted coordinator's.
type phase struct {
	dir     string
	cfg     coordinator.Config
	srv     *coordinator.Server
	handler atomic.Pointer[http.Handler]
	url     string
}

// newPhase starts Phase 1 at domain 8 under cfg, filled in with the
// directory and short durations.
func newPhase(t *testing.T, cfg coordinator.Config) *phase {
	t.Helper()
	p := &phase{dir: t.TempDir()}
	if _, err := ceremony.NewPhase1(filepath.Join(p.dir, "state-0"), 8); err != nil {
		t.Fatal(err)
	}
	cfg.Dir, cfg.Turn, cfg.Presence, cfg.Poll = p.dir, time.Minute, 5*time.Second, 20*time.Millisecond
	p.cfg = cfg
	p.restart(t)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		(*p.handler.Load()).ServeHTTP(w, r)
	}))
	t.Cleanup(s.Close)
	p.url = s.URL
	return p
}

// restart serves the phase from a new coordinator on the same directory, as
// after a stop: it remembers the transcript, and forgets every registration.
func (p *phase) restart(t *testing.T) {
	t.Helper()
	srv, err := coordinator.New(p.cfg)
	if err != nil {
		t.Fatal(err)
	}
	h := srv.Handler()
	p.srv = srv
	p.handler.Store(&h)
}

// client is name's client of the phase.
func (p *phase) client(t *testing.T, name string) *Client {
	return &Client{URL: p.url, Name: name}
}

// runAll runs clients at once and returns their results.
func runAll(t *testing.T, clients ...*Client) []Result {
	t.Helper()
	results := make([]Result, len(clients))
	errs := make([]error, len(clients))
	var wg sync.WaitGroup
	for i, c := range clients {
		wg.Go(func() { results[i], errs[i] = c.Run(context.Background()) })
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		t.Fatal(err)
	}
	return results
}

// TestAnyoneContributes: in an open phase, programs started together each
// join by themselves, wait their turn and contribute once; the result each
// reports is the transcript's entry for it, and the transcript seals.
func TestAnyoneContributes(t *testing.T) {
	p := newPhase(t, coordinator.Config{Auth: coordinator.AuthOpen})
	names := []string{"alice", "bob", "carol"}
	var clients []*Client
	for _, n := range names {
		clients = append(clients, p.client(t, n))
	}
	results := runAll(t, clients...)
	states := p.srv.States()
	if len(states) != len(names) {
		t.Fatalf("%d contributions, want %d", len(states), len(names))
	}
	for i, r := range results {
		info, err := ceremony.Inspect(states[r.Index-1])
		if err != nil {
			t.Fatal(err)
		}
		if info.Hash != r.SHA256 || r.Already {
			t.Errorf("%s: reported %+v; state %d has sha256 %x", names[i], r, r.Index, info.Hash)
		}
	}
	if _, err := ceremony.SealPhase1(filepath.Join(t.TempDir(), "srs"), bytes.Repeat([]byte{1}, 32), states); err != nil {
		t.Fatal(err)
	}
}

// TestGitHubPhase: in a GitHub phase the program signs the participant in on
// GitHub's device flow, showing the code to enter, and contributes under the
// account's login.
func TestGitHubPhase(t *testing.T) {
	var polls atomic.Int32
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil || r.Form.Get("client_id") != "Iv1.test" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/login/device/code":
			fmt.Fprint(w, `{"device_code":"dc-1","user_code":"WDJB-MJHT","verification_uri":"https://github.com/login/device","expires_in":900,"interval":0}`)
		case "/login/oauth/access_token":
			if r.Form.Get("device_code") != "dc-1" {
				fmt.Fprint(w, `{"error":"incorrect_device_code"}`)
			} else if polls.Add(1) == 1 {
				fmt.Fprint(w, `{"error":"authorization_pending"}`)
			} else {
				fmt.Fprint(w, `{"access_token":"gho_octocat","token_type":"bearer"}`)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer github.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user" || r.Header.Get("Authorization") != "Bearer gho_octocat" {
			http.Error(w, `{"message":"Bad credentials"}`, http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, `{"login":"octocat","id":583231,"created_at":"2011-01-25T18:44:36Z"}`)
	}))
	defer api.Close()

	p := newPhase(t, coordinator.Config{Auth: coordinator.AuthGitHub, GitHubClientID: "Iv1.test", GitHubAPI: api.URL})
	var out bytes.Buffer
	c := p.client(t, "")
	c.GitHub, c.Out = github.URL, &out
	first := runAll(t, c)[0]
	if !strings.Contains(out.String(), "WDJB-MJHT") {
		t.Fatalf("the program did not show the code to enter:\n%s", out.String())
	}
	var tr struct {
		Contributions []struct{ Participant, Identity string }
	}
	req, _ := http.NewRequest(http.MethodGet, p.url+"/v1/transcript", nil)
	if err := do(req, &tr); err != nil {
		t.Fatal(err)
	}
	if e := tr.Contributions[0]; e.Participant != "octocat" || e.Identity != "github:583231" {
		t.Fatalf("entry %+v", e)
	}

	t.Run("running again reports the contribution the account made, given its sha256", func(t *testing.T) {
		polls.Store(0)
		again := p.client(t, "")
		again.GitHub, again.Mine = github.URL, fmt.Sprintf("%x", first.SHA256)
		r := runAll(t, again)[0]
		if !r.Already || r.Index != first.Index || r.SHA256 != first.SHA256 {
			t.Fatalf("%+v, want contribution %d %x, already made", r, first.Index, first.SHA256)
		}
	})

	// Audit R-002: a coordinator's "you contributed already" is not a receipt.
	t.Run("running again without its sha256 does not take the coordinator's word", func(t *testing.T) {
		polls.Store(0)
		again := p.client(t, "")
		again.GitHub = github.URL
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := again.Run(ctx); err == nil || !strings.Contains(err.Error(), "this program made no such contribution") {
			t.Fatalf("got %v, want a refusal of a contribution this run did not make", err)
		}
	})
}

// TestRejoinsAfterARestart: a program waiting its turn when the coordinator
// restarts, forgetting every registration, joins again by itself and
// contributes, however many times that happens: more than the three times
// in a row it gives up after, as being queued again starts the count over.
func TestRejoinsAfterARestart(t *testing.T) {
	p := newPhase(t, coordinator.Config{Auth: coordinator.AuthOpen})
	p.blocker(t) // holds the turn, so alice's program waits

	var out syncBuffer
	c := p.client(t, "alice")
	c.Out = &out
	done := make(chan error, 1)
	go func() {
		_, err := c.Run(context.Background())
		done <- err
	}()
	const restarts = 4
	for i := 1; i <= restarts; i++ {
		waitForTimes(t, &out, "Waiting for your turn", i, done)
		p.restartBlocked(t)
	}
	waitForTimes(t, &out, "Waiting for your turn", restarts+1, done)
	p.restart(t)
	if err := <-done; err != nil {
		t.Fatalf("Run: %v\n%s", err, out.String())
	}
	if n := strings.Count(out.String(), "joining again"); n != restarts+1 {
		t.Fatalf("the program joined again %d times, want %d:\n%s", n, restarts+1, out.String())
	}
}

// restartBlocked restarts the phase, as restart does, with a participant
// already holding the turn in the new coordinator, so that a program waiting
// before the restart waits again after it.
func (p *phase) restartBlocked(t *testing.T) {
	t.Helper()
	srv, err := coordinator.New(p.cfg)
	if err != nil {
		t.Fatal(err)
	}
	h := srv.Handler()
	call := func(path, token, body string, v any) {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if err := json.Unmarshal(rec.Body.Bytes(), v); rec.Code != http.StatusOK || err != nil {
			t.Fatalf("POST %s: %d %s", path, rec.Code, rec.Body)
		}
	}
	var s coordinator.Session
	call("/v1/register", "", `{"name":"blocker"}`, &s)
	var r coordinator.Reply
	if call("/v1/turn", s.Token, "", &r); r.Status != coordinator.Go {
		t.Fatalf("blocker: %+v", r)
	}
	p.srv = srv
	p.handler.Store(&h)
}

// TestWaitsForAPlaceFromItsAddress: when its network address has as many
// participants queued as the coordinator takes, a program waits for a place
// rather than giving up, and contributes once there is one.
func TestWaitsForAPlaceFromItsAddress(t *testing.T) {
	p := newPhase(t, coordinator.Config{Auth: coordinator.AuthOpen, MaxPerAddress: 1})
	blocker := p.blocker(t) // the one place this address has

	var out syncBuffer
	c := p.client(t, "alice")
	c.Out = &out
	done := make(chan error, 1)
	go func() {
		_, err := c.Run(context.Background())
		done <- err
	}()
	waitFor(t, &out, "Waiting for a place in the queue", done)
	blocker.contribute(t)
	if err := <-done; err != nil {
		t.Fatalf("Run: %v\n%s", err, out.String())
	}
	if n := len(p.srv.States()); n != 2 {
		t.Fatalf("%d contributions, want 2", n)
	}
}

// rawParticipant takes a turn by hand, request by request.
type rawParticipant struct {
	url, token string
}

// blocker registers a participant by hand and takes the turn with it.
func (p *phase) blocker(t *testing.T) rawParticipant {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, p.url+"/v1/register", strings.NewReader(`{"name":"blocker"}`))
	req.Header.Set("Content-Type", "application/json")
	var s coordinator.Session
	if err := do(req, &s); err != nil {
		t.Fatal(err)
	}
	req, _ = http.NewRequest(http.MethodPost, p.url+"/v1/turn", nil)
	req.Header.Set("Authorization", "Bearer "+s.Token)
	var r coordinator.Reply
	if err := do(req, &r); err != nil || r.Status != coordinator.Go {
		t.Fatalf("blocker: %+v %v", r, err)
	}
	return rawParticipant{url: p.url, token: s.Token}
}

// contribute downloads the state, contributes and uploads, by hand.
func (b rawParticipant) contribute(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	in, out := filepath.Join(dir, "in"), filepath.Join(dir, "out")
	req, _ := http.NewRequest(http.MethodGet, b.url+"/v1/state", nil)
	req.Header.Set("Authorization", "Bearer "+b.token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(in, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ceremony.Contribute(in, out); err != nil {
		t.Fatal(err)
	}
	next, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	req, _ = http.NewRequest(http.MethodPut, b.url+"/v1/contribution", bytes.NewReader(next))
	req.Header.Set("Authorization", "Bearer "+b.token)
	var r coordinator.Reply
	if err := do(req, &r); err != nil {
		t.Fatal(err)
	}
}

// TestAsksForAName: in an open phase, a program given no name asks its
// participant for one, and the transcript shows it.
func TestAsksForAName(t *testing.T) {
	p := newPhase(t, coordinator.Config{Auth: coordinator.AuthOpen})
	c := p.client(t, "")
	c.AskName = func() string { return "asked" }
	runAll(t, c)
	if got := p.participants(t); len(got) != 1 || got[0] != "asked" {
		t.Fatalf("transcript shows %v, want [asked]", got)
	}
}

// participants lists the transcript's participants, in order.
func (p *phase) participants(t *testing.T) []string {
	t.Helper()
	var tr struct {
		Contributions []struct{ Participant string }
	}
	req, _ := http.NewRequest(http.MethodGet, p.url+"/v1/transcript", nil)
	if err := do(req, &tr); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range tr.Contributions {
		names = append(names, e.Participant)
	}
	return names
}

// TestWaitsWhenRegistrationsAreFull: a program the coordinator turns away
// for too many registrations from its network address waits as told, then
// joins.
func TestWaitsWhenRegistrationsAreFull(t *testing.T) {
	p := newPhase(t, coordinator.Config{Auth: coordinator.AuthOpen})
	real := *p.handler.Load()
	var refused atomic.Bool
	full := http.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/register" && refused.CompareAndSwap(false, true) {
			w.Header().Set("Retry-After", "1")
			http.Error(w, `{"error":"at most 100 registrations an hour from one network address"}`, http.StatusTooManyRequests)
			return
		}
		real.ServeHTTP(w, r)
	}))
	p.handler.Store(&full)
	var out syncBuffer
	c := p.client(t, "alice")
	c.Out = &out
	runAll(t, c)
	if !strings.Contains(out.String(), "takes no more registrations") {
		t.Fatalf("the program did not say it was waiting:\n%s", out.String())
	}
}

// TestGivesUpOnACoordinatorThatForgetsItAtOnce: a program joins again when
// the coordinator forgets it, but not without end when the coordinator
// forgets it every time.
func TestGivesUpOnACoordinatorThatForgetsItAtOnce(t *testing.T) {
	p := newPhase(t, coordinator.Config{Auth: coordinator.AuthOpen})
	real := *p.handler.Load()
	forgetful := http.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/turn" {
			http.Error(w, `{"error":"no registration with this token"}`, http.StatusUnauthorized)
			return
		}
		real.ServeHTTP(w, r)
	}))
	p.handler.Store(&forgetful)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := p.client(t, "alice").Run(ctx); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("Run: %v, want %v", err, ErrUnauthorized)
	}
}

// TestRejoinsAfterARestartDuringItsTurn: a coordinator restarted during a
// participant's turn forgets the turn, and the contribution under way with
// it; the program joins again and contributes in a new turn.
func TestRejoinsAfterARestartDuringItsTurn(t *testing.T) {
	p := newPhase(t, coordinator.Config{Auth: coordinator.AuthOpen})
	c := p.client(t, "alice")
	var out syncBuffer
	c.Out = &out
	restarted := false
	c.Contribute = func(in io.Reader, size int64, w io.Writer, report func(ceremony.Progress)) (ceremony.Receipt, error) {
		if !restarted {
			restarted = true
			p.restart(t) // mid-contribution
		}
		return ceremony.ContributeWithProgress(in, size, w, report)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if _, err := c.Run(ctx); err != nil {
		t.Fatalf("Run: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "restarted during your turn; joining again") {
		t.Fatalf("the program did not join again:\n%s", out.String())
	}
	if got := p.participants(t); !slices.Equal(got, []string{"alice"}) {
		t.Fatalf("transcript: %q, want alice's contribution only", got)
	}
}

// TestGivesUpOnACoordinatorThatForgetsItsTurnAtOnce: a program whose every
// turn the coordinator forgets at once, refusing its download as from no
// one it knows, joins again a few times, then gives up, rather than for ever.
func TestGivesUpOnACoordinatorThatForgetsItsTurnAtOnce(t *testing.T) {
	answer := func(v any) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(v); err != nil {
				t.Error(err)
			}
		}
	}
	var joins atomic.Int32
	mux := http.NewServeMux()
	mux.Handle("GET /v1/config", answer(coordinator.ClientConfig{Auth: coordinator.AuthOpen}))
	mux.HandleFunc("POST /v1/register", func(w http.ResponseWriter, r *http.Request) {
		joins.Add(1)
		answer(coordinator.Session{Token: "forgotten at once"})(w, r)
	})
	mux.Handle("POST /v1/turn", answer(coordinator.Reply{
		Status: coordinator.Go, SHA256: strings.Repeat("0", 64), Deadline: time.Now().Add(time.Hour),
	}))
	mux.HandleFunc("GET /v1/state", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"no registration with this token"}`, http.StatusUnauthorized)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c := &Client{URL: srv.URL, Name: "alice", Heartbeat: -1}
	if _, err := c.Run(ctx); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("Run: %v, want %v", err, ErrUnauthorized)
	}
	if n := joins.Load(); n != 4 {
		t.Fatalf("joined %d times, want 4: once, then again 3 times", n)
	}
}

// TestRefusesAnAddressThatIsNotOne: a program given no coordinator, or
// something that is no web address, says so at once instead of retrying.
func TestRefusesAnAddressThatIsNotOne(t *testing.T) {
	for _, u := range []string{"", "ftp://ceremony.example.org", "https://"} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // retrying would outlast it
		c := &Client{URL: u, Patience: time.Hour}
		if _, err := c.Run(ctx); err == nil || !strings.Contains(err.Error(), "is not a coordinator's address") {
			t.Errorf("Run with URL %q: %v", u, err)
		}
		cancel()
	}
	t.Run("takes an address typed without https://", func(t *testing.T) {
		c := &Client{URL: "ceremony.example.org"}
		if err := c.checkURL(); err != nil || c.URL != "https://ceremony.example.org" {
			t.Fatalf("checkURL: %v, URL %q", err, c.URL)
		}
	})
}

// TestHeartbeatKeepsASlowTurn: while it contributes, a program tells the
// coordinator it is still at it, so a slow machine keeps its turn however
// soon the coordinator passes on the turn of a silent one.
func TestHeartbeatKeepsASlowTurn(t *testing.T) {
	p := newPhase(t, coordinator.Config{Auth: coordinator.AuthOpen, TurnPresence: 200 * time.Millisecond})
	c := p.client(t, "slow")
	c.Heartbeat = 50 * time.Millisecond
	c.Contribute = func(in io.Reader, size int64, out io.Writer, report func(ceremony.Progress)) (ceremony.Receipt, error) {
		time.Sleep(time.Second) // five times the coordinator's patience with a silent turn
		return ceremony.ContributeWithProgress(in, size, out, report)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var out syncBuffer
	c.Out = &out
	if _, err := c.Run(ctx); err != nil {
		t.Fatalf("Run: %v\n%s", err, out.String())
	}
	if strings.Contains(out.String(), "turn ended") {
		t.Fatalf("the slow turn was lost on the way:\n%s", out.String())
	}
}

// TestOutlastsItsTurn: a contribution that outlasts its turn, while another
// participant takes the next, is refused as late, and the program queues
// for a new turn and contributes on top of the other's. Its heartbeats,
// sent on through the late contribution, must not queue it meanwhile: a
// turn handed to it then would take the late upload, built on a state no
// longer the latest, and refuse it as no contribution.
func TestOutlastsItsTurn(t *testing.T) {
	p := newPhase(t, coordinator.Config{Auth: coordinator.AuthOpen})
	p.cfg.Turn = 300 * time.Millisecond
	p.restart(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	quickDone := make(chan struct{})
	slow := p.client(t, "slow")
	slow.Heartbeat = 20 * time.Millisecond
	var out syncBuffer
	slow.Out = &out
	var contributions atomic.Int32
	slow.Contribute = func(in io.Reader, size int64, w io.Writer, report func(ceremony.Progress)) (ceremony.Receipt, error) {
		if contributions.Add(1) == 1 {
			select { // outlasts the turn, while quick takes the next
			case <-quickDone:
			case <-ctx.Done():
			}
		}
		return ceremony.ContributeWithProgress(in, size, w, report)
	}
	slowDone := make(chan error, 1)
	go func() {
		_, err := slow.Run(ctx)
		slowDone <- err
	}()
	waitFor(t, &out, "Your turn", slowDone)
	if _, err := p.client(t, "quick").Run(ctx); err != nil {
		t.Fatalf("quick: %v", err)
	}
	close(quickDone)
	if err := <-slowDone; err != nil {
		t.Fatalf("slow: %v\n%s", err, out.String())
	}
	if got := p.participants(t); !slices.Equal(got, []string{"quick", "slow"}) {
		t.Fatalf("transcript: %q, want quick's contribution, then slow's", got)
	}
}

// TestReportsProgress: a run reports how far its download, contribution and
// upload have got, each only ever moving forward and each reaching the end,
// so a page can show progress bars for all three.
func TestReportsProgress(t *testing.T) {
	p := newPhase(t, coordinator.Config{Auth: coordinator.AuthOpen})
	c := p.client(t, "alice")
	var events []Event
	c.Events = func(e Event) { events = append(events, e) }
	runAll(t, c)
	for _, stage := range []string{StageDownloading, StageContributing, StageUploading} {
		var last Event
		seen := false
		for _, e := range events {
			if e.Stage != stage || (e.Total == 0 && e.Fraction == 0) {
				continue
			}
			if seen && (e.Done < last.Done && e.Part == last.Part || e.Fraction < last.Fraction) {
				t.Fatalf("%s goes back: %+v after %+v", stage, e, last)
			}
			last, seen = e, true
		}
		switch {
		case !seen:
			t.Errorf("%s reported no progress", stage)
		case stage == StageContributing && last.Fraction != 1:
			t.Errorf("the contribution's progress ends at %v, want 1", last.Fraction)
		case stage != StageContributing && (last.Total == 0 || last.Done != last.Total):
			t.Errorf("%s's progress ends at %d of %d bytes", stage, last.Done, last.Total)
		}
	}
}

// TestStopsOnAClosedPhase: a client stops at once, with the reason, and the
// coordinator's notice when it gives one, when the phase is closed.
func TestStopsOnAClosedPhase(t *testing.T) {
	t.Run("with the coordinator's notice", func(t *testing.T) {
		p := newPhase(t, coordinator.Config{Auth: coordinator.AuthOpen, Notice: "Phase 2 opens on Monday"})
		p.srv.Close()
		_, err := p.client(t, "alice").Run(context.Background())
		if !errors.Is(err, ErrClosed) || !strings.Contains(err.Error(), "Phase 2 opens on Monday") {
			t.Fatalf("Run: %v, want %v with the notice", err, ErrClosed)
		}
	})
	t.Run("without one", func(t *testing.T) {
		p := newPhase(t, coordinator.Config{Auth: coordinator.AuthOpen})
		p.srv.Close()
		if _, err := p.client(t, "alice").Run(context.Background()); !errors.Is(err, ErrClosed) {
			t.Fatalf("Run: %v, want %v", err, ErrClosed)
		}
	})
}

// TestRefusesARecordOfAnotherContribution: a program whose upload the
// coordinator answers with another contribution than the one it made says
// so, rather than handing its participant a receipt for something else.
func TestRefusesARecordOfAnotherContribution(t *testing.T) {
	p := newPhase(t, coordinator.Config{Auth: coordinator.AuthOpen})
	real := *p.handler.Load()
	misrecording := http.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/contribution" {
			real.ServeHTTP(w, r)
			return
		}
		rec := httptest.NewRecorder()
		real.ServeHTTP(rec, r)
		var reply coordinator.Reply
		if err := json.Unmarshal(rec.Body.Bytes(), &reply); err != nil {
			t.Error(err)
		}
		reply.SHA256 = strings.Repeat("0", 64)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(rec.Code)
		if err := json.NewEncoder(w).Encode(reply); err != nil {
			t.Error(err)
		}
	}))
	p.handler.Store(&misrecording)
	_, err := p.client(t, "alice").Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "the coordinator recorded contribution 1 sha256 "+strings.Repeat("0", 64)) {
		t.Fatalf("Run: %v, want the record refused", err)
	}
}

// TestRunRefusesAStateOtherThanAnnounced: the state a client contributes to
// is the one the coordinator announced, the latest of its public transcript.
func TestRunRefusesAStateOtherThanAnnounced(t *testing.T) {
	p := newPhase(t, coordinator.Config{Auth: coordinator.AuthOpen})
	real := *p.handler.Load()
	announced := strings.Repeat("ab", 32)
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/turn" {
			real.ServeHTTP(w, r)
			return
		}
		rec := httptest.NewRecorder()
		real.ServeHTTP(rec, r)
		var turn coordinator.Reply
		if err := json.Unmarshal(rec.Body.Bytes(), &turn); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		turn.SHA256 = announced
		if err := json.NewEncoder(w).Encode(turn); err != nil {
			t.Error(err)
		}
	}))
	defer fake.Close()
	c := p.client(t, "alice")
	c.URL = fake.URL
	_, err := c.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "having announced "+announced) {
		t.Fatalf("Run: %v, want the announced sha256 refused", err)
	}
}

// waitFor waits until out shows text, failing if the client's Run ends first.
func waitFor(t *testing.T, out *syncBuffer, text string, done <-chan error) {
	t.Helper()
	waitForTimes(t, out, text, 1, done)
}

// waitForTimes waits until out shows text n times, failing if the client's
// Run ends first.
func waitForTimes(t *testing.T, out *syncBuffer, text string, n int, done <-chan error) {
	t.Helper()
	deadline := time.After(30 * time.Second)
	for strings.Count(out.String(), text) < n {
		select {
		case err := <-done:
			t.Fatalf("Run ended (%v) before showing %q %d times:\n%s", err, text, n, out.String())
		case <-deadline:
			t.Fatalf("Run never showed %q %d times:\n%s", text, n, out.String())
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// do sends req and decodes its JSON reply into v.
func do(req *http.Request, v any) error {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

// syncBuffer is a bytes.Buffer safe to write from one goroutine and read from another.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
