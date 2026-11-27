package ceremony

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"time"
)

// Info describes a state file.
type Info struct {
	Kind     Kind
	Index    uint32   // Phase 1 and 2: contributions so far; SRS: contributions sealed
	N        uint64   // Phase 1 and SRS: the domain size
	Ceremony string   // Phase 2: the ceremony's name
	Circuits []string // Phase 2: the circuits, in order
	Hash     [32]byte // sha256 of the file
}

// Receipt reports one step: the state it wrote and, for a contribution, the
// state it built on. A participant publishes Hash, so anyone can find their
// contribution in the transcript.
type Receipt struct {
	Info          // the state written; Hash is its sha256
	In   [32]byte // sha256 of the state contributed to; zero when the step contributed nothing
}

// Inspect reads the state file at path and describes it.
func Inspect(path string) (Info, error) {
	kind, err := KindOf(path)
	if err != nil {
		return Info{}, err
	}
	switch kind {
	case KindPhase1:
		s := new(phase1State)
		h, err := open(path, kind, s.decode)
		return s.info(h), err
	case KindPhase2:
		s := new(phase2State)
		h, err := open(path, kind, s.decode)
		return s.info(h), err
	case KindSRS:
		s, h, err := readSRS(path)
		return Info{Kind: KindSRS, N: s.n, Hash: h}, err
	}
	return Info{}, fmt.Errorf("%s: %w: unknown %s", path, ErrFormat, kind)
}

// Contribute reads the Phase 1 or Phase 2 state at in, adds this machine's
// randomness, and writes the next state to out, which must not exist. A
// failed contribution leaves no file at out.
func Contribute(in, out string) (Receipt, error) {
	if _, err := os.Stat(out); err == nil {
		return Receipt{}, fmt.Errorf("%s already exists", out)
	}
	f, err := os.Open(in)
	if err != nil {
		return Receipt{}, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return Receipt{}, err
	}
	var r Receipt
	err = writeNew(out, func(w io.Writer) error {
		var err error
		r, err = ContributeTo(f, fi.Size(), w)
		return err
	})
	if err != nil {
		return Receipt{}, fmt.Errorf("%s: %w", in, err)
	}
	return r, nil
}

// contributable is a state a participant contributes to.
type contributable interface {
	decode(*body) error
	encode(io.Writer) error
	contribute()
	info(hash [32]byte) Info
}

// contribute adds this machine's randomness to every power of τ, α and β.
func (s *phase1State) contribute() {
	s.mpc.Contribute()
	s.index++
}

// contribute adds this machine's randomness to every circuit's δ and σs.
func (s *phase2State) contribute() {
	for i := range s.circuits {
		s.circuits[i].mpc.Contribute()
	}
	s.index++
}

// ContributeTo reads a Phase 1 or Phase 2 state of size bytes from in, adds
// this machine's randomness, and writes the next state to out: Contribute
// without files, as the participant's program contributes. gnark's mpcsetup draws the
// randomness from crypto/rand inside the call and keeps no copy of it;
// nothing of it outlives the process.
func ContributeTo(in io.Reader, size int64, out io.Writer) (Receipt, error) {
	return ContributeWithProgress(in, size, out, nil)
}

// The parts of a contribution, in order (Progress.Part).
const (
	PartReading     = "reading"     // decoding the state, checking every point
	PartRandomizing = "randomizing" // adding this machine's randomness to every point
	PartWriting     = "writing"     // encoding the next state
)

// Progress is how far a contribution has got.
type Progress struct {
	Part string
	// Done and Total are the bytes read or written so far, for PartReading
	// and PartWriting.
	Done, Total int64
	// Fraction is how much of the whole contribution is done, from 0 to 1,
	// weighing each part by how long it takes.
	Fraction float64
	// Estimate is how long PartRandomizing should take, and Until the
	// Fraction done when it ends. gnark reports no progress while it
	// randomizes, so a progress bar moves from Fraction to Until over
	// Estimate. Estimate is judged from how long reading took on this
	// machine just now, so a slow or throttled machine gets a longer one.
	Estimate time.Duration
	Until    float64
}

// partsPerReading is how many times longer randomizing, and writing, take
// than reading a state, by kind. Measured natively on the real-size states:
//
//	Phase 1 at 2^18:           randomizing 2.64× reading; writing under 0.01×
//	retail's Phase 2:          6.1×; writing 0.02×
//	auctions' Phase 2:         8.1×; writing 0.02-0.04×
var partsPerReading = map[Kind]struct{ randomizing, writing float64 }{
	KindPhase1: {2.6, 0.01},
	KindPhase2: {7, 0.03},
}

