package dvpDestination

import "math/big"

// DvPDestinationRequest is the JSON body accepted by /proof/dvpDestination.
//
// Fixed config: Merkle depth 8.
// Public statement returned: [msg, treeNum, root, nf_B, commitA]
type DvPDestinationRequest struct {
	StMessage    string `json:"stMessage"    binding:"required"`
	StTreeNumber string `json:"stTreeNumber" binding:"required"`
	StMerkleRoot string `json:"stMerkleRoot" binding:"required"`
	StNullifier  string `json:"stNullifier"  binding:"required"`
	StCommitA    string `json:"stCommitA"    binding:"required"`

	WtSpendKeyIn   string    `json:"wtSpendKeyIn"   binding:"required"`
	WtValueIn      string    `json:"wtValueIn"      binding:"required"`
	WtSaltIn       string    `json:"wtSaltIn"       binding:"required"`
	WtTokenIdIn    string    `json:"wtTokenIdIn"    binding:"required"`
	WtPathElements [8]string `json:"wtPathElements" binding:"required"`
	WtPathIndex    string    `json:"wtPathIndex"    binding:"required"`

	WtSpendPkAlice string `json:"wtSpendPkAlice" binding:"required"`
	WtSaltA        string `json:"wtSaltA"        binding:"required"`
	WtSaltB        string `json:"wtSaltB"        binding:"required"`
	WtValueAlice   string `json:"wtValueAlice"   binding:"required"`
	WtTokenIdAlice string `json:"wtTokenIdAlice" binding:"required"`
}

// DvPDestinationOutput is the JSON response from /proof/dvpDestination.
type DvPDestinationOutput struct {
	Proof        []*big.Int `json:"proof"`
	PublicSignal []*big.Int `json:"publicSignal"`
}
