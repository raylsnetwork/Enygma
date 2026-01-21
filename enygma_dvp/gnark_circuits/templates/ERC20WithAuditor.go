package templates

import(
	"github.com/consensys/gnark/frontend"
	//"github.com/consensys/gnark/std/math/cmp"
	"gnark_server/primitives"
)

type Erc20WithAuditorConfig struct {
	TmNInputs int
	TmMOutputs  int
	TmMerkleTreeDepth int
	TmRange frontend.Variable 	
}

type Erc20WithAuditorCircuit struct {

	Config    				Erc20WithAuditorConfig
	StMessage      			frontend.Variable   `gnark:",public"` 
	StTreeNumber      		[]frontend.Variable  `gnark:",public"`  // nInputsERC20
	StMerkleRoots     		[]frontend.Variable  `gnark:",public"` // nInputsERC20
	StNullifiers  			[]frontend.Variable  `gnark:",public"` // nInputsERC20
	StCommitmentOut   		[]frontend.Variable  `gnark:",public"` //MOutputs
	
	WtPrivateKeysIn   		[]frontend.Variable  // nInputsERC20
	WtValuesIn				[]frontend.Variable   // nInputsERC20
	WtPathElements    		[][] frontend.Variable // nInputsERC20 //MerkleTreeDepthERC20
	WtPathIndices     		[]frontend.Variable // nInputsERC20
	WtErc20ContractAddress   frontend.Variable
	
	WtPublicKeysOut         []frontend.Variable //MOutputs
	WtValuesOut				[]frontend.Variable //MOutputs

	StAuditorPublickey 		 [2]frontend.Variable `gnark:",public"` 
	StAuditorAuthKey		 [2]frontend.Variable `gnark:",public"` 
	StAuditorNonce 		     frontend.Variable	  `gnark:",public"` 
	StAuditorEncryptedValues []frontend.Variable  `gnark:",public"` 
	StAuditorRandom		     frontend.Variable
}


func (circuit *Erc20WithAuditorCircuit) Define(api frontend.API) error{

	erc20Config := Erc20CircuitConfig{
		TmNInputs: circuit.Config.TmNInputs,
		TmMOutputs: circuit.Config.TmMOutputs,
		TmMerkleTreeDepth: circuit.Config.TmMerkleTreeDepth,
		TmRange: circuit.Config.TmRange,
	}

	erc20Circuit:= Erc20Circuit{
		Config:   				erc20Config,
		StMessage:			    circuit.StMessage,	
		StTreeNumber:      		circuit.StTreeNumber,
		StMerkleRoots:			circuit.StMerkleRoots,     	
		StNullifiers:			circuit.StNullifiers,  			
		StCommitmentOut:	    circuit.StCommitmentOut,   		
		WtPrivateKeysIn:			circuit.WtPrivateKeysIn,	   		
		WtValuesIn:				circuit.WtValuesIn,				
		WtPathElements:			circuit.WtPathElements,    		
		WtPathIndices:			circuit.WtPathIndices,     		
		WtErc20ContractAddress:   circuit.WtErc20ContractAddress,
		WtPublicKeysOut:			circuit.WtPublicKeysOut,             
		WtValuesOut:				circuit.WtValuesOut,				
	}

	err := erc20Circuit.Define(api)
	if err != nil {
		return err
	}
	plainLength := circuit.Config.TmNInputs +circuit.Config.TmMOutputs+1
	
	auditorCircuit:= primitives.AuditorAccessCircuit{
		TmRealLength: plainLength,                    
		StPublicKey:  circuit.StAuditorPublickey,
		StNounce:	  circuit.StAuditorNonce,      		
		StEncryptedValues: circuit.StAuditorEncryptedValues,
		WtRandom:		   circuit.StAuditorRandom,
		WtValues:  make([]frontend.Variable,plainLength),
		
	}

	for i:=0; i< circuit.Config.TmNInputs; i++{
			auditorCircuit.WtValues[i] = circuit.WtValuesIn[i]
	}

	for i:=0; i< circuit.Config.TmMOutputs; i++{
			auditorCircuit.WtValues[i+circuit.Config.TmNInputs] = circuit.WtValuesOut[i]
	}
	auditorCircuit.WtValues[circuit.Config.TmNInputs+circuit.Config.TmMOutputs] = circuit.WtErc20ContractAddress;


	errAuditor := auditorCircuit.Define(api)
	if errAuditor != nil {
		return errAuditor
	}

	return nil

}