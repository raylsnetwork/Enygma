// Package coordinator runs one phase of a ceremony as a service. Participants
// queue; each in turn downloads the latest state, contributes on their own
// machine (package participant does it for them), and uploads the next state,
// which the coordinator checks with ceremony.Check before anyone builds on it.
//
// Anyone may queue: a participant's program registers itself (AuthOpen, the
// default), or signs in with GitHub (AuthGitHub). Each admitted identity
// contributes once. A queued participant who goes quiet
// keeps their place for a while (Config.Away) but is passed over if they are
// not asking when their turn comes (Config.Presence); a turn holder who runs
// out of time (Config.Turn) loses the turn. So no one stalls the queue for
// longer than a turn.
//
// The phase's directory is its transcript (package transcript), and a
// restarted coordinator carries on from it. The coordinator also serves a
// page anyone can follow the phase on. It serves no participant code:
// participants run the program from the ceremony's GitHub release
// (Config.ReleaseURL), or built from source, so the coordinator never
// chooses the code that draws their randomness.
package coordinator

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"enygma_ceremony/ceremony"
	"enygma_ceremony/transcript"
)

// Auth is how a coordinator admits participants.
type Auth string

const (
	// AuthOpen admits anyone: the participant's program registers itself,
	// under a name of the participant's choosing.
	AuthOpen Auth = "open"
	// AuthGitHub admits anyone signed in with a GitHub account older than
	// Config.GitHubMinAge, once per account, under its login.
	AuthGitHub Auth = "github"
)

// Config is how a coordinator runs a phase. Zero durations take the defaults
// noted.
type Config struct {
	// Dir holds the phase: state-0, and every contribution accepted so far.
	Dir string
	// Auth is how participants are admitted; AuthOpen when empty.
	Auth Auth
	// GitHubClientID is the GitHub OAuth app participants sign in to, for
	// AuthGitHub. It needs the device flow enabled.
	GitHubClientID string
	// GitHubMinAge is how old a GitHub account must be, for AuthGitHub (30
	// days when zero).
	GitHubMinAge time.Duration
	// GitHubAPI is GitHub's API, for AuthGitHub (https://api.github.com).
	GitHubAPI string
	// GitHub is GitHub's site, whose device flow the coordinator relays,
	// for AuthGitHub (https://github.com).
	GitHub string
	// MaxPerAddress bounds how many participants from one network address
	// may be queued or holding the turn at once (3 when zero); NoAddressLimit
	// lifts the bound.
	MaxPerAddress  int
	NoAddressLimit bool
	// TrustedProxies are the reverse proxies whose X-Forwarded-For says where
	// a request comes from: a request from one of them comes from the last
	// address it added. Any other request comes from its connection's
	// address, whatever it says.
	TrustedProxies []netip.Prefix
	// IPv6Prefix is how many leading bits of an IPv6 address count as one
	// network address for the limits (64 when zero): one host is given a
	// whole /64, and could otherwise use a new address for every request.
	IPv6Prefix int
	// MaxQueue bounds how many participants are queued at once, from
	// everywhere (1000 when zero).
	MaxQueue int
	// MaxTurns bounds the turns one identity is given without contributing
	// (3 when zero): one that holds turns without ever contributing would
	// otherwise hold up the queue again and again.
	MaxTurns int
	// Turn is how long a participant has, from being handed the turn, to
	// finish uploading their contribution (60 minutes): room for a slow
	// laptop, since a turn whose holder has gone passes on at TurnPresence.
	Turn time.Duration
	// TurnPresence is how long the turn holder's program may go without
	// a heartbeat before the turn passes on (2 minutes).
	TurnPresence time.Duration
	// Presence is how recently a queued participant must have asked to be
	// given the turn when it reaches them (30 seconds).
	Presence time.Duration
	// Away is how long a queued participant may go without asking before
	// losing their place, and how long an unused registration lasts (10 minutes).
	Away time.Duration
	// Poll is how often queued participants are told to ask (5 seconds).
	Poll time.Duration
	// CloseAt, when set, is when the phase stops handing out turns.
	CloseAt time.Time
	// Notice is a message for participants, shown on the page and by their program.
	Notice string
	// ReleaseURL is the ceremony's GitHub release, where participants get
	// the program (https only); the page links to it. The coordinator
	// itself serves no participant code.
	ReleaseURL string
	// Log receives what happens: turns given, lost and taken.
	Log *log.Logger

	now func() time.Time // the clock; time.Now unless a test sets it
}

// Answers to a participant asking for their turn (Reply.Status).
const (
	Queued = "queued" // wait, and ask again after Reply.Poll seconds
	Go     = "go"     // your turn: download the state, contribute, upload by Reply.Deadline
	Done   = "done"   // your contribution is in the transcript
	Closed = "closed" // the phase takes no more contributions
)

// ClientConfig tells a participant's program how to join (GET /v1/config).
type ClientConfig struct {
	Auth           Auth    `json:"auth"`
	GitHubClientID string  `json:"github_client_id,omitzero"`
	Notice         string  `json:"notice,omitzero"`
	Heartbeat      float64 `json:"heartbeat,omitzero"` // seconds between the turn holder's heartbeats
}

