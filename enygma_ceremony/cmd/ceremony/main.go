// Command ceremony runs Enygma's multi-party Groth16 setup (package
// enygma_ceremony/ceremony). Run with no arguments, a build made for a
// ceremony (make ceremony-dist CEREMONY_URL=…) joins that ceremony's
// coordinator and contributes: all a participant does. The coordinator runs
// the rest, and anyone can check the transcript and rerun the seals.
package main

import (
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"enygma_ceremony/ceremony"
	"enygma_ceremony/internal/termsafe"
)

const usage = `usage: ceremony [<command> [flags]]

participant
  (no command)                            join the ceremony this program was built for, and contribute
  join        [-url URL] [-name NAME] [-mine SHA256]
                                          join a coordinator, contribute on your turn, upload
  contribute  -in STATE -out STATE        add your randomness to a state handed to you by hand

anyone
  verify      [-url URL [-sync]] -dir DIR [-mine SHA256] [-srs SRS -r1cs DIR]
                                          check a phase's transcript: downloaded into DIR, or DIR as it is
  verify-keys -ceremony NAME -srs SRS -r1cs DIR -keys DIR -phase1-seal-sha256 SHA256 -phase2-seal-sha256 SHA256
                                          check installed keys against independently approved seals
  check       -prev STATE -next STATE     verify one contribution
  info        STATE                       describe a state file
  version                                 the commit this program was built from

coordinator
  coordinate  -dir DIR [-listen ADDR | -acme-domain HOST] …
                                          run a phase from DIR/state-0 for anyone with its address
                                          (ceremony coordinate -h)
  phase1-new  -size N -out STATE          start Phase 1 for circuits of up to N constraints
  phase2-new  -ceremony NAME -srs SRS -r1cs DIR [-circuits A,B] -out STATE
                                          start Phase 2 for DIR's circuits (keys.json, or -circuits)
  beacon      -close-at TIME [-turn D] | -round N
                                          the drand round to announce for a close, or round N's beacon
  phase1-seal -head SHA256 (-transcript DIR | STATE...) BEACON -out SRS
                                          replay Phase 1 up to the announced head, apply the beacon,
                                          write the SRS and its record
  phase2-seal -ceremony NAME -srs SRS -r1cs DIR [-circuits A,B] -head SHA256 (-transcript DIR | STATE...)
              BEACON -keys DIR            replay Phase 2 up to the announced head, apply the beacon,
                                          write the keys and their record

  BEACON is -drand ROUND -close-at TIME (the round announced, out after the close and the last
  contribution), or, for tests and rehearsals only, -unverified-beacon HEX.
`

// defaultURL is the coordinator this program joins when run with no
// command, set when it is built for a ceremony:
//
//	go build -ldflags "-X main.defaultURL=https://ceremony.example.org" ./cmd/ceremony
var defaultURL string

func main() {
	disableCoreDumps()
	if isChild() {
		os.Exit(runChild(os.Stdin, os.Stdout, os.Stderr)) // stdout carries the contribution, and stderr JSON, to the parent
	}
	// Everything a command prints goes through termsafe's writer: text a
	// coordinator, a transcript's host or a state chose may reach any line,
	// and none of it may act on the terminal (audit M1 to M3).
	os.Exit(run(os.Args[1:], termsafe.NewWriter(os.Stdout), termsafe.NewWriter(os.Stderr)))
}

// errUsage marks an error in how a command was called, not in what it did.
var errUsage = errors.New("usage")

// command declares a command's flags on fs and returns its body, which takes
// the arguments left after the flags.
type command func(fs *flag.FlagSet) func(args []string, out io.Writer) error

// commands are the commands by name.
var commands = map[string]command{
	"join":        join,
	"verify":      verify,
	"version":     versionCmd,
	"coordinate":  coordinate,
	"beacon":      beacon,
	"contribute":  contribute,
	"phase1-new":  phase1New,
	"check":       check,
	"phase1-seal": phase1Seal,
	"phase2-new":  phase2New,
	"phase2-seal": phase2Seal,
	"info":        info,
	"verify-keys": verifyKeys,
}

// run executes one command and returns its exit code: 0 done, 1 failed, 2 called wrongly.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		if defaultURL == "" {
			fmt.Fprint(stderr, usage)
			return 2
		}
		return joinDefault(stdout, stderr)
	}
	cmd, ok := commands[args[0]]
	if !ok {
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return 2
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(stderr)
	body := cmd(fs)
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if err := body(fs.Args(), stdout); err != nil {
		printError(stderr, "ceremony "+args[0], err)
		if errors.Is(err, errUsage) {
			fs.Usage()
			return 2
		}
		return 1
	}
	return 0
}

