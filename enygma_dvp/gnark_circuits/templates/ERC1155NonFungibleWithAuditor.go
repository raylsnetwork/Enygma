package templates

import(
	"github.com/consensys/gnark/frontend"
	//"github.com/consensys/gnark/std/math/cmp"
	"gnark_server/primitives"
)


type ERC1155NonFungibleWithAuditorCircuitConfig struct{
	TmNumOfTokens int
	TmMerkleTreeDepth int
	TmAssetGroupMerkleTree int
}

type ERC1155NonFungibleWithAuditorCircuit struct {
	
	Config 					 ERC1155NonFungibleWithAuditorCircuitConfig

	StMessage      			 frontend.Variable   `gnark:",public"` 
	StTreeNumbers      		 []frontend.Variable `gnark:",public"` // numOfTokens
	StMerkleRoots     		 []frontend.Variable `gnark:",public"` // numOfTokens
	StNullifiers  			 []frontend.Variable `gnark:",public"`  // numOfTokens
	StCommitmentOut   		 []frontend.Variable `gnark:",public"`  // numOfTokens
	
	StAuditorPublickey 		[2]frontend.Variable `gnark:",public"`
	StAuditorAuthKey		[2]frontend.Variable `gnark:",public"`
	StAuditorNonce 		     frontend.Variable  `gnark:",public"`
	StAuditorEncryptedValues []frontend.Variable `gnark:",public"`
	WtAuditorRandom		     frontend.Variable  

	StAssetGroupTreeNumber	 []frontend.Variable `gnark:",public"`  // numOfTokens
	StAssetGroupMerkleRoot	 []frontend.Variable `gnark:",public"`  // numOfTokens

	WtPrivateKeysIn		 	 []frontend.Variable //numOfTokens
	WtValues    		 	 []frontend.Variable //numOfTokens
	WtPathElements   		 [][]frontend.Variable //numOfTokens  //MerkleTreeDepthERC1155NonFungible
	WtPathIndices     		 []frontend.Variable //numOfTokens
	WtErc1155TokenIds         []frontend.Variable //numOfTokens
	WtErc1155ContractAddress frontend.Variable

	WtPublicKeysOut	         []frontend.Variable //numOfTokens

	WtAssetGroupPathElements [][]frontend.Variable //numOfTokens  //AssetGroupMerkleTreeDepth
	WtAssetGroupPathIndices  []frontend.Variable //numOfTokens
	
}

func (circuit *ERC1155NonFungibleWithAuditorCircuit) Define(api frontend.API) error{

	config:= ERC1155NonFungibleCircuitConfig{
		TmNumOfTokens: circuit.Config.TmNumOfTokens,
		TmMerkleTreeDepth: circuit.Config.TmMerkleTreeDepth,
		TmAssetGroupMerkleTreeDepth: circuit.Config.TmAssetGroupMerkleTree,
	}

	erc1155NonFungibleCircuit :=ERC1155NonFungibleCircuit{

		Config: config,
		StMessage: circuit.StMessage,
		StTreeNumbers: circuit.StTreeNumbers,
		StMerkleRoots: circuit.StMerkleRoots,	
		StNullifiers : circuit.StNullifiers,
		StCommitmentOut: circuit.StCommitmentOut,
		StAssetGroupMerkleRoot: circuit.StAssetGroupMerkleRoot,
		StAssetGroupTreeNumber: circuit.StAssetGroupTreeNumber,
		
		WtPrivateKeysIn: circuit.WtPrivateKeysIn,
		WtValues : circuit.WtValues,
		WtPathElements : circuit.WtPathElements,
		WtPathIndices: circuit.WtPathIndices,
		WtErc1155ContractAddress: circuit.WtErc1155ContractAddress,
		WtErc1155TokenId: circuit.WtErc1155TokenIds,

		WtPublicKeysOut:circuit.WtPublicKeysOut,

		WtAssetGroupPathElements:circuit.WtAssetGroupPathElements,
		WtAssetGroupPathIndices:circuit.WtAssetGroupPathIndices,

	}
	err := erc1155NonFungibleCircuit.Define(api)
	if err != nil {
		return err
	}	

	plainLength := circuit.Config.TmNumOfTokens*2+1;
	
	auditorCircuit:= primitives.AuditorAccessCircuit{
		TmRealLength: 	   plainLength,                    
		StPublicKey:  	   circuit.StAuditorPublickey,
		StNounce:	  	   circuit.StAuditorNonce,      		
		StEncryptedValues: circuit.StAuditorEncryptedValues,
		WtRandom:		   circuit.WtAuditorRandom,
		WtValues:  make([]frontend.Variable,plainLength),
	}

	for i:=0; i< circuit.Config.TmNumOfTokens; i++{
		auditorCircuit.WtValues[i] = circuit.WtValues[i]
	}

	for i:=0; i< circuit.Config.TmNumOfTokens; i++{
		auditorCircuit.WtValues[i+circuit.Config.TmNumOfTokens] = circuit.WtErc1155TokenIds[i]
	}

	auditorCircuit.WtValues[2*circuit.Config.TmNumOfTokens] = circuit.WtErc1155ContractAddress

	errAuditor := auditorCircuit.Define(api)
	if errAuditor != nil {
		return err
	}

	return nil

}