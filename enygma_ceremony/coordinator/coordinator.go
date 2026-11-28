// Package coordinator runs one phase of a ceremony as a service. Participants
// queue; each in turn downloads the latest state, contributes on their own
// machine (package participant does it for them), and uploads the next state,
// which the coordinator checks with ceremony.Check before anyone builds on it.
//
// Anyone may queue: a participant's program registers itself (AuthOpen, the
// default), or signs in with GitHub (AuthGitHub). Each admitted identity
// contributes once: a GitHub account once, and in an open phase each
// registration, which is an identity of its own. A queued participant who
// goes quiet keeps their place for a while (Config.Away) but is passed over if they are
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
	mrand "math/rand/v2"
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
	// AuthGitHub. It needs the device flow enabled. GitHubClientSecret is
	// the app's client secret: with it the coordinator asks GitHub whether a
	// participant's token is the app's, and admits no other (audit R-022).
	GitHubClientID     string
	GitHubClientSecret string
	// GitHubMinAge is how old a GitHub account must be, for AuthGitHub (30
	// days when zero).
	GitHubMinAge time.Duration
	// GitHubAPI is GitHub's API, for AuthGitHub (https://api.github.com).
	GitHubAPI string
	// MaxPerAddress bounds how many participants from one network address
	// may be queued or holding the turn at once (3 when zero); NoAddressLimit
	// lifts the bound, and MaxContributionsPerAddress, for a phase whose
	// participants all come from one address.
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
	// FairPrefix4 and FairPrefix6 are how many leading bits of an address
	// make the network turns are shared out by (24, and 48 or IPv6Prefix
	// when that is fewer, when zero): a free turn goes to the earliest
	// queued from a network drawn at random among those given the fewest
	// turns (draw; audit H3). A home or a server is given a whole IPv6 /56
	// or /48, or many IPv4 addresses in one /24, and could otherwise queue
	// ahead of everyone from each.
	FairPrefix4, FairPrefix6 int
	// Deny is the networks this phase takes no part from (Server.Deny
	// changes it while serving): an operator who sees a network hold the
	// queue denies it here when the firewall cannot, such as behind a proxy
	// of their own (audit H3).
	Deny []netip.Prefix
	// MaxQueue bounds how many participants are queued at once, from
	// everywhere (1000 when zero), and MaxPerNetwork how many from one
	// network (FairPrefix4, FairPrefix6) (10 when zero): one /48 holds tens
	// of thousands of /64s, and could otherwise fill the queue, so nobody
	// else was even drawn (audit H3). A full queue makes room for a network
	// with fewer queued (makeRoom). NoAddressLimit lifts MaxPerNetwork, as
	// it lifts MaxPerAddress.
	MaxQueue, MaxPerNetwork int
	// MaxTurns bounds the turns one identity is given without contributing
	// (3 when zero): one that holds turns without ever contributing would
	// otherwise hold up the queue again and again.
	MaxTurns int
	// MaxContributions bounds the contributions the phase accepts (1000 when
	// zero), and MaxContributionsPerAddress those accepted from one network
	// address (10 when zero; counted since the coordinator started). An open
	// phase's every registration is a new identity, and every contribution a
	// state file kept for the phase's life (audit R-012).
	MaxContributions           int
	MaxContributionsPerAddress int
	// MinFree is the disk space under Dir, besides room for one more state,
	// below which no turn is given (1 GiB when zero) (audit R-012).
	MinFree int64
	// MaxSessions bounds the registrations kept at once, and the network
	// addresses whose registrations are counted (10,000 when zero) (audit R-013).
	MaxSessions int
	// Turn is how long a participant has, from being handed the turn, to
	// finish uploading their contribution (60 minutes), of which they have
	// UploadWithin from their download to start uploading: room for a slow
	// laptop, since a turn whose holder has gone passes on at TurnPresence.
	Turn time.Duration
	// TurnPresence is how long the turn holder's program may go without
	// a heartbeat before the turn passes on (2 minutes): at least twice Poll,
	// so a queued program hears of its turn in time (audit L3).
	TurnPresence time.Duration
	// UploadWithin is how long the turn holder has, from first starting to
	// download the state, to start uploading their contribution (45
	// minutes, or Turn when shorter, when zero; at most Turn): room for a
	// slow link and a slow laptop,
	// while a program that downloads, or stalls a download, and sends
	// heartbeats but never contributes passes the turn on long before Turn
	// (audit M11). Downloading again does not restart it.
	UploadWithin time.Duration
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
	// Phase1 is, for a Phase 2, what the SRS it starts from was sealed from,
	// as that SRS says (ceremony.Provenance); the page shows it beside the
	// SRS's sha256 for participants to compare with the announcement.
	Phase1 *ceremony.Provenance
	// Log receives what happens: turns given, lost and taken.
	Log *log.Logger

	now  func() time.Time // the clock; time.Now unless a test sets it
	rand func(n int) int  // a number from 0 to n-1, drawn at random: rand.IntN unless a test sets it
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
	Position int       `json:"position,omitzero"` // Queued: a rough rank, 1 being next, were turns given to the fewest served, then in the order queued (Server.place)
	Draw     int       `json:"draw,omitzero"`     // Queued: how many networks, yours among them, the next turn is drawn among; zero when networks served fewer come first
	Wait     float64   `json:"wait,omitzero"`     // Queued: an estimate of the seconds until your turn
	Index    uint32    `json:"index,omitzero"`    // Go: the latest state's; Done: your contribution's
	SHA256   string    `json:"sha256,omitzero"`   // Go: the latest state's; Done: your contribution's
	Deadline time.Time `json:"deadline,omitzero"` // Go: by when the upload must have started (UploadWithin of the download, or the turn's end)
	Poll     float64   `json:"poll,omitzero"`     // Queued: seconds until you ask again
	Notice   string    `json:"notice,omitzero"`   // Closed
}