// shares returns the fraction of a contribution to a state of kind that
// reading takes, and that randomizing takes.
func shares(kind Kind) (reading, randomizing float64) {
	r := partsPerReading[kind]
	total := 1 + r.randomizing + r.writing
	return 1 / total, r.randomizing / total
}

// ContributeWithProgress is ContributeTo reporting its progress to report,
// when report is not nil: reading and writing by the byte, about every 1%,
// and randomizing once, as it starts, with an estimate of how long it takes.
func ContributeWithProgress(in io.Reader, size int64, out io.Writer, report func(Progress)) (Receipt, error) {
	if report == nil {
		report = func(Progress) {}
	}
	inHash := sha256.New()
	started := time.Now()
	pr := &progressReader{r: in, total: size, report: report}
	br := bufio.NewReaderSize(io.TeeReader(pr, inHash), 1<<20)
	head, err := br.Peek(len(magic) + 1)
	if err != nil || string(head[:len(magic)]) != magic {
		return Receipt{}, ErrFormat
	}
	var s contributable
	kind := Kind(head[len(magic)])
	switch kind {
	case KindPhase1:
		s = new(phase1State)
	case KindPhase2:
		s = new(phase2State)
	default:
		return Receipt{}, fmt.Errorf("%w: a %s takes no contribution", ErrKind, kind)
	}
	readingShare, randomizingShare := shares(kind)
	pr.share, pr.from = readingShare, 0
	if err := read(br, size, kind, s.decode); err != nil {
		return Receipt{}, err
	}
	reading := time.Since(started)
	report(Progress{
		Part: PartRandomizing, Fraction: readingShare, Until: readingShare + randomizingShare,
		Estimate: time.Duration(float64(reading) * partsPerReading[kind].randomizing),
	})
	s.contribute()
	outHash := sha256.New()
	pw := &progressWriter{w: out, total: size, report: report, from: readingShare + randomizingShare, share: 1 - readingShare - randomizingShare}
	w := bufio.NewWriterSize(io.MultiWriter(pw, outHash), 1<<20)
	if err := write(w, kind, s.encode); err != nil {
		return Receipt{}, err
	}
	if err := w.Flush(); err != nil {
		return Receipt{}, err
	}
	report(Progress{Part: PartWriting, Done: pw.done, Total: pw.done, Fraction: 1})
	var r Receipt
	copy(r.In[:], inHash.Sum(nil))
	var h [32]byte
	copy(h[:], outHash.Sum(nil))
	r.Info = s.info(h)
	return r, nil
}

// Check verifies that the state at next is one valid contribution on top of
// the state at prev: of the same ceremony, numbered next, and an update gnark's
// mpcsetup proves correct. The coordinator runs it on every upload before
// handing the state on; a participant can run it on their own step.
func Check(prev, next string) error {
	kind, err := KindOf(prev)
	if err != nil {
		return err
	}
	switch kind {
	case KindPhase1:
		p, q := new(phase1State), new(phase1State)
		if _, err := open(prev, kind, p.decode); err != nil {
			return err
		}
		if _, err := open(next, kind, q.decode); err != nil {
			return err
		}
		return checkPhase1Step(p, q)
	case KindPhase2:
		p, q := new(phase2State), new(phase2State)
		if _, err := open(prev, kind, p.decode); err != nil {
			return err
		}
		if _, err := open(next, kind, q.decode); err != nil {
			return err
		}
		return checkPhase2Step(p, q)
	}
	return fmt.Errorf("%s: %w: a %s is not a contribution", prev, ErrKind, kind)
}

// progressReader reports the bytes read through it, about every 1% of total,
// as the part of the contribution from from to from+share.
type progressReader struct {
	r           io.Reader
	done, total int64
	next        int64 // the count at which to report next
	from, share float64
	report      func(Progress)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.done += int64(n)
	if p.done >= p.next || err == io.EOF {
		done := min(p.done, p.total)
		p.report(Progress{Part: PartReading, Done: done, Total: p.total, Fraction: p.from + p.share*float64(done)/float64(max(p.total, 1))})
		p.next = p.done + max(p.total/100, 1)
	}
	return n, err
}

// progressWriter reports the bytes written through it, about every 1% of
// total (the size of the state contributed to, which the next state's size
// is within a few bytes of), as the part from from to from+share.
type progressWriter struct {
	w           io.Writer
	done, total int64
	next        int64
	from, share float64
	report      func(Progress)
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	p.done += int64(n)
	if p.done >= p.next {
		done := min(p.done, p.total)
		p.report(Progress{Part: PartWriting, Done: done, Total: p.total, Fraction: p.from + p.share*float64(done)/float64(max(p.total, 1))})
		p.next = p.done + max(p.total/100, 1)
	}
	return n, err
}
