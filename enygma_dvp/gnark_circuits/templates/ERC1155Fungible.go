package templates

import(
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/cmp"
	"gnark_server/primitives"
)

// const nInputsERC1155Fungible = 1
// const mOutputsERC1155Fungible = 1
// const MerkleTreeDepthERC1155Fungible = 8
// const assetGroupMerkleTreeDepthERC1155Fungible=8
// const RangeERC1155Fungible="1000000000000000000000000000000000000"

type ERC1155FungibleCircuitConfig struct{
	TmNInputs int
	TmMOutputs int
	TmMerkleTreeDepth int
	TmAssetGroupMerkleTree int
	TmRange frontend.Variable
}


type Erc1155FungibleCircuit struct {

	Config   				 ERC1155FungibleCircuitConfig
	StMessage      			 frontend.Variable 	 `gnark:",public"`
	StTreeNumbers      		 []frontend.Variable `gnark:",public"`  //NInputs
	StMerkleRoots     		 []frontend.Variable `gnark:",public"`  //NInputs	
	StNullifiers  			 []frontend.Variable `gnark:",public"`  //NInputs
	StCommitmentOut   		 []frontend.Variable `gnark:",public"`  //MOutputs
	StAssetGroupMerkleRoot   frontend.Variable   `gnark:",public"`  
	StAssetGroupTreeNumber   frontend.Variable   `gnark:",public"`
	
	WtPrivateKeysIn		     []frontend.Variable //NInputs
	WtValuesIn    		 	 []frontend.Variable //NInputs
	WtPathElements   		 [][]frontend.Variable //NInputs//MerkleTreeDepth
	WtPathIndices     		 []frontend.Variable //NInputs
	WtErc1155ContractAddress frontend.Variable
	WtErc1155TokenId         frontend.Variable

	WtPublicKeysOut			 []frontend.Variable //MOutputs
	WtValuesOut              []frontend.Variable //MOutputs

	WtAssetGroupPathElements []frontend.Variable //AssetGroupMerkleTree
	WtAssetGroupPathIndices  frontend.Variable
	
}

func (circuit *Erc1155FungibleCircuit) Define(api frontend.API) error{

	inputsTotal :=frontend.Variable(0)
	outputsTotal:=frontend.Variable(0)

	erc1155uniqueId := primitives.Erc1155UniqueId2(api, circuit.WtErc1155ContractAddress,circuit.WtErc1155TokenId,frontend.Variable(0))
	
	pathElement := make([]frontend.Variable, circuit.Config.TmMerkleTreeDepth)
	for j:=0 ; j< circuit.Config.TmMerkleTreeDepth;j++{
		pathElement[j] = circuit.WtAssetGroupPathElements[j]
	}
	root := primitives.MerkleProofNative(api, erc1155uniqueId,circuit.WtAssetGroupPathIndices,pathElement)

	
	api.AssertIsEqual(root,circuit.StAssetGroupMerkleRoot)

	for i:=0; i< circuit.Config.TmNInputs; i++{

		isValid0 := cmp.IsLess(api,circuit.WtValuesIn[i], circuit.Config.TmRange)
		api.AssertIsEqual(isValid0, 1)

		isValid1 := cmp.IsLessOrEqual(api, 0,circuit.WtValuesIn[i])
		api.AssertIsEqual(isValid1, 1)

		erc1155uniqueId := primitives.Erc1155UniqueId2(api, circuit.WtErc1155ContractAddress,circuit.WtErc1155TokenId,circuit.WtValuesIn[i])

		publicKey := primitives.PublicKeyNative(api,circuit.WtPrivateKeysIn[i])

		nullifier := primitives.Nullifier(api,circuit.WtPrivateKeysIn[i], circuit.WtPathIndices[i])
		api.AssertIsEqual(nullifier, circuit.StNullifiers[i])

		commitment  := primitives.CommitmentNative(api,erc1155uniqueId,publicKey)

		pathElementVar := make([]frontend.Variable, circuit.Config.TmMerkleTreeDepth)
		for j:=0 ; j<  circuit.Config.TmMerkleTreeDepth;j++{
			pathElementVar[j] = circuit.WtPathElements[i][j]
		}
		root := primitives.MerkleProofNative(api, commitment,circuit.WtPathIndices[i],pathElementVar)

		isZero := api.IsZero(circuit.WtValuesIn[i]) // ValueIn[i] ?0 =  1:0
		Enable := api.Mul(1,api.Sub(1,isZero))  
		Diff   := api.Sub(root, circuit.StMerkleRoots[i])
		
		api.AssertIsEqual(api.Mul(Diff, Enable), 0)

		inputsTotal = api.Add( inputsTotal , circuit.WtValuesIn[i])
		
	}

	for i:=0; i< circuit.Config.TmMOutputs; i++{
		isValid2 := cmp.IsLess(api,circuit.WtValuesOut[i], circuit.Config.TmRange)
		api.AssertIsEqual(isValid2, 1)

		isValid3 := cmp.IsLessOrEqual(api, 0,circuit.WtValuesOut[i])
		api.AssertIsEqual(isValid3, 1)

		erc1155uniqueId2 := primitives.Erc1155UniqueId(api, circuit.WtErc1155ContractAddress,circuit.WtErc1155TokenId,circuit.WtValuesOut[i])
	
		commitment  := primitives.Commitment(api,erc1155uniqueId2,circuit.WtPublicKeysOut[i])

		api.AssertIsEqual(commitment, circuit.StCommitmentOut[i])

		outputsTotal = api.Add( outputsTotal , circuit.WtValuesOut[i])
	}
	
	api.AssertIsEqual(outputsTotal, inputsTotal)

	return nil
}
