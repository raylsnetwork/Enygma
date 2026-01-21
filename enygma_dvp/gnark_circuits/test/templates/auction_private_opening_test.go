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



func TestAuctionPrivateOpening(t *testing.T) {
    // 1) Instantiate an empty circuit
    var circuit templates.AuctionPrivateOpeningCircuit
	solver.RegisterHint(primitives.ModHint)
	solver.RegisterHint(primitives.ERC155UniqueIdNative)
	solver.RegisterHint(primitives.PoseidonNative)

    // 2) Compile it to R1CS
    cc, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
    if err != nil {
        t.Fatalf("circuit compilation failed: %v", err)
    }

	pk, vk, err := groth16.Setup(cc)
    if err != nil {
        t.Fatalf("setup error: %v", err)
    }

	var witness templates.AuctionPrivateOpeningCircuit


	witness.StVaultId= frontend.Variable("2")
	witness.StBlindedBid = frontend.Variable("15074251153079271525615984288402449514308256302225335825857691378304772035514")
	witness.WtBidAmount = frontend.Variable("2719994")
	witness.WtBidRandom = frontend.Variable("292291162803841693174561616677868964484")

	witnessFull, err := frontend.NewWitness(&witness, ecc.BN254.ScalarField())
		if err != nil {
			log.Fatal(err)
		}
	proof, err := groth16.Prove(cc, pk, witnessFull)

	witnessPublic, err := frontend.NewWitness(&witness, ecc.BN254.ScalarField(), frontend.PublicOnly())
	if err != nil {
		log.Fatal(err)
	}

	
	err = groth16.Verify(proof, vk, witnessPublic)
	if err != nil {
		panic(err)
	}

	fmt.Println("Proof verified successfully!")
}