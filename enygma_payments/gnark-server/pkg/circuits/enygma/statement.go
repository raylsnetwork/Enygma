package enygma

import "enygma_gnark_shared/statement"

// DefaultConfig is the anonymity-set size the transfer circuit is compiled,
// keyed and deployed at (Enygma.sol's DEFAULT_SIZE).
var DefaultConfig = EnygmaCircuitConfig{NCommitment: 6}

// statementNames gives each public field of EnygmaCircuit its domain name.
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
	"DomainId":                   "DomainId",
}

// StatementLayout is the transfer circuit's statement layout at DefaultConfig.
func StatementLayout() statement.Layout {
	c := createCircuitTemplate(DefaultConfig)
	return statement.MustDerive("Transfer", &c, statementNames)
}
