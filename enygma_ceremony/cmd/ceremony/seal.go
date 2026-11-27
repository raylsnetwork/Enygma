package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"enygma_ceremony/ceremony"
	"enygma_ceremony/coordinator"
	"enygma_ceremony/drand"
	"enygma_ceremony/transcript"
)

// circuitsFlags declares -r1cs and -circuits and returns the -r1cs directory
// and what lists the circuits in it (audit R-036). Its key manifest
// (keys.json) names them, or else -circuits must, and every .r1cs file there
// must be one of them: a stray file never becomes a circuit of the keys. It
// prints each circuit, with its constraint system's sha256, to w.
func circuitsFlags(fs *flag.FlagSet) (*string, func(w io.Writer) ([]ceremony.Circuit, error)) {
	r1cs := fs.String("r1cs", "", "the directory of the protocol's <Name>.r1cs files")
	names := fs.String("circuits", "", "the circuits, by name, comma-separated: needed when the -r1cs directory has no keys.json")
	return r1cs, func(w io.Writer) ([]ceremony.Circuit, error) {
		all, err := ceremony.CircuitsIn(*r1cs)
		if err != nil {
			return nil, err
		}
		_, err = os.Stat(filepath.Join(*r1cs, ceremony.ManifestFile))
		manifest := err == nil
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if *names == "" && !manifest {
			return nil, fmt.Errorf("%w: %s has no %s, so name its circuits with -circuits", errUsage, *r1cs, ceremony.ManifestFile)
		}
		if *names != "" {
			want := strings.Split(*names, ",")
			slices.Sort(want)
			var have []string
			for _, c := range all {
				have = append(have, c.Name)
			}
			if !slices.Equal(want, have) {
				return nil, fmt.Errorf("-circuits names %s, but %s holds %s: they must be the same circuits",
					strings.Join(want, ", "), *r1cs, strings.Join(have, ", "))
			}
		}
		for _, c := range all {
			h, _, err := transcript.HashFile(c.R1CS)
			if err != nil {
				return nil, err
			}
			fmt.Fprintf(w, "circuit %-24s %s, sha256 %x\n", c.Name, c.R1CS, h)
		}
		return all, nil
	}
}

// chain lists a seal's contributions, in order.
type chain struct {
	paths   []string
	head    string     // the sha256 of the last, as announced
	include [][32]byte // receipts that must be among them (-include)
	last    time.Time  // when the last was accepted, as its entry says; zero for files given by hand
}

// sealed is c as the library seals it: up to its head, with its receipts.
func (c chain) sealed() ceremony.Chain {
	sc := ceremony.Chain{Paths: c.paths, Include: c.include}
	_, _ = hex.Decode(sc.Head[:], []byte(c.head)) // checked by chainFlags
	return sc
}

// included is c's -include, in hex, for a seal's record.
func (c chain) included() []string {
	var out []string
	for _, r := range c.include {
		out = append(out, hex.EncodeToString(r[:]))
	}
	return out
}

// chainFlags declares a seal's -transcript and -head and returns what reads
// the chain to seal (audit R-003): the contributions of the transcript in
// -transcript, or the files given, in order, which must end at -head, the
// head announced at the close. A seal of a chain cut short would leave out
// every contribution after its end.
func chainFlags(fs *flag.FlagSet) func(args []string, w io.Writer) (chain, error) {
	dir := fs.String("transcript", "", "the phase's transcript directory (as verify downloads it): its contributions, in order")
	head := fs.String("head", "", "the sha256 of the phase's last contribution, as announced at the close: what is sealed must end there")
	include := fs.String("include", "", "the receipts (sha256s, comma-separated) of contributors lined up for the phase, as they published them: each must be in the chain sealed")
	return func(args []string, w io.Writer) (chain, error) {
		h := strings.ToLower(*head)
		if b, err := hex.DecodeString(h); err != nil || len(b) != 32 {
			return chain{}, fmt.Errorf("%w: -head is the sha256 of the last contribution, as announced at the close", errUsage)
		}
		if (*dir == "") == (len(args) == 0) {
			return chain{}, fmt.Errorf("%w: give -transcript, or the contributions in order, not both", errUsage)
		}
		receipts, err := sha256List(*include)
		if err != nil {
			return chain{}, fmt.Errorf("%w: -include: %v", errUsage, err)
		}
		var c chain
		if *dir != "" {
			c, err = loadChain(*dir, h, w)
		} else {
			c, err = checkChain(chain{paths: args, head: h}, w)
		}
		c.include = receipts
		if err == nil {
			err = earlyIncluded(c)
		}
		if err == nil && len(receipts) == 0 {
			fmt.Fprintln(w, "No -include: nothing checks that the contributors lined up for this phase are in the chain sealed. "+
				"A chain without them is one nobody you trust contributed to.")
		}
		return c, err
	}
}

