package templates

import (
	"github.com/consensys/gnark/frontend"
	"gnark_server/primitives"
)

// AuctionWithdrawCircuit proves that the caller knows the preimage of an
// on-chain commitA — i.e., that they own the bid they want to withdraw.
//
// The circuit does NOT derive the auctionId from commitA (those values are
// linked only by the contract's _bids mapping), so StAuctionId is included
// in the public statement purely for auditability; its correctness is
// enforced by the contract's _bids[auctionId][commitA].active check.
//
// Public statement (2 elements):
//
//	[StAuctionId, StCommitA]
type AuctionWithdrawCircuit struct {
	// --- public inputs ---
	StAuctionId frontend.Variable `gnark:",public"` // identifies which auction
	StCommitA   frontend.Variable `gnark:",public"` // the bid commitment being withdrawn

	// --- private witnesses ---
	WtSpendKey frontend.Variable // bidder's spend secret key
	WtSaltA    frontend.Variable // salt used when constructing commitA
	WtAmount   frontend.Variable // USDC amount committed in commitA
	WtTokenId  frontend.Variable // USDC token ID
}

func (circuit *AuctionWithdrawCircuit) Define(api frontend.API) error {
	// 1. Derive the bidder's spend public key: pk_A = Poseidon(sk_A).
	pkAlice := primitives.PublicKey(api, circuit.WtSpendKey)

	// 2. Recompute commitA from its preimage and assert it matches the
	//    on-chain value. This proves the caller knows saltA — i.e., they
	//    are the original bidder who chose saltA at submitBid() time.
	commitA := primitives.Erc20CommitmentV2(api,
		pkAlice,
		circuit.WtSaltA,
		circuit.WtAmount,
		circuit.WtTokenId,
	)
	api.AssertIsEqual(commitA, circuit.StCommitA)

	return nil
}
