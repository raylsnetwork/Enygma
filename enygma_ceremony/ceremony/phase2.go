package ceremony

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/consensys/gnark/backend/groth16/bn254/mpcsetup"
	cs "github.com/consensys/gnark/constraint/bn254"
)

// Circuit is one circuit of a Phase 2 ceremony.
type Circuit struct {
	// Name names the circuit in the ceremony's states.
	Name string
	// R1CS is the path of its compiled constraint system, as gnark's WriteTo writes it.
	R1CS string
	// PK and VK are where its proving and verifying keys are sealed, relative
	// to the keys directory: <Name>PK.key and <Name>VK.key unless the
	// circuits' directory has a key manifest (CircuitsIn).
	PK, VK string
}

// ManifestFile is the key manifest a directory of circuits may hold: a JSON
// object mapping a circuit's name to {"pk": path, "vk": path}, the paths
// its keys are sealed at, relative to the keys directory. A protocol's
// export_r1cs writes it from the paths its gnark server loads keys from,
// so the sealed keys directory has the server's own layout.
const ManifestFile = "keys.json"

// CircuitsIn lists dir's <Name>.r1cs files as circuits, sorted by name,
// with their key paths from dir's key manifest when it has one. The
// manifest must name exactly the circuits there, each at paths of its own.
// dir is a directory's path, never a pattern: a name holding '[' or '*'
// lists that directory (audit I10).
func CircuitsIn(dir string) ([]Circuit, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var circuits []Circuit
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".r1cs")
		if !ok || e.IsDir() {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if err := checkName("circuit", name); err != nil {
			return nil, fmt.Errorf("%q: %w", p, err)
		}
		circuits = append(circuits, Circuit{Name: name, R1CS: p, PK: name + "PK.key", VK: name + "VK.key"})
	}
	if len(circuits) == 0 {
		return nil, fmt.Errorf("no .r1cs files in %s", dir)
	}
	// By circuit name, as -circuits is compared: the file names sort
	// otherwise ("Cubic-2.r1cs" before "Cubic.r1cs"; audit I9).
	slices.SortFunc(circuits, func(a, b Circuit) int { return strings.Compare(a.Name, b.Name) })
	data, err := os.ReadFile(filepath.Join(dir, ManifestFile))
	if errors.Is(err, os.ErrNotExist) {
		return circuits, nil
	}
	if err != nil {
		return nil, err
	}
	var manifest map[string]struct{ PK, VK string }
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Join(dir, ManifestFile), err)
	}
	if len(manifest) != len(circuits) {
		return nil, fmt.Errorf("%s names %d circuits, %s has %d", ManifestFile, len(manifest), dir, len(circuits))
	}
	taken := map[string]string{}
	for i := range circuits {
		c := &circuits[i]
		keys, ok := manifest[c.Name]
		if !ok {
			return nil, fmt.Errorf("%s has no keys for %s", ManifestFile, c.Name)
		}
		for _, p := range []string{keys.PK, keys.VK} {
			if !filepath.IsLocal(p) {
				return nil, fmt.Errorf("%s: %s's key path %q is not inside the keys directory", ManifestFile, c.Name, p)
			}
			if other, ok := taken[filepath.Clean(p)]; ok {
				return nil, fmt.Errorf("%s: %s and %s both seal keys at %s", ManifestFile, other, c.Name, p)
			}
			taken[filepath.Clean(p)] = c.Name
		}
		c.PK, c.VK = keys.PK, keys.VK
	}
	return circuits, nil
}

// phase2State is Phase 2 after index contributions, for every circuit of one ceremony.
type phase2State struct {
	ceremony string   // the protocol whose circuits these are, e.g. enygma_retail_payments
	srs      [32]byte // sha256 of the SRS file Phase 2 started from
	index    uint32
	circuits []phase2Circuit
}

// phase2Circuit is one circuit's part of a Phase 2 state.
type phase2Circuit struct {
	name string   // its keys are written as <name>PK.key and <name>VK.key
	r1cs [32]byte // sha256 of its compiled constraint system file
	mpc  mpcsetup.Phase2
}