// earlyIncluded finds each of c's -include receipts among its files' sha256s
// before a seal fetches its beacon and replays them, which takes hours for
// a large phase: a receipt mistyped, or missing, is said at once. The seal
// checks again on the bytes it replays.
func earlyIncluded(c chain) error {
	if len(c.include) == 0 {
		return nil
	}
	have := map[[32]byte]bool{}
	for _, p := range c.paths {
		h, _, err := transcript.HashFile(p)
		if err != nil {
			return err
		}
		have[h] = true
	}
	for _, r := range c.include {
		if !have[r] {
			return fmt.Errorf("%w: sha256 %x is not one of the %d contributions up to the head", ceremony.ErrNotIncluded, r, len(c.paths))
		}
	}
	return nil
}

// sha256List reads sha256s in hex, comma-separated; "" is none.
func sha256List(s string) ([][32]byte, error) {
	var out [][32]byte
	for _, f := range strings.Split(s, ",") {
		if f = strings.TrimSpace(f); f == "" {
			continue
		}
		var h [32]byte
		b, err := hex.DecodeString(f)
		if err != nil || len(b) != len(h) {
			return nil, fmt.Errorf("%q is not a sha256", f)
		}
		copy(h[:], b)
		out = append(out, h)
	}
	return out, nil
}

// loadChain lists the contributions of the transcript in dir, which must end
// at head.
func loadChain(dir, head string, w io.Writer) (chain, error) {
	t, err := transcript.Load(dir)
	if err != nil {
		return chain{}, err
	}
	c := chain{head: head}
	for _, e := range t.Contributions {
		c.paths = append(c.paths, transcript.Path(dir, e.Index))
		c.last = e.At
	}
	return checkChain(c, w)
}

// checkChain checks that c ends at its head.
func checkChain(c chain, w io.Writer) (chain, error) {
	if len(c.paths) == 0 {
		return chain{}, errors.New("no contributions to seal")
	}
	got, _, err := transcript.HashFile(c.paths[len(c.paths)-1])
	if err != nil {
		return chain{}, err
	}
	if hex.EncodeToString(got[:]) != c.head {
		return chain{}, fmt.Errorf("the chain ends at sha256 %x, not the announced head %s: sealing it would leave contributions out", got, c.head)
	}
	fmt.Fprintf(w, "Sealing %d contributions, ending at the announced head %s.\n", len(c.paths), c.head)
	return c, nil
}

// beaconRecord is how a seal's beacon was chosen.
type beaconRecord struct {
	DrandRound uint64    `json:"drand_round,omitzero"`
	DrandAt    time.Time `json:"drand_at,omitzero"`
	CloseAt    time.Time `json:"close_at,omitzero"`
	Unverified bool      `json:"unverified,omitzero"` // chosen by whoever sealed: keys fit for tests only
	Beacon     string    `json:"beacon"`              // hex
}

// beaconChoice is a seal's beacon flags.
type beaconChoice struct {
	round      *uint64
	closeAt    *string
	unverified *string
}

// beaconFlags declares a seal's beacon flags (audit R-005): -drand, a drand
// round announced before the close, which must come out after both -close-at
// and the last contribution, and is fetched and checked against drand's key;
// or, for tests and rehearsals only, -unverified-beacon, which nothing checks,
// said loudly and recorded as such. A beacon whoever seals could choose, or
// one known before the last contribution, would let them pick the keys.
func beaconFlags(fs *flag.FlagSet) *beaconChoice {
	return &beaconChoice{
		round:      fs.Uint64("drand", 0, "the drand quicknet round announced as the beacon: it must come out after -close-at and after the last contribution"),
		closeAt:    fs.String("close-at", "", "-drand: when the phase closed, as announced, RFC 3339"),
		unverified: fs.String("unverified-beacon", "", "a beacon in hex that nothing checks, for tests and rehearsals only: the seal's record says so"),
	}
}

