package utils

import (
	"fmt"
	"io"
	"log"
	"math/big"
	"os"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/backend/witness"
	"github.com/iden3/go-iden3-crypto/babyjub"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

var (
	A = big.NewInt(168700)
	D = big.NewInt(168696)

	hx, _    = new(big.Int).SetString("10100005861917718053548237064487763771145251762383025193119768015180892676690", 10)
	hy, _    = new(big.Int).SetString("7512830269827713629724023825249861327768672768516116945507944076335453576011", 10)
	HBabyJub = &babyjub.Point{X: hx, Y: hy}

	P, _ = new(big.Int).SetString("2736030358979909402780800718157159386076813972158567259200215660948447373041", 10)

	// CircuitGBabyJub is the G generator used inside the gnark circuit's
	// PedersenCommitment, and the only Pedersen value generator this
	// package exposes — must match utils/circuits.go var G. NUMS
	// hash-to-curve derivation, seed "2" (H-11 fix — matches HBabyJub's
	// derivation from seed "1" above; see cmd/derive_generator and nums.go).
	// Reproduce with: go run ./cmd/derive_generator
	//
	// Fix L-15: this file used to also declare GBabyJub (the standard
	// iden3 base point B8) alongside three helpers built from it —
	// GetPK, GetH, PedersenCommitmentBabyJub — none of which were called
	// from anywhere (confirmed: GetPK's only caller was
	// PedersenCommitmentBabyJub, called by nothing; B8 appeared nowhere
	// else in the repo). Every live commitment site already used
	// CircuitGBabyJub (utils/circuits.go, CurveBabyJubJub.sol,
	// go_client/internal/curve/curve.go, go_client/utils/utils.go [since
	// deleted, Fix L-13], demo/main.go, enygma_fee/handler.go) — a
	// maintainer reaching for the obviously-named
	// utils.PedersenCommitmentBabyJub would silently have committed
	// against the wrong generator. Deleted rather than fixed in place:
	// there is no live caller to preserve, and PedersenCommitment (the
	// in-circuit gadget in utils/circuits.go, built on CircuitGBabyJub/
	// HBabyJub) already covers every real use.
	circuitGx, _    = new(big.Int).SetString("12337812418750581066638756637363471856433191340622504180842886595232027947307", 10)
	circuitGy, _    = new(big.Int).SetString("15225366398330386329633463986700597127113326976080712967801565482915963669722", 10)
	CircuitGBabyJub = &babyjub.Point{X: circuitGx, Y: circuitGy}
)

// Fix L-15: GetPkHash discarded poseidon.Hash's error — Hash only
// actually errors on a malformed input width, which sk/sk never is, but
// silently returning a nil *big.Int on any future misuse would surface
// as a confusing downstream panic rather than here, at the point of the
// actual failure.
func GetPkHash(sk *big.Int) (*big.Int, error) {
	hash, err := poseidon.Hash([]*big.Int{sk, sk})
	if err != nil {
		return nil, fmt.Errorf("poseidon hash: %w", err)
	}
	return hash, nil
}

func AddPks(pk1 *babyjub.Point, pk2 *babyjub.Point) *babyjub.Point {
	return babyjub.NewPoint().Projective().Add(pk1.Projective(), pk2.Projective()).Affine()
}

func LoadProvingKey(curve ecc.ID, filename string) (groth16.ProvingKey, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	pk := groth16.NewProvingKey(curve) // e.g., ecc.BN254
	_, err = pk.ReadFrom(file)
	return pk, err
}

// Load verifying key from file
func LoadVerifyingKey(curve ecc.ID, filename string) (groth16.VerifyingKey, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	vk := groth16.NewVerifyingKey(curve) // e.g., ecc.BN254
	_, err = vk.ReadFrom(file)
	return vk, err
}

// MustLoadKeys loads a proving key and its matching verifying key, exiting
// the process immediately if either is missing or unreadable.
//
// Fix L-04 part 1: every handler used to do `pk, _ := LoadProvingKey(...)`,
// discarding the error. A missing or unreadable key file (every key path
// in config.go is relative to gnark-server/, so starting the process from
// anywhere else reproduces this trivially) left pk a nil interface; the
// server still printed its healthy startup banner, and the first real
// proof request panicked inside groth16.Prove, recovered into a generic
// 500 forever — the failure was silent at the only point it could have
// been caught cheaply. Failing fast here turns a bad key path into an
// immediate, unambiguous boot-time crash instead.
func MustLoadKeys(curve ecc.ID, pkPath, vkPath string) (groth16.ProvingKey, groth16.VerifyingKey) {
	pk, err := LoadProvingKey(curve, pkPath)
	if err != nil {
		log.Fatalf("load proving key %q: %v", pkPath, err)
	}
	vk, err := LoadVerifyingKey(curve, vkPath)
	if err != nil {
		log.Fatalf("load verifying key %q: %v", vkPath, err)
	}
	return pk, vk
}

// SelfVerify re-verifies a freshly generated proof against its own public
// witness before a handler returns it to the caller.
//
// Fix L-04 part 2: `grep -rn vkPath pkg/` returned only parameter
// declarations — no handler ever called LoadVerifyingKey or groth16.Verify,
// so the vkPath every handler already accepted was silently ignored. A
// mismatched or corrupted pk/vk pair (e.g. one regenerated, one stale)
// would only have been discovered by whoever received the bad proof next
// — potentially after it was already submitted on-chain. This closes that
// gap at the one place the server can catch it for free: right after it
// generates the proof, before it ever leaves the process.
func SelfVerify(proof groth16.Proof, vk groth16.VerifyingKey, witnessFull witness.Witness) error {
	publicWitness, err := witnessFull.Public()
	if err != nil {
		return fmt.Errorf("extract public witness: %w", err)
	}
	if err := groth16.Verify(proof, vk, publicWitness); err != nil {
		return fmt.Errorf("self-verification failed: %w", err)
	}
	return nil
}

// ModHint is the hint-solver half of ReduceModP (utils/circuits.go):
// given value, it suggests (r, q) = (value % P, value / P), which
// ReduceModP's own in-circuit constraints then verify (q*P + r == value,
// r < P, q < 8) — the hint itself is untrusted input to the circuit, not
// a source of soundness, which is why it can freely use big.Int.DivMod
// rather than anything constant-time.
//
// Fix L-15: this used to re-parse the JubJub subgroup order from a
// second, separately-written copy of the same literal already declared
// as the package-level P above — one of five copies of that constant
// across the repo the audit counted, with no single source of truth.
// Using P here removes this one. The `mod` parameter is gnark's hint
// signature convention (github.com/consensys/gnark/constraint/solver.Hint):
// it is the SNARK's native scalar field modulus (BN254's r), supplied
// automatically by the solver — a different, unrelated modulus from the
// JubJub subgroup order P this hint actually reduces by, so it is
// correctly unused here, not merely uses the wrong name for it. Also
// removed: an unreachable second `return nil` after the first.
func ModHint(mod *big.Int, inputs []*big.Int, res []*big.Int) error {
	value := inputs[0]
	q := new(big.Int)
	r := new(big.Int)

	q.DivMod(value, P, r) // q = value / P, r = value % P

	res[0] = r // remainder
	res[1] = q // quotient
	return nil
}

// ParseBigInt parses a base-10 string into a *big.Int.
//
// Fix M-08: this used to discard SetString's ok flag and return nil for
// any non-decimal input. That nil reached frontend.NewWitness, which
// errors out — but gnark's witness.Fill returns on its first error
// without draining or closing its unbuffered internal channel, which
// permanently blocks the producer goroutine holding the whole circuit
// assignment (gnark's own source comments "we may leek a chan + producer
// go routine"). One malformed ~200-byte POST leaked one goroutine
// forever, with no self-healing short of a restart. Returning the error
// here — and having every handler check it via BigIntParser before ever
// calling frontend.NewWitness (the audit's own remediation: "this alone
// removes the leak trigger") — means a malformed request is rejected on
// the cheap validation path and never reaches the code that leaks.
func ParseBigInt(s string) (*big.Int, error) {
	n, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return nil, fmt.Errorf("invalid decimal integer %q", s)
	}
	return n, nil
}