// encode writes the state's body: the ceremony, SRS hash, index, then each
// circuit's name, constraint system hash and gnark Phase 2.
func (s *phase2State) encode(w io.Writer) error {
	if err := putString(w, s.ceremony); err != nil {
		return err
	}
	if err := put(w, s.srs, s.index, uint16(len(s.circuits))); err != nil {
		return err
	}
	for i := range s.circuits {
		c := &s.circuits[i]
		if err := putString(w, c.name); err != nil {
			return err
		}
		if err := put(w, c.r1cs); err != nil {
			return err
		}
		if _, err := c.mpc.WriteTo(w); err != nil {
			return err
		}
	}
	return nil
}

// decode reads a body as encode wrote it.
func (s *phase2State) decode(r *body) error {
	var err error
	if s.ceremony, err = getString(r); err != nil {
		return err
	}
	if err := checkName("ceremony", s.ceremony); err != nil {
		return fmt.Errorf("%w: %v", ErrFormat, err)
	}
	var count uint16
	if err := get(r, &s.srs, &s.index); err != nil {
		return err
	}
	if err := get(r.bounded(), &count); err != nil {
		return err
	}
	if count == 0 {
		// A step of no circuits checks nothing, and seals no key (audit R-034).
		return fmt.Errorf("%w: a Phase 2 of no circuits", ErrFormat)
	}
	// bounded holds the count to 32 bytes a circuit, where each allocates
	// about 576: a circuit takes at least minCircuit in the file (audit R-073).
	if room := max(r.left, 0) / minCircuit; int64(count) > room {
		return fmt.Errorf("%w: %d circuits, where %d bytes are left", ErrFormat, count, r.left)
	}
	s.circuits = make([]phase2Circuit, count)
	for i := range s.circuits {
		c := &s.circuits[i]
		if c.name, err = getString(r); err != nil {
			return err
		}
		if err := checkName("circuit", c.name); err != nil {
			return fmt.Errorf("%w: %v", ErrFormat, err)
		}
		if err := get(r, &c.r1cs); err != nil {
			return err
		}
		if _, err := c.mpc.ReadFrom(r.bounded()); err != nil {
			return fmt.Errorf("%w: circuit %q: %v", ErrFormat, c.name, err)
		}
		// gnark's Contribute panics past 255 commitments, after drawing the
		// randomness (audit R-009), and its seal past maxCommitments (I7):
		// refused here, before anyone contributes.
		if n := len(c.mpc.Parameters.G1.SigmaCKK); n > maxCommitments {
			return fmt.Errorf("%w: circuit %q: %d commitments, more than a Phase 2 can seal (%d)", ErrFormat, c.name, n, maxCommitments)
		}
	}
	return nil
}

// maxName is the longest a Phase 2's ceremony or circuit name may be.
const maxName = 64

// checkName refuses a ceremony or circuit name (what says which) that is not
// plain: letters, digits, '_', '.' and '-', starting with a letter or a
// digit, at most maxName bytes. Every name of Enygma's protocols and
// circuits is one. A Phase 2's names come from its state file, which a
// coordinator may have made: they are printed, logged, and put in file
// paths and in the commands the coordinator prints, so a plain name is one
// no terminal or shell acts on (audit M2, M3).
func checkName(what, name string) error {
	ok := name != "" && len(name) <= maxName
	for i := 0; ok && i < len(name); i++ {
		switch b := name[i]; {
		case 'a' <= b && b <= 'z', 'A' <= b && b <= 'Z', '0' <= b && b <= '9':
		case (b == '_' || b == '.' || b == '-') && i > 0:
		default:
			ok = false
		}
	}
	if !ok {
		return fmt.Errorf("a %s name must be letters, digits, '_', '.' and '-', starting with a letter or digit, at most %d bytes: not %q", what, maxName, name)
	}
	return nil
}

