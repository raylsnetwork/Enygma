// Package participant takes a participant through a coordinated phase from
// their own machine, with nothing for them to do but start it:
//
//  1. it asks the coordinator how to join, and joins: registers itself, or
//     in a GitHub phase signs in on GitHub's device flow first;
//  2. it queues, and waits for its turn;
//  3. on its turn it downloads the latest state, contributes the machine's
//     randomness (ceremony.Contribute, which draws it from the operating
//     system and keeps no copy), and uploads the result.
//
// It rides out what a long wait brings: a coordinator restarted (it joins
// again), unreachable for a while (it retries), or full for its network
// address (it waits for a place). The randomness never leaves the process;
// what leaves is the contribution, which is public.
package participant

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"enygma_ceremony/ceremony"
	"enygma_ceremony/coordinator"
	"enygma_ceremony/transcript"
)

var (
	// ErrClosed is a phase that closed before the participant's turn.
	ErrClosed = errors.New("the phase is closed: it takes no more contributions")
	// ErrUnauthorized is a token the coordinator does not know.
	ErrUnauthorized = errors.New("the coordinator does not know this participant")
	// errTurnLost is a turn that ended before the upload was in.
	errTurnLost = errors.New("turn lost")
)

// minPoll is the least time between asks for a turn, whatever the
// coordinator says, so a misconfigured one is not asked in a tight loop.
const minPoll = 10 * time.Millisecond

// The steps of a run (Event.Stage).
const (
	StageSignIn       = "signin"       // sign in with GitHub: open Event.CodeURL, enter Event.Code
	StageJoined       = "joined"       // admitted, as Event.Name
	StageFull         = "full"         // waiting for a place: Event.Message says why
	StageQueued       = "queued"       // queued at Event.Position, about Event.Wait seconds from the turn
	StageTurn         = "turn"         // the turn, on top of state Event.Index, until Event.Deadline
	StageDownloading  = "downloading"  // downloading the latest state: Event.Done of Event.Total bytes
	StageContributing = "contributing" // adding this machine's randomness: Event.Fraction done (see Event.Part)
	StageUploading    = "uploading"    // uploading: Event.Done of Event.Total bytes sent; once all are, the coordinator checks them
	StageDone         = "done"         // contribution Event.Index, sha256 Event.SHA256, is in the transcript
)

// Event is one step of a run.
type Event struct {
	Stage    string    `json:"stage"`
	Name     string    `json:"name,omitzero"`
	Message  string    `json:"message,omitzero"`
	Position int       `json:"position,omitzero"`
	Wait     float64   `json:"wait,omitzero"`
	Code     string    `json:"code,omitzero"`
	CodeURL  string    `json:"code_url,omitzero"`
	Index    uint32    `json:"index,omitzero"`
	Deadline time.Time `json:"deadline,omitzero"`
	SHA256   string    `json:"sha256,omitzero"`
	Already  bool      `json:"already,omitzero"` // StageDone: contributed before this run
	// Done and Total are bytes: downloaded (StageDownloading), read or
	// written (StageContributing), or sent (StageUploading).
	Done  int64 `json:"done,omitzero"`
	Total int64 `json:"total,omitzero"`
	// Part is the part of the contribution under way (ceremony.PartReading,
	// PartRandomizing or PartWriting), and Fraction how much of the whole
	// contribution is done. While randomizing, which reports no progress of
	// its own, the contribution should reach Until in about Estimate seconds.
	Part     string  `json:"part,omitzero"`
	Fraction float64 `json:"fraction,omitzero"`
	Until    float64 `json:"until,omitzero"`
	Estimate float64 `json:"estimate,omitzero"`
	// Session is the token the coordinator knows this participant by
	// (StageTurn), for a program sending heartbeats itself.
	Session string `json:"session,omitzero"`
}

// event reports e, when someone listens.
func (c *Client) event(e Event) {
	if c.Events != nil {
		c.Events(e)
	}
}

