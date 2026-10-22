package templates

import "enygma_gnark_shared/statement"

// paymentStatementNames gives each public field of the Payment-family
// circuits its domain name. Order, offsets and sizes come from the circuits,
// not from here.
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

// PaymentStatementLayout is the Payment circuit's statement layout at
// PaymentConfig.
func PaymentStatementLayout() statement.Layout {
	c := NewPaymentCircuit(PaymentConfig)
	return statement.MustDerive("Payment", &c, paymentStatementNames)
}

// Payment2inStatementLayout is the Payment circuit's statement layout at
// Payment2inConfig.
func Payment2inStatementLayout() statement.Layout {
	c := NewPaymentCircuit(Payment2inConfig)
	return statement.MustDerive("Payment2in", &c, paymentStatementNames)
}

// PaymentFeeStatementLayout is the PaymentFee circuit's statement layout at
// PaymentFeeConfig.
func PaymentFeeStatementLayout() statement.Layout {
	c := NewPaymentFeeCircuit(PaymentFeeConfig)
	return statement.MustDerive("PaymentFee", &c, feeStatementNames)
}

// PaymentRelayerFeeStatementLayout is the PaymentRelayerFeePublic circuit's
// statement layout at PaymentRelayerFeePublicConfig.
func PaymentRelayerFeeStatementLayout() statement.Layout {
	c := NewPaymentRelayerFeePublicCircuit(PaymentRelayerFeePublicConfig)
	return statement.MustDerive("PaymentRelayerFee", &c, feeStatementNames)
}

// UsdrFeeStatementLayout is the UsdrFee circuit's statement layout at
// UsdrFeeConfig.
func UsdrFeeStatementLayout() statement.Layout {
	c := NewUsdrFeeCircuit(UsdrFeeConfig)
	return statement.MustDerive("UsdrFee", &c, usdrFeeStatementNames)
}

// StatementLayouts returns every Payment-family statement layout.
func StatementLayouts() []statement.Layout {
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
