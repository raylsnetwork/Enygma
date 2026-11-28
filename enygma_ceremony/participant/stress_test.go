package participant

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"enygma_ceremony/ceremony"
	"enygma_ceremony/coordinator"
	"enygma_ceremony/transcript"
)

// queueLength is how many participants the phase has queued.
func queueLength(t *testing.T, p *phase) int {
	t.Helper()
	resp, err := http.Get(p.url + "/v1/status")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var st coordinator.Status
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		t.Fatal(err)
	}
	return st.Queue
}

// misbehaver is a participant who takes a turn and then does one thing wrong.
type misbehaver struct {
	p     *phase
	name  string
	token string
}

// call sends a request with the misbehaver's token and returns the status and body.
func (m *misbehaver) call(t *testing.T, method, path string, body []byte) (int, []byte) {
	req, err := http.NewRequest(method, m.p.url+path, bytes.NewReader(body))
	if err != nil {
		t.Error(err)
		return 0, nil
	}
	if m.token != "" {
		req.Header.Set("Authorization", "Bearer "+m.token)
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Error(err)
		return 0, nil
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
	}
	return resp.StatusCode, data
}

// takeTurn registers and asks until the turn is the misbehaver's. It reports
// false if the test is over first.
func (m *misbehaver) takeTurn(t *testing.T, ctx context.Context) bool {
	code, data := m.call(t, http.MethodPost, "/v1/register", fmt.Appendf(nil, `{"name":%q}`, m.name))
	var s coordinator.Session
	if code != http.StatusOK || json.Unmarshal(data, &s) != nil {
		t.Errorf("%s registering: %d %s", m.name, code, data)
		return false
	}
	m.token = s.Token
	for ctx.Err() == nil {
		code, data := m.call(t, http.MethodPost, "/v1/turn", nil)
		var r coordinator.Reply
		if code != http.StatusOK || json.Unmarshal(data, &r) != nil {
			t.Errorf("%s asking: %d %s", m.name, code, data)
			return false
		}
		if r.Status == coordinator.Go {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// TestManyAtOnce: a busy phase holds. Forty programs join at once, among
// participants who misbehave once their turn comes: some go silent, some
// upload garbage, some upload back the state they were given, some upload a
// real contribution after their turn has passed, and some programs give up
// while they wait. Every program that stays finishes, contributing once; the
// transcript holds their contributions and nothing else, and checks from end
// to end; no misbehaviour is accepted, and each holds the line up only until
// its silence ends its turn.
func TestManyAtOnce(t *testing.T) {
	if testing.Short() {
		t.Skip("a busy phase takes some seconds")
	}
	p := newPhase(t, coordinator.Config{
		Auth: coordinator.AuthOpen, TurnPresence: 300 * time.Millisecond,
		MaxPerAddress: 1000, MaxPerNetwork: 1000, MaxContributionsPerAddress: 1000, // every program runs on this machine
	})
	p.cfg.Presence = time.Second // a program that gives up stops holding the line soon after
	p.restart(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	const programs = 40
	var wg sync.WaitGroup
	results := make([]Result, programs)
	errs := make([]error, programs)
	for i := range programs {
		c := p.client(t, fmt.Sprintf("program %d", i))
		c.Out = io.Discard
		wg.Go(func() { results[i], errs[i] = c.Run(ctx) })
	}
	// Quitters join once others are waiting, and give up as soon as they are
	// told they wait too: never after their turn has come. (On a timer, one
	// that joined first could have its turn, and finish, before giving up.)
	for deadline := time.Now().Add(30 * time.Second); queueLength(t, p) < 5; {
		if time.Now().After(deadline) {
			t.Fatal("the programs never queued")
		}
		time.Sleep(10 * time.Millisecond)
	}
	for i := range 3 {
		c := p.client(t, fmt.Sprintf("quitter %d", i))
		c.Out = io.Discard
		quit, stop := context.WithCancel(ctx)
		c.Events = func(e Event) {
			if e.Stage == StageQueued {
				stop()
			}
		}
		wg.Go(func() {
			defer stop()
			if r, err := c.Run(quit); err == nil {
				t.Errorf("%s gave up, yet contributed: %+v", c.Name, r)
			}
		})
	}
	misdeeds := map[string]func(t *testing.T, m *misbehaver) (int, []byte){
		"silent": func(*testing.T, *misbehaver) (int, []byte) { return 0, nil },
		"garbage": func(t *testing.T, m *misbehaver) (int, []byte) {
			junk := make([]byte, 4096)
			if _, err := rand.Read(junk); err != nil {
				t.Error(err)
			}
			return m.call(t, http.MethodPut, "/v1/contribution", junk)
		},
		"replay": func(t *testing.T, m *misbehaver) (int, []byte) {
			_, state := m.call(t, http.MethodGet, "/v1/state", nil)
			return m.call(t, http.MethodPut, "/v1/contribution", state)
		},
		"late": func(t *testing.T, m *misbehaver) (int, []byte) {
			_, state := m.call(t, http.MethodGet, "/v1/state", nil)
			var next bytes.Buffer
			if _, err := ceremony.ContributeTo(bytes.NewReader(state), int64(len(state)), &next); err != nil {
				t.Error(err)
			}
			time.Sleep(time.Second) // silent past TurnPresence: the turn has passed on
			return m.call(t, http.MethodPut, "/v1/contribution", next.Bytes())
		},
	}
	var acted atomic.Int32 // misbehavers whose turn came
	for kind, misdeed := range misdeeds {
		for i := range 3 {
			m := &misbehaver{p: p, name: fmt.Sprintf("%s %d", kind, i)}
			wg.Go(func() {
				if !m.takeTurn(t, ctx) {
					return
				}
				acted.Add(1)
				if code, body := misdeed(t, m); code == http.StatusOK {
					t.Errorf("%s's upload was accepted: %s", m.name, body)
				}
			})
		}
	}
	wg.Wait()

	if n := int(acted.Load()); n != 3*len(misdeeds) {
		t.Errorf("%d misbehavers had their turn, want all %d", n, 3*len(misdeeds))
	}
	seen := map[uint32]string{}
	for i, r := range results {
		if errs[i] != nil {
			t.Errorf("program %d: %v", i, errs[i])
			continue
		}
		if other, dup := seen[r.Index]; dup {
			t.Errorf("programs %d and %s both report contribution %d", i, other, r.Index)
		}
		seen[r.Index] = fmt.Sprint(i)
	}
	tr, err := transcript.Check(p.dir, io.Discard)
	if err != nil {
		t.Fatalf("the transcript does not check: %v", err)
	}
	if len(tr.Contributions) != programs {
		t.Fatalf("%d contributions, want one from each of the %d programs", len(tr.Contributions), programs)
	}
	for _, e := range tr.Contributions {
		if _, ok := seen[e.Index]; !ok {
			t.Errorf("contribution %d is by %s, which no program reports", e.Index, e.Participant)
		}
	}
}
