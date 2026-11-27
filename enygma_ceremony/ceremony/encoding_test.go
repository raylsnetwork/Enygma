package ceremony

// Ported from the audit's fuzz lane (2026-11-26). Two gaps the other targets
// cannot reach by byte mutation:
//
//   - gnark-crypto's decoder takes a point compressed (as the ceremony writes
//     it) or uncompressed (64 bytes for G1, 128 for G2). Turning one into the
//     other needs a valid Y coordinate, which no bit flip produces, so
//     FuzzDecode and FuzzCheckEdits never try it. The tests below re-encode
//     points of genuine states and hold the properties to them: one encoding
//     per state (audit R-008), and Check accepting no edit of a contribution.
//   - FuzzContribute mutates bytes, which almost never decode. FuzzPhase2Shapes
//     builds well-formed Phase 2 states of any shape (circuits, commitments,
//     slice lengths, challenge) and contributes to them (audit R-009).

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	curve "github.com/consensys/gnark-crypto/ecc/bn254"
	cmpc "github.com/consensys/gnark-crypto/ecc/bn254/mpcsetup"

	"enygma_ceremony/internal/fuzzedit"
)

// statePoint is where one point sits in a state file, and its group.
type statePoint struct {
	off int
	g2  bool
}

// phase1Points lists the points of a Phase 1 state of domain n in file
// order, and returns the offset of the challenge's length byte after them.
func phase1Points(n int) ([]statePoint, int) {
	off := len(magic) + 1 + 8 + 4
	var pts []statePoint
	for range 3 { // the update proofs of τ, α, β: [x]₁ then π in G2
		pts = append(pts, statePoint{off, false}, statePoint{off + 32, true})
		off += 96
	}
	off += 8                                 // gnark's domain
	pts = append(pts, statePoint{off, true}) // [β]₂
	off += 64
	for range 2*n - 2 { // [τⁱ]₁, i ≥ 1
		pts = append(pts, statePoint{off, false})
		off += 32
	}
	for range n - 1 { // [τⁱ]₂, i ≥ 1
		pts = append(pts, statePoint{off, true})
		off += 64
	}
	for range 2 * n { // [βτⁱ]₁ then [ατⁱ]₁
		pts = append(pts, statePoint{off, false})
		off += 32
	}
	return pts, off
}

// phase2Points lists the points of the Phase 2 state s in file order, and
// returns the offset just past the state.
func phase2Points(s *phase2State) ([]statePoint, int) {
	off := len(magic) + 1 + 1 + len(s.ceremony) + 32 + 4 + 2
	var pts []statePoint
	g1 := func() { pts = append(pts, statePoint{off, false}); off += 32 }
	g2 := func() { pts = append(pts, statePoint{off, true}); off += 64 }
	for i := range s.circuits {
		c := &s.circuits[i]
		off += 1 + len(c.name) + 32 + 2 // name, r1cs hash, gnark's commitment count
		p := &c.mpc.Parameters
		g1() // [δ]₁
		off += 4
		for range p.G1.PKK {
			g1()
		}
		off += 4
		for range p.G1.Z {
			g1()
		}
		g2() // [δ]₂
		for _, ckk := range p.G1.SigmaCKK {
			off += 4
			for range ckk {
				g1()
			}
		}
		for range p.G2.Sigma {
			g2()
		}
		g1() // δ's update proof
		g2()
		for range c.mpc.Sigmas {
			g1()
			g2()
		}
		off += 1 + len(c.mpc.Challenge)
	}
	return pts, off
}

// pointsOf lists the points of the state file data, checking the layout
// against the file so the tests cannot go vacuous on a format change.
func pointsOf(t testing.TB, data []byte) []statePoint {
	t.Helper()
	switch Kind(data[len(magic)]) {
	case KindPhase1:
		s := new(phase1State)
		if err := read(bytes.NewReader(data), int64(len(data)), KindPhase1, s); err != nil {
			t.Fatal(err)
		}
		pts, end := phase1Points(int(s.n))
		if end+1+int(data[end]) != len(data) {
			t.Fatalf("phase 1 layout: points end at %d, file is %d bytes", end, len(data))
		}
		return pts
	case KindPhase2:
		s := new(phase2State)
		if err := read(bytes.NewReader(data), int64(len(data)), KindPhase2, s); err != nil {
			t.Fatal(err)
		}
		pts, end := phase2Points(s)
		if end != len(data) {
			t.Fatalf("phase 2 layout: points end at %d, file is %d bytes", end, len(data))
		}
		return pts
	}
	t.Fatalf("no layout for kind %d", data[len(magic)])
	return nil
}

