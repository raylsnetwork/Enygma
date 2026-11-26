package ceremony

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"enygma_ceremony/internal/fuzzedit"
)

// The fuzz targets below run their seeds with every go test; make
// ceremony-fuzz runs each for longer, searching for inputs that fail them.
// A state file comes from whoever uploaded it, or from whoever serves the
// transcript a verifier checks, so no bytes may crash what reads it.

// codec is a kind of state: how it reads and writes its body.
type codec interface {
	decode(*body) error
	encode(io.Writer) error
}

// codecFor returns an empty state of kind to decode into; phase 1 for a kind
// no state has, which read then refuses.
func codecFor(kind Kind) codec {
	switch kind {
	case KindPhase2:
		return new(phase2State)
	case KindSRS:
		return new(srsFile)
	}
	return new(phase1State)
}

// refusal reports whether err is one of the package's reasons to refuse a
// state, rather than an error no caller could explain.
func refusal(err error) bool {
	for _, e := range []error{ErrFormat, ErrKind, ErrDomain, ErrIndex, ErrMismatch, ErrInvalidContribution} {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}

// FuzzDecode: no bytes make a state's decoder panic, or allocate more than a
// small multiple of their size (gnark's decoders allocate what a state's
// counts say before reading it); every refusal gives a reason; and bytes that
// decode write back byte for byte, so a state has one encoding, and one
// sha256, the transcript's name for it.
func FuzzDecode(f *testing.F) {
	fx := newFixture(f)
	for _, p := range []string{fx.p1[0], fx.p1[2], fx.srs, fx.p2[0], fx.p2[3]} {
		f.Add(readFile(f, p))
	}
	// gnark's domain, after the header, n, index and three update proofs (as
	// in TestCraftedCountsAreRefusedBeforeAllocating): set as large as fits,
	// then to 0, which gnark sizes its powers from.
	domainAt := len(magic) + 1 + 8 + 4 + 3*(32+64)
	for _, domain := range []uint64{1<<63 - 1, 0} {
		data := readFile(f, fx.p1[1])
		binary.BigEndian.PutUint64(data[domainAt:], domain)
		f.Add(data)
	}
	// Phase 2's first circuit's count of [K]₁ (as in
	// TestCraftedCountsAreRefusedBeforeAllocating), set to 4 million points:
	// 256 MiB, from a file of a few kilobytes.
	p2 := new(phase2State)
	if _, err := open(fx.p2[1], KindPhase2, p2.decode); err != nil {
		f.Fatal(err)
	}
	kAt := len(magic) + 1 + 1 + len(p2.ceremony) + 32 + 4 + 2 + 1 + len(p2.circuits[0].name) + 32 + 2 + 32
	data := readFile(f, fx.p2[1])
	binary.BigEndian.PutUint32(data[kAt:], 1<<22)
	f.Add(data)
	f.Add(readFile(f, fx.p1[1])[:len(magic)+1+6]) // cut short in its domain size
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		kind := KindPhase1
		if len(data) > len(magic) {
			kind = Kind(data[len(magic)])
		}
		s := codecFor(kind)
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		err := read(bytes.NewReader(data), int64(len(data)), kind, s.decode)
		runtime.ReadMemStats(&after)
		if grew := after.TotalAlloc - before.TotalAlloc; grew > 64*uint64(len(data))+8<<20 {
			t.Fatalf("decoding %d bytes allocated %d", len(data), grew)
		}
		if err != nil {
			if !refusal(err) {
				t.Fatalf("refused for no reason the package gives: %v", err)
			}
			return
		}
		var again bytes.Buffer
		if err := write(&again, kind, s.encode); err != nil {
			t.Fatalf("decoded, but does not write back: %v", err)
		}
		if !bytes.Equal(again.Bytes(), data) {
			t.Fatalf("decoded, but writes back differently: a state with two encodings")
		}
	})
}

// FuzzCheck: Check never panics, whatever the two files hold, refuses for a
// reason, and accepts only a step: a different file of the same kind,
// numbered one past the state it builds on. FuzzCheckEdits holds Check to more.
func FuzzCheck(f *testing.F) {
	fx := newFixture(f)
	for _, chain := range [][]string{fx.p1, fx.p2} {
		for i := range len(chain) - 1 {
			f.Add(readFile(f, chain[i]), readFile(f, chain[i+1]))
		}
	}
	// The second contribution, numbered as the first: a step in order, on
	// top of the wrong state.
	skipped := readFile(f, fx.p1[2])
	binary.BigEndian.PutUint32(skipped[len(magic)+1+8:], 1)
	f.Add(readFile(f, fx.p1[0]), skipped)
	f.Fuzz(func(t *testing.T, prev, next []byte) {
		dir := t.TempDir()
		pp, np := filepath.Join(dir, "prev"), filepath.Join(dir, "next")
		if err := os.WriteFile(pp, prev, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(np, next, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := Check(pp, np); err != nil {
			if !refusal(err) {
				t.Fatalf("refused for no reason the package gives: %v", err)
			}
			return
		}
		pi, perr := Inspect(pp)
		ni, nerr := Inspect(np)
		if perr != nil || nerr != nil || bytes.Equal(prev, next) || ni.Kind != pi.Kind || ni.Index != pi.Index+1 {
			t.Fatalf("accepted %+v (%v) on top of %+v (%v)", ni, nerr, pi, perr)
		}
	})
}

// FuzzCheckEdits: on top of a state, Check accepts a contribution made on it
// and nothing edited from it: whatever bytes the edits change, add or take
// away, the file no longer passes. A fuzzer cannot make a contribution of
// its own (that takes the arithmetic of one), so a pass would be a hole in
// the check: as a contribution stripped of the challenge naming the state it
// builds on passed gnark's Verify. The fuzzer mutates the edits (package
// fuzzedit), so the property holds in each of its processes, each of which
// makes its own contributions.
func FuzzCheckEdits(f *testing.F) {
	fx := newFixture(f)
	pairs := [][2]string{{fx.p1[0], fx.p1[1]}, {fx.p1[1], fx.p1[2]}, {fx.p2[0], fx.p2[1]}, {fx.p2[1], fx.p2[2]}, {fx.p2[2], fx.p2[3]}}
	for i := range pairs {
		f.Add(uint8(i), []byte(nil))
		f.Add(uint8(i), fuzzedit.Op(1, 1000, 0))
	}
	// A phase 1 state ends with its challenge, a byte of length and 32 bytes:
	// say it has none, and take it away.
	n := len(readFile(f, fx.p1[2]))
	f.Add(uint8(1), append(fuzzedit.Op(0, n-33, 0), fuzzedit.Op(2, n-32, 31)...))
	f.Fuzz(func(t *testing.T, pair uint8, script []byte) {
		p := pairs[int(pair)%len(pairs)]
		genuine := readFile(t, p[1])
		next := fuzzedit.Apply(genuine, script)
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

// FuzzContribute: contributing to any bytes never panics: a participant's
// program contributes to whatever state the coordinator serves it, and the
// browser's would stop the page's worker. What it refuses, it refuses for a
// reason, having written nothing.
func FuzzContribute(f *testing.F) {
	fx := newFixture(f)
	for _, p := range []string{fx.p1[1], fx.p2[2], fx.srs} {
		f.Add(readFile(f, p))
	}
	f.Add([]byte("not a state"))
	f.Fuzz(func(t *testing.T, data []byte) {
		var out bytes.Buffer
		if _, err := ContributeTo(bytes.NewReader(data), int64(len(data)), &out); err != nil && !refusal(err) {
			t.Fatalf("refused for no reason the package gives: %v", err)
		}
	})
}