// printError prints a command's error on w, after what: as one line, and
// not an endless one, since an error can carry a server's words, which must
// not start a line that looks like this program's (audit M1).
func printError(w io.Writer, what string, err error) {
	fmt.Fprintf(w, "%s: %s\n", what, termsafe.Short(err.Error(), 1000))
}

// contribute adds this machine's randomness to a state handed over by hand.
func contribute(fs *flag.FlagSet) func([]string, io.Writer) error {
	in := fs.String("in", "", "the latest state, as the coordinator hands it to you")
	out := fs.String("out", "", "where to write your contribution (must not exist)")
	return func(args []string, w io.Writer) error {
		if *in == "" || *out == "" || len(args) != 0 {
			return fmt.Errorf("%w: contribute takes -in and -out", errUsage)
		}
		fmt.Fprintln(w, "Contributing. Your randomness comes from this machine's operating system and is gone when this exits.")
		r, err := ceremony.Contribute(*in, *out)
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "Wrote %s to %s\n", describe(r.Info), *out)
		fmt.Fprintf(w, "  on top of sha256 %x\n", r.In)
		fmt.Fprintf(w, "  your contribution sha256 %x  <- publish this\n", r.Hash)
		return nil
	}
}

// phase1New writes the start of Phase 1.
func phase1New(fs *flag.FlagSet) func([]string, io.Writer) error {
	size := fs.Uint64("size", 0, "domain size: a power of two from 2 to 2^28 (268435456), at least the largest circuit's constraint count")
	out := fs.String("out", "", "where to write the start (must not exist)")
	return func(args []string, w io.Writer) error {
		if *size == 0 || *out == "" || len(args) != 0 {
			return fmt.Errorf("%w: phase1-new takes -size and -out", errUsage)
		}
		r, err := ceremony.NewPhase1(*out, *size)
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "Wrote %s to %s\n  sha256 %x\n", describe(r.Info), *out, r.Hash)
		return nil
	}
}

// check verifies one contribution.
func check(fs *flag.FlagSet) func([]string, io.Writer) error {
	prev := fs.String("prev", "", "the state contributed to")
	next := fs.String("next", "", "the contribution")
	return func(args []string, w io.Writer) error {
		if *prev == "" || *next == "" || len(args) != 0 {
			return fmt.Errorf("%w: check takes -prev and -next", errUsage)
		}
		if err := ceremony.Check(*prev, *next); err != nil {
			return err
		}
		// One step, not a transcript: prev itself, and how it was reached, are
		// not checked here (audit R-034); verify checks a phase from its start.
		fmt.Fprintf(w, "OK: %s is a valid step from %s (%s itself not checked: verify checks a whole phase)\n", *next, *prev, *prev)
		return nil
	}
}

// phase1Seal replays Phase 1 and writes the SRS, and beside it a record of
// what it sealed (<out>.json).
func phase1Seal(fs *flag.FlagSet) func([]string, io.Writer) error {
	chain := chainFlags(fs)
	beacon := beaconFlags(fs)
	out := fs.String("out", "", "where to write the SRS (must not exist; the record goes to <out>.json)")
	return func(args []string, w io.Writer) error {
		if *out == "" {
			return fmt.Errorf("%w: phase1-seal takes -head, -transcript or the contributions in order, the beacon and -out", errUsage)
		}
		if err := beacon.usage(); err != nil {
			return err
		}
		c, err := chain(args, w)
		if err != nil {
			return err
		}
		b, rec, err := beacon.read(w, c.last)
		if err != nil {
			return err
		}
		r, err := ceremony.SealPhase1(*out, b, c.paths)
		if err != nil {
			return err
		}
		record := sealRecord{Kind: "srs", Head: c.head, Contributions: len(c.paths), Beacon: rec, SRS: hex.EncodeToString(r.Hash[:])}
		if err := writeRecord(*out+".json", record); err != nil {
			return err
		}
		fmt.Fprintf(w, "Wrote %s, from %d contributions, to %s\n  sha256 %x\n  and its record to %s.json\n",
			describe(r.Info), r.Index, *out, r.Hash, *out)
		return nil
	}
}

