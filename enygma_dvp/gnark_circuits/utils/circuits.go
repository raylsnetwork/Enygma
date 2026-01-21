package utils

import(
	"math/big"

	"github.com/consensys/gnark/std/algebra/native/twistededwards"	
	"github.com/consensys/gnark/frontend"
)

var(
	A = big.NewInt(168700) 
	D =  big.NewInt(168696)
)

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