// usage checks that the beacon is given one way, as a call's own error.
func (b *beaconChoice) usage() error {
	switch {
	case (*b.unverified == "") == (*b.round == 0):
		return fmt.Errorf("%w: give the beacon as -drand (with -close-at) or -unverified-beacon, not both", errUsage)
	case *b.round != 0 && *b.closeAt == "":
		return fmt.Errorf("%w: -drand takes -close-at, the close as announced", errUsage)
	}
	return nil
}

// read returns the beacon, after a chain whose last contribution was
// accepted at last (zero when unknown).
func (b *beaconChoice) read(w io.Writer, last time.Time) (ceremony.Beacon, beaconRecord, error) {
	if err := b.usage(); err != nil {
		return ceremony.Beacon{}, beaconRecord{}, err
	}
	if *b.round == 0 {
		beacon, err := hex.DecodeString(*b.unverified)
		if err != nil {
			return ceremony.Beacon{}, beaconRecord{}, fmt.Errorf("%w: -unverified-beacon: %v", errUsage, err)
		}
		return unverifiedBeacon(w, beacon)
	}
	closes, err := time.Parse(time.RFC3339, *b.closeAt)
	if err != nil {
		return ceremony.Beacon{}, beaconRecord{}, fmt.Errorf("%w: -close-at: %v", errUsage, err)
	}
	return drandBeacon(w, *b.round, closes, last)
}

// unverifiedBeacon is beacon, which nothing checks, said loudly.
func unverifiedBeacon(w io.Writer, beacon []byte) (ceremony.Beacon, beaconRecord, error) {
	if len(beacon) < ceremony.MinBeacon {
		return ceremony.Beacon{}, beaconRecord{}, fmt.Errorf("%w: %d bytes", ceremony.ErrBeacon, len(beacon))
	}
	fmt.Fprintln(w, "WARNING: -unverified-beacon: nothing checks this beacon, so whoever chose it may have chosen the keys."+
		" They are fit for tests and rehearsals only, and their record says so.")
	return ceremony.Beacon{Value: beacon}, beaconRecord{Unverified: true, Beacon: hex.EncodeToString(beacon)}, nil
}

// drandBeacon fetches drand quicknet's round, checked against its key, for
// a phase that closed at closes and whose last contribution was accepted at
// last (zero when unknown): the round must come out after both.
func drandBeacon(w io.Writer, round uint64, closes, last time.Time) (ceremony.Beacon, beaconRecord, error) {
	out := drand.Time(round)
	if !out.After(closes) {
		return ceremony.Beacon{}, beaconRecord{}, fmt.Errorf("drand round %d came out at %s, not after the close at %s: "+
			"a beacon known before the close could have been chosen", round, out.UTC().Format(time.RFC3339), closes.UTC().Format(time.RFC3339))
	}
	if !out.After(last) {
		return ceremony.Beacon{}, beaconRecord{}, fmt.Errorf("drand round %d came out at %s, not after the last contribution, accepted at %s",
			round, out.UTC().Format(time.RFC3339), last.UTC().Format(time.RFC3339))
	}
	beacon, err := drand.Fetch(context.Background(), &http.Client{Timeout: 30 * time.Second}, drand.Relays, round)
	if err != nil {
		return ceremony.Beacon{}, beaconRecord{}, err
	}
	fmt.Fprintf(w, "Beacon: drand quicknet round %d, out at %s, %x\n", round, out.UTC().Format(time.RFC3339), beacon)
	return ceremony.Beacon{Value: beacon[:], DrandRound: round, CloseAt: closes.UTC()}, beaconRecord{
		DrandRound: round, DrandAt: out.UTC(), CloseAt: closes.UTC(), Beacon: hex.EncodeToString(beacon[:]),
	}, nil
}

// srsChoice is a Phase 2 command's SRS flags.
type srsChoice struct {
	path, sha256, phase1, phase1Head *string
	rehearsal                        *bool
}

