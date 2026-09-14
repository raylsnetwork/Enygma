package payment2in

import "math/big"

// PaymentRequest2in is the JSON body accepted by the /proof/payment2in endpoint.
//
// Config is fixed at 2 inputs / 2 outputs / depth 8:
//   - Input 0, 1 : Alice's two notes being consolidated or spent (value > 0 / = 0 for padding).
//   - Output 0   : Payment to Bob (or merged note).
//   - Output 1   : Change back to Alice.
type PaymentRequest2in struct {
	StMessage        string       `json:"stMessage"        binding:"required"`
	StTreeNumbers    [2]string    `json:"stTreeNumbers"    binding:"required"`
	StMerkleRoots    [2]string    `json:"stMerkleRoots"    binding:"required"`
	StNullifiers     [2]string    `json:"stNullifiers"     binding:"required"`
	StCommitmentsOut [2]string    `json:"stCommitmentsOut" binding:"required"`
	StContractAddress string      `json:"stContractAddress" binding:"required"`

	WtPrivateKeysIn      [2]string    `json:"wtPrivateKeysIn"      binding:"required"`
	WtValuesIn           [2]string    `json:"wtValuesIn"           binding:"required"`
	WtSaltsIn            [2]string    `json:"wtSaltsIn"            binding:"required"`
	WtPathElements       [2][8]string `json:"wtPathElements"       binding:"required"`
	WtPathIndices        [2]string    `json:"wtPathIndices"        binding:"required"`
	WtTokenId            string       `json:"wtTokenId"            binding:"required"`
	WtSpendPublicKeysOut [2]string    `json:"wtSpendPublicKeysOut" binding:"required"`
	WtValuesOut          [2]string    `json:"wtValuesOut"          binding:"required"`
	WtSaltsOut           [2]string    `json:"wtSaltsOut"           binding:"required"`
}

// PaymentOutput2in is the JSON response returned by the /proof/payment2in endpoint.
type PaymentOutput2in struct {
	Proof        []*big.Int `json:"proof"`
	PublicSignal []*big.Int `json:"publicSignal"`
}
