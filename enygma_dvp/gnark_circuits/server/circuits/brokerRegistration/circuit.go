package brokerRegistration

import(
	"math/big"
)

type BrokerRegistrationRequest struct {
	
	StBeacon        string 		`json:"stBeacon" binding:"required"`	
	StVaultId      	string      `json:"stVaultId" binding:"required"`
	StGroupId       string      `json:"stGroupId" binding:"required"`
	StDelegatorTreeNumbers      [2]string 		`json:"StDelegatorTreeNumbers" binding:"required,len=2""`
	StDelegatorMerkleRoots      [2]string 		`json:"stDelegatorMerkleRoots" binding:"required,len=2""`
	StDelegatorNullifier        [2]string 		`json:"stDelegatorNullifier" binding:"required,len=2""`

	StBrokerBlindedPublicKey       string      `json:"stBrokerBlindedPublicKey" binding:"required"`
	StBrokerMinComissionRate       string      `json:"stBrokerMinComissionRate" binding:"required"`
	StBrokerMaxComissionRate       string      `json:"stBrokerMaxComissionRate" binding:"required"`

	StAssetGroupTreeNumber       string      `json:"stAssetGroupTreeNumber" binding:"required"`
	StAssetGroupMerkleRoot       string      `json:"stAssetGroupMerkleRoot" binding:"required"`

	WtDelegatorPrivatekeys        [2]string 		`json:"wtDelegatorPrivatekeys" binding:"required,len=2""`
	WtDelegatorPathElements    	  [2][8]string		`json:"wtDelegatorPathElements" binding:"required,len=2,dive,len=8"`
	WtDelegatorPathIndices        [2]string 		`json:"wtDelegatorPathIndices" binding:"required,len=2""`
	WtDelegatorIdParams			  [2][8]string		`json:"wtDelegatorIdParams" binding:"required,len=2,dive,len=8"`
	WtContractAddress       string      `json:"wtContractAddress" binding:"required"`
	WtBrokerPublickey       string      `json:"wtBrokerPublickey" binding:"required"`	

	WtAssetGroupPathElements        [8]string 		`json:"wtAssetGroupPathElements" binding:"required,len=8""`
	WtAssetGroupPathIndices       string      `json:"wtAssetGroupPathIndices" binding:"required"`	
}


type BrokerRegistrationOutput struct{
	Proof 			[]*big.Int `json:"proof"`
	PublicSignal    []*big.Int `json:"publicSignal"`
}