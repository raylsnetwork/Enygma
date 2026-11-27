package enygma

import (
	"testing"

	"enygma_gnark_shared/circuitconfig"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	bngroth16 "github.com/consensys/gnark/backend/groth16/bn254"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

func TestEnygmaCircuitConfigHoldsNoWitnessVariables(t *testing.T) {
	if err := circuitconfig.Check(EnygmaCircuit{}); err != nil {
		t.Fatal(err)
	}
}

func TestDomainIdHasNonZeroVerificationKeyTerm(t *testing.T) {
	circuit := createCircuitTemplate(DefaultConfig)
	// Inspect the verification key even if the compiler detects an unused input.
	cs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit, frontend.IgnoreUnconstrainedInputs())
	if err != nil {
		t.Fatalf("compile the transfer circuit: %v", err)
	}
	_, vk, err := groth16.Setup(cs)
	if err != nil {
		t.Fatalf("generate a test verification key: %v", err)
	}

	domain, ok := StatementLayout().Field("DomainId")
	if !ok || domain.Size != 1 {
		t.Fatal("expected one DomainId public input")
	}
	// K[0] is the constant term. Public inputs start at K[1].
	if vk.(*bngroth16.VerifyingKey).G1.K[domain.Offset+1].IsInfinity() {
		t.Fatal("DomainId's verification key term is zero")
	}
}
