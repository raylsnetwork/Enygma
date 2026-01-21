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



type ERC155UniqueIdCircuit struct {
	Erc1155ContractAddress frontend.Variable
	Erc1155TokenId frontend.Variable
	Amount frontend.Variable
	
    // we expose the resulting ID as a public output
    Expected   frontend.Variable `gnark:",public"`
}

func (c *ERC155UniqueIdCircuit) Define(api frontend.API) error {
    
    commitment := primitives.Erc1155UniqueId(api, c.Erc1155ContractAddress, c.Erc1155TokenId, c.Amount)
    
    api.AssertIsEqual(commitment, c.Expected)
    return nil
}

func TestERC155UniqueId(t *testing.T){
	Address,_ := new(big.Int).SetString("4125151263534231231", 10)
	TokenId,_ := new(big.Int).SetString("1241412", 10)
	Amount,_  := new(big.Int).SetString("1000", 10)

	Hash0, _ := poseidon.Hash([]*big.Int{Address,TokenId})
	Hash1, _ := poseidon.Hash([]*big.Int{Hash0,Amount})
   
	var circuit ERC155UniqueIdCircuit
    cc, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
    if err != nil {
        t.Fatalf("compile error: %v", err)
    }

	pk, vk, err := groth16.Setup(cc)
    if err != nil {
        t.Fatalf("setup error: %v", err)
    }
     
	witness := ERC155UniqueIdCircuit{
        Erc1155ContractAddress: frontend.Variable(Address),
		Erc1155TokenId: frontend.Variable(TokenId),
		Amount: frontend.Variable(Amount),
        Expected:   Hash1,
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