// maxCommitments is the most commitments a circuit's Phase 2 may hold. gnark
// tells each one's proof apart by a byte (255), but its Phase 2 seal draws
// 1 + commitments field elements from the beacon with RFC 9380's
// expand_message, which gives at most 170: past 169 commitments the seal
// panics, after the whole phase has run (audit I7).
const maxCommitments = 169

// minCircuit is the fewest bytes a circuit takes in a Phase 2 file: its [δ]₂
// (64) and δ's update proof (32 and 64) alone, compressed.
const minCircuit = 160

// info describes the state, whose file has sha256 h.
func (s *phase2State) info(h [32]byte) Info {
	names := make([]string, len(s.circuits))
	r1cs := make([][32]byte, len(s.circuits))
	for i, c := range s.circuits {
		names[i], r1cs[i] = c.name, c.r1cs
	}
	return Info{Kind: KindPhase2, Index: s.index, Ceremony: s.ceremony, Circuits: names, R1CS: r1cs, SRS: s.srs, Hash: h}
}

// checkPhase2Step checks that next is one valid contribution on top of prev.
func checkPhase2Step(prev, next *phase2State) error {
	if err := sameCeremony(prev, next); err != nil {
		return err
	}
	if next.index != prev.index+1 {
		return fmt.Errorf("%w: contribution %d on top of state %d", ErrIndex, next.index, prev.index)
	}
	for i := range prev.circuits {
		// As in checkPhase1Step: a contribution with no challenge passes
		// gnark's Verify, which fills it in, and would fail the next one's seal.
		if len(next.circuits[i].mpc.Challenge) == 0 {
			return fmt.Errorf("%w: circuit %q: it does not name the state it builds on (no challenge)",
				ErrInvalidContribution, prev.circuits[i].name)
		}
		if err := prev.circuits[i].mpc.Verify(&next.circuits[i].mpc); err != nil {
			return fmt.Errorf("%w: circuit %q: %v", ErrInvalidContribution, prev.circuits[i].name, err)
		}
	}
	return nil
}

// sameCeremony checks that b is a state of a's ceremony: the same name, SRS,
// and circuits in the same order.
func sameCeremony(a, b *phase2State) error {
	if b.ceremony != a.ceremony {
		return fmt.Errorf("%w: ceremony %q, want %q", ErrMismatch, b.ceremony, a.ceremony)
	}
	if b.srs != a.srs {
		return fmt.Errorf("%w: started from SRS %x, want %x", ErrMismatch, b.srs, a.srs)
	}
	if !slices.EqualFunc(a.circuits, b.circuits, sameCircuit) {
		return fmt.Errorf("%w: circuits %s, want %s", ErrMismatch, circuitList(b), circuitList(a))
	}
	return nil
}

// sameCircuit reports whether two circuit entries name the same constraint system.
func sameCircuit(a, b phase2Circuit) bool { return a.name == b.name && a.r1cs == b.r1cs }

// circuitList prints a state's circuits as "name":hash-prefix, for errors.
// A name is whatever an uploaded state says, so it is quoted (audit R-042).
func circuitList(s *phase2State) string {
	parts := make([]string, len(s.circuits))
	for i, c := range s.circuits {
		parts[i] = fmt.Sprintf("%q:%x", c.name, c.r1cs[:4])
	}
	return "[" + strings.Join(parts, " ") + "]"
}

// sealInput is what sealing a circuit needs besides its last state: its cut
// of the SRS and gnark's evaluations of its constraints there.
type sealInput struct {
	commons *mpcsetup.SrsCommons
	evals   mpcsetup.Phase2Evaluations
}

// SRS is the SRS a Phase 2 starts from: its file, and the sha256 announced
// for it, which the file must have (audit H1). Any file decodes as an SRS,
// including one made from a τ, α and β someone kept, and the keys take α
// and β from it, so a Phase 2 is only as sound as the Phase 1 its SRS is
// the seal of: the announced sha256 is the one independent resealers of
// that Phase 1 got. A rehearsal's SRS, sealed with a beacon not from drand,
// is taken only when Rehearsal says so.
type SRS struct {
	Path      string
	SHA256    [32]byte
	Rehearsal bool
}