// uncompress returns data with the point at p written uncompressed, as
// RawBytes writes it: the same point, in the other encoding the decoder takes.
func uncompress(t testing.TB, data []byte, p statePoint) []byte {
	t.Helper()
	var raw []byte
	size := 32
	if p.g2 {
		size = 64
		var q curve.G2Affine
		if _, err := q.SetBytes(data[p.off : p.off+size]); err != nil {
			t.Fatalf("G2 point at %d: %v", p.off, err)
		}
		r := q.RawBytes()
		raw = r[:]
	} else {
		var q curve.G1Affine
		if _, err := q.SetBytes(data[p.off : p.off+size]); err != nil {
			t.Fatalf("G1 point at %d: %v", p.off, err)
		}
		r := q.RawBytes()
		raw = r[:]
	}
	out := append([]byte(nil), data[:p.off]...)
	out = append(out, raw...)
	return append(out, data[p.off+size:]...)
}

// TestOneEncoding (audit R-008): a genuine state with one point re-encoded
// uncompressed is the same state in another file, with another sha256; the
// decoder refuses it.
func TestOneEncoding(t *testing.T) {
	fx := newFixture(t)
	for _, path := range []string{fx.p1[1], fx.p2[1]} {
		data := readFile(t, path)
		pts := pointsOf(t, data)
		for _, i := range []int{0, len(pts) / 2, len(pts) - 1} {
			t.Run(fmt.Sprintf("%s point %d of %d", filepath.Base(path), i, len(pts)), func(t *testing.T) {
				edited := uncompress(t, data, pts[i])
				kind := Kind(edited[len(magic)])
				if err := read(bytes.NewReader(edited), int64(len(edited)), kind, codecFor(kind)); !errors.Is(err, ErrFormat) {
					t.Fatalf("a state of %d bytes, re-encoded: %v, want it refused as not its one encoding", len(edited), err)
				}
			})
		}
	}
}

