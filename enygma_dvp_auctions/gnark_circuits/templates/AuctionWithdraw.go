package templates

import (
	"github.com/consensys/gnark/frontend"
	"gnark_server/primitives"
)

// AuctionWithdrawCircuit proves that the caller knows the preimage of an
// on-chain commitA — i.e., that they own the bid they want to withdraw —
// AND that the proof is bound to a specific auction ID so it cannot be
// replayed against a different auction that happens to share the same commitA.
//
// Public statement (2 elements):
//
//	[StAuctionId, StCommitA]
type AuctionWithdrawCircuit struct {
	// --- public inputs ---
	StAuctionId frontend.Variable `gnark:",public"` // identifies which auction
	StCommitA   frontend.Variable `gnark:",public"` // the bid commitment being withdrawn

	// --- private witnesses ---
	WtSpendKey  frontend.Variable // bidder's spend secret key
	WtSaltA     frontend.Variable // salt used when constructing commitA
	WtAmount    frontend.Variable // USDC amount committed in commitA
	WtTokenId   frontend.Variable // USDC token ID
	WtAuctionId frontend.Variable // must equal StAuctionId; binds proof to one auction
}

func (circuit *AuctionWithdrawCircuit) Define(api frontend.API) error {
	// 1. Bind the proof to a single auction: the private witness must equal the
	//    public StAuctionId, so the same proof cannot be replayed with a
	//    different StAuctionId against another auction sharing the same commitA.
	api.AssertIsEqual(circuit.WtAuctionId, circuit.StAuctionId)

	// 2. Derive the bidder's spend public key: pk_A = Poseidon(sk_A).
	pkAlice := primitives.PublicKey(api, circuit.WtSpendKey)

	// 3. Recompute commitA from its preimage and assert it matches the
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
