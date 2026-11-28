package coordinator

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// holderOf asks for each of names' turns, in order, and returns the one
// told it holds the turn, failing when none is.
func holderOf(h *harness, names []string) string {
	h.t.Helper()
	for _, n := range names {
		if h.ask(n).Status == Go {
			return n
		}
	}
	h.t.Fatalf("none of %v holds the turn", names)
	return ""
}

// endTurn lets holder's turn run out by its program going silent, while the
// others ask, as their programs do.
func endTurn(h *harness, holder string, others []string) {
	h.t.Helper()
	var asking []string
	for _, n := range others {
		if n != holder {
			asking = append(asking, n)
		}
	}
	h.pass(3*time.Minute, asking...) // past TurnPresence (2 minutes)
}

// TestTurnsAreDrawnFairly (audit H3): a free turn goes to a queued
// participant whose network has been given the fewest turns, drawn at
// random among them. Queueing many entries first, from many addresses of a
// few networks, no longer takes every turn of an open phase: each network
// served moves behind every one not yet served.
func TestTurnsAreDrawnFairly(t *testing.T) {
	t.Run("many addresses of one network", func(t *testing.T) {
		h := newHarness(t, Config{Auth: AuthOpen, MaxPerNetwork: 100}) // the draw alone, not the cap on a network's entries
		var names []string
		for i := range 20 { // 20 /64s of one /48, one household's
			n := fmt.Sprintf("a%d", i)
			h.addrs[n] = fmt.Sprintf("2001:db8:0:%x::1", i)
			h.register(n, Registration{})
			h.ask(n)
			names = append(names, n)
		}
		h.addrs["honest"] = "198.51.100.7"
		h.register("honest", Registration{})
		h.ask("honest")
		names = append(names, "honest")
		first := holderOf(h, names)
		if first == "honest" {
			t.Fatal("the honest participant, queued last, was drawn first: the test draws the first of the least served")
		}
		endTurn(h, first, names)
		if next := holderOf(h, names); next != "honest" {
			t.Fatalf("after one turn to the network that queued 20 entries, the turn went to %s, not to the honest participant's network", next)
		}
	})
	t.Run("many networks, queued first", func(t *testing.T) {
		h := newHarness(t, Config{Auth: AuthOpen})
		var names []string
		for i := range 10 { // ten networks, two entries each, all queued before the honest participant
			for j := range 2 {
				n := fmt.Sprintf("n%d-%d", i, j)
				h.addrs[n] = fmt.Sprintf("2001:db8:%x:%x::1", i+1, j)
				h.register(n, Registration{})
				h.ask(n)
				names = append(names, n)
			}
		}
		h.addrs["honest"] = "198.51.100.7"
		h.register("honest", Registration{})
		h.ask("honest")
		names = append(names, "honest")
		for turn := 1; ; turn++ {
			holder := holderOf(h, names)
			if holder == "honest" {
				if turn > 11 {
					t.Fatalf("the honest participant's turn came %dth, after some network's second", turn)
				}
				return
			}
			if turn > 11 {
				t.Fatalf("turn %d went to %s, before the honest participant's network was served once", turn, holder)
			}
			endTurn(h, holder, names)
		}
	})
	t.Run("a network is one ticket", func(t *testing.T) {
		var drawnFrom []int
		h := newHarness(t, Config{Auth: AuthOpen, rand: func(n int) int { drawnFrom = append(drawnFrom, n); return 0 }})
		names := []string{"holder", "a1", "a2", "a3", "b"}
		for i, n := range names {
			h.addrs[n] = fmt.Sprintf("2001:db8:%x:%x::1", map[bool]int{true: 1, false: 2}[n[0] == 'a'], i)
			if n == "holder" {
				h.addrs[n] = "203.0.113.1"
			}
			h.register(n, Registration{})
		}
		wantStatus(t, "holder", h.ask("holder"), Go, 0)
		for _, n := range names[1:] {
			h.ask(n) // three entries from one network, one from another
		}
		endTurn(h, "holder", names[1:])
		if len(drawnFrom) == 0 {
			t.Fatal("the turn was not drawn")
		}
		if got := drawnFrom[len(drawnFrom)-1]; got != 2 {
			t.Fatalf("the turn was drawn among %d, want the 2 networks, not their 4 entries", got)
		}
	})
	t.Run("a draw, not an order", func(t *testing.T) {
		var drawnFrom []int
		h := newHarness(t, Config{Auth: AuthOpen, rand: func(n int) int { drawnFrom = append(drawnFrom, n); return n - 1 }})
		names := []string{"x", "y", "z"}
		for i, n := range names {
			h.addrs[n] = fmt.Sprintf("192.0.%d.1", 10+i) // three networks
			h.register(n, Registration{})
		}
		wantStatus(t, "x", h.ask("x"), Go, 0) // drawn from x alone; x's network is served
		h.ask("y")                            // y and z queue while x holds the turn
		h.ask("z")
		endTurn(h, "x", []string{"y", "z"})
		if holder := holderOf(h, []string{"z", "y"}); holder != "z" {
			t.Fatalf("the draw named the last of y and z (x's network served), and the turn went to %s", holder)
		}
		if got := drawnFrom[len(drawnFrom)-1]; got != 2 {
			t.Fatalf("the last turn was drawn from %d, want y and z", got)
		}
	})
}

