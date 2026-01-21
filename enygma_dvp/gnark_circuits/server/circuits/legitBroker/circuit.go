package legitBroker

import(
	"math/big"
)
type LegitBrokerRequest struct {
	StBeacon         string                     `json:"stBeacon" binding:"required"`
	StBlindedPublicKey         string                     `json:"stBlindedPublicKey" binding:"required"`
	WtPrivatekey         string                     `json:"wtPrivatekey" binding:"required"`
	
}
type LegitBrokerOutput struct{
	Proof 			[]*big.Int `json:"proof"`
	PublicSignal    []*big.Int `json:"publicSignal"`
}
