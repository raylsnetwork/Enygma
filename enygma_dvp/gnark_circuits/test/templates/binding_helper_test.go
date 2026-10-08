package test

import (
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

// assertPublicInputBound checks that a public input is bound by the proof: a
// proof made for valid must verify against valid's public inputs and must not
// verify against tampered's, which differ in that one input. A public input
// that takes part in no constraint would verify either way.
func assertPublicInputBound(t *testing.T, empty, valid, tampered frontend.Circuit) {
	t.Helper()
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, empty)
	if err != nil {
		t.Fatal(err)
	}
	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		t.Fatal(err)
	}
	full, err := frontend.NewWitness(valid, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatal(err)
	}
	proof, err := groth16.Prove(ccs, pk, full)
	if err != nil {
		t.Fatal(err)
	}
	pub := func(c frontend.Circuit) error {
		w, err := frontend.NewWitness(c, ecc.BN254.ScalarField(), frontend.PublicOnly())
		if err != nil {
			t.Fatal(err)
		}
		return groth16.Verify(proof, vk, w)
	}
	if err := pub(valid); err != nil {
		t.Fatalf("the proof does not verify against its own public inputs: %v", err)
	}
	if pub(tampered) == nil {
		t.Fatal("the proof verified with a rewritten public input: it is not bound")
	}
}
