package auctionInit

import(
	"math/big"
)
type AuctionInitRequest struct {
	StBeacon         string         `json:"stBeacon" binding:"required"`
	StVaultId        string 		`json:"stVaultId" binding:"required"`	
	StAuctionId      string         `json:"stAuctionId" binding:"required"`
	StTreeNumber     string 		`json:"stTreeNumber" binding:"required"`
	StMerkleRoot     string         `json:"stMerkleRoot" binding:"required"`
	StNullifier      string 		`json:"stNullifier" binding:"required"`
	StAssetGroupMerkleRoot string   `json:"stAssetGroupMerkleRoot" binding:"required"` 

	WtCommiment      string 		`json:"wtCommiment" binding:"required"`
	WtPathElements  [8]string		`json:"wtPathElements" binding:"required,len=8"`
	WtPathIndices   string			`json:"wtPathIndices" binding:"required"`
	WtPrivateKey    string			`json:"wtPrivateKey" binding:"required"`
	WtIdParams      [5]string		`json:"wtIdParams" binding:"required,len=5`
	WtContractAddress string		`json:"wtContractAddress" binding:"required"`	

	WtAssetGroupPathElements  [8]string		`json:"wtAssetGroupPathElements" binding:"required,len=8"`
	WtAssetGroupPathIndices string 	`json:"wtAssetGroupPathIndices" binding:"required`
}

type AuctionInitOutput struct{
	Proof 			[]*big.Int `json:"proof"`
	PublicSignal    []*big.Int `json:"publicSignal"`
}


