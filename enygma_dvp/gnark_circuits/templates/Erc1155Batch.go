package templates

import(
	"github.com/consensys/gnark/frontend"
	"gnark_server/primitives"
)



type Erc1155BatchCircuitConfig struct{
	TmNumOfTokens int
	TmMerkleTreeDepth int
}

type Erc1155BatchCircuit struct {
	Config				     Erc1155BatchCircuitConfig
	StMessage      			 frontend.Variable 
	StTreeNumbers      		 []frontend.Variable   
	StMerkleRoots     		 []frontend.Variable
	StNullifiers  			 []frontend.Variable
	StCommitmentOut   		 []frontend.Variable
	StMembershipMerkleRoots  []frontend.Variable 
	
	WtPrivateKeys			 []frontend.Variable
	WtValues    		 	 []frontend.Variable
	WtPathElements   		 [][]frontend.Variable
	WtPathIndices     		 []frontend.Variable
	WtErc1155TokenIds     	 []frontend.Variable
	WtOutPublicKeys          []frontend.Variable
	WtErc1155ContractAddress frontend.Variable
	WtMembershipPathElements [][]frontend.Variable
	WtMembershipPathIndices  []frontend.Variable

}

func (circuit *Erc1155BatchCircuit) Define(api frontend.API) error{

	for i:=0; i< circuit.Config.TmNumOfTokens; i++{
		isZero := api.IsZero(circuit.WtValues[i]) // ValueIn[i] ?0 =  1:0
		Enable := api.Mul(1,isZero)  

		publicKey := primitives.PublicKey(api,circuit.WtPrivateKeys[i])

		nullifier := primitives.Nullifier(api,circuit.WtPrivateKeys[i], circuit.WtPathIndices[i])

		Diff   := api.Sub(nullifier, circuit.StNullifiers[i])
		api.AssertIsEqual(api.Mul(Diff, Enable), 0)

		erc1155uniqueId := primitives.Erc1155UniqueId(api, circuit.WtErc1155ContractAddress,circuit.WtErc1155TokenIds[i],circuit.WtValues[i])

		commitment  := primitives.Commitment(api,erc1155uniqueId,publicKey)

		var pathElement []frontend.Variable
		for j:=0 ; j< circuit.Config.TmMerkleTreeDepth;j++{
			pathElement[j] = circuit.WtPathElements[i][j]
		}
		root := primitives.MerkleProof(api, commitment,circuit.WtPathIndices[i],pathElement)

		Diff2   := api.Sub(root, circuit.StMerkleRoots[i])
		api.AssertIsEqual(api.Mul(Diff2, Enable), 0)

		commitment2 :=primitives.Commitment(api,erc1155uniqueId,circuit.WtOutPublicKeys[i])

		Diff3   := api.Sub(commitment2, circuit.StCommitmentOut[i])
		api.AssertIsEqual(api.Mul(Diff3, Enable), 0)
	}
	return nil
}
