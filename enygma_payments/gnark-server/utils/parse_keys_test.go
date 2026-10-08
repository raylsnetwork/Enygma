package utils

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

// ParseBigInt must accept exactly the canonical field elements: a value the
// witness would silently reduce mod Fr must be rejected instead, so the proof
// and the returned public signals stay the same number.
func TestParseBigIntRange(t *testing.T) {
	fr := ecc.BN254.ScalarField()
	frMinus1 := new(big.Int).Sub(fr, big.NewInt(1)).String()

	for _, s := range []string{"0", "1", "007", frMinus1} {
		if _, err := ParseBigInt(s); err != nil {
			t.Errorf("ParseBigInt(%q) rejected a canonical value: %v", s, err)
		}
	}
	for _, s := range []string{"-1", fr.String(), new(big.Int).Add(fr, big.NewInt(5)).String(), "0x10", "abc", ""} {
		if _, err := ParseBigInt(s); err == nil {
			t.Errorf("ParseBigInt(%q) accepted a value outside [0, Fr) or non-decimal", s)
		}
	}
}

type keyPairCircuit struct {
	A frontend.Variable `gnark:",public"`
	B frontend.Variable
}

func (c *keyPairCircuit) Define(api frontend.API) error {
	api.AssertIsEqual(api.Mul(c.B, c.B), c.A)
	return nil
}

func TestCheckKeyPair(t *testing.T) {
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &keyPairCircuit{})
	if err != nil {
		t.Fatal(err)
	}
	pk1, vk1, err := groth16.Setup(ccs)
	if err != nil {
		t.Fatal(err)
	}
	pk2, vk2, err := groth16.Setup(ccs) // independent setup of the same circuit
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckKeyPair(pk1, vk1); err != nil {
		t.Errorf("matching pair rejected: %v", err)
	}
	if err := CheckKeyPair(pk2, vk2); err != nil {
		t.Errorf("matching pair rejected: %v", err)
	}
	if err := CheckKeyPair(pk1, vk2); err == nil {
		t.Error("pk and vk from different setups were accepted as a pair")
	}
}
