package utils

import (

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/algebra/native/twistededwards"
	cmp "github.com/consensys/gnark/std/math/cmp"

)

var(
	// G: NUMS hash-to-curve derivation, seed "2" (H-11 fix — must match
	// utils/utils.go CircuitGBabyJub). Reproduce with:
	// go run ./cmd/derive_generator
	G = twistededwards.Point{
		X: "12337812418750581066638756637363471856433191340622504180842886595232027947307",
		Y: "15225366398330386329633463986700597127113326976080712967801565482915963669722",
	}

	H= twistededwards.Point{
		X:"10100005861917718053548237064487763771145251762383025193119768015180892676690",
		Y:"7512830269827713629724023825249861327768672768516116945507944076335453576011",
	}
)


func PointAdd(api frontend.API, p1, p2 twistededwards.Point) twistededwards.Point {
	x1y2 := api.Mul(p1.X, p2.Y)
	y1x2 := api.Mul(p1.Y, p2.X)
	x1x2 := api.Mul(p1.X, p2.X)
	y1y2 := api.Mul(p1.Y, p2.Y)

	denomX := api.Add(api.Mul(D, api.Mul(x1x2, y1y2)), frontend.Variable(1))
	denomY := api.Sub(frontend.Variable(1), api.Mul(D, api.Mul(x1x2, y1y2)))

	// Ensure denominators are non-zero.
	api.AssertIsDifferent(denomX, frontend.Variable(0))
	api.AssertIsDifferent(denomY, frontend.Variable(0))

	numerX := api.Add(x1y2, y1x2)
	numerY := api.Sub(y1y2, api.Mul(A, x1x2))

	invDenomX := api.Inverse(denomX)
	invDenomY := api.Inverse(denomY)

	x := api.Mul(numerX, invDenomX)
	y := api.Mul(numerY, invDenomY)

	return twistededwards.Point{X: x, Y: y}
}

// pointDouble doubles a point by simply adding it to itself.
func PointDouble(api frontend.API, p twistededwards.Point) twistededwards.Point {
	return PointAdd(api, p, p)
}

// pointSelect conditionally selects one of two points based on a boolean condition.
func PointSelect(api frontend.API, cond frontend.Variable, p0, p1 twistededwards.Point) twistededwards.Point {
	return twistededwards.Point{
		X: api.Select(cond, p1.X, p0.X),
		Y: api.Select(cond, p1.Y, p0.Y),
	}
}

// scalarMul multiplies a point by a scalar using the double and add algorithm.
// It uses api.ToBinary to decompose the scalar (assumed 256 bits, little-endian).
func ScalarMul(api frontend.API, p twistededwards.Point, scalar frontend.Variable) twistededwards.Point {
	// Convert scalar to its 256-bit binary representation.
	bits := api.ToBinary(scalar, 256)
	// Initialize result to the identity point: (0,1)
	result := twistededwards.Point{
		X: frontend.Variable(0),
		Y: frontend.Variable(1),
	}
	// Use a temporary variable for the point to add.
	temp := p
	for i := 0; i < 256; i++ {
		// If the i-th bit is 1, then add temp to result.
		// Here we compute add := pointAdd(result, temp) and conditionally update result.
		add := PointAdd(api, result, temp)
		result = PointSelect(api, bits[i], result, add)
		// Double the temp point for the next bit.
		temp = PointDouble(api, temp)
	}
	return result
}



// ReduceModP performs a fully-constrained reduction of value modulo the Baby
// Jubjub prime subgroup order P. Fixes C-01: the raw two-constraint hint
// gadget this replaces (q·P + r == value, r < P, copy-pasted at all 28 call
// sites across the four circuits) left the quotient q completely unbounded.
// Because P is invertible in the BN254 scalar field Fr, a satisfying q =
// (value - r)·P⁻¹ exists for ANY chosen r in [0, P), so the "reduction"
// proved nothing — a prover could make ModHint return whatever r it wanted
// (e.g. to mint from a zero balance) simply by overriding the hint at prove
// time; the circuit itself never rejected it. Fr is ~254 bits and P is ~251
// bits, so floor(Fr/P) == 7: binding q to 3 bits closes this off.
//
// That bound is api.ToBinary(q, 3), not api.AssertIsLessOrEqual(q, 7):
// gnark's AssertIsLessOrEqual, even for a constant bound, decomposes the
// compared value into the FULL field width (~254 bits, api.go's
// ToBinary(..., FieldBitLen())) before comparing — measured at roughly
// 1.7k-2.1k extra constraints per call site, 28 sites. ToBinary(q, 3)
// decomposes into exactly 3 bits and gets the identical soundness
// property (no valid witness exists for q > 7) for a handful of
// constraints — this is what "three-bit decomposition, essentially free"
// actually requires.
func ReduceModP(api frontend.API, value frontend.Variable) frontend.Variable {
	out, _ := api.NewHint(ModHint, 2, value)
	r, q := out[0], out[1]
	api.AssertIsEqual(api.Add(api.Mul(q, P), r), value)
	api.AssertIsEqual(cmp.IsLess(api, r, P), 1)
	api.ToBinary(q, 3)
	return r
}

func AssertPointsIsOnCurve(api frontend.API, X, Y frontend.Variable) {
	// Compute X² and Y²
	x2 := api.Mul(X, X)
	y2 := api.Mul(Y, Y)

	// Compute X²Y²
	x2y2 := api.Mul(x2, y2)

	// Compute left-hand side (A*X² + Y²)
	lhs := api.Add(api.Mul(A, x2), y2)

	// Compute right-hand side (1 + D*X²Y²)
	rhs := api.Add(1, api.Mul(D, x2y2))

	// Assert equality of both sides
	api.AssertIsEqual(lhs, rhs)
}


func PedersenCommitment(api frontend.API,X,Y frontend.Variable)twistededwards.Point{
	

	vG := ScalarMul(api, G, X)             
	rH := ScalarMul(api, H, Y) 

	commitOutput := PointAdd(api, vG, rH) 

	return commitOutput

}