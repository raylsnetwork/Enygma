// Regression tests for the prover-chosen value bound. DvPInitiator,
// DvPDestination and the Payment-family circuits used to range-check values
// against Config.TmRange, a frontend.Variable that gnark compiled as a secret
// witness, so the prover picked the bound. It is now the fixed valuebound
// constant.
package test

import (
	"math/big"
	"testing"

	"enygma_gnark_shared/circuitconfig"
	"enygma_gnark_shared/valuebound"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/test"
	"github.com/iden3/go-iden3-crypto/poseidon"

	"enygma_dvp/gnark_circuits/templates"
)

// TestDvpInitiatorCircuit_ValueBobAboveBound_Fails: Alice cannot ask to be
// paid an amount at or above 10^36 (here p - 1, which the old witness bound let
// through), with COMMIT_A and StMessage recomputed to match.
func TestDvpInitiatorCircuit_ValueBobAboveBound_Fails(t *testing.T) {
	for name, v := range map[string]*big.Int{
		"bound": valuebound.Bound(),
		"p - 1": new(big.Int).Sub(ecc.BN254.ScalarField(), big.NewInt(1)),
	} {
		t.Run(name, func(t *testing.T) {
			w := buildValidDvpInitiatorWitness(t)
			w.valueBob = v
			commitA, err := poseidon.Hash([]*big.Int{w.pkAlice, w.saltA, w.valueBob, w.tokenIdBob})
			if err != nil {
				t.Fatal(err)
			}
			w.commitA = commitA
			wc := w.circuit()
			wc.Config = dvpInitiatorTestConfig()
			if err := test.IsSolved(emptyDvpInitiatorCircuit(), wc, ecc.BN254.ScalarField()); err == nil {
				t.Fatalf("valueBob %s solved", v)
			}
		})
	}
}

// TestCircuitConfigs_HoldNoVariables fails if any circuit this module
// compiles keeps a frontend.Variable (or any other non-constant) in its
// config, which gnark would compile as a prover-chosen witness.
func TestCircuitConfigs_HoldNoVariables(t *testing.T) {
	for name, c := range map[string]any{
		"DvPInitiator":            templates.DvPInitiatorCircuit{},
		"DvPDestination":          templates.DvPDestinationCircuit{},
		"Payment":                 templates.PaymentCircuit{},
		"PaymentFee":              templates.PaymentFeeCircuit{},
		"PaymentRelayerFeePublic": templates.PaymentRelayerFeePublicCircuit{},
		"UsdrFee":                 templates.UsdrFeeCircuit{},
		"PrivateMint":             templates.PrivateMintCircuit{},
	} {
		if err := circuitconfig.Check(c); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}
