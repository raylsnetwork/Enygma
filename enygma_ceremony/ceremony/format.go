// Package ceremony runs Enygma's multi-party Groth16 setup on gnark's
// mpcsetup (the two-phase MPC of Bowe, Gabizon and Miers, eprint 2017/1050).
//
// Phase 1, the powers of τ, runs once and serves every circuit of up to its
// domain size. Phase 2 runs once per protocol, for all of that protocol's
// circuits at once. Each step reads one state file and writes the next, so a
// participant needs nothing but the state before theirs; the coordinator
// checks each step on its own (Check), and anyone can replay a whole phase
// from its start (SealPhase1, SealPhase2). The keys are sound as long as one
// participant in each phase discarded their randomness.
package ceremony

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

// magic opens every state file; its last byte is the format version.
const magic = "ENYGMA-CEREMONY\x01"

// Kind is what a state file holds.
type Kind uint8

const (
	// KindPhase1 is Phase 1 after some contributions: the powers of τ, α and β.
	KindPhase1 Kind = 1
	// KindPhase2 is Phase 2 after some contributions: each circuit's δ, and its σ per commitment.
	KindPhase2 Kind = 2
	// KindSRS is the sealed Phase 1 that every Phase 2 starts from.
	KindSRS Kind = 3
)

// String names the kind as the command prints it.
func (k Kind) String() string {
	switch k {
	case KindPhase1:
		return "phase 1"
	case KindPhase2:
		return "phase 2"
	case KindSRS:
		return "SRS"
	}
	return fmt.Sprintf("kind %d", uint8(k))
}

var (
	// ErrFormat is a file that is not a state file of this format, or is cut short or padded.
	ErrFormat = errors.New("not a ceremony state file")
	// ErrKind is a state file of another kind than the step takes.
	ErrKind = errors.New("wrong kind of state file")
	// ErrIndex is a contribution that does not directly follow the state before it.
	ErrIndex = errors.New("contribution out of order")
	// ErrMismatch is two states of different ceremonies: another domain size,
	// ceremony name, SRS or set of circuits.
	ErrMismatch = errors.New("states of different ceremonies")
	// ErrInvalidContribution is a contribution whose update gnark's mpcsetup
	// cannot verify against the state before it.
	ErrInvalidContribution = errors.New("invalid contribution")
	// ErrBeacon is a beacon shorter than MinBeacon bytes.
	ErrBeacon = errors.New("beacon too short")
	// ErrTooLarge is a circuit that needs a larger domain than the SRS has.
	ErrTooLarge = errors.New("circuit too large for the SRS")
	// ErrDomain is a Phase 1 domain size that is not a power of two from 2 to 2^28.
	ErrDomain = errors.New("domain size is not a power of two from 2 to 2^28")
)

// MinBeacon is the shortest beacon a seal takes, in bytes. A drand round's
// randomness is 32.
const MinBeacon = 16

// maxDomain is the largest domain BN254's FFT supports.
const maxDomain = 1 << 28

// DefaultMaxState is the most a participant's program or a verifier
// downloads as one state, unless told otherwise (join and verify -max-state):
// room for the largest state the ceremony plans, a Phase 1 of 2^18 (50 MB).
// A decoder allocates at most a small multiple of a state's size
// (body.bounded), so the bound on a state's size bounds what a coordinator
// can make a participant allocate (audit R-037): about 1.5 GiB at this one,
// where 1 GiB states allowed tens of GiB.
const DefaultMaxState = 64 << 20

// checkBeacon accepts a beacon of at least MinBeacon bytes.
func checkBeacon(beacon []byte) error {
	if len(beacon) < MinBeacon {
		return fmt.Errorf("%w: %d bytes, want at least %d", ErrBeacon, len(beacon), MinBeacon)
	}
	return nil
}

// checkDomain accepts a Phase 1 domain size: a power of two from 2 to maxDomain.
func checkDomain(n uint64) error {
	if n < 2 || n > maxDomain || n&(n-1) != 0 {
		return fmt.Errorf("%w: %d", ErrDomain, n)
	}
	return nil
}

// minElement is the fewest bytes an element a state counts takes in the
// file: a compressed point of BN254's G1.
const minElement = 32

