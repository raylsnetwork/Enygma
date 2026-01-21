package auctionBidAuditor

import(
	"math/big"
)

type AuctionBidAuditorRequest struct{
	StBeacon			string 		   `json:"stBeacon" binding:"required"`
	StAuctionId         string         `json:"stAuctionId" binding:"required"`
	StBlindedBid        string 		   `json:"stBlindedBid" binding:"required"`	
	StVaultId      		string         `json:"stVaultId" binding:"required"`
	StTreeNumbers       [2]string      `json:"stTreeNumbers" binding:"required,len=2"`
	StMerkleRoots       [2]string      `json:"stMerkleRoots" binding:"required,len=2"`
	StNullifiers        [2]string      `json:"stNullifiers" binding:"required,len=2"`
	StCommitmentsOuts   [2]string      `json:"stCommitmentsOuts" binding:"required,len=2"`
	StAssetGroupTreeNumber string	   `json:"stAssetGroupTreeNumber" binding:"required"`	
	StAssetGrupoMerkleRoot string	   `json:"stAssetGrupoMerkleRoot" binding:"required"`


	StAuctioneerPublicKey  [2]string	    `json:"stAuctioneerPublicKey" binding:"required,len=2"`
	StAuctioneerAuthKey    [2]string	    `json:"stAuctioneerAuthKey" binding:"required,len=2"`
	StAuctioneerNonce      string			`json:"stAuctioneerNonce" binding:"required"`
	StAuctioneerEncryptedValues  [4]string	`json:"stAuctioneerEncryptedValues" binding:"required,len=4"`
	WtAuctioneerRandom		string 			`json:"wtAuctioneerRandom" binding:"required"`

	StAuditorPublicKey  [2]string	    `json:"stAuditorPublicKey" binding:"required,len=2"`
	StAuditorAuthKey    [2]string	     `json:"stAuditorAuthKey" binding:"required,len=2"`
	StAuditorNonce      string			`json:"stAuditorNonce" binding:"required"`
	StAuditorEncryptedValues  [25]string	    `json:"stAuditorEncryptedValues" binding:"required,len=25"`
	WtAuditoRandom		string 			`json:"wtAuditoRandom" binding:"required"`

	WtBidAmount 		string	   		`json:"wtBidAmount" binding:"required"`
	WtBidRandom 		string	   		`json:"wtBidRandom" binding:"required"`

	WtPrivateKeysIn     [2]string			`json:"wtPrivateKeysIn" binding:"required,len=2"`
	WtPathElements    	[2][8]string		`json:"wtPathElements" binding:"required,len=2,dive,len=8"`
	WtPathIndices    	[2]string			`json:"wtPathIndices" binding:"required,len=2"`
	WtContractAddress 		string	   	`json:"wtContractAddress" binding:"required"`
	
	WtPublicKeysOut			[2]string 		`json:"wtPublicKeysOut" binding:"required,len=2"`

	WtAssetGroupPathElements [8]string 		`json:"wtAssetGroupPathElements" binding:"required,len=8"`
	WtAssetGroupPathIndices string	   		`json:"wtAssetGroupPathIndices" binding:"required"`

	WtIdParamsIn    	[2][5]string		`json:"wtIdParamsIn" binding:"required,len=2,dive,len=5"`
	WtIdParamsOut    	[2][5]string		`json:"wtIdParamsOut" binding:"required,len=2,dive,len=5"`
}


type AuctionBidAuditorOutput struct{
	Proof 			[]*big.Int `json:"proof"`
	PublicSignal    []*big.Int `json:"publicSignal"`
}


