package joinSplitERC20_10_2

import(
	"math/big"
)
type JoinSplitERC20_10_2Request struct {
	StMessage         string                     `json:"stMessage" binding:"required"`
	StTreeNumber      [10]string				   `json:"stTreeNumber" binding:"required,len=10"`
	StMerkleRoots     [10]string                  `json:"stMerkleRoots" binding:"required,len=10""`
	StNullifiers      [10]string                  `json:"stNullifiers" binding:"required,len=10"`
	StCommitmentOut   [2]string                  `json:"stCommitmentOut" binding:"required,len=2"`
	WtPrivateKeysIn     [10]string                  `json:"wtPrivateKeysIn" binding:"required,len=10"`
	WtValuesIn        [10]string                  `json:"wtValuesIn" binding:"required,len=10"`
	WtPathElements    [10][8]string     		   `json:"wtPathElements" binding:"required,len=10,dive,len=8"`
	WtPathIndices     [10]string                  `json:"wtPathIndices" binding:"required,len=10"`
	WtErc20ContractAddress string                `json:"wtErc20ContractAddress" binding:"required"`
	WtPublicKeysOut     [2]string                  `json:"wtPublicKeysOut" binding:"required,len=2"`
	WtValuesOut       [2]string                  `json:"wtValuesOut" binding:"required,len=2"`
}

type JoinSplitERC20_10_2Output struct{
	Proof 			[]*big.Int `json:"proof"`
	PublicSignal    []*big.Int `json:"publicSignal"`
}


//Definition already in templates folder

	// Config    				Erc20CircuitConfig
	// Message      			frontend.Variable   `gnark:",public"` 
	// TreeNumber      		[]frontend.Variable  `gnark:",public"`  // nInputsERC20
	// MerkleRoots     		[]frontend.Variable  `gnark:",public"` // nInputsERC20
	// Nullifiers  			[]frontend.Variable  `gnark:",public"` // nInputsERC20
	// CommitmentOut   		[]frontend.Variable  `gnark:",public"` //MOutputs
	// PrivateKeys   			[]frontend.Variable  // nInputsERC20
	// ValuesIn				[]frontend.Variable   // nInputsERC20
	// PathElements    		[][] frontend.Variable // nInputsERC20 //MerkleTreeDepthERC20
	// PathIndices     		[]frontend.Variable // nInputsERC20
	// Erc20ContractAddress    frontend.Variable
	// RecipientPk             []frontend.Variable //MOutputs
	// ValuesOut				[]frontend.Variable //MOutputs