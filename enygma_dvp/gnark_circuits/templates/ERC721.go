package templates

import(
	"github.com/consensys/gnark/frontend"
	"gnark_server/primitives"
)

// const nInputsERC721 = 1
// const mOutputsERC721 = 1
// const MerkleTreeDepthERC721=8

type Erc721CircuitConfig struct{
	TmNumOfTokens int
	TmMerkleTreeDepth int
}

type Erc721Circuit struct {

	Config   				Erc721CircuitConfig
	StMessage      			frontend.Variable    `gnark:",public"` 
	StTreeNumbers      		[]frontend.Variable    `gnark:",public"` 
	StMerkleRoots     		[]frontend.Variable    `gnark:",public"` 
	StNullifiers  			[]frontend.Variable  `gnark:",public"`  //NInputs
	StCommitmentOut   		[]frontend.Variable  `gnark:",public"`  //MOutputs
	
	WtPrivateKeysIn   		[]frontend.Variable //NInputs
	WtValues				[]frontend.Variable //NInputs
	WtPathElements    		[][] frontend.Variable //NInputs //MerkleTreeDepth 
	WtPathIndices     		[]frontend.Variable //NInputs
	WtPublicKeysOut         []frontend.Variable //MOutputs
	
}


func (circuit *Erc721Circuit) Define(api frontend.API) error{
	
	

	//verify input notes
	for i:=0; i< circuit.Config.TmNumOfTokens;i++{

		publicKey := primitives.PublicKey(api, circuit.WtPrivateKeysIn[i])

		nullifier := primitives.Nullifier(api,circuit.WtPrivateKeysIn[i],circuit.WtPathIndices[i])
		api.AssertIsEqual(nullifier,circuit.StNullifiers[i])

		commitment :=primitives.Commitment(api, circuit.WtValues[i],publicKey)
		
		pathElement := make([]frontend.Variable, circuit.Config.TmMerkleTreeDepth)

		for j:=0 ; j< circuit.Config.TmMerkleTreeDepth;j++{
			pathElement[j] = circuit.WtPathElements[i][j]
		}
		root := primitives.MerkleProof(api, commitment,circuit.WtPathIndices[i],pathElement)

		isZero := api.IsZero(circuit.WtValues[i]) // ValueIn[i] ?0 =  1:0
		Enable := api.Mul(1,isZero)  

		Diff   := api.Sub(circuit.StMerkleRoots[i], root)

		api.AssertIsEqual(api.Mul(Diff, Enable), 0)
	
		commitmentOut :=primitives.Commitment(api, circuit.WtValues[i],circuit.WtPublicKeysOut[i])
		api.AssertIsEqual(commitmentOut, circuit.StCommitmentOut[i])
		
	}

	return nil
}
