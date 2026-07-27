package paymentRelayerFeePublic

import "math/big"

// PaymentRelayerFeePublicRequest is the JSON body for POST /proof/paymentRelayerFeePublic.
//
// Circuit: 1 input / 3 outputs / depth 8.
//
// Conservation law: Σ(valuesIn) == valOut[0] + valOut[1] + valOut[2]
// Fee binding:      valOut[2] == StFee   (relayer's amount equals the public fee signal)
//
//   - Output[0]: Bob's payment    — any pk_spend
//   - Output[1]: Alice's change   — must equal senderPk (enforced in-circuit)
//   - Output[2]: Relayer fee note — any pk_spend, amount == StFee (publicly verifiable)
//
// Public signal layout (9 elements):
//
//	[StMessage, StTreeNumbers[0], StMerkleRoots[0], StNullifiers[0],
//	 StCommitmentsOut[0], StCommitmentsOut[1], StCommitmentsOut[2],
//	 StContractAddress, StFee]
type PaymentRelayerFeePublicRequest struct {
	StMessage         string       `json:"stMessage"         binding:"required"`
	StTreeNumbers     [1]string    `json:"stTreeNumbers"     binding:"required"`
	StMerkleRoots     [1]string    `json:"stMerkleRoots"     binding:"required"`
	StNullifiers      [1]string    `json:"stNullifiers"      binding:"required"`
	StCommitmentsOut  [3]string    `json:"stCommitmentsOut"  binding:"required"`
	StContractAddress string       `json:"stContractAddress" binding:"required"`
	StFee             string       `json:"stFee"             binding:"required"`

	WtPrivateKeysIn      [1]string    `json:"wtPrivateKeysIn"      binding:"required"`
	WtValuesIn           [1]string    `json:"wtValuesIn"           binding:"required"`
	WtSaltsIn            [1]string    `json:"wtSaltsIn"            binding:"required"`
	WtPathElements       [1][8]string `json:"wtPathElements"       binding:"required"`
	WtPathIndices        [1]string    `json:"wtPathIndices"        binding:"required"`
	WtTokenId            string       `json:"wtTokenId"            binding:"required"`
	WtSpendPublicKeysOut [3]string    `json:"wtSpendPublicKeysOut" binding:"required"`
	WtValuesOut          [3]string    `json:"wtValuesOut"          binding:"required"`
	WtSaltsOut           [3]string    `json:"wtSaltsOut"           binding:"required"`
}

// PaymentRelayerFeePublicOutput is the JSON response from the endpoint.
type PaymentRelayerFeePublicOutput struct {
	Proof        []*big.Int `json:"proof"`
	PublicSignal []*big.Int `json:"publicSignal"`
}
