package erc1155FungibleWithBroker

import(
	"math/big"
)

type ERC1155FungibleWithBrokerRequest struct {
	
	StMessage        	string 		`json:"stMessage" binding:"required"`	
	StTreeNumbers   	[2]string      `json:"stTreeNumbers" binding:"required,len=2""`
	StMerkleRoots       [2]string      `json:"StMerkleRoots" binding:"required,len=2""`
	StNullifiers      	[2]string 		`json:"stNullifiers" binding:"required,len=2""`
	StCommitmentOut      [3]string 		`json:"stCommitmentOut" binding:"required,len=3""`

	StBrokerBlindedPublicKey     string      `json:"stBrokerBlindedPublicKey" binding:"required"`
	StBrokerCommisionRate        string      `json:"stBrokerCommisionRate" binding:"required"`
	StAssetGroupTreeNumber       string      `json:"stAssetGroupTreeNumber" binding:"required"`
	StAssetGroupMerkleRoot       string      `json:"stAssetGroupMerkleRoot" binding:"required"`

	WtPrivateKeys      [2]string 		`json:"wtPrivateKeys" binding:"required,len=2""`
	WtValuesIn      	[2]string 			`json:"wtValuesIn" binding:"required,len=2""`
	WtPathElements      [2][8]string 		`json:"wtPathElements" binding:"required,len=2,dive,len=8"`
	WtPathIndices      [2]string 			`json:"wtPathIndices" binding:"required,len=2""`

	WtErc1155ContractAddress       string      `json:"wtErc1155ContractAddress" binding:"required"`
	WtErc1155TokenId       		   string      `json:"wtErc1155TokenId" binding:"required"`

	WtRecipientPk      [3]string 			`json:"wtRecipientPk" binding:"required,len=3""`
	WtValuesOut      [3]string 			`json:"wtValuesOut" binding:"required,len=3""`

	WtAssetGroupPathElements      [8]string 			`json:"wtAssetGroupPathElements" binding:"required,len=8""`
	WtAssetGroupPathIndices       		   string      `json:"wtAssetGroupPathIndices" binding:"required"`
}

type ERC1155FungibleWithBrokerOutput struct{
	Proof 			[]*big.Int `json:"proof"`
	PublicSignal    []*big.Int `json:"publicSignal"`
}