// Registration asks to join an open or GitHub phase (POST /v1/register).
type Registration struct {
	Name        string `json:"name,omitzero"`         // AuthOpen: the name shown; "anonymous" when empty
	GitHubToken string `json:"github_token,omitzero"` // AuthGitHub: a token from GitHub's device flow, used once
}

// Session is a registered participant's token, for the requests that take one.
type Session struct {
	Token       string `json:"token"`
	Participant string `json:"participant"` // the name the transcript will show
}

// Reply answers a participant asking for their turn.
type Reply struct {
	Status   string    `json:"status"`
	Position int       `json:"position,omitzero"` // Queued: 1 is next
	Wait     float64   `json:"wait,omitzero"`     // Queued: an estimate of the seconds until your turn
	Index    uint32    `json:"index,omitzero"`    // Go: the latest state's; Done: your contribution's
	SHA256   string    `json:"sha256,omitzero"`   // Go: the latest state's; Done: your contribution's
	Deadline time.Time `json:"deadline,omitzero"` // Go
	Poll     float64   `json:"poll,omitzero"`     // Queued: seconds until you ask again
	Notice   string    `json:"notice,omitzero"`   // Closed
}

// Status is what anyone may see of the phase (GET /v1/status).
type Status struct {
	Phase         string    `json:"phase"`
	Ceremony      string    `json:"ceremony,omitzero"`
	Domain        uint64    `json:"domain,omitzero"`
	Circuits      []string  `json:"circuits,omitzero"`
	Auth          Auth      `json:"auth"`
	Contributions int       `json:"contributions"`
	Latest        string    `json:"latest"`
	Queue         int       `json:"queue"`
	Contributing  string    `json:"contributing,omitzero"`
	AverageTurn   float64   `json:"average_turn,omitzero"`  // seconds, over the last few turns
	AverageCheck  float64   `json:"average_check,omitzero"` // seconds the coordinator takes to check an upload, over the last few
	Closed        bool      `json:"closed"`
	CloseAt       time.Time `json:"close_at,omitzero"`
	Notice        string    `json:"notice,omitzero"`
	ReleaseURL    string    `json:"release_url,omitzero"` // where participants get the program (Config.ReleaseURL)
	Version       string    `json:"version,omitzero"`     // the commit the coordinator was built from
}

// registrationsPerHour bounds the registrations one network address makes
// in an hour, so an open phase's sessions cannot be made without end.
const registrationsPerHour = 100

// Server coordinates one phase.
type Server struct {
	cfg   Config
	start ceremony.Info // state-0

	mu         sync.Mutex
	entries    []transcript.Entry // entries[i] is state-(i+1)
	latest     state              // the state the next contribution builds on
	done       map[string]int     // identity -> their entry
	sessions   map[[32]byte]*session
	registered map[string][]time.Time // address -> its registrations in the last hour
	queue      []waiter               // in turn order
	turn       *turn                  // nil when no one holds the turn
	turnsGiven map[string]int         // identity -> turns given it, while it has not contributed
	turnTimes  []time.Duration        // the last few accepted turns' lengths
	checkTimes []time.Duration        // how long the last few uploads took to check
	closed     bool
	download   chan struct{} // bounds the public downloads of states at once
}

// state is one state file of the phase.
type state struct {
	index uint32
	hash  [32]byte
	size  int64
}

// participant is who a request comes from.
type participant struct {
	name     string // shown in the transcript
	identity string // what contributes once: open:<session> or github:<id>
	addr     string // the network address the request came from
}

// session is a registered participant of an open or GitHub phase.
type session struct {
	p    participant
	seen time.Time // their last request
}

// waiter is a queued participant.
type waiter struct {
	p    participant
	seen time.Time // when they last asked for their turn
}

// turn is a participant's turn.
type turn struct {
	p                 participant
	started, deadline time.Time
	seen              time.Time // their program's last heartbeat
	downloaded        bool      // they have started downloading the state
	uploading         bool      // their upload is coming in or being checked; it ends the turn either way
}