// srsFlags declares a Phase 2 command's SRS flags (audit H1): -srs, the SRS
// file, and -srs-sha256, the sha256 announced for it, which the file must
// have: any file decodes as an SRS, including one made from a τ, α and β
// someone kept, and a Phase 2 is only as sound as the Phase 1 its SRS seals.
// With reseal, -phase1 may stand in for -srs-sha256: the Phase 1 transcript
// to reseal as the SRS says it was sealed, which must give the same file,
// with -phase1-head, Phase 1's head as announced, which the SRS must say it
// was sealed up to: any transcript reseals to the SRS sealed from it, and
// the one that counts is the announced one. -rehearsal takes a rehearsal's
// SRS, sealed with a beacon not from drand.
func srsFlags(fs *flag.FlagSet, reseal bool) *srsChoice {
	c := &srsChoice{
		path:      fs.String("srs", "", "the SRS Phase 2 starts from, as phase1-seal wrote it"),
		sha256:    fs.String("srs-sha256", "", "the SRS's sha256, as announced: the SRS must be that file"),
		rehearsal: fs.Bool("rehearsal", false, "take a rehearsal's SRS, sealed with a beacon not from drand (-unverified-beacon)"),
	}
	if reseal {
		c.phase1 = fs.String("phase1", "", "in place of -srs-sha256: Phase 1's transcript directory, resealed as the SRS says it was sealed; the SRS must be that seal")
		c.phase1Head = fs.String("phase1-head", "", "with -phase1: Phase 1's head as announced, which the SRS must have been sealed up to")
	}
	return c
}

// given reports whether -srs was given.
func (c *srsChoice) given() bool { return *c.path != "" }

// read returns the SRS, pinned to the sha256 announced for it or to the
// seal of the Phase 1 transcript in -phase1, and describes it to w.
func (c *srsChoice) read(w io.Writer) (ceremony.SRS, ceremony.Info, error) {
	phase1, phase1Head := "", ""
	if c.phase1 != nil {
		phase1, phase1Head = *c.phase1, strings.ToLower(*c.phase1Head)
	}
	if *c.path == "" || (*c.sha256 == "") == (phase1 == "") || (phase1 == "") != (phase1Head == "") {
		if c.phase1 != nil {
			return ceremony.SRS{}, ceremony.Info{}, fmt.Errorf("%w: give -srs with its sha256 as announced (-srs-sha256), or with Phase 1's transcript to reseal and its head as announced (-phase1, -phase1-head)", errUsage)
		}
		return ceremony.SRS{}, ceremony.Info{}, fmt.Errorf("%w: give -srs with its sha256 as announced (-srs-sha256)", errUsage)
	}
	info, err := ceremony.Inspect(*c.path)
	if err != nil {
		return ceremony.SRS{}, ceremony.Info{}, err
	}
	if info.Kind != ceremony.KindSRS {
		return ceremony.SRS{}, ceremony.Info{}, fmt.Errorf("%s holds a %s, not an SRS", *c.path, info.Kind)
	}
	fmt.Fprintf(w, "SRS %s: sha256 %x\n  %s\n", *c.path, info.Hash, sealedFrom(*info.Phase1))
	srs := ceremony.SRS{Path: *c.path, Rehearsal: *c.rehearsal}
	if phase1 == "" {
		b, err := hex.DecodeString(*c.sha256)
		if err != nil || len(b) != len(srs.SHA256) {
			return ceremony.SRS{}, ceremony.Info{}, fmt.Errorf("%w: -srs-sha256 is the SRS's sha256, as announced", errUsage)
		}
		copy(srs.SHA256[:], b) // the Phase 2 starts only from a file of this sha256, which start checks on the bytes it reads
		if srs.SHA256 != info.Hash {
			// Said now, before a seal loads its chain and fetches its beacon.
			return ceremony.SRS{}, ceremony.Info{}, fmt.Errorf("%w: %s has sha256 %x, and the SRS announced %x", ceremony.ErrSRS, *c.path, info.Hash, srs.SHA256)
		}
		return srs, info, nil
	}
	if head := hex.EncodeToString(info.Phase1.Head[:]); head != phase1Head {
		return ceremony.SRS{}, ceremony.Info{}, fmt.Errorf("%w: the SRS says it was sealed up to head %s, not the announced %s", ceremony.ErrSRS, head, phase1Head)
	}
	resealed, err := resealPhase1(w, phase1, *info.Phase1)
	if err != nil {
		return ceremony.SRS{}, ceremony.Info{}, err
	}
	if resealed != info.Hash {
		return ceremony.SRS{}, ceremony.Info{}, fmt.Errorf("%w: Phase 1 in %s, resealed as the SRS says it was sealed, gives sha256 %x, not the SRS's %x: the SRS is not that Phase 1's seal",
			ceremony.ErrSRS, phase1, resealed, info.Hash)
	}
	fmt.Fprintf(w, "Resealed Phase 1 from %s: the same SRS.\n", phase1)
	srs.SHA256 = info.Hash
	return srs, info, nil
}

