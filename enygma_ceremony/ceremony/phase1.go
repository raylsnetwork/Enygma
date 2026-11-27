package ceremony

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16/bn254/mpcsetup"
	cs "github.com/consensys/gnark/constraint/bn254"
)

// phase1State is Phase 1 for domain size n after index contributions.
type phase1State struct {
	n     uint64
	index uint32
	mpc   mpcsetup.Phase1
}

// encode writes the state's body: n, index, then gnark's Phase 1.
func (s *phase1State) encode(w io.Writer) error {
	if err := put(w, s.n, s.index); err != nil {
		return err
	}
	_, err := s.mpc.WriteTo(w)
	return err
}

// decode reads a body as encode wrote it.
func (s *phase1State) decode(r *body) error {
	if err := get(r, &s.n, &s.index); err != nil {
		return err
	}
	if err := checkDomain(s.n); err != nil {
		return err
	}
	if _, err := s.mpc.ReadFrom(domainOf(r.bounded(), s.n)); err != nil {
		return fmt.Errorf("%w: %v", ErrFormat, err)
	}
	return nil
}

// checkPhase1Step checks that next is one valid contribution on top of prev.
func checkPhase1Step(prev, next *phase1State) error {
	if next.n != prev.n {
		return fmt.Errorf("%w: domain size %d, want %d", ErrMismatch, next.n, prev.n)
	}
	if next.index != prev.index+1 {
		return fmt.Errorf("%w: contribution %d on top of state %d", ErrIndex, next.index, prev.index)
	}
	// gnark's Verify passes a contribution with no challenge (the hash of the
	// state it builds on) and fills it in. A check reading the state from its
	// file would then see it without one, but a seal carries the filled-in
	// state on: the next contribution, bound to the file, would pass every
	// check and fail the seal. Every contribution names the state it builds on.
	if len(next.mpc.Challenge) == 0 {
		return fmt.Errorf("%w: it does not name the state it builds on (no challenge)", ErrInvalidContribution)
	}
	if err := prev.mpc.Verify(&next.mpc); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidContribution, err)
	}
	return nil
}

// NewPhase1 writes the start of Phase 1 for domain size n to out. The start
// holds no randomness, so anyone recomputes it; the SRS sealed from it serves
// any circuit of up to n constraints.
func NewPhase1(out string, n uint64) (Receipt, error) {
	if err := checkDomain(n); err != nil {
		return Receipt{}, err
	}
	s := &phase1State{n: n, mpc: *mpcsetup.NewPhase1(n)}
	h, err := create(out, KindPhase1, s.encode)
	return Receipt{Info: s.info(h)}, err
}

// Beacon is the public randomness a seal applies as a last contribution, and
// where it came from.
type Beacon struct {
	Value []byte // at least MinBeacon bytes
	// DrandRound is the drand quicknet round whose randomness Value is, as the
	// seal fetched and checked it; 0 for a beacon not from drand, which
	// whoever sealed chose: for tests and rehearsals only.
	DrandRound uint64
	CloseAt    time.Time // the phase's close, as announced; zero when not given
}

// Provenance is what an SRS was sealed from (audit H1): the Phase 1 chain
// that ends at Head, after Contributions contributions, and the beacon.
// SealPhase1 writes it into the SRS file, so the SRS's sha256 names it, and
// so does every Phase 2 state built on that SRS, which records the sha256.
// A file can claim any provenance: what makes the claim true is a reseal of
// that Phase 1 that gives the same file.
type Provenance struct {
	Head          [32]byte
	Contributions uint32
	Beacon        Beacon
}

// encode writes the provenance: head, count, drand round, close (Unix
// seconds, 0 for none), then the beacon after its one-byte length.
func (p *Provenance) encode(w io.Writer) error {
	var closeAt int64
	if !p.Beacon.CloseAt.IsZero() {
		closeAt = p.Beacon.CloseAt.Unix()
	}
	if err := put(w, p.Head, p.Contributions, p.Beacon.DrandRound, closeAt); err != nil {
		return err
	}
	return putBytes(w, p.Beacon.Value)
}

// decode reads a provenance as encode wrote it. Its integers are not counts,
// so they are read from r itself, not r.bounded().
func (p *Provenance) decode(r *body) error {
	var closeAt int64
	if err := get(r, &p.Head, &p.Contributions, &p.Beacon.DrandRound, &closeAt); err != nil {
		return err
	}
	if p.Contributions == 0 {
		return fmt.Errorf("%w: an SRS sealed from no contribution", ErrFormat)
	}
	if (p.Beacon.DrandRound == 0) != (closeAt == 0) {
		// A drand round is checked against the close it came out after; a
		// beacon not from drand has none (audit H1).
		return fmt.Errorf("%w: an SRS with a drand round and no close, or a close and no drand round", ErrFormat)
	}
	p.Beacon.CloseAt = time.Time{}
	if closeAt != 0 {
		p.Beacon.CloseAt = time.Unix(closeAt, 0).UTC()
	}
	value, err := getString(r)
	if err != nil {
		return err
	}
	p.Beacon.Value = []byte(value)
	if err := checkBeacon(p.Beacon.Value); err != nil {
		return fmt.Errorf("%w: %v", ErrFormat, err)
	}
	return nil
}

