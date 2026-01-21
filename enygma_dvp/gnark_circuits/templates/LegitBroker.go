package templates

import(
	"github.com/consensys/gnark/frontend"
	"gnark_server/primitives"
)
// const maxCommissionPercentage = 4
// const commissionPercentageDecimals = 4

type LegitBrokerCircuitConfig struct{

}

type LegitBrokerCircuit struct {
	StBeacon      	     	 	frontend.Variable  `gnark:",public"` 
	StBlindedPublicKey			frontend.Variable  `gnark:",public"` 
	
	WtPrivatekey   				frontend.Variable
}


func (circuit *LegitBrokerCircuit) Define(api frontend.API) error{

	
	publicKey := primitives.PublicKey(api,circuit.WtPrivatekey)

	blinder := primitives.Blinder(api,publicKey)

	api.AssertIsEqual(blinder,circuit.StBlindedPublicKey)

	return nil
}