// BigIntParser accumulates the first error across many ParseBigInt calls
// so a handler can build a witness field-by-field in its existing style
// and check once — before frontend.NewWitness — whether every field
// actually parsed (Fix M-08; see ParseBigInt's doc for why that specific
// ordering matters).
type BigIntParser struct{ err error }

// Parse parses s, remembering the first failure. On failure it returns 0
// rather than nil — a caller MUST check Err() before using any witness
// built this way; the zero placeholder exists only so intermediate
// assignments don't need their own nil checks.
func (p *BigIntParser) Parse(s string) *big.Int {
	n, err := ParseBigInt(s)
	if err != nil {
		if p.err == nil {
			p.err = err
		}
		return big.NewInt(0)
	}
	return n
}

// Err returns the first parse error encountered, or nil if every Parse
// call so far succeeded.
func (p *BigIntParser) Err() error { return p.err }

// SavingFiles writes pk and vk to pkFile and vkFile.
//
// Fix L-15: this used to log.Fatalf on every failure and could therefore
// only ever return nil — generate_keys.go's own `fmt.Errorf(...: %w, err)`
// wrapper around this call was unreachable, and (worse) a failure partway
// through — e.g. the verifying key's disk write failing after the proving
// key's had already succeeded — left a partial, mismatched key pair on
// disk that a later run would happily load with no indication anything
// was wrong. Now every failure returns an error instead of exiting the
// process, and each file is written to a sibling temp file and renamed
// into place only on a fully successful write — `os.Rename` is atomic on
// the same filesystem, so a process killed mid-write, or an error on
// either file, leaves the previous good key set (or nothing) at pkFile/
// vkFile, never a truncated one.
func SavingFiles(pkFile string, vkFile string, pk groth16.ProvingKey, vk groth16.VerifyingKey) error {
	if err := writeAtomic(pkFile, pk.WriteTo); err != nil {
		return fmt.Errorf("write proving key %s: %w", pkFile, err)
	}
	if err := writeAtomic(vkFile, vk.WriteTo); err != nil {
		return fmt.Errorf("write verifying key %s: %w", vkFile, err)
	}
	return nil
}

// writeAtomic writes via writeTo (e.g. a gnark key's WriteTo) to a
// sibling ".tmp" file, then renames it into place — see SavingFiles' doc
// for why. The temp file is removed on any failure so a botched attempt
// never lingers next to the real path.
func writeAtomic(path string, writeTo func(io.Writer) (int64, error)) (err error) {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			f.Close()
		}
		if err != nil {
			os.Remove(tmp)
		}
	}()
	if _, err = writeTo(f); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	closed = true
	return os.Rename(tmp, path)
}
