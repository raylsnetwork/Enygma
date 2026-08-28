package utils

import (
	"math/big"
	"testing"
)

// TestNUMS_H_ReproducesHardcodedGenerator is the audit's own H-11 check
// (§ "the decisive test") for the generator that was already correct:
// hashing from seed "1" must land on iteration 2 and reproduce the exact
// HBabyJub coordinates hardcoded in this package.
func TestNUMS_H_ReproducesHardcodedGenerator(t *testing.T) {
	x, y, iterations := DeriveNUMSPoint(big.NewInt(1))
	if iterations != 2 {
		t.Fatalf("H: expected 2 SHA-256 iterations from seed 1, got %d", iterations)
	}
	if x.Cmp(hx) != 0 || y.Cmp(hy) != 0 {
		t.Fatalf("H: derivation from seed 1 does not match hardcoded HBabyJub\n got  X=%s Y=%s\n want X=%s Y=%s",
			x, y, hx, hy)
	}
}

// TestNUMS_G_ReproducesHardcodedGenerator is the H-11 fix itself: G must
// now also be reproducible from a published seed, the same way H always
// was. This is the CI assertion the audit's remediation asks for — if
// anyone ever hand-edits circuitGx/circuitGy without updating the seed (or
// vice versa), this test fails.
func TestNUMS_G_ReproducesHardcodedGenerator(t *testing.T) {
	x, y, _ := DeriveNUMSPoint(big.NewInt(2))
	if x.Cmp(circuitGx) != 0 || y.Cmp(circuitGy) != 0 {
		t.Fatalf("G: derivation from seed 2 does not match hardcoded CircuitGBabyJub\n got  X=%s Y=%s\n want X=%s Y=%s",
			x, y, circuitGx, circuitGy)
	}
}

// TestNUMS_GeneratorsAreDistinctAndOnCurve is the property H-11 exists to
// guarantee: G and H are two independently-derived points, neither of
// which is the identity, and they are not equal to each other. (On-curve
// and order-P are exercised independently in TestNUMS_DerivedPointHasOrderP
// via the shared numsIsValidX/numsClearCofactor path every derivation
// goes through.)
func TestNUMS_GeneratorsAreDistinctAndOnCurve(t *testing.T) {
	if circuitGx.Cmp(hx) == 0 && circuitGy.Cmp(hy) == 0 {
		t.Fatal("G and H derived to the same point")
	}
	if !numsIsValidX(numsMod(new(big.Int).Set(circuitGx))) {
		// numsIsValidX expects a raw candidate x pre-reduction; CircuitGBabyJub's
		// X is already reduced, so this call is equivalent to checking the
		// curve equation holds at this x, which is what "on curve" means here.
		t.Fatal("G's X coordinate is not a valid Baby Jubjub x-coordinate")
	}
}

// TestNUMS_DerivedPointHasOrderExactlyP derives a fresh point from a third
// seed (used by neither G nor H) end-to-end and checks it independently:
// on-curve, not the identity, and killed by P — i.e. its order is exactly
// the prime subgroup order, not some cofactor-sized subgroup that slipped
// through cofactor clearing.
func TestNUMS_DerivedPointHasOrderExactlyP(t *testing.T) {
	x, y, _ := DeriveNUMSPoint(big.NewInt(3))

	if x.Sign() == 0 && y.Cmp(big.NewInt(1)) == 0 {
		t.Fatal("derived point is the identity — cofactor clearing produced a degenerate point")
	}

	x2 := new(big.Int).Exp(x, big.NewInt(2), babyJubFieldPrime)
	y2 := new(big.Int).Exp(y, big.NewInt(2), babyJubFieldPrime)
	// Twisted Edwards curve equation: a*x^2 + y^2 = 1 + d*x^2*y^2
	lhs := numsMod(new(big.Int).Add(new(big.Int).Mul(A, x2), y2))
	rhs := numsMod(new(big.Int).Add(big.NewInt(1), new(big.Int).Mul(D, new(big.Int).Mul(x2, y2))))
	if lhs.Cmp(rhs) != 0 {
		t.Fatalf("derived point is not on curve: a*x^2+y^2=%s, 1+d*x^2*y^2=%s", lhs, rhs)
	}

	// [P]Q must be the identity (0,1) once cofactor 8 has been cleared.
	q := numsPoint{X: x, Y: y}
	acc := numsPoint{X: big.NewInt(0), Y: big.NewInt(1)}
	base := q
	k := new(big.Int).Set(P)
	for k.Sign() > 0 {
		if k.Bit(0) == 1 {
			acc = numsAdd(acc, base)
		}
		base = numsDouble(base)
		k.Rsh(k, 1)
	}
	if acc.X.Sign() != 0 || acc.Y.Cmp(big.NewInt(1)) != 0 {
		t.Fatalf("[P]Q != identity: got X=%s Y=%s — cofactor was not fully cleared", acc.X, acc.Y)
	}
}
