package utils

import (
	
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/algebra/native/twistededwards"
	
)

var(
	G = twistededwards.Point{
		X: "16540640123574156134436876038791482806971768689494387082833631921987005038935",
		Y: "20819045374670962167435360035096875258406992893633759881276124905556507972311",
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