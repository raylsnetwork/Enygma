// Package valuebound holds the one bound every Enygma circuit puts on a note
// value, fee or bid, and the constraint that enforces it.
//
// The bound is a compile-time constant, part of every circuit's constraint
// system and so of its verifying key. It used to be a frontend.Variable in each
// circuit's config struct; gnark compiled that field as a secret witness, so the
// prover chose the bound and could set it to p-1, which disabled the check and
// let a 100-unit note be spent into a 10^30 note plus a wrapped change note.
package valuebound

import (
	"math/big"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/bits"
)

// Bits is the width every bounded value is decomposed to. 2^Bits is above
// Bound, and far enough below the BN254 scalar field (about 2^254) that a sum of
// any number of bounded values a circuit adds up (at most a handful of inputs,
// outputs and a fee) cannot wrap mod p.
const Bits = 120

// bound is 10^36, the exclusive upper bound on every value: the same value as
// MAX_FEE_AMOUNT in enygma_dvp's Erc20CoinVault.sol.
var bound = new(big.Int).Exp(big.NewInt(10), big.NewInt(36), nil)

// Bound returns 10^36, the exclusive upper bound Assert enforces.
func Bound() *big.Int { return new(big.Int).Set(bound) }

// Assert constrains 0 <= v < 10^36 as an integer.
//
// The first decomposition makes v a Bits-bit integer, so v < 2^Bits. The second
// decomposes 10^36 - 1 - v: for v <= 10^36 - 1 that is a Bits-bit integer, and
// for 10^36 <= v < 2^Bits it wraps to p - (v - 10^36 + 1), above 2^Bits, so no
// decomposition exists. Each takes Bits+1 constraints.
func Assert(api frontend.API, v frontend.Variable) {
	bits.ToBinary(api, v, bits.WithNbDigits(Bits))
	max := new(big.Int).Sub(bound, big.NewInt(1))
	bits.ToBinary(api, api.Sub(max, v), bits.WithNbDigits(Bits))
}
