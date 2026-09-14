package payment

import "math/big"

// PaymentRequest is the JSON body accepted by the /proof/payment endpoint.
//
// Config is fixed at 1 input / 2 outputs / depth 8:
//   - Input 0  : Alice's real note (value > 0).
//   - Output 0 : Payment to Bob.
//   - Output 1 : Change back to Alice.
type PaymentRequest struct {
	StMessage        string       `json:"stMessage"        binding:"required"`
	StTreeNumbers    [1]string    `json:"stTreeNumbers"    binding:"required"`
	StMerkleRoots    [1]string    `json:"stMerkleRoots"    binding:"required"`
	StNullifiers     [1]string    `json:"stNullifiers"     binding:"required"`
	StCommitmentsOut [2]string    `json:"stCommitmentsOut" binding:"required"`
	// VULN-5: vault contract address bound into the proof to prevent cross-deployment replay.
	StContractAddress string      `json:"stContractAddress" binding:"required"`

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

// PaymentOutput is the JSON response returned by the /proof/payment endpoint.
type PaymentOutput struct {
	Proof        []*big.Int `json:"proof"`
	PublicSignal []*big.Int `json:"publicSignal"`
}