// Status is what anyone may see of the phase (GET /v1/status).
type Status struct {
	Phase         string   `json:"phase"`
	Ceremony      string   `json:"ceremony,omitzero"`
	Domain        uint64   `json:"domain,omitzero"`
	Circuits      []string `json:"circuits,omitzero"`
	R1CS          []string `json:"r1cs_sha256,omitzero"` // Phase 2: each circuit's constraint system's sha256, in order
	SRS           string   `json:"srs_sha256,omitzero"`  // Phase 2: the SRS it starts from (audit H1)
	Phase1        *Phase1  `json:"phase1,omitzero"`      // Phase 2: what that SRS says it was sealed from
	Auth          Auth     `json:"auth"`
	Contributions int      `json:"contributions"`
	Latest        string   `json:"latest"`
	Queue         int      `json:"queue"`
	Contributing  string   `json:"contributing,omitzero"`
	AverageTurn   float64  `json:"average_turn,omitzero"`  // seconds, over the last few turns
	AverageCheck  float64  `json:"average_check,omitzero"` // seconds the coordinator takes to check an upload, over the last few
	// Wasted counts the turns this coordinator handed out that ended without
	// a contribution: many, with an average turn near Config.Turn, is the
	// queue held by programs that do not contribute (audit H3, M11).
	Wasted     int       `json:"turns_without_contribution"`
	Closed     bool      `json:"closed"`
	CloseAt    time.Time `json:"close_at,omitzero"`
	Notice     string    `json:"notice,omitzero"`
	ReleaseURL string    `json:"release_url,omitzero"` // where participants get the program (Config.ReleaseURL)
	Version    string    `json:"version,omitzero"`     // the commit the coordinator was built from
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
	done       map[identity]int   // identity -> their entry
	sessions   map[[32]byte]*session
	registered map[string][]time.Time // address -> its registrations in the last hour
	queue      []waiter               // in the order queued; turns are drawn (draw)
	turn       *turn                  // nil when no one holds the turn
	turnsGiven map[identity]int       // identity -> turns given it, while it has not contributed
	// turnsByNetwork counts the turns given each network (participant.network)
	// since this coordinator started, contributed or not: turns go first to
	// the networks given fewest (draw, audit H3).
	turnsByNetwork map[string]int
	wasted         int             // turns given that ended without a contribution (Status.Wasted)
	turnTimes      []time.Duration // the last few accepted turns' lengths
	checkTimes     []time.Duration // how long the last few uploads took to check
	closed         bool            // the phase takes no more contributions: Close, or Config.CloseAt
	// stopping is the coordinator about to stop (Stop, Release): it hands out
	// no more turns, but the phase is not closed, and the queue waits for the
	// coordinator started next.
	stopping bool
	// downloading holds the network addresses with a public download of a
	// state under way: one each, publicDownloads in all.
	downloading map[string]bool
	// contributedFrom counts the contributions accepted from each network
	// address since this coordinator started (Config.MaxContributionsPerAddress).
	contributedFrom map[string]int
	// lastSweep is when advance last forgot unused registrations and
	// addresses: at most every sweepEvery, not on every request (audit R-013).
	lastSweep time.Time
	diskShort bool           // turns are held back for want of disk space (Config.MinFree)
	deny      []netip.Prefix // Config.Deny, as Deny last set it
	serving   bool           // Handler was called: the server takes uploads, and Append is refused
	lock      *os.File       // holds Dir's lock (LockFile) while the server lives
	// githubMinute and githubAsked count the registrations asked of GitHub
	// in the current minute (githubLookups).
	githubMinute time.Time
	githubAsked  int
}

// state is one state file of the phase.
type state struct {
	index uint32
	hash  [32]byte
	size  int64
}

// participant is who a request comes from.
type participant struct {
	name     string     // shown in the transcript
	identity identity   // what contributes once
	addr     string     // the network address the request came from, as the limits count it (address)
	network  string     // the wider network it is in, as turns are shared out (network)
	ip       netip.Addr // the address itself, as Deny matches it: set by participant on each request with the session's token
}

// identity is what a participant is admitted as, and contributes once as:
// github:<user id>, or in an open phase open:<random>, a value of the
// registration's own. The transcript publishes it (transcript.Entry), so it
// is never a sessionToken, of which it is a type apart (audit R-074).
type identity string

// sessionToken is a registration's bearer secret (Session.Token): the
// participant's alone. The coordinator keeps only its sha256.
type sessionToken string

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
	downloaded        time.Time // when their program first started downloading the state; zero until then
	uploading         bool      // their upload is coming in or being checked; it ends the turn either way
}

// New opens the phase in cfg.Dir, carrying on after its latest accepted
// contribution. It locks the directory (LockFile) until Release, or until
// the Server is garbage collected.
func New(cfg Config) (_ *Server, err error) {
	if cfg.now == nil {
		cfg.now = time.Now
	}
	if cfg.rand == nil {
		cfg.rand = mrand.IntN // seeded from the operating system's randomness: nobody knows the draw ahead
	}
	if cfg.GitHubAPI == "" {
		cfg.GitHubAPI = "https://api.github.com"
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
		{"UploadWithin", &cfg.UploadWithin, -1},
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
		{"FairPrefix4", &cfg.FairPrefix4, 24}, {"FairPrefix6", &cfg.FairPrefix6, -1},
		{"MaxQueue", &cfg.MaxQueue, 1000}, {"MaxPerNetwork", &cfg.MaxPerNetwork, 10}, {"MaxTurns", &cfg.MaxTurns, 3},
		{"MaxContributions", &cfg.MaxContributions, 1000}, {"MaxContributionsPerAddress", &cfg.MaxContributionsPerAddress, 10},
		{"MaxSessions", &cfg.MaxSessions, 10000},
	} {
		switch {
		case *n.v < 0:
			return nil, fmt.Errorf("coordinator: %s is %d, negative", n.name, *n.v)
		case *n.v == 0:
			*n.v = n.def
		}
	}
	if cfg.UploadWithin == -1 {
		cfg.UploadWithin = min(45*time.Minute, cfg.Turn)
	}
	if cfg.FairPrefix6 == -1 {
		// No wider a network than an address, for an operator who counts
		// addresses by wider prefixes than /48 (-ipv6-prefix).
		cfg.FairPrefix6 = min(48, cfg.IPv6Prefix)
	}
	switch {
	case cfg.MinFree < 0:
		return nil, fmt.Errorf("coordinator: MinFree is %d, negative", cfg.MinFree)
	case cfg.MinFree == 0:
		cfg.MinFree = 1 << 30
	}
	switch {
	case cfg.IPv6Prefix > 128:
		return nil, fmt.Errorf("coordinator: IPv6Prefix is %d, more bits than an IPv6 address has", cfg.IPv6Prefix)
	case cfg.FairPrefix4 > 32:
		return nil, fmt.Errorf("coordinator: FairPrefix4 is %d, more bits than an IPv4 address has", cfg.FairPrefix4)
	case cfg.FairPrefix6 > cfg.IPv6Prefix:
		return nil, fmt.Errorf("coordinator: FairPrefix6 (%d) is narrower than IPv6Prefix (%d): turns are shared by networks no smaller than an address", cfg.FairPrefix6, cfg.IPv6Prefix)
	case cfg.UploadWithin > cfg.Turn:
		return nil, fmt.Errorf("coordinator: UploadWithin (%s) is longer than Turn (%s)", cfg.UploadWithin, cfg.Turn)
	case cfg.TurnPresence < 2*cfg.Poll:
		return nil, fmt.Errorf("coordinator: TurnPresence (%s) is less than twice Poll (%s): a queued program, asking every Poll, could lose turns it never heard of (audit L3)", cfg.TurnPresence, cfg.Poll)
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
		if cfg.GitHubClientSecret == "" {
			return nil, errors.New("coordinator: GitHub sign-in needs the OAuth app's client secret, to admit only the app's tokens")
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
		cfg: cfg, deny: slices.Clone(cfg.Deny), done: map[identity]int{}, sessions: map[[32]byte]*session{},
		registered: map[string][]time.Time{}, turnsGiven: map[identity]int{}, turnsByNetwork: map[string]int{},
		downloading: map[string]bool{}, contributedFrom: map[string]int{},
	}
	// One coordinator to a phase: a second would clear the first's uploads
	// in progress, and the two would race to commit (audit R-043).
	if s.lock, err = lockDir(cfg.Dir); err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			unlockDir(s.lock)
		}
	}()
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
		s.done[identity(e.Identity)] = i
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

