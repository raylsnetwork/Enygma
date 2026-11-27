package main

import (
	"bufio"
	"context"
	"encoding/hex"
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
		printError(stderr, "ceremony", err)
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
		fmt.Fprint(w, checkAdvice(r.SHA256, r.Of.Kind))
		return nil
	}
	fmt.Fprintf(w, "\nThe coordinator accepted your contribution as number %d.\n", r.Index)
	switch r.Of.Kind {
	case ceremony.KindPhase1:
		fmt.Fprintf(w, "  to Phase 1 for circuits of up to %d constraints\n", r.Of.N)
	case ceremony.KindPhase2:
		// Whose keys depends on the SRS it builds on (audit H1).
		fmt.Fprintf(w, "  to a Phase 2 on the SRS of sha256 %x: check it is the one announced\n", r.Of.SRS)
	}
	fmt.Fprintf(w, "  on top of sha256 %x\n", r.Prev)
	fmt.Fprintf(w, "  your contribution sha256 %x  <- publish this yourself, where the coordinator has no say\n", r.SHA256)
	fmt.Fprint(w, checkAdvice(r.SHA256, r.Of.Kind))
	return nil
}

// checkAdvice says what a contribution of sha256 mine, to a phase of kind
// (0 when not known), needs to count, and how its contributor checks it
// (audit H2): the program knows only what the coordinator said, which it
// can say to one participant alone, and so does any check that asks the
// coordinator.
func checkAdvice(mine [32]byte, kind ceremony.Kind) string {
	phase2 := " -srs <the SRS> -srs-sha256 <its announced sha256> -r1cs <the circuits>"
	also, note := "", ""
	switch kind {
	case ceremony.KindPhase2:
		also = phase2
	case ceremony.KindPhase1:
	default:
		note = "  (for a Phase 2, with" + phase2 + ")\n"
	}
	return "That is the coordinator's word, which it could give you alone. Your contribution counts only if it is in the\n" +
		"chain that ends at the head announced at the close. Then check that, from a copy of the phase the coordinator\n" +
		"does not serve to you alone:\n" +
		fmt.Sprintf("  ceremony verify -static <the phase's published copy> -dir <a new directory> -head <the announced head>%s -mine %x\n", also, mine) + note
}

// isTerminal reports whether f is a terminal, as when the program was started
// from a desktop or a shell, not from a script.
func isTerminal(f *os.File) bool { return term.IsTerminal(int(f.Fd())) }

