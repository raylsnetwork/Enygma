package templates

import(
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/cmp"
	"gnark_server/primitives"
)

type Erc20CircuitConfig struct {
	TmNInputs int
	TmMOutputs  int
	TmMerkleTreeDepth int
	TmRange frontend.Variable 	
}

type Erc20Circuit struct {

	Config    				Erc20CircuitConfig
	StMessage      			frontend.Variable   `gnark:",public"` 
	StTreeNumber      		[]frontend.Variable  `gnark:",public"`  // nInputsERC20
	StMerkleRoots     		[]frontend.Variable  `gnark:",public"` // nInputsERC20
	StNullifiers  			[]frontend.Variable  `gnark:",public"` // nInputsERC20
	StCommitmentOut   		[]frontend.Variable  `gnark:",public"` //MOutputs
	
	WtPrivateKeysIn   		[]frontend.Variable  // nInputsERC20
	WtValuesIn				[]frontend.Variable   // nInputsERC20
	WtPathElements    		[][] frontend.Variable // nInputsERC20 //MerkleTreeDepthERC20
	WtPathIndices     		[]frontend.Variable // nInputsERC20
	WtErc20ContractAddress    frontend.Variable
	
	WtPublicKeysOut         []frontend.Variable //MOutputs
	WtValuesOut				[]frontend.Variable //MOutputs
}



func (circuit *Erc20Circuit) Define(api frontend.API) error{
	
	inputsTotals:=frontend.Variable(0)
	outputsTotals:=frontend.Variable(0)

	//verify input notes
	for i:=0; i< circuit.Config.TmNInputs;i++{
		isValid0 := cmp.IsLess(api, circuit.WtValuesIn[i], circuit.Config.TmRange)
		api.AssertIsEqual(isValid0, 1)

		isValid1 := cmp.IsLessOrEqual(api, 0,circuit.WtValuesIn[i] )
		api.AssertIsEqual(isValid1, 1)

		uniqueId  := primitives.UniqueId(api,circuit.WtErc20ContractAddress,circuit.WtValuesIn[i])

		publicKey := primitives.PublicKey(api, circuit.WtPrivateKeysIn[i])
		
		nullifier := primitives.Nullifier(api,circuit.WtPrivateKeysIn[i],circuit.WtPathIndices[i])
		
		api.AssertIsEqual(nullifier,circuit.StNullifiers[i])

		commitment :=primitives.Commitment(api, uniqueId,publicKey)

		pathElement := make([]frontend.Variable, circuit.Config.TmMerkleTreeDepth)
	
		for j:=0 ; j< circuit.Config.TmMerkleTreeDepth;j++{
			pathElement[j] = circuit.WtPathElements[i][j]
		}
		root := primitives.MerkleProof(api, commitment,circuit.WtPathIndices[i],pathElement)

		isZero := api.IsZero(circuit.WtValuesIn[i]) // ValueIn[i] ?0 =  1:0
		Enable := api.Mul(1,api.Sub(1,isZero))  
		Diff   := api.Sub(circuit.StMerkleRoots[i], root)

		api.AssertIsEqual(api.Mul(Diff, Enable), 0)

		inputsTotals = api.Add( inputsTotals , circuit.WtValuesIn[i])
	}

	//Verifying Outputs
	for i:=0; i< circuit.Config.TmMOutputs;i++{
		isValid0 := cmp.IsLess(api, circuit.WtValuesOut[i], circuit.Config.TmRange)
		api.AssertIsEqual(isValid0, 1)

		isValid1 := cmp.IsLessOrEqual(api, 0,circuit.WtValuesOut[i] )
		api.AssertIsEqual(isValid1, 1)

		uniqueId  := primitives.UniqueId(api,circuit.WtErc20ContractAddress,circuit.WtValuesOut[i])
		commitment :=primitives.Commitment(api, uniqueId,circuit.WtPublicKeysOut[i])
		api.AssertIsEqual(commitment, circuit.StCommitmentOut[i])

		outputsTotals = api.Add( outputsTotals , circuit.WtValuesOut[i])
	}

	api.AssertIsEqual(outputsTotals, inputsTotals)

	return nil
}



