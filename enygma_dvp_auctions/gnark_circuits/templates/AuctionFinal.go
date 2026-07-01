package templates

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/cmp"
	"gnark_server/primitives"
)

// AuctionFinalBatches is the maximum number of Phase-1 batch results that the
// Phase-2 final circuit can consume.  With AuctionBatchSize==100 this supports
// up to 1 000 total bids per auction.
const AuctionFinalBatches = 10

// AuctionFinalCircuit is the Phase-2 (settlement) circuit.  It receives the
// public outputs of up to AuctionFinalBatches AuctionBatch proofs and proves:
//
//  1. The overall winner has the maximum amount across all active batches.
//
//  2. The winner's USDC payout commitment (StWinnerCommitB) is correctly
//     formed from the auctioneer's ML-KEM decryption, proving they honestly
//     read the winning bid.
//
//  3. The winner's NFT commitment uses StWinnerPk — a value that is publicly
//     proven by the Phase-1 batch proof — so the auctioneer cannot redirect
//     the NFT to an arbitrary key.
//
// Inactive batch slots are indicated by StBatchWinnerCommit[i]==0 and
// WtBatchActive[i]==0.
//
// StFloorPrice enforces the seller's reserve price in-circuit: a proof simply
// cannot exist if the winning amount falls short, so settle()/settleOptimistic()
// can never succeed for a sub-floor auction. Bob's floor is recorded on-chain
// at initAuction() time and cross-checked by the contract against this input.
//
// Public statement (3*AuctionFinalBatches+8 elements):
//
//	[StAuctionId,
//	 StBatchWinnerCommit[0..9], StBatchWinnerPk[0..9], StBatchWinnerAmount[0..9],
//	 StOverallWinnerCommit, StWinnerPk, StWinnerCommitB,
//	 StWinnerNftCommit, StNftTokenId, StWinningAmount, StFloorPrice]
type AuctionFinalCircuit struct {
	// --- public inputs: Phase-1 batch outputs ---
	StAuctionId         frontend.Variable  `gnark:",public"`
	StBatchWinnerCommit []frontend.Variable `gnark:",public"` // commitA per batch; 0 = inactive
	StBatchWinnerPk     []frontend.Variable `gnark:",public"` // winner spend pk per batch
	StBatchWinnerAmount []frontend.Variable `gnark:",public"` // winner amount per batch

	// --- public outputs: settlement ---
	StOverallWinnerCommit frontend.Variable `gnark:",public"` // overall winner's commitA
	StWinnerPk            frontend.Variable `gnark:",public"` // overall winner's spend public key
	StWinnerCommitB       frontend.Variable `gnark:",public"` // winner's USDC payout commitment (Bob receives)
	StWinnerNftCommit     frontend.Variable `gnark:",public"` // new NFT commitment for winner (Alice receives)
	StNftTokenId          frontend.Variable `gnark:",public"` // NFT token ID
	StWinningAmount       frontend.Variable `gnark:",public"` // overall winning amount
	StFloorPrice          frontend.Variable `gnark:",public"` // seller's reserve price; must match auctions[id].floorPrice

	// --- private witnesses ---
	WtAuctionId      frontend.Variable   // must equal StAuctionId; binds proof to one auction
	WtBatchActive    []frontend.Variable // boolean; length AuctionFinalBatches
	WtWinnerBatchIdx frontend.Variable   // which batch contains the overall winner
	WtPkBob          frontend.Variable   // NFT seller's spend key (from auctioneer's decryption)
	WtSaltB          frontend.Variable   // HKDF(ss_winner, "note salt") for Bob's USDC note
	WtUsdcTokenId    frontend.Variable   // USDC token ID
	WtSaltAlice      frontend.Variable   // fresh random salt for winner's NFT commitment
	WtNftTokenId     frontend.Variable   // NFT token ID (constrained == StNftTokenId)
}

func (circuit *AuctionFinalCircuit) Define(api frontend.API) error {
	// Bind the proof to a single auction so it cannot be replayed across auctions.
	api.AssertIsEqual(circuit.WtAuctionId, circuit.StAuctionId)

	k := AuctionFinalBatches

	for i := 0; i < k; i++ {
		api.AssertIsBoolean(circuit.WtBatchActive[i])

		// Inactive batch: commitment must be 0.
		api.AssertIsEqual(
			api.Mul(api.Sub(1, circuit.WtBatchActive[i]), circuit.StBatchWinnerCommit[i]),
			0,
		)

		// Overall winner dominance: for every active batch, overall amount >= batch amount.
		isGreater := cmp.IsLess(api, circuit.StWinningAmount, circuit.StBatchWinnerAmount[i])
		api.AssertIsEqual(api.Mul(circuit.WtBatchActive[i], isGreater), 0)
	}

	// Select the winning batch by private index WtWinnerBatchIdx.
	var selAmount, selCommit, selPk, selActive frontend.Variable
	selAmount = 0
	selCommit = 0
	selPk = 0
	selActive = 0
	for i := 0; i < k; i++ {
		isIdx := api.IsZero(api.Sub(circuit.WtWinnerBatchIdx, i))
		selAmount = api.Add(selAmount, api.Mul(isIdx, circuit.StBatchWinnerAmount[i]))
		selCommit = api.Add(selCommit, api.Mul(isIdx, circuit.StBatchWinnerCommit[i]))
		selPk = api.Add(selPk, api.Mul(isIdx, circuit.StBatchWinnerPk[i]))
		selActive = api.Add(selActive, api.Mul(isIdx, circuit.WtBatchActive[i]))
	}
	api.AssertIsEqual(selAmount, circuit.StWinningAmount)
	api.AssertIsEqual(selCommit, circuit.StOverallWinnerCommit)
	api.AssertIsEqual(selPk, circuit.StWinnerPk)
	// Winning batch must be active.
	api.AssertIsEqual(selActive, 1)

	// Verify commitB: the auctioneer proves they correctly decrypted the winning
	// bid.  WtPkBob and WtSaltB are recovered via ML-KEM.Decapsulate(sk_auction).
	commitB := primitives.Erc20CommitmentV2(api,
		circuit.WtPkBob,
		circuit.WtSaltB,
		circuit.StWinningAmount,
		circuit.WtUsdcTokenId,
	)
	api.AssertIsEqual(commitB, circuit.StWinnerCommitB)

	// Build the winner's NFT commitment.  StWinnerPk is public and was proven by
	// the Phase-1 batch proof, so the auctioneer cannot substitute an arbitrary key.
	commitNft := primitives.Erc721Commitment(api,
		circuit.WtNftTokenId,
		circuit.StWinnerPk,
		circuit.WtSaltAlice,
	)
	api.AssertIsEqual(commitNft, circuit.StWinnerNftCommit)

	// NFT tokenId consistency.
	api.AssertIsEqual(circuit.WtNftTokenId, circuit.StNftTokenId)

	// Winning amount must be strictly positive.``
	isPositive := cmp.IsLess(api, 0, circuit.StWinningAmount)
	api.AssertIsEqual(isPositive, 1)

	
	// Check that StWinningAmount > StFloorPrice!
	belowFloor := cmp.IsLess(api, circuit.StWinningAmount, circuit.StFloorPrice)
	api.AssertIsEqual(belowFloor, 0)

	return nil
}