// TestQueuePlace: a queued participant is told where they stand as turns
// are drawn: how many come first were there no draw, and how many, them
// among them, a turn is drawn from.
func TestQueuePlace(t *testing.T) {
	h := newHarness(t, Config{Auth: AuthOpen})
	for n, a := range map[string]string{"holder": "192.0.2.1", "x": "192.0.2.2", "y": "192.0.2.3", "w": "198.51.100.1"} {
		h.addrs[n] = a
		h.register(n, Registration{})
	}
	wantStatus(t, "holder", h.ask("holder"), Go, 0) // 192.0.2.0/24 served once
	h.ask("x")
	h.ask("y")
	h.ask("w")
	x, y, w := h.ask("x"), h.ask("y"), h.ask("w")
	// x and y share a network, served once; w's network, not yet served,
	// comes first, so x and y are in no draw yet, and are told a place.
	if x.Position != 2 || x.Draw != 0 || y.Position != 3 || y.Draw != 0 || w.Position != 1 || w.Draw != 1 {
		t.Fatalf("x %d of %d, y %d of %d, w %d of %d; want x 2 of 0, y 3 of 0, w 1 of 1", x.Position, x.Draw, y.Position, y.Draw, w.Position, w.Draw)
	}
	// Asking from a network not yet served, x is drawn as one from it.
	h.addrs["x"] = "203.0.113.9"
	if x := h.ask("x"); x.Position != 1 || x.Draw != 2 {
		t.Fatalf("x from a network not yet served: %d of %d, want 1 of 2 (with w)", x.Position, x.Draw)
	}
}

// TestTurnsHeldWithoutContributing (audit M11): a turn passes on when its
// holder does not download the state (a HEAD request is no download), or
// has it and does not upload within UploadWithin, however its program
// keeps sending heartbeats; and the turn counts as one without a
// contribution.
func TestTurnsHeldWithoutContributing(t *testing.T) {
	for _, c := range []struct {
		name  string
		then  time.Duration // long enough for the holder's turn to pass on, not the next holder's
		stall func(h *harness)
	}{
		{"a HEAD request", 2*time.Minute + 20*time.Second, func(h *harness) {
			if code, body := h.do("holder", http.MethodHead, "/v1/state", nil); code != http.StatusMethodNotAllowed {
				h.t.Fatalf("HEAD of the state: %d %s, want 405", code, body)
			}
		}},
		{"a download and no upload", 40 * time.Second, func(h *harness) {
			if code, body := h.do("holder", http.MethodGet, "/v1/state", nil); code != http.StatusOK {
				h.t.Fatalf("downloading: %d %s", code, body)
			}
			h.pass(40*time.Second, "holder", "next")
			if r := h.ask("holder"); r.Status != Go {
				h.t.Fatalf("within UploadWithin: %+v, want the turn still", r)
			}
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t, Config{Auth: AuthOpen, UploadWithin: time.Minute})
			for n, a := range map[string]string{"holder": "192.0.2.1", "next": "198.51.100.1"} {
				h.addrs[n] = a
				h.register(n, Registration{})
			}
			wantStatus(t, "holder", h.ask("holder"), Go, 0)
			h.ask("next")
			c.stall(h)
			h.pass(c.then, "holder", "next") // heartbeats all along
			if r := h.ask("next"); r.Status != Go {
				t.Fatalf("next: %+v, want the turn, the holder's having passed on", r)
			}
			if got := h.status().Wasted; got != 1 {
				t.Fatalf("turns without a contribution: %d, want 1", got)
			}
			if code, body := h.do("holder", http.MethodPost, "/v1/turn", nil); code != http.StatusOK { // one turn charged, of MaxTurns
				t.Fatalf("the holder asking again: %d %s, want queued", code, body)
			}
		})
	}
}

