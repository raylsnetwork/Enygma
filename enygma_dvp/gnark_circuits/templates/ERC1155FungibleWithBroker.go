package templates

import(
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/cmp"
	"gnark_server/primitives"
)

// const nInputsERC1155FungibleBroker = 2
// const mOutputsERC1155FungibleBroker = 3
// const MerkleTreeDepthERC1155FungibleBroker = 8
// const assetGroupMerkleTreeDepthERC1155FungibleBroker=8
// const maxPermittedCommissionRate=10
// const commissionRateDecimals=2
// const RangeERC1155FungibleBroker="1000000000000000000000000000000000000"

type ERC1155FungibleWithBrokerCircuitConfig struct{
	NInputs int
	MOutputs int
	MerkleTreeDepth int
	AssetGroupMerkleTreeDepth int
	MaxPermittedCommissionRate int
	ComissionRateDecimals int
	Range 			frontend.Variable
}

type Erc1155FungibleWithBrokerCircuit struct {

	Config 					 ERC1155FungibleWithBrokerCircuitConfig

	StMessage      			 frontend.Variable   `gnark:",public"` 
	StTreeNumbers      		 []frontend.Variable `gnark:",public"`    //NInputs
	StMerkleRoots     		 []frontend.Variable `gnark:",public"`   //NInputs
	StNullifiers  			 []frontend.Variable `gnark:",public"`   //NInputs
	StCommitmentOut   		 []frontend.Variable `gnark:",public"`  //MOutputs
	StBrokerBlindedPublicKey frontend.Variable   `gnark:",public"` 
	StBrokerCommisionRate    frontend.Variable   `gnark:",public"` 
	StAssetGroupTreeNumber   frontend.Variable   `gnark:",public"` 
	StAssetGroupMerkleRoot   frontend.Variable	 `gnark:",public"` 

	WtPrivateKeys			 []frontend.Variable //NInputs
	WtValuesIn    		 	 []frontend.Variable //NInputs
	WtPathElements   		 [][]frontend.Variable //NInputs //MerkleTreeDepth
	WtPathIndices     		 []frontend.Variable //NInputs
	WtErc1155ContractAddress frontend.Variable
	WtErc1155TokenId         frontend.Variable

	WtRecipientPk			 []frontend.Variable  //MOutput
	WtValuesOut              []frontend.Variable  //MOutput

	WtAssetGroupPathElements []frontend.Variable //AssetGroupMerkleTreeDepth
	WtAssetGroupPathIndices  frontend.Variable
	
}

func (circuit *Erc1155FungibleWithBrokerCircuit) Define(api frontend.API) error{
	inputsTotal :=frontend.Variable(0)
	outputsTotal:=frontend.Variable(0)

	blinder := primitives.Blinder(api, circuit.WtRecipientPk[2])
	api.AssertIsEqual(blinder, circuit.StBrokerBlindedPublicKey)

	erc1155uniqueId := primitives.Erc1155UniqueId(api, circuit.WtErc1155ContractAddress,circuit.WtErc1155TokenId,frontend.Variable(0))

	
	pathElement := make([]frontend.Variable, circuit.Config.MerkleTreeDepth)
	for j:=0 ; j< circuit.Config.MerkleTreeDepth;j++{
		pathElement[j] = circuit.WtAssetGroupPathElements[j]
	}
	root := primitives.MerkleProof(api, erc1155uniqueId,circuit.WtAssetGroupPathIndices,pathElement)
	api.AssertIsEqual(root,circuit.StAssetGroupMerkleRoot)


	for i:=0; i<circuit.Config.NInputs; i++{
		isValid0 := cmp.IsLess(api,circuit.WtValuesIn[i], circuit.Config.Range)
		api.AssertIsEqual(isValid0, 1)

		isValid1 := cmp.IsLessOrEqual(api, 0,circuit.WtValuesIn[i])
		api.AssertIsEqual(isValid1, 1)

		erc1155uniqueId := primitives.Erc1155UniqueId(api, circuit.WtErc1155ContractAddress,circuit.WtErc1155TokenId,circuit.WtValuesIn[i])

		publicKey := primitives.PublicKey(api,circuit.WtPrivateKeys[i])

		nullifier := primitives.Nullifier(api,circuit.WtPrivateKeys[i], circuit.WtPathIndices[i])
		api.AssertIsEqual(nullifier, circuit.StNullifiers[i])

		commitment  := primitives.Commitment(api,erc1155uniqueId,publicKey)

		pathElementVar := make([]frontend.Variable, circuit.Config.MerkleTreeDepth)
		for j:=0 ; j< circuit.Config.MerkleTreeDepth;j++{
			pathElementVar[j] = circuit.WtPathElements[i][j]
		}
		root := primitives.MerkleProof(api, commitment,circuit.WtPathIndices[i],pathElementVar)

		isZero := api.IsZero(circuit.WtValuesIn[i]) // ValueIn[i] ?0 =  1:0
		Enable := api.Mul(1,api.Sub(1,isZero))  

		Diff   := api.Sub(root, circuit.StMerkleRoots[i])
		api.AssertIsEqual(api.Mul(Diff, Enable), 0)

		inputsTotal = api.Add( inputsTotal , circuit.WtValuesIn[i])
	}

	isValid0 := cmp.IsLess(api,circuit.StBrokerCommisionRate, circuit.Config.Range)
	api.AssertIsEqual(isValid0, 1)
	
	power:=frontend.Variable(1)
	for z:=0; z<circuit.Config.ComissionRateDecimals;z++{
		power =  api.Mul(power,10)

	}

	api.AssertIsEqual(circuit.WtValuesOut[2], api.Mul(circuit.WtValuesOut[0], api.Div(circuit.StBrokerCommisionRate,power)))


	for i:=0; i< circuit.Config.MOutputs; i++{
		isValid2 := cmp.IsLess(api,circuit.WtValuesOut[i], circuit.Config.Range)
		api.AssertIsEqual(isValid2, 1)

		isValid3 := cmp.IsLessOrEqual(api, 0,circuit.WtValuesOut[i])
		api.AssertIsEqual(isValid3, 1)

		erc1155uniqueId2 := primitives.Erc1155UniqueId(api, circuit.WtErc1155ContractAddress,circuit.WtErc1155TokenId,circuit.WtValuesOut[i])
	
		commitment  := primitives.Commitment(api,erc1155uniqueId2,circuit.WtRecipientPk[i])

		api.AssertIsEqual(commitment, circuit.StCommitmentOut[i])

		outputsTotal = api.Add( outputsTotal , circuit.WtValuesOut[i])
	}

	api.AssertIsEqual(outputsTotal, inputsTotal)

	return nil
}