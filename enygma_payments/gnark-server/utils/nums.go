package utils

// Nothing-up-my-sleeve (NUMS) derivation for the Pedersen commitment
// generators G and H. Fixes H-11: H already had a documented, reproducible
// derivation (see cmd/setup/main.go and I-01); G did not — it was a
// non-standard point with no script, comment, test or document anywhere
// deriving it, so nobody could rule out that whoever chose it also knows
// e such that G = e·H, which would let them open any commitment to any
// value they like. This file is the single source of truth for the
// algorithm both cmd/setup/main.go (H, seed "1") and
// cmd/derive_generator/main.go (both G and H) use, and what
// nums_test.go checks every hardcoded copy of G and H against.
//
// Algorithm (unchanged from cmd/setup/main.go, extracted here so it has
// exactly one implementation instead of one committed script and zero
// others): iterate SHA-256 from a seed until the digest is a valid Baby
// Jubjub x-coordinate; recover y via Tonelli-Shanks; canonicalize to the
// even root; clear the cofactor with three point-doublings (x8, matching
// Baby Jubjub's cofactor).

import (
	"crypto/sha256"
	"math/big"
)

// babyJubFieldPrime is the BN254 scalar field, the base field Baby Jubjub
// point coordinates live in. Deliberately not reusing the package-level P
// (BabyJubYFromX etc. need the field the curve equation is defined over,
// not the ~251-bit subgroup order P is elsewhere in this package — mixing
// the two up here would silently derive nonsense points).
var babyJubFieldPrime, _ = new(big.Int).SetString(
	"21888242871839275222246405745257275088548364400416034343698204186575808495617", 10)

type numsPoint struct{ X, Y *big.Int }

func numsAdd(p1, p2 numsPoint) numsPoint {
	fp := babyJubFieldPrime
	x1x2 := new(big.Int).Mod(new(big.Int).Mul(p1.X, p2.X), fp)
	y1y2 := new(big.Int).Mod(new(big.Int).Mul(p1.Y, p2.Y), fp)
	t := new(big.Int).Mod(new(big.Int).Mul(D, new(big.Int).Mul(x1x2, y1y2)), fp)

	xnum := new(big.Int).Mod(new(big.Int).Add(new(big.Int).Mul(p1.X, p2.Y), new(big.Int).Mul(p1.Y, p2.X)), fp)
	xden := new(big.Int).Mod(new(big.Int).Add(big.NewInt(1), t), fp)

	ayx := new(big.Int).Mod(new(big.Int).Mul(A, x1x2), fp)
	ynum := numsMod(new(big.Int).Sub(y1y2, ayx))
	yden := numsMod(new(big.Int).Sub(big.NewInt(1), t))

	x3 := new(big.Int).Mod(new(big.Int).Mul(xnum, new(big.Int).ModInverse(xden, fp)), fp)
	y3 := new(big.Int).Mod(new(big.Int).Mul(ynum, new(big.Int).ModInverse(yden, fp)), fp)
	return numsPoint{X: x3, Y: y3}
}

func numsDouble(p numsPoint) numsPoint { return numsAdd(p, p) }

// numsClearCofactor computes [8]P — Baby Jubjub's cofactor is 8.
func numsClearCofactor(p numsPoint) numsPoint {
	q := numsDouble(p)
	q = numsDouble(q)
	q = numsDouble(q)
	return q
}

func numsMod(x *big.Int) *big.Int {
	fp := babyJubFieldPrime
	x.Mod(x, fp)
	if x.Sign() < 0 {
		x.Add(x, fp)
	}
	return x
}

func numsLegendreSymbol(x *big.Int) *big.Int {
	fp := babyJubFieldPrime
	e := new(big.Int).Rsh(new(big.Int).Sub(fp, big.NewInt(1)), 1)
	return new(big.Int).Exp(numsMod(new(big.Int).Set(x)), e, fp)
}