// New opens the phase in cfg.Dir, carrying on after its latest accepted contribution.
func New(cfg Config) (*Server, error) {
	if cfg.now == nil {
		cfg.now = time.Now
	}
	if cfg.GitHubAPI == "" {
		cfg.GitHubAPI = "https://api.github.com"
	}
	if cfg.GitHub == "" {
		cfg.GitHub = "https://github.com"
	}
	if cfg.Log == nil {
		cfg.Log = log.New(io.Discard, "", 0)
	}
	// Zero takes the default; a negative value is a mistake, not a way to
	// turn a limit off (audit R-059, R-061).
	for _, d := range []struct {
		name string
		v    *time.Duration
		def  time.Duration
	}{
		{"Turn", &cfg.Turn, 60 * time.Minute}, {"TurnPresence", &cfg.TurnPresence, 2 * time.Minute},
		{"Presence", &cfg.Presence, 30 * time.Second}, {"Away", &cfg.Away, 10 * time.Minute},
		{"Poll", &cfg.Poll, 5 * time.Second}, {"GitHubMinAge", &cfg.GitHubMinAge, 30 * 24 * time.Hour},
	} {
		switch {
		case *d.v < 0:
			return nil, fmt.Errorf("coordinator: %s is %s, negative", d.name, *d.v)
		case *d.v == 0:
			*d.v = d.def
		}
	}
	for _, n := range []struct {
		name string
		v    *int
		def  int
	}{
		{"MaxPerAddress", &cfg.MaxPerAddress, 3}, {"IPv6Prefix", &cfg.IPv6Prefix, 64},
		{"MaxQueue", &cfg.MaxQueue, 1000}, {"MaxTurns", &cfg.MaxTurns, 3},
	} {
		switch {
		case *n.v < 0:
			return nil, fmt.Errorf("coordinator: %s is %d, negative", n.name, *n.v)
		case *n.v == 0:
			*n.v = n.def
		}
	}
	switch {
	case cfg.IPv6Prefix > 128:
		return nil, fmt.Errorf("coordinator: IPv6Prefix is %d, more bits than an IPv6 address has", cfg.IPv6Prefix)
	case cfg.Presence < 2*cfg.Poll:
		return nil, fmt.Errorf("coordinator: Presence (%s) is less than twice Poll (%s): queued participants, asking every Poll, would be passed over", cfg.Presence, cfg.Poll)
	case cfg.Away < cfg.Presence:
		return nil, fmt.Errorf("coordinator: Away (%s) is less than Presence (%s)", cfg.Away, cfg.Presence)
	}
	switch cfg.Auth {
	case "":
		cfg.Auth = AuthOpen
	case AuthOpen:
	case AuthGitHub:
		if cfg.GitHubClientID == "" {
			return nil, errors.New("coordinator: GitHub sign-in needs the OAuth app's client ID")
		}
	default:
		return nil, fmt.Errorf("coordinator: Auth is %q, want %s or %s", cfg.Auth, AuthOpen, AuthGitHub)
	}
	if cfg.ReleaseURL != "" {
		// The page makes it a link: an https address, nothing else.
		if u, err := url.Parse(cfg.ReleaseURL); err != nil || u.Scheme != "https" || u.Host == "" {
			return nil, fmt.Errorf("coordinator: ReleaseURL %q is not an https address", cfg.ReleaseURL)
		}
	}
	s := &Server{
		cfg: cfg, done: map[string]int{}, sessions: map[[32]byte]*session{},
		registered: map[string][]time.Time{}, turnsGiven: map[string]int{}, download: make(chan struct{}, 4),
	}
	var err error
	if s.start, err = ceremony.Inspect(s.path(0)); err != nil {
		return nil, err
	}
	if s.start.Kind != ceremony.KindPhase1 && s.start.Kind != ceremony.KindPhase2 {
		return nil, fmt.Errorf("%s holds a %s, not the start of a phase", s.path(0), s.start.Kind)
	}
	if s.start.Index != 0 {
		return nil, fmt.Errorf("%s is contribution %d, not the start of a phase", s.path(0), s.start.Index)
	}
	t, err := transcript.Load(cfg.Dir)
	if err != nil {
		return nil, err
	}
	for i, e := range t.Contributions {
		s.done[e.Identity] = i
	}
	s.entries = t.Contributions
	s.latest.index = uint32(len(s.entries))
	if s.latest.hash, s.latest.size, err = transcript.HashFile(s.path(s.latest.index)); err != nil {
		return nil, err
	}
	if got := hex.EncodeToString(s.latest.hash[:]); got != t.Latest() {
		return nil, fmt.Errorf("%s has sha256 %s, its entry says %s", s.path(s.latest.index), got, t.Latest())
	}
	// A phase closed at its time stays closed (ClosedFile).
	switch _, err := os.Stat(filepath.Join(cfg.Dir, ClosedFile)); {
	case err == nil:
		s.closed = true
	case !errors.Is(err, os.ErrNotExist):
		return nil, err
	}
	// An upload interrupted by a stop is not part of the phase.
	if err := os.RemoveAll(filepath.Join(cfg.Dir, "incoming")); err != nil {
		return nil, err
	}
	if err := os.Mkdir(filepath.Join(cfg.Dir, "incoming"), 0o755); err != nil {
		return nil, err
	}
	return s, nil
}

// path is state i's file.
func (s *Server) path(i uint32) string { return transcript.Path(s.cfg.Dir, i) }

// States lists the accepted contributions' files in order: what a seal replays.
func (s *Server) States() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	paths := make([]string, s.latest.index)
	for i := range paths {
		paths[i] = s.path(uint32(i + 1))
	}
	return paths
}

// Close stops handing out turns. A turn under way may still finish.
func (s *Server) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
}

// Busy reports whether someone holds the turn.
func (s *Server) Busy() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.advance(s.cfg.now())
	return s.turn != nil
}

// Closed reports whether the phase has stopped handing out turns, by Close
// or at Config.CloseAt.
func (s *Server) Closed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.advance(s.cfg.now())
	return s.closed
}

// ClosedFile, in a phase's directory, marks a phase that has closed at its
// scheduled time (Config.CloseAt): a coordinator started on it again serves
// it closed, whatever its own CloseAt. An interrupted phase (Close) is not
// marked: a coordinator stopped to be restarted carries on.
const ClosedFile = "closed"