// body is the reader a state file is decoded from. It fills every Read
// completely or fails: gnark reads a state's challenge with one bare Read
// (gnark/io.ReadBytesShort), which a buffered reader may answer short at a
// buffer boundary, misaligning the rest of the state without an error. And
// it counts the bytes of the file still to come, for bounded.
type body struct {
	r    io.Reader
	left int64 // bytes of the file not read yet
}

// Read reads exactly len(p) bytes, as io.ReadFull does.
func (b *body) Read(p []byte) (int, error) {
	n, err := io.ReadFull(b.r, p)
	b.left -= int64(n)
	return n, err
}

// bounded returns b as the reader for gnark's decoders, and for the counts a
// state declares itself. A decoder allocates as many elements as the file
// says before reading them, and every count in a state is a 2-, 4- or 8-byte
// big-endian integer read on its own, of elements of at least minElement
// bytes each; so bounded refuses any such integer larger than the bytes left
// could hold. A crafted file then makes its decoder allocate at most a small
// multiple of its own size, where it could otherwise ask for terabytes in its
// first few bytes. A crafted Phase 2 is the worst, at 23 times as measured
// (TestCraftedPhase2Allocation: gnark allocates each commitment's points
// before reading any), with its circuit count held to minCircuit bytes each;
// a Phase 1 about 15. FuzzDecode holds every state to 64 times.
func (b *body) bounded() io.Reader { return countsBounded{b} }

// countsBounded is a body whose integer reads are checked as bounded says.
type countsBounded struct{ b *body }

// Read reads exactly len(p) bytes, and refuses them if they are a count
// larger than the rest of the file could hold. A refusal reports no bytes
// read: io.ReadFull, which gnark reads through, drops the error of a Read
// that filled its buffer.
func (c countsBounded) Read(p []byte) (int, error) {
	n, err := c.b.Read(p)
	if err != nil {
		return n, err
	}
	var count uint64
	switch len(p) {
	case 2:
		count = uint64(binary.BigEndian.Uint16(p))
	case 4:
		count = uint64(binary.BigEndian.Uint32(p))
	case 8:
		count = binary.BigEndian.Uint64(p)
	default:
		return n, nil
	}
	if room := uint64(max(c.b.left, 0)) / minElement; count > room {
		return 0, fmt.Errorf("%w: a count of %d, where %d bytes are left", ErrFormat, count, c.b.left)
	}
	return n, nil
}

// domainOf returns r, the reader for a gnark Phase 1 or SRS, refusing the
// domain size gnark reads for it unless it is n, the state's own. gnark
// sizes the state from its domain alone (2N-1 powers of τ in G1, and N of
// each other kind) before reading any of it, and its first 8-byte read is
// that domain: its update proofs before it are points, read 32 or 64 bytes
// at a time. Bound to n, which checkDomain has checked, a crafted domain of 0
// or 1 can no longer reach gnark's arithmetic, which then panics (2N-1 out of
// range; an index past the only power).
func domainOf(r io.Reader, n uint64) io.Reader { return &domainBound{r: r, n: n} }

// domainBound is a reader whose first 8-byte read must be n, as domainOf says.
type domainBound struct {
	r    io.Reader
	n    uint64
	seen bool
}

// Read reads through to r; a refusal reports no bytes read, as countsBounded's does.
func (d *domainBound) Read(p []byte) (int, error) {
	n, err := d.r.Read(p)
	if err != nil || d.seen || len(p) != 8 {
		return n, err
	}
	d.seen = true
	if got := binary.BigEndian.Uint64(p); got != d.n {
		return 0, fmt.Errorf("%w: gnark's domain size is %d, the state's %d", ErrFormat, got, d.n)
	}
	return n, nil
}

// write writes a state file of kind to w: the header, then the state.
func write(w io.Writer, kind Kind, encode func(io.Writer) error) error {
	if _, err := io.WriteString(w, magic); err != nil {
		return err
	}
	if _, err := w.Write([]byte{byte(kind)}); err != nil {
		return err
	}
	return encode(w)
}

// stateCodec is a state that reads and writes its file's body.
type stateCodec interface {
	decode(*body) error
	encode(io.Writer) error
}

