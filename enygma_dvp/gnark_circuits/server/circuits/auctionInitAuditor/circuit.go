package auctionInitAuditor

import(
	"math/big"
)
type AuctionInitAuditorRequest struct {
	StBeacon         string         `json:"stBeacon" binding:"required"`
	StVaultId        string 		`json:"stVaultId" binding:"required"`	
	StAuctionId      string         `json:"stAuctionId" binding:"required"`
	StTreeNumber     string 		`json:"stTreeNumber" binding:"required"`
	StMerkleRoot     string         `json:"stMerkleRoot" binding:"required"`
	StNullifier      string 		`json:"stNullifier" binding:"required"`

	StAuditorPublicKey 		 [2]string	`json:"stAuditorPublicKey" binding:"required,len=2"`
	StAuditorAuthKey   		 [2]string	`json:"stAuditorAuthKey" binding:"required,len=2"`
	StAuditorNonce 				string	`json:"stAuditorNonce" binding:"required"`
	StAuditorEncryptedValues [7]string  `json:"stAuditorEncryptedValues" binding:"required,len=7"`
	WtAuditorRandom 			string  `json:"wtAuditorRandom" binding:"required"`
	
	StAssetGroupTreeNumber      string 		`json:"stAssetGroupTreeNumber" binding:"required"`
	StAssetGroupMerkleRoot      string 		`json:"stAssetGroupMerkleRoot" binding:"required"`

	WtCommitment      string 		`json:"wtCommitment" binding:"required"`
	WtPathElements  [8]string		`json:"wtPathElements" binding:"required,len=8"`
	WtPathIndices   string			`json:"wtPathIndices" binding:"required"`
	WtPrivateKey    string			`json:"wtPrivateKey" binding:"required"`
	WtIdParams      [5]string		`json:"wtIdParams" binding:"required,len=5`
	WtContractAddress string		`json:"wtContractAddress" binding:"required"`	

	WtAssetGroupPathElements  [8]string		`json:"wtAssetGroupPathElements" binding:"required,len=8"`
	WtAssetGroupPathIndices 	string 	`json:"wtAssetGroupPathIndices" binding:"required`
}

type AuctionInitAuditorOutput struct{
	Proof 			[]*big.Int `json:"proof"`
	PublicSignal    []*big.Int `json:"publicSignal"`
}


