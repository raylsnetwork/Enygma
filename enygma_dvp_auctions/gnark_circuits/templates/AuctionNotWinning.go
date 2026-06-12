package templates

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/cmp"
	"gnark_server/primitives"
)

// AuctionNotWinningCircuit proves that a losing bidder's bid was strictly less
// than the winning bid, without revealing the exact losing amount.
//
// This makes the auctioneer's winner selection trustless: if the auctioneer
// announces a winning amount W, but a bidder had bid B > W, that bidder
// cannot produce a valid AuctionNotWinning proof — only the true loser can.
// Any bidder who cannot submit this proof before a deadline is assumed to be
// the actual winner (or cheated), and their commitA remains locked.
//
// After a valid proof is accepted on-chain the contract releases the loser's
// commitA back to them (they can spend it to reclaim their locked USDC).
//
// The key secret is WtSaltA: only the original bidder who ran
// ML-KEM.Encapsulate for their own bid knows saltA (HKDF-derived from their
// personal shared secret). This prevents a third party from submitting a
// not-winning proof on behalf of someone else.
//
// Public statement (3 elements):
//
//	[stAuctionId, stCommitA, stWinningAmount]
type AuctionNotWinningCircuit struct {
	// --- public inputs ---
	StAuctionId     frontend.Variable `gnark:",public"` // identifies the auction
	StCommitA       frontend.Variable `gnark:",public"` // the loser's locked bid commitment (on-chain)
	StWinningAmount frontend.Variable `gnark:",public"` // the public winning amount from AuctionSettle

	// --- private witnesses ---
	WtPkBidder  frontend.Variable // the loser's spend public key (Poseidon(sk))
	WtSaltA     frontend.Variable // salt used when building commitA — only the original bidder knows this
	WtMyAmount  frontend.Variable // the loser's bid amount (proven < StWinningAmount)
	WtTokenId   frontend.Variable // USDC token ID
}

func (circuit *AuctionNotWinningCircuit) Define(api frontend.API) error {
	// 1. Verify commitA preimage — proves this is the loser's own locked bid.
	// Only the bidder who created commitA knows WtSaltA (HKDF(personal ss)).
	commitA := primitives.Erc20CommitmentV2(api,
		circuit.WtPkBidder,
		circuit.WtSaltA,
		circuit.WtMyAmount,
		circuit.WtTokenId,
	)
	api.AssertIsEqual(commitA, circuit.StCommitA)

	// 2. Strict less-than: the loser's amount must be strictly less than the
	// winning amount. cmp.IsLess returns 1 iff a < b over the BN254 field.
	isLess := cmp.IsLess(api, circuit.WtMyAmount, circuit.StWinningAmount)
	api.AssertIsEqual(isLess, 1)

	// 3. Sanity range check: losing amount must be positive (no zero bids).
	isPositive := cmp.IsLess(api, 0, circuit.WtMyAmount)
	api.AssertIsEqual(isPositive, 1)

	return nil
}