// start computes the start of Phase 2 for ceremony's circuits from the SRS
// at srs.Path. It holds no randomness, so a verifier recomputes it rather
// than trusting a file. gnark evaluates a circuit's constraints one by one,
// so the circuits are started in parallel.
func start(ceremony string, srsIn SRS, circuits []Circuit) (*phase2State, []sealInput, Provenance, error) {
	if err := checkName("ceremony", ceremony); err != nil {
		return nil, nil, Provenance{}, err
	}
	if len(circuits) == 0 {
		return nil, nil, Provenance{}, errors.New("no circuits")
	}
	for _, c := range circuits {
		if err := checkName("circuit", c.Name); err != nil {
			return nil, nil, Provenance{}, err
		}
	}
	srs, srsHash, err := readSRS(srsIn.Path)
	if err != nil {
		return nil, nil, Provenance{}, err
	}
	if srsHash != srsIn.SHA256 {
		return nil, nil, Provenance{}, fmt.Errorf("%w: %s has sha256 %x, and the SRS announced %x", ErrSRS, srsIn.Path, srsHash, srsIn.SHA256)
	}
	if srs.prov.Beacon.DrandRound == 0 && !srsIn.Rehearsal {
		return nil, nil, Provenance{}, fmt.Errorf("%w: %s was sealed with a beacon not from drand, so it is a rehearsal's", ErrSRS, srsIn.Path)
	}
	s := &phase2State{ceremony: ceremony, srs: srsHash, circuits: make([]phase2Circuit, len(circuits))}
	seal := make([]sealInput, len(circuits))
	errs := make([]error, len(circuits))
	var wg sync.WaitGroup
	for i, c := range circuits {
		wg.Go(func() {
			// gnark's Initialize panics on a circuit it cannot set up; that is
			// this circuit's error, not a crash of every circuit's setup.
			defer func() {
				if r := recover(); r != nil {
					errs[i] = fmt.Errorf("circuit %q: %v", c.Name, r)
				}
			}()
			r1cs, h, err := readR1CS(c.R1CS)
			if err != nil {
				errs[i] = err
				return
			}
			commons, err := srs.forCircuit(r1cs)
			if err != nil {
				errs[i] = fmt.Errorf("circuit %q: %w", c.Name, err)
				return
			}
			s.circuits[i].name, s.circuits[i].r1cs = c.Name, h
			seal[i] = sealInput{commons: commons, evals: s.circuits[i].mpc.Initialize(r1cs, commons)}
			if n := len(s.circuits[i].mpc.Parameters.G1.SigmaCKK); n > maxCommitments {
				errs[i] = fmt.Errorf("circuit %q: %d commitments, more than a Phase 2 can seal (%d)", c.Name, n, maxCommitments)
			}
		})
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, nil, Provenance{}, err
	}
	return s, seal, srs.prov, nil
}

// readR1CS reads a compiled constraint system and returns it with the
// sha256 of its file.
func readR1CS(path string) (*cs.R1CS, [32]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, [32]byte{}, err
	}
	r1cs := new(cs.R1CS) // empty, as groth16.NewCS makes it; NewR1CS is for gnark's compiler only
	if _, err := r1cs.ReadFrom(bytes.NewReader(data)); err != nil {
		return nil, [32]byte{}, fmt.Errorf("%s: %w", path, err)
	}
	return r1cs, sha256.Sum256(data), nil
}

// NewPhase2 writes the start of ceremony's Phase 2 for circuits to out, from
// the SRS srs names.
func NewPhase2(out, ceremony string, srs SRS, circuits []Circuit) (Receipt, error) {
	s, _, _, err := start(ceremony, srs, circuits)
	if err != nil {
		return Receipt{}, err
	}
	h, err := create(out, KindPhase2, s.encode)
	return Receipt{Info: s.info(h)}, err
}