// Release unlocks the phase's directory, for another coordinator to open.
// The Server serves nothing more that changes it, and hands out no more
// turns, as after Stop.
func (s *Server) Release() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopping = true
	unlockDir(s.lock)
	s.lock = nil
}

// Stop stops handing out turns, for the coordinator to stop or restart: a
// turn under way may still finish. The phase is not closed, so a queued
// participant asking is told to keep waiting, and their program joins the
// coordinator started next by itself; told the phase was closed, it would
// give up.
func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopping = true
}

// Close closes the phase: it hands out no more turns, and tells everyone who
// asks that the phase takes no more contributions. A turn under way may still
// finish. Only a close at Config.CloseAt is recorded in the directory.
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

// Closed reports whether the phase is closed, by Close or at
// Config.CloseAt. A stopping coordinator's phase is not (Stop).
func (s *Server) Closed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.advance(s.cfg.now())
	return s.closed
}

// LockFile, in a phase's directory, is what the coordinator serving it holds
// locked, so that no second one serves it at once (audit R-043).
const LockFile = ".lock"

// unlockDir lets go of a directory's lock, taken by lockDir.
func unlockDir(f *os.File) {
	if f != nil {
		_ = f.Close() // closing the file is what releases the lock
	}
}

// ClosedFile, in a phase's directory, marks a phase that has closed at its
// scheduled time (Config.CloseAt): a coordinator started on it again serves
// it closed, whatever its own CloseAt. An interrupted phase (Stop) is not
// marked, nor is one closed by Close: started again, it carries on.
const ClosedFile = "closed"

// advance closes the phase at its time, ends a turn past its time, drops
// queued participants away too long, forgets unused registrations, and hands
// a free turn to the queued participant still asking that draw picks. s.mu
// is held.
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
		why := ""
		switch {
		case now.After(t.deadline):
			why = "turn ran out"
		case now.Sub(t.seen) > s.cfg.TurnPresence:
			why = "program stopped answering during their turn"
		case t.downloaded.IsZero() && now.Sub(t.started) > s.cfg.TurnPresence:
			// Heartbeats show a program is there, not that it is
			// contributing: one that does not even fetch the state holds
			// the queue up for nothing (audit R-019).
			why = "program did not start downloading the state"
		case !t.downloaded.IsZero() && now.Sub(t.downloaded) > s.cfg.UploadWithin:
			// Nor does one that fetches it, or stalls fetching it, and
			// never uploads (audit M11).
			why = fmt.Sprintf("program did not upload within %s of starting to download the state", s.cfg.UploadWithin)
		}
		if why != "" {
			s.endTurn(why)
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
	if now.Sub(s.lastSweep) >= sweepEvery || now.Before(s.lastSweep) {
		s.lastSweep = now
		s.sweep(now)
	}
	// The turn goes to a queued participant still asking, drawn as draw
	// says. One not asking just now (a laptop asleep, a background tab
	// throttled) is passed over but keeps their place, as long as Away
	// allows (audit R-020).
	if s.turn == nil && !s.closed && !s.stopping && len(s.queue) > 0 && !s.full() && s.diskRoom() {
		if i := s.draw(now); i >= 0 {
			w := s.queue[i]
			s.queue = slices.Delete(s.queue, i, i+1)
			s.turn = &turn{p: w.p, started: now, seen: now, deadline: now.Add(s.cfg.Turn)}
			s.cfg.Log.Printf("%s's turn (from %s, network %s, given %d turns), on top of state %d, until %s",
				w.p.name, w.p.addr, w.p.network, s.served(w.p), s.latest.index, s.turn.deadline.Format(time.TimeOnly))
			// Charged as drawn, used or not (audit H3, R-019): a turn whose
			// holder never comes back holds the queue up for TurnPresence,
			// and costs them as one they took. TurnPresence is at least
			// twice Poll, so a program still asking always hears of it.
			s.turnsGiven[w.p.identity]++
			s.turnsByNetwork[w.p.network]++
		}
	}
}

// endTurn ends the turn under way, without a contribution, for why, and
// counts it so (Status.Wasted). s.mu is held.
func (s *Server) endTurn(why string) {
	t := s.turn
	s.turn = nil
	s.cfg.Log.Printf("%s's %s (from %s)", t.p.name, why, t.p.addr)
	s.wasted++
}

// uploadBy is when t's holder must have started uploading: UploadWithin
// from their first download (or from now, before it), and no later than
// the turn's deadline. s.mu is held.
func (s *Server) uploadBy(t *turn, now time.Time) time.Time {
	from := t.downloaded
	if from.IsZero() {
		from = now
	}
	if by := from.Add(s.cfg.UploadWithin); by.Before(t.deadline) {
		return by
	}
	return t.deadline
}

// draw picks the queued participant still asking that a free turn goes to,
// or -1 when there is none: the earliest from a network drawn at random
// among the networks whose participants (or, in a GitHub phase, accounts)
// have been given the fewest turns (audit H3). First come, first served,
// as it was, gave every turn of a phase to whoever queued many entries at
// its opening, from as few as a household's addresses: the queue is open
// to anyone, and its order was a race. Now each network served moves
// behind every network not yet served, and among those the draw is
// random, so queueing first wins nothing: to keep a participant from a
// network not yet served out with probability 1 - e over T turns, someone
// needs about T/e networks. One from a network served already waits
// behind every network not yet served, so someone with a fresh network
// for every turn can keep them waiting: a seal's -include, and a lined-up
// contributor's contribution by hand, are what then keep the phase sound.
// s.mu is held.
func (s *Server) draw(now time.Time) int {
	// The draw is among networks, not entries: a network's many entries
	// are one ticket, so to win more draws someone needs more networks,
	// not more addresses in one. Within the network drawn, its earliest
	// entry goes.
	least, picks, seen := math.MaxInt, []int(nil), map[string]bool{}
	for i, w := range s.queue {
		if now.Sub(w.seen) > s.cfg.Presence || s.addressDone(w.p.addr) {
			continue
		}
		switch n := s.served(w.p); {
		case n < least:
			least, picks, seen = n, []int{i}, map[string]bool{w.p.network: true}
		case n == least && !seen[w.p.network]:
			picks, seen[w.p.network] = append(picks, i), true
		}
	}
	if len(picks) == 0 {
		return -1
	}
	return picks[s.cfg.rand(len(picks))]
}