// Client is one participant's side of a phase.
type Client struct {
	URL  string // the coordinator, e.g. https://ceremony.example.org
	Name string // the name the transcript shows; anonymous when empty (a GitHub phase shows the login)
	HTTP *http.Client
	// Out receives progress for the participant to read.
	Out io.Writer
	// Patience is how long to keep retrying while the coordinator cannot
	// be reached; 10 minutes when zero.
	Patience time.Duration
	// GitHub is GitHub's site, for signing in (https://github.com when empty).
	GitHub string
	// AskName, when set, asks the participant for Name if an open phase
	// would otherwise show them as anonymous.
	AskName func() string
	// Events, when set, receives each step of the run, for a program that
	// shows progress its own way.
	Events func(Event)
	// Heartbeat is how often to tell the coordinator, during the turn, that
	// this participant is still at it; 10 seconds when zero. Negative sends
	// none, for a program that sends them itself.
	Heartbeat time.Duration
	// Contribute adds this machine's randomness to a state, as
	// ceremony.ContributeWithProgress does, which it is when nil. The
	// ceremony program runs it in a short-lived child process instead, so
	// that the randomness never lives in the process that keeps running.
	Contribute func(in io.Reader, size int64, out io.Writer, report func(ceremony.Progress)) (ceremony.Receipt, error)
	// Mine is the sha256, in hex, of a contribution this participant made in
	// an earlier run. The coordinator's word that this participant has
	// contributed already is taken only for a contribution this run made, or
	// for Mine: a coordinator could otherwise hand them someone else's.
	Mine string
	// MaxState is the most to download as one state; ceremony.DefaultMaxState
	// when zero. A coordinator sending more is refused before it is decoded.
	MaxState int64

	cfg         coordinator.ClientConfig
	made        []ceremony.Receipt // every contribution this run made, uploaded or not
	session     string             // the token the coordinator knows this participant by
	githubToken string             // a GitHub phase: kept in memory, to join again after a coordinator restart
}

// Result is the participant's contribution, as the coordinator recorded it.
type Result struct {
	Index   uint32   // its place in the transcript
	SHA256  [32]byte // the sha256 the participant publishes
	Prev    [32]byte // the state it built on; zero when Already
	Already bool     // contributed before: in an earlier run, or before a restart
}

// Run joins, waits, contributes and uploads, and returns the contribution.
// It returns early with ErrClosed if the phase closes first.
func (c *Client) Run(ctx context.Context) (Result, error) {
	if c.HTTP == nil {
		c.HTTP = HTTPClient()
	}
	if c.Out == nil {
		c.Out = io.Discard
	}
	if c.Patience == 0 {
		c.Patience = 10 * time.Minute
	}
	if c.GitHub == "" {
		c.GitHub = "https://github.com"
	}
	if err := c.checkURL(); err != nil {
		return Result{}, err
	}
	if err := c.patiently(ctx, func() error { return c.call(ctx, http.MethodGet, "/v1/config", nil, &c.cfg) }); err != nil {
		return Result{}, err
	}
	if c.cfg.Notice != "" {
		fmt.Fprintf(c.Out, "Notice from the coordinator: %s\n", printable(c.cfg.Notice))
	}
	if c.Heartbeat == 0 && c.cfg.Heartbeat > 0 {
		// As often as the coordinator asks, within bounds (audit R-059).
		c.Heartbeat = min(max(time.Duration(c.cfg.Heartbeat*float64(time.Second)), 10*time.Millisecond), time.Minute)
	}
	if err := c.join(ctx); err != nil {
		return Result{}, err
	}
	// rejoined counts the times the coordinator forgot this participant
	// since it last queued them, so one that forgets them at once each time
	// is given up on, rather than joined again for ever.
	position, full, rejoined := 0, false, 0
	rejoin := func(why string) error {
		fmt.Fprintf(c.Out, "%s; joining again.\n", why)
		position, rejoined = 0, rejoined+1
		return c.join(ctx)
	}
	for {
		var r coordinator.Reply
		err := c.patiently(ctx, func() error { return c.call(ctx, http.MethodPost, "/v1/turn", nil, &r) })
		var h *httpError
		switch {
		case errors.Is(err, ErrUnauthorized) && rejoined < 3:
			// The coordinator restarted, or forgot a registration unused for long.
			if err := rejoin("The coordinator no longer knows this session"); err != nil {
				return Result{}, err
			}
			continue
		case errors.As(err, &h) && h.code == http.StatusTooManyRequests:
			if !full {
				fmt.Fprintf(c.Out, "Waiting for a place in the queue: %s\n", h.msg)
				c.event(Event{Stage: StageFull, Message: h.msg})
				full = true
			}
			if err := sleep(ctx, h.retryAfter); err != nil {
				return Result{}, err
			}
			continue
		case err != nil:
			return Result{}, err
		}
		full = false
		switch r.Status {
		case coordinator.Done:
			return c.done(ctx, r)
		case coordinator.Closed:
			if r.Notice != "" {
				return Result{}, fmt.Errorf("%w (%s)", ErrClosed, printable(r.Notice))
			}
			return Result{}, ErrClosed
		case coordinator.Queued:
			rejoined = 0
			if r.Position != position {
				position = r.Position
				c.waiting(r)
			}
			if err := sleep(ctx, pollFor(r.Poll)); err != nil {
				return Result{}, err
			}
		case coordinator.Go:
			res, err := c.contribute(ctx, r)
			switch {
			case errors.Is(err, errTurnLost):
				fmt.Fprintln(c.Out, "Your turn ended before your contribution was in; queueing again.")
				position = 0
				continue
			case errors.Is(err, ErrUnauthorized) && rejoined < 3:
				// The coordinator restarted during the turn, forgetting the
				// turn and the contribution under way: queue for another.
				if err := rejoin("The coordinator restarted during your turn"); err != nil {
					return Result{}, err
				}
				continue
			case err != nil && unreachable(err):
				// Still this participant's turn until its deadline, or the
				// upload went in: asking again says which.
				fmt.Fprintf(c.Out, "Lost the coordinator during your turn (%v); asking again.\n", err)
				if err := sleep(ctx, 5*time.Second); err != nil {
					return Result{}, err
				}
				continue
			}
			return res, err
		default:
			return Result{}, fmt.Errorf("the coordinator answered %q", r.Status)
		}
	}
}

