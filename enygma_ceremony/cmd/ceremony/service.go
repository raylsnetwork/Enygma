package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/net/netutil"
	"golang.org/x/term"

	"enygma_ceremony/ceremony"
	"enygma_ceremony/coordinator"
	"enygma_ceremony/drand"
	"enygma_ceremony/participant"
	"enygma_ceremony/transcript"
)

// join takes a participant through a coordinated phase: the one command a
// participant runs, when their program was not built for the ceremony.
func join(fs *flag.FlagSet) func([]string, io.Writer) error {
	url := fs.String("url", defaultURL, "the coordinator's address")
	name := fs.String("name", "", "the name the transcript shows for you (default: asked in a terminal, else anonymous)")
	mine := fs.String("mine", "", "the sha256 an earlier run printed for your contribution, when the coordinator says you have contributed already")
	maxState := fs.Int64("max-state", ceremony.DefaultMaxState, "the most to download as one state, in bytes: raise it only for a ceremony announced as larger than 2^18")
	return func(args []string, w io.Writer) error {
		if *url == "" || len(args) != 0 || *maxState < 1 {
			return fmt.Errorf("%w: join takes -url, and a positive -max-state", errUsage)
		}
		c := &participant.Client{URL: *url, Name: *name, Mine: *mine, MaxState: *maxState}
		if isTerminal(os.Stdin) {
			c.AskName = askName(bufio.NewReader(os.Stdin), w)
		}
		return joinAndReport(c, w)
	}
}

// joinDefault is what the program does run with no command: join the
// coordinator it was built for. Started from a desktop rather than a shell,
// it asks for a name and keeps its window open until Enter, so the
// participant can read the sha256 to publish.
func joinDefault(stdout, stderr io.Writer) int {
	interactive := isTerminal(os.Stdin)
	in := bufio.NewReader(os.Stdin)
	c := &participant.Client{URL: defaultURL}
	if interactive {
		c.AskName = askName(in, stdout)
	}
	code := 0
	if err := joinAndReport(c, stdout); err != nil {
		fmt.Fprintf(stderr, "ceremony: %v\n", err)
		code = 1
	}
	if interactive {
		fmt.Fprint(stdout, "\nPress Enter to close.")
		if _, err := in.ReadString('\n'); err != nil {
			return code
		}
	}
	return code
}

// askName asks the participant, on w, for the name an open phase shows for
// them, and reads it from in.
func askName(in *bufio.Reader, w io.Writer) func() string {
	return func() string {
		fmt.Fprint(w, "Name to show in the transcript (press Enter to stay anonymous): ")
		line, err := in.ReadString('\n')
		if err != nil {
			return ""
		}
		return strings.TrimSpace(line)
	}
}

// joinAndReport runs c until it has contributed, and tells the participant
// what to publish.
func joinAndReport(c *participant.Client, w io.Writer) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	c.HTTP, c.Out = participant.HTTPClient(), w
	c.Contribute = contributeInChild(ctx) // the randomness never lives in this process
	fmt.Fprint(w, selfDescription())
	fmt.Fprintf(w, "Joining the Enygma setup ceremony at %s.\nLeave this running until it says you are done; it waits for your turn by itself.\n", c.URL)
	r, err := c.Run(ctx)
	if err != nil {
		return err
	}
	if r.Already {
		fmt.Fprintf(w, "You have contributed already: contribution %d, sha256 %x\n", r.Index, r.SHA256)
		return nil
	}
	fmt.Fprintf(w, "\nDone: your contribution is number %d of the transcript.\n", r.Index)
	fmt.Fprintf(w, "  on top of sha256 %x\n", r.Prev)
	fmt.Fprintf(w, "  your contribution sha256 %x  <- publish this\n", r.SHA256)
	fmt.Fprintf(w, "Check it is in the transcript any time: ceremony verify -url %s -dir transcript -mine %x\n", c.URL, r.SHA256)
	return nil
}

// isTerminal reports whether f is a terminal, as when the program was started
// from a desktop or a shell, not from a script.
func isTerminal(f *os.File) bool { return term.IsTerminal(int(f.Fd())) }