// served is how many turns p's network, or p (a GitHub account, whose turns
// count wherever it asks from), has been given. s.mu is held.
func (s *Server) served(p participant) int {
	return max(s.turnsByNetwork[p.network], s.turnsGiven[p.identity])
}

// sweepEvery is how often advance forgets unused registrations, and the
// addresses with no registration in the last hour: a sweep reads them all,
// so it is not made on every request (audit R-013). An unused registration
// is also refused when it is next used (participant), sweep or not.
const sweepEvery = 10 * time.Second

// sweep forgets unused registrations, and addresses with no registration in
// the last hour. s.mu is held.
func (s *Server) sweep(now time.Time) {
	queued := make(map[identity]bool, len(s.queue))
	for _, w := range s.queue {
		queued[w.p.identity] = true
	}
	for k, sess := range s.sessions {
		if now.Sub(sess.seen) > s.cfg.Away && !queued[sess.p.identity] && !s.holds(sess.p) {
			delete(s.sessions, k)
		}
	}
	for a, times := range s.registered {
		if len(times) == 0 || now.Sub(times[len(times)-1]) >= time.Hour {
			delete(s.registered, a)
		}
	}
}

// full reports whether the phase has taken Config.MaxContributions. s.mu is held.
func (s *Server) full() bool { return len(s.entries) >= s.cfg.MaxContributions }

// addressDone reports whether addr has made Config.MaxContributionsPerAddress
// contributions, which NoAddressLimit lifts with the other bound on an
// address. s.mu is held.
func (s *Server) addressDone(addr string) bool {
	return !s.cfg.NoAddressLimit && s.contributedFrom[addr] >= s.cfg.MaxContributionsPerAddress
}

// diskRoom reports whether the disk under Dir has room for one more state
// and Config.MinFree besides; when it has not, no turn is given until it has
// (audit R-012). s.mu is held.
func (s *Server) diskRoom() bool {
	free, ok := freeSpace(s.cfg.Dir)
	short := ok && free < uint64(s.cfg.MinFree)+2*uint64(max(s.latest.size, 0))
	if short != s.diskShort {
		s.diskShort = short
		if short {
			s.cfg.Log.Printf("WARNING: %d MiB free under %s: no turns until there is room for another state and %d MiB besides",
				free>>20, s.cfg.Dir, s.cfg.MinFree>>20)
		} else {
			s.cfg.Log.Printf("disk space under %s is back: turns resume", s.cfg.Dir)
		}
	}
	return !short
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
// "Authorization: Bearer <token>", the Session's token. Those and
// /v1/register refuse a network denied (Deny); the rest answer anyone.
func (s *Server) Handler() http.Handler {
	s.mu.Lock()
	s.serving = true
	s.mu.Unlock()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handlePage)
	mux.HandleFunc("GET /v1/config", s.handleConfig)
	mux.HandleFunc("POST /v1/register", s.admit(s.handleRegister))
	mux.HandleFunc("POST /v1/turn", s.admit(s.handleTurn))
	mux.HandleFunc("POST /v1/heartbeat", s.admit(s.handleHeartbeat))
	mux.HandleFunc("GET /v1/state", s.admit(s.handleState))
	mux.HandleFunc("PUT /v1/contribution", s.admit(s.handleContribution))
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
	addr, _ := s.source(r)
	return addr
}

// source is where r comes from: its address, as address says, and its
// network, the wider prefix turns are shared out by (Config.FairPrefix4,
// Config.FairPrefix6).
func (s *Server) source(r *http.Request) (addr, network string) {
	ip := s.from(r)
	if !ip.IsValid() {
		return "unknown", "unknown"
	}
	addr, network = ip.String(), ip.String()
	bits := s.cfg.FairPrefix4
	if ip.Is6() {
		if p, err := ip.Prefix(s.cfg.IPv6Prefix); err == nil {
			addr = p.String()
		}
		bits = s.cfg.FairPrefix6
	}
	if p, err := ip.Prefix(bits); err == nil {
		network = p.String()
	}
	return addr, network
}

// admit refuses a request from a network denied (Deny) with 403, and hands
// the rest to next.
func (s *Server) admit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		p := participant{ip: s.from(r)}
		if !s.denied(p.ip) {
			s.mu.Unlock()
			next(w, r)
			return
		}
		// One registered, who has come to the network denied since: their
		// place, or their turn, goes as Deny's would have. One who has
		// contributed is told so (handleTurn), as before the deny.
		if token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
			if sess, ok := s.sessions[sha256.Sum256([]byte(token))]; ok && !s.expired(sess, s.cfg.now()) {
				if _, done := s.done[sess.p.identity]; done {
					s.mu.Unlock()
					next(w, r)
					return
				}
				p.name, p.identity = sess.p.name, sess.p.identity
			}
		}
		s.refuseDenied(w, p)
		s.mu.Unlock()
	}
}

// denied reports whether ip is in a network denied (Deny). s.mu is held.
func (s *Server) denied(ip netip.Addr) bool {
	for _, p := range s.deny {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// Deny makes networks the phase takes no part from, in place of those
// denied before: their queued participants leave the queue, a turn held
// from one of them ends (or, its upload coming in, has its upload refused
// before it is committed), and their requests to take part are refused
// from now on. It reports how many left the queue.
func (s *Server) Deny(networks []netip.Prefix) (dropped int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deny = slices.Clone(networks)
	before := len(s.queue)
	s.queue = slices.DeleteFunc(s.queue, func(w waiter) bool { return s.denied(w.p.ip) })
	dropped = before - len(s.queue)
	// A turn whose upload is coming in is left to it: the upload is
	// refused before it is committed (handleContribution).
	if s.turn != nil && !s.turn.uploading && s.denied(s.turn.p.ip) {
		s.endTurn("turn ended: its network is denied")
	}
	s.cfg.Log.Printf("denying %d networks: %d queued participants from them left the queue", len(s.deny), dropped)
	s.advance(s.cfg.now())
	return dropped
}

// from is the IP address r comes from: its connection's, or, from a trusted
// proxy (Config.TrustedProxies), the last address that proxy added to
// X-Forwarded-For. Not valid when the connection's is no address.
func (s *Server) from(r *http.Request) netip.Addr {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
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
	return ip
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
			sess.p.addr, sess.p.network = s.source(r)
			sess.p.ip = s.from(r)
			p = sess.p // a copy: another request with this token may change sess.p once unlocked
		}
		s.mu.Unlock()
		if ok {
			return p, true
		}
	}
	s.fail(w, http.StatusUnauthorized, "no registration with this token")
	return participant{}, false
}

