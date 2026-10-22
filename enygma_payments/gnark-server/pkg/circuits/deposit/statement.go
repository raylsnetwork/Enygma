package deposit

import "enygma_gnark_shared/statement"

// DefaultConfig is the anonymity-set size the deposit circuit is compiled,
// keyed and deployed at (Enygma.sol's DEFAULT_SIZE).
var DefaultConfig = DepositEnygmaCircuitConfig{NCommitment: 6}

// statementNames gives each public field of DepositEnygmaCircuit its domain
// name. Order, offsets and sizes come from the circuit, not from here.
var statementNames = map[string]string{
	"HashedSharedSecrets": "HashedSharedSecrets",
	"PublicKey":           "PublicKeys",
	"PreviousCommit":      "PreviousCommitments",
	"TxCommit":            "TxCommitments",
	"BlockNumber":         "BlockNumber",
	"AnonymitySet":        "AnonymitySet",
	"MessageTags":         "MessageTags",
	"Nullifier":           "Nullifier",
	"Hash":                "Hash",
	"DomainId":            "DomainId",
}

// StatementLayout is the deposit circuit's statement layout at DefaultConfig.
func StatementLayout() statement.Layout {
	c := createDepositCircuitTemplate(DefaultConfig)
	return statement.MustDerive("Deposit", &c, statementNames)
}