// SealPhase2 replays ceremony's Phase 2 from its start, recomputed from the
// SRS srs names and the circuits, through chain's contributions in order,
// checking the chain as Chain says and each
// step as Check does; applies the beacon as a last, public contribution; and
// writes each circuit's proving and verifying key into keysDir at its PK
// and VK paths, in the form gnark's ReadFrom loads, never over an existing
// file. The keys appear all together or not at all: each is written in
// full, and synced, before any is put in place. It returns what the SRS
// says it was sealed from.
func SealPhase2(keysDir, ceremony string, srs SRS, circuits []Circuit, beacon []byte, chain Chain) (Provenance, error) {
	if err := checkBeacon(beacon); err != nil {
		return Provenance{}, err
	}
	if len(chain.Paths) == 0 {
		return Provenance{}, errors.New("no contributions to seal")
	}
	for _, c := range circuits { // before the hours a seal can take
		for _, p := range []string{c.PK, c.VK} {
			if _, err := os.Lstat(filepath.Join(keysDir, p)); !errors.Is(err, os.ErrNotExist) {
				return Provenance{}, fmt.Errorf("%s: a key is there already, or cannot be checked (%v)", filepath.Join(keysDir, p), err)
			}
		}
	}
	prev, seal, phase1, err := start(ceremony, srs, circuits)
	if err != nil {
		return Provenance{}, err
	}
	replayed := make([][32]byte, len(chain.Paths))
	for i, path := range chain.Paths {
		next := new(phase2State)
		if replayed[i], err = open(path, KindPhase2, next); err != nil {
			return Provenance{}, err
		}
		if err := checkPhase2Step(prev, next); err != nil {
			return Provenance{}, fmt.Errorf("contribution %d (%s): %w", i+1, path, err)
		}
		prev = next
	}
	if err := chain.check(replayed); err != nil {
		return Provenance{}, err
	}
	type written struct{ tmp, path string }
	var keys []written
	var placed []string
	undo := func(err error) error {
		for _, k := range keys {
			if rerr := os.Remove(k.tmp); rerr != nil && !errors.Is(rerr, os.ErrNotExist) {
				err = errors.Join(err, rerr)
			}
		}
		for _, p := range placed {
			err = errors.Join(err, os.Remove(p))
		}
		return err
	}
	for i := range prev.circuits {
		c := &prev.circuits[i]
		pk, vk := c.mpc.Seal(seal[i].commons, &seal[i].evals, beacon)
		for _, k := range []struct {
			path string
			key  io.WriterTo
		}{{filepath.Join(keysDir, circuits[i].PK), pk}, {filepath.Join(keysDir, circuits[i].VK), vk}} {
			tmp, err := writeKey(k.path, k.key)
			if err != nil {
				return Provenance{}, undo(err)
			}
			keys = append(keys, written{tmp, k.path})
		}
	}
	for _, k := range keys {
		// Link, not rename: it fails, rather than replace, if a key appeared there meanwhile.
		if err := os.Link(k.tmp, k.path); err != nil {
			return Provenance{}, undo(err)
		}
		placed = append(placed, k.path)
	}
	err = nil
	for _, k := range keys {
		err = errors.Join(err, os.Remove(k.tmp))
	}
	return phase1, err
}

// writeKey writes a gnark key with its WriteTo, the form the gnark servers
// load, to a new file beside path, synced, and returns that file's name. A
// failed write leaves no file behind.
func writeKey(path string, key io.WriterTo) (tmp string, err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".")
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			f.Close()
			err = errors.Join(err, os.Remove(f.Name()))
		}
	}()
	w := bufio.NewWriterSize(f, 1<<20)
	if _, err = key.WriteTo(w); err != nil {
		return "", err
	}
	if err = w.Flush(); err != nil {
		return "", err
	}
	if err = f.Sync(); err != nil {
		return "", err
	}
	if err = f.Chmod(0o644); err != nil {
		return "", err
	}
	return f.Name(), f.Close()
}