// numsTonelliShanks computes sqrt(n) mod babyJubFieldPrime.
func numsTonelliShanks(n *big.Int) (*big.Int, bool) {
	fp := babyJubFieldPrime
	one, two := big.NewInt(1), big.NewInt(2)
	n = numsMod(new(big.Int).Set(n))
	if n.Sign() == 0 {
		return big.NewInt(0), true
	}
	if numsLegendreSymbol(n).Cmp(one) != 0 {
		return nil, false
	}

	q := new(big.Int).Sub(fp, one)
	s := 0
	for q.Bit(0) == 0 {
		q.Rsh(q, 1)
		s++
	}

	z := big.NewInt(2)
	nMinus1 := new(big.Int).Sub(fp, one)
	for numsLegendreSymbol(z).Cmp(nMinus1) != 0 {
		z.Add(z, one)
	}

	c := new(big.Int).Exp(z, q, fp)
	t := new(big.Int).Exp(n, q, fp)
	r := new(big.Int).Exp(n, new(big.Int).Rsh(new(big.Int).Add(q, one), 1), fp)

	for {
		if t.Cmp(one) == 0 {
			return r, true
		}
		i := 1
		t2i := new(big.Int).Exp(t, two, fp)
		for i < s {
			if t2i.Cmp(one) == 0 {
				break
			}
			t2i.Exp(t2i, two, fp)
			i++
		}
		e := s - i - 1
		b := new(big.Int).Set(c)
		for j := 0; j < e; j++ {
			b.Exp(b, two, fp)
		}
		r.Mul(r, b).Mod(r, fp)
		t.Mul(t, new(big.Int).Exp(b, two, fp)).Mod(t, fp)
		c.Exp(b, two, fp)
		s = i
	}
}

// numsIsValidX reports whether x is the x-coordinate of a point on Baby
// Jubjub (i.e. the curve equation's y^2 has a square root at this x).
func numsIsValidX(x *big.Int) bool {
	fp := babyJubFieldPrime
	x2 := new(big.Int).Exp(x, big.NewInt(2), fp)
	num := numsMod(new(big.Int).Sub(big.NewInt(1), new(big.Int).Mul(A, x2)))
	den := numsMod(new(big.Int).Sub(big.NewInt(1), new(big.Int).Mul(D, x2)))
	if den.Sign() == 0 {
		return false
	}
	y2 := new(big.Int).Mod(new(big.Int).Mul(num, new(big.Int).ModInverse(den, fp)), fp)
	return numsLegendreSymbol(y2).Cmp(big.NewInt(1)) == 0
}

// numsYFromX recovers the (canonical, even) y for a valid Baby Jubjub x.
func numsYFromX(x *big.Int) (*big.Int, bool) {
	fp := babyJubFieldPrime
	x2 := new(big.Int).Exp(x, big.NewInt(2), fp)
	num := numsMod(new(big.Int).Sub(big.NewInt(1), new(big.Int).Mul(A, x2)))
	den := numsMod(new(big.Int).Sub(big.NewInt(1), new(big.Int).Mul(D, x2)))
	if den.Sign() == 0 {
		return nil, false
	}
	y2 := new(big.Int).Mod(new(big.Int).Mul(num, new(big.Int).ModInverse(den, fp)), fp)
	y, ok := numsTonelliShanks(y2)
	if !ok {
		return nil, false
	}
	if y.Bit(0) == 1 {
		y.Sub(fp, y)
	}
	return y, true
}

func numsHash256(n *big.Int) *big.Int {
	h := sha256.Sum256(n.Bytes())
	return new(big.Int).SetBytes(h[:])
}

// DeriveNUMSPoint runs the project's published hash-to-curve construction
// from the given seed and returns the resulting generator (post
// cofactor-clearing), along with how many SHA-256 iterations it took to
// land on a valid x-coordinate — matching what cmd/setup/main.go already
// does for H (seed "1") and what the audit's own validation reproduced.
func DeriveNUMSPoint(seed *big.Int) (x, y *big.Int, iterations int) {
	cur := new(big.Int).Set(seed)
	for {
		iterations++
		h := numsHash256(cur)
		if numsIsValidX(h) {
			yc, ok := numsYFromX(h)
			if !ok {
				panic("valid x-coordinate produced no y — numsIsValidX/numsYFromX disagree")
			}
			q := numsClearCofactor(numsPoint{X: h, Y: yc})
			return q.X, q.Y, iterations
		}
		cur = h
	}
}