// TestCheckRefusesReencodedContribution (audit R-008): a genuine
// contribution with one point written uncompressed is not the file its
// contributor wrote, and its sha256 is not the one they published.
func TestCheckRefusesReencodedContribution(t *testing.T) {
	fx := newFixture(t)
	for _, pair := range [][2]string{{fx.p1[0], fx.p1[1]}, {fx.p2[0], fx.p2[1]}} {
		genuine := readFile(t, pair[1])
		pts := pointsOf(t, genuine)
		edited := uncompress(t, genuine, pts[len(pts)-1])
		np := filepath.Join(t.TempDir(), "next")
		if err := os.WriteFile(np, edited, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := Check(pair[0], np); err == nil {
			t.Errorf("%s: Check accepts the contribution re-encoded (%d bytes, was %d)", filepath.Base(pair[1]), len(edited), len(genuine))
		}
	}
}

// FuzzReencode: FuzzCheckEdits with one more edit, a point written
// uncompressed (which = the point, uncompressed = whether), before the edit
// script. Check accepts the genuine contribution and nothing else, and a
// state that decodes writes back byte for byte.
func FuzzReencode(f *testing.F) {
	fx := newFixture(f)
	pairs := [][2]string{{fx.p1[0], fx.p1[1]}, {fx.p1[1], fx.p1[2]}, {fx.p2[0], fx.p2[1]}, {fx.p2[1], fx.p2[2]}}
	for i := range pairs {
		f.Add(uint8(i), uint16(0), false, []byte(nil))
		f.Add(uint8(i), uint16(7), true, []byte(nil))
	}
	f.Fuzz(func(t *testing.T, pair uint8, which uint16, uncompressed bool, script []byte) {
		p := pairs[int(pair)%len(pairs)]
		genuine := readFile(t, p[1])
		next := genuine
		if uncompressed {
			pts := pointsOf(t, genuine)
			next = uncompress(t, genuine, pts[int(which)%len(pts)])
		}
		next = fuzzedit.Apply(next, script)
		if len(next) > len(magic) {
			kind := Kind(next[len(magic)])
			s := codecFor(kind)
			if read(bytes.NewReader(next), int64(len(next)), kind, s) == nil {
				var again bytes.Buffer
				if err := write(&again, kind, s.encode); err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(again.Bytes(), next) {
					t.Fatalf("decodes, but writes back differently (%d bytes -> %d): two encodings", len(next), again.Len())
				}
			}
		}
		np := filepath.Join(t.TempDir(), "next")
		if err := os.WriteFile(np, next, 0o644); err != nil {
			t.Fatal(err)
		}
		err := Check(p[0], np)
		if err != nil && !refusal(err) {
			t.Fatalf("refused for no reason the package gives: %v", err)
		}
		if changed := !bytes.Equal(next, genuine); (err == nil) == changed {
			t.Fatalf("a contribution edited (%v) to %d bytes: Check gives %v", changed, len(next), err)
		}
	})
}

// phase2OfShape builds a well-formed Phase 2 state of the given shape:
// circuits circuits, each with commitments commitments of ckk committed
// points, z points of Z, pkk of [K]₁, and a challenge of chal bytes. Points
// are the generator or infinity, as sel says, so every one decodes.
func phase2OfShape(circuits, commitments, ckk, z, pkk, chal int, sel []byte) *phase2State {
	_, _, gen1, gen2 := curve.Generators()
	k := 0
	zero := func() bool {
		k++
		return len(sel) > 0 && sel[k%len(sel)]&1 == 1
	}
	pick1 := func() curve.G1Affine {
		if zero() {
			return curve.G1Affine{}
		}
		return gen1
	}
	pick2 := func() curve.G2Affine {
		if zero() {
			return curve.G2Affine{}
		}
		return gen2
	}
	g1s := func(n int) []curve.G1Affine {
		s := make([]curve.G1Affine, n)
		for i := range s {
			s[i] = pick1()
		}
		return s
	}
	s := &phase2State{ceremony: "shape", index: 1, circuits: make([]phase2Circuit, circuits)}
	for i := range s.circuits {
		c := &s.circuits[i]
		c.name = fmt.Sprintf("C%d", i)
		m := &c.mpc
		m.Parameters.G1.Delta = pick1()
		m.Parameters.G1.Z = g1s(z)
		m.Parameters.G1.PKK = g1s(pkk)
		m.Parameters.G2.Delta = pick2()
		m.Parameters.G1.SigmaCKK = make([][]curve.G1Affine, commitments)
		m.Parameters.G2.Sigma = make([]curve.G2Affine, commitments)
		for j := range commitments {
			m.Parameters.G1.SigmaCKK[j] = g1s(ckk)
			m.Parameters.G2.Sigma[j] = pick2()
		}
		m.Sigmas = make([]cmpc.UpdateProof, commitments)
		m.Challenge = make([]byte, chal)
	}
	return s
}

// encodePhase2 writes s as a Phase 2 state file.
func encodePhase2(t testing.TB, s *phase2State) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := write(&buf, KindPhase2, s.encode); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// FuzzPhase2Shapes: FuzzContribute's property (contributing to any state
// never panics; a refusal gives a reason) over well-formed Phase 2 states of
// any shape, which byte mutation of a fixture almost never yields (audit
// R-009). A participant contributes to whatever state its coordinator serves.
//
// The commitment count is twice a byte: Go's mutator moves an integer by at
// most 100 a step and keeps only steps with new coverage, so a uint16 count
// plateaus in the loop-count bucket of 128 and up. One seed sits below
// gnark's limit of 255, and one above it.
func FuzzPhase2Shapes(f *testing.F) {
	f.Add(uint8(1), uint8(0), uint8(1), uint8(3), uint8(2), uint8(0), []byte{0})
	f.Add(uint8(2), uint8(1), uint8(2), uint8(7), uint8(1), uint8(32), []byte{0, 1})
	f.Add(uint8(0), uint8(2), uint8(0), uint8(0), uint8(0), uint8(5), []byte(nil))
	f.Add(uint8(1), uint8(120), uint8(0), uint8(3), uint8(1), uint8(0), []byte(nil))
	f.Add(uint8(1), uint8(128), uint8(0), uint8(3), uint8(1), uint8(0), []byte(nil))
	f.Fuzz(func(t *testing.T, circuits, commitments, ckk, z, pkk, chal uint8, sel []byte) {
		s := phase2OfShape(int(circuits%3), min(2*int(commitments), 300), int(ckk%4), int(z%9), int(pkk%5), int(chal%40), sel)
		data := encodePhase2(t, s)
		var out bytes.Buffer
		if _, err := ContributeTo(bytes.NewReader(data), int64(len(data)), &out); err != nil && !refusal(err) {
			t.Fatalf("refused for no reason the package gives: %v", err)
		}
	})
}

// TestManyCommitments (audit R-009): gnark's Contribute panics on a circuit
// of more than 255 commitments, after drawing the randomness. Such a state
// is refused as it is decoded, so contributing to it fails with a reason and
// never panics; 255 is the control, and contributes.
func TestManyCommitments(t *testing.T) {
	for _, n := range []int{255, 256} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			data := encodePhase2(t, phase2OfShape(1, n, 0, 3, 1, 0, nil))
			path := filepath.Join(t.TempDir(), "state")
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatal(err)
			}
			_, inspectErr := Inspect(path)
			var out bytes.Buffer
			var err error
			panicked := func() (p any) {
				defer func() { p = recover() }()
				_, err = ContributeTo(bytes.NewReader(data), int64(len(data)), &out)
				return nil
			}()
			if panicked != nil {
				t.Fatalf("%d commitments: ContributeTo panics (%v)", n, panicked)
			}
			switch n {
			case 255:
				if inspectErr != nil || err != nil {
					t.Fatalf("255 commitments: Inspect %v, ContributeTo %v; want both to take it", inspectErr, err)
				}
			default:
				if !errors.Is(inspectErr, ErrFormat) || !errors.Is(err, ErrFormat) {
					t.Fatalf("%d commitments: Inspect %v, ContributeTo %v; want both to refuse it", n, inspectErr, err)
				}
			}
		})
	}
}