// advance closes the phase at its time, ends a turn past its time, drops
// queued participants away too long, forgets unused registrations, and hands
// a free turn to the first queued participant still asking. s.mu is held.
func (s *Server) advance(now time.Time) {
	if !s.closed && !s.cfg.CloseAt.IsZero() && !now.Before(s.cfg.CloseAt) {
		s.cfg.Log.Printf("closed, as scheduled: no new turns")
		s.closed = true
		mark := []byte(s.cfg.CloseAt.UTC().Format(time.RFC3339) + "\n")
		if err := os.WriteFile(filepath.Join(s.cfg.Dir, ClosedFile), mark, 0o644); err != nil {
			s.cfg.Log.Printf("marking the phase closed: %v", err)
		}
	}
	if t := s.turn; t != nil && !t.uploading {
		switch {
		case now.After(t.deadline):
			s.cfg.Log.Printf("%s's turn ran out", t.p.name)
			s.turn = nil
		case now.Sub(t.seen) > s.cfg.TurnPresence:
			s.cfg.Log.Printf("%s's program stopped answering during their turn", t.p.name)
			s.turn = nil
		case !t.downloaded && now.Sub(t.started) > s.cfg.TurnPresence:
			// Heartbeats show a program is there, not that it is
			// contributing: one that does not even fetch the state holds
			// the queue up for nothing (audit R-019).
			s.cfg.Log.Printf("%s's program did not start downloading the state", t.p.name)
			s.turn = nil
		}
	}
	kept := s.queue[:0]
	for _, w := range s.queue {
		if now.Sub(w.seen) <= s.cfg.Away {
			kept = append(kept, w)
		} else {
			s.cfg.Log.Printf("%s stopped asking and left the queue", w.p.name)
		}
	}
	s.queue = kept
	for k, sess := range s.sessions {
		if s.expired(sess, now) {
			delete(s.sessions, k)
		}
	}
	// The turn goes to the first queued participant still asking. One not
	// asking just now (a laptop asleep, a background tab throttled) is passed
	// over but keeps their place, as long as Away allows (audit R-020).
	if s.turn == nil && !s.closed {
		if i := slices.IndexFunc(s.queue, func(w waiter) bool { return now.Sub(w.seen) <= s.cfg.Presence }); i >= 0 {
			w := s.queue[i]
			s.queue = slices.Delete(s.queue, i, i+1)
			s.turn = &turn{p: w.p, started: now, seen: now, deadline: now.Add(s.cfg.Turn)}
			s.turnsGiven[w.p.identity]++
			s.cfg.Log.Printf("%s's turn, on top of state %d, until %s", w.p.name, s.latest.index, s.turn.deadline.Format(time.TimeOnly))
		}
	}
}

// expired reports whether a registration has gone unused for longer than
// Config.Away, its participant neither queued nor holding the turn. s.mu is held.
func (s *Server) expired(sess *session, now time.Time) bool {
	return now.Sub(sess.seen) > s.cfg.Away && !s.queued(sess.p) && !s.holds(sess.p)
}

// holds reports whether p holds the turn. s.mu is held.
func (s *Server) holds(p participant) bool { return s.turn != nil && s.turn.p.identity == p.identity }

// queued reports whether p is queued. s.mu is held.
func (s *Server) queued(p participant) bool {
	return slices.ContainsFunc(s.queue, func(w waiter) bool { return w.p.identity == p.identity })
}

// Handler serves the phase:
//
//	GET  /                  the phase's page: follow the phase, and where to get the program
//	POST /github/login/…    GitHub's device flow, relayed (AuthGitHub)
//	GET  /v1/config         how to join (ClientConfig)
//	POST /v1/register       join the phase (Registration -> Session)
//	POST /v1/turn           a participant asks for their turn (Reply)
//	POST /v1/heartbeat      the turn holder's program says it is still at it
//	GET  /v1/state          the turn holder downloads the latest state
//	PUT  /v1/contribution   the turn holder uploads their contribution
//	GET  /v1/status         the phase at a glance (Status)
//	GET  /v1/transcript     every accepted contribution (transcript.Transcript)
//	GET  /v1/states/{i}     state i's file, for anyone checking
//
// /v1/turn, /v1/heartbeat, /v1/state and /v1/contribution take
// "Authorization: Bearer <token>", the Session's token.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handlePage)
	mux.HandleFunc("POST /github/login/device/code", s.handleGitHubRelay)
	mux.HandleFunc("POST /github/login/oauth/access_token", s.handleGitHubRelay)
	mux.HandleFunc("GET /v1/config", s.handleConfig)
	mux.HandleFunc("POST /v1/register", s.handleRegister)
	mux.HandleFunc("POST /v1/turn", s.handleTurn)
	mux.HandleFunc("POST /v1/heartbeat", s.handleHeartbeat)
	mux.HandleFunc("GET /v1/state", s.handleState)
	mux.HandleFunc("PUT /v1/contribution", s.handleContribution)
	mux.HandleFunc("GET /v1/status", s.handleStatus)
	mux.HandleFunc("GET /v1/transcript", s.handleTranscript)
	mux.HandleFunc("GET /v1/states/{index}", s.handleStates)
	return mux
}