// TestBothAddressesCount (audit L1): a contribution counts against both the
// address its turn was given to and the one it was uploaded from, and is
// refused when either has made its contributions.
func TestBothAddressesCount(t *testing.T) {
	const a, b = "192.0.2.1", "198.51.100.1"
	t.Run("charged to both", func(t *testing.T) {
		h := newHarness(t, Config{Auth: AuthOpen, MaxContributionsPerAddress: 1})
		for n, addr := range map[string]string{"alice": a, "bob": a, "carol": b} {
			h.addrs[n] = addr
			h.register(n, Registration{})
		}
		wantStatus(t, "alice", h.ask("alice"), Go, 0)
		h.addrs["alice"] = b // uploads over another address
		h.mustContribute("alice")
		for _, n := range []string{"bob", "carol"} {
			if code, body := h.do(n, http.MethodPost, "/v1/turn", nil); code != http.StatusForbidden {
				t.Errorf("%s, from an address alice's contribution counted against: %d %s, want 403", n, code, body)
			}
		}
	})
	t.Run("refused from an address that has made its contributions", func(t *testing.T) {
		h := newHarness(t, Config{Auth: AuthOpen, MaxContributionsPerAddress: 1})
		for n, addr := range map[string]string{"carol": b, "alice": a} {
			h.addrs[n] = addr
			h.register(n, Registration{})
		}
		wantStatus(t, "carol", h.ask("carol"), Go, 0)
		h.mustContribute("carol")
		wantStatus(t, "alice", h.ask("alice"), Go, 0)
		h.addrs["next"] = "203.0.113.1"
		h.register("next", Registration{})
		h.ask("next")
		h.addrs["alice"] = b
		if code, body, _ := h.contribute("alice"); code != http.StatusForbidden {
			t.Fatalf("alice's upload from carol's address: %d %s, want 403", code, body)
		}
		wantStatus(t, "next", h.ask("next"), Go, 0) // the refusal ended alice's turn
	})
}

// TestUntoldTurnsCount (audit H3, L3): a turn counts against its holder and
// their network as it is drawn, used or not: one whose program never comes
// back held the queue up for TurnPresence, and costs them a turn like any
// other, so nobody stalls turns for free. A TurnPresence shorter than twice
// Poll, which could let a turn pass on before a program still asking hears
// of it, is refused.
func TestUntoldTurnsCount(t *testing.T) {
	if _, err := New(Config{Dir: t.TempDir(), TurnPresence: time.Second, Poll: time.Second, Presence: 2 * time.Second}); err == nil || !strings.Contains(err.Error(), "coordinator: TurnPresence") {
		t.Fatalf("New with TurnPresence under twice Poll: %v, want it refused", err)
	}
	var logged bytes.Buffer
	h := newHarness(t, Config{Auth: AuthOpen, MaxTurns: 1, Log: log.New(&logged, "", 0)})
	for n, a := range map[string]string{"bob": "203.0.113.1", "alice": "192.0.2.1", "carol": "198.51.100.1"} {
		h.addrs[n] = a
		h.register(n, Registration{})
	}
	wantStatus(t, "bob", h.ask("bob"), Go, 0)
	h.tick(2*time.Minute - 10*time.Second) // bob's program goes silent
	h.ask("alice")                         // alice and carol queue, asking just before bob's turn runs out
	h.ask("carol")
	h.tick(15 * time.Second)
	h.ask("carol") // bob's turn passes on, and is drawn for alice, whose program does not come back
	h.pass(2*time.Minute+20*time.Second, "carol")
	if code, body := h.do("alice", http.MethodPost, "/v1/turn", nil); code != http.StatusForbidden {
		t.Fatalf("alice, drawn and never back: %d %s, want her charged the turn (MaxTurns 1)", code, body)
	}
	if got := h.status().Wasted; got != 2 {
		t.Fatalf("turns without a contribution: %d, want bob's and alice's", got)
	}
	if !strings.Contains(logged.String(), "from 192.0.2.1") {
		t.Fatalf("the log names no address:\n%s", logged.String())
	}
}

