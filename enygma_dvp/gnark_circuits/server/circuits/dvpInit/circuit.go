package dvpInit

import "math/big"

// DvPInitiatorRequest is the JSON body accepted by /proof/dvpInitiator.
//
// Fixed config: Merkle depth 8.
// Public statement returned: [msg, treeNum, root, nf_A, commitB, commitA, revertCommitA, counterVault]
type DvPInitiatorRequest struct {
	StMessage       string `json:"stMessage"       binding:"required"`
	StTreeNumber    string `json:"stTreeNumber"    binding:"required"`
	StMerkleRoot    string `json:"stMerkleRoot"    binding:"required"`
	StNullifier     string `json:"stNullifier"     binding:"required"`
	StCommitB       string `json:"stCommitB"       binding:"required"`
	StCommitA       string `json:"stCommitA"       binding:"required"`
	StRevertCommitA string `json:"stRevertCommitA" binding:"required"`
	// Address (decimal) of the vault the prover expects to be paid from.
	StCounterVault string `json:"stCounterVault" binding:"required"`

	WtSpendKeyIn   string    `json:"wtSpendKeyIn"   binding:"required"`
	WtValueIn      string    `json:"wtValueIn"      binding:"required"`
	WtSaltIn       string    `json:"wtSaltIn"       binding:"required"`
	WtTokenIdIn    string    `json:"wtTokenIdIn"    binding:"required"`
	WtPathElements [8]string `json:"wtPathElements" binding:"required"`
	WtPathIndex    string    `json:"wtPathIndex"    binding:"required"`

	WtSpendPkBob string `json:"wtSpendPkBob" binding:"required"`
	WtSaltB      string `json:"wtSaltB"      binding:"required"`
	WtValueBob   string `json:"wtValueBob"   binding:"required"`
	WtTokenIdBob string `json:"wtTokenIdBob" binding:"required"`
	WtSaltA      string `json:"wtSaltA"      binding:"required"`
	WtRevertSalt string `json:"wtRevertSalt" binding:"required"`
}

// DvPInitiatorOutput is the JSON response from /proof/dvpInitiator.
type DvPInitiatorOutput struct {
	Proof        []*big.Int `json:"proof"`
	PublicSignal []*big.Int `json:"publicSignal"`
}
