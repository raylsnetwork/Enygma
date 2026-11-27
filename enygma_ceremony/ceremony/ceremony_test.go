package ceremony

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/consensys/gnark/frontend"

	"enygma_ceremony/internal/testcircuits"
)

// Stand-ins for a drand round's randomness.
var (
	beaconA = bytes.Repeat([]byte{0xa5}, 32)
	beaconB = bytes.Repeat([]byte{0x5a}, 32)
)

// fixture is a whole small ceremony in a temporary directory: Phase 1 at
// domain 64 with two contributions, its SRS, and Phase 2 for the test
// circuits (domains 4 and 32, so both cut the SRS down) with three.
type fixture struct {
	dir      string
	r1cs     string // the test circuits' <Name>.r1cs files
	circuits []Circuit
	p1       []string // p1[i] is Phase 1 after i contributions
	srs      string
	p2       []string // p2[i] is Phase 2 after i contributions
}

// newFixture runs the fixture's ceremony up to, not including, the Phase 2 seal.
func newFixture(t testing.TB) *fixture {
	t.Helper()
	f := &fixture{dir: t.TempDir()}
	f.r1cs = f.path("r1cs")
	if err := os.Mkdir(f.r1cs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testcircuits.WriteR1CS(f.r1cs); err != nil {
		t.Fatal(err)
	}
	var err error
	if f.circuits, err = CircuitsIn(f.r1cs); err != nil {
		t.Fatal(err)
	}
	f.p1 = f.chain(t, "p1", 2, func(out string) (Receipt, error) { return NewPhase1(out, 64) })
	f.srs = f.path("srs")
	if _, err := SealPhase1(f.srs, Beacon{Value: beaconA}, f.p1[1:]); err != nil {
		t.Fatal(err)
	}
	f.p2 = f.chain(t, "p2", 3, func(out string) (Receipt, error) { return NewPhase2(out, "test", rehearsal(t, f.srs), f.circuits) })
	return f
}

// path is name inside the fixture's directory.
func (f *fixture) path(name string) string { return filepath.Join(f.dir, name) }

// chain writes a start with begin, then n contributions on top of it, and
// returns the n+1 states' paths in order.
func (f *fixture) chain(t testing.TB, prefix string, n int, begin func(out string) (Receipt, error)) []string {
	t.Helper()
	paths := []string{f.path(prefix + "-0")}
	if _, err := begin(paths[0]); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= n; i++ {
		paths = append(paths, f.path(fmt.Sprintf("%s-%d", prefix, i)))
		if _, err := Contribute(paths[i-1], paths[i]); err != nil {
			t.Fatal(err)
		}
	}
	return paths
}

// rewritePhase1 writes a copy of the Phase 1 state at path, changed by edit, as name.
func (f *fixture) rewritePhase1(t *testing.T, path, name string, edit func(*phase1State)) string {
	t.Helper()
	s := new(phase1State)
	if _, err := open(path, KindPhase1, s); err != nil {
		t.Fatal(err)
	}
	edit(s)
	if _, err := create(f.path(name), KindPhase1, s.encode); err != nil {
		t.Fatal(err)
	}
	return f.path(name)
}

// rewritePhase2 writes a copy of the Phase 2 state at path, changed by edit, as name.
func (f *fixture) rewritePhase2(t *testing.T, path, name string, edit func(*phase2State)) string {
	t.Helper()
	s := new(phase2State)
	if _, err := open(path, KindPhase2, s); err != nil {
		t.Fatal(err)
	}
	edit(s)
	if _, err := create(f.path(name), KindPhase2, s.encode); err != nil {
		t.Fatal(err)
	}
	return f.path(name)
}

// seal seals the fixture's Phase 2 with beacon into a new keys directory name.
func (f *fixture) seal(t *testing.T, name string, beacon []byte) string {
	t.Helper()
	if _, err := SealPhase2(f.path(name), "test", rehearsal(t, f.srs), f.circuits, beacon, f.p2[1:]); err != nil {
		t.Fatal(err)
	}
	return f.path(name)
}

// TestSealedKeysProve: the keys a whole ceremony seals load as the gnark
// servers load keys, prove a satisfying assignment, and reject a proof
// against a wrong public input, for a circuit with an in-circuit commitment
// as for one without.
func TestSealedKeysProve(t *testing.T) {
	f := newFixture(t)
	s := new(phase2State)
	if _, err := open(f.p2[1], KindPhase2, s); err != nil {
		t.Fatal(err)
	}
	for _, c := range s.circuits {
		if want := map[string]int{"Committed": 1, "Cubic": 0}[c.name]; len(c.mpc.Sigmas) != want {
			t.Fatalf("%s has %d commitments, want %d", c.name, len(c.mpc.Sigmas), want)
		}
	}
	if err := testcircuits.CheckKeys(f.seal(t, "keys", beaconB), f.r1cs); err != nil {
		t.Fatal(err)
	}
}

// TestSealIsReproducible: anyone who replays the transcript with the same
// beacon gets byte-identical keys, and the beacon changes them.
func TestSealIsReproducible(t *testing.T) {
	f := newFixture(t)
	first, again, other := f.seal(t, "first", beaconB), f.seal(t, "again", beaconB), f.seal(t, "other", beaconA)
	for _, c := range f.circuits {
		for _, key := range []string{c.Name + "PK.key", c.Name + "VK.key"} {
			if !bytes.Equal(readFile(t, filepath.Join(first, key)), readFile(t, filepath.Join(again, key))) {
				t.Errorf("%s differs between two seals with the same beacon", key)
			}
		}
		if vk := c.Name + "VK.key"; bytes.Equal(readFile(t, filepath.Join(first, vk)), readFile(t, filepath.Join(other, vk))) {
			t.Errorf("%s is the same under another beacon", vk)
		}
	}
}

// TestCheck: Check accepts each contribution on top of the one before it,
// and refuses one that is out of order, of another ceremony, or built on
// another state.
func TestCheck(t *testing.T) {
	f := newFixture(t)

	t.Run("accepts each contribution in turn", func(t *testing.T) {
		for _, chain := range [][]string{f.p1, f.p2} {
			for i := 1; i < len(chain); i++ {
				if err := Check(chain[i-1], chain[i]); err != nil {
					t.Errorf("%s on top of %s: %v", chain[i], chain[i-1], err)
				}
			}
		}
	})

	cases := []struct {
		name       string
		prev, next func(t *testing.T) string
		want       error
	}{
		{
			name: "refuses a skipped phase 1 contribution",
			prev: fixed(f.p1[0]), next: fixed(f.p1[2]),
			want: ErrIndex,
		},
		{
			name: "refuses a skipped phase 2 contribution",
			prev: fixed(f.p2[0]), next: fixed(f.p2[2]),
			want: ErrIndex,
		},
		{
			name: "refuses a phase 1 contribution of another domain size",
			prev: fixed(f.p1[0]),
			next: func(t *testing.T) string {
				small := f.chain(t, "small", 1, func(out string) (Receipt, error) { return NewPhase1(out, 32) })
				return small[1]
			},
			want: ErrMismatch,
		},
		{
			name: "refuses a phase 1 contribution built on another state",
			prev: fixed(f.p1[1]),
			next: func(t *testing.T) string {
				// A second contribution on top of the start, numbered as if it followed p1[1].
				fork := f.path("p1-fork")
				if _, err := Contribute(f.p1[0], fork); err != nil {
					t.Fatal(err)
				}
				return f.rewritePhase1(t, fork, "p1-fork-renumbered", func(s *phase1State) { s.index = 2 })
			},
			want: ErrInvalidContribution,
		},
		{
			// gnark's Verify passes a contribution with no challenge; the seal
			// would then fail every honest contribution after it.
			name: "refuses a phase 1 contribution with its challenge stripped",
			prev: fixed(f.p1[0]),
			next: func(t *testing.T) string {
				return f.rewritePhase1(t, f.p1[1], "p1-stripped", func(s *phase1State) { s.mpc.Challenge = nil })
			},
			want: ErrInvalidContribution,
		},
		{
			name: "refuses a phase 2 contribution with a circuit's challenge stripped",
			prev: fixed(f.p2[0]),
			next: func(t *testing.T) string {
				return f.rewritePhase2(t, f.p2[1], "p2-stripped", func(s *phase2State) { s.circuits[1].mpc.Challenge = nil })
			},
			want: ErrInvalidContribution,
		},
		{
			name: "refuses a phase 2 contribution whose δ is not the one it proves",
			prev: fixed(f.p2[0]),
			next: func(t *testing.T) string {
				// Another contribution's parameters under p2[1]'s proofs.
				other := f.path("p2-other")
				if _, err := Contribute(f.p2[0], other); err != nil {
					t.Fatal(err)
				}
				s := new(phase2State)
				if _, err := open(other, KindPhase2, s); err != nil {
					t.Fatal(err)
				}
				return f.rewritePhase2(t, f.p2[1], "p2-swapped", func(p *phase2State) {
					p.circuits[0].mpc.Parameters = s.circuits[0].mpc.Parameters
				})
			},
			want: ErrInvalidContribution,
		},
		{
			name: "refuses a phase 2 state of another ceremony name",
			prev: fixed(f.p2[0]),
			next: func(t *testing.T) string {
				return f.rewritePhase2(t, f.p2[1], "p2-renamed", func(s *phase2State) { s.ceremony = "other" })
			},
			want: ErrMismatch,
		},
		{
			name: "refuses a phase 2 state from another SRS",
			prev: fixed(f.p2[0]),
			next: func(t *testing.T) string {
				srs := f.path("srs-other")
				if _, err := SealPhase1(srs, Beacon{Value: beaconB}, f.p1[1:]); err != nil {
					t.Fatal(err)
				}
				return f.chain(t, "p2-srs-other", 1, func(out string) (Receipt, error) {
					return NewPhase2(out, "test", rehearsal(t, srs), f.circuits)
				})[1]
			},
			want: ErrMismatch,
		},
		{
			name: "refuses a phase 2 state for other circuits",
			prev: fixed(f.p2[0]),
			next: func(t *testing.T) string {
				return f.chain(t, "p2-one-circuit", 1, func(out string) (Receipt, error) {
					return NewPhase2(out, "test", rehearsal(t, f.srs), f.circuits[:1])
				})[1]
			},
			want: ErrMismatch,
		},
		{
			name: "refuses a phase 2 state with a circuit renamed",
			prev: fixed(f.p2[0]),
			next: func(t *testing.T) string {
				return f.rewritePhase2(t, f.p2[1], "p2-circuit-renamed", func(s *phase2State) { s.circuits[0].name = "Renamed" })
			},
			want: ErrMismatch,
		},
		{
			name: "refuses a phase 2 state after a phase 1 state",
			prev: fixed(f.p1[0]), next: fixed(f.p2[1]),
			want: ErrKind,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := Check(c.prev(t), c.next(t)); !errors.Is(err, c.want) {
				t.Fatalf("Check: %v, want %v", err, c.want)
			}
		})
	}
}