// checkURL makes c.URL a coordinator's address: https:// when it names no
// scheme, as a participant may type it. An address that is no web address
// fails at once, rather than as a coordinator unreachable for c.Patience;
// so does plain http, but to this machine (audit R-055): over it, anyone in
// between could read the session and GitHub tokens, and change what the
// coordinator says.
func (c *Client) checkURL() error {
	if c.URL != "" && !strings.Contains(c.URL, "://") {
		c.URL = "https://" + c.URL
	}
	u, err := url.Parse(c.URL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return fmt.Errorf("%q is not a coordinator's address, such as https://ceremony.example.org", c.URL)
	}
	if u.Scheme == "http" && !loopback(u.Hostname()) {
		return fmt.Errorf("%q is plain http: a coordinator is reached over https (plain http only for one on this machine)", c.URL)
	}
	return nil
}

// loopback reports whether host is this machine.
func loopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// ReplyTimeout is how long a coordinator may take to start answering a
// request: it checks an upload before it answers, which takes minutes for
// the largest states.
const ReplyTimeout = 5 * time.Minute

// HTTPClient is the HTTP client for a coordinator, as the ceremony program
// uses it:
//   - it follows no redirect (audit R-056): the protocol has none, and one
//     could carry the session or GitHub token elsewhere, or to plain http;
//   - it gives up on a reply whose headers do not come within ReplyTimeout
//     (R-057), so a coordinator that takes a connection and says nothing
//     cannot hang the program. A body, such as a state, takes as long as it
//     takes.
func HTTPClient() *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.ResponseHeaderTimeout = ReplyTimeout
	return &http.Client{
		Transport: t,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			return fmt.Errorf("%w to %s", errRedirected, req.URL.Redacted())
		},
	}
}

// errRedirected is a coordinator redirecting a request: not something trying
// again gets past.
var errRedirected = errors.New("a coordinator never redirects, and this one redirected")

// callTimeout bounds one of the small JSON exchanges with a coordinator.
const callTimeout = 30 * time.Second

// maxPoll and maxRetryAfter bound how long a coordinator can tell this
// program to wait before asking again (audit R-054).
const (
	maxPoll       = 5 * time.Minute
	maxRetryAfter = 10 * time.Minute
)

