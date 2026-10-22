package server

import (
	"fmt"
	"math/big"
	"strings"

	"enygma_dvp/layout"
)

// Every receipt the relayer accepts is one circuit's statement, and each route
// declares which circuits' statement layouts its receipts may have. The relayer
// reads a statement's fields only at the positions enygma_dvp/layout declares,
// and refuses a publicSignal that is not the statement length before it
// reads anything else or calls the chain.

// field is one named field of a statement layout: its offset and size, as
// enygma_dvp/layout declares them.
type field struct{ offset, size int }

// in returns the field's signals in statement, or nil if the layout has no
// such field.
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

// at returns the field's i-th signal as a single-signal field.
func (f field) at(i int) field { return field{f.offset + i, 1} }

// statementLayout is one circuit's statement layout as the relayer reads it:
// its length, the numberOfInputs/numberOfOutputs its receipts report on-chain,
// and the fields the relayer inspects. A field the relayer does not read for
// that circuit is left zero.
type statementLayout struct {
	name      string
	length    int
	nIn, nOut int

	treeNumbers, merkleRoots, nullifiers field

	// message, commitmentsOut, fee and tokenId are read only on the fee and
	// withdrawal routes.
	message, commitmentsOut, fee, tokenId field
}

var (
	paymentLayout = statementLayout{
		name: "Payment", length: layout.PaymentLen, nIn: 1, nOut: 2,
		treeNumbers: field{layout.PaymentTreeNumbersOffset, layout.PaymentTreeNumbersSize},
		merkleRoots: field{layout.PaymentMerkleRootsOffset, layout.PaymentMerkleRootsSize},
		nullifiers:  field{layout.PaymentNullifiersOffset, layout.PaymentNullifiersSize},
	}
	payment2inLayout = statementLayout{
		name: "Payment2in", length: layout.Payment2inLen, nIn: 2, nOut: 2,
		treeNumbers: field{layout.Payment2inTreeNumbersOffset, layout.Payment2inTreeNumbersSize},
		merkleRoots: field{layout.Payment2inMerkleRootsOffset, layout.Payment2inMerkleRootsSize},
		nullifiers:  field{layout.Payment2inNullifiersOffset, layout.Payment2inNullifiersSize},
	}
	paymentRelayerFeeLayout = statementLayout{
		name: "PaymentRelayerFee", length: layout.PaymentRelayerFeeLen, nIn: 1, nOut: 3,
		treeNumbers:    field{layout.PaymentRelayerFeeTreeNumbersOffset, layout.PaymentRelayerFeeTreeNumbersSize},
		merkleRoots:    field{layout.PaymentRelayerFeeMerkleRootsOffset, layout.PaymentRelayerFeeMerkleRootsSize},
		nullifiers:     field{layout.PaymentRelayerFeeNullifiersOffset, layout.PaymentRelayerFeeNullifiersSize},
		message:        field{layout.PaymentRelayerFeeMessageOffset, layout.PaymentRelayerFeeMessageSize},
		commitmentsOut: field{layout.PaymentRelayerFeeCommitmentsOutOffset, layout.PaymentRelayerFeeCommitmentsOutSize},
		fee:            field{layout.PaymentRelayerFeeFeeOffset, layout.PaymentRelayerFeeFeeSize},
	}
	usdrFeeLayout = statementLayout{
		name: "UsdrFee", length: layout.UsdrFeeLen, nIn: 1, nOut: 2,
		treeNumbers:    field{layout.UsdrFeeTreeNumbersOffset, layout.UsdrFeeTreeNumbersSize},
		merkleRoots:    field{layout.UsdrFeeMerkleRootsOffset, layout.UsdrFeeMerkleRootsSize},
		nullifiers:     field{layout.UsdrFeeNullifiersOffset, layout.UsdrFeeNullifiersSize},
		commitmentsOut: field{layout.UsdrFeeCommitmentsOutOffset, layout.UsdrFeeCommitmentsOutSize},
		fee:            field{layout.UsdrFeeFeeOffset, layout.UsdrFeeFeeSize},
		tokenId:        field{layout.UsdrFeeTokenIdOffset, layout.UsdrFeeTokenIdSize},
	}
	// A DvP leg reports one output on-chain: only its first commitment is a
	// payment output (see EnygmaDvp's settlement).
	dvpInitiatorLayout = statementLayout{
		name: "DvpInitiator", length: layout.DvpInitiatorLen, nIn: 1, nOut: 1,
		treeNumbers: field{layout.DvpInitiatorTreeNumberOffset, layout.DvpInitiatorTreeNumberSize},
		merkleRoots: field{layout.DvpInitiatorMerkleRootOffset, layout.DvpInitiatorMerkleRootSize},
		nullifiers:  field{layout.DvpInitiatorNullifierOffset, layout.DvpInitiatorNullifierSize},
	}
	dvpDestinationLayout = statementLayout{
		name: "DvpDestination", length: layout.DvpDestinationLen, nIn: 1, nOut: 1,
		treeNumbers: field{layout.DvpDestinationTreeNumberOffset, layout.DvpDestinationTreeNumberSize},
		merkleRoots: field{layout.DvpDestinationMerkleRootOffset, layout.DvpDestinationMerkleRootSize},
		nullifiers:  field{layout.DvpDestinationNullifierOffset, layout.DvpDestinationNullifierSize},
	}
)

// relayerFeeNote is the relayer's fee note in a PaymentRelayerFee statement:
// output 2, after the recipient's note and the sender's change.
var relayerFeeNote = paymentRelayerFeeLayout.commitmentsOut.at(2)

// usdrFeeNote is the relayer's fee note in a UsdrFee statement: output 0,
// before the sender's change.
var usdrFeeNote = usdrFeeLayout.commitmentsOut.at(0)

// withdrawalNote is the note a PaymentRelayerFee withdrawal pays out: output 0.
var withdrawalNote = paymentRelayerFeeLayout.commitmentsOut.at(0)

// layoutFor returns the statement layout among accepted whose receipts report
// nIn inputs and nOut outputs, and checks that the publicSignal has
// signalCount signals, its statement length.
func layoutFor(nIn, nOut, signalCount int, accepted ...statementLayout) (statementLayout, error) {
	shapes := make([]string, len(accepted))
	for i, l := range accepted {
		shapes[i] = fmt.Sprintf("a %d-input/%d-output %s", l.nIn, l.nOut, l.name)
		if l.nIn != nIn || l.nOut != nOut {
			continue
		}
		if signalCount != l.length {
			return statementLayout{}, fmt.Errorf("publicSignal has %d signals; a %s statement has %d", signalCount, l.name, l.length)
		}
		return l, nil
	}
	return statementLayout{}, fmt.Errorf("expected %s receipt, got a %d-input/%d-output one",
		strings.Join(shapes, " or "), nIn, nOut)
}