// TestSealPhase2Refuses: a seal recomputes the start of Phase 2 from the SRS
// and circuits it is given, so a transcript started from other circuits, or
// a short beacon, seals nothing.
func TestSealPhase2Refuses(t *testing.T) {
	f := newFixture(t)

	t.Run("a short beacon", func(t *testing.T) {
		_, err := SealPhase2(f.path("keys-short"), "test", rehearsal(t, f.srs), f.circuits, beaconA[:MinBeacon-1], f.p2[1:])
		if !errors.Is(err, ErrBeacon) {
			t.Fatalf("SealPhase2: %v, want %v", err, ErrBeacon)
		}
	})

	t.Run("another circuit under a transcript circuit's name", func(t *testing.T) {
		// Committed's constraint system filed as Cubic.
		swapped := []Circuit{f.circuits[0], {Name: "Cubic", R1CS: f.circuits[0].R1CS}}
		if f.circuits[0].Name != "Committed" || f.circuits[1].Name != "Cubic" {
			t.Fatalf("circuits %v, want Committed then Cubic", f.circuits)
		}
		_, err := SealPhase2(f.path("keys-swapped"), "test", rehearsal(t, f.srs), swapped, beaconB, f.p2[1:])
		if !errors.Is(err, ErrMismatch) {
			t.Fatalf("SealPhase2: %v, want %v", err, ErrMismatch)
		}
	})

	// Audit R-035: the keys appear all together or not at all.
	t.Run("a key already in place, before sealing, writing nothing", func(t *testing.T) {
		keys := f.path("keys-taken")
		if err := os.MkdirAll(keys, 0o755); err != nil {
			t.Fatal(err)
		}
		taken := filepath.Join(keys, f.circuits[1].VK)
		if err := os.WriteFile(taken, []byte("an old key"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := SealPhase2(keys, "test", rehearsal(t, f.srs), f.circuits, beaconA, f.p2[1:])
		if err == nil || !strings.Contains(err.Error(), "a key is there already") {
			t.Fatalf("SealPhase2: %v, want a key already in place refused", err)
		}
		entries, err := os.ReadDir(keys)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].Name() != filepath.Base(taken) {
			t.Fatalf("keys directory holds %v after a refused seal, want only the key that was there", entries)
		}
	})
}

// TestKeyManifest: a key manifest beside the circuits seals each key where
// it says, as a protocol's server loads it, and must name every circuit
// there, each at paths of its own inside the keys directory.
func TestKeyManifest(t *testing.T) {
	f := newFixture(t)
	// withManifest copies the fixture's circuits into a new directory with manifest.
	withManifest := func(t *testing.T, manifest string) string {
		t.Helper()
		dir := t.TempDir()
		for _, c := range f.circuits {
			if err := os.WriteFile(filepath.Join(dir, c.Name+".r1cs"), readFile(t, c.R1CS), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(dir, ManifestFile), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
		return dir
	}

	t.Run("seals each key at its path", func(t *testing.T) {
		dir := withManifest(t, `{
			"Committed": {"pk": "keys/CommittedPk.key", "vk": "keys/CommittedVk.key"},
			"Cubic":     {"pk": "keys/zk/CubicPk1.key", "vk": "keys/zk/CubicVk1.key"}}`)
		circuits, err := CircuitsIn(dir)
		if err != nil {
			t.Fatal(err)
		}
		keys := f.path("manifest-keys")
		if _, err := SealPhase2(keys, "test", rehearsal(t, f.srs), circuits, beaconA, f.p2[1:]); err != nil {
			t.Fatal(err)
		}
		if err := testcircuits.CheckKeys(keys, dir); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(keys, "CubicPK.key")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("a key at the default path: %v", err)
		}
	})
	for _, c := range []struct{ name, manifest, want string }{
		{"refuses a manifest missing a circuit",
			`{"Cubic": {"pk": "a", "vk": "b"}, "Other": {"pk": "c", "vk": "d"}}`, "no keys for Committed"},
		{"refuses a manifest naming a circuit not there",
			`{"Cubic": {"pk": "a", "vk": "b"}, "Committed": {"pk": "c", "vk": "d"}, "Other": {"pk": "e", "vk": "f"}}`, "names 3 circuits"},
		{"refuses a key path outside the keys directory",
			`{"Cubic": {"pk": "../a", "vk": "b"}, "Committed": {"pk": "c", "vk": "d"}}`, "not inside the keys directory"},
		{"refuses two keys at one path",
			`{"Cubic": {"pk": "a", "vk": "b"}, "Committed": {"pk": "c", "vk": "./a"}}`, "both seal keys at"},
	} {
		t.Run(c.name, func(t *testing.T) {
			circuits, err := CircuitsIn(withManifest(t, c.manifest))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("CircuitsIn: %+v, %v; want an error saying %q", circuits, err, c.want)
			}
		})
	}
}

// TestNewPhase2RefusesCircuitTooLargeForSRS: a circuit with more constraints
// than the SRS's domain gets an error, not a panic or a short key.
func TestNewPhase2RefusesCircuitTooLargeForSRS(t *testing.T) {
	f := newFixture(t)
	p1 := f.chain(t, "p1-small", 1, func(out string) (Receipt, error) { return NewPhase1(out, 8) })
	srs := f.path("srs-small")
	if _, err := SealPhase1(srs, Beacon{Value: beaconA}, p1[1:]); err != nil {
		t.Fatal(err)
	}
	if _, err := NewPhase2(f.path("p2-small"), "test", rehearsal(t, srs), f.circuits); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("NewPhase2: %v, want %v", err, ErrTooLarge)
	}
}