// verify checks a phase's transcript from end to end: one downloaded from a
// coordinator into DIR (checking each step as it arrives), or DIR as it is.
func verify(fs *flag.FlagSet) func([]string, io.Writer) error {
	url := fs.String("url", "", "the coordinator to download the transcript from; without it (or -static), DIR is checked as it is")
	static := fs.String("static", "", "in place of -url: a copy of the phase's directory published at this address, which the coordinator does not serve")
	dir := fs.String("dir", "", "the directory to download the transcript into, or holding it")
	head := fs.String("head", "", "the sha256 of the phase's last contribution, as announced at the close: the chain checked must end there")
	var mine []string
	fs.Func("mine", "a contribution's sha256 to find in the chain, as its contributor published it (yours, or one you trust); repeat it, or separate them with commas", func(v string) error {
		mine = append(mine, strings.Split(v, ",")...)
		return nil
	})
	sync := fs.Bool("sync", false, "-url, -static: let the download move aside states and entries in DIR that the host now contradicts, and those past its end (without it, a DIR holding anything else is refused)")
	maxBytes := fs.Int64("max-bytes", transcript.DefaultMaxBytes, "-url, -static: the most to download in all")
	maxState := fs.Int64("max-state", ceremony.DefaultMaxState, "-url, -static: the most to download as one state, in bytes")
	size := fs.Uint64("size", 0, "a Phase 1: the domain size announced; anything else is refused")
	name := fs.String("ceremony", "", "a Phase 2: the protocol announced, e.g. enygma_retail_payments; anything else is refused")
	srsIn := srsFlags(fs, true)
	r1cs, circuitsIn := circuitsFlags(fs)
	return func(args []string, w io.Writer) error {
		if *dir == "" || len(args) != 0 || srsIn.given() != (*r1cs != "") || *maxState < 1 || *maxBytes < 1 || (*size != 0 && *name != "") || (*url != "" && *static != "") {
			return fmt.Errorf("%w: verify takes -dir, at most one of -url and -static, -srs (with -srs-sha256 or -phase1) and -r1cs for a Phase 2, at most one of -size and -ceremony, and positive limits", errUsage)
		}
		receipts, err := sha256List(strings.Join(mine, ","))
		if err != nil {
			return fmt.Errorf("%w: -mine: %v", errUsage, err)
		}
		announced, err := sha256List(*head)
		if err != nil || len(announced) > 1 {
			return fmt.Errorf("%w: -head is the sha256 of the last contribution, as announced at the close", errUsage)
		}
		var p2 *transcript.Phase2Start
		if srsIn.given() {
			circuits, err := circuitsIn(w)
			if err != nil {
				return err
			}
			srs, _, err := srsIn.read(w)
			if err != nil {
				return err
			}
			p2 = &transcript.Phase2Start{SRS: srs, Circuits: circuits}
		}
		var t transcript.Transcript
		if from := *url + *static; from != "" {
			fmt.Fprintf(w, "Downloading the transcript from %s into %s, checking each step as it arrives.\n", from, *dir)
			opt := transcript.FetchOptions{Static: *static != "", Sync: *sync, Check: true, MaxBytes: *maxBytes, MaxState: *maxState}
			client := participant.HTTPClient()
			if opt.Static {
				client = staticClient()
			}
			if t, err = transcript.Fetch(context.Background(), client, from, *dir, w, opt); err != nil {
				return err
			}
			err = transcript.CheckStart(*dir, p2, w)
		} else {
			t, err = transcript.CheckWith(*dir, p2, w)
		}
		if err != nil && !errors.Is(err, transcript.ErrStartNotChecked) {
			return err
		}
		// What was checked must be the phase asked for (audit I3): a host can
		// serve any phase, and a Phase 2's name comes from its own state-0.
		start, ierr := ceremony.Inspect(transcript.Path(*dir, 0))
		switch {
		case ierr != nil:
			return ierr
		case start.Kind == ceremony.KindPhase1 && p2 != nil:
			return fmt.Errorf("this is a Phase 1, which starts from no SRS: -srs and -r1cs are for a Phase 2")
		case *size != 0 && (start.Kind != ceremony.KindPhase1 || start.N != *size):
			return fmt.Errorf("this is %s, not Phase 1 for circuits of up to %d constraints", phaseOf(start), *size)
		case *name != "" && (start.Kind != ceremony.KindPhase2 || start.Ceremony != *name):
			return fmt.Errorf("this is %s, not %s's Phase 2", phaseOf(start), *name)
		}
		// A contribution counts only in the chain that ends at the announced
		// head (audit H2): a host can serve each reader a chain of its own,
		// every one valid from the real start, and hold a reader's
		// contribution in theirs alone.
		ending := ""
		if len(announced) == 1 {
			if latest := t.Latest(); latest != hex.EncodeToString(announced[0][:]) {
				return fmt.Errorf("the chain checked ends at sha256 %s, not at the announced head %x: it is not the chain announced, whatever it holds", latest, announced[0])
			}
			ending = ", the announced head"
		} else if len(receipts) > 0 {
			fmt.Fprintln(w, "Without -head, this checks the chain this host served, which may not be the one announced: give -head <the head announced at the close>.")
		}
		var missing []string
		for _, r := range receipts {
			h := hex.EncodeToString(r[:])
			e, ok := t.Find(h)
			if !ok {
				missing = append(missing, h)
				continue
			}
			fmt.Fprintf(w, "Contribution %d is sha256 %s, by %q, accepted %s.\n", e.Index, h, e.Participant, e.At.Format(time.RFC3339))
		}
		if len(missing) > 0 {
			return fmt.Errorf("not in the chain checked: %s", strings.Join(missing, ", "))
		}
		if err != nil {
			return fmt.Errorf("NOT VERIFIED: %d contributions, each a valid step on top of the one before it, but %w", len(t.Contributions), err)
		}
		fmt.Fprintf(w, "OK: %s: the start, and %d contributions, each a valid step on top of the one before it; latest sha256 %s%s\n",
			phaseOf(start), len(t.Contributions), t.Latest(), ending)
		return nil
	}
}