// address is the network address r comes from, as the limits count it: its
// connection's, or, from a trusted proxy (Config.TrustedProxies), the last
// address that proxy added to X-Forwarded-For. It is always an IP address,
// parsed, never the header's text, and an IPv6 address counts as its
// Config.IPv6Prefix (audit R-016, R-017, R-018).
func (s *Server) address(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return "unknown"
	}
	ip = ip.Unmap().WithZone("")
	if s.trusted(ip) {
		if fwd := r.Header.Values("X-Forwarded-For"); len(fwd) > 0 {
			hops := strings.Split(fwd[len(fwd)-1], ",")
			// What is not an address leaves the proxy's own: all such
			// requests then count as one, which fails closed.
			if a, err := netip.ParseAddr(strings.TrimSpace(hops[len(hops)-1])); err == nil {
				ip = a.Unmap().WithZone("")
			}
		}
	}
	if ip.Is6() {
		if p, err := ip.Prefix(s.cfg.IPv6Prefix); err == nil {
			return p.String()
		}
	}
	return ip.String()
}

// trusted reports whether ip is one of Config.TrustedProxies.
func (s *Server) trusted(ip netip.Addr) bool {
	for _, p := range s.cfg.TrustedProxies {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// participant returns who the request's session token belongs to, or answers 401.
func (s *Server) participant(w http.ResponseWriter, r *http.Request) (participant, bool) {
	if token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
		h := sha256.Sum256([]byte(token))
		s.mu.Lock()
		now := s.cfg.now()
		sess, ok := s.sessions[h]
		if ok && s.expired(sess, now) {
			delete(s.sessions, h)
			ok = false
		}
		var p participant
		if ok {
			sess.seen = now
			sess.p.addr = s.address(r)
			p = sess.p // a copy: another request with this token may change sess.p once unlocked
		}
		s.mu.Unlock()
		if ok {
			return p, true
		}
	}
	fail(w, http.StatusUnauthorized, "no registration with this token")
	return participant{}, false
}

// handleConfig tells a participant's program how to join.
func (s *Server) handleConfig(w http.ResponseWriter, _ *http.Request) {
	reply(w, ClientConfig{
		Auth: s.cfg.Auth, GitHubClientID: s.cfg.GitHubClientID, Notice: s.cfg.Notice,
		Heartbeat: s.heartbeat().Seconds(),
	})
}

// heartbeat is how often the turn holder's program is to say it is still at
// it: three times in every TurnPresence, and at least every 10 seconds, so a
// short TurnPresence does not pass every turn on.
func (s *Server) heartbeat() time.Duration {
	return min(max(s.cfg.TurnPresence/3, 10*time.Millisecond), 10*time.Second)
}

// handleRegister admits a participant and gives them a session token. A
// registration counts against its address's hourly limit, so it must come
// from a participant's program, not from a page elsewhere that makes its
// visitors' browsers post (audit R-021): a JSON content type needs a
// preflight no other site's page gets past, and a browser saying the request
// is cross-site is refused.
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		fail(w, http.StatusUnsupportedMediaType, "want a JSON registration (Content-Type: application/json)")
		return
	}
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		fail(w, http.StatusForbidden, "registrations come from a participant's program, not from another site")
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		if u, err := url.Parse(origin); err != nil || u.Host != r.Host {
			fail(w, http.StatusForbidden, "registrations come from a participant's program, not from another site")
			return
		}
	}
	var reg Registration
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&reg); err != nil {
		fail(w, http.StatusBadRequest, "want a JSON registration")
		return
	}
	addr, now := s.address(r), s.cfg.now()
	s.mu.Lock()
	allowed := s.allowRegistration(addr, now)
	s.mu.Unlock()
	if !allowed {
		w.Header().Set("Retry-After", "600")
		fail(w, http.StatusTooManyRequests, fmt.Sprintf("at most %d registrations an hour from one network address", registrationsPerHour))
		return
	}
	var p participant
	switch s.cfg.Auth {
	case AuthOpen:
		name := "anonymous"
		if strings.TrimSpace(reg.Name) != "" {
			var err error
			if name, err = cleanName(reg.Name); err != nil {
				fail(w, http.StatusBadRequest, err.Error())
				return
			}
		}
		p = participant{name: name, identity: "open:" + randomToken(), addr: addr}
	case AuthGitHub:
		account, err := s.githubUser(r.Context(), reg.GitHubToken)
		switch {
		case errors.Is(err, errGitHubToken):
			fail(w, http.StatusUnauthorized, "GitHub did not accept the sign-in")
			return
		case err != nil:
			s.cfg.Log.Printf("asking GitHub who signed in: %v", err)
			fail(w, http.StatusBadGateway, "GitHub could not be asked who signed in; try again")
			return
		case account.Created.Add(s.cfg.GitHubMinAge).After(now):
			fail(w, http.StatusForbidden, fmt.Sprintf("GitHub accounts younger than %s cannot contribute", s.cfg.GitHubMinAge))
			return
		}
		p = participant{name: account.Login, identity: fmt.Sprintf("github:%d", account.ID), addr: addr}
	}
	token := randomToken()
	s.mu.Lock()
	s.sessions[sha256.Sum256([]byte(token))] = &session{p: p, seen: now}
	s.mu.Unlock()
	s.cfg.Log.Printf("%s registered (%s)", p.name, p.identity)
	reply(w, Session{Token: token, Participant: p.name})
}

