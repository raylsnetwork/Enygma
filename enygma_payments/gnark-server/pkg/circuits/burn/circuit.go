// Package burn implements the H-13 fix: burn() used to be plaintext
// arithmetic on a hidden (Pedersen-committed) balance, with no way for
// anyone — including the contract's own owner — to check that the amount
// being burned did not exceed the account's actual balance. Burning more
// than the balance wrapped mod P into a value just under P, which the
// transfer circuit's 252-bit range check (wider than the ~251-bit
// subgroup order) happily accepted as spendable — the contract itself
// manufactured a spendable near-P balance through ordinary administrative
// use, no attacker or malformed proof required.
//
// This circuit makes burn proof-carrying, exactly like transfer/withdraw
// already are: the account (not the contract, and not the caller's
// say-so) proves in zero knowledge that it knows an opening of its
// current balance commitment, that the opened balance is at least the
// amount being burned, and what the correctly-updated commitment is. The
// contract (Enygma.sol's burn()) no longer does arithmetic on the hidden
// value at all — it verifies the proof, checks the proof is bound to the
// right account and the right on-chain balance, and writes the new
// commitment the proof asserts.
//
// Unlike transfer/withdraw, burn has no anonymity set: the account being
// burned is already named in plaintext by burn()'s own accountId
// argument, so there is no k-anonymity property to preserve here, and the
// circuit is correspondingly simpler — single account, no loops.
package burn

import (
	"math/big"

	pos "enygma-server/poseidon"
	utils "enygma-server/utils"

	"github.com/consensys/gnark/frontend"
)

type BurnCircuit struct {
	// Public signals
	PublicKey      frontend.Variable    `gnark:",public"` // Poseidon(sk,sk) mod P — must match publicKeys[accountId] on chain
	PreviousCommit [2]frontend.Variable `gnark:",public"` // Must match the account's current on-chain balance commitment
	NewCommit      [2]frontend.Variable `gnark:",public"` // Com(previousBalance - amount, PreviousRandomValue) — what the contract writes; blinding factor unchanged, see Define()
	Amount         frontend.Variable    `gnark:",public"` // What's actually decremented from totalSupply — single source of truth, not a separately-trusted contract argument
	BlockNumber    frontend.Variable    `gnark:",public"` // Epoch anchor, same convention as every other circuit's nullifier
	Nullifier      frontend.Variable    `gnark:",public"` // Prevents replaying the same burn proof
	// Fix L-01: chainId<<160 | contractAddress — see gnark-server/pkg/circuits/enygma's
	// DomainId field doc comment for the full reasoning; identical here.
	DomainId frontend.Variable `gnark:",public"`

	// Private signals
	SecretKey           frontend.Variable // Proves authorization: knowledge of the account's spend key
	PreviousBalance     frontend.Variable // Opens PreviousCommit
	PreviousRandomValue frontend.Variable // Opens PreviousCommit; also the nullifier preimage's input, same as every other circuit
}