// pollFor is how long to wait before asking again, as a coordinator asks:
// seconds, within minPoll and maxPoll.
func pollFor(seconds float64) time.Duration {
	if !(seconds > 0) { // also NaN
		return minPoll
	}
	if seconds >= maxPoll.Seconds() {
		return maxPoll
	}
	return max(time.Duration(seconds*float64(time.Second)), minPoll)
}

// waiting tells the participant where they are in the queue.
func (c *Client) waiting(r coordinator.Reply) {
	ahead := "you are next"
	if r.Position > 1 {
		ahead = fmt.Sprintf("%d ahead of you", r.Position-1)
	}
	if r.Wait >= 60 && r.Wait <= 30*24*3600 { // an estimate past a month says nothing, and would overflow
		ahead += fmt.Sprintf(", about %s", (time.Duration(r.Wait) * time.Second).Round(time.Minute))
	}
	fmt.Fprintf(c.Out, "Waiting for your turn: %s. Leave this running.\n", ahead)
	c.event(Event{Stage: StageQueued, Position: r.Position, Wait: r.Wait})
}

// join gets the session token the coordinator will know this participant by.
func (c *Client) join(ctx context.Context) error {
	switch c.cfg.Auth {
	case coordinator.AuthOpen:
		if c.Name == "" && c.AskName != nil {
			c.Name = c.AskName()
		}
		return c.register(ctx, coordinator.Registration{Name: c.Name})
	case coordinator.AuthGitHub:
		if c.githubToken == "" {
			token, err := c.githubSignIn(ctx, c.cfg.GitHubClientID)
			if err != nil {
				return err
			}
			c.githubToken = token
		}
		return c.register(ctx, coordinator.Registration{GitHubToken: c.githubToken})
	}
	return fmt.Errorf("the coordinator admits participants by %q, which this program does not know: is it out of date?", c.cfg.Auth)
}

// register registers with the coordinator, waiting while it takes no more
// registrations from this network address.
func (c *Client) register(ctx context.Context, reg coordinator.Registration) error {
	body, err := json.Marshal(reg)
	if err != nil {
		return err
	}
	for {
		var s coordinator.Session
		err := c.patiently(ctx, func() error {
			return c.call(ctx, http.MethodPost, "/v1/register", bytes.NewReader(body), &s)
		})
		var h *httpError
		if errors.As(err, &h) && h.code == http.StatusTooManyRequests {
			fmt.Fprintf(c.Out, "The coordinator takes no more registrations from your network address for now (%s); waiting.\n", h.msg)
			if err := sleep(ctx, h.retryAfter); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("joining: %w", err)
		}
		c.session = s.Token
		name := printable(s.Participant)
		fmt.Fprintf(c.Out, "Joined as %s.\n", name)
		c.event(Event{Stage: StageJoined, Name: name})
		return nil
	}
}

// patiently runs try until it succeeds, fails in a way trying again cannot
// fix, or the coordinator has been unreachable for longer than c.Patience.
func (c *Client) patiently(ctx context.Context, try func() error) error {
	var failingSince time.Time
	for {
		err := try()
		if err == nil || !unreachable(err) {
			return err
		}
		if failingSince.IsZero() {
			failingSince = time.Now()
		}
		if time.Since(failingSince) > c.Patience {
			return fmt.Errorf("the coordinator has not answered for %s: %w", c.Patience, err)
		}
		fmt.Fprintf(c.Out, "Cannot reach the coordinator (%v); trying again.\n", err)
		if err := sleep(ctx, 5*time.Second); err != nil {
			return err
		}
	}
}