// SealPhase1 replays Phase 1 from its start through contributions, in order,
// checking each step as Check does; applies the beacon as a last, public
// contribution; and writes the SRS to out, with its provenance: the head it
// replayed to, and the beacon.
func SealPhase1(out string, beacon Beacon, contributions []string) (Receipt, error) {
	if err := checkBeacon(beacon.Value); err != nil {
		return Receipt{}, err
	}
	if len(contributions) == 0 {
		return Receipt{}, errors.New("no contributions to seal")
	}
	var (
		prev *phase1State
		head [32]byte
	)
	for i, path := range contributions {
		next := new(phase1State)
		var err error
		if head, err = open(path, KindPhase1, next); err != nil {
			return Receipt{}, err
		}
		if prev == nil {
			prev = &phase1State{n: next.n, mpc: *mpcsetup.NewPhase1(next.n)}
		}
		if err := checkPhase1Step(prev, next); err != nil {
			return Receipt{}, fmt.Errorf("contribution %d (%s): %w", i+1, path, err)
		}
		prev = next
	}
	srs := &srsFile{n: prev.n, prov: Provenance{Head: head, Contributions: prev.index, Beacon: beacon}, commons: prev.mpc.Seal(beacon.Value)}
	h, err := create(out, KindSRS, srs.encode)
	return Receipt{Info: srs.info(h)}, err
}

// info describes the state, whose file has sha256 h.
func (s *phase1State) info(h [32]byte) Info {
	return Info{Kind: KindPhase1, Index: s.index, N: s.n, Hash: h}
}

// srsFile is the sealed Phase 1: the powers of τ, α and β that every
// circuit's Phase 2 starts from, and what they were sealed from.
type srsFile struct {
	n       uint64
	prov    Provenance
	commons mpcsetup.SrsCommons
}

// encode writes the SRS body: n, the provenance, then gnark's SRS.
func (s *srsFile) encode(w io.Writer) error {
	if err := put(w, s.n); err != nil {
		return err
	}
	if err := s.prov.encode(w); err != nil {
		return err
	}
	_, err := s.commons.WriteTo(w)
	return err
}

// decode reads a body as encode wrote it.
func (s *srsFile) decode(r *body) error {
	if err := get(r, &s.n); err != nil {
		return err
	}
	if err := checkDomain(s.n); err != nil {
		return err
	}
	if err := s.prov.decode(r); err != nil {
		return err
	}
	if _, err := s.commons.ReadFrom(domainOf(r.bounded(), s.n)); err != nil {
		return fmt.Errorf("%w: %v", ErrFormat, err)
	}
	// gnark v0.14 sizes the powers from the domain domainOf checked, so this
	// holds already; it stays for a gnark that reads their lengths apart.
	if got := uint64(len(s.commons.G2.Tau)); got != s.n {
		return fmt.Errorf("%w: header says domain %d, the powers say %d", ErrFormat, s.n, got)
	}
	return nil
}

// info describes the SRS, whose file has sha256 h.
func (s *srsFile) info(h [32]byte) Info {
	prov := s.prov
	return Info{Kind: KindSRS, Index: prov.Contributions, N: s.n, Phase1: &prov, Hash: h}
}

// readSRS reads the SRS file at path and returns it with the file's sha256.
func readSRS(path string) (*srsFile, [32]byte, error) {
	s := new(srsFile)
	h, err := open(path, KindSRS, s)
	return s, h, err
}

// forCircuit cuts the SRS down to the domain r1cs needs: the power of two at
// or above its constraint count, as gnark's own groth16.Setup sizes it. The
// powers are of τ itself, so the first ones of a larger Phase 1 are a valid
// Phase 1 of the smaller domain.
func (s *srsFile) forCircuit(r1cs *cs.R1CS) (*mpcsetup.SrsCommons, error) {
	n := ecc.NextPowerOfTwo(uint64(r1cs.GetNbConstraints()))
	if n > s.n {
		return nil, fmt.Errorf("%w: %d constraints need a domain of %d, the SRS has %d",
			ErrTooLarge, r1cs.GetNbConstraints(), n, s.n)
	}
	var c mpcsetup.SrsCommons
	c.G1.Tau = s.commons.G1.Tau[:2*n-1]
	c.G1.AlphaTau = s.commons.G1.AlphaTau[:n]
	c.G1.BetaTau = s.commons.G1.BetaTau[:n]
	c.G2.Tau = s.commons.G2.Tau[:n]
	c.G2.Beta = s.commons.G2.Beta
	return &c, nil
}