// allowRegistration counts a registration from addr now, unless addr has
// made registrationsPerHour in the last hour. s.mu is held.
func (s *Server) allowRegistration(addr string, now time.Time) bool {
	recent := slices.DeleteFunc(s.registered[addr], func(t time.Time) bool { return now.Sub(t) >= time.Hour })
	if len(recent) >= registrationsPerHour {
		s.registered[addr] = recent
		return false
	}
	s.registered[addr] = append(recent, now)
	for a, times := range s.registered { // forget addresses with none left in the hour
		if len(times) == 0 || now.Sub(times[len(times)-1]) >= time.Hour {
			delete(s.registered, a)
		}
	}
	return true
}

// handleTurn queues the participant, or answers that their turn has come,
// their contribution is in, or the phase is closed.
func (s *Server) handleTurn(w http.ResponseWriter, r *http.Request) {
	p, ok := s.participant(w, r)
	if !ok {
		return
	}
	now := s.cfg.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if i, ok := s.done[p.identity]; ok {
		e := s.entries[i]
		reply(w, Reply{Status: Done, Index: e.Index, SHA256: e.SHA256})
		return
	}
	s.advance(now)
	if !s.holds(p) {
		if s.closed {
			reply(w, Reply{Status: Closed, Notice: s.cfg.Notice})
			return
		}
		if !s.queued(p) {
			retry := strconv.Itoa(int(math.Ceil(s.cfg.Poll.Seconds())))
			switch {
			case s.turnsGiven[p.identity] >= s.cfg.MaxTurns: // audit R-019
				fail(w, http.StatusForbidden, fmt.Sprintf(
					"you have been given %d turns without contributing: this phase gives you no more", s.cfg.MaxTurns))
				return
			case len(s.queue) >= s.cfg.MaxQueue:
				w.Header().Set("Retry-After", retry)
				fail(w, http.StatusTooManyRequests, fmt.Sprintf(
					"the queue is full, at %d; you will be queued when there is room", s.cfg.MaxQueue))
				return
			case !s.room(p.addr):
				w.Header().Set("Retry-After", retry)
				fail(w, http.StatusTooManyRequests, fmt.Sprintf(
					"%d participants from your network address are queued already; you will be queued when one of them is done", s.cfg.MaxPerAddress))
				return
			}
		}
		s.seen(p, now)
		s.advance(now)
	}
	if s.holds(p) {
		s.turn.seen = now // asking for it shows the holder is there, as a heartbeat does
		reply(w, Reply{Status: Go, Index: s.latest.index, SHA256: hex.EncodeToString(s.latest.hash[:]), Deadline: s.turn.deadline})
		return
	}
	for i, q := range s.queue {
		if q.p.identity == p.identity {
			reply(w, Reply{Status: Queued, Position: i + 1, Wait: float64(i+1) * s.averageTurn().Seconds(), Poll: s.cfg.Poll.Seconds()})
			return
		}
	}
	panic("unreachable: a participant who asked is neither queued nor holding the turn")
}

// handleHeartbeat keeps the turn of a holder whose program is still at it.
// Unlike asking for a turn, it never queues: a heartbeat sent on after the
// turn has passed on, by a program still contributing, is answered 409, as
// its upload will be, and leaves the participant out of the queue until
// they ask.
func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	p, ok := s.participant(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if t := s.holder(w, p); t != nil {
		t.seen = s.cfg.now()
		w.WriteHeader(http.StatusNoContent)
	}
}

// room reports whether one more participant from addr may be queued. s.mu is held.
func (s *Server) room(addr string) bool {
	return s.cfg.NoAddressLimit || s.fromAddress(addr) < s.cfg.MaxPerAddress
}

// fromAddress counts the participants from addr queued or holding the turn. s.mu is held.
func (s *Server) fromAddress(addr string) int {
	n := 0
	for _, w := range s.queue {
		if w.p.addr == addr {
			n++
		}
	}
	if s.turn != nil && s.turn.p.addr == addr {
		n++
	}
	return n
}

// seen marks p as having asked now, queueing them last if they weren't
// queued. A queued participant asking from another address is counted
// there only if it has room (audit R-015): otherwise an entry could be moved
// off a full address, and the place it left taken again, without end. s.mu
// is held.
func (s *Server) seen(p participant, now time.Time) {
	for i := range s.queue {
		if s.queue[i].p.identity == p.identity {
			s.queue[i].seen = now
			if p.addr != s.queue[i].p.addr && s.room(p.addr) {
				s.queue[i].p.addr = p.addr
			}
			return
		}
	}
	s.queue = append(s.queue, waiter{p: p, seen: now})
	s.cfg.Log.Printf("%s joined the queue, at %d", p.name, len(s.queue))
}