// read reads a state file of kind, size bytes long, from r into s, which must
// consume it exactly. The file must be the one encoding of what it holds
// (audit R-008): gnark also decodes points written uncompressed, so one state
// could otherwise have several files, and several sha256s, while the
// transcript, receipts and every seal name a state by its file's sha256.
func read(r io.Reader, size int64, kind Kind, s stateCodec) error {
	in := sha256.New()
	b := &body{r: io.TeeReader(r, in), left: size}
	got, err := readHeader(b)
	if err != nil {
		return err
	}
	if got != kind {
		return fmt.Errorf("%w: holds %s, want %s", ErrKind, got, kind)
	}
	if err := s.decode(b); err != nil {
		return err
	}
	if _, err := b.Read(make([]byte, 1)); err != io.EOF {
		return fmt.Errorf("%w: data after the state", ErrFormat)
	}
	again := sha256.New()
	if err := write(again, kind, s.encode); err != nil {
		return err
	}
	if !bytes.Equal(in.Sum(nil), again.Sum(nil)) {
		return fmt.Errorf("%w: not the state's one encoding (points are written compressed)", ErrFormat)
	}
	return nil
}

// readHeader reads the magic and kind that open a state file.
func readHeader(r io.Reader) (Kind, error) {
	head := make([]byte, len(magic)+1)
	if _, err := io.ReadFull(r, head); err != nil || string(head[:len(magic)]) != magic {
		return 0, ErrFormat
	}
	return Kind(head[len(magic)]), nil
}

// create writes a new state file of kind at path, never over an existing
// file, and returns its sha256. A failed write leaves no file behind.
func create(path string, kind Kind, encode func(io.Writer) error) (digest [32]byte, err error) {
	h := sha256.New()
	err = writeNew(path, func(w io.Writer) error { return write(io.MultiWriter(w, h), kind, encode) })
	copy(digest[:], h.Sum(nil))
	return digest, err
}

// writeNew writes a new file at path through fill, never over an existing
// file, and syncs it. A failed write leaves no file behind.
func writeNew(path string, fill func(io.Writer) error) (err error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			f.Close()
			err = errors.Join(err, os.Remove(path))
		}
	}()
	w := bufio.NewWriterSize(f, 1<<20)
	if err = fill(w); err != nil {
		return err
	}
	if err = w.Flush(); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	return f.Close()
}

// open reads the state file of kind at path into s and returns the file's
// sha256.
func open(path string, kind Kind, s stateCodec) (digest [32]byte, err error) {
	f, err := os.Open(path)
	if err != nil {
		return digest, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return digest, err
	}
	h := sha256.New()
	if err := read(bufio.NewReaderSize(io.TeeReader(f, h), 1<<20), st.Size(), kind, s); err != nil {
		return digest, fmt.Errorf("%s: %w", path, err)
	}
	copy(digest[:], h.Sum(nil))
	return digest, nil
}

// KindOf reads, from its header alone, which kind of state the file at path holds.
func KindOf(path string) (Kind, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	k, err := readHeader(f)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", path, err)
	}
	return k, nil
}

// put writes fixed-size values, big-endian.
func put(w io.Writer, vs ...any) error {
	for _, v := range vs {
		if err := binary.Write(w, binary.BigEndian, v); err != nil {
			return err
		}
	}
	return nil
}

// get reads fixed-size values as put wrote them.
func get(r io.Reader, vs ...any) error {
	for _, v := range vs {
		if err := binary.Read(r, binary.BigEndian, v); err != nil {
			return fmt.Errorf("%w: %v", ErrFormat, err)
		}
	}
	return nil
}

// putString writes s after its one-byte length.
func putString(w io.Writer, s string) error {
	if len(s) > 255 {
		return fmt.Errorf("%q is longer than 255 bytes", s)
	}
	if err := put(w, uint8(len(s))); err != nil {
		return err
	}
	_, err := io.WriteString(w, s)
	return err
}

// getString reads a string as putString wrote it.
func getString(r io.Reader) (string, error) {
	var n uint8
	if err := get(r, &n); err != nil {
		return "", err
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(r, b); err != nil {
		return "", fmt.Errorf("%w: %v", ErrFormat, err)
	}
	return string(b), nil
}
