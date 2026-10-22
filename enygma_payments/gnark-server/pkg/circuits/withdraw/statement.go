package withdraw

import "enygma_gnark_shared/statement"

// DefaultConfig is the anonymity-set size the withdraw circuit is compiled,
// keyed and deployed at (Enygma.sol's DEFAULT_SIZE).
var DefaultConfig = WithdrawEnygmaCircuitConfig{NCommitment: 6}

// statementNames gives each public field of WithdrawEnygmaCircuit its domain
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
	"TotalDepositValue":   "TotalDepositValue",
	"DomainId":            "DomainId",
	"Hashes":              "DepositCommitments",
}

// StatementLayout is the withdraw circuit's statement layout at DefaultConfig.
func StatementLayout() statement.Layout {
	c := createWithdrawCircuitTemplate(DefaultConfig)
	return statement.MustDerive("Withdraw", &c, statementNames)
}