// averageTurn is the mean length of the last few accepted turns. Before this
// coordinator has seen any (after a restart), it is the mean time between the
// transcript's last few contributions, leaving out gaps longer than a turn
// lasts: those held time with no turn under way, such as a night nobody
// asked. Zero before there is a gap to go by. s.mu is held.
func (s *Server) averageTurn() time.Duration {
	times := s.turnTimes
	if len(times) == 0 {
		recent := s.entries[max(len(s.entries)-11, 0):]
		for i := 1; i < len(recent); i++ {
			if gap := recent[i].At.Sub(recent[i-1].At); gap <= s.cfg.Turn {
				times = append(times, gap)
			}
		}
	}
	return mean(times)
}

// mean is the mean of ds; zero when there are none.
func mean(ds []time.Duration) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	var sum time.Duration
	for _, d := range ds {
		sum += d
	}
	return sum / time.Duration(len(ds))
}

// holder returns the turn when p holds it, and otherwise answers 409. s.mu is held.
func (s *Server) holder(w http.ResponseWriter, p participant) *turn {
	s.advance(s.cfg.now())
	if !s.holds(p) {
		fail(w, http.StatusConflict, "it is not your turn")
		return nil
	}
	return s.turn
}

// handleState sends the turn holder the latest state.
func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	p, ok := s.participant(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	t := s.holder(w, p)
	latest := s.latest
	var deadline time.Time
	if t != nil {
		t.downloaded = true
		deadline = t.deadline
	}
	s.mu.Unlock()
	if t == nil {
		return
	}
	if err := http.NewResponseController(w).SetWriteDeadline(deadline); err != nil && !errors.Is(err, http.ErrNotSupported) {
		s.cfg.Log.Printf("sending %s state %d: %v", p.name, latest.index, err)
	}
	w.Header().Set("X-Ceremony-Index", strconv.FormatUint(uint64(latest.index), 10))
	w.Header().Set("X-Ceremony-Sha256", hex.EncodeToString(latest.hash[:]))
	if err := sendFile(w, s.path(latest.index)); err != nil {
		s.cfg.Log.Printf("sending %s state %d: %v", p.name, latest.index, err)
	}
}

// handleContribution takes the turn holder's contribution, checks it on top
// of the latest state, and on success makes it the latest. The turn ends
// either way: a participant whose upload fails may ask again, and queues last.
func (s *Server) handleContribution(w http.ResponseWriter, r *http.Request) {
	p, ok := s.participant(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	t := s.holder(w, p)
	if t != nil && t.uploading {
		fail(w, http.StatusConflict, "your upload is already coming in")
		t = nil
	}
	if t != nil {
		t.uploading = true
	}
	prev := s.latest
	s.mu.Unlock()
	if t == nil {
		return
	}
	accepted := false
	defer func() {
		if !accepted {
			s.mu.Lock()
			if s.turn == t {
				s.turn = nil
			}
			s.advance(s.cfg.now())
			s.mu.Unlock()
		}
	}()

	tmp, hash, err := s.receive(w, r, t.deadline, prev.size)
	if err != nil {
		s.cfg.Log.Printf("%s's upload failed: %v", p.name, err)
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			fail(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("a contribution is at most %d bytes", tooLarge.Limit))
		} else {
			fail(w, http.StatusBadRequest, "upload failed: "+err.Error())
		}
		return
	}
	defer func() {
		// Gone already when the rename below made it a state.
		if err := os.Remove(tmp); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.cfg.Log.Printf("removing %s: %v", tmp, err)
		}
	}()
	checking := time.Now()
	err = ceremony.Check(s.path(prev.index), tmp)
	checked := time.Since(checking)
	if err != nil {
		s.cfg.Log.Printf("%s's contribution was refused: %v", p.name, err)
		fail(w, http.StatusUnprocessableEntity, "contribution refused: "+err.Error())
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	// Accepted contributions' checks only: a refused upload often fails in a
	// moment, and would make the next honest participant's estimate short.
	s.checkTimes = append(s.checkTimes, checked)
	if len(s.checkTimes) > 10 {
		s.checkTimes = s.checkTimes[1:]
	}
	now := s.cfg.now()
	e := transcript.Entry{
		Index:       prev.index + 1,
		Participant: p.name,
		Identity:    p.identity,
		SHA256:      hex.EncodeToString(hash[:]),
		Prev:        hex.EncodeToString(prev.hash[:]),
		At:          now.UTC(),
	}
	if err := s.commit(tmp, e); err != nil {
		s.cfg.Log.Printf("saving %s's contribution: %v", p.name, err)
		fail(w, http.StatusInternalServerError, "the coordinator could not save your contribution")
		return
	}
	accepted = true
	s.done[p.identity] = len(s.entries)
	s.entries = append(s.entries, e)
	s.latest = state{index: e.Index, hash: hash, size: prev.size}
	if fi, err := os.Stat(s.path(e.Index)); err == nil {
		s.latest.size = fi.Size()
	}
	s.turnTimes = append(s.turnTimes, now.Sub(t.started))
	if len(s.turnTimes) > 10 {
		s.turnTimes = s.turnTimes[1:]
	}
	s.turn = nil
	s.cfg.Log.Printf("%s's contribution is state %d, sha256 %s", p.name, e.Index, e.SHA256)
	s.advance(now)
	reply(w, Reply{Status: Done, Index: e.Index, SHA256: e.SHA256})
}

