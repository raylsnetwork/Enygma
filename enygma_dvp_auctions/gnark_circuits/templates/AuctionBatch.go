package templates

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/cmp"
	"gnark_server/primitives"
)

// AuctionBatchSize is the maximum number of bids processed in one Phase-1 proof.
const AuctionBatchSize = 100

// AuctionBatchCircuit is the Phase-1 circuit.  The auctioneer opens every bid
// in a batch by providing the preimage of each on-chain commitA, then proves
// which slot carries the highest bid without revealing individual amounts.
//
// Inactive slots are marked by WtActive[i]==0 and must have StBidCommitA[i]==0.
// The batch winner slot must be active and have a strictly positive amount.
//
// Public statement (AuctionBatchSize+4 elements):
//
//	[StAuctionId, StBidCommitA[0..99], StBatchWinnerCommit, StBatchWinnerPk, StBatchWinnerAmount]
type AuctionBatchCircuit struct {
	// --- public inputs ---
	StAuctionId         frontend.Variable  `gnark:",public"` // identifies the auction
	StBidCommitA        []frontend.Variable `gnark:",public"` // on-chain commitA values; 0 = inactive slot
	StBatchWinnerCommit frontend.Variable  `gnark:",public"` // commitA of the batch winner
	StBatchWinnerPk     frontend.Variable  `gnark:",public"` // spend public key of the batch winner
	StBatchWinnerAmount frontend.Variable  `gnark:",public"` // bid amount of the batch winner

	// --- private witnesses (auctioneer's ML-KEM decryptions) ---
	WtActive    []frontend.Variable // boolean 0/1 per slot; length AuctionBatchSize
	WtPkBidder  []frontend.Variable // bidder's spend public key = Poseidon(sk)
	WtSaltA     []frontend.Variable // bid salt A = HKDF(ss_personal, "bid salt A")
	WtAmount    []frontend.Variable // bid amount for each slot
	WtTokenId   []frontend.Variable // USDC token ID for each slot
	WtWinnerIdx frontend.Variable   // index [0, AuctionBatchSize) of the winning slot
}

func (circuit *AuctionBatchCircuit) Define(api frontend.API) error {
	n := AuctionBatchSize

	for i := 0; i < n; i++ {
		// WtActive[i] must be 0 or 1.
		api.AssertIsBoolean(circuit.WtActive[i])

		// Inactive slot: StBidCommitA[i] must be 0.
		// (1 - WtActive[i]) * StBidCommitA[i] == 0
		api.AssertIsEqual(
			api.Mul(api.Sub(1, circuit.WtActive[i]), circuit.StBidCommitA[i]),
			0,
		)

		// Active slot: verify commitA preimage.
		// WtActive[i] * (computed - StBidCommitA[i]) == 0
		computed := primitives.Erc20CommitmentV2(api,
			circuit.WtPkBidder[i],
			circuit.WtSaltA[i],
			circuit.WtAmount[i],
			circuit.WtTokenId[i],
		)
		api.AssertIsEqual(
			api.Mul(circuit.WtActive[i], api.Sub(computed, circuit.StBidCommitA[i])),
			0,
		)

		// Winner dominance: for every active slot, batch winner amount >= slot amount.
		// IsLess(StBatchWinnerAmount, WtAmount[i]) == 1  ⟺  winner < slot  ⟹  violation.
		isGreater := cmp.IsLess(api, circuit.StBatchWinnerAmount, circuit.WtAmount[i])
		api.AssertIsEqual(api.Mul(circuit.WtActive[i], isGreater), 0)
	}

	// Select the winning slot's fields by private index WtWinnerIdx.
	var selAmount, selCommit, selPk, selActive frontend.Variable
	selAmount = 0
	selCommit = 0
	selPk = 0
	selActive = 0
	for i := 0; i < n; i++ {
		isIdx := api.IsZero(api.Sub(circuit.WtWinnerIdx, i))
		selAmount = api.Add(selAmount, api.Mul(isIdx, circuit.WtAmount[i]))
		selCommit = api.Add(selCommit, api.Mul(isIdx, circuit.StBidCommitA[i]))
		selPk = api.Add(selPk, api.Mul(isIdx, circuit.WtPkBidder[i]))
		selActive = api.Add(selActive, api.Mul(isIdx, circuit.WtActive[i]))
	}
	api.AssertIsEqual(selAmount, circuit.StBatchWinnerAmount)
	api.AssertIsEqual(selCommit, circuit.StBatchWinnerCommit)
	api.AssertIsEqual(selPk, circuit.StBatchWinnerPk)
	// Winner slot must be active.
	api.AssertIsEqual(selActive, 1)

	// Batch winner must have a strictly positive bid.
	isPositive := cmp.IsLess(api, 0, circuit.StBatchWinnerAmount)
	api.AssertIsEqual(isPositive, 1)

	return nil
}
