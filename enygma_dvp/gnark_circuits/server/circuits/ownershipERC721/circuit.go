package ownershipERC721

import(
	"math/big"
)
type OwnershipERC721Request struct {
	StMessage          string                     	`json:"stMessage" binding:"required"`
	StTreeNumbers     [1]string				      	`json:"stTreeNumbers" binding:"required,len=1"`
	StMerkleRoots     [1]string                   	`json:"stMerkleRoots" binding:"required,len=1"`
	StNullifiers      [1]string                   	`json:"stNullifiers" binding:"required,len=1"`
	StCommitmentOut   [1]string                   	`json:"stCommitmentOut" binding:"required,len=1"`
	WtPrivateKeysIn   [1]string                  	`json:"wtPrivateKeysIn" binding:"required,len=1"`
	WtValues          [1]string                  	`json:"wtValues" binding:"required,len=1"`
	WtPathElements    [1][8]string     		   		`json:"wtPathElements" binding:"required,len=1,dive,len=8"`
	WtPathIndices     [1]string                  	`json:"wtPathIndices" binding:"required,len=1"`
	WtPublicKeysOut   [1]string                  	`json:"wtPublicKeysOut" binding:"required,len=1"`
}

type OwnershipERC721Output struct{
	Proof 			[]*big.Int `json:"proof"`
	PublicSignal    []*big.Int `json:"publicSignal"`
}


//Definition already in templates folder