// TestNewPhase2RefusesNamesThatAreNotPlain (audit M3): Phase 2 starts only
// for a ceremony and circuits with plain names, which every state of it
// then carries, and decoding one requires.
func TestNewPhase2RefusesNamesThatAreNotPlain(t *testing.T) {
	f := newFixture(t)
	circuits := slices.Clone(f.circuits)
	circuits[0].Name = "Cubic\x1b[2J"
	if _, err := NewPhase2(f.path("p2-circuit"), "test", rehearsal(t, f.srs), circuits); err == nil || strings.Contains(err.Error(), "\x1b") {
		t.Fatalf("NewPhase2 of a circuit named with an escape: %v, want it refused", err)
	}
	if _, err := NewPhase2(f.path("p2-ceremony"), "test\x1b[2J", rehearsal(t, f.srs), f.circuits); err == nil || strings.Contains(err.Error(), "\x1b") {
		t.Fatalf("NewPhase2 of a ceremony named with an escape: %v, want it refused", err)
	}
}

// TestNewPhase1RefusesADomainNotAPowerOfTwo: Phase 1's domain is a power of
// two, as the FFTs over it need.
func TestNewPhase1RefusesADomainNotAPowerOfTwo(t *testing.T) {
	if _, err := NewPhase1(filepath.Join(t.TempDir(), "p1-0"), 48); !errors.Is(err, ErrDomain) {
		t.Fatalf("NewPhase1(48): %v, want %v", err, ErrDomain)
	}
}

