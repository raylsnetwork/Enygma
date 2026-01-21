package templates

import(
	"github.com/consensys/gnark/frontend"
	"gnark_server/primitives"
)

// const nInputsERC721 = 1
// const mOutputsERC721 = 1
// const MerkleTreeDepthERC721=8

type Erc721WithAuditorCircuitConfig struct{
	TmNumOfTokens int
	TmMerkleTreeDepth int
}

type Erc721WithAuditorCircuit struct {

	Config   				Erc721WithAuditorCircuitConfig
	StMessage      			frontend.Variable    `gnark:",public"` 
	StTreeNumbers      		[]frontend.Variable    `gnark:",public"` 
	StMerkleRoots     		[]frontend.Variable    `gnark:",public"` 
	StNullifiers  			[]frontend.Variable  `gnark:",public"`  //NInputs
	StCommitmentOut   		[]frontend.Variable  `gnark:",public"`  //MOutputs
	
	StAuditorPublicKeys		 [2]frontend.Variable	`gnark:",public"` 
	StAuditorAuthKey		 [2]frontend.Variable	`gnark:",public"` 
	StAuditorNonce 			 frontend.Variable		`gnark:",public"` 
	StAuditorEncryptedValues []frontend.Variable	`gnark:",public"` 
	WtAuditorRandom			 frontend.Variable

	WtPrivateKeysIn   		[]frontend.Variable //NInputs
	WtValues				[]frontend.Variable //NInputs
	WtPathElements    		[][] frontend.Variable //NInputs //MerkleTreeDepth 
	WtPathIndices     		[]frontend.Variable //NInputs
	WtPrivateKeysOut        []frontend.Variable //MOutputs

}


func (circuit *Erc721WithAuditorCircuit) Define(api frontend.API) error{
	
	erc721Config:= Erc721CircuitConfig{
			TmNumOfTokens: circuit.Config.TmNumOfTokens,
			TmMerkleTreeDepth: circuit.Config.TmMerkleTreeDepth,
		}
	erc721Circuit:= Erc721Circuit{

		Config: erc721Config,
		StMessage: circuit.StMessage,
		StTreeNumbers: circuit.StTreeNumbers,    		
		StMerkleRoots: circuit.StMerkleRoots,
		StNullifiers: circuit.StNullifiers,
		StCommitmentOut: circuit.StCommitmentOut,
		
		WtPrivateKeysIn: circuit.WtPrivateKeysIn, 
		WtValues: circuit.WtValues,			
		WtPathElements: circuit.WtPathElements,
		WtPathIndices:circuit.WtPathIndices,
		WtPublicKeysOut:circuit.WtPrivateKeysOut,
	}

	err := erc721Circuit.Define(api)
	if err != nil {
		return err
	}	

	auditorAccessCircuit:=primitives.AuditorAccessCircuit{
		TmRealLength: circuit.Config.TmNumOfTokens,
		StPublicKey: circuit.StAuditorPublicKeys,
		StNounce: circuit.StAuditorNonce,
		StEncryptedValues: circuit.StAuditorEncryptedValues,
		WtRandom: circuit.WtAuditorRandom,
		WtValues: circuit.WtValues,
	}

	errAuditor := auditorAccessCircuit.Define(api)
	if errAuditor != nil {
		return errAuditor
	}	

	return nil
}