// handleConfig tells a participant's program how to join.
func (s *Server) handleConfig(w http.ResponseWriter, _ *http.Request) {
	s.reply(w, ClientConfig{
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
		s.fail(w, http.StatusUnsupportedMediaType, "want a JSON registration (Content-Type: application/json)")
		return
	}
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		s.fail(w, http.StatusForbidden, "registrations come from a participant's program, not from another site")
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		if u, err := url.Parse(origin); err != nil || u.Host != r.Host {
			s.fail(w, http.StatusForbidden, "registrations come from a participant's program, not from another site")
			return
		}
	}
	var reg Registration
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&reg); err != nil {
		s.fail(w, http.StatusBadRequest, "want a JSON registration")
		return
	}
	addr, network := s.source(r)
	now := s.cfg.now()
	s.mu.Lock()
	// Registrations, and the addresses they are counted by, are bounded in
	// number too: each is kept, and swept, under the lock (audit R-013).
	crowded := len(s.sessions) >= s.cfg.MaxSessions || (s.registered[addr] == nil && len(s.registered) >= s.cfg.MaxSessions)
	allowed := !crowded && s.allowRegistration(addr, now)
	s.mu.Unlock()
	if crowded {
		w.Header().Set("Retry-After", "60")
		s.fail(w, http.StatusServiceUnavailable, "the coordinator holds as many registrations as it takes; try again shortly")
		return
	}
	if !allowed {
		w.Header().Set("Retry-After", "600")
		s.fail(w, http.StatusTooManyRequests, fmt.Sprintf("at most %d registrations an hour from one network address", registrationsPerHour))
		return
	}
	var p participant
	switch s.cfg.Auth {
	case AuthOpen:
		name := "anonymous"
		if strings.TrimSpace(reg.Name) != "" {
			var err error
			if name, err = cleanName(reg.Name); err != nil {
				s.fail(w, http.StatusBadRequest, err.Error())
				return
			}
		}
		p = participant{name: name, identity: identity("open:" + randomToken()), addr: addr, network: network}
	case AuthGitHub:
		account, err := s.githubUser(r.Context(), reg.GitHubToken)
		switch {
		case errors.Is(err, errGitHubTokenForm):
			s.fail(w, http.StatusBadRequest, "want a GitHub token")
			return
		case errors.Is(err, errGitHubToken):
			s.fail(w, http.StatusUnauthorized, "GitHub did not accept the sign-in as one with this coordinator's app")
			return
		case errors.Is(err, errGitHubBusy):
			w.Header().Set("Retry-After", "60")
			s.fail(w, http.StatusServiceUnavailable, "too many GitHub sign-ins at once; try again shortly")
			return
		case err != nil:
			s.cfg.Log.Printf("asking GitHub who signed in: %v", err)
			s.fail(w, http.StatusBadGateway, "GitHub could not be asked who signed in; try again")
			return
		case account.Created.Add(s.cfg.GitHubMinAge).After(now):
			s.fail(w, http.StatusForbidden, fmt.Sprintf("GitHub accounts younger than %s cannot contribute", s.cfg.GitHubMinAge))
			return
		}
		p = participant{name: account.Login, identity: identity(fmt.Sprintf("github:%d", account.ID)), addr: addr, network: network}
	}
	token := sessionToken(randomToken())
	s.mu.Lock()
	s.sessions[sha256.Sum256([]byte(token))] = &session{p: p, seen: now}
	s.mu.Unlock()
	s.cfg.Log.Printf("%s registered (%s), from %s", p.name, p.identity, p.addr)
	s.reply(w, Session{Token: string(token), Participant: p.name})
}

// allowRegistration counts a registration from addr now, unless addr has
// made registrationsPerHour in the last hour. s.mu is held.
func (s *Server) allowRegistration(addr string, now time.Time) bool {
	recent := slices.DeleteFunc(s.registered[addr], func(t time.Time) bool { return now.Sub(t) >= time.Hour })
	if len(recent) >= registrationsPerHour {
		s.registered[addr] = recent
		return false
	}
	s.registered[addr] = append(recent, now) // addresses with none left in the hour go at the next sweep
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
		s.reply(w, Reply{Status: Done, Index: e.Index, SHA256: e.SHA256})
		return
	}
	s.advance(now)
	if s.refuseDenied(w, p) {
		return
	}
	if !s.holds(p) {
		switch {
		case s.closed:
			s.reply(w, Reply{Status: Closed, Notice: s.cfg.Notice})
			return
		case s.full(): // audit R-012
			s.reply(w, Reply{Status: Closed, Notice: fmt.Sprintf("this phase has taken the %d contributions it takes", s.cfg.MaxContributions)})
			return
		case s.addressDone(p.addr): // audit R-012
			s.queue = slices.DeleteFunc(s.queue, func(w waiter) bool { return w.p.identity == p.identity })
			s.fail(w, http.StatusForbidden, fmt.Sprintf(
				"%d contributions have come from your network address: this phase takes no more from it", s.cfg.MaxContributionsPerAddress))
			return
		}
		if !s.queued(p) {
			retry := strconv.Itoa(int(math.Ceil(s.cfg.Poll.Seconds())))
			switch {
			case s.turnsGiven[p.identity] >= s.cfg.MaxTurns: // audit R-019
				s.fail(w, http.StatusForbidden, fmt.Sprintf(
					"you have been given %d turns without contributing: this phase gives you no more", s.cfg.MaxTurns))
				return
			case !s.room(p.addr):
				w.Header().Set("Retry-After", retry)
				s.fail(w, http.StatusTooManyRequests, fmt.Sprintf(
					"%d participants from your network address are queued already; you will be queued when one of them is done", s.cfg.MaxPerAddress))
				return
			case !s.networkRoom(p.network):
				w.Header().Set("Retry-After", retry)
				s.fail(w, http.StatusTooManyRequests, fmt.Sprintf(
					"%d participants from your network are queued already; you will be queued when one of them is done", s.cfg.MaxPerNetwork))
				return
			case len(s.queue) >= s.cfg.MaxQueue && !s.makeRoom(p.network, now):
				w.Header().Set("Retry-After", retry)
				s.fail(w, http.StatusTooManyRequests, fmt.Sprintf(
					"the queue is full, at %d; you will be queued when there is room", s.cfg.MaxQueue))
				return
			}
		}
		if i := slices.IndexFunc(s.queue, func(q waiter) bool { return q.p.identity == p.identity }); i >= 0 &&
			(s.queue[i].p.addr != p.addr && !s.room(p.addr) ||
				s.queue[i].p.network != p.network && !s.networkRoom(p.network)) {
			// A place held from one address is kept alive from another
			// only if that one, and its network, has room: otherwise one
			// request from each address borrowed parks entries there for
			// good, counted where they were queued (audit L2), and moving
			// entries from one network to another queues a network past
			// MaxPerNetwork.
			w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(s.cfg.Poll.Seconds()))))
			s.fail(w, http.StatusTooManyRequests, fmt.Sprintf(
				"your place in the queue is held from the address you were queued from: this one, or its network, has as many queued as it may, so asking from here does not keep it, and it lapses %s after you last asked from there",
				s.cfg.Away))
			return
		}
		s.seen(p, now)
		s.advance(now)
	}
	if s.holds(p) {
		t := s.turn
		t.seen = now  // asking for it shows the holder is there, as a heartbeat does
		t.p.ip = p.ip // as holder keeps it
		// The deadline told is the one that counts: the upload must start by
		// UploadWithin of the download, which comes next.
		s.reply(w, Reply{Status: Go, Index: s.latest.index, SHA256: hex.EncodeToString(s.latest.hash[:]), Deadline: s.uploadBy(t, now)})
		return
	}
	if position, draw := s.place(p, now); position > 0 {
		s.reply(w, Reply{Status: Queued, Position: position, Draw: draw, Wait: float64(position) * s.averageTurn().Seconds(), Poll: s.cfg.Poll.Seconds()})
		return
	}
	panic("unreachable: a participant who asked is neither queued nor holding the turn")
}

