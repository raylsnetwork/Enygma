package core

import (
	"math/big"
	"testing"
)

// ContractStatement submits the statement as held, never resized from the
// receipt's input and output counts: a trailing signal that
// NumberOfOutputs does not count is kept, and nothing is reordered.
func TestProofResult_ContractStatementIsTheStatement(t *testing.T) {
	st := []*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3), big.NewInt(4), big.NewInt(5), big.NewInt(6)}
	r := &ProofResult{Statement: st, NumberOfInputs: 1, NumberOfOutputs: 1}
	assertInts(t, "ContractStatement", r.ContractStatement(), 1, 2, 3, 4, 5, 6)
}
