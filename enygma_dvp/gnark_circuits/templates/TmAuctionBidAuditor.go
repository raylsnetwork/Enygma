package templates

import(
	//"fmt"
	"github.com/consensys/gnark/frontend"
	//"github.com/consensys/gnark/std/math/cmp"
	"gnark_server/primitives"
)


type TmAuctionBidAuditorCircuitConfig struct {
    TmInputs    		 int
    TmOutputs   		 int
    TmNumOfIdParms 		 int
    TmMerkleTreeDepth    int
	TmRange      		frontend.Variable
    TmAssetGroupMerkleTreeDepth int
}


type TmAuctionBidAuditorCircuit struct {
	Config 					 TmAuctionBidAuditorCircuitConfig
	StBeacon     	 	     frontend.Variable  `gnark:",public"` 
	StAuctionId      		 frontend.Variable  `gnark:",public"`  
	StBlindedBid     		 frontend.Variable	 `gnark:",public"` 
	StVaultId  			 	 frontend.Variable   `gnark:",public"` 
	StTreeNumbers   		 []frontend.Variable `gnark:",public"` 
	StMerkleRoots   		 []frontend.Variable `gnark:",public"` 
	StNullifiers		 	 []frontend.Variable `gnark:",public"` 
	StCommitmentsOuts	     []frontend.Variable `gnark:",public"` 
	StAssetGroupTreeNumber 	 frontend.Variable 	 `gnark:",public"` 
	StAssetGrupoMerkleRoot 	 frontend.Variable	  `gnark:",public"` 


	StAuctioneerPublicKey    [2]frontend.Variable `gnark:",public"` 
	StAuctioneerAuthKey		 [2]frontend.Variable `gnark:",public"` 
	StAuctioneerNonce 		 	frontend.Variable 	  `gnark:",public"`
	StAuctioneeEncryptedValues []frontend.Variable  `gnark:",public"` 
	WtAuctioneerRandom 		   frontend.Variable 	

	StAuditorPublicKey       [2]frontend.Variable  `gnark:",public"` 
	StAuditorAuthKey		 [2]frontend.Variable  `gnark:",public"` 
	StAuditorNonce			 frontend.Variable     `gnark:",public"` 
	StAuditorEncryptedValues []frontend.Variable    `gnark:",public"` 
	WtAuditoRandom			 frontend.Variable

	WtBidAmount 			 frontend.Variable
	WtBidRandom				 frontend.Variable

	WtPrivateKeysIn			 []frontend.Variable
	WtPathElements			 [][]frontend.Variable
	WtPathIndices			 []frontend.Variable
	WtContractAddress		 frontend.Variable

	WtPublicKeysOut			 []frontend.Variable

	WtAssetGroupPathElements []frontend.Variable
	WtAssetGroupPathIndices   frontend.Variable

	WtIdParamsIn            [][]frontend.Variable
	WtIdParamsOut            [][]frontend.Variable

}