// TestNetworkQueueCap (audit H3): one network queues at most MaxPerNetwork
// at once, so it cannot fill the queue and leave everyone else undrawn.
func TestNetworkQueueCap(t *testing.T) {
	h := newHarness(t, Config{Auth: AuthOpen, MaxPerNetwork: 2})
	for i, n := range []string{"holder", "a", "b", "c"} {
		h.addrs[n] = fmt.Sprintf("2001:db8:0:%x::1", i) // four /64s of one /48
		h.register(n, Registration{})
	}
	wantStatus(t, "holder", h.ask("holder"), Go, 0)
	h.ask("a")
	h.ask("b")
	if code, body := h.do("c", http.MethodPost, "/v1/turn", nil); code != http.StatusTooManyRequests || !strings.Contains(string(body), "from your network") {
		t.Fatalf("a third from one network: %d %s, want 429", code, body)
	}
	h.addrs["d"] = "198.51.100.1"
	h.register("d", Registration{})
	h.ask("d")                       // another network has room
	h.addrs["d"] = "2001:db8:0:9::1" // and d, queued, cannot move to the full one
	if code, body := h.do("d", http.MethodPost, "/v1/turn", nil); code != http.StatusTooManyRequests {
		t.Fatalf("a queued entry moving to a full network: %d %s, want 429", code, body)
	}

	h = newHarness(t, Config{Auth: AuthOpen, MaxPerNetwork: 1, NoAddressLimit: true})
	for i, n := range []string{"holder", "a", "b"} {
		h.addrs[n] = fmt.Sprintf("2001:db8:0:%x::1", i)
		h.register(n, Registration{})
		h.ask(n) // NoAddressLimit lifts it, as it lifts MaxPerAddress
	}
}

// TestGoTellsTheUploadDeadline (audit M11): the turn's holder is told when
// they must have started uploading: UploadWithin from the download that
// follows, not the turn's own deadline, when that comes first.
func TestGoTellsTheUploadDeadline(t *testing.T) {
	h := newHarness(t, Config{Auth: AuthOpen, UploadWithin: time.Minute})
	h.addrs["holder"] = "192.0.2.1"
	h.register("holder", Registration{})
	r := h.ask("holder")
	if want := time.Unix(0, h.clock.Load()).Add(time.Minute); r.Status != Go || !r.Deadline.Equal(want) {
		t.Fatalf("Go with deadline %s, want %s", r.Deadline, want)
	}
}

// TestFairPrefixes: the network turns are shared by is no wider than an
// address can be, and no narrower than one counts for the limits.
func TestFairPrefixes(t *testing.T) {
	for _, c := range []Config{{FairPrefix4: 33}, {IPv6Prefix: 48, FairPrefix6: 56}} {
		c.Dir = t.TempDir()
		if _, err := New(c); err == nil || !strings.Contains(err.Error(), "coordinator: FairPrefix") { // not the temporary directory's name
			t.Errorf("New(%+v): %v, want it refused", c, err)
		}
	}
}

// TestGitHubAccountTurnsCount (audit H3): in a GitHub phase, an account's
// turns count wherever it asks from: a fresh network does not put an
// account already served ahead of one not yet served.
func TestGitHubAccountTurnsCount(t *testing.T) {
	created := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	api := fakeGitHub(t, map[string]githubAccount{
		"served-token": {Login: "served", ID: 1, Created: created},
		"fresh-token":  {Login: "fresh", ID: 2, Created: created},
		"other-token":  {Login: "other", ID: 3, Created: created},
	})
	h := newHarness(t, Config{Auth: AuthGitHub, GitHubClientID: testClientID, GitHubClientSecret: testClientSecret, GitHubAPI: api})
	h.addrs["served"], h.addrs["fresh"], h.addrs["other"] = "192.0.2.1", "198.51.100.1", "100.64.0.1"
	for _, n := range []string{"served", "fresh", "other"} {
		h.register(n, Registration{GitHubToken: n + "-token"})
	}
	wantStatus(t, "served", h.ask("served"), Go, 0)
	endTurn(h, "served", nil)
	wantStatus(t, "other", h.ask("other"), Go, 0)
	h.addrs["served"] = "203.0.113.1" // a network not yet served
	h.ask("served")                   // queued first, while other holds the turn
	h.ask("fresh")
	endTurn(h, "other", []string{"served", "fresh"})
	if holder := holderOf(h, []string{"served", "fresh"}); holder != "fresh" {
		t.Fatalf("the turn went to %s, whose account was served already, from a new network", holder)
	}
}

