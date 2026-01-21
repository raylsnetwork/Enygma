package templates

import(
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/cmp"
	"gnark_server/primitives"
)

// const nInputsERC20WithBroker = 8
// const mOutputsERC20WithBroker = 8
// const MerkleTreeDepthERC20WithBroker=8
// const RangeERC20WithBroker=100
// const maxCommissionPercentageERC20WithBroker = 5
// const commissionPercentageDecimalsERC20WithBroker=5

type Erc20BrokerConfig struct{
	TmNInputs  					  int
	TmMOutputs	 				  int
	TmMerkleTree 				  int
	TmRange 					  frontend.Variable
	TmMaxComissionPercentage 	  int
	TmComissionPercentageDecimals int
	
}

type Erc20WithBrokerCircuit struct {
	Config   					Erc20BrokerConfig
	StMessage      			    frontend.Variable 
	StTreeNumber      		    []frontend.Variable  // Inputs
	StMerkleRoots     		    []frontend.Variable  // Inputs
	StNullifiers  			    []frontend.Variable  // Inputs
	StCommitmentOut   		    []frontend.Variable  // MOutputs
	StBrokerBlindedPublicKey    frontend.Variable  // Inputs
	StBrokerCommisionPercentage frontend.Variable


	WtPrivateKeys   			[]frontend.Variable  // Input
	WtValuesIn					[]frontend.Variable // Input
	WtPathElements    			[][]frontend.Variable // Input //MerkleTree
	WtPathIndices     			[]frontend.Variable // Input
	WtErc20ContractAddress    	frontend.Variable

	WtPublicKeyOut              []frontend.Variable //Moutputs
	WtValuesOut					[]frontend.Variable  //Moutputs
}


func (circuit *Erc20WithBrokerCircuit) Define(api frontend.API) error{

	inputsTotals :=frontend.Variable(0)
	outputsTotal:=frontend.Variable(0)

	api.AssertIsEqual(frontend.Variable(circuit.Config.TmMOutputs),3)

	blinder := primitives.Blinder(api,circuit.WtPublicKeyOut[2])
	api.AssertIsEqual(blinder,circuit.StBrokerBlindedPublicKey)

	isValid0 := cmp.IsLess(api, circuit.WtValuesOut[2], circuit.WtValuesOut[0])
	api.AssertIsEqual(isValid0, 1)
	
	for i:=0; i<circuit.Config.TmNInputs ; i++{
		isValid1 := cmp.IsLess(api, circuit.WtValuesIn[i], circuit.Config.TmRange)
		api.AssertIsEqual(isValid1, 1)

		isValid2 := cmp.IsLess(api, 0, circuit.WtValuesIn[i])
		api.AssertIsEqual(isValid2, 1)

		id := primitives.UniqueId(api, circuit.WtErc20ContractAddress, circuit.WtValuesIn[i])

		publicKey := primitives.PublicKey(api, circuit.WtPrivateKeys[i])

		nullifier := primitives.Nullifier(api, publicKey, circuit.WtPathIndices[i])

		api.AssertIsEqual(nullifier, circuit.StNullifiers[i])

		commitment := primitives.Commitment(api, id, publicKey)

		pathElement := make([]frontend.Variable,circuit.Config.TmMerkleTree)
		for j:=0 ; j< circuit.Config.TmMerkleTree;j++{
			pathElement[j] = circuit.WtPathElements[i][j]
		}
		root := primitives.MerkleProof(api, commitment,circuit.WtPathIndices[i],pathElement)

		isZero := api.IsZero(circuit.WtValuesIn[i]) // ValueIn[i] ?0 =  1:0
		Enable := api.Mul(1,isZero)  
		Diff   := api.Sub(circuit.StMerkleRoots[i], root)

		api.AssertIsEqual(api.Mul(Diff, Enable), 0)

		inputsTotals = api.Add( inputsTotals , circuit.WtValuesIn[i])
	}
	isValid1 := cmp.IsLessOrEqual(api, circuit.StBrokerCommisionPercentage, circuit.Config.TmMaxComissionPercentage)
	api.AssertIsEqual(isValid1, 1)

	api.AssertIsEqual(circuit.WtValuesOut[2], api.Mul(circuit.WtValuesOut[2], circuit.StBrokerCommisionPercentage))


	for i:=0 ; i< circuit.Config.TmMOutputs; i++{
		isValid0 := cmp.IsLessOrEqual(api, 0, circuit.WtValuesOut[i])
		api.AssertIsEqual(isValid0, 1)

		isValid1 := cmp.IsLess(api, circuit.WtValuesOut[i], circuit.Config.TmRange)
		api.AssertIsEqual(isValid1, 1)

		uniqueId := primitives.UniqueId(api, circuit.WtErc20ContractAddress, circuit.WtValuesOut[i])
		commitment := primitives.Commitment(api,uniqueId, circuit.WtPublicKeyOut[i])

		api.AssertIsEqual(commitment, circuit.StCommitmentOut[i])	

	
		outputsTotal = api.Add( outputsTotal , circuit.WtValuesOut[i])

	}

	api.AssertIsEqual(inputsTotals,outputsTotal)

	return nil
}
