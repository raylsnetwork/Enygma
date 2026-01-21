package templates

import(
	"github.com/consensys/gnark/frontend"
	//"github.com/consensys/gnark/std/math/cmp"
	"gnark_server/primitives"
)

// const MerkleTreeDepthAuctionInit = 8
// const GroupMerkleTreeDepthAuctioInit = 8

type AuctionInitCircuitConfig struct{
	TmNumOfIdParms int
	TmMerkleTreeDepth int
	TmGroupMerkleTreeDepth int
}


type AuctionInitCircuit struct {
	Config 					 AuctionInitCircuitConfig
	StBeacon      			 frontend.Variable  `gnark:",public"` 
	StVaultId      		     frontend.Variable  `gnark:",public"` 
	StAuctionId     		 frontend.Variable  `gnark:",public"` 
	StTreeNumber  			 frontend.Variable	`gnark:",public"` 
	StMerkleRoot   		     frontend.Variable  `gnark:",public"` 
	StNullifier   			 frontend.Variable  `gnark:",public"` 

	StAssetGroupTreeNumber   frontend.Variable  `gnark:",public"` 
	StAssetGroupMerkleRoot   frontend.Variable  `gnark:",public"` 
	
	WtCommiment				 frontend.Variable
	WtPathElements    		 []frontend.Variable       //[MerkleTreeDepthAuctionInit]frontend.Variable
	WtPathIndices    		 frontend.Variable
	WtPrivateKey    		 frontend.Variable
	WtIdParams               []frontend.Variable
	WtContractAddress	     frontend.Variable

	WtAssetGroupPathElements []frontend.Variable    //[GroupMerkleTreeDepthAuctioInit]frontend.Variable
	WtAssetGroupPathIndices  frontend.Variable
}

func (circuit *AuctionInitCircuit) Define(api frontend.API) error{

	id := primitives.UniqueIdMix(api, circuit.StVaultId,circuit.WtContractAddress,circuit.WtIdParams)
	
	publicKey := primitives.PublicKey(api, circuit.WtPrivateKey)
	
	nullifier := primitives.Nullifier(api,circuit.WtPrivateKey, circuit.WtPathIndices)
	api.AssertIsEqual(nullifier, circuit.StNullifier)

	commitment := primitives.CommitmentNative(api, id,publicKey)
	api.AssertIsEqual(commitment, circuit.WtCommiment)

	pathElement := make([]frontend.Variable, circuit.Config.TmMerkleTreeDepth)
	for j:=0 ; j< circuit.Config.TmMerkleTreeDepth;j++{
		pathElement[j] = circuit.WtPathElements[j]
	}
	root := primitives.MerkleProofNative(api, commitment,circuit.WtPathIndices,pathElement)
	api.AssertIsEqual(root, circuit.StMerkleRoot)

	auctionId := primitives.AuctionId(api,circuit.WtCommiment)
	api.AssertIsEqual(auctionId,circuit.StAuctionId)

	
	WtIdParams := make([]frontend.Variable, circuit.Config.TmNumOfIdParms)
	WtIdParams[0] =  frontend.Variable(0)
	for i:=1; i<circuit.Config.TmNumOfIdParms; i++{
		WtIdParams[i] = circuit.WtIdParams[i]
	}
	assertGroupId := primitives.UniqueIdMix(api, circuit.StVaultId,circuit.WtContractAddress,WtIdParams)

	assetGroupPathElement := make([]frontend.Variable, circuit.Config.TmGroupMerkleTreeDepth)
	for j:=0 ; j< circuit.Config.TmGroupMerkleTreeDepth;j++{
		assetGroupPathElement[j] = circuit.WtAssetGroupPathElements[j]
	}
	assetGroupRoot := primitives.MerkleProofNative(api, assertGroupId,circuit.WtPathIndices,assetGroupPathElement)

	isZero := api.IsZero(circuit.StAssetGroupMerkleRoot) // ValueIn[i] ?0 =  1:0
	Enable := api.Mul(1,api.Sub(1,isZero))  
	Diff   := api.Sub(circuit.StAssetGroupMerkleRoot, assetGroupRoot)

	api.AssertIsEqual(api.Mul(Diff, Enable), 0)
	return nil
}