// TestPlaceSkipsWhatIsNotDrawn: a queued participant's place does not count
// those the draw passes over, such as one from an address that has made
// its contributions, and counts a network once in the draw.
func TestPlaceSkipsWhatIsNotDrawn(t *testing.T) {
	h := newHarness(t, Config{Auth: AuthOpen})
	s := h.srv
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.cfg.now()
	done := participant{identity: "done", addr: "192.0.2.1", network: "192.0.2.0/24"}
	me := participant{identity: "me", addr: "198.51.100.1", network: "198.51.100.0/24"}
	s.queue = []waiter{{p: done, seen: now}, {p: me, seen: now}}
	s.contributedFrom[done.addr] = s.cfg.MaxContributionsPerAddress
	if position, draw := s.place(me, now); position != 1 || draw != 1 {
		t.Fatalf("place %d of %d, want 1 of 1: one from an address that has made its contributions is never drawn", position, draw)
	}
	// Three from one network are one network in the draw.
	s.queue = nil
	for i := range 3 {
		s.queue = append(s.queue, waiter{p: participant{identity: identity(fmt.Sprint("a", i)), addr: fmt.Sprint("203.0.113.", i+1), network: "203.0.113.0/24"}, seen: now})
	}
	s.queue = append(s.queue, waiter{p: me, seen: now})
	if position, draw := s.place(me, now); position != 4 || draw != 2 {
		t.Fatalf("place %d of %d, want 4 of 2: a network is one in the draw, however many it has queued", position, draw)
	}
}

// TestTurnsAreChargedOnAnyRoute (audit H3, L3): a turn counts against its
// holder however their program learns of it: one that learns of its turn
// from a heartbeat's answer, and never asks for it, is charged as one that
// asks.
func TestTurnsAreChargedOnAnyRoute(t *testing.T) {
	h := newHarness(t, Config{Auth: AuthOpen})
	for n, a := range map[string]string{"holder": "192.0.2.1", "next": "198.51.100.1"} {
		h.addrs[n] = a
		h.register(n, Registration{})
	}
	wantStatus(t, "next", h.ask("next"), Go, 0) // and then goes silent
	h.tick(time.Minute + 45*time.Second)
	h.ask("holder") // queued, and present when next's turn runs out
	h.tick(20 * time.Second)
	h.status() // next's turn runs out, and is drawn for holder, whose program never asks for it
	if code, body := h.do("holder", http.MethodPost, "/v1/heartbeat", nil); code != http.StatusNoContent {
		t.Fatalf("holder's heartbeat: %d %s, want 204, the turn being theirs", code, body)
	}
	h.tick(3 * time.Minute) // and then goes silent
	h.status()
	if got := h.status().Wasted; got != 2 {
		t.Fatalf("turns without a contribution: %d, want 2 (next's, and holder's, who heard of it by heartbeat)", got)
	}
}

// TestUploadWithinFromTheFirstDownload (audit M11): a turn's holder has
// UploadWithin from first starting to download the state; downloading it
// again does not restart it.
func TestUploadWithinFromTheFirstDownload(t *testing.T) {
	h := newHarness(t, Config{Auth: AuthOpen, UploadWithin: time.Minute})
	for n, a := range map[string]string{"holder": "192.0.2.1", "next": "198.51.100.1"} {
		h.addrs[n] = a
		h.register(n, Registration{})
	}
	wantStatus(t, "holder", h.ask("holder"), Go, 0)
	h.ask("next")
	for range 2 {
		if code, body := h.do("holder", http.MethodGet, "/v1/state", nil); code != http.StatusOK {
			t.Fatalf("downloading: %d %s", code, body)
		}
		h.pass(40*time.Second, "holder", "next")
	}
	if r := h.ask("next"); r.Status != Go {
		t.Fatalf("next: %+v, want the turn: 80 s after the first download, past UploadWithin", r)
	}
}