// verify checks a phase's transcript from end to end: one downloaded from a
// coordinator into DIR (checking each step as it arrives), or DIR as it is.
func verify(fs *flag.FlagSet) func([]string, io.Writer) error {
	url := fs.String("url", "", "the coordinator to download the transcript from; without it, DIR is checked as it is")
	dir := fs.String("dir", "", "the directory to download the transcript into, or holding it")
	mine := fs.String("mine", "", "your contribution's sha256, to find in the transcript")
	sync := fs.Bool("sync", false, "-url: let the download replace states in DIR that differ from the coordinator's, and remove states past its end (without it, a DIR holding anything else is refused)")
	maxBytes := fs.Int64("max-bytes", transcript.DefaultMaxBytes, "-url: the most to download in all")
	maxState := fs.Int64("max-state", ceremony.DefaultMaxState, "-url: the most to download as one state, in bytes")
	srs := fs.String("srs", "", "Phase 2: the SRS it started from, to check its start (with -r1cs)")
	r1cs, circuitsIn := circuitsFlags(fs)
	return func(args []string, w io.Writer) error {
		if *dir == "" || len(args) != 0 || (*srs == "") != (*r1cs == "") || *maxState < 1 || *maxBytes < 1 {
			return fmt.Errorf("%w: verify takes -dir, -srs with -r1cs for a Phase 2, and positive limits", errUsage)
		}
		var p2 *transcript.Phase2Start
		if *srs != "" {
			circuits, err := circuitsIn(w)
			if err != nil {
				return err
			}
			p2 = &transcript.Phase2Start{SRS: *srs, Circuits: circuits}
		}
		var (
			t   transcript.Transcript
			err error
		)
		if *url != "" {
			fmt.Fprintf(w, "Downloading the transcript from %s into %s, checking each step as it arrives.\n", *url, *dir)
			opt := transcript.FetchOptions{Sync: *sync, Check: true, MaxBytes: *maxBytes, MaxState: *maxState}
			if t, err = transcript.Fetch(context.Background(), participant.HTTPClient(), *url, *dir, w, opt); err != nil {
				return err
			}
			err = transcript.CheckStart(*dir, p2, w)
		} else {
			t, err = transcript.CheckWith(*dir, p2, w)
		}
		if err != nil && !errors.Is(err, transcript.ErrStartNotChecked) {
			return err
		}
		if *mine != "" {
			e, ok := t.Find(*mine)
			if !ok {
				return fmt.Errorf("no contribution in the transcript has sha256 %s", *mine)
			}
			fmt.Fprintf(w, "Yours is contribution %d, by %q, accepted %s.\n", e.Index, e.Participant, e.At.Format(time.RFC3339))
		}
		if err != nil {
			return fmt.Errorf("NOT VERIFIED: %d contributions, each a valid step on top of the one before it, but %w", len(t.Contributions), err)
		}
		fmt.Fprintf(w, "OK: the start, and %d contributions, each a valid step on top of the one before it; latest sha256 %s\n",
			len(t.Contributions), t.Latest())
		return nil
	}
}

// versionCmd prints the commit this program was built from.
func versionCmd(*flag.FlagSet) func([]string, io.Writer) error {
	return func(args []string, w io.Writer) error {
		if len(args) != 0 {
			return fmt.Errorf("%w: version takes no arguments", errUsage)
		}
		v := coordinator.Version()
		if v == "" {
			v = "unknown: built without version control information"
		}
		fmt.Fprintf(w, "ceremony, commit %s\n", v)
		if defaultURL != "" {
			fmt.Fprintf(w, "built for the coordinator at %s\n", defaultURL)
		}
		return nil
	}
}

// selfDescription says which program is about to draw the participant's
// randomness: the commit it was built from, and its own sha256, to compare
// with the ceremony's GitHub release (its tag, and SHA256SUMS). Built from
// source with go run, the sha256 is of a local build; the commit is what to
// compare.
func selfDescription() string {
	v := coordinator.Version()
	if v == "" {
		v = "unknown (built without version control information)"
	}
	s := "This program: commit " + v
	if path, err := os.Executable(); err == nil {
		if h, _, err := transcript.HashFile(path); err == nil {
			s += fmt.Sprintf(", sha256 %x", h)
		}
	}
	return s + ". Compare them with the ceremony's GitHub release.\n"
}