// place is where queued participant p stands as turns are drawn (draw):
// position counts those still asking who would come first were turns given
// to the fewest served, then in the order queued, p included: a rough rank
// (a network served fewer turns has its turns before p's, one per turn,
// not one per entry), and the draw decides. draw counts the networks, p's
// among them, the next turn is drawn among when p's network is among them,
// none being served fewer turns; otherwise zero. Both zero when p is not
// queued. s.mu is held.
func (s *Server) place(p participant, now time.Time) (position, draw int) {
	me := slices.IndexFunc(s.queue, func(w waiter) bool { return w.p.identity == p.identity })
	if me < 0 {
		return 0, 0
	}
	mine, networks, behind := s.served(s.queue[me].p), map[string]bool{}, false
	for i, w := range s.queue {
		if now.Sub(w.seen) > s.cfg.Presence || s.addressDone(w.p.addr) { // as draw passes them over
			continue
		}
		n := s.served(w.p)
		if n < mine || n == mine && i <= me {
			position++
		}
		if n < mine {
			behind = true
		}
		if n == mine && !networks[w.p.network] {
			networks[w.p.network] = true
			draw++
		}
	}
	if behind {
		draw = 0 // networks served fewer turns are drawn first
	}
	return position, draw
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

// makeRoom makes room in a full queue for one from network, when network
// has at least two fewer queued than the network with the most: that
// network's latest entry not asking of late (Presence), or else its latest,
// leaves the queue, and may queue again when there is room. First come,
// first queued, let whoever filled the queue at a phase's opening, from
// MaxQueue/MaxPerNetwork networks, keep everyone else out of the draw,
// taking each place freed before an honest program asked again (audit
// H3): now a full queue holds as many networks as it can. s.mu is held.
func (s *Server) makeRoom(network string, now time.Time) bool {
	counts := map[string]int{}
	for _, w := range s.queue {
		counts[w.p.network]++
	}
	most := 0
	for _, n := range counts {
		most = max(most, n)
	}
	if counts[network]+1 >= most {
		return false
	}
	leaving := -1
	for i := len(s.queue) - 1; i >= 0; i-- {
		if w := s.queue[i]; counts[w.p.network] == most {
			if leaving < 0 || now.Sub(w.seen) > s.cfg.Presence && now.Sub(s.queue[leaving].seen) <= s.cfg.Presence {
				leaving = i
			}
		}
	}
	w := s.queue[leaving]
	s.cfg.Log.Printf("the queue is full: %s (from %s, network %s, %d of it queued) leaves it, for one from a network with fewer",
		w.p.name, w.p.addr, w.p.network, most)
	s.queue = slices.Delete(s.queue, leaving, leaving+1)
	return true
}

// networkRoom reports whether one more participant from network may be
// queued. s.mu is held.
func (s *Server) networkRoom(network string) bool {
	if s.cfg.NoAddressLimit {
		return true
	}
	n := 0
	for _, w := range s.queue {
		if w.p.network == network {
			n++
		}
	}
	return n < s.cfg.MaxPerNetwork
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
// there from now on: handleTurn has checked it has room (audit R-015, L2).
// s.mu is held.
func (s *Server) seen(p participant, now time.Time) {
	for i := range s.queue {
		if s.queue[i].p.identity == p.identity {
			s.queue[i].seen = now
			s.queue[i].p.addr, s.queue[i].p.network, s.queue[i].p.ip = p.addr, p.network, p.ip
			return
		}
	}
	s.queue = append(s.queue, waiter{p: p, seen: now})
	s.cfg.Log.Printf("%s joined the queue, at %d (from %s, network %s)", p.name, len(s.queue), p.addr, p.network)
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

// holder returns the turn when p holds it, and otherwise answers 409; from
// a network denied, it answers 403 (refuseDenied). The turn keeps the
// address it was given to (turn.p.addr, audit L1), and takes p's own as
// Deny matches it. s.mu is held.
func (s *Server) holder(w http.ResponseWriter, p participant) *turn {
	s.advance(s.cfg.now())
	switch {
	case s.refuseDenied(w, p):
		return nil
	case !s.holds(p):
		s.fail(w, http.StatusConflict, "it is not your turn")
		return nil
	}
	s.turn.p.ip = p.ip
	return s.turn
}

// refuseDenied answers 403 to p when p asks from a network denied,
// dropping p's place in the queue and ending a turn p holds whose upload
// is not coming in (as Deny does), and reports whether it did. s.mu is
// held.
func (s *Server) refuseDenied(w http.ResponseWriter, p participant) bool {
	if !s.denied(p.ip) {
		return false
	}
	if p.identity != "" {
		s.queue = slices.DeleteFunc(s.queue, func(q waiter) bool { return q.p.identity == p.identity })
		if s.holds(p) && !s.turn.uploading {
			s.endTurn("turn ended: its network is denied")
			s.advance(s.cfg.now())
		}
	}
	s.fail(w, http.StatusForbidden, "this phase takes no part from your network: its organizers have denied it")
	return true
}

// handleState sends the turn holder the latest state.
func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	// Only a download counts as one: a HEAD request, which the GET route
	// also takes, sends nothing (audit M11).
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		s.fail(w, http.StatusMethodNotAllowed, "the state is downloaded with GET")
		return
	}
	p, ok := s.participant(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	t := s.holder(w, p)
	latest := s.latest
	var deadline time.Time
	if t != nil {
		// UploadWithin runs from the first download, which no later one, or
		// one stalled, restarts (audit M11).
		if t.downloaded.IsZero() {
			t.downloaded = s.cfg.now()
		}
		deadline = s.uploadBy(t, s.cfg.now())
	}
	s.mu.Unlock()
	if t == nil {
		return
	}
	w.Header().Set("X-Ceremony-Index", strconv.FormatUint(uint64(latest.index), 10))
	w.Header().Set("X-Ceremony-Sha256", hex.EncodeToString(latest.hash[:]))
	if err := s.sendFile(w, s.path(latest.index), deadline); err != nil {
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
	switch {
	case t != nil && t.uploading:
		s.fail(w, http.StatusConflict, "your upload is already coming in")
		t = nil
	case t != nil && (s.addressDone(p.addr) || s.addressDone(t.p.addr)):
		// Both the address the turn was given to and the one uploading
		// count (audit L1): checked on one and charged to the other, a
		// host with two addresses was never counted.
		s.fail(w, http.StatusForbidden, fmt.Sprintf(
			"%d contributions have come from your network address: this phase takes no more from it", s.cfg.MaxContributionsPerAddress))
		s.endTurn("upload refused: its address has made its contributions")
		s.advance(s.cfg.now())
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
				s.endTurn("upload made no contribution")
			}
			s.advance(s.cfg.now())
			s.mu.Unlock()
		}
	}()

	tmp, hash, err := s.receive(w, r, t.deadline, prev.size)
	answerWithin(w, time.Minute) // the upload took as long as it took; the answer gets its own time
	if err != nil {
		s.cfg.Log.Printf("%s's upload failed: %v", p.name, err)
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			s.fail(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("a contribution is at most %d bytes", tooLarge.Limit))
		} else {
			// What failed, and where, is for the coordinator's log (audit R-040).
			s.fail(w, http.StatusBadRequest, "upload failed: it did not arrive whole by your turn's deadline, or could not be stored")
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
	answerWithin(w, time.Minute)
	if err != nil {
		// The reason holds what the upload holds, such as its circuits'
		// names: quoted in the log (audit R-042), and told to the uploader
		// without the coordinator's file names (audit R-040).
		s.cfg.Log.Printf("%s's contribution was refused: %q", p.name, err.Error())
		s.fail(w, http.StatusUnprocessableEntity, "contribution refused: "+refusal(err, tmp, s.path(prev.index)))
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.lock == nil: // Released: another coordinator may have the directory now
		s.fail(w, http.StatusServiceUnavailable, "the coordinator is stopping; your contribution was not saved")
		return
	case s.turn != t || s.latest != prev:
		// Nothing ends a turn whose upload is coming in; were one to, its
		// contribution would no longer be on top of the latest, or would
		// take another's turn with it.
		s.cfg.Log.Printf("%s's upload refused: the turn, or the latest state, changed while it came in", p.name)
		s.fail(w, http.StatusConflict, "your turn ended before your contribution was in")
		return
	case s.denied(p.ip): // denied while it came in (Deny)
		s.endTurn("upload refused: its network is denied")
		s.fail(w, http.StatusForbidden, "this phase takes no part from your network: its organizers have denied it")
		return
	}
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
		Identity:    string(p.identity),
		SHA256:      hex.EncodeToString(hash[:]),
		Prev:        hex.EncodeToString(prev.hash[:]),
		At:          now.UTC(),
	}
	if err := s.commit(tmp, e); err != nil {
		s.cfg.Log.Printf("saving %s's contribution: %v", p.name, err)
		s.fail(w, http.StatusInternalServerError, "the coordinator could not save your contribution")
		return
	}
	accepted = true
	s.contributedFrom[t.p.addr]++
	if p.addr != t.p.addr {
		s.contributedFrom[p.addr]++
	}
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
	s.cfg.Log.Printf("%s's contribution is state %d, sha256 %s (from %s)", p.name, e.Index, e.SHA256, p.addr)
	s.advance(now)
	s.reply(w, Reply{Status: Done, Index: e.Index, SHA256: e.SHA256})
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

// Append adds the contribution in, made by hand on top of the phase's
// latest state, as its next state, under name: the rescue for a
// contributor lined up who got no turn (ADR 0008), while no coordinator
// serves the directory, whose lock New holds. It checks in as an upload is
// checked, and commits it as one, entry and then state; in itself is left
// as it is. Its entry's identity is hand:<the first 16 hex digits of its
// sha256>.
func (s *Server) Append(in, name string) (transcript.Entry, error) {
	name, err := cleanName(name)
	if err != nil {
		return transcript.Entry{}, fmt.Errorf("the contributor's name: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.lock == nil:
		return transcript.Entry{}, errors.New("the coordinator has released the directory")
	case s.serving:
		return transcript.Entry{}, errors.New("the coordinator is serving the phase: stop it first")
	}
	prev := s.latest
	tmp, hash, err := copyIn(in, filepath.Join(s.cfg.Dir, "incoming"))
	if err != nil {
		return transcript.Entry{}, err
	}
	defer func() {
		// Gone already when commit made it a state.
		if err := os.Remove(tmp); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.cfg.Log.Printf("removing %s: %v", tmp, err)
		}
	}()
	if err := ceremony.Check(s.path(prev.index), tmp); err != nil {
		return transcript.Entry{}, fmt.Errorf("%s is no contribution on top of state %d, the latest: %w", in, prev.index, err)
	}
	sum := hex.EncodeToString(hash[:])
	e := transcript.Entry{
		Index:       prev.index + 1,
		Participant: name,
		Identity:    "hand:" + sum[:16],
		SHA256:      sum,
		Prev:        hex.EncodeToString(prev.hash[:]),
		At:          s.cfg.now().UTC(),
	}
	if err := s.commit(tmp, e); err != nil {
		return transcript.Entry{}, err
	}
	s.done[identity(e.Identity)] = len(s.entries)
	s.entries = append(s.entries, e)
	s.latest = state{index: e.Index, hash: hash, size: prev.size}
	if fi, err := os.Stat(s.path(e.Index)); err == nil {
		s.latest.size = fi.Size()
	}
	s.cfg.Log.Printf("%s's contribution, added by hand, is state %d, sha256 %s", name, e.Index, e.SHA256)
	return e, nil
}

// copyIn copies the file at path to a new file in dir, synced, and returns
// the new file and its sha256.
func copyIn(path, dir string) (string, [32]byte, error) {
	src, err := os.Open(path)
	if err != nil {
		return "", [32]byte{}, err
	}
	defer src.Close()
	f, err := os.CreateTemp(dir, "by-hand-")
	if err != nil {
		return "", [32]byte{}, err
	}
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(f, h), src)
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
// Each is on disk, synced with the directory naming it, before the next step,
// and the state before the participant is told it is in (audit R-014): a
// crash could otherwise lose a contribution its participant has published.
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
	if err == nil {
		err = entry.Sync()
	}
	if cerr := entry.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(entry.Name(), s.path(e.Index)+".json")
	}
	if err == nil {
		err = syncDir(s.cfg.Dir)
	}
	if err != nil {
		return errors.Join(err, os.Remove(entry.Name()))
	}
	if err := os.Rename(tmp, s.path(e.Index)); err != nil {
		return err
	}
	// The state is in place, and the transcript holds it from here on: a
	// failure to sync now is a warning, not a refusal.
	if err := syncDir(s.cfg.Dir); err != nil {
		s.cfg.Log.Printf("WARNING: syncing %s after state %d: %v: the state may not survive a crash", s.cfg.Dir, e.Index, err)
	}
	return nil
}