// TestRefusedUploadIsATurnWasted: a turn whose upload is refused ended
// without a contribution, and is counted so.
func TestRefusedUploadIsATurnWasted(t *testing.T) {
	h := newHarness(t, Config{Auth: AuthOpen})
	h.addrs["holder"] = "192.0.2.1"
	h.register("holder", Registration{})
	wantStatus(t, "holder", h.ask("holder"), Go, 0)
	if code, body := h.do("holder", http.MethodPut, "/v1/contribution", []byte("not a contribution")); code == http.StatusOK {
		t.Fatalf("garbage accepted: %s", body)
	}
	if got := h.status().Wasted; got != 1 {
		t.Fatalf("turns without a contribution: %d, want 1", got)
	}
}

// TestTurnDefaults: UploadWithin is at most Turn, and the network turns
// are shared by is no narrower than an address by default.
func TestTurnDefaults(t *testing.T) {
	if _, err := New(Config{Dir: t.TempDir(), Turn: time.Minute, UploadWithin: 2 * time.Minute}); err == nil || !strings.Contains(err.Error(), "coordinator: UploadWithin") {
		t.Fatalf("UploadWithin longer than Turn: %v, want it refused", err)
	}
	for _, c := range []Config{{Turn: time.Minute}, {IPv6Prefix: 40}} {
		c.Dir = t.TempDir()
		if _, err := New(c); err == nil || strings.Contains(err.Error(), "coordinator:") { // only the missing state-0
			t.Errorf("New(%+v): %v, want its defaults taken", c, err)
		}
	}
}

// TestFullQueueMakesRoom (audit H3): a full queue takes one from a network
// with at least two fewer queued than the network with the most, in place
// of that network's latest, so whoever fills the queue first does not keep
// everyone else out of the draw.
func TestFullQueueMakesRoom(t *testing.T) {
	var logged bytes.Buffer
	h := newHarness(t, Config{Auth: AuthOpen, MaxQueue: 3, Log: log.New(&logged, "", 0)})
	addrs := map[string]string{"holder": "203.0.113.1", "a1": "192.0.2.1", "a2": "192.0.2.2", "a3": "192.0.2.3",
		"b": "198.51.100.1", "c": "198.18.0.1", "d": "198.19.0.1"}
	for _, n := range []string{"holder", "a1", "a2", "a3", "b", "c", "d"} {
		h.addrs[n] = addrs[n]
		h.register(n, Registration{Name: n})
	}
	wantStatus(t, "holder", h.ask("holder"), Go, 0)
	for _, n := range []string{"a1", "a2", "a3"} {
		h.ask(n) // one network fills the queue
	}
	h.ask("b") // a3 leaves it for b
	if !strings.Contains(logged.String(), "the queue is full: a3 (from") {
		t.Fatalf("the log does not say a3 left the queue:\n%s", logged.String())
	}
	if code, body := h.do("a3", http.MethodPost, "/v1/turn", nil); code != http.StatusTooManyRequests {
		t.Fatalf("a3 again, its network still the most queued: %d %s, want 429", code, body)
	}
	h.ask("c") // a2 leaves it for c
	if code, body := h.do("d", http.MethodPost, "/v1/turn", nil); code != http.StatusTooManyRequests {
		t.Fatalf("d, with a queue of one from each network: %d %s, want 429", code, body)
	}
	for _, n := range []string{"a1", "b", "c"} {
		if r := h.ask(n); r.Status != Queued {
			t.Fatalf("%s: %+v, want queued", n, r)
		}
	}

	// Of the network with the most queued, one no longer asking leaves
	// first, though queued earlier than one still asking.
	logged.Reset()
	h = newHarness(t, Config{Auth: AuthOpen, MaxQueue: 3, Log: log.New(&logged, "", 0)})
	for _, n := range []string{"holder", "a1", "a2", "a3", "b"} {
		h.addrs[n] = addrs[n]
		h.register(n, Registration{Name: n})
	}
	wantStatus(t, "holder", h.ask("holder"), Go, 0)
	h.ask("a1")
	h.ask("a2")
	h.tick(presence + time.Second)
	h.ask("a1")
	h.ask("a3") // a2 has not asked since presence
	h.ask("b")
	if !strings.Contains(logged.String(), "the queue is full: a2 (from") {
		t.Fatalf("the log does not say a2, no longer asking, left the queue:\n%s", logged.String())
	}
}