// coordinate runs a phase as a service until interrupted. On the first
// interrupt it hands out no more turns and waits for the one under way; on
// the second it stops at once.
func coordinate(fs *flag.FlagSet) func([]string, io.Writer) error {
	dir := fs.String("dir", "", "the phase's directory, holding state-0 from phase1-new or phase2-new")
	auth := fs.String("auth", string(coordinator.AuthOpen), "how anyone joins: open (with the link alone) or github (signed in with GitHub)")
	githubClientID := fs.String("github-client-id", "", "-auth github: the client ID of a GitHub OAuth app with the device flow enabled")
	githubMinAge := fs.Duration("github-min-age", 30*24*time.Hour, "-auth github: how old a GitHub account must be")
	maxPerAddress := fs.Int("max-per-address", 3, "participants from one network address queued, or holding the turn, at once: at least 1 (-no-address-limit lifts the bound)")
	noAddressLimit := fs.Bool("no-address-limit", false, "no bound on the participants from one network address")
	ipv6Prefix := fs.Int("ipv6-prefix", 64, "how many leading bits of an IPv6 address count as one network address, for the limits")
	maxQueue := fs.Int("max-queue", 1000, "participants queued at once, from everywhere")
	maxTurns := fs.Int("max-turns", 3, "turns one identity is given without contributing")
	maxContributions := fs.Int("max-contributions", 1000, "contributions the phase takes in all; each is a state file kept for the phase's life")
	maxContributionsPerAddress := fs.Int("max-contributions-per-address", 10, "contributions the phase takes from one network address (-no-address-limit lifts the bound)")
	minFreeMiB := fs.Int64("min-free-mib", 1024, "MiB of disk space under -dir to keep free, besides room for another state: below it, no turns are given")
	maxSessions := fs.Int("max-sessions", 10000, "registrations kept at once, and network addresses whose registrations are counted")
	listen := fs.String("listen", "", "the address to serve on: with -tls-cert, :443 by default; behind a TLS proxy (-trusted-proxy) or with -insecure-http, 127.0.0.1:8080 by default; not with -acme-domain, which serves on :443 and :80")
	acmeDomain := fs.String("acme-domain", "", "serve HTTPS on :443 for this host name, with a certificate from Let's Encrypt (port 80 must reach this machine too)")
	acmeCache := fs.String("acme-cache", "", "-acme-domain: the directory keeping the certificate and the ACME account key, an absolute path (default: acme-cache beside -dir)")
	acmeEmail := fs.String("acme-email", "", "-acme-domain: an address for Let's Encrypt to write to about the certificate")
	trustedProxy := fs.String("trusted-proxy", "", "behind a TLS proxy of your own: its addresses or CIDRs, comma-separated. "+
		"Serves it plain HTTP, and takes the address of a request from it as the last it put in X-Forwarded-For; "+
		"from anywhere else, the connection's address")
	insecureHTTP := fs.Bool("insecure-http", false, "serve plain HTTP with no TLS in front: for tests and rehearsals on this machine only, since participants' tokens, and the page, travel in clear")
	turn := fs.Duration("turn", 60*time.Minute, "a participant's time to download, contribute and upload: about 2 minutes at 2^18 on a laptop, room for a slow one")
	turnPresence := fs.Duration("turn-presence", 2*time.Minute, "how long the turn holder's program may go silent before the turn passes on")
	presence := fs.Duration("presence", 30*time.Second, "how recently a queued participant must have asked to be given the turn")
	away := fs.Duration("away", 10*time.Minute, "how long a queued participant may go quiet and keep their place")
	poll := fs.Duration("poll", 5*time.Second, "how often queued participants ask")
	closeAt := fs.String("close-at", "", "when to stop handing out turns, RFC 3339 (2026-12-01T15:00:00Z); a phase closed at its time stays closed. "+
		"The last turn may run -turn past it: announce a drand round at least -turn after it (ceremony beacon -close-at … -turn …)")
	srs := fs.String("srs", "", "Phase 2: the SRS it started from, to check state-0 before serving (with -r1cs)")
	r1cs, circuitsIn := circuitsFlags(fs)
	notice := fs.String("notice", "", "a message for participants, shown on the page and by their program")
	releaseURL := fs.String("release-url", "", "the ceremony's GitHub release (https://github.com/<owner>/<repo>/releases/tag/<tag>), where participants get the program; the page links to it")
	cert := fs.String("tls-cert", "", "a TLS certificate, to serve HTTPS on -listen (with -tls-key)")
	key := fs.String("tls-key", "", "the certificate's key")
	return func(args []string, w io.Writer) error {
		if *dir == "" || len(args) != 0 || (*cert == "") != (*key == "") {
			return fmt.Errorf("%w: coordinate takes -dir, and -tls-cert with -tls-key", errUsage)
		}
		// A limit of 0, or less, would quietly be no limit (audit R-061).
		switch {
		case *maxPerAddress < 1:
			return fmt.Errorf("%w: -max-per-address is at least 1 (-no-address-limit lifts the bound)", errUsage)
		case *githubMinAge <= 0, *maxQueue < 1, *maxTurns < 1, *ipv6Prefix < 1, *ipv6Prefix > 128:
			return fmt.Errorf("%w: -github-min-age, -max-queue and -max-turns are positive, and -ipv6-prefix from 1 to 128", errUsage)
		case *maxContributions < 1, *maxContributionsPerAddress < 1, *minFreeMiB < 1, *maxSessions < 1:
			return fmt.Errorf("%w: -max-contributions, -max-contributions-per-address, -min-free-mib and -max-sessions are positive", errUsage)
		}
		proxies, err := parseProxies(*trustedProxy)
		if err != nil {
			return fmt.Errorf("%w: -trusted-proxy: %v", errUsage, err)
		}
		// Participants are served over TLS, or the operator says otherwise
		// (audit R-006): over plain http the page, the session and GitHub
		// tokens, and the uploads are anyone's on the way.
		modes := 0
		for _, on := range []bool{*acmeDomain != "", *cert != "", len(proxies) > 0, *insecureHTTP} {
			if on {
				modes++
			}
		}
		if modes != 1 {
			return fmt.Errorf("%w: coordinate serves participants over TLS: give one of -acme-domain, -tls-cert with -tls-key, "+
				"-trusted-proxy (a TLS proxy of your own in front), or -insecure-http (tests on this machine only)", errUsage)
		}
		switch {
		case *acmeDomain != "" && *listen != "":
			return fmt.Errorf("%w: -listen does not go with -acme-domain, which serves on :443 and :80", errUsage)
		case *listen != "":
		case *cert != "":
			*listen = ":443"
		default: // behind a proxy, or plain http: this machine only, unless told otherwise
			*listen = "127.0.0.1:8080"
		}
		if *acmeDomain != "" {
			if *acmeCache == "" {
				abs, err := filepath.Abs(*dir)
				if err != nil {
					return err
				}
				*acmeCache = filepath.Join(filepath.Dir(abs), "acme-cache")
			}
			if !filepath.IsAbs(*acmeCache) {
				return fmt.Errorf("%w: -acme-cache must be an absolute path: it holds the TLS and ACME account keys", errUsage)
			}
		}
		var closes time.Time
		if *closeAt != "" {
			var err error
			if closes, err = time.Parse(time.RFC3339, *closeAt); err != nil {
				return fmt.Errorf("%w: -close-at: %v", errUsage, err)
			}
		}
		// The start is what every contribution builds on: checked before
		// anyone does (audit R-060).
		if (*srs == "") != (*r1cs == "") {
			return fmt.Errorf("%w: -srs goes with -r1cs", errUsage)
		}
		var p2 *transcript.Phase2Start
		if *srs != "" {
			circuits, err := circuitsIn(w)
			if err != nil {
				return err
			}
			p2 = &transcript.Phase2Start{SRS: *srs, Circuits: circuits}
		}
		switch err := transcript.CheckStart(*dir, p2, w); {
		case errors.Is(err, transcript.ErrStartNotChecked):
			fmt.Fprintln(w, "WARNING: this Phase 2's start is not checked: give -srs and -r1cs to check it before serving it.")
		case err != nil:
			return err
		}
		logger := log.New(w, "", log.LstdFlags)
		c, err := coordinator.New(coordinator.Config{
			Dir: *dir, Auth: coordinator.Auth(*auth),
			GitHubClientID: *githubClientID, GitHubMinAge: *githubMinAge,
			MaxPerAddress: *maxPerAddress, NoAddressLimit: *noAddressLimit, TrustedProxies: proxies,
			IPv6Prefix: *ipv6Prefix, MaxQueue: *maxQueue, MaxTurns: *maxTurns,
			MaxContributions: *maxContributions, MaxContributionsPerAddress: *maxContributionsPerAddress,
			MinFree: *minFreeMiB << 20, MaxSessions: *maxSessions,
			Turn: *turn, TurnPresence: *turnPresence, Presence: *presence, Away: *away, Poll: *poll,
			CloseAt: closes, Notice: *notice, ReleaseURL: *releaseURL, Log: logger,
		})
		if err != nil {
			return err
		}
		handler := c.Handler()
		if *acmeDomain != "" || *cert != "" {
			handler = strictTransport(handler)
		}
		// Every request, and every answer, has a deadline: a slow or stalled
		// client holds a handler only so long (audit R-011). The handlers that
		// need longer give themselves it: an upload until its turn ends, a
		// state sent at a minimum pace, an answer after a long check.
		srv := &http.Server{
			Addr:              *listen,
			Handler:           handler,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       time.Minute,
			WriteTimeout:      2 * time.Minute,
			IdleTimeout:       2 * time.Minute,
			MaxHeaderBytes:    64 << 10, // ample for this protocol's headers (audit R-018)
			HTTP2: &http.HTTP2Config{
				MaxConcurrentStreams: 16,          // this protocol has a few requests in flight per participant
				WriteByteTimeout:     time.Minute, // a connection whose reader has stopped reading is closed
				SendPingTimeout:      time.Minute,
				PingTimeout:          15 * time.Second,
			},
			ErrorLog: logger,
		}
		served := make(chan error, 2)
		var redirect *http.Server
		switch {
		case *acmeDomain != "":
			m := &autocert.Manager{
				Prompt:     autocert.AcceptTOS,
				HostPolicy: autocert.HostWhitelist(*acmeDomain),
				Cache:      autocert.DirCache(*acmeCache),
				Email:      *acmeEmail,
			}
			srv.Addr, srv.TLSConfig = ":443", m.TLSConfig()
			// Port 80 answers Let's Encrypt's challenges and sends everyone else to HTTPS.
			redirect = &http.Server{Addr: ":80", Handler: m.HTTPHandler(nil), ReadHeaderTimeout: 10 * time.Second,
				ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: time.Minute, ErrorLog: logger}
			go func() { served <- serve(redirect, false, "", "") }()
			go func() { served <- serve(srv, true, "", "") }()
			logger.Printf("serving https://%s/ (and :80 for certificates), keeping the certificate in %s", *acmeDomain, *acmeCache)
		case *cert != "":
			go func() { served <- serve(srv, true, *cert, *key) }()
			// On the standard port, port 80 sends whoever arrives there to HTTPS (audit R-024).
			if host, port, err := net.SplitHostPort(*listen); err == nil && port == "443" {
				redirect = &http.Server{Addr: net.JoinHostPort(host, "80"), Handler: http.HandlerFunc(toHTTPS),
					ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second,
					IdleTimeout: time.Minute, ErrorLog: logger}
				go func() { served <- serve(redirect, false, "", "") }()
			}
			logger.Printf("serving HTTPS on %s", *listen)
		case len(proxies) > 0:
			go func() { served <- serve(srv, false, "", "") }()
			logger.Printf("serving plain HTTP on %s, to the TLS proxy in front of it (%s)", *listen, *trustedProxy)
		default:
			go func() { served <- serve(srv, false, "", "") }()
			logger.Printf("serving plain HTTP on %s, with no TLS: for tests on this machine only", *listen)
		}
		// A phase closed at its time stays closed: started again, the
		// coordinator serves its transcript, read-only, for anyone to check.
		closedAtStart := c.Closed()
		if closedAtStart {
			logger.Printf("phase in %s: closed, after %d contributions; serving its transcript, read-only, until interrupted",
				*dir, len(c.States()))
		} else {
			logger.Printf("phase in %s: %d contributions so far; participants are admitted %s; interrupt to close",
				*dir, len(c.States()), *auth)
		}

		sigs := make(chan os.Signal, 2)
		signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(sigs)
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
	serving: // until an interrupt, or the scheduled close
		for {
			select {
			case err := <-served:
				return err
			case <-sigs:
				break serving
			case <-tick.C:
				if !closedAtStart && c.Closed() {
					break serving
				}
			}
		}
		logger.Printf("closing: no new turns; waiting for the one under way (interrupt again to stop now)")
		c.Close()
	wait:
		for c.Busy() {
			select {
			case <-sigs:
				break wait
			case <-tick.C:
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, s := range []*http.Server{srv, redirect} {
			if s == nil {
				continue
			}
			if err := s.Shutdown(ctx); err != nil {
				logger.Printf("shutting down: %v", err)
			}
		}
		// The head, published before the beacon round is out, binds the seal
		// to this chain: nothing can be added after, or left off the end
		// (audit R-003, R-004).
		t, err := transcript.Load(*dir)
		if err != nil {
			return err
		}
		seal := "phase1-seal"
		if info, err := ceremony.Inspect(transcript.Path(*dir, 0)); err == nil && info.Kind == ceremony.KindPhase2 {
			seal = "phase2-seal -ceremony " + info.Ceremony + " -srs <srs> -r1cs <r1cs dir> -keys <keys dir>"
		}
		fmt.Fprintf(w, "Closed after %d contributions. Announce the head now, before the beacon round is out:\n"+
			"  head sha256 %s (contribution %d)\n"+
			"Then seal, with the round you announced:\n"+
			"  ceremony %s -transcript %s -head %s -drand <round> -close-at <the close>\n",
			len(t.Contributions), t.Latest(), len(t.Contributions), seal, *dir, t.Latest())
		return nil
	}
}

// parseProxies reads -trusted-proxy: addresses or CIDRs, comma-separated.
func parseProxies(s string) ([]netip.Prefix, error) {
	var proxies []netip.Prefix
	for _, item := range strings.Split(s, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if p, err := netip.ParsePrefix(item); err == nil {
			proxies = append(proxies, p.Masked())
			continue
		}
		a, err := netip.ParseAddr(item)
		if err != nil {
			return nil, fmt.Errorf("%q is not an address or a CIDR", item)
		}
		proxies = append(proxies, netip.PrefixFrom(a.Unmap(), a.Unmap().BitLen()))
	}
	return proxies, nil
}

// strictTransport tells browsers to come back over HTTPS only (audit
// R-024): after one visit, a plain-http link or a typed host name can no
// longer be served a hostile page by someone in between.
func strictTransport(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		next.ServeHTTP(w, r)
	})
}

