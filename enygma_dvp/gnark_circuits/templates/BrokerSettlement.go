package templates

import(
	"github.com/consensys/gnark/frontend"
)
const maxCommissionPercentage = 4
const commissionPercentageDecimals = 4


type BrokerageSettlementConfig struct{
	MaxPermittedCommissionRate int
	ComissionRateDecimals int
}

type BrokerSettlementCircuit struct {
	StBeacon      	     	 	frontend.Variable 
	StVaultId      			 	frontend.Variable   
	StBrokerBlindedPublicKey	frontend.Variable
	
	WtContractAddress   		frontend.Variable
	WtBrokerPrivateKey   		frontend.Variable
	WtBrokerIdParams   			[5]frontend.Variable

	WtBrokerPublickey   		frontend.Variable
}


func (circuit *BrokerSettlementCircuit) Define(api frontend.API) error{

	// isValid0 := cmp.IsLess(api, circuit.WtDelegatorIdParams[0],circuit.Config.MaxPermittedCommissionRate)
	// api.AssertIsEqual(isValid0, 1)

	// isValid1 := cmp.IsLessOrEqual(api, 0,circuit.WtDelegatorIdParams[0] )
	// api.AssertIsEqual(isValid1, 1)

	// WtIdParams := make([]frontend.Variable,5)

	// for j:=0; j<5; j++{
	// 	WtIdParams[j] = circuit.WtDelegatorIdParams[i][j]
	// }
	// id := primitives.UniqueIdMix(api, circuit.StVaultId,circuit.WtContractAddress,WtIdParams)

	return nil

}
