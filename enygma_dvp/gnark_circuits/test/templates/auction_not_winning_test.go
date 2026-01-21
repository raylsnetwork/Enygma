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

func TestAuctionNotWinning(t *testing.T) {
	var circuit templates.AuctionNotWinningCircuit 

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

	var witness templates.AuctionNotWinningCircuit
	witness.StVaultId = frontend.Variable("1")
	witness.StBlindedBidDifference = frontend.Variable("8228254630354675004232187547117538513024441602986046271893840932482592516375")
	witness.StBidBlockNumber = frontend.Variable("0")
	witness.StWinningBidBlockNumber = frontend.Variable("0")

	witness.WtBidAmount = frontend.Variable("1297333")
	witness.WtBidRandom = frontend.Variable("152553770450859978284081040009409152562")
	witness.WtWinningBidAmount = frontend.Variable("7312673")
	witness.WtWinningBidRandom = frontend.Variable("223094580719981088368667724876347698586")




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

	fmt.Println("Auction Not Winning Circuit Test - Proof verified successfully!")

}