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

// paymentStatementNames gives each public field of the Payment-family
// circuits its domain name. These must stay the names retail's copies of the
// templates use: cmd/genlayout's parity test checks the rendered layouts
// against retail's PaymentStatementLayout.
var paymentStatementNames = map[string]string{
	"StMessage":         "Message",
	"StTreeNumbers":     "TreeNumbers",
	"StMerkleRoots":     "MerkleRoots",
	"StNullifiers":      "Nullifiers",
	"StCommitmentsOut":  "CommitmentsOut",
	"StContractAddress": "ContractAddress",
}

// feeStatementNames adds the public fee of PaymentFee, PaymentRelayerFeePublic
// and UsdrFee.
var feeStatementNames = withName(paymentStatementNames, "StFee", "Fee")

// usdrFeeStatementNames adds UsdrFee's public token id.
var usdrFeeStatementNames = withName(feeStatementNames, "StTokenId", "TokenId")

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
	payment = coveredCircuit{"Payment", func() frontend.Circuit {
		c := NewPaymentCircuit(PaymentConfig)
		return &c
	}, paymentStatementNames}
	payment2in = coveredCircuit{"Payment2in", func() frontend.Circuit {
		c := NewPaymentCircuit(Payment2inConfig)
		return &c
	}, paymentStatementNames}
	paymentFee = coveredCircuit{"PaymentFee", func() frontend.Circuit {
		c := NewPaymentFeeCircuit(PaymentFeeConfig)
		return &c
	}, feeStatementNames}
	paymentRelayerFee = coveredCircuit{"PaymentRelayerFee", func() frontend.Circuit {
		c := NewPaymentRelayerFeePublicCircuit(PaymentRelayerFeePublicConfig)
		return &c
	}, feeStatementNames}
	usdrFee = coveredCircuit{"UsdrFee", func() frontend.Circuit {
		c := NewUsdrFeeCircuit(UsdrFeeConfig)
		return &c
	}, usdrFeeStatementNames}
)

func coveredCircuits() []coveredCircuit {
	return []coveredCircuit{
		dvpInitiator, dvpDestination, privateMint,
		payment, payment2in, paymentFee, paymentRelayerFee, usdrFee,
	}
}

// DvpInitiatorStatementLayout is the DvPInitiator circuit's statement layout.
func DvpInitiatorStatementLayout() statement.Layout { return dvpInitiator.mustDerive() }

// DvpDestinationStatementLayout is the DvPDestination circuit's statement
// layout.
func DvpDestinationStatementLayout() statement.Layout { return dvpDestination.mustDerive() }

// PrivateMintStatementLayout is the PrivateMint circuit's statement layout.
func PrivateMintStatementLayout() statement.Layout { return privateMint.mustDerive() }

// PaymentStatementLayout is the Payment circuit's statement layout at
// PaymentConfig.
func PaymentStatementLayout() statement.Layout { return payment.mustDerive() }

// Payment2inStatementLayout is the Payment circuit's statement layout at
// Payment2inConfig.
func Payment2inStatementLayout() statement.Layout { return payment2in.mustDerive() }

// PaymentFeeStatementLayout is the PaymentFee circuit's statement layout at
// PaymentFeeConfig.
func PaymentFeeStatementLayout() statement.Layout { return paymentFee.mustDerive() }

// PaymentRelayerFeeStatementLayout is the PaymentRelayerFeePublic circuit's
// statement layout at PaymentRelayerFeePublicConfig.
func PaymentRelayerFeeStatementLayout() statement.Layout { return paymentRelayerFee.mustDerive() }

// UsdrFeeStatementLayout is the UsdrFee circuit's statement layout at
// UsdrFeeConfig.
func UsdrFeeStatementLayout() statement.Layout { return usdrFee.mustDerive() }

// DvpStatementLayouts returns the DvP and mint statement layouts, the ones
// only enygma_dvp has.
func DvpStatementLayouts() []statement.Layout {
	return []statement.Layout{
		DvpInitiatorStatementLayout(),
		DvpDestinationStatementLayout(),
		PrivateMintStatementLayout(),
	}
}

// PaymentStatementLayouts returns enygma_dvp's Payment-family statement
// layouts, in the order retail renders PaymentStatementLayout.
func PaymentStatementLayouts() []statement.Layout {
	return []statement.Layout{
		PaymentStatementLayout(),
		Payment2inStatementLayout(),
		PaymentFeeStatementLayout(),
		PaymentRelayerFeeStatementLayout(),
		UsdrFeeStatementLayout(),
	}
}

func withName(names map[string]string, goName, name string) map[string]string {
	out := map[string]string{goName: name}
	for k, v := range names {
		out[k] = v
	}
	return out
}
