package paymentFee

import "math/big"

// PaymentFeeRequest is the JSON body for POST /proof/paymentFee.
//
// Circuit: 1 input / 2 outputs / depth 8.
//
// Conservation law: Σ(valuesIn) == valOut[0] + valOut[1] + StFee
// (fee is absorbed into the sender's input, not a separate output note)
//
//   - Output[0]: Bob's payment  — any pk_spend
//   - Output[1]: Alice's change — must equal senderPk (enforced in-circuit)
//
// Public signal layout (8 elements):
//
//	[StMessage, StTreeNumbers[0], StMerkleRoots[0], StNullifiers[0],
//	 StCommitmentsOut[0], StCommitmentsOut[1], StContractAddress, StFee]
type PaymentFeeRequest struct {
	StMessage         string    `json:"stMessage"         binding:"required"`
	StTreeNumbers     [1]string `json:"stTreeNumbers"     binding:"required"`
	StMerkleRoots     [1]string `json:"stMerkleRoots"     binding:"required"`
	StNullifiers      [1]string `json:"stNullifiers"      binding:"required"`
	StCommitmentsOut  [2]string `json:"stCommitmentsOut"  binding:"required"`
	StContractAddress string    `json:"stContractAddress" binding:"required"`
	StFee             string    `json:"stFee"             binding:"required"`

	WtPrivateKeysIn      [1]string    `json:"wtPrivateKeysIn"      binding:"required"`
	WtValuesIn           [1]string    `json:"wtValuesIn"           binding:"required"`
	WtSaltsIn            [1]string    `json:"wtSaltsIn"            binding:"required"`
	WtPathElements       [1][8]string `json:"wtPathElements"       binding:"required"`
	WtPathIndices        [1]string    `json:"wtPathIndices"        binding:"required"`
	WtTokenId            string       `json:"wtTokenId"            binding:"required"`
	WtSpendPublicKeysOut [2]string    `json:"wtSpendPublicKeysOut" binding:"required"`
	WtValuesOut          [2]string    `json:"wtValuesOut"          binding:"required"`
	WtSaltsOut           [2]string    `json:"wtSaltsOut"           binding:"required"`
}

// PaymentFeeOutput is the JSON response from the endpoint.
type PaymentFeeOutput struct {
	Proof        []*big.Int `json:"proof"`
	PublicSignal []*big.Int `json:"publicSignal"`
}
