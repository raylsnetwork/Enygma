package valuebound

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/test"
)

type boundCircuit struct {
	V frontend.Variable
}

func (c *boundCircuit) Define(api frontend.API) error {
	Assert(api, c.V)
	return nil
}

// TestAssert accepts 0 and 10^36-1 and rejects 10^36, 2^Bits, a value just
// above 2^Bits, and values near p that a sum could wrap through.
func TestAssert(t *testing.T) {
	p := ecc.BN254.ScalarField()
	one := big.NewInt(1)
	twoBits := new(big.Int).Lsh(one, Bits)
	cases := []struct {
		name string
		v    *big.Int
		ok   bool
	}{
		{"zero", big.NewInt(0), true},
		{"one", one, true},
		{"bound - 1", new(big.Int).Sub(Bound(), one), true},
		{"bound", Bound(), false},
		{"2^Bits - 1", new(big.Int).Sub(twoBits, one), false},
		{"2^Bits", twoBits, false},
		{"p - 1", new(big.Int).Sub(p, one), false},
		{"100 - 10^30 mod p", new(big.Int).Mod(new(big.Int).Sub(big.NewInt(100), new(big.Int).Exp(big.NewInt(10), big.NewInt(30), nil)), p), false},
		{"p - bound", new(big.Int).Sub(p, Bound()), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := test.IsSolved(&boundCircuit{}, &boundCircuit{V: c.v}, p)
			if c.ok && err != nil {
				t.Fatalf("%s rejected: %v", c.v, err)
			}
			if !c.ok && err == nil {
				t.Fatalf("%s accepted", c.v)
			}
		})
	}
}

// TestBoundIsBelowWidth keeps 10^36 within the Bits-bit decomposition, so an
// honest value always has one.
func TestBoundIsBelowWidth(t *testing.T) {
	if Bound().BitLen() > Bits {
		t.Fatalf("bound has %d bits, more than Bits = %d", Bound().BitLen(), Bits)
	}
}
