package acutionBid

import(
	"math/big"
)

type AuctionBidRequest struct{
	StAuctionId         string         `json:"stAuctionId" binding:"required"`
	StBlindedBid        string 		   `json:"stBlindedBid" binding:"required"`	
	StVaultId      		string         `json:"stVaultId" binding:"required"`
	StTreeNumber        [2]string      `json:"stTreeNumber" binding:"required,len=2"`
	StMerkleRoot        [2]string      `json:"stMerkleRoot" binding:"required,len=2"`
	StNullifier         [2]string      `json:"stNullifier" binding:"required,len=2"`
	StCommitmentsOuts   [2]string      `json:"stCommitmentsOuts" binding:"required,len=2"`
	StAssetGroupMerkleRoot string	   `json:"stAssetGroupMerkleRoot" binding:"required"`

	WtBidAmount 		string	   		`json:"wtBidAmount" binding:"required"`
	WtBidRandom 		string	   		`json:"wtBidRandom" binding:"required"`

	WtPrivateKeysIn     [2]string			`json:"wtPrivateKeys" binding:"required,len=2"`
	WtPathElements    [2][8]string		`json:"wtPathElements" binding:"required,len=2,dive,len=8"`
	WtPathIndices     [2]string			`json:"wtPathIndices" binding:"required,len=2"`

	WtContractAddress 		string	   	`json:"wtContractAddress" binding:"required"`
	WtPublicKeysOut			[2]string 		`json:"wtRecipientPK" binding:"required,len=2"`

	WtAssetGroupPathElements [8]string 		`json:"wtAssetGroupPathElements" binding:"required,len=8"`
	WtAssetGroupPathIndices string	   	`json:"wtAssetGroupPathIndices" binding:"required"`

	WtIdParamsIn    	[2][5]string		`json:"wtIdParamsIn" binding:"required,len=2,dive,len=5"`
	WtIdParamsOut    	[2][5]string		`json:"wtIdParamsOut" binding:"required,len=2,dive,len=5"`
}


type AuctionBidOutput struct{
	Proof 			[]*big.Int `json:"proof"`
	PublicSignal    []*big.Int `json:"publicSignal"`
}


