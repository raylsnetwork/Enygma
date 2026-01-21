package templates

import(
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/cmp"
	"gnark_server/primitives"
)
// const tmNumOfInputs = 2
// const tmMerkleTreeDepth = 8
// const tmGroupMerkleTreeDepth = 8
// const tmRange = "1000000000000000000000000000000000000"
// const tmMaxPermittedCommissionRate = 10
// const tmCommissionRateDecimals = 2

type BrokerageRegistrationConfig struct{

	TmNumOfInputs		  int
	TmMerkleTreeDepth 	  int
	TmGroupMerkleTreeDepth  int
	TmMaxPermittedCommissionRate int
	TmComissionRateDecimals int
	TmRange string

}

type BrokerageRegistrationCircuit struct {
	
	Config 					 BrokerageRegistrationConfig
	StBeacon      	     	 frontend.Variable  `gnark:",public"` 
	StVaultId      			 frontend.Variable  `gnark:",public"`  
	StGroupId	     	 	 frontend.Variable  `gnark:",public"` 
 	StDelegatorTreeNumbers   []frontend.Variable `gnark:",public"`  // Input
	StDelegatorMerkleRoots   []frontend.Variable `gnark:",public"`  // Input
	StDelegatorNullifier   	 []frontend.Variable  `gnark:",public"` // Input
	StBrokerBlindedPublicKey frontend.Variable   `gnark:",public"` 
	StBrokerMinComissionRate frontend.Variable	`gnark:",public"` 
	StBrokerMaxComissionRate frontend.Variable	`gnark:",public"` 
	
	StAssetGroupTreeNumber   frontend.Variable `gnark:",public"` 
	StAssetGroupMerkleRoot   frontend.Variable  `gnark:",public"` 

	WtDelegatorPrivatekeys   []frontend.Variable   // Input
	WtDelegatorPathElements	 [][]frontend.Variable  //// Input //MerkleTreeDepth
	WtDelegatorPathIndices   []frontend.Variable   // input
	WtDelegatorIdParams      [][5]frontend.Variable //input
	WtContractAddress        frontend.Variable
	WtBrokerPublickey        frontend.Variable

	WtAssetGroupPathElements []frontend.Variable   // groupMerkleTreeDepth
	WtAssetGroupPathIndices  frontend.Variable
}


func (circuit *BrokerageRegistrationCircuit) Define(api frontend.API) error{

	blinder := primitives.Blinder(api, circuit.WtBrokerPublickey)
	api.AssertIsEqual(blinder, circuit.StBrokerBlindedPublicKey)

	totalAmount :=frontend.Variable(0)

	for i:=0; i< circuit.Config.TmNumOfInputs; i++{
		isValid0 := cmp.IsLess(api, circuit.WtDelegatorIdParams[i][0],circuit.Config.TmRange)
		api.AssertIsEqual(isValid0, 1)

		isValid1 := cmp.IsLessOrEqual(api, 0,circuit.WtDelegatorIdParams[i][0] )
		api.AssertIsEqual(isValid1, 1)

		
		WtIdParams := make([]frontend.Variable,5)

		for j:=0; j<5; j++{
			WtIdParams[j] = circuit.WtDelegatorIdParams[i][j]
		}
		id := primitives.UniqueIdMix(api, circuit.StVaultId,circuit.WtContractAddress,WtIdParams)

		publicKey := primitives.PublicKey(api, circuit.WtDelegatorPrivatekeys[i])

		commitment := primitives.Commitment(api, id,publicKey)
		
		nullifier := primitives.Nullifier(api,circuit.WtDelegatorPrivatekeys[i], circuit.WtDelegatorPathIndices[i])

		api.AssertIsEqual(nullifier, circuit.StDelegatorNullifier[i])

		delegatorMerklePathElement:= make([]frontend.Variable,circuit.Config.TmMerkleTreeDepth)
		for j:=0 ; j< circuit.Config.TmMerkleTreeDepth;j++{
			delegatorMerklePathElement[j] = circuit.WtDelegatorPathElements[i][j]
		}
		delegatorRoot := primitives.MerkleProof(api, commitment,circuit.WtDelegatorPathIndices[i],delegatorMerklePathElement)

		api.AssertIsEqual(delegatorRoot, circuit.StDelegatorMerkleRoots[i])

		totalAmount = api.Add( totalAmount , circuit.WtDelegatorIdParams[i][0])

	}


	WtIdParams2 := make([]frontend.Variable,5)
	WtIdParams2[0] =  frontend.Variable(0)
	for i:=1; i<5; i++{
		WtIdParams2[i] = circuit.WtDelegatorIdParams[0][i]
	}
	id := primitives.UniqueIdMix(api, circuit.StVaultId,circuit.WtContractAddress,WtIdParams2)


	assetGroupPathElement := make([]frontend.Variable,circuit.Config.TmGroupMerkleTreeDepth)
	for j:=0 ; j< circuit.Config.TmGroupMerkleTreeDepth;j++{
		assetGroupPathElement[j] = circuit.WtAssetGroupPathElements[j]
	}
	delegatorRoot := primitives.MerkleProof(api, id,circuit.WtAssetGroupPathIndices,assetGroupPathElement)

	isZero := api.IsZero(circuit.StAssetGroupMerkleRoot) // ValueIn[i] ?0 =  1:0
	Enable := api.Mul(1,api.Sub(1,isZero))  
	Diff   := api.Sub(circuit.StAssetGroupMerkleRoot, delegatorRoot)

	api.AssertIsEqual(api.Mul(Diff, Enable), 0)

	isValid2 := cmp.IsLessOrEqual(api, 0, circuit.StBrokerMinComissionRate)
	api.AssertIsEqual(isValid2, 1)

	isValid3 := cmp.IsLessOrEqual(api, 0, circuit.StBrokerMaxComissionRate)
	api.AssertIsEqual(isValid3, 1)

	isValid4 := cmp.IsLessOrEqual(api, circuit.StBrokerMinComissionRate,circuit.StBrokerMaxComissionRate )
	api.AssertIsEqual(isValid4, 1)

	power:=frontend.Variable(1)
	for z:=0; z<circuit.Config.TmComissionRateDecimals;z++{
		power =  api.Mul(power,10)
	}
	
	scaledMax:=api.Div(circuit.StBrokerMaxComissionRate,power)
	isLess:= cmp.IsLess(api,circuit.StBrokerMaxComissionRate,power)

	scaledMax =  api.Mul(api.Sub(1,isLess),scaledMax)

	isValid5 := cmp.IsLessOrEqual(api,scaledMax,circuit.Config.TmMaxPermittedCommissionRate )

	api.AssertIsEqual(isValid5, 1)

	return nil

}