func (circuit *BurnCircuit) Define(api frontend.API) error {
	// Knowledge of secret key matching the account's registered PublicKey —
	// this is what makes burn require the account's cooperation instead of
	// being purely owner-administrative: whoever calls Enygma.burn() must
	// have obtained a proof only the account holder could produce.
	pk := pos.Poseidon(api, []frontend.Variable{circuit.SecretKey, circuit.SecretKey})
	pkMod := utils.ReduceModP(api, pk)
	api.AssertIsEqual(circuit.PublicKey, pkMod)

	// Knowledge of the account's current balance commitment.
	computedPreviousCommitment := utils.PedersenCommitment(api, circuit.PreviousBalance, circuit.PreviousRandomValue)
	api.AssertIsEqual(circuit.PreviousCommit[0], computedPreviousCommitment.X)
	api.AssertIsEqual(circuit.PreviousCommit[1], computedPreviousCommitment.Y)

	// Range-check both quantities to 64 bits before comparing — same
	// pattern as C-02/C-08: comparing/subtracting raw field elements
	// without first bounding them lets a witness alias mod P (Amount or
	// PreviousBalance claimed as x, x+P, x+2P, ... all satisfy a mod-P
	// congruence identically), which is the exact "wrapped balance" defect
	// H-13 is about — proof-gating the operation is worthless if the
	// proof's own arithmetic can be aliased the same way the plaintext
	// contract code was.
	amountBits := api.ToBinary(circuit.Amount, 64)
	amountConstrained := api.FromBinary(amountBits...)
	previousVBits := api.ToBinary(circuit.PreviousBalance, 64)
	previousVConstrained := api.FromBinary(previousVBits...)

	// The fix's core assertion: previousBalance >= amount, checked as
	// integers (both operands bounded above to 64 bits, so this is a
	// genuine magnitude comparison, not a mod-P one — matching C-08's
	// withdraw solvency comparator exactly).
	prevGreaterEqualAmount := api.Cmp(previousVConstrained, amountConstrained)
	api.AssertIsEqual(api.IsZero(api.Add(prevGreaterEqualAmount, frontend.Variable(1))), frontend.Variable(0))

	// newBalance = previousBalance - amount is a safe integer subtraction
	// here specifically because the Cmp assertion above already forces
	// previousVConstrained >= amountConstrained for any witness that
	// satisfies the circuit — there is no way to reach this line with a
	// witness where the subtraction would be negative. No separate range
	// check is needed on the result: it's a deterministic linear
	// combination of two already-bounded wires, not a fresh witness value
	// a prover could choose independently.
	newBalance := api.Sub(previousVConstrained, amountConstrained)

	// The "correct new commitment" half of the remediation: the circuit
	// — not the contract — asserts what the post-burn commitment is.
	// Reuses PreviousRandomValue (not an independently-chosen blinding
	// factor) — this is required, not a simplification: Enygma.sol's
	// totalSupply tracking adjusts homomorphically by Com(-amount, 0) (a
	// zero-blinded delta, mirroring mintSupply's Com(amount,0) in
	// reverse), so Σ(balances) stays equal to totalSupply as *points* only
	// if this account's own blinding factor is unchanged by the burn —
	// exactly like the original plaintext code's Com(balance,r) +
	// Com(-amount,0) = Com(balance-amount, r) did. A caught-in-testing
	// bug: an earlier draft exposed a free NewRandomValue private input
	// here, which let a witness satisfy this circuit while silently
	// breaking check() on chain (TestBurnBalanceUpdate reverted
	// BalanceMismatch against the real, unmodified contract logic).
	computedNewCommitment := utils.PedersenCommitment(api, newBalance, circuit.PreviousRandomValue)
	api.AssertIsEqual(circuit.NewCommit[0], computedNewCommitment.X)
	api.AssertIsEqual(circuit.NewCommit[1], computedNewCommitment.Y)

	// Nullifier: same construction as every other circuit
	// (Poseidon(Poseidon(prevR, sk) mod P, BlockNumber)) — replay
	// protection, and incidentally a second proof of sk knowledge.
	secretSenderCalculated := pos.Poseidon(api, []frontend.Variable{circuit.PreviousRandomValue, circuit.SecretKey})
	secretRemain := utils.ReduceModP(api, secretSenderCalculated)
	computedNullifier := pos.Poseidon(api, []frontend.Variable{secretRemain, circuit.BlockNumber})
	api.AssertIsEqual(computedNullifier, circuit.Nullifier)

	// Fix L-01: keep DomainId a genuinely constrained wire.
	api.AssertIsEqual(circuit.DomainId, circuit.DomainId)

	return nil
}

type BurnRequest struct {
	PublicKey           string    `json:"public_key" binding:"required"`
	PreviousCommit      [2]string `json:"previous_commit" binding:"required,len=2"`
	NewCommit           [2]string `json:"new_commit" binding:"required,len=2"`
	Amount              string    `json:"amount" binding:"required"`
	BlockNumber         string    `json:"block_number" binding:"required"`
	Nullifier           string    `json:"nullifier" binding:"required"`
	SecretKey           string    `json:"secret_key" binding:"required"`
	PreviousBalance     string    `json:"previous_balance" binding:"required"`
	PreviousRandomValue string    `json:"previous_random_value" binding:"required"`
	// Fix L-01: caller-supplied chainId<<160|contractAddress — the
	// handler doesn't compute this itself since it has no chain
	// connection of its own; the client (which does) supplies it.
	DomainId string `json:"domain_id" binding:"required"`
}

type BurnOutput struct {
	Proof        []*big.Int `json:"proof"`
	PublicSignal []*big.Int `json:"publicSignal"`
}
