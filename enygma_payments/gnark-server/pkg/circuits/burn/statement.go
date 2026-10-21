package burn

import "enygma_payments/gnark-server/pkg/statement"

// statementNames gives each public field of BurnCircuit its domain name.
// Order, offsets and sizes come from the circuit, not from here.
var statementNames = map[string]string{
	"PublicKey":      "PublicKey",
	"PreviousCommit": "PreviousCommitment",
	"NewCommit":      "NewCommitment",
	"Amount":         "Amount",
	"BlockNumber":    "BlockNumber",
	"Nullifier":      "Nullifier",
	"DomainId":       "DomainId",
}

// StatementLayout is the burn circuit's statement layout. The circuit has
// no anonymity set, so its layout does not depend on k.
func StatementLayout() statement.Layout {
	return statement.MustDerive("Burn", &BurnCircuit{}, statementNames)
}