// staticClient is the HTTP client for a static copy of a phase: the
// program's own (participant.HTTPClient), but following redirects over
// HTTPS (followHTTPS).
func staticClient() *http.Client {
	c := participant.HTTPClient()
	c.CheckRedirect = followHTTPS
	return c
}

// followHTTPS lets a static host redirect, as release downloads and object
// storage do, over HTTPS only and at most 10 times. A coordinator never
// redirects (participant.HTTPClient refuses it); the copy's files are
// checked against their sha256s wherever they come from.
func followHTTPS(req *http.Request, via []*http.Request) error {
	if req.URL.Scheme != "https" {
		return fmt.Errorf("a redirect to %s, which is not over HTTPS", req.URL.Redacted())
	}
	if len(via) >= 10 {
		return errors.New("more than 10 redirects")
	}
	return nil
}

// phaseFlags are the flags with which verify checks the phase that start
// begins, as announced: its domain, or its protocol, SRS and circuits.
func phaseFlags(start ceremony.Info) string {
	if start.Kind == ceremony.KindPhase2 {
		return fmt.Sprintf("-ceremony %s -srs <the SRS> -srs-sha256 %x -r1cs <r1cs dir>", start.Ceremony, start.SRS)
	}
	return fmt.Sprintf("-size %d", start.N)
}

// phaseOf names the phase that start is the start of.
func phaseOf(start ceremony.Info) string {
	if start.Kind == ceremony.KindPhase2 {
		return fmt.Sprintf("%s's Phase 2, on the SRS of sha256 %x", start.Ceremony, start.SRS)
	}
	return fmt.Sprintf("Phase 1 for circuits of up to %d constraints", start.N)
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
	githubSecretFile := fs.String("github-client-secret-file", "", "-auth github: a file holding the app's client secret, with which the coordinator admits only the app's own tokens")
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
	srsIn := srsFlags(fs, false) // Phase 2: the SRS it started from, to check state-0 before serving (with -r1cs)
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
		// The secret is read from a file, never given on the command line,
		// where any user of the machine could read it.
		var githubSecret string
		if *githubSecretFile != "" {
			data, err := os.ReadFile(*githubSecretFile)
			if err != nil {
				return err
			}
			githubSecret = strings.TrimSpace(string(data))
		}
		if coordinator.Auth(*auth) == coordinator.AuthGitHub && githubSecret == "" {
			return fmt.Errorf("%w: -auth github takes -github-client-secret-file, a file holding the OAuth app's client secret", errUsage)
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
		// anyone does (audit R-060), from the SRS announced (H1).
		if srsIn.given() != (*r1cs != "") {
			return fmt.Errorf("%w: -srs goes with -r1cs", errUsage)
		}
		var (
			p2     *transcript.Phase2Start
			phase1 *ceremony.Provenance
		)
		if srsIn.given() {
			circuits, err := circuitsIn(w)
			if err != nil {
				return err
			}
			srs, info, err := srsIn.read(w)
			if err != nil {
				return err
			}
			p2, phase1 = &transcript.Phase2Start{SRS: srs, Circuits: circuits}, info.Phase1
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
			GitHubClientID: *githubClientID, GitHubClientSecret: githubSecret, GitHubMinAge: *githubMinAge,
			MaxPerAddress: *maxPerAddress, NoAddressLimit: *noAddressLimit, TrustedProxies: proxies,
			IPv6Prefix: *ipv6Prefix, MaxQueue: *maxQueue, MaxTurns: *maxTurns,
			MaxContributions: *maxContributions, MaxContributionsPerAddress: *maxContributionsPerAddress,
			MinFree: *minFreeMiB << 20, MaxSessions: *maxSessions,
			Turn: *turn, TurnPresence: *turnPresence, Presence: *presence, Away: *away, Poll: *poll,
			CloseAt: closes, Notice: *notice, ReleaseURL: *releaseURL, Phase1: phase1, Log: logger,
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
			logger.Printf("phase in %s: %d contributions so far; participants are admitted %s; interrupt to stop",
				*dir, len(c.States()), *auth)
		}

		sigs := make(chan os.Signal, 2)
		stopSignals(sigs)
		defer signal.Stop(sigs)
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		interrupted := false
	serving: // until an interrupt, or the scheduled close
		for {
			select {
			case err := <-served:
				return err
			case <-sigs:
				interrupted = true
				break serving
			case <-tick.C:
				if !closedAtStart && c.Closed() {
					break serving
				}
			}
		}
		// An interrupt stops the coordinator, to restart it or to end the
		// phase by hand, and the phase is not closed: queued participants
		// keep waiting, and their programs join the coordinator started next.
		if interrupted {
			c.Stop()
			logger.Printf("stopping: no new turns; waiting for the one under way (interrupt again to stop now)")
		} else {
			logger.Printf("waiting for the turn under way, if any (interrupt to stop now)") // advance logged the close
		}
		busy, hurried := c.Busy(), false
	wait:
		for c.Busy() {
			select {
			case <-sigs:
				hurried = true
				break wait
			case <-tick.C:
			}
		}
		// The program whose upload ended the last turn reads the transcript
		// back after its reply: answered still, read-only, for a moment
		// (audit L5), when a turn was under way or a contribution was just
		// accepted, unless told to stop at once.
		recent := false
		if t, err := transcript.Load(*dir); err == nil && len(t.Contributions) > 0 {
			recent = time.Since(t.Contributions[len(t.Contributions)-1].At) < readGrace
		}
		if !hurried && (busy || recent) {
			logger.Printf("serving the transcript, read-only, for %s (interrupt to stop now)", readGrace)
			select {
			case <-sigs:
			case <-time.After(readGrace):
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
		start, err := ceremony.Inspect(transcript.Path(*dir, 0))
		if err != nil {
			return err
		}
		if c.Closed() {
			fmt.Fprintf(w, "Closed after %d contributions. Announce the head now, before the beacon round is out:\n", len(t.Contributions))
		} else {
			fmt.Fprintf(w, "Stopped after %d contributions. The phase is not closed: started again, it carries on.\n"+
				"If this stop ends it, announce the head now, before the beacon round is out:\n", len(t.Contributions))
		}
		fmt.Fprintf(w, "  head sha256 %s (contribution %d)\n"+
			"Before you announce it, check that every contributor you lined up for this phase is in this chain (audit H2, H3):\n"+
			"  ceremony verify -dir %s -head %s %s -mine <their published receipts, comma-separated>\n"+
			"If one is missing, announce nothing and seal nothing: see the README, on running a ceremony.\n"+
			"Then seal, with the round you announced and their receipts:\n"+
			"  ceremony %s -transcript %s -head %s -include <their receipts> -drand <round> -close-at <the close>\n",
			t.Latest(), len(t.Contributions), *dir, t.Latest(), phaseFlags(start), sealCommand(start), *dir, t.Latest())
		return nil
	}
}

// readGrace is how long coordinate answers, read-only, after its last turn,
// for the program that made it to read the transcript back.
var readGrace = 30 * time.Second

// sealCommand is the seal command for the phase that start begins, as
// coordinate prints it at the end, before -transcript and the rest: a
// Phase 2's names the SRS by the sha256 its start records, the one it was
// served from (audit H1).
func sealCommand(start ceremony.Info) string {
	if start.Kind == ceremony.KindPhase2 {
		return fmt.Sprintf("phase2-seal -ceremony %s -srs <the SRS> -srs-sha256 %x -r1cs <r1cs dir> -keys <keys dir>", start.Ceremony, start.SRS)
	}
	return "phase1-seal -out <the SRS to write>"
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

// stopSignals has c told of the signals that stop a coordinator: Ctrl-C, or
// a systemd stop. Tests send their own.
var stopSignals = func(c chan<- os.Signal) { signal.Notify(c, os.Interrupt, syscall.SIGTERM) }

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