// sealedFrom says in a line what an SRS was sealed from.
func sealedFrom(p ceremony.Provenance) string {
	beacon := "a beacon not from drand: a rehearsal's"
	if p.Beacon.DrandRound != 0 {
		beacon = fmt.Sprintf("drand round %d, for the close at %s", p.Beacon.DrandRound, p.Beacon.CloseAt.Format(time.RFC3339))
	}
	return fmt.Sprintf("the seal of Phase 1 up to head %x (%d contributions), with %s", p.Head, p.Contributions, beacon)
}

// resealPhase1 reseals the Phase 1 transcript in dir as p says an SRS was
// sealed: up to its head, with its beacon (drand's round, fetched and
// checked again, or a rehearsal's, which the Phase 2 then refuses unless
// told), and returns the sha256 of the SRS it gets.
func resealPhase1(w io.Writer, dir string, p ceremony.Provenance) ([32]byte, error) {
	c, err := loadChain(dir, hex.EncodeToString(p.Head[:]), w)
	if err != nil {
		return [32]byte{}, err
	}
	beacon := ceremony.Beacon{Value: p.Beacon.Value}
	if p.Beacon.DrandRound != 0 {
		if beacon, _, err = drandBeacon(w, p.Beacon.DrandRound, p.Beacon.CloseAt, c.last); err != nil {
			return [32]byte{}, err
		}
	} else {
		fmt.Fprintln(w, "The SRS was sealed with a beacon not from drand, which whoever sealed it chose: a rehearsal's.")
	}
	tmp, err := os.MkdirTemp("", "ceremony-phase1-")
	if err != nil {
		return [32]byte{}, err
	}
	r, err := ceremony.SealPhase1(filepath.Join(tmp, "srs"), beacon, c.sealed())
	return r.Hash, errors.Join(err, os.RemoveAll(tmp))
}

// sealRecord is what a seal writes beside what it sealed: which chain, which
// beacon, which circuits and keys, so that anyone resealing can compare.
type sealRecord struct {
	Kind          string          `json:"kind"` // srs or keys
	Ceremony      string          `json:"ceremony,omitzero"`
	Head          string          `json:"head"` // sha256 of the last contribution sealed
	Contributions int             `json:"contributions"`
	Beacon        beaconRecord    `json:"beacon"`
	SRS           string          `json:"srs_sha256"`        // Phase 1: the SRS written; Phase 2: the SRS it started from
	Phase1        *phase1Record   `json:"phase1,omitzero"`   // Phase 2: what that SRS was sealed from (audit H1)
	Included      []string        `json:"included,omitzero"` // receipts found in the chain sealed (-include)
	Circuits      []circuitRecord `json:"circuits,omitzero"`
	SealedBy      string          `json:"sealed_by,omitzero"` // the commit of the program that sealed
}

// phase1Record is what a Phase 2's SRS was sealed from, as the SRS says.
type phase1Record struct {
	Head          string    `json:"head"`
	Contributions uint32    `json:"contributions"`
	DrandRound    uint64    `json:"drand_round,omitzero"`
	CloseAt       time.Time `json:"close_at,omitzero"`
	Unverified    bool      `json:"unverified,omitzero"` // a rehearsal's: its beacon not from drand
}

// circuitRecord is one circuit of a sealed Phase 2: its constraint system's
// sha256, and its keys' paths and sha256s.
type circuitRecord struct {
	Name   string `json:"name"`
	R1CS   string `json:"r1cs_sha256"`
	PK     string `json:"pk"`
	PKHash string `json:"pk_sha256"`
	VK     string `json:"vk"`
	VKHash string `json:"vk_sha256"`
}

// writeRecord writes r as JSON at path, which must not exist.
func writeRecord(path string, r sealRecord) error {
	r.SealedBy = coordinator.Version()
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	_, err = f.Write(append(data, '\n'))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// hashHex is the sha256, in hex, of the file at path.
func hashHex(path string) (string, error) {
	h, _, err := transcript.HashFile(path)
	return hex.EncodeToString(h[:]), err
}
