package core

import (
	"encoding/json"
	"fmt"
	"math/big"
)

// Every proof result holds its statement in statement layout order: the
// public signals exactly as the gnark server returned them, which is the
// order the contracts read. Its accessors name the fields of that statement,
// at the positions enygma_dvp/layout declares, and ContractStatement returns
// it unchanged for on-chain submission. A result that one circuit proves reads
// its layout's offsets directly; PaymentResult, which several Payment-family
// circuits prove, reads them through the field values of its circuit's layout.

// field is one named field of a statement layout: its offset and size, as
// enygma_dvp/layout declares them.
type field struct{ offset, size int }

// in returns the field's signals in statement, or nil if the layout has no
// such field. The result shares statement's backing array but cannot be
// appended into it.
func (f field) in(statement []*big.Int) []*big.Int {
	if f.size == 0 {
		return nil
	}
	return statement[f.offset : f.offset+f.size : f.offset+f.size]
}

// one returns a single-signal field's value in statement, or nil if the
// layout has no such field.
func (f field) one(statement []*big.Int) *big.Int {
	if f.size == 0 {
		return nil
	}
	return statement[f.offset]
}

// proofResponse decodes a prove handler's response: the proof as decimal
// strings and the public signals it was proved against. A statement that is
// not statementLen signals long is an error, so a result's accessors never
// read past it or read the wrong circuit's fields.
func proofResponse(body []byte, statementLen int) (proof []string, statement []*big.Int, err error) {
	var resp struct {
		Proof        []json.Number `json:"proof"`
		PublicSignal []json.Number `json:"publicSignal"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, nil, err
	}
	if len(resp.PublicSignal) != statementLen {
		return nil, nil, fmt.Errorf("publicSignal has %d signals, want the statement length %d", len(resp.PublicSignal), statementLen)
	}
	proof = make([]string, len(resp.Proof))
	for i, n := range resp.Proof {
		proof[i] = n.String()
	}
	statement = make([]*big.Int, len(resp.PublicSignal))
	for i, n := range resp.PublicSignal {
		v, ok := new(big.Int).SetString(n.String(), 10)
		if !ok {
			return nil, nil, fmt.Errorf("publicSignal[%d]: %q is not a decimal integer", i, n)
		}
		statement[i] = v
	}
	return proof, statement, nil
}

// joinSplitStatement builds a legacy join-split circuit's statement in the
// order the contracts read it: the message, the inputs' tree numbers, Merkle
// roots and nullifiers as three runs, then the output commitments. These
// circuits have no template in enygma_dvp, so no statement layout either.
func joinSplitStatement(message *big.Int, treeNumbers, merkleRoots, nullifiers, commitmentsOut []*big.Int) []*big.Int {
	statement := []*big.Int{message}
	statement = append(statement, treeNumbers...)
	statement = append(statement, merkleRoots...)
	statement = append(statement, nullifiers...)
	return append(statement, commitmentsOut...)
}
