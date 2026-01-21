package templates

import(
	//"fmt"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/cmp"
	"gnark_server/primitives"
)


// const nInputsAuctionBid = 2
// const mOutputsAuctionBid =2
// const MerkleTreeDepthAuctionBid = 8
// const RangeAuctionBid = "1000000000000000000000000000000000000"
// const GroupMerkleTreeDepthAuctionBid = 8

type AuctionBidCircuitConfig struct {
    TmNInputs    int
    TmMOutputs   int
	TmNumOfIdParams int
    TmDepthMerkle int
    TmRange      frontend.Variable
    TmGroupMerkleTreeDepth int
}


type AuctionBidCircuit struct {
	Config 					 AuctionBidCircuitConfig
	StBeacon				 frontend.Variable 	 `gnark:",public"`
	StAuctionId      	     frontend.Variable   `gnark:",public"`
	StBlindedBid      		 frontend.Variable   `gnark:",public"`
	StVaultId     		     frontend.Variable	 `gnark:",public"`
	StTreeNumber  			 []frontend.Variable `gnark:",public"`
	StMerkleRoot   		     []frontend.Variable `gnark:",public"`
	StNullifier   			 []frontend.Variable `gnark:",public"`
	StCommitmentsOuts		 []frontend.Variable `gnark:",public"`
	StAssetGroupTreeNumber   frontend.Variable   `gnark:",public"`
	StAssetGroupMerkleRoot   frontend.Variable   `gnark:",public"`
	
	WtBidAmount   			 frontend.Variable
	WtBidRandom   			 frontend.Variable

	WtPrivateKeysIn			 []frontend.Variable
	WtPathElements    		 [][]frontend.Variable
	WtPathIndices    		 []frontend.Variable
	WtContractAddress	     frontend.Variable

	WtPublicKeysOut    	 	 []frontend.Variable  //WtRecipientPKOut
	
	WtAssetGroupPathElements []frontend.Variable
	WtAssetGroupPathIndices  frontend.Variable

	WtIdParamsIn 			 [][]frontend.Variable
	WtIdParamsOut		     [][]frontend.Variable		
}

func (circuit *AuctionBidCircuit) Define(api frontend.API) error{
	
	inputsTotal :=frontend.Variable(0)
	outputsTotal:=frontend.Variable(0)

	isValid0 := cmp.IsLess(api,circuit.WtBidAmount, circuit.Config.TmRange)
	
	api.AssertIsEqual(isValid0, 1)

	isValid1 := cmp.IsLessOrEqual(api, 0,circuit.WtBidAmount)
	
	api.AssertIsEqual(isValid1, 1)

	pedersen := primitives.Pedersen(api,circuit.WtBidAmount, circuit.WtBidRandom)
	
	
	api.AssertIsEqual(pedersen,circuit.StBlindedBid)

	WtIdParams2 := make([]frontend.Variable, circuit.Config.TmNumOfIdParams)

	WtIdParams2[0] =  frontend.Variable(0)
	for i:=1; i< circuit.Config.TmNumOfIdParams; i++{
		WtIdParams2[i] = circuit.WtIdParamsIn[0][i]
	}
	
	id := primitives.UniqueIdMix(api, circuit.StVaultId,circuit.WtContractAddress,WtIdParams2)
	
	pathElement := make([]frontend.Variable, circuit.Config.TmDepthMerkle)
	for j:=0 ; j< circuit.Config.TmDepthMerkle;j++{
		pathElement[j] = circuit.WtAssetGroupPathElements[j]
	}
	root := primitives.MerkleProofNative(api, id,circuit.WtAssetGroupPathIndices,pathElement)

	isZero := api.IsZero(circuit.StAssetGroupMerkleRoot) // ValueIn[i] ?0 =  1:0
	Enable := api.Mul(1,api.Sub(1,isZero))  
	Diff   := api.Sub(circuit.StAssetGroupMerkleRoot, root)

	
	
	api.AssertIsEqual(api.Mul(Diff, Enable), 0)
	
	for i:=0; i<circuit.Config.TmNInputs; i++{
		
		isValid0 := cmp.IsLess(api, circuit.WtIdParamsIn[i][0], circuit.Config.TmRange)
		
		api.AssertIsEqual(isValid0, 1)
		
		isValid1 := cmp.IsLess(api, 0,circuit.WtIdParamsIn[i][0] )
		
		api.AssertIsEqual(isValid1, 1)

		
		WtIdParams3 := make([]frontend.Variable, circuit.Config.TmNumOfIdParams)
		for j := 0; j < circuit.Config.TmNumOfIdParams; j++ {  
			WtIdParams3[j] = circuit.WtIdParamsIn[i][j]  
		}
		
		id := primitives.UniqueIdMix(api, circuit.StVaultId,circuit.WtContractAddress,WtIdParams3)
		
		publicKey := primitives.PublicKey(api, circuit.WtPrivateKeysIn[i])

		nullifier := primitives.Nullifier(api,circuit.WtPrivateKeysIn[i], circuit.WtPathIndices[i])
		
		api.AssertIsEqual(nullifier, circuit.StNullifier[i])

		commitment := primitives.Commitment(api, id,publicKey)
		
		wtPathElement := make([]frontend.Variable, circuit.Config.TmGroupMerkleTreeDepth)
		for j:=0 ; j< circuit.Config.TmGroupMerkleTreeDepth;j++{
			wtPathElement[j] = circuit.WtPathElements[i][j]
		}

		root := primitives.MerkleProofNative(api, commitment,circuit.WtPathIndices[i],wtPathElement)
		
		api.AssertIsEqual(root, circuit.StMerkleRoot[i])

		inputsTotal = api.Add( inputsTotal , circuit.WtIdParamsIn[i][0])
	}
	
	api.AssertIsEqual(circuit.WtIdParamsOut[0][0], circuit.WtBidAmount)

	for i:=0 ; i< circuit.Config.TmMOutputs;i++{
		isValid0 := cmp.IsLess(api, circuit.WtIdParamsOut[i][0], circuit.Config.TmRange)
	
		api.AssertIsEqual(isValid0, 1)

		isValid1 := cmp.IsLessOrEqual(api, 0,circuit.WtIdParamsOut[i][0] )
		
		api.AssertIsEqual(isValid1, 1)
		
		WtIdParams4 := make([]frontend.Variable, circuit.Config.TmNumOfIdParams)
		for j:=0; j<circuit.Config.TmNumOfIdParams; j++{
			WtIdParams4[j] = circuit.WtIdParamsOut[i][j]
		}

		id := primitives.UniqueIdMix(api, circuit.StVaultId,circuit.WtContractAddress,WtIdParams4)

		commitment := primitives.CommitmentNative(api, id,circuit.WtPublicKeysOut[i])
		
		

		api.AssertIsEqual(commitment, circuit.StCommitmentsOuts[i])
		
		outputsTotal = api.Add( outputsTotal , circuit.WtIdParamsOut[i][0])

	}
	
	api.AssertIsEqual(outputsTotal,inputsTotal )
	return nil
}