// receive writes the request body to a new file in the incoming directory,
// by deadline and no larger than a state of prevSize may grow in one
// contribution, and returns the file and its sha256.
func (s *Server) receive(w http.ResponseWriter, r *http.Request, deadline time.Time, prevSize int64) (string, [32]byte, error) {
	if err := http.NewResponseController(w).SetReadDeadline(deadline); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return "", [32]byte{}, err
	}
	// A contribution is the size of the state it builds on, but for the
	// challenge each gnark state gains in its first one: 32 bytes per circuit.
	body := http.MaxBytesReader(w, r.Body, prevSize+1<<16)
	f, err := os.CreateTemp(filepath.Join(s.cfg.Dir, "incoming"), "upload-")
	if err != nil {
		return "", [32]byte{}, err
	}
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(f, h), body)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", [32]byte{}, errors.Join(err, os.Remove(f.Name()))
	}
	var digest [32]byte
	copy(digest[:], h.Sum(nil))
	return f.Name(), digest, nil
}

// commit makes the checked contribution at tmp state e.Index: its entry
// first, then the state itself, whose arrival is what counts on a restart.
// s.mu is held.
func (s *Server) commit(tmp string, e transcript.Entry) error {
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	entry, err := os.CreateTemp(filepath.Join(s.cfg.Dir, "incoming"), "entry-")
	if err != nil {
		return err
	}
	_, err = entry.Write(append(data, '\n'))
	if cerr := entry.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(entry.Name(), s.path(e.Index)+".json")
	}
	if err != nil {
		return errors.Join(err, os.Remove(entry.Name()))
	}
	return os.Rename(tmp, s.path(e.Index))
}

// handleStatus answers with the phase at a glance.
func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	reply(w, s.status())
}

// status is the phase at a glance.
func (s *Server) status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.advance(s.cfg.now())
	st := Status{
		Phase:         s.start.Kind.String(),
		Ceremony:      s.start.Ceremony,
		Domain:        s.start.N,
		Circuits:      s.start.Circuits,
		Auth:          s.cfg.Auth,
		Contributions: len(s.entries),
		Latest:        hex.EncodeToString(s.latest.hash[:]),
		Queue:         len(s.queue),
		AverageTurn:   s.averageTurn().Seconds(),
		AverageCheck:  mean(s.checkTimes).Seconds(),
		Closed:        s.closed,
		CloseAt:       s.cfg.CloseAt,
		Notice:        s.cfg.Notice,
		ReleaseURL:    s.cfg.ReleaseURL,
		Version:       Version(),
	}
	if s.turn != nil {
		st.Contributing = s.turn.p.name
	}
	return st
}

// handleTranscript answers with every accepted contribution.
func (s *Server) handleTranscript(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	t := transcript.Transcript{Start: hex.EncodeToString(s.start.Hash[:]), Contributions: slices.Clone(s.entries)}
	s.mu.Unlock()
	if t.Contributions == nil {
		t.Contributions = []transcript.Entry{}
	}
	reply(w, t)
}

// handleStates sends state i's file, a few downloads at a time.
func (s *Server) handleStates(w http.ResponseWriter, r *http.Request) {
	i, err := strconv.ParseUint(r.PathValue("index"), 10, 32)
	s.mu.Lock()
	latest := s.latest.index
	s.mu.Unlock()
	if err != nil || i > uint64(latest) {
		fail(w, http.StatusNotFound, fmt.Sprintf("states run from 0 to %d", latest))
		return
	}
	select {
	case s.download <- struct{}{}:
		defer func() { <-s.download }()
	default:
		w.Header().Set("Retry-After", "30")
		fail(w, http.StatusServiceUnavailable, "too many downloads at once; try again shortly")
		return
	}
	// A download slot is not for a reader that never finishes.
	if err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(30 * time.Minute)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		s.cfg.Log.Printf("sending state %d: %v", i, err)
	}
	if err := sendFile(w, s.path(uint32(i))); err != nil {
		s.cfg.Log.Printf("sending state %d: %v", i, err)
	}
}

// sendFile writes the file at path as the response body.
func sendFile(w http.ResponseWriter, path string) error {
	f, err := os.Open(path)
	if err != nil {
		fail(w, http.StatusInternalServerError, "state unavailable")
		return err
	}
	defer f.Close()
	if fi, err := f.Stat(); err == nil {
		w.Header().Set("Content-Length", strconv.FormatInt(fi.Size(), 10))
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	_, err = io.Copy(w, f)
	return err
}

// Version is the commit this program was built from, as Go stamps it, with
// " (modified)" when the tree had changes; empty when Go stamped none.
func Version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	var rev, modified string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			if s.Value == "true" {
				modified = " (modified)"
			}
		}
	}
	if rev == "" {
		return ""
	}
	return rev + modified
}

// randomToken returns 160 random bits, base32.
func randomToken() string {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand does not fail on supported systems
	}
	return strings.ToLower(base32.StdEncoding.EncodeToString(b))
}

// reply answers with v as JSON.
func reply(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("coordinator: writing a reply: %v", err)
	}
}

// fail answers with an error status and {"error": msg}.
func fail(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": msg}); err != nil {
		log.Printf("coordinator: writing an error: %v", err)
	}
}