// phase2New writes the start of a protocol's Phase 2.
func phase2New(fs *flag.FlagSet) func([]string, io.Writer) error {
	name := fs.String("ceremony", "", "the ceremony's name, e.g. enygma_retail_payments")
	srs := fs.String("srs", "", "the SRS phase1-seal wrote")
	r1cs, circuitsIn := circuitsFlags(fs)
	out := fs.String("out", "", "where to write the start (must not exist)")
	return func(args []string, w io.Writer) error {
		if *name == "" || *srs == "" || *r1cs == "" || *out == "" || len(args) != 0 {
			return fmt.Errorf("%w: phase2-new takes -ceremony, -srs, -r1cs and -out", errUsage)
		}
		circuits, err := circuitsIn(w)
		if err != nil {
			return err
		}
		r, err := ceremony.NewPhase2(*out, *name, *srs, circuits)
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "Wrote %s to %s\n  sha256 %x\n", describe(r.Info), *out, r.Hash)
		return nil
	}
}

// phase2Seal replays a protocol's Phase 2 and writes its keys, and a record
// of what it sealed (ceremony-seal.json in the keys directory).
func phase2Seal(fs *flag.FlagSet) func([]string, io.Writer) error {
	name := fs.String("ceremony", "", "the ceremony's name, as phase2-new was given it")
	srs := fs.String("srs", "", "the SRS Phase 2 started from")
	r1cs, circuitsIn := circuitsFlags(fs)
	chain := chainFlags(fs)
	beacon := beaconFlags(fs)
	keys := fs.String("keys", "", "the directory to write the keys into, at their manifest paths or as <Name>PK.key and <Name>VK.key (no key may exist)")
	return func(args []string, w io.Writer) error {
		if *name == "" || *srs == "" || *r1cs == "" || *keys == "" {
			return fmt.Errorf("%w: phase2-seal takes -ceremony, -srs, -r1cs, -head, -transcript or the contributions in order, the beacon and -keys", errUsage)
		}
		if err := beacon.usage(); err != nil {
			return err
		}
		recordPath := filepath.Join(*keys, phase2SealFile)
		if _, err := os.Lstat(recordPath); !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%s: a seal's record is there already, or cannot be checked (%v)", recordPath, err)
		}
		circuits, err := circuitsIn(w)
		if err != nil {
			return err
		}
		c, err := chain(args, w)
		if err != nil {
			return err
		}
		b, rec, err := beacon.read(w, c.last)
		if err != nil {
			return err
		}
		if err := ceremony.SealPhase2(*keys, *name, *srs, circuits, b, c.paths); err != nil {
			return err
		}
		record := sealRecord{Kind: "keys", Ceremony: *name, Head: c.head, Contributions: len(c.paths), Beacon: rec}
		if record.SRS, err = hashHex(*srs); err != nil {
			return err
		}
		fmt.Fprintf(w, "Wrote, from %d contributions, into %s:\n", len(c.paths), *keys)
		for _, ci := range circuits {
			cr := circuitRecord{Name: ci.Name, PK: ci.PK, VK: ci.VK}
			for _, h := range []struct {
				path string
				to   *string
			}{{ci.R1CS, &cr.R1CS}, {filepath.Join(*keys, ci.PK), &cr.PKHash}, {filepath.Join(*keys, ci.VK), &cr.VKHash}} {
				if *h.to, err = hashHex(h.path); err != nil {
					return err
				}
			}
			record.Circuits = append(record.Circuits, cr)
			fmt.Fprintf(w, "  %-24s %s  %s\n", ci.Name, ci.PK, ci.VK)
		}
		if err := writeRecord(recordPath, record); err != nil {
			return err
		}
		fmt.Fprintf(w, "and its record to %s\n", recordPath)
		return nil
	}
}

// info describes a state file.
func info(fs *flag.FlagSet) func([]string, io.Writer) error {
	return func(args []string, w io.Writer) error {
		if len(args) != 1 {
			return fmt.Errorf("%w: info takes one state file", errUsage)
		}
		i, err := ceremony.Inspect(args[0])
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "%s\n  sha256 %x\n", describe(i), i.Hash)
		return nil
	}
}

// describe says in a line what a state file holds. A Phase 2's names come
// from the file, which a coordinator may have made: decoding refuses any
// but plain ones, which no terminal acts on (audit M3).
func describe(i ceremony.Info) string {
	switch i.Kind {
	case ceremony.KindPhase1:
		return fmt.Sprintf("Phase 1 for circuits of up to %d constraints, after %d contributions", i.N, i.Index)
	case ceremony.KindPhase2:
		return fmt.Sprintf("%s Phase 2 (%s), after %d contributions", i.Ceremony, strings.Join(i.Circuits, ", "), i.Index)
	case ceremony.KindSRS:
		return fmt.Sprintf("the SRS for circuits of up to %d constraints", i.N)
	}
	return i.Kind.String()
}
