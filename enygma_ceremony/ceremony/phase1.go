package ceremony

import (
	"errors"
	"fmt"
	"io"

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

// SealPhase1 replays Phase 1 from its start through contributions, in order,
// checking each step as Check does; applies the beacon as a last, public
// contribution; and writes the SRS to out.
func SealPhase1(out string, beacon []byte, contributions []string) (Receipt, error) {
	if err := checkBeacon(beacon); err != nil {
		return Receipt{}, err
	}
	if len(contributions) == 0 {
		return Receipt{}, errors.New("no contributions to seal")
	}
	var prev *phase1State
	for i, path := range contributions {
		next := new(phase1State)
		if _, err := open(path, KindPhase1, next.decode); err != nil {
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
	srs := &srsFile{n: prev.n, commons: prev.mpc.Seal(beacon)}
	h, err := create(out, KindSRS, srs.encode)
	return Receipt{Info: Info{Kind: KindSRS, Index: prev.index, N: srs.n, Hash: h}}, err
}

// info describes the state, whose file has sha256 h.
func (s *phase1State) info(h [32]byte) Info {
	return Info{Kind: KindPhase1, Index: s.index, N: s.n, Hash: h}
}

// srsFile is the sealed Phase 1: the powers of τ, α and β that every
// circuit's Phase 2 starts from.
type srsFile struct {
	n       uint64
	commons mpcsetup.SrsCommons
}

// encode writes the SRS body: n, then gnark's SRS.
func (s *srsFile) encode(w io.Writer) error {
	if err := put(w, s.n); err != nil {
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

// readSRS reads the SRS file at path and returns it with the file's sha256.
func readSRS(path string) (*srsFile, [32]byte, error) {
	s := new(srsFile)
	h, err := open(path, KindSRS, s.decode)
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