// TestContributeWithProgress: a contribution reports its progress in order,
// reading, then randomizing (with an estimate and where it will end), then
// writing, with a fraction that only grows, from reading's first report to
// 1 when done, and the state it writes is a valid contribution.
func TestContributeWithProgress(t *testing.T) {
	f := newFixture(t)
	for _, prev := range []string{f.p1[1], f.p2[1]} {
		data := readFile(t, prev)
		var got []Progress
		var out bytes.Buffer
		if _, err := ContributeWithProgress(bytes.NewReader(data), int64(len(data)), &out, func(p Progress) { got = append(got, p) }); err != nil {
			t.Fatal(err)
		}
		order := map[string]int{PartReading: 0, PartRandomizing: 1, PartWriting: 2}
		randomized := false
		for i, p := range got {
			if i > 0 && (order[p.Part] < order[got[i-1].Part] || p.Fraction < got[i-1].Fraction) {
				t.Fatalf("%s: report %d %+v goes back from %+v", prev, i, p, got[i-1])
			}
			if p.Part == PartRandomizing {
				randomized = true
				if p.Estimate <= 0 || p.Until <= p.Fraction || p.Until >= 1 {
					t.Fatalf("%s: randomizing reported %+v", prev, p)
				}
				// Reading ends where randomizing starts: all of the state read.
				if r := got[max(i-1, 0)]; i == 0 || r.Part != PartReading || r.Done != r.Total || r.Fraction != p.Fraction {
					t.Fatalf("%s: randomizing %+v follows %+v, not the end of reading", prev, p, r)
				}
			}
		}
		if !randomized || len(got) < 3 || got[len(got)-1].Fraction != 1 || got[len(got)-1].Part != PartWriting {
			t.Fatalf("%s: reports %+v", prev, got)
		}
		next := f.path("progress-" + filepath.Base(prev))
		if err := os.WriteFile(next, out.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := Check(prev, next); err != nil {
			t.Fatalf("%s: the contribution written with progress: %v", prev, err)
		}
	}
}

// TestContributeToRefusesWhatIsNoState: a contribution starts from a state
// file; anything else is refused before any arithmetic, and nothing is written.
func TestContributeToRefusesWhatIsNoState(t *testing.T) {
	data := []byte("<html>not a state, but long enough to read a header from</html>")
	var out bytes.Buffer
	if _, err := ContributeTo(bytes.NewReader(data), int64(len(data)), &out); !errors.Is(err, ErrFormat) || out.Len() != 0 {
		t.Fatalf("ContributeTo: %v, %d bytes written; want %v and none", err, out.Len(), ErrFormat)
	}
}

// TestStateWithDataAfterItIsRefused: a state file holds exactly one state,
// so the sha256 a participant publishes names one state and nothing else.
func TestStateWithDataAfterItIsRefused(t *testing.T) {
	f := newFixture(t)
	padded := f.path("p2-padded")
	if err := os.WriteFile(padded, append(readFile(t, f.p2[1]), 0), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Inspect(padded); !errors.Is(err, ErrFormat) {
		t.Fatalf("Inspect: %v, want %v", err, ErrFormat)
	}
}

// TestCraftedCountsAreRefusedBeforeAllocating: a state file's counts tell
// gnark's decoders how much to allocate before they read what is counted, so
// one that counts more than the rest of its file could hold is refused at the
// count, not after the decoder has asked for the memory: the coordinator
// reads uploads from anyone with its address.
func TestCraftedCountsAreRefusedBeforeAllocating(t *testing.T) {
	f := newFixture(t)
	p2 := new(phase2State)
	if _, err := open(f.p2[1], KindPhase2, p2); err != nil {
		t.Fatal(err)
	}
	headerLen := len(magic) + 1
	for _, c := range []struct {
		name   string
		path   string
		offset int    // where the count sits in the file
		want   uint64 // the count there, so a layout change can't leave the test vacuous
		width  int
	}{
		// After the header, n (8) and index (4), and gnark's three update
		// proofs (G1 32 + G2 64 each), gnark's SRS opens with its domain size.
		{"phase 1's domain size", f.p1[1], headerLen + 8 + 4 + 3*(32+64), 64, 8},
		// After the header, the ceremony name, SRS hash (32), index (4),
		// circuit count (2), the first circuit's name and r1cs hash (32), and
		// gnark's commitment count (2) and [δ]₁ (32): the length of [K]₁.
		{"phase 2's first circuit's [K]₁", f.p2[1],
			headerLen + 1 + len(p2.ceremony) + 32 + 4 + 2 + 1 + len(p2.circuits[0].name) + 32 + 2 + 32,
			uint64(len(p2.circuits[0].mpc.Parameters.G1.PKK)), 4},
	} {
		t.Run(c.name, func(t *testing.T) {
			data := readFile(t, c.path)
			field := data[c.offset : c.offset+c.width]
			if got := beUint(field); got != c.want {
				t.Fatalf("the count at offset %d is %d, want %d: has the layout changed?", c.offset, got, c.want)
			}
			for i := range field {
				field[i] = 0xff >> 1 // as large as fits without the top bit
			}
			crafted := f.path("crafted-" + filepath.Base(c.path))
			if err := os.WriteFile(crafted, data, 0o644); err != nil {
				t.Fatal(err)
			}
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			_, err := Inspect(crafted)
			runtime.ReadMemStats(&after)
			if !errors.Is(err, ErrFormat) {
				t.Fatalf("Inspect: %v, want %v", err, ErrFormat)
			}
			if grew := after.TotalAlloc - before.TotalAlloc; grew > 64<<20 {
				t.Fatalf("decoding a crafted count allocated %d MiB", grew>>20)
			}
		})
	}
}

// TestGnarksDomainIsTheStatesDomain: gnark sizes a Phase 1, and an SRS, from
// a domain size of its own inside the state, after the one the state's header
// gives. A state whose two differ is refused as it is read: gnark computes
// its sizes as 2N-1 and the like, so a domain of 0 made it panic in the
// coordinator's check of an upload, and a domain of 1 in a contribution's
// arithmetic, where every index past the first is out of range.
func TestGnarksDomainIsTheStatesDomain(t *testing.T) {
	f := newFixture(t)
	headerLen := len(magic) + 1
	for _, c := range []struct {
		name   string
		path   string
		offset int // where gnark's domain sits in the file
	}{
		// As in TestCraftedCountsAreRefusedBeforeAllocating.
		{"phase 1", f.p1[1], headerLen + 8 + 4 + 3*(32+64)},
		// After the header, n (8) and the provenance (head 32, count 4,
		// drand round 8, close 8, the beacon after its length), gnark's SRS
		// opens with its domain size.
		{"SRS", f.srs, headerLen + 8 + 32 + 4 + 8 + 8 + 1 + len(beaconA)},
	} {
		for _, domain := range []uint64{0, 1, 2, 32, 128} {
			t.Run(fmt.Sprintf("%s with gnark's domain %d", c.name, domain), func(t *testing.T) {
				data := readFile(t, c.path)
				field := data[c.offset : c.offset+8]
				if got := beUint(field); got != 64 {
					t.Fatalf("gnark's domain at offset %d is %d, want 64: has the layout changed?", c.offset, got)
				}
				binary.BigEndian.PutUint64(field, domain)
				crafted := f.path(fmt.Sprintf("crafted-domain-%d-%s", domain, filepath.Base(c.path)))
				if err := os.WriteFile(crafted, data, 0o644); err != nil {
					t.Fatal(err)
				}
				if _, err := Inspect(crafted); !errors.Is(err, ErrFormat) {
					t.Fatalf("Inspect: %v, want %v", err, ErrFormat)
				}
			})
		}
	}
}

// beUint reads a big-endian unsigned integer of up to 8 bytes.
func beUint(b []byte) uint64 {
	var v uint64
	for _, x := range b {
		v = v<<8 | uint64(x)
	}
	return v
}

// TestStatesSurviveShortReads: every kind of state reads back intact from a
// reader that returns one byte per Read, as a buffered reader may at a
// buffer boundary, where gnark's bare Read of the challenge would come up short.
func TestStatesSurviveShortReads(t *testing.T) {
	f := newFixture(t)
	for _, c := range []struct {
		path  string
		kind  Kind
		state codec
	}{
		{f.p1[1], KindPhase1, new(phase1State)},
		{f.p2[1], KindPhase2, new(phase2State)},
		{f.srs, KindSRS, new(srsFile)},
	} {
		data := readFile(t, c.path)
		if err := read(iotest.OneByteReader(bytes.NewReader(data)), int64(len(data)), c.kind, c.state); err != nil {
			t.Fatalf("%s: %v", c.kind, err)
		}
		var again bytes.Buffer
		if err := write(&again, c.kind, c.state.encode); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(again.Bytes(), data) {
			t.Errorf("%s: read back one byte at a time, the state writes out differently", c.kind)
		}
	}
}

// fixed returns a test step that yields path.
func fixed(path string) func(*testing.T) string { return func(*testing.T) string { return path } }

// rehearsal names the SRS at path for a Phase 2, as announced by its own
// sha256: the tests' SRSs are sealed with beacons not from drand.
func rehearsal(t testing.TB, path string) SRS {
	t.Helper()
	data := readFile(t, path)
	return SRS{Path: path, SHA256: sha256.Sum256(data), Rehearsal: true}
}

// readFile reads path or fails the test.
func readFile(t testing.TB, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestNewPhase2RefusesTooManyCommitments (audit I7): a circuit of more
// commitments than gnark's Phase 2 seal takes is refused as its Phase 2
// starts, not at the seal, after the whole phase has run.
func TestNewPhase2RefusesTooManyCommitments(t *testing.T) {
	f := newFixture(t)
	p1 := f.chain(t, "p1-wide", 1, func(out string) (Receipt, error) { return NewPhase1(out, 1024) })
	srs := f.path("srs-wide")
	if _, err := SealPhase1(srs, Beacon{Value: beaconA}, p1[1:]); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, n := range []int{169, 170} {
		if err := testcircuits.WriteCircuit(dir, fmt.Sprintf("C%d", n), &testcircuits.Commitments{X: make([]frontend.Variable, n)}); err != nil {
			t.Fatal(err)
		}
	}
	circuits, err := CircuitsIn(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewPhase2(f.path("p2-wide-ok"), "test", rehearsal(t, srs), circuits[:1]); err != nil {
		t.Fatalf("169 commitments: %v", err)
	}
	if _, err := NewPhase2(f.path("p2-wide"), "test", rehearsal(t, srs), circuits[1:]); err == nil || !strings.Contains(err.Error(), "commitments") {
		t.Fatalf("170 commitments: %v, want it refused", err)
	}
}

// TestSRSNamesItsPhase1 (audit H1): an SRS records what it was sealed from:
// the head of the Phase 1 chain it replayed, how many contributions, and
// the beacon with its drand round and the close; so its sha256 names them.
func TestSRSNamesItsPhase1(t *testing.T) {
	f := newFixture(t)
	i, err := Inspect(f.srs)
	if err != nil {
		t.Fatal(err)
	}
	want := Provenance{Head: sha256.Sum256(readFile(t, f.p1[2])), Contributions: 2, Beacon: Beacon{Value: beaconA}}
	if i.Phase1 == nil || i.Phase1.Head != want.Head || i.Phase1.Contributions != 2 || !bytes.Equal(i.Phase1.Beacon.Value, beaconA) ||
		i.Phase1.Beacon.DrandRound != 0 || !i.Phase1.Beacon.CloseAt.IsZero() || i.Index != 2 {
		t.Fatalf("the SRS says %+v, want %+v", i.Phase1, want)
	}
	drand := Beacon{Value: beaconB, DrandRound: 12345, CloseAt: time.Date(2026, 12, 1, 15, 0, 0, 0, time.UTC)}
	other := f.path("srs-drand")
	if _, err := SealPhase1(other, drand, f.p1[1:]); err != nil {
		t.Fatal(err)
	}
	if i, err := Inspect(other); err != nil || i.Phase1.Beacon.DrandRound != drand.DrandRound || !i.Phase1.Beacon.CloseAt.Equal(drand.CloseAt) {
		t.Fatalf("the SRS sealed with drand round %d says %+v (%v)", drand.DrandRound, i.Phase1, err)
	}
	t.Run("refuses a provenance no seal writes", func(t *testing.T) {
		for name, edit := range map[string]func(*srsFile){
			"no contributions":           func(s *srsFile) { s.prov.Contributions = 0 },
			"a beacon too short":         func(s *srsFile) { s.prov.Beacon.Value = beaconA[:MinBeacon-1] },
			"a drand round and no close": func(s *srsFile) { s.prov.Beacon.DrandRound = 5 },
			"a close and no drand round": func(s *srsFile) { s.prov.Beacon.CloseAt = time.Unix(1, 0) },
		} {
			s, _, err := readSRS(f.srs)
			if err != nil {
				t.Fatal(err)
			}
			edit(s)
			var buf bytes.Buffer
			if err := write(&buf, KindSRS, s.encode); err != nil {
				t.Fatal(err)
			}
			path := f.path("srs-" + strings.ReplaceAll(name, " ", "-"))
			if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := Inspect(path); !errors.Is(err, ErrFormat) {
				t.Errorf("an SRS with %s: %v, want %v", name, err, ErrFormat)
			}
		}
	})
}

// TestSealPhase1RefusesALongBeacon: an SRS records its beacon after a
// one-byte length, so a beacon of more than 255 bytes is refused rather
// than written with the wrong length.
func TestSealPhase1RefusesALongBeacon(t *testing.T) {
	f := newFixture(t)
	if _, err := SealPhase1(f.path("srs-long"), Beacon{Value: bytes.Repeat([]byte{1}, 256)}, f.p1[1:]); err == nil {
		t.Fatal("SealPhase1 took a beacon of 256 bytes")
	}
}

// TestPhase2StartsOnlyFromTheAnnouncedSRS (audit H1): a Phase 2 starts, is
// checked and is sealed only from the SRS whose sha256 was announced, read
// from the same bytes it decodes; and from a rehearsal's SRS only when told.
// Another Phase 1's seal, or any SRS made from a τ, α and β someone kept,
// would let them forge proofs however many contributed to the Phase 2.
func TestPhase2StartsOnlyFromTheAnnouncedSRS(t *testing.T) {
	f := newFixture(t)
	other := f.path("srs-other")
	if _, err := SealPhase1(other, Beacon{Value: beaconB}, f.p1[1:]); err != nil {
		t.Fatal(err)
	}
	announced := rehearsal(t, f.srs)
	substituted := SRS{Path: other, SHA256: announced.SHA256, Rehearsal: true}
	if _, err := NewPhase2(f.path("p2-other"), "test", substituted, f.circuits); !errors.Is(err, ErrSRS) {
		t.Fatalf("NewPhase2 from another SRS: %v, want %v", err, ErrSRS)
	}
	if _, err := SealPhase2(f.path("keys-other"), "test", substituted, f.circuits, beaconB, f.p2[1:]); !errors.Is(err, ErrSRS) {
		t.Fatalf("SealPhase2 from another SRS: %v, want %v", err, ErrSRS)
	}
	notRehearsal := announced
	notRehearsal.Rehearsal = false
	if _, err := NewPhase2(f.path("p2-rehearsal"), "test", notRehearsal, f.circuits); !errors.Is(err, ErrSRS) {
		t.Fatalf("NewPhase2 from a rehearsal's SRS, not told so: %v, want %v", err, ErrSRS)
	}
	phase1, err := SealPhase2(f.path("keys"), "test", announced, f.circuits, beaconB, f.p2[1:])
	if err != nil {
		t.Fatal(err)
	}
	if want := sha256.Sum256(readFile(t, f.p1[2])); phase1.Head != want || phase1.Contributions != 2 {
		t.Fatalf("SealPhase2 reports %+v, want Phase 1's head %x", phase1, want)
	}
	i, err := Inspect(f.p2[3])
	if err != nil {
		t.Fatal(err)
	}
	if i.SRS != announced.SHA256 || len(i.R1CS) != len(f.circuits) || i.R1CS[0] != sha256.Sum256(readFile(t, f.circuits[0].R1CS)) {
		t.Fatalf("Phase 2 state says SRS %x, circuits %x", i.SRS, i.R1CS)
	}
}

// TestFormat1IsRefused: a state file of format 1, from before an SRS
// carried its provenance, is not read as one of format 2.
func TestFormat1IsRefused(t *testing.T) {
	f := newFixture(t)
	for _, path := range []string{f.p1[1], f.srs, f.p2[1]} {
		data := readFile(t, path)
		data[len(magic)-1] = 1
		old := path + "-format-1"
		if err := os.WriteFile(old, data, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Inspect(old); !errors.Is(err, ErrFormat) {
			t.Errorf("%s of format 1: %v, want %v", filepath.Base(path), err, ErrFormat)
		}
	}
}

// TestCircuitsInListsByName (audit I9, I10): a directory's circuits are
// listed by circuit name, as -circuits names are compared, though their
// file names sort otherwise; and a directory whose name holds a glob's
// metacharacters lists its own files, not another directory's.
func TestCircuitsInListsByName(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "r1cs[x]")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Cubic", "Cubic-2"} {
		if err := testcircuits.WriteCircuit(dir, name, &testcircuits.Cubic{}); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "Directory.r1cs"), 0o755); err != nil { // not a circuit
		t.Fatal(err)
	}
	circuits, err := CircuitsIn(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range circuits {
		names = append(names, c.Name)
	}
	if !slices.Equal(names, []string{"Cubic", "Cubic-2"}) {
		t.Fatalf("CircuitsIn lists %v, want [Cubic Cubic-2]", names)
	}
}
