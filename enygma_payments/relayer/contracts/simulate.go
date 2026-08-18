package Enygma

// Hand-written sibling to enygma.go (which is abigen-generated — do not add
// this to that file, it gets overwritten on regeneration).
//
// Transfer() and TransferWithFee() are state-mutating, so abigen only ever
// generates Transactor (eth_sendTransaction) bindings for them — never a
// read-only Caller variant. These Simulate* methods fill that gap: they run
// the exact same call via eth_call (through the low-level EnygmaRaw.Call,
// same mechanism EnygmaCaller uses for view functions) against current chain
// state, without broadcasting or spending gas. A caller that would revert
// returns its decoded revert error here; one that would succeed returns nil.

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
)

// SimulateTransfer dry-runs Transfer via eth_call. Returns the decoded
// on-chain revert error if the call would fail; nil if it would succeed.
func (_Enygma *Enygma) SimulateTransfer(opts *bind.CallOpts, commitmentDeltas []IEnygmaPoint, proof IEnygmaProof, participantIds []*big.Int) error {
	var out []interface{}
	raw := EnygmaRaw{Contract: _Enygma}
	return raw.Call(opts, &out, "transfer", commitmentDeltas, proof, participantIds)
}

// SimulateTransferWithFee dry-runs TransferWithFee via eth_call. Returns the
// decoded on-chain revert error if the call would fail; nil if it would succeed.
func (_Enygma *Enygma) SimulateTransferWithFee(opts *bind.CallOpts, commitmentDeltas []IEnygmaPoint, proof IEnygmaFeeProof, participantIds []*big.Int) error {
	var out []interface{}
	raw := EnygmaRaw{Contract: _Enygma}
	return raw.Call(opts, &out, "transferWithFee", commitmentDeltas, proof, participantIds)
}
