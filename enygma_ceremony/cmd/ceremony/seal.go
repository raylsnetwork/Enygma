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
	paths []string
	head  string    // the sha256 of the last, as announced
	last  time.Time // when the last was accepted, as its entry says; zero for files given by hand
}

// chainFlags declares a seal's -transcript and -head and returns what reads
// the chain to seal (audit R-003): the contributions of the transcript in
// -transcript, or the files given, in order, which must end at -head, the
// head announced at the close. A seal of a chain cut short would leave out
// every contribution after its end.
func chainFlags(fs *flag.FlagSet) func(args []string, w io.Writer) (chain, error) {
	dir := fs.String("transcript", "", "the phase's transcript directory (as verify downloads it): its contributions, in order")
	head := fs.String("head", "", "the sha256 of the phase's last contribution, as announced at the close: what is sealed must end there")
	return func(args []string, w io.Writer) (chain, error) {
		h := strings.ToLower(*head)
		if b, err := hex.DecodeString(h); err != nil || len(b) != 32 {
			return chain{}, fmt.Errorf("%w: -head is the sha256 of the last contribution, as announced at the close", errUsage)
		}
		if (*dir == "") == (len(args) == 0) {
			return chain{}, fmt.Errorf("%w: give -transcript, or the contributions in order, not both", errUsage)
		}
		c := chain{paths: args, head: h}
		if *dir != "" {
			t, err := transcript.Load(*dir)
			if err != nil {
				return chain{}, err
			}
			c.paths = nil
			for _, e := range t.Contributions {
				c.paths = append(c.paths, transcript.Path(*dir, e.Index))
				c.last = e.At
			}
		}
		if len(c.paths) == 0 {
			return chain{}, errors.New("no contributions to seal")
		}
		got, _, err := transcript.HashFile(c.paths[len(c.paths)-1])
		if err != nil {
			return chain{}, err
		}
		if hex.EncodeToString(got[:]) != h {
			return chain{}, fmt.Errorf("the chain ends at sha256 %x, not the announced head %s: sealing it would leave contributions out", got, h)
		}
		fmt.Fprintf(w, "Sealing %d contributions, ending at the announced head %s.\n", len(c.paths), h)
		return c, nil
	}
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
func (b *beaconChoice) read(w io.Writer, last time.Time) ([]byte, beaconRecord, error) {
	if err := b.usage(); err != nil {
		return nil, beaconRecord{}, err
	}
	if *b.round == 0 {
		beacon, err := hex.DecodeString(*b.unverified)
		if err != nil {
			return nil, beaconRecord{}, fmt.Errorf("%w: -unverified-beacon: %v", errUsage, err)
		}
		if len(beacon) < ceremony.MinBeacon {
			return nil, beaconRecord{}, fmt.Errorf("%w: %d bytes", ceremony.ErrBeacon, len(beacon))
		}
		fmt.Fprintln(w, "WARNING: -unverified-beacon: nothing checks this beacon, so whoever chose it may have chosen the keys."+
			" They are fit for tests and rehearsals only, and their record says so.")
		return beacon, beaconRecord{Unverified: true, Beacon: hex.EncodeToString(beacon)}, nil
	}
	closes, err := time.Parse(time.RFC3339, *b.closeAt)
	if err != nil {
		return nil, beaconRecord{}, fmt.Errorf("%w: -close-at: %v", errUsage, err)
	}
	out := drand.Time(*b.round)
	if !out.After(closes) {
		return nil, beaconRecord{}, fmt.Errorf("drand round %d came out at %s, not after the close at %s: "+
			"a beacon known before the close could have been chosen", *b.round, out.UTC().Format(time.RFC3339), closes.UTC().Format(time.RFC3339))
	}
	if !out.After(last) {
		return nil, beaconRecord{}, fmt.Errorf("drand round %d came out at %s, not after the last contribution, accepted at %s",
			*b.round, out.UTC().Format(time.RFC3339), last.UTC().Format(time.RFC3339))
	}
	beacon, err := drand.Fetch(context.Background(), &http.Client{Timeout: 30 * time.Second}, drand.Relays, *b.round)
	if err != nil {
		return nil, beaconRecord{}, err
	}
	fmt.Fprintf(w, "Beacon: drand quicknet round %d, out at %s, %x\n", *b.round, out.UTC().Format(time.RFC3339), beacon)
	return beacon[:], beaconRecord{
		DrandRound: *b.round, DrandAt: out.UTC(), CloseAt: closes.UTC(), Beacon: hex.EncodeToString(beacon[:]),
	}, nil
}

// sealRecord is what a seal writes beside what it sealed: which chain, which
// beacon, which circuits and keys, so that anyone resealing can compare.
type sealRecord struct {
	Kind          string          `json:"kind"` // srs or keys
	Ceremony      string          `json:"ceremony,omitzero"`
	Head          string          `json:"head"` // sha256 of the last contribution sealed
	Contributions int             `json:"contributions"`
	Beacon        beaconRecord    `json:"beacon"`
	SRS           string          `json:"srs_sha256"` // Phase 1: the SRS written; Phase 2: the SRS it started from
	Circuits      []circuitRecord `json:"circuits,omitzero"`
	SealedBy      string          `json:"sealed_by,omitzero"` // the commit of the program that sealed
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
