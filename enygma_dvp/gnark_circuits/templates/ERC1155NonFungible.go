package templates

import(
	"github.com/consensys/gnark/frontend"
	"gnark_server/primitives"
)

// const numOfTokens = 10
// const MerkleTreeDepthERC1155NonFungible = 8 
// const assetGroupMerkleTreeDepthERC1155NonFungible=8

type ERC1155NonFungibleCircuitConfig struct{
	TmNumOfTokens int
	TmMerkleTreeDepth int
	TmAssetGroupMerkleTreeDepth int
}


type ERC1155NonFungibleCircuit struct {
	
	Config 					 ERC1155NonFungibleCircuitConfig

	StMessage      			 frontend.Variable  `gnark:",public"` 
	StTreeNumbers      		 []frontend.Variable   `gnark:",public"` // numOfTokens
	StMerkleRoots     		 []frontend.Variable `gnark:",public"` // numOfTokens
	StNullifiers  			 []frontend.Variable `gnark:",public"`  // numOfTokens
	StCommitmentOut   		 []frontend.Variable `gnark:",public"`  // numOfTokens
	StAssetGroupTreeNumber	 []frontend.Variable `gnark:",public"`  // numOfTokens
	StAssetGroupMerkleRoot	 []frontend.Variable `gnark:",public"`  // numOfTokens

	WtPrivateKeysIn		 	 []frontend.Variable //numOfTokens
	WtValues    		 	 []frontend.Variable //numOfTokens
	WtPathElements   		 [][]frontend.Variable //numOfTokens  //MerkleTreeDepthERC1155NonFungible
	WtPathIndices     		 []frontend.Variable //numOfTokens
	WtErc1155TokenId         []frontend.Variable //numOfTokens
	WtPublicKeysOut	     	 []frontend.Variable //numOfTokens
	WtErc1155ContractAddress frontend.Variable

	WtAssetGroupPathElements [][]frontend.Variable //numOfTokens  //AssetGroupMerkleTreeDepth
	WtAssetGroupPathIndices  []frontend.Variable //numOfTokens
	
}

func (circuit *ERC1155NonFungibleCircuit) Define(api frontend.API) error{
	
	for i:=0; i < circuit.Config.TmNumOfTokens; i++{
		
		isZero := api.IsZero(circuit.WtValues[i]) // ValueIn[i] ?0 =  1:0
		Enable := api.Mul(1,api.Sub(1,isZero))
	
		erc1155uniqueId :=  primitives.Erc1155UniqueId2(api, circuit.WtErc1155ContractAddress,circuit.WtErc1155TokenId[i],frontend.Variable(0))
		
		pathElement := make([]frontend.Variable, circuit.Config.TmAssetGroupMerkleTreeDepth)
		for j:=0 ; j< circuit.Config.TmAssetGroupMerkleTreeDepth;j++{
			pathElement[j] = circuit.WtAssetGroupPathElements[i][j]
		}
		root := primitives.MerkleProofNative(api, erc1155uniqueId,circuit.WtAssetGroupPathIndices[i],pathElement)
		
		Diff   := api.Sub(root, circuit.StAssetGroupMerkleRoot[i])
		
		api.AssertIsEqual(api.Mul(Diff, Enable), 0)

		publicKey := primitives.PublicKeyNative(api,circuit.WtPrivateKeysIn[i])
		
		nullifier := primitives.Nullifier(api,circuit.WtPrivateKeysIn[i], circuit.WtPathIndices[i])
		
		Diff2   := api.Sub(nullifier, circuit.StNullifiers[i])
	
		api.AssertIsEqual(api.Mul(Diff2, Enable), 0)
		
		erc1155uniqueId2 := primitives.Erc1155UniqueId2(api, circuit.WtErc1155ContractAddress,circuit.WtErc1155TokenId[i],circuit.WtValues[i])
		
	
		commitment  := primitives.CommitmentNative(api,erc1155uniqueId2,publicKey)
		
		merklePathElement := make([]frontend.Variable,circuit.Config.TmMerkleTreeDepth)
		for j:=0 ; j< circuit.Config.TmMerkleTreeDepth;j++{
			merklePathElement[j] = circuit.WtPathElements[i][j]
		}
		root2 := primitives.MerkleProofNative(api,commitment ,circuit.WtPathIndices[i],merklePathElement)
		
		Diff3  := api.Sub(root2, circuit.StMerkleRoots[i])
		
		api.AssertIsEqual(api.Mul(Diff3, Enable), 0)
		
		commitment2  := primitives.CommitmentNative(api,erc1155uniqueId2,circuit.WtPublicKeysOut[i])

		Diff4  := api.Sub(circuit.StCommitmentOut[i], commitment2)
		
		api.AssertIsEqual(api.Mul(Diff4, Enable), 0)

	}
	return nil
}


