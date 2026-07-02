package main

import (

	"fmt"
	"math/big"
	
	utils "enygma-server/utils"
	"github.com/consensys/gnark/frontend"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend/cs/r1cs"

	"crypto/sha256"


	
)
var (
	a, _ = new(big.Int).SetString("168700", 10)
	d, _ = new(big.Int).SetString("168696", 10)
	p, _ = new(big.Int).SetString("21888242871839275222246405745257275088548364400416034343698204186575808495617", 10)
	one  = big.NewInt(1)
	two  = big.NewInt(2)
)

type Point struct{ X, Y *big.Int }

type HSetupCircuit struct {
	// X and Y can be marked as secret (or public) depending on your use case
	X  frontend.Variable 
	Y  frontend.Variable 
}
func add(P, Q Point) Point {
// t = d*x1*x2*y1*y2
	x1x2 := new(big.Int).Mul(P.X, Q.X)
	x1x2.Mod(x1x2, p)
	y1y2 := new(big.Int).Mul(P.Y, Q.Y)
	y1y2.Mod(y1y2, p)
	t := new(big.Int).Mul(d, new(big.Int).Mul(x1x2, y1y2))
	t.Mod(t, p)

	// x numerator: x1*y2 + y1*x2
	xnum := new(big.Int).Add(new(big.Int).Mul(P.X, Q.Y), new(big.Int).Mul(P.Y, Q.X))
	xnum.Mod(xnum, p)

	// x denom: 1 + t
	xden := new(big.Int).Add(one, t)
	xden.Mod(xden, p)

	// y numerator: y1*y2 - a*x1*x2
	ayx := new(big.Int).Mul(a, x1x2)
	ayx.Mod(ayx, p)
	ynum := new(big.Int).Sub(y1y2, ayx)
	mod(ynum)

	// y denom: 1 - t
	yden := new(big.Int).Sub(one, t)
	mod(yden)

	// x3 = xnum * inv(xden)
	ixd := modInverse(xden)
	iyd := modInverse(yden)
	x3 := new(big.Int).Mul(xnum, ixd)
	x3.Mod(x3, p)
	y3 := new(big.Int).Mul(ynum, iyd)
	y3.Mod(y3, p)

	return Point{X: x3, Y: y3}
}

func dbl(P Point) Point { return add(P, P) }
func ClearCofactor(P Point) Point {
	Q := dbl(P) // [2]P
	Q = dbl(Q)  // [4]P
	Q = dbl(Q)  // [8]P
	return Q
}


func (circuit *HSetupCircuit) Define(api frontend.API) error {


	utils.AssertPointsIsOnCurve(api,circuit.X,circuit.Y)

	return nil
}

func isValidBabyJubX(x *big.Int) bool {
	x2 := new(big.Int).Exp(x, big.NewInt(2), p)

	num := new(big.Int).Sub(big.NewInt(1), new(big.Int).Mul(a, x2))
	num.Mod(num, p)
	den := new(big.Int).Sub(big.NewInt(1), new(big.Int).Mul(d, x2))
	den.Mod(den, p)

	// Check denominator ≠ 0
	if den.Sign() == 0 {
		return false
	}

	invDen := new(big.Int).ModInverse(den, p)
	y2 := new(big.Int).Mul(num, invDen)
	y2.Mod(y2, p)

	// Check if y2 is quadratic residue (Legendre symbol == 1)
	ls := new(big.Int).Exp(y2, new(big.Int).Rsh(new(big.Int).Sub(p, big.NewInt(1)), 1), p)
	return ls.Cmp(big.NewInt(1)) == 0
}


func hash256(number *big.Int)*big.Int{

		buf := number.Bytes()

	// Compute SHA256 hash
	hash := sha256.Sum256(buf)

	result := new(big.Int)
	result.SetBytes(hash[:])
	return result

}

// mod wraps x mod p to [0, p-1]
func mod(x *big.Int) *big.Int {
	x.Mod(x, p)
	if x.Sign() < 0 {
		x.Add(x, p)
	}
	return x
}

func legendreSymbol(x *big.Int) *big.Int {
	e := new(big.Int).Sub(p, one)
	e.Rsh(e, 1) // (p-1)/2
	return new(big.Int).Exp(mod(new(big.Int).Set(x)), e, p)
}

// modInverse computes modular inverse of x mod p; returns nil if not invertible.
func modInverse(x *big.Int) *big.Int {
	return new(big.Int).ModInverse(mod(new(big.Int).Set(x)), p)
}

// tonelliShanks computes sqrt(n) mod p (p is an odd prime, here p ≡ 1 (mod 4)).
// Returns (root, true) if exists, otherwise (nil, false). If exists, root^2 ≡ n (mod p).
func tonelliShanks(n *big.Int) (*big.Int, bool) {
	n = mod(new(big.Int).Set(n))
	if n.Sign() == 0 {
		return big.NewInt(0), true
	}
	// Check quadratic residue by Legendre symbol
	ls := legendreSymbol(n)
	if ls.Cmp(one) != 0 {
		return nil, false // non-residue
	}

	// Factor p-1 = q * 2^s with q odd
	q := new(big.Int).Sub(p, one)
	s := 0
	for q.Bit(0) == 0 {
		q.Rsh(q, 1)
		s++
	}

	// Find a quadratic non-residue z
	var z = big.NewInt(2)
	for {
		if legendreSymbol(z).Cmp(new(big.Int).Sub(p, one)) == 0 {
			break
		}
		z.Add(z, one)
	}

	// Initialization
	c := new(big.Int).Exp(z, q, p)
	t := new(big.Int).Exp(n, q, p)
	r := new(big.Int).Exp(n, new(big.Int).Rsh(new(big.Int).Add(q, one), 1), p)

	for {
		if t.Cmp(one) == 0 {
			return r, true
		}
		// Find least i (0 < i < s) with t^(2^i) ≡ 1
		i := 1
		t2i := new(big.Int).Exp(t, two, p) // t^(2)
		for i < s {
			if t2i.Cmp(one) == 0 {
				break
			}
			t2i.Exp(t2i, two, p) // square again
			i++
		}
		// b = c^(2^(s-i-1))
		e := s - i - 1
		b := new(big.Int).Set(c)
		for j := 0; j < e; j++ {
			b.Exp(b, two, p)
		}
		// Update r, t, c, s
		r.Mul(r, b).Mod(r, p)
		t.Mul(t, new(big.Int).Exp(b, two, p)).Mod(t, p)
		c.Exp(b, two, p)
		s = i
	}
}


func BabyJubYFromX(xIn *big.Int) (*big.Int, bool) {
	x := mod(new(big.Int).Set(xIn))
	// x2 = x^2
	x2 := new(big.Int).Exp(x, two, p)

	// num = 1 - a*x^2
	ax2 := new(big.Int).Mul(a, x2)
	num := new(big.Int).Sub(one, ax2)
	mod(num)

	// den = 1 - d*x^2
	dx2 := new(big.Int).Mul(d, x2)
	den := new(big.Int).Sub(one, dx2)
	mod(den)

	// If den == 0 -> no solution on BabyJubJub
	if den.Sign() == 0 {
		return nil, false
	}

	// y2 = num * inv(den) mod p
	invDen := modInverse(den)
	if invDen == nil {
		return nil, false
	}
	y2 := new(big.Int).Mul(num, invDen)
	y2.Mod(y2, p)

	// sqrt via Tonelli–Shanks
	y, ok := tonelliShanks(y2)
	if !ok {
		return nil, false
	}

	// Canonicalize: choose even y
	if y.Bit(0) == 1 {
		y.Sub(p, y)
	}
	return y, true
}

func main(){

	found:= false

	// Random seed 
	seed,_:= new(big.Int).SetString("1", 10)
	var HashNumber *big.Int
	for !found{
		// Hash of Seed
		Hash:= hash256(seed)
		// Check if Hash is a valid X coordinate in Baby Jubjub curve
		isValid := isValidBabyJubX(Hash)
		
		if isValid==true{
			found =true
			HashNumber = Hash
		}
		seed =Hash
	}
	//From X coordinate generate corresponding y coordinate
	y,_:=BabyJubYFromX(HashNumber)

	P := Point{X: HashNumber, Y: y}
	

	// Mod subgroup order r
	Q := ClearCofactor(P)
	var circuit HSetupCircuit


	//Using Gnark Circuit to double check if point is indeed a point in the Baby Jub Jub point
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	fmt.Println(ecc.BN254.ScalarField())
	if err != nil {
		panic(err)
	}

	// Generate proving and verifying keys
	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		panic(err)
	}


	witness := HSetupCircuit{

		X:  frontend.Variable(Q.X.String()),
		Y: frontend.Variable(Q.Y.String()),
	}
	witnessFull, err := frontend.NewWitness(&witness, ecc.BN254.ScalarField())
	proof, err := groth16.Prove(ccs, pk, witnessFull)

	witnessPublic, err := frontend.NewWitness(&witness, ecc.BN254.ScalarField(), frontend.PublicOnly())


	err = groth16.Verify(proof, vk, witnessPublic)
	if err != nil {
		panic(err)
	}

	fmt.Println("Q.X", Q.X, "Q.Y", Q.Y)

	println("Proof verified successfully!")

	
}