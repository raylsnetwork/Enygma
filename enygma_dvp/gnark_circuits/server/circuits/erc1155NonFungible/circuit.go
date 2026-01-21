package erc1155NonFungible

import(
	"math/big"
)

// ownership_erc1155_Non_Fungible_config := templates.ERC1155NonFungibleCircuitConfig{
// 		NumOfTokens:10,
// 		MerkleTreeDepth:8,
// 		AssetGroupMerkleTreeDepth:8,
// 		}

type ERC1155NonFungibleRequest struct {
	
	StMessage        	string 		`json:"stMessage" binding:"required"`	
	StTreeNumbers   			[1]string      `json:"stTreeNumbers" binding:"required,len=1""`
	StMerkleRoots       		[1]string      `json:"StMerkleRoots" binding:"required,len=1""`
	StNullifiers      			[1]string 		`json:"stNullifiers" binding:"required,len=1""`
	StCommitmentOut      		[1]string 		`json:"stCommitmentOut" binding:"required,len=1""`
	StAssetGroupTreeNumber      [1]string 		`json:"stAssetGroupTreeNumber" binding:"required,len=1""`
	StAssetGroupMerkleRoot      [1]string 		`json:"stAssetGroupMerkleRoot" binding:"required,len=1""`
	
	WtPrivateKeysIn      			[1]string 		`json:"wtPrivateKeysIn" binding:"required,len=1""`
	WtValues      				[1]string 		`json:"wtValues" binding:"required,len=1""`
	WtPathElements				[1][8]string 		`json:"wtPathElements" binding:"required,len=1,dive,len=8"`

	WtPathIndices      			[1]string 		`json:"wtPathIndices" binding:"required,len=1""`
	WtErc1155TokenId      		[1]string 		`json:"wtErc1155TokenId" binding:"required,len=1""`
	WtPublicKeysOut      		[1]string 		`json:"wtPublicKeysOut" binding:"required,len=1""`

	WtErc1155ContractAddress     string      `json:"wtErc1155ContractAddress" binding:"required"`

	WtAssetGroupPathElements		[1][8]string 		`json:"wtAssetGroupPathElements" binding:"required,len=1,dive,len=8"`
	WtAssetGroupPathIndices      		[1]string 		`json:"wtAssetGroupPathIndices" binding:"required,len=1""`
	
}

type ERC1155NonFungibleOutput struct{
	Proof 			[]*big.Int `json:"proof"`
	PublicSignal    []*big.Int `json:"publicSignal"`
}