// refusal is what an uploader is told of why their contribution was refused:
// ceremony's reason, without the coordinator's file names, and not too long
// to show (audit R-040).
func refusal(err error, paths ...string) string {
	msg := err.Error()
	for _, p := range paths {
		msg = strings.ReplaceAll(msg, p+": ", "")
		msg = strings.ReplaceAll(msg, p, "the file")
	}
	if r := []rune(msg); len(r) > 300 {
		msg = string(r[:300]) + "…"
	}
	return msg
}

// answerWithin gives the answer to a request that ran long (an upload, then
// its check) d to be written, whatever the server's WriteTimeout.
func answerWithin(w http.ResponseWriter, d time.Duration) {
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(d)) // not supported: no deadline to move
}

// Phase1 is what a Phase 2's SRS says it was sealed from (Status.Phase1).
type Phase1 struct {
	Head          string    `json:"head"`
	Contributions uint32    `json:"contributions"`
	DrandRound    uint64    `json:"drand_round,omitzero"` // 0: a rehearsal's beacon, not from drand
	CloseAt       time.Time `json:"close_at,omitzero"`
}

// handleStatus answers with the phase at a glance.
func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	s.reply(w, s.status())
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
		Wasted:        s.wasted,
		Closed:        s.closed || s.full(),
		CloseAt:       s.cfg.CloseAt,
		Notice:        s.cfg.Notice,
		ReleaseURL:    s.cfg.ReleaseURL,
		Version:       Version(),
	}
	if s.start.Kind == ceremony.KindPhase2 {
		st.SRS = hex.EncodeToString(s.start.SRS[:])
		for _, h := range s.start.R1CS {
			st.R1CS = append(st.R1CS, hex.EncodeToString(h[:]))
		}
		if p := s.cfg.Phase1; p != nil {
			st.Phase1 = &Phase1{Head: hex.EncodeToString(p.Head[:]), Contributions: p.Contributions, DrandRound: p.Beacon.DrandRound, CloseAt: p.Beacon.CloseAt}
		}
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
	s.reply(w, t)
}

