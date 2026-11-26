// Package witnessinput validates request fields before gnark builds a witness.
package witnessinput

import (
	"errors"
	"math/big"
	"strings"

	"github.com/consensys/gnark-crypto/ecc"
)

// Parser parses BN254 scalar field elements and records the first
// error. Check Err before passing any result to frontend.NewWitness.
type Parser struct {
	err error
}

// Parse returns the field element, or zero if an input was invalid.
func (p *Parser) Parse(value string) *big.Int {
	if p.err != nil {
		return new(big.Int)
	}
	// Match payments' decimal parsing while preserving prefixed hex witnesses.
	base := 10
	if strings.HasPrefix(value, "0x") || strings.HasPrefix(value, "0X") {
		base = 0
	}
	n, ok := new(big.Int).SetString(value, base)
	if !ok || n.Sign() < 0 || n.Cmp(ecc.BN254.ScalarField()) >= 0 {
		p.err = errors.New("invalid witness number: expected a BN254 scalar field element")
		return new(big.Int)
	}
	return n
}

// Err returns the first parse error.
func (p *Parser) Err() error { return p.err }