// maxConnections bounds the connections a coordinator holds open at once
// (audit R-011): past it, new ones wait to be accepted, rather than each
// taking memory and a file of its own without end.
const maxConnections = 4096

// serve serves s on its address, over TLS when tls is set (from certFile and
// keyFile, or s.TLSConfig), holding at most maxConnections at once.
func serve(s *http.Server, tls bool, certFile, keyFile string) error {
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	ln = netutil.LimitListener(ln, maxConnections)
	if tls {
		return s.ServeTLS(ln, certFile, keyFile)
	}
	return s.Serve(ln)
}

// toHTTPS sends a plain-http request to the same address over HTTPS.
func toHTTPS(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	http.Redirect(w, r, "https://"+host+r.URL.RequestURI(), http.StatusMovedPermanently)
}

// beacon prints the drand round to announce for a phase's close, or a
// round's beacon. The last turn handed out before the close may run -turn
// past it (audit R-031), so the round to announce is the first out at least
// -turn after the close: no contribution can follow it.
func beacon(fs *flag.FlagSet) func([]string, io.Writer) error {
	closeAt := fs.String("close-at", "", "when the phase closes, RFC 3339 (2026-12-01T15:00:00Z): print the drand round to announce for it")
	turn := fs.Duration("turn", 60*time.Minute, "the phase's -turn: the last turn handed out may run this long past the close")
	round := fs.Uint64("round", 0, "a drand round: fetch it, check its signature, and print its randomness as the beacon")
	return func(args []string, w io.Writer) error {
		if (*closeAt == "") == (*round == 0) || len(args) != 0 || *turn < 0 {
			return fmt.Errorf("%w: beacon takes -close-at (and -turn), or -round", errUsage)
		}
		if *closeAt != "" {
			t, err := time.Parse(time.RFC3339, *closeAt)
			if err != nil {
				return fmt.Errorf("%w: -close-at: %v", errUsage, err)
			}
			r := drand.RoundAt(t.Add(*turn))
			fmt.Fprintf(w, "drand quicknet round %d, out at %s: the first out at least %s after the close, once the last turn is over\n",
				r, drand.Time(r).UTC().Format(time.RFC3339), *turn)
			return nil
		}
		b, err := drand.Fetch(context.Background(), &http.Client{Timeout: 30 * time.Second}, drand.Relays, *round)
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "%x\n", b)
		return nil
	}
}
