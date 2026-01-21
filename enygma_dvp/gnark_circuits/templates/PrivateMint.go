package templates

import(
	"github.com/consensys/gnark/frontend"
	"gnark_server/primitives"
)
// const maxCommissionPercentage = 4
// const commissionPercentageDecimals = 4

type PrivateMintConfig struct{

}

type PrivateMintCircuit struct {
	Commitment      	     	 frontend.Variable  `gnark:",public"` 
	ContractAddress				 frontend.Variable  `gnark:",public"`
	TokenId						 frontend.Variable  `gnark:",public"` 
	CipherText				     frontend.Variable  `gnark:",public"`

	Salt				 		 frontend.Variable 
	Amount						 frontend.Variable
	PublicKey				     frontend.Variable
	
	
}


func (circuit *PrivateMintCircuit) Define(api frontend.API) error{


	erc1155uniqueId :=  primitives.Erc1155UniqueId2(api, circuit.ContractAddress,circuit.TokenId,circuit.Amount)
	
	calculatedCommitmentPart1  := primitives.Commitment(api,erc1155uniqueId,circuit.PublicKey)

	calculatedCommitmentPart2  := primitives.Commitment(api,calculatedCommitmentPart1,circuit.Salt)

	api.AssertIsEqual(calculatedCommitmentPart2,circuit.Commitment)

	calculatedCipherText  := primitives.Commitment(api,circuit.PublicKey,circuit.Salt)

	api.AssertIsEqual(calculatedCipherText,circuit.CipherText)

	return nil
}
