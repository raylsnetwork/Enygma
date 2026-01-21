package erc1155NonFungibleAuditor

import(
	"math/big"
)

// ownership_erc1155_Non_Fungible_config := templates.ERC1155NonFungibleCircuitConfig{
// 		NumOfTokens:10,
// 		MerkleTreeDepth:8,
// 		AssetGroupMerkleTreeDepth:8,
// 		}

type ERC1155NonFungibleAuditorRequest struct {
	
	StMessage        			string 		   `json:"stMessage" binding:"required"`	
	StTreeNumbers   			[1]string      `json:"stTreeNumbers" binding:"required,len=1""`
	StMerkleRoots       		[1]string      `json:"StMerkleRoots" binding:"required,len=1""`
	StNullifiers      			[1]string 	   `json:"stNullifiers" binding:"required,len=1""`
	StCommitmentOut      		[1]string 	   `json:"stCommitmentOut" binding:"required,len=1""`
	
	StAuditorPublickey  		[2]string	    `json:"stAuditorPublicKey" binding:"required,len=2"`
	StAuditorAuthKey    		[2]string	    `json:"stAuditorAuthKey" binding:"required,len=2"`
	StAuditorNonce      		string			`json:"stAuditorNonce" binding:"required"`
	StAuditorEncryptedValues  	[4]string	    `json:"stAuditorEncryptedValues" binding:"required,len=4"`
	WtAuditorRandom				string 			`json:"wtAuditorRandom" binding:"required"`

	StAssetGroupTreeNumber      [1]string 		`json:"stAssetGroupTreeNumber" binding:"required,len=1""`
	StAssetGroupMerkleRoot      [1]string 		`json:"stAssetGroupMerkleRoot" binding:"required,len=1""`

	WtPrivateKeysIn      		[1]string 		`json:"wtPrivateKeysIn" binding:"required,len=1""`
	WtValues      				[1]string 		`json:"wtValues" binding:"required,len=1""`
	WtPathElements				[1][8]string 	`json:"wtPathElements" binding:"required,len=1,dive,len=8"`
	WtPathIndices               [1]string 		`json:"wtPathIndices" binding:"required,len=1""`
	WtErc1155TokenIds            [1]string 		`json:"wtErc1155TokenIds" binding:"required,len=1""`
	WtErc1155ContractAddress	string 			`json:"wtErc1155ContractAddress" binding:"required"`

	WtPublicKeysOut      		[1]string 		`json:"wtPublicKeysOut" binding:"required,len=1""`

	WtAssetGroupPathElements	[1][8]string 	`json:"wtAssetGroupPathElements" binding:"required,len=1,dive,len=8"`
	WtAssetGroupPathIndices     [1]string 		`json:"wtAssetGroupPathIndices" binding:"required,len=1""`
	
}

type ERC1155NonFungibleAuditorOutput struct{
	Proof 			[]*big.Int `json:"proof"`
	PublicSignal    []*big.Int `json:"publicSignal"`
}
