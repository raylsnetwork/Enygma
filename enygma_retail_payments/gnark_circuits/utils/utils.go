package utils

import (
	"fmt"
	"math/big"
	"os"

	"github.com/consensys/gnark-crypto/ecc"
	fr_bn254 "github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/backend/witness"
)

func LoadProvingKey(curve ecc.ID, filename string) (groth16.ProvingKey, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	pk := groth16.NewProvingKey(curve)
	_, err = pk.ReadFrom(file)
	return pk, err
}

func LoadVerifyingKey(curve ecc.ID, filename string) (groth16.VerifyingKey, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	vk := groth16.NewVerifyingKey(curve)
	_, err = vk.ReadFrom(file)
	return vk, err
}

// PublicSignals returns the public part of witnessFull, in public-witness
// order, as the big ints a prove handler responds with. It is read off the
// same witness the proof is made and verified from, so the returned
// statement is by construction the one the proof verifies against; handlers
// must not rebuild it from request fields.
func PublicSignals(witnessFull witness.Witness) ([]*big.Int, error) {
	publicWitness, err := witnessFull.Public()
	if err != nil {
		return nil, fmt.Errorf("extract public witness: %w", err)
	}
	vec, ok := publicWitness.Vector().(fr_bn254.Vector)
	if !ok {
		return nil, fmt.Errorf("public witness is %T, want a BN254 vector", publicWitness.Vector())
	}
	signals := make([]*big.Int, len(vec))
	for i := range vec {
		signals[i] = vec[i].BigInt(new(big.Int))
	}
	return signals, nil
}