// contribute takes a turn: downloads the state, contributes, uploads, all
// in memory, telling the coordinator throughout that it is still at it.
func (c *Client) contribute(ctx context.Context, turn coordinator.Reply) (Result, error) {
	fmt.Fprintf(c.Out, "Your turn: contributing on top of state %d.\n", turn.Index)
	c.event(Event{Stage: StageTurn, Index: turn.Index, Deadline: turn.Deadline, Session: c.session})
	want, err := parseHash(turn.SHA256)
	if err != nil {
		return Result{}, err
	}
	defer c.keepTurn(ctx)()

	fmt.Fprintln(c.Out, "Downloading the latest state.")
	c.event(Event{Stage: StageDownloading})
	state, err := c.download(ctx, want)
	if err != nil {
		return Result{}, err
	}
	fmt.Fprintln(c.Out, "Contributing. Your randomness comes from this machine's operating system and is gone when this exits.")
	c.event(Event{Stage: StageContributing})
	var next bytes.Buffer
	contributeTo := c.Contribute
	if contributeTo == nil {
		contributeTo = ceremony.ContributeWithProgress
	}
	receipt, err := contributeTo(bytes.NewReader(state), int64(len(state)), &next, func(p ceremony.Progress) {
		if p.Part == ceremony.PartRandomizing {
			fmt.Fprintf(c.Out, "Adding your randomness: about %s on this machine.\n", p.Estimate.Round(time.Second))
		}
		c.event(Event{
			Stage: StageContributing, Part: p.Part, Done: p.Done, Total: p.Total,
			Fraction: p.Fraction, Until: p.Until, Estimate: p.Estimate.Seconds(),
		})
	})
	if err != nil {
		return Result{}, err
	}
	// Kept before the upload: if its answer is lost, the coordinator's later
	// "done" is checked against this, not taken on its word.
	c.made = append(c.made, receipt)
	state = nil // the next state is all an upload needs
	fmt.Fprintln(c.Out, "Uploading; the coordinator checks your contribution before answering.")
	r, err := c.upload(ctx, next.Bytes())
	if err != nil {
		return Result{}, err
	}
	got, err := parseHash(r.SHA256)
	if err != nil {
		return Result{}, err
	}
	if got != receipt.Hash || r.Index != receipt.Index {
		return Result{}, fmt.Errorf("the coordinator recorded contribution %d sha256 %x, but yours is %d sha256 %x",
			r.Index, got, receipt.Index, receipt.Hash)
	}
	if err := c.confirm(ctx, receipt.Index, receipt.Hash, &receipt.In); err != nil {
		return Result{}, err
	}
	c.event(Event{Stage: StageDone, Index: r.Index, SHA256: r.SHA256})
	return Result{Index: r.Index, SHA256: got, Prev: receipt.In}, nil
}

// done answers a coordinator saying this participant has contributed
// already. Its word is taken only for a contribution this run made, or the
// one the participant says they made in an earlier run (Client.Mine), and
// only once its transcript holds that contribution: otherwise a coordinator
// could hand a participant a contribution of its own as their receipt, and
// leave theirs out.
func (c *Client) done(ctx context.Context, r coordinator.Reply) (Result, error) {
	h, err := parseHash(r.SHA256)
	if err != nil {
		return Result{}, err
	}
	for _, m := range c.made {
		if m.Hash == h && m.Index == r.Index {
			if err := c.confirm(ctx, m.Index, m.Hash, &m.In); err != nil {
				return Result{}, err
			}
			c.event(Event{Stage: StageDone, Index: r.Index, SHA256: r.SHA256})
			return Result{Index: r.Index, SHA256: h, Prev: m.In}, nil
		}
	}
	if c.Mine != "" {
		mine, err := parseHash(c.Mine)
		if err != nil {
			return Result{}, fmt.Errorf("%q is not a sha256 of a contribution", c.Mine)
		}
		if mine == h {
			if err := c.confirm(ctx, r.Index, h, nil); err != nil {
				return Result{}, err
			}
			c.event(Event{Stage: StageDone, Index: r.Index, SHA256: r.SHA256, Already: true})
			return Result{Index: r.Index, SHA256: h, Already: true}, nil
		}
	}
	return Result{}, fmt.Errorf("the coordinator says you have contributed already, as contribution %d with sha256 %x, "+
		"but this program made no such contribution: if you contributed in an earlier run, "+
		"give the sha256 it printed (ceremony join -mine <sha256>), and do not publish this one", r.Index, h)
}

