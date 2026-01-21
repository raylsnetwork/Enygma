package test

import (
	"testing"
	"fmt"
	"math/big"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
    "github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/iden3/go-iden3-crypto/poseidon"
    "github.com/consensys/gnark/constraint/solver"
	"gnark_server/primitives"
)

type auctionCircuit struct {
    Commitment frontend.Variable
    // we expose the resulting ID as a public output
    Expected   frontend.Variable `gnark:",public"`
}


func (c *auctionCircuit) Define(api frontend.API) error {
    
    id := primitives.AuctionId(api, c.Commitment)
    
    api.AssertIsEqual(id, c.Expected)
    return nil
}


func TestAucionID(t *testing.T){
	num1,_ := new(big.Int).SetString("4125151263534231231", 10)
	Commiment, _ := poseidon.Hash([]*big.Int{num1})
    fmt.Println("Commitment", Commiment)
	var circuit auctionCircuit
    solver.RegisterHint(primitives.ModHint)

    cc, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
    if err != nil {
        t.Fatalf("compile error: %v", err)
    }

	pk, vk, err := groth16.Setup(cc)
    if err != nil {
        t.Fatalf("setup error: %v", err)
    }
     
	witness := auctionCircuit{
        Commitment: frontend.Variable(num1),
        Expected:   Commiment,
    }
    fullWit, err := frontend.NewWitness(&witness, ecc.BN254.ScalarField())
    if err != nil {
        t.Fatalf("new witness error: %v", err)
    }
    publicWit, err := fullWit.Public()
    if err != nil {
        t.Fatalf("public witness error: %v", err)
    }

    // 5) generate a proof
    proof, err := groth16.Prove(cc, pk, fullWit)
    if err != nil {
        t.Fatalf("prove error: %v", err)
    }

	  if err := groth16.Verify(proof, vk, publicWit); err != nil {
        t.Fatalf("verify error: %v", err)
    }
	fmt.Println("Proof verified successfully!")


}