func (circuit *TmAuctionBidAuditorCircuit) Define(api frontend.API) error{


	var plainLength = circuit.Config.TmNumOfIdParms* (circuit.Config.TmInputs+ circuit.Config.TmOutputs) +4;
	
	
	auctionConfig := AuctionBidCircuitConfig{
		TmNInputs   : circuit.Config.TmInputs,
		TmMOutputs  : circuit.Config.TmOutputs,
		TmNumOfIdParams: circuit.Config.TmNumOfIdParms,
		TmDepthMerkle: circuit.Config.TmMerkleTreeDepth,
		TmRange: circuit.Config.TmRange,
		TmGroupMerkleTreeDepth: circuit.Config.TmAssetGroupMerkleTreeDepth,
		
	}

	auctionBidCircuit := AuctionBidCircuit{
		Config: 		auctionConfig,
		StBeacon: 		circuit.StBeacon, 					 
		StAuctionId: 	circuit.StAuctionId,
		StBlindedBid:   circuit.StBlindedBid,   	 
		StVaultId: 	    circuit.StVaultId, 		     
		StTreeNumber: 	circuit.StTreeNumbers,			 
		StMerkleRoot:	circuit.StMerkleRoots,	   
		StNullifier: 	circuit.StNullifiers,   			
		StCommitmentsOuts: circuit.StCommitmentsOuts,		
		StAssetGroupTreeNumber: circuit.StAssetGroupTreeNumber,
		StAssetGroupMerkleRoot: circuit.StAssetGrupoMerkleRoot,
		
		WtBidAmount: circuit.WtBidAmount,   			
		WtBidRandom: circuit.WtBidRandom,   			 

		WtPrivateKeysIn: circuit.WtPrivateKeysIn,
		WtPathElements: circuit.WtPathElements,
		WtPathIndices : circuit.WtPathIndices,
		WtContractAddress: circuit.WtContractAddress,

		WtPublicKeysOut:circuit.WtPublicKeysOut,
		
		
		WtAssetGroupPathElements: circuit.WtAssetGroupPathElements,
		WtAssetGroupPathIndices: circuit.WtAssetGroupPathIndices,

		WtIdParamsIn:circuit.WtIdParamsIn,
		WtIdParamsOut:circuit.WtIdParamsOut,
	}

	err := auctionBidCircuit.Define(api)
	if err != nil {
		return err
	}



	auctionnerCircuit :=primitives.AuditorAccessCircuit{
		TmRealLength:		3,
		StPublicKey: 		circuit.StAuctioneerPublicKey,
		StNounce: 			circuit.StAuctioneerNonce,
		StEncryptedValues:  circuit.StAuctioneeEncryptedValues,
		WtRandom:  			circuit.WtAuctioneerRandom,
		WtValues:  			make([]frontend.Variable,3),
	}

	auctionnerCircuit.WtValues[0] = circuit.WtBidAmount
	auctionnerCircuit.WtValues[1] = circuit.WtBidRandom
	auctionnerCircuit.WtValues[2] = frontend.Variable(0)

	auditorAccess := primitives.AuditorAccessCircuit{
		TmRealLength:		plainLength,
		StPublicKey: 		circuit.StAuditorPublicKey,
		StNounce: 			circuit.StAuditorNonce,
		StEncryptedValues:  circuit.StAuditorEncryptedValues,
		WtRandom:  			circuit.WtAuditoRandom,
		WtValues:  			make([]frontend.Variable,plainLength),
	}

	for i:=0; i< circuit.Config.TmInputs; i++{
		for j:=0; j< circuit.Config.TmNumOfIdParms; j++{
			auditorAccess.WtValues[j+(i*circuit.Config.TmNumOfIdParms)] = circuit.WtIdParamsIn[i][j]}
	}

	for i:=0; i< circuit.Config.TmOutputs; i++{
		for j:=0; j< circuit.Config.TmNumOfIdParms; j++{
			auditorAccess.WtValues[j+(i*circuit.Config.TmNumOfIdParms)+(circuit.Config.TmInputs *circuit.Config.TmNumOfIdParms)] = circuit.WtIdParamsOut[i][j]}
	}

	auditorAccess.WtValues[circuit.Config.TmNumOfIdParms*(circuit.Config.TmInputs +circuit.Config.TmOutputs)] = circuit.WtContractAddress
	auditorAccess.WtValues[circuit.Config.TmNumOfIdParms*(circuit.Config.TmInputs +circuit.Config.TmOutputs)+1] = circuit.WtBidAmount
	auditorAccess.WtValues[circuit.Config.TmNumOfIdParms*(circuit.Config.TmInputs +circuit.Config.TmOutputs)+2] = circuit.WtBidRandom
	auditorAccess.WtValues[circuit.Config.TmNumOfIdParms*(circuit.Config.TmInputs +circuit.Config.TmOutputs)+3] = 0

	errAuditor := auditorAccess.Define(api)
	if errAuditor != nil {
		return errAuditor
	}

	return nil
}	