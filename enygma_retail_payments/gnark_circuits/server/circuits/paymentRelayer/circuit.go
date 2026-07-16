package paymentRelayer

import "math/big"

// PaymentRelayerRequest is the JSON body for POST /proof/paymentRelayer.
//
// Circuit: 1 input / 3 outputs / depth 8.
// Conservation law: Σ(valuesIn) == valOut[0] + valOut[1] + valOut[2]
//
//   - Output[0]: Bob's payment    — any pk_spend
//   - Output[1]: Alice's change   — must equal senderPk (enforced in-circuit)
//   - Output[2]: Relayer fee note — any pk_spend (relayer's key)
type PaymentRelayerRequest struct {
	StMessage         string       `json:"stMessage"         binding:"required"`
	StTreeNumbers     [1]string    `json:"stTreeNumbers"     binding:"required"`
	StMerkleRoots     [1]string    `json:"stMerkleRoots"     binding:"required"`
	StNullifiers      [1]string    `json:"stNullifiers"      binding:"required"`
	StCommitmentsOut  [3]string    `json:"stCommitmentsOut"  binding:"required"`
	StContractAddress string       `json:"stContractAddress" binding:"required"`

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

// PaymentRelayerOutput is the JSON response from the /proof/paymentRelayer endpoint.
type PaymentRelayerOutput struct {
	Proof        []*big.Int `json:"proof"`
	PublicSignal []*big.Int `json:"publicSignal"`
}
