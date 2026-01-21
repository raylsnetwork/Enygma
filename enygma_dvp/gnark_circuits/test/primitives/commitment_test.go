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
	"gnark_server/primitives"
)


type commitmentCircuit struct {
	UniqueId frontend.Variable
    PublicKey frontend.Variable
	
    // we expose the resulting ID as a public output
    Expected   frontend.Variable `gnark:",public"`
}


func (c *commitmentCircuit) Define(api frontend.API) error {
    
    commitment := primitives.Commitment(api, c.UniqueId, c.PublicKey)
    
    api.AssertIsEqual(commitment, c.Expected)
    return nil
}

func TestCommtiment(t *testing.T){
	num1,_ := new(big.Int).SetString("1412412", 10)
	num2,_ := new(big.Int).SetString("4125151263534231231", 10)
	Commiment, _ := poseidon.Hash([]*big.Int{num1,num2})

    
	var circuit commitmentCircuit
    cc, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
    if err != nil {
        t.Fatalf("compile error: %v", err)
    }

	pk, vk, err := groth16.Setup(cc)
    if err != nil {
        t.Fatalf("setup error: %v", err)
    }
     
	witness := commitmentCircuit{
        UniqueId: frontend.Variable(num1),
		PublicKey:frontend.Variable(num2),
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
