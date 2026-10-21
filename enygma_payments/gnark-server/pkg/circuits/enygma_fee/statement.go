package enygma_fee

import "enygma_payments/gnark-server/pkg/statement"

// DefaultConfig is the anonymity-set size the transfer-with-fee circuit is
// compiled, keyed and deployed at (Enygma.sol's DEFAULT_SIZE).
var DefaultConfig = EnygmaFeeCircuitConfig{NCommitment: 6}

// statementNames gives each public field of EnygmaFeeCircuit its domain name.
// Order, offsets and sizes come from the circuit, not from here.
var statementNames = map[string]string{
	"HashedSharedSecrets": "HashedSharedSecrets",
	"PublicKey":           "PublicKeys",
	"PreviousCommit":      "PreviousCommitments",
	"TxCommit":            "TxCommitments",
	"BlockNumber":         "BlockNumber",
	"AnonymitySet":        "AnonymitySet",
	"MessageTags":         "MessageTags",
	"Nullifier":           "Nullifier",
	"Fee":                 "FeeAmount",
	"SumTxCommit":         "SumTxCommitments",
	"SumTxValuesWithFee":  "SumTxValuesWithFee",
	"DomainId":            "DomainId",
}

// StatementLayout is the transfer-with-fee circuit's statement layout at
// DefaultConfig.
func StatementLayout() statement.Layout {
	c := createCircuitTemplate(DefaultConfig)
	return statement.MustDerive("TransferFee", &c, statementNames)
}