// stalledUpload starts name's upload, sends a few bytes of it, and stalls:
// it returns how long the coordinator held on, up to 10 s, when the upload
// is given up.
func stalledUpload(h *harness, name string) time.Duration {
	h.t.Helper()
	body, stall := io.Pipe()
	go func() { _, _ = stall.Write([]byte("the first bytes, and then nothing")) }()
	giveUp := time.AfterFunc(10*time.Second, func() { stall.CloseWithError(errors.New("given up")) })
	defer giveUp.Stop()
	req, err := http.NewRequest(http.MethodPut, h.http.URL+"/v1/contribution", body)
	if err != nil {
		h.t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+h.tokens[name])
	req.Header.Set("X-Forwarded-For", h.addrs[name])
	start := time.Now()
	if resp, err := http.DefaultClient.Do(req); err == nil {
		resp.Body.Close()
	}
	return time.Since(start)
}

// TestUploadEndsAtTheTurnsDeadline (audit M11): an upload, once started,
// ends at the turn's deadline at the latest, so a stalled or trickled one
// holds the turn no longer than -turn.
func TestUploadEndsAtTheTurnsDeadline(t *testing.T) {
	h := newHarness(t, Config{Auth: AuthOpen})
	h.clock.Store(time.Now().Add(-turnTime + time.Second).UnixNano()) // the turn drawn now ends a second from now, in real time
	h.addrs["holder"] = "192.0.2.1"
	h.register("holder", Registration{})
	wantStatus(t, "holder", h.ask("holder"), Go, 0)
	if held := stalledUpload(h, "holder"); held > 5*time.Second {
		t.Fatalf("the upload went on for %s, past the turn's deadline", held)
	}
}

// TestUploadCutShort: an upload that ends before all it said it would send
// failed, and is not checked as a contribution.
func TestUploadCutShort(t *testing.T) {
	var logged bytes.Buffer
	h := newHarness(t, Config{Auth: AuthOpen, Log: log.New(&logged, "", 0)})
	h.addrs["holder"] = "192.0.2.1"
	h.register("holder", Registration{Name: "holder"})
	wantStatus(t, "holder", h.ask("holder"), Go, 0)
	conn, err := net.Dial("tcp", strings.TrimPrefix(h.http.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	fmt.Fprintf(conn, "PUT /v1/contribution HTTP/1.1\r\nHost: coordinator\r\nAuthorization: Bearer %s\r\nX-Forwarded-For: %s\r\nContent-Length: 1000\r\n\r\nten bytes.",
		h.tokens["holder"], h.addrs["holder"])
	if err := conn.(*net.TCPConn).CloseWrite(); err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, conn) // the answer, then the coordinator closes
	if !strings.Contains(logged.String(), "holder's upload failed") {
		t.Fatalf("the log does not say the upload failed:\n%s", logged.String())
	}
}

// TestStateSendStopsAtTheUploadDeadline (audit M11): the turn holder's
// download of the state stops when they must have started uploading: a
// download kept stalled does not hold the turn past it.
func TestStateSendStopsAtTheUploadDeadline(t *testing.T) {
	h := newHarness(t, Config{Auth: AuthOpen, UploadWithin: 500 * time.Millisecond})
	h.clock.Store(time.Now().UnixNano()) // the send's deadline is the network's, in real time
	big := make([]byte, 64<<20)          // more than the sockets between hold
	if err := os.WriteFile(filepath.Join(h.dir, "state-0"), big, 0o600); err != nil {
		t.Fatal(err)
	}
	h.addrs["holder"] = "192.0.2.1"
	h.register("holder", Registration{})
	wantStatus(t, "holder", h.ask("holder"), Go, 0)
	req, err := http.NewRequest(http.MethodGet, h.http.URL+"/v1/state", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+h.tokens["holder"])
	req.Header.Set("X-Forwarded-For", h.addrs["holder"])
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	time.Sleep(1500 * time.Millisecond) // reading none of it, past the upload deadline
	if n, _ := io.Copy(io.Discard, resp.Body); n == int64(len(big)) {
		t.Fatal("the whole state came, read after its holder's upload deadline")
	}
}
