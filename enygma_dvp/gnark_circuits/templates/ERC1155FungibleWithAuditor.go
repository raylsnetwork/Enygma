package templates

import(
	"github.com/consensys/gnark/frontend"
	
	"gnark_server/primitives"
)


type ERC1155FungibleWithAuditorCircuitConfig struct{
	TmNInputs int
	TmMOutputs int
	TmMerkleTreeDepth int
	TmAssetGroupMerkleTree int
	TmRange frontend.Variable
}


type Erc1155FungibleWithAuditorCircuit struct {

	Config   				 ERC1155FungibleWithAuditorCircuitConfig
	StMessage      			 frontend.Variable  	`gnark:",public"` 
	StTreeNumbers      		 []frontend.Variable   `gnark:",public"` 
	StMerkleRoots     		 []frontend.Variable   `gnark:",public"` 
	StNullifiers  			 []frontend.Variable   `gnark:",public"` 
	StCommitmentOut   		 []frontend.Variable   `gnark:",public"` 

	StAuditorPublickey 		 [2]frontend.Variable	`gnark:",public"` 
	StAuditorAuthKey		 [2]frontend.Variable	`gnark:",public"` 
	StAuditorNonce 		     frontend.Variable		`gnark:",public"` 
	StAuditorEncryptedValues []frontend.Variable	`gnark:",public"` 
	WtAuditorRandom		     frontend.Variable

	StAssetGroupMerkleRoot   frontend.Variable		`gnark:",public"` 
	StAssetGroupTreeNumber   frontend.Variable		`gnark:",public"` 
	
	WtPrivateKeysIn		     []frontend.Variable //NInputs
	WtValuesIn    		 	 []frontend.Variable //NInputs
	WtPathElements   		 [][]frontend.Variable //NInputs//MerkleTreeDepth
	WtPathIndices     		 []frontend.Variable //NInputs
	WtErc1155ContractAddress frontend.Variable
	WtErc1155TokenId         frontend.Variable

	WtPublicKeysOut			 []frontend.Variable //MOutputs
	WtValuesOut              []frontend.Variable //MOutputs

	WtAssetGroupPathElements []frontend.Variable //AssetGroupMerkleTree
	WtAssetGroupPathIndices  frontend.Variable
	
}

func (circuit *Erc1155FungibleWithAuditorCircuit) Define(api frontend.API) error{

	erc1155FungibleConfig:= ERC1155FungibleCircuitConfig{
		
		TmNInputs: circuit.Config.TmNInputs,
		TmMOutputs: circuit.Config.TmMOutputs,
		TmMerkleTreeDepth: circuit.Config.TmMerkleTreeDepth,
		TmAssetGroupMerkleTree: circuit.Config.TmAssetGroupMerkleTree,
		TmRange: circuit.Config.TmRange,
	}
	
	erc1155FungibleCircuit:= Erc1155FungibleCircuit{

		Config: erc1155FungibleConfig,
		StMessage: circuit.StMessage,
		StTreeNumbers: circuit.StTreeNumbers,
		StMerkleRoots: circuit.StMerkleRoots,	
		StNullifiers : circuit.StNullifiers,
		StCommitmentOut: circuit.StCommitmentOut,
		StAssetGroupMerkleRoot: circuit.StAssetGroupMerkleRoot,
		StAssetGroupTreeNumber: circuit.StAssetGroupTreeNumber,
		
		WtPrivateKeysIn: circuit.WtPrivateKeysIn,
		WtValuesIn : circuit.WtValuesIn,
		WtPathElements : circuit.WtPathElements,
		WtPathIndices: circuit.WtPathIndices,
		WtErc1155ContractAddress: circuit.WtErc1155ContractAddress,
		WtErc1155TokenId: circuit.WtErc1155TokenId,

		WtPublicKeysOut:circuit.WtPublicKeysOut,
		WtValuesOut: circuit.WtValuesOut,

		WtAssetGroupPathElements:circuit.WtAssetGroupPathElements,
		WtAssetGroupPathIndices:circuit.WtAssetGroupPathIndices,
	}

	err := erc1155FungibleCircuit.Define(api)
	if err != nil {
		return err
	}	

	plainLength := circuit.Config.TmNInputs +circuit.Config.TmMOutputs+2;
	
	auditorCircuit:= primitives.AuditorAccessCircuit{
		TmRealLength: plainLength,
		StPublicKey:  circuit.StAuditorPublickey,
		StNounce:	  circuit.StAuditorNonce,      		
		StEncryptedValues: circuit.StAuditorEncryptedValues,
		WtRandom:		   circuit.WtAuditorRandom,
		WtValues:  make([]frontend.Variable,plainLength),
	}

	for i:=0; i< circuit.Config.TmNInputs; i++{
			auditorCircuit.WtValues[i] = circuit.WtValuesIn[i]
	}

	for i:=0; i< circuit.Config.TmMOutputs; i++{
			auditorCircuit.WtValues[i+circuit.Config.TmNInputs] = circuit.WtValuesOut[i]
	}

	auditorCircuit.WtValues[circuit.Config.TmNInputs+circuit.Config.TmMOutputs] = circuit.WtErc1155TokenId
	auditorCircuit.WtValues[circuit.Config.TmNInputs+circuit.Config.TmMOutputs+1] = circuit.WtErc1155ContractAddress

	errAuditor := auditorCircuit.Define(api)
	if errAuditor != nil {
		return err
	}
	
	return nil
}