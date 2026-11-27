package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/acme/autocert"
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
	return func(args []string, w io.Writer) error {
		if *url == "" || len(args) != 0 {
			return fmt.Errorf("%w: join takes -url", errUsage)
		}
		c := &participant.Client{URL: *url, Name: *name}
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
	c.HTTP, c.Out = &http.Client{}, w
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

// verify downloads a phase's transcript and checks every step.
func verify(fs *flag.FlagSet) func([]string, io.Writer) error {
	url := fs.String("url", defaultURL, "the coordinator to download the transcript from; none to check DIR as it is")
	dir := fs.String("dir", "", "the directory to download the transcript into, or holding it")
	mine := fs.String("mine", "", "your contribution's sha256, to find in the transcript")
	return func(args []string, w io.Writer) error {
		if *dir == "" || len(args) != 0 {
			return fmt.Errorf("%w: verify takes -dir", errUsage)
		}
		if *url != "" {
			fmt.Fprintf(w, "Downloading the transcript from %s into %s.\n", *url, *dir)
			if _, err := transcript.Fetch(context.Background(), &http.Client{}, *url, *dir, w); err != nil {
				return err
			}
		}
		t, err := transcript.Check(*dir, w)
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "OK: %d contributions, each a valid step on top of the one before it; latest sha256 %s\n",
			len(t.Contributions), t.Latest())
		if *mine != "" {
			e, ok := t.Find(*mine)
			if !ok {
				return fmt.Errorf("no contribution in the transcript has sha256 %s", *mine)
			}
			fmt.Fprintf(w, "Yours is contribution %d, by %s, accepted %s.\n", e.Index, e.Participant, e.At.Format(time.RFC3339))
		}
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
	maxPerAddress := fs.Int("max-per-address", 3, "participants from one network address queued at once; 0 for no bound")
	listen := fs.String("listen", ":8080", "the address to serve plain HTTP on, behind a TLS proxy; ignored with -acme-domain")
	acmeDomain := fs.String("acme-domain", "", "serve HTTPS on :443 for this host name, with a certificate from Let's Encrypt (port 80 must reach this machine too)")
	acmeCache := fs.String("acme-cache", "acme-cache", "-acme-domain: the directory keeping the certificate")
	behindProxy := fs.Bool("behind-proxy", false, "take participants' addresses from X-Forwarded-For, as a reverse proxy sets it")
	turn := fs.Duration("turn", 60*time.Minute, "a participant's time to download, contribute and upload: about 2 minutes at 2^18 on a laptop, room for a slow one")
	turnPresence := fs.Duration("turn-presence", 2*time.Minute, "how long the turn holder's program may go silent before the turn passes on")
	presence := fs.Duration("presence", 30*time.Second, "how recently a queued participant must have asked to be given the turn")
	away := fs.Duration("away", 10*time.Minute, "how long a queued participant may go quiet and keep their place")
	poll := fs.Duration("poll", 5*time.Second, "how often queued participants ask")
	closeAt := fs.String("close-at", "", "when to stop handing out turns, RFC 3339 (2026-12-01T15:00:00Z); announce the beacon round for after it")
	notice := fs.String("notice", "", "a message for participants, shown on the page and by their program")
	releaseURL := fs.String("release-url", "", "the ceremony's GitHub release (https://github.com/<owner>/<repo>/releases/tag/<tag>), where participants get the program; the page links to it")
	cert := fs.String("tls-cert", "", "a TLS certificate, to serve HTTPS on -listen (with -tls-key)")
	key := fs.String("tls-key", "", "the certificate's key")
	return func(args []string, w io.Writer) error {
		if *dir == "" || len(args) != 0 || (*cert == "") != (*key == "") || (*cert != "" && *acmeDomain != "") {
			return fmt.Errorf("%w: coordinate takes -dir; -tls-cert with -tls-key, or -acme-domain", errUsage)
		}
		var closes time.Time
		if *closeAt != "" {
			var err error
			if closes, err = time.Parse(time.RFC3339, *closeAt); err != nil {
				return fmt.Errorf("%w: -close-at: %v", errUsage, err)
			}
		}
		logger := log.New(w, "", log.LstdFlags)
		c, err := coordinator.New(coordinator.Config{
			Dir: *dir, Auth: coordinator.Auth(*auth),
			GitHubClientID: *githubClientID, GitHubMinAge: *githubMinAge,
			MaxPerAddress: *maxPerAddress, BehindProxy: *behindProxy,
			Turn: *turn, TurnPresence: *turnPresence, Presence: *presence, Away: *away, Poll: *poll,
			CloseAt: closes, Notice: *notice, ReleaseURL: *releaseURL, Log: logger,
		})
		if err != nil {
			return err
		}
		srv := &http.Server{
			Addr:              *listen,
			Handler:           c.Handler(),
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       2 * time.Minute,
			ErrorLog:          logger,
		}
		served := make(chan error, 2)
		var redirect *http.Server
		switch {
		case *acmeDomain != "":
			m := &autocert.Manager{
				Prompt:     autocert.AcceptTOS,
				HostPolicy: autocert.HostWhitelist(*acmeDomain),
				Cache:      autocert.DirCache(*acmeCache),
			}
			srv.Addr, srv.TLSConfig = ":443", m.TLSConfig()
			// Port 80 answers Let's Encrypt's challenges and sends everyone else to HTTPS.
			redirect = &http.Server{Addr: ":80", Handler: m.HTTPHandler(nil), ReadHeaderTimeout: 10 * time.Second, ErrorLog: logger}
			go func() { served <- redirect.ListenAndServe() }()
			go func() { served <- srv.ListenAndServeTLS("", "") }()
			logger.Printf("serving https://%s/ (and :80 for certificates)", *acmeDomain)
		case *cert != "":
			go func() { served <- srv.ListenAndServeTLS(*cert, *key) }()
			logger.Printf("serving HTTPS on %s", *listen)
		default:
			go func() { served <- srv.ListenAndServe() }()
			logger.Printf("serving HTTP on %s; put a TLS proxy in front of it for participants", *listen)
		}
		logger.Printf("phase in %s: %d contributions so far; participants are admitted %s; interrupt to close",
			*dir, len(c.States()), *auth)

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
				if c.Closed() {
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
		states := c.States()
		fmt.Fprintf(w, "Closed after %d contributions. Seal them, in order, with the beacon round you announced:\n  %s\n",
			len(states), strings.Join(states, " "))
		return nil
	}
}

// beacon prints the drand round to announce for a time, or a round's beacon.
func beacon(fs *flag.FlagSet) func([]string, io.Writer) error {
	at := fs.String("at", "", "a time, RFC 3339 (2026-12-01T15:00:00Z): print the first drand round out at or after it")
	round := fs.Uint64("round", 0, "a drand round: fetch it, check its signature, and print its randomness as the beacon")
	return func(args []string, w io.Writer) error {
		if (*at == "") == (*round == 0) || len(args) != 0 {
			return fmt.Errorf("%w: beacon takes -at or -round", errUsage)
		}
		if *at != "" {
			t, err := time.Parse(time.RFC3339, *at)
			if err != nil {
				return fmt.Errorf("%w: -at: %v", errUsage, err)
			}
			r := drand.RoundAt(t)
			fmt.Fprintf(w, "drand quicknet round %d, out at %s\n", r, drand.Time(r).UTC().Format(time.RFC3339))
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

// beaconFlags declares a seal's -beacon and -drand flags and returns what
// reads the beacon from whichever was given: hex as is, or a drand round
// fetched and checked.
func beaconFlags(fs *flag.FlagSet) func(w io.Writer) ([]byte, error) {
	hexBeacon := fs.String("beacon", "", "the beacon, hex: a drand round's randomness, announced before the phase closed")
	round := fs.Uint64("drand", 0, "a drand quicknet round, announced before the phase closed: its randomness is the beacon")
	return func(w io.Writer) ([]byte, error) {
		switch {
		case (*hexBeacon == "") == (*round == 0):
			return nil, fmt.Errorf("%w: give the beacon as -beacon or -drand, not both", errUsage)
		case *round != 0:
			b, err := drand.Fetch(context.Background(), &http.Client{Timeout: 30 * time.Second}, drand.Relays, *round)
			if err != nil {
				return nil, err
			}
			fmt.Fprintf(w, "Beacon: drand quicknet round %d, %x\n", *round, b)
			return b[:], nil
		}
		b, err := hex.DecodeString(*hexBeacon)
		if err != nil {
			return nil, fmt.Errorf("%w: -beacon: %v", errUsage, err)
		}
		if len(b) < ceremony.MinBeacon {
			return nil, fmt.Errorf("%w: %d bytes", ceremony.ErrBeacon, len(b))
		}
		return b, nil
	}
}