// publicDownloads bounds the public downloads of states (GET /v1/states/{i})
// at once, from everywhere; a network address has one at a time (audit R-010).
const publicDownloads = 16

// A public download sends a state sendChunk at a time, each within
// sendChunkTime: a reader slower than about 34 KiB/s loses the download, and
// one at a fair pace takes as long as the state needs. A flat deadline let
// one slow reader hold a download slot for 30 minutes (audit R-010).
const (
	sendChunk     = 1 << 20
	sendChunkTime = 30 * time.Second
)

// handleStates sends state i's file, a few downloads at a time and one per
// network address.
func (s *Server) handleStates(w http.ResponseWriter, r *http.Request) {
	i, err := strconv.ParseUint(r.PathValue("index"), 10, 32)
	addr := s.address(r)
	s.mu.Lock()
	latest := s.latest.index
	found := err == nil && i <= uint64(latest)
	busy := s.downloading[addr] || len(s.downloading) >= publicDownloads
	if found && !busy {
		s.downloading[addr] = true
	}
	s.mu.Unlock()
	switch {
	case !found:
		s.fail(w, http.StatusNotFound, fmt.Sprintf("states run from 0 to %d", latest))
		return
	case busy:
		w.Header().Set("Retry-After", "30")
		s.fail(w, http.StatusServiceUnavailable, "too many downloads at once, or one from your network address already; try again shortly")
		return
	}
	defer func() {
		s.mu.Lock()
		delete(s.downloading, addr)
		s.mu.Unlock()
	}()
	if err := s.sendFile(w, s.path(uint32(i)), time.Time{}); err != nil {
		s.cfg.Log.Printf("sending state %d: %v", i, err)
	}
}

// sendFile writes the file at path as the response body: all of it by
// deadline, or with none, each sendChunk within sendChunkTime.
func (s *Server) sendFile(w http.ResponseWriter, path string, deadline time.Time) error {
	f, err := os.Open(path)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, "state unavailable")
		return err
	}
	defer f.Close()
	if fi, err := f.Stat(); err == nil {
		w.Header().Set("Content-Length", strconv.FormatInt(fi.Size(), 10))
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	rc := http.NewResponseController(w)
	within := func(t time.Time) error {
		if err := rc.SetWriteDeadline(t); err != nil && !errors.Is(err, http.ErrNotSupported) {
			return err
		}
		return nil
	}
	if !deadline.IsZero() {
		if err := within(deadline); err != nil {
			return err
		}
		_, err = io.Copy(w, f)
		return err
	}
	buf := make([]byte, sendChunk)
	for {
		n, rerr := io.ReadFull(f, buf)
		if n > 0 {
			if err := within(time.Now().Add(sendChunkTime)); err != nil {
				return err
			}
			if _, err := w.Write(buf[:n]); err != nil {
				return err
			}
		}
		switch rerr {
		case nil:
		case io.EOF, io.ErrUnexpectedEOF:
			return nil
		default:
			return rerr
		}
	}
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
func (s *Server) reply(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		s.cfg.Log.Printf("writing a reply: %v", err) // the coordinator's log, as all else (audit R-044)
	}
}

// fail answers with an error status and {"error": msg}. msg is for whoever
// asked: what went wrong on the coordinator's side stays in its log (audit R-040).
func (s *Server) fail(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": msg}); err != nil {
		s.cfg.Log.Printf("writing an error: %v", err)
	}
}