// confirm checks that the coordinator's transcript holds contribution index
// with sha256 hash, built on prev when prev is not nil.
func (c *Client) confirm(ctx context.Context, index uint32, hash [32]byte, prev *[32]byte) error {
	req, err := c.request(ctx, http.MethodGet, "/v1/transcript", nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("checking the transcript holds your contribution: %w", err)
	}
	defer resp.Body.Close()
	if err := statusError(resp); err != nil {
		return fmt.Errorf("checking the transcript holds your contribution: %w", err)
	}
	var t transcript.Transcript
	if err := json.NewDecoder(io.LimitReader(resp.Body, transcript.MaxJSON)).Decode(&t); err != nil {
		return fmt.Errorf("checking the transcript holds your contribution: %w", err)
	}
	want := hex.EncodeToString(hash[:])
	for _, e := range t.Contributions {
		if e.Index != index {
			continue
		}
		if !strings.EqualFold(e.SHA256, want) || (prev != nil && !strings.EqualFold(e.Prev, hex.EncodeToString(prev[:]))) {
			break
		}
		return nil
	}
	return fmt.Errorf("the coordinator's transcript does not hold your contribution %d with sha256 %s", index, want)
}

// printable is s, from the coordinator, with no control or formatting
// characters, which a terminal could take as commands (escape sequences, or
// text reordered to show something else), and no longer than a line or two.
func printable(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, s)
	if r := []rune(s); len(r) > 300 {
		s = string(r[:300]) + "…"
	}
	return s
}

// keepTurn tells the coordinator every c.Heartbeat, until the returned stop
// is called, that this participant is still taking its turn, so that a turn
// whose holder has gone (a laptop closed) passes on long before its deadline.
func (c *Client) keepTurn(ctx context.Context) (stop func()) {
	every := c.Heartbeat
	if every == 0 {
		every = 10 * time.Second
	}
	if every < 0 {
		return func() {}
	}
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				// Not /v1/turn: asking for a turn queues a participant whose
				// turn has passed on, who would then be handed a new one
				// while this one's contribution is still under way.
				_ = c.call(ctx, http.MethodPost, "/v1/heartbeat", nil, nil) // a missed heartbeat is retried at the next tick
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

// upload sends the contribution, reporting the bytes sent about every 1%,
// and returns the coordinator's answer once it has checked it.
func (c *Client) upload(ctx context.Context, data []byte) (coordinator.Reply, error) {
	total := int64(len(data))
	next := int64(0)
	sent := func(n int64) {
		if n >= next || n >= total {
			c.event(Event{Stage: StageUploading, Done: min(n, total), Total: total})
			next = n + max(total/100, 1)
		}
	}
	sent(0)
	target := strings.TrimSuffix(c.URL, "/") + "/v1/contribution"
	body := &uploadProgress{r: bytes.NewReader(data), sent: sent}
	req, err := c.request(ctx, http.MethodPut, "/v1/contribution", body)
	if err != nil {
		return coordinator.Reply{}, err
	}
	req.ContentLength = total
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return coordinator.Reply{}, err
	}
	defer resp.Body.Close()
	code, retryAfter := resp.StatusCode, resp.Header.Get("Retry-After")
	reply, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return coordinator.Reply{}, &url.Error{Op: http.MethodPut, URL: target, Err: err}
	}
	if err := replyError(code, reply, retryAfter); err != nil {
		return coordinator.Reply{}, err
	}
	var r coordinator.Reply
	return r, json.Unmarshal(reply, &r)
}

// uploadProgress counts the bytes of an upload read off it, as the HTTP
// client sends them.
type uploadProgress struct {
	r    io.Reader
	n    int64
	sent func(int64)
}

func (u *uploadProgress) Read(b []byte) (int, error) {
	n, err := u.r.Read(b)
	u.n += int64(n)
	u.sent(u.n)
	return n, err
}

