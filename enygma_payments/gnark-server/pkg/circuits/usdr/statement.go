package usdr

import "enygma_payments/gnark-server/pkg/statement"

// DefaultConfig is the anonymity-set size the USDr circuit is compiled, keyed
// and deployed at (Enygma.sol's DEFAULT_SIZE).
var DefaultConfig = USDrCircuitConfig{NCommitment: 6}

// statementNames gives each public field of USDrCircuit its domain name.
// Order, offsets and sizes come from the circuit, not from here.
var statementNames = map[string]string{
	"FingerPrintofSharedSecrets": "Fingerprint",
	"PublicKey":                  "PublicKeys",
	"PreviousCommit":             "PreviousCommitments",
	"TxCommit":                   "TxCommitments",
	"BlockNumber":                "BlockNumber",
	"AnonymitySet":               "AnonymitySet",
	"MessageTags":                "MessageTags",
	"Nullifier":                  "Nullifier",
	"FeeAmount":                  "FeeAmount",
	"DomainId":                   "DomainId",
	"FeeRecipientKey":            "FeeRecipientKey",
}

// StatementLayout is the USDr circuit's statement layout at DefaultConfig.
func StatementLayout() statement.Layout {
	c := createCircuitTemplate(DefaultConfig)
	return statement.MustDerive("Usdr", &c, statementNames)
}