// TestCraftedPhase2Allocation (audit R-073) measures the worst case the
// decoder's bounds allow, as format.go's bounded describes it: a Phase 2 of
// zeros declaring as many circuits as its bytes could hold (minCircuit
// each), its first circuit as many commitments (minElement each). Each count
// passes its bound, and allocates more per element than an element takes in
// the file. It was 37.6 times before the circuits' own bound.
func TestCraftedPhase2Allocation(t *testing.T) {
	const rest = 1 << 20
	var b bytes.Buffer
	b.WriteString(magic)
	b.WriteByte(byte(KindPhase2))
	if err := putString(&b, "x"); err != nil {
		t.Fatal(err)
	}
	if err := put(&b, [32]byte{}, uint32(1)); err != nil {
		t.Fatal(err)
	}
	circuitsAt := b.Len()
	b.Write(make([]byte, 2))
	if err := putString(&b, "C"); err != nil {
		t.Fatal(err)
	}
	b.Write(make([]byte, 32)) // the constraint system's sha256
	commitmentsAt := b.Len()
	b.Write(make([]byte, 2+rest))
	data := b.Bytes()
	room := func(at, each int) uint16 { return uint16(min((len(data)-at-2)/each, 1<<16-1)) }
	binary.BigEndian.PutUint16(data[circuitsAt:], room(circuitsAt, minCircuit))
	binary.BigEndian.PutUint16(data[commitmentsAt:], room(commitmentsAt, minElement))

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	err := read(bytes.NewReader(data), int64(len(data)), KindPhase2, new(phase2State))
	runtime.ReadMemStats(&after)
	if !errors.Is(err, ErrFormat) {
		t.Fatalf("a crafted Phase 2: %v, want it refused", err)
	}
	ratio := float64(after.TotalAlloc-before.TotalAlloc) / float64(len(data))
	t.Logf("a crafted Phase 2 of %d bytes allocated %.1f times its size", len(data), ratio)
	if ratio > 64 { // FuzzDecode's bound
		t.Fatalf("a crafted Phase 2 of %d bytes allocated %.1f times its size", len(data), ratio)
	}
}

// TestNoCircuits (audit R-034): a Phase 2 of no circuits checks nothing and
// seals no key; it is refused as it is decoded.
func TestNoCircuits(t *testing.T) {
	data := encodePhase2(t, phase2OfShape(0, 0, 0, 0, 0, 0, nil))
	path := filepath.Join(t.TempDir(), "state")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Inspect(path); !errors.Is(err, ErrFormat) {
		t.Fatalf("a Phase 2 of no circuits: %v, want it refused", err)
	}
}