// download returns the latest state, checked to be the one the coordinator
// announced.
func (c *Client) download(ctx context.Context, want [32]byte) ([]byte, error) {
	req, err := c.request(ctx, http.MethodGet, "/v1/state", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := statusError(resp); err != nil {
		return nil, err
	}
	limit := c.MaxState
	if limit <= 0 {
		limit = ceremony.DefaultMaxState
	}
	if resp.ContentLength > limit {
		return nil, fmt.Errorf("the coordinator would send %d bytes for a state, more than %d (join -max-state raises it)", resp.ContentLength, limit)
	}
	body := &downloadProgress{r: io.LimitReader(resp.Body, limit+1), total: resp.ContentLength, event: c.event}
	state, err := io.ReadAll(body)
	if err != nil {
		// A transfer cut short is the network's failure, as a failed request is.
		return nil, &url.Error{Op: req.Method, URL: req.URL.String(), Err: err}
	}
	if int64(len(state)) > limit {
		return nil, fmt.Errorf("the coordinator sent more than %d bytes for a state (join -max-state raises it)", limit)
	}
	if got := sha256.Sum256(state); got != want {
		return nil, fmt.Errorf("the coordinator sent a state of sha256 %x, having announced %x", got, want)
	}
	return state, nil
}

// downloadProgress reports the bytes of the state downloaded so far, about
// every 1% of total (the response's length; no report when it is unknown).
type downloadProgress struct {
	r           io.Reader
	done, total int64
	next        int64
	event       func(Event)
}

func (d *downloadProgress) Read(b []byte) (int, error) {
	n, err := d.r.Read(b)
	d.done += int64(n)
	if d.total > 0 && (d.done >= d.next || err == io.EOF) {
		d.event(Event{Stage: StageDownloading, Done: min(d.done, d.total), Total: d.total})
		d.next = d.done + max(d.total/100, 1)
	}
	return n, err
}

// call sends a request with body and decodes the JSON reply into v, unless
// v is nil.
func (c *Client) call(parent context.Context, method, path string, body io.Reader, v any) error {
	ctx, cancel := context.WithTimeout(parent, callTimeout)
	defer cancel()
	req, err := c.request(ctx, method, path, body)
	if err != nil {
		return err
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		if parent.Err() == nil && errors.Is(err, context.DeadlineExceeded) {
			// No answer in time: the coordinator unreachable for now, as a
			// connection refused is, not a reason to stop.
			return &url.Error{Op: method, URL: req.URL.Redacted(), Err: fmt.Errorf("no answer within %s", callTimeout)}
		}
		return err
	}
	defer resp.Body.Close()
	if err := statusError(resp); err != nil {
		return err
	}
	if v == nil {
		return nil
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(v)
}

// request builds a request to the coordinator carrying the session token.
func (c *Client) request(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimSuffix(c.URL, "/")+path, body)
	if err != nil {
		return nil, err
	}
	if c.session != "" {
		req.Header.Set("Authorization", "Bearer "+c.session)
	}
	return req, nil
}

// httpError is an error status from the coordinator.
type httpError struct {
	code       int
	msg        string
	retryAfter time.Duration // when the coordinator says to try again: Retry-After, or 5 s
}

func (e *httpError) Error() string {
	return fmt.Sprintf("%d %s: %s", e.code, http.StatusText(e.code), e.msg)
}

// statusError turns an error status into an error (replyError).
func statusError(resp *http.Response) error {
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16)) // the status alone says enough if this fails
	return replyError(resp.StatusCode, data, resp.Header.Get("Retry-After"))
}

// replyError turns an answer of status code with body into an error:
// errTurnLost for 409, ErrUnauthorized for 401, and an httpError with the
// coordinator's message otherwise; nil for 200.
func replyError(code int, data []byte, retryAfter string) error {
	if code == http.StatusOK {
		return nil
	}
	var body struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(data, &body) != nil || body.Error == "" {
		body.Error = strings.TrimSpace(string(data))
	}
	body.Error = printable(body.Error)
	switch code {
	case http.StatusConflict:
		return fmt.Errorf("%w: %s", errTurnLost, body.Error)
	case http.StatusUnauthorized:
		return fmt.Errorf("%w: %s", ErrUnauthorized, body.Error)
	}
	e := &httpError{code: code, msg: body.Error, retryAfter: 5 * time.Second}
	if s, err := strconv.Atoi(retryAfter); err == nil && s > 0 {
		e.retryAfter = time.Duration(min(s, int(maxRetryAfter/time.Second))) * time.Second
	}
	return e
}

// unreachable reports whether err is the coordinator unreachable, or
// answering that it is failing: what trying again may get past.
func unreachable(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, errRedirected) {
		return false
	}
	var h *httpError
	if errors.As(err, &h) {
		return h.code >= 500
	}
	var u *url.Error
	return errors.As(err, &u)
}

// parseHash reads a sha256 in hex.
func parseHash(s string) ([32]byte, error) {
	var h [32]byte
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != len(h) {
		return h, fmt.Errorf("the coordinator sent %q for a sha256", s)
	}
	copy(h[:], b)
	return h, nil
}

// sleep waits for d, or until ctx is done.
func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
