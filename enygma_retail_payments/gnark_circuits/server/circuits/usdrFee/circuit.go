package usdrFee

import "math/big"

// UsdrFeeRequest is the JSON body for POST /proof/usdrFee.
//
// Circuit: 1 input / 2 outputs / depth 8.
//
// Conservation law: Σ(valuesIn) == valOut[0] + valOut[1]  (nothing burned)
// Fee binding:      valOut[0] == StFee   (relayer's amount equals the public fee signal)
//
//   - Output[0]: Relayer fee note — any pk_spend, amount == StFee (publicly verifiable)
//   - Output[1]: Sender's change  — must equal senderPk (enforced in-circuit)
//
// Unlike every other circuit in this codebase, StTokenId is a PUBLIC signal
// here (not a private WtTokenId witness) — USDr is a single, fixed, known
// token, so there's nothing to hide about its identity, and making it public
// gives this circuit's statement a distinct length (9) from every other
// 1-in/2-out circuit's (7 or 8) — see Erc20CoinVault.checkReceiptConditions.
//
// Public signal layout (9 elements):
//
//	[StMessage, StTreeNumbers[0], StMerkleRoots[0], StNullifiers[0],
//	 StCommitmentsOut[0], StCommitmentsOut[1], StContractAddress, StFee, StTokenId]
type UsdrFeeRequest struct {
	StMessage         string    `json:"stMessage"         binding:"required"`
	StTreeNumbers     [1]string `json:"stTreeNumbers"     binding:"required"`
	StMerkleRoots     [1]string `json:"stMerkleRoots"     binding:"required"`
	StNullifiers      [1]string `json:"stNullifiers"      binding:"required"`
	StCommitmentsOut  [2]string `json:"stCommitmentsOut"  binding:"required"`
	StContractAddress string    `json:"stContractAddress" binding:"required"`
	StFee             string    `json:"stFee"             binding:"required"`
	StTokenId         string    `json:"stTokenId"         binding:"required"`

	WtPrivateKeysIn      [1]string    `json:"wtPrivateKeysIn"      binding:"required"`
	WtValuesIn           [1]string    `json:"wtValuesIn"           binding:"required"`
	WtSaltsIn            [1]string    `json:"wtSaltsIn"            binding:"required"`
	WtPathElements       [1][8]string `json:"wtPathElements"       binding:"required"`
	WtPathIndices        [1]string    `json:"wtPathIndices"        binding:"required"`
	WtSpendPublicKeysOut [2]string    `json:"wtSpendPublicKeysOut" binding:"required"`
	WtValuesOut          [2]string    `json:"wtValuesOut"          binding:"required"`
	WtSaltsOut           [2]string    `json:"wtSaltsOut"           binding:"required"`
}

// UsdrFeeOutput is the JSON response from the endpoint.
type UsdrFeeOutput struct {
	Proof        []*big.Int `json:"proof"`
	PublicSignal []*big.Int `json:"publicSignal"`
}
