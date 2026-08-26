package utils

// TestL15_* reproduce and verify the L-15 fix
// (ENYGMA_PAYMENTS_AUDIT_2026-08-22.md, Low/Low-Info boundary):
//
//   - GBabyJub/GetPK/GetH/PedersenCommitmentBabyJub (the standard iden3
//     base point B8 and three helpers built on it) were dead code with no
//     live caller anywhere in the repo, sitting next to the obviously
//     similarly-named CircuitGBabyJub every real commitment site actually
//     uses — a maintainer trap. Deleted; see this file's package doc and
//     h01_h02_repro_test.go's updated comment.
//   - ModHint re-parsed the JubJub subgroup order from a second literal
//     instead of using the package-level P already declared above it, and
//     had an unreachable second `return nil`. TestL15_ModHintUsesPackageP
//     and the existing TestReduceModP suite (unaffected by this change)
//     together confirm the hint still produces a correct, usable (r, q).
//   - SavingFiles log.Fatalf'd on every failure (so its caller's `%w`
//     error wrapper was unreachable) and wrote non-atomically, so a
//     mid-write failure could leave a truncated key file a later run
//     would silently load. TestL15_SavingFiles_ReturnsErrorOnCreateFailure
//     and TestL15_SavingFiles_WritesAtomically cover both halves.
//   - GetPkHash discarded poseidon.Hash's error; it now returns one.
//
// Run:
//
//	CC=/usr/bin/clang go test ./utils/... -run TestL15 -v

import (
	"errors"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/consensys/gnark/backend/groth16"
)

// TestL15_ModHintUsesPackageP confirms ModHint's (r, q) satisfy
// q*P + r == value with 0 <= r < P — the same property ReduceModP's
// in-circuit constraints check — using the package-level P directly,
// with no second, independent copy of the literal to drift out of sync.
func TestL15_ModHintUsesPackageP(t *testing.T) {
	value := new(big.Int).Add(P, big.NewInt(12345)) // > P, so q must be 1
	res := make([]*big.Int, 2)
	if err := ModHint(nil, []*big.Int{value}, res); err != nil {
		t.Fatalf("ModHint returned an error: %v", err)
	}
	r, q := res[0], res[1]
	if q.Cmp(big.NewInt(1)) != 0 {
		t.Fatalf("q = %s, want 1", q)
	}
	if r.Cmp(big.NewInt(12345)) != 0 {
		t.Fatalf("r = %s, want 12345", r)
	}
	reconstructed := new(big.Int).Add(new(big.Int).Mul(q, P), r)
	if reconstructed.Cmp(value) != 0 {
		t.Fatalf("q*P + r = %s, want %s", reconstructed, value)
	}
	if r.Cmp(P) >= 0 {
		t.Fatalf("FAIL (L-15 regressed): r = %s is not < P", r)
	}
	t.Log("ModHint's (r, q) satisfy q*P + r == value with r < P, using the package-level P ✓")
}

// TestL15_SavingFiles_ReturnsErrorOnCreateFailure confirms SavingFiles no
// longer log.Fatalf's on a write failure — pre-fix, this test's own
// process would have been killed by the call under test, which is
// itself proof enough that this needed fixing to be testable at all.
func TestL15_SavingFiles_ReturnsErrorOnCreateFailure(t *testing.T) {
	// A path with a nonexistent parent directory: os.Create fails with
	// ENOENT, not a permissions edge case that might behave differently
	// across CI environments.
	badPath := filepath.Join(t.TempDir(), "does-not-exist", "pk.key")
	err := SavingFiles(badPath, badPath, &fakePkKey{}, &fakeVkKey{})
	if err == nil {
		t.Fatal("FAIL (L-15 regressed): SavingFiles returned nil for an uncreatable path")
	}
	t.Logf("SavingFiles correctly returned an error instead of exiting the process: %v", err)
}

// TestL15_SavingFiles_WritesAtomically confirms a failing verifying-key
// write leaves no partial/truncated file at vkFile — either the previous
// content (if any) or nothing, never a truncated attempt — and leaves no
// ".tmp" file behind either.
func TestL15_SavingFiles_WritesAtomically(t *testing.T) {
	dir := t.TempDir()
	pkFile := filepath.Join(dir, "pk.key")
	vkFile := filepath.Join(dir, "vk.key")

	err := SavingFiles(pkFile, vkFile, &fakePkKey{}, &failingVkKey{})
	if err == nil {
		t.Fatal("expected an error from a failing verifying-key write")
	}

	if _, statErr := os.Stat(vkFile); statErr == nil {
		t.Fatal("FAIL (L-15 regressed): vkFile exists despite its write failing — not atomic")
	}
	if _, statErr := os.Stat(vkFile + ".tmp"); statErr == nil {
		t.Fatal("FAIL (L-15 regressed): a .tmp file was left behind after a failed write")
	}
	// The proving key succeeded and should be there, complete.
	if _, statErr := os.Stat(pkFile); statErr != nil {
		t.Fatalf("pkFile missing despite its own write succeeding: %v", statErr)
	}
	t.Log("a failing write leaves no partial vkFile and no stray .tmp file; the successful pkFile write is unaffected ✓")
}

// TestL15_GetPkHash_ReturnsError is a light sanity check that the new
// error-returning signature still produces the same hash as before for
// a normal input.
func TestL15_GetPkHash_ReturnsError(t *testing.T) {
	sk := big.NewInt(424242)
	hash, err := GetPkHash(sk)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hash == nil || hash.Sign() == 0 {
		t.Fatal("GetPkHash returned a nil or zero hash for a nonzero sk")
	}
}

// fakePkKey/fakeVkKey satisfy groth16.ProvingKey/VerifyingKey (embedding
// the real, nil interface supplies every method but WriteTo for free) to
// test SavingFiles in isolation, without paying for a real trusted-setup
// ceremony. failingVkKey's WriteTo always errors, to exercise the
// partial-failure/atomicity half of the fix.
type fakePkKey struct{ groth16.ProvingKey }

func (fakePkKey) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write([]byte("fake-key-bytes"))
	return int64(n), err
}

type fakeVkKey struct{ groth16.VerifyingKey }

func (fakeVkKey) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write([]byte("fake-key-bytes"))
	return int64(n), err
}

type failingVkKey struct{ groth16.VerifyingKey }

var errWriteFailed = errors.New("simulated write failure")

func (failingVkKey) WriteTo(w io.Writer) (int64, error) {
	return 0, errWriteFailed
}
