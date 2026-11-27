package templates

import (
	"enygma_gnark_shared/statement"

	"github.com/consensys/gnark/frontend"
)

// dvpInitiatorStatementNames gives each public field of DvPInitiatorCircuit
// its domain name. Order, offsets and sizes come from the circuit, not from
// here.
var dvpInitiatorStatementNames = map[string]string{
	"StMessage":       "Message",
	"StTreeNumber":    "TreeNumber",
	"StMerkleRoot":    "MerkleRoot",
	"StNullifier":     "Nullifier",
	"StCommitB":       "CommitB",
	"StCommitA":       "CommitA",
	"StRevertCommitA": "RevertCommitA",
	"StCounterVault":  "CounterVault",
}

// dvpDestinationStatementNames names DvPDestinationCircuit's public fields.
var dvpDestinationStatementNames = map[string]string{
	"StMessage":      "Message",
	"StTreeNumber":   "TreeNumber",
	"StMerkleRoot":   "MerkleRoot",
	"StNullifier":    "Nullifier",
	"StCommitA":      "CommitA",
	"StCounterVault": "CounterVault",
}

// privateMintStatementNames names PrivateMintCircuit's public fields, whose
// Go names already are their domain names.
var privateMintStatementNames = map[string]string{
	"Commitment":      "Commitment",
	"ContractAddress": "ContractAddress",
	"TokenId":         "TokenId",
	"CipherText":      "CipherText",
}

// coveredCircuit is a circuit whose statement layout is generated: its
// layout name, a fresh circuit at its deployed shape, and its domain names.
type coveredCircuit struct {
	layout  string
	circuit func() frontend.Circuit
	names   map[string]string
}

func (c coveredCircuit) mustDerive() statement.Layout {
	return statement.MustDerive(c.layout, c.circuit(), c.names)
}

var (
	dvpInitiator = coveredCircuit{"DvpInitiator", func() frontend.Circuit {
		return &DvPInitiatorCircuit{}
	}, dvpInitiatorStatementNames}
	dvpDestination = coveredCircuit{"DvpDestination", func() frontend.Circuit {
		return &DvPDestinationCircuit{}
	}, dvpDestinationStatementNames}
	privateMint = coveredCircuit{"PrivateMint", func() frontend.Circuit {
		return &PrivateMintCircuit{}
	}, privateMintStatementNames}
)

func coveredCircuits() []coveredCircuit {
	return []coveredCircuit{dvpInitiator, dvpDestination, privateMint}
}

// DvpInitiatorStatementLayout is the DvPInitiator circuit's statement layout.
func DvpInitiatorStatementLayout() statement.Layout { return dvpInitiator.mustDerive() }

// DvpDestinationStatementLayout is the DvPDestination circuit's statement
// layout.
func DvpDestinationStatementLayout() statement.Layout { return dvpDestination.mustDerive() }

// PrivateMintStatementLayout is the PrivateMint circuit's statement layout.
func PrivateMintStatementLayout() statement.Layout { return privateMint.mustDerive() }

// DvpStatementLayouts returns the DvP and mint statement layouts, the ones
// only enygma_dvp has.
func DvpStatementLayouts() []statement.Layout {
	return []statement.Layout{
		DvpInitiatorStatementLayout(),
		DvpDestinationStatementLayout(),
		PrivateMintStatementLayout(),
	}
}
