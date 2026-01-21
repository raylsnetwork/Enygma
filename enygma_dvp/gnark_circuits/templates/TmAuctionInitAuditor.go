package templates

import(
	//"fmt"
	"github.com/consensys/gnark/frontend"
	//"github.com/consensys/gnark/std/math/cmp"
	"gnark_server/primitives"
)


type TmAuctionInitAuditorCircuitConfig struct {
    TmNumOfIdParms 		 int
    TmMerkleTreeDepth    int
    TmAssetGroupMerkleTreeDepth int
}


type TmAuctionInitAuditorCircuit struct {
	Config 					 TmAuctionInitAuditorCircuitConfig
	StBeacon     	 	     frontend.Variable   `gnark:",public"` 
	StAuctionId      		 frontend.Variable   `gnark:",public"` 
	StVaultId  			 	 frontend.Variable   `gnark:",public"` 
	StTreeNumber    		 frontend.Variable   `gnark:",public"` 
	StMerkleRoot    		 frontend.Variable	 `gnark:",public"` 
	StNullifier			 	 frontend.Variable   `gnark:",public"` 

	StAuditorPublicKey       [2]frontend.Variable   `gnark:",public"` 
	StAuditorAuthKey		 [2]frontend.Variable	`gnark:",public"` 
	StAuditorNonce			 frontend.Variable 		`gnark:",public"` 
	StAuditorEncryptedValues []frontend.Variable	`gnark:",public"` 
	WtAuditorRandom			 frontend.Variable	

	StAssetGroupTreeNumber   frontend.Variable  `gnark:",public"` 
	StAssetGroupMerkleRoot   frontend.Variable  `gnark:",public"` 

	WtCommitment 			 frontend.Variable
	WtPathElements  		 []frontend.Variable
	WtPathIndices            frontend.Variable
	WtPrivateKey			 frontend.Variable
	WtIdParams               []frontend.Variable
	WtContractAddress        frontend.Variable

	WtAssetGroupPathElements []frontend.Variable
	WtAssetGroupPathIndices  frontend.Variable

}

func (circuit *TmAuctionInitAuditorCircuit) Define(api frontend.API) error{

	
	auctionInitConfig := AuctionInitCircuitConfig{
		TmNumOfIdParms : circuit.Config.TmNumOfIdParms,
		TmMerkleTreeDepth   : circuit.Config.TmMerkleTreeDepth,
		TmGroupMerkleTreeDepth  : circuit.Config.TmAssetGroupMerkleTreeDepth,
	}


	auctionInitCircuit:=AuctionInitCircuit{
		Config:			auctionInitConfig,
		StBeacon:		circuit.StBeacon,
		StVaultId:		circuit.StVaultId,
		StAuctionId:	circuit.StAuctionId,
		StTreeNumber:   circuit.StTreeNumber, 			
		StMerkleRoot:    circuit.StMerkleRoot,
		StNullifier:     circuit.StNullifier,   		

		StAssetGroupTreeNumber:  circuit.StAssetGroupTreeNumber,
		StAssetGroupMerkleRoot: circuit.StAssetGroupMerkleRoot,

		WtCommiment: 			circuit.WtCommitment,	 
		WtPathElements:    		 circuit.WtPathElements,     //[MerkleTreeDepthAuctionInit]frontend.Variable
		WtPathIndices:			 circuit.WtPathIndices,
		WtPrivateKey:			 circuit.WtPrivateKey,
		WtIdParams:			     circuit.WtIdParams,
		WtContractAddress:	     circuit.WtContractAddress,

		WtAssetGroupPathElements: circuit.WtAssetGroupPathElements, //[GroupMerkleTreeDepthAuctioInit]frontend.Variable
		WtAssetGroupPathIndices: circuit.WtAssetGroupPathIndices,
	}

	err := auctionInitCircuit.Define(api)
	if err != nil {
		return err
	}

	var plainLength = circuit.Config.TmNumOfIdParms+1;
	
	auditorAccess := primitives.AuditorAccessCircuit{
		TmRealLength:plainLength,
		StPublicKey: circuit.StAuditorPublicKey,
		StNounce: circuit.StAuditorNonce,
		StEncryptedValues:  circuit.StAuditorEncryptedValues,
		WtRandom:  		circuit.WtAuditorRandom,
		WtValues:  make([]frontend.Variable,plainLength),
	}

	

	for j:=0; j< circuit.Config.TmNumOfIdParms; j++{
		auditorAccess.WtValues[j] = circuit.WtIdParams[j]
	}
	auditorAccess.WtValues[circuit.Config.TmNumOfIdParms] = circuit.WtContractAddress


	errAuditor := auditorAccess.Define(api)
	if errAuditor != nil {
		return errAuditor
	}

	return nil

}