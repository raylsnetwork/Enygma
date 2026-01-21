package test

import (
	"testing"
	"fmt"
	"log"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
    "github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark-crypto/ecc"
	"gnark_server/templates"
	"gnark_server/primitives"
)

func TestAuctionBid(t *testing.T) {
	var circuit templates.LegitBrokerCircuit 

	solver.RegisterHint(primitives.ModHint)
	solver.RegisterHint(primitives.ERC155UniqueIdNative)
	solver.RegisterHint(primitives.PoseidonNative)

	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		panic(err)
	}

	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		panic(err)
	}

	var witness templates.LegitBrokerCircuit

	witness.StBeacon = frontend.Variable("0")
	witness.StBlindedPublicKey = frontend.Variable("8969476311670677319797407799556038378097907203341127544058653959227850785471")
	witness.WtPrivatekey = frontend.Variable("17656537491812230106898016272069026548238004574584156795070466176245410138683")
	

	witnessFull, err := frontend.NewWitness(&witness, ecc.BN254.ScalarField())
		if err != nil {
			log.Fatal(err)
		}
	proof, err := groth16.Prove(ccs, pk, witnessFull)

	witnessPublic, err := frontend.NewWitness(&witness, ecc.BN254.ScalarField(), frontend.PublicOnly())
	if err != nil {
		log.Fatal(err)
	}

	
	err = groth16.Verify(proof, vk, witnessPublic)
	if err != nil {
		panic(err)
	}

	fmt.Println("Legit Broker - Proof verified successfully")
	
}