package tests

// Integration test for the two-phase ZK sealed-bid auction with 252 bidders.
//
// Scenario:
//   Bob locks an ERC-721 NFT (tokenId=77) for auction.
//   252 bidders each submit a sealed USDC bid (bidder i → amount = i+1).
//   The auctioneer processes 3 Phase-1 batches of up to 100 bids each:
//     Batch 0: bidders   0–99  (amounts   1–100), local winner = bidder  99 (amount 100)
//     Batch 1: bidders 100–199 (amounts 101–200), local winner = bidder 199 (amount 200)
//     Batch 2: bidders 200–251 (amounts 201–252), local winner = bidder 251 (amount 252)
//   Phase-2 final proof picks the overall winner: bidder 251 (amount 252), which
//   clears the seller's floor price.
//
// The test does NOT generate AuctionBid proofs for each bidder — that would
// require ~252 separate gnark server calls and is omitted for runtime.  The
// AuctionBatch circuit already verifies the preimage of every commitA, so
// correctness is still exercised end-to-end. One AuctionBid proof (bidder 0)
// is generated as a representative sample of the endpoint/flow.
//
// All crypto/Merkle/HTTP plumbing is provided by the shared
// github.com/raylsnetwork/enygma_dvp_auctions/src/core library — this test
// only supplies scenario data.
//
// Prerequisites:
//   1. Auction gnark server running on :8083
//      cd gnark_circuits && go run generation.go   (once)
//      cd gnark_circuits && go run main.go
//
// Run with:
//   cd test && go mod tidy && CC=/usr/bin/clang go test -run TestAuction_252Bids -v -timeout 1800s

import (
	"math/big"
	"testing"

	"github.com/raylsnetwork/enygma_dvp_auctions/src/core"
)

const (
	numBidders   = 252
	nftTokenIdV  = 77
	usdcTokenIdV = 0
)

type bidderState struct {
	sk      *core.SpendKeyPair
	saltIn  *big.Int
	amount  *big.Int
	saltA   *big.Int
	saltB   *big.Int
	commitA *big.Int
	commitB *big.Int
	commitIn *big.Int
}

func TestAuction_252Bids(t *testing.T) {
	if !serverAvailable(serverAddr) {
		t.Skip("auction gnark server not running on " + serverAddr + " — skipping")
	}

	client := core.NewAuctionClient("")
	nftTokenId := big.NewInt(nftTokenIdV)
	usdcTokenId := big.NewInt(usdcTokenIdV)

	// ── Phase 0a: Bob locks his NFT ───────────────────────────────────────
	t.Log("── Phase 0a: AuctionLock ─────────────────────────────────────────────")

	bob, err := core.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("NewSpendKeyPair: %v", err)
	}

	bobSaltIn, err := core.RandomInField()
	if err != nil {
		t.Fatalf("RandomInField: %v", err)
	}
	nftCommitIn, err := core.Erc721Commitment(nftTokenId, bob.PublicKey, bobSaltIn)
	if err != nil {
		t.Fatalf("Erc721Commitment: %v", err)
	}

	nftTree := core.NewMerkleTree(core.AuctionMerkleDepth)
	nftTree.InsertLeaf(nftCommitIn)
	nftProof, err := nftTree.GenerateProof(nftCommitIn)
	if err != nil {
		t.Fatalf("GenerateProof(nft): %v", err)
	}

	saltLocked, err := core.RandomInField()
	if err != nil {
		t.Fatalf("RandomInField: %v", err)
	}
	saltRevertBob, err := core.RandomInField()
	if err != nil {
		t.Fatalf("RandomInField: %v", err)
	}

	lockResult, err := client.AuctionLockProof(core.AuctionLockParams{
		Bob:         bob,
		TokenId:     nftTokenId,
		SaltIn:      bobSaltIn,
		TreeNumber:  big.NewInt(0),
		MerkleProof: nftProof,
		SaltLocked:  saltLocked,
		SaltRevert:  saltRevertBob,
	})
	if err != nil {
		t.Fatalf("AuctionLock: %v", err)
	}

	auctionId := lockResult.PublicSignal[0]
	t.Logf("auctionId   = %s", auctionId)
	t.Logf("bob pk      = %s", bob.PublicKey)
	t.Log("AuctionLock OK")

	// ── Deposit 252 bidders' USDC notes ───────────────────────────────────
	usdcTree := core.NewMerkleTree(core.AuctionMerkleDepth)
	bidders := make([]*bidderState, numBidders)

	for i := 0; i < numBidders; i++ {
		sk, err := core.NewSpendKeyPair()
		if err != nil {
			t.Fatalf("NewSpendKeyPair(bidder %d): %v", i, err)
		}
		saltIn, err := core.RandomInField()
		if err != nil {
			t.Fatalf("RandomInField(bidder %d): %v", i, err)
		}
		amount := big.NewInt(int64(i + 1))
		commitIn, err := core.Erc20CommitmentV2(sk.PublicKey, saltIn, amount, usdcTokenId)
		if err != nil {
			t.Fatalf("Erc20CommitmentV2(bidder %d): %v", i, err)
		}
		usdcTree.InsertLeaf(commitIn)
		bidders[i] = &bidderState{sk: sk, saltIn: saltIn, amount: amount, commitIn: commitIn}
	}
	t.Logf("USDC tree root (%d leaves) = %s", numBidders, usdcTree.Root())

	// Generate Merkle proofs only after every leaf is inserted, so every
	// proof is against the same final root.
	proofs := make([]*core.MerkleProof, numBidders)
	for i := 0; i < numBidders; i++ {
		p, err := usdcTree.GenerateProof(bidders[i].commitIn)
		if err != nil {
			t.Fatalf("GenerateProof(bidder %d): %v", i, err)
		}
		proofs[i] = p
	}

	// ── Phase 0b: one sample AuctionBid proof (bidder 0) ──────────────────
	t.Log("── Phase 0b: AuctionBid sample (bidder 0) ────────────────────────────")

	saltA0, err := core.RandomInField()
	if err != nil {
		t.Fatalf("RandomInField: %v", err)
	}
	saltB0, err := core.RandomInField()
	if err != nil {
		t.Fatalf("RandomInField: %v", err)
	}
	saltRevert0, err := core.RandomInField()
	if err != nil {
		t.Fatalf("RandomInField: %v", err)
	}

	bidResult, err := client.AuctionBidProof(core.AuctionBidParams{
		AuctionId:   auctionId,
		Bidder:      bidders[0].sk,
		BidAmount:   bidders[0].amount,
		TokenId:     usdcTokenId,
		SaltIn:      bidders[0].saltIn,
		TreeNumber:  big.NewInt(0),
		MerkleProof: proofs[0],
		BobPk:       bob.PublicKey,
		SaltA:       saltA0,
		SaltB:       saltB0,
		SaltRevert:  saltRevert0,
	})
	if err != nil {
		t.Fatalf("AuctionBid: %v", err)
	}
	bidders[0].saltA = saltA0
	bidders[0].saltB = saltB0
	bidders[0].commitA = bidResult.PublicSignal[4]
	bidders[0].commitB = bidResult.PublicSignal[5]
	t.Logf("AuctionBid bidder 0 OK (amount=%s) — public signals: %d", bidders[0].amount, len(bidResult.PublicSignal))

	// Remaining 251 bidders: compute commitA/commitB directly (no proof —
	// see header comment). AuctionBatch verifies every preimage in-circuit.
	for i := 1; i < numBidders; i++ {
		saltA, err := core.RandomInField()
		if err != nil {
			t.Fatalf("RandomInField: %v", err)
		}
		saltB, err := core.RandomInField()
		if err != nil {
			t.Fatalf("RandomInField: %v", err)
		}
		commitA, err := core.Erc20CommitmentV2(bidders[i].sk.PublicKey, saltA, bidders[i].amount, usdcTokenId)
		if err != nil {
			t.Fatalf("Erc20CommitmentV2 commitA(bidder %d): %v", i, err)
		}
		commitB, err := core.Erc20CommitmentV2(bob.PublicKey, saltB, bidders[i].amount, usdcTokenId)
		if err != nil {
			t.Fatalf("Erc20CommitmentV2 commitB(bidder %d): %v", i, err)
		}
		bidders[i].saltA, bidders[i].saltB = saltA, saltB
		bidders[i].commitA, bidders[i].commitB = commitA, commitB
	}

	bidderByCommitA := make(map[string]*bidderState, numBidders)
	for _, b := range bidders {
		bidderByCommitA[b.commitA.String()] = b
	}

	// ── Phase 1: three AuctionBatch proofs ────────────────────────────────
	type batchWinner struct {
		commit *big.Int
		pk     *big.Int
		amount *big.Int
	}
	var winners []batchWinner

	for batchNum := 0; batchNum*core.AuctionBatchSize < numBidders; batchNum++ {
		t.Logf("── Phase 1: AuctionBatch %d ──────────────────────────────────────────", batchNum)

		start := batchNum * core.AuctionBatchSize
		end := start + core.AuctionBatchSize
		if end > numBidders {
			end = numBidders
		}

		var slots [core.AuctionBatchSize]core.AuctionBatchSlot
		for i := start; i < end; i++ {
			b := bidders[i]
			slots[i-start] = core.AuctionBatchSlot{
				Active:  true,
				CommitA: b.commitA,
				Pk:      b.sk.PublicKey,
				SaltA:   b.saltA,
				Amount:  b.amount,
				TokenId: usdcTokenId,
			}
		}

		batchResult, err := client.AuctionBatchProof(core.AuctionBatchParams{
			AuctionId: auctionId,
			Slots:     slots,
		})
		if err != nil {
			t.Fatalf("AuctionBatch %d: %v", batchNum, err)
		}

		sig := batchResult.PublicSignal
		winnerCommit, winnerPk, winnerAmount := sig[101], sig[102], sig[103]
		winners = append(winners, batchWinner{commit: winnerCommit, pk: winnerPk, amount: winnerAmount})
		t.Logf("AuctionBatch %d OK — winner commitA=%s amount=%s signals=%d",
			batchNum, winnerCommit, winnerAmount, len(sig))
	}

	// ── Phase 2: AuctionFinal ──────────────────────────────────────────────
	t.Log("── Phase 2: AuctionFinal ─────────────────────────────────────────────")

	var batchResults [core.AuctionFinalBatches]core.AuctionFinalBatchResult
	overallIdx := 0
	for i, w := range winners {
		batchResults[i] = core.AuctionFinalBatchResult{Active: true, Commit: w.commit, Pk: w.pk, Amount: w.amount}
		if w.amount.Cmp(winners[overallIdx].amount) > 0 {
			overallIdx = i
		}
	}
	overallWinner := winners[overallIdx]

	overallWinnerBidder := bidderByCommitA[overallWinner.commit.String()]
	if overallWinnerBidder == nil {
		t.Fatalf("could not find bidder for overall winner commitA=%s", overallWinner.commit)
	}

	saltAlice, err := core.RandomInField()
	if err != nil {
		t.Fatalf("RandomInField: %v", err)
	}
	floorPrice := big.NewInt(10) // well below the 252 winning amount, so settlement succeeds

	finalResult, err := client.AuctionFinalProof(core.AuctionFinalParams{
		AuctionId:   auctionId,
		Batches:     batchResults,
		BobPk:       bob.PublicKey,
		SaltB:       overallWinnerBidder.saltB,
		UsdcTokenId: usdcTokenId,
		SaltAlice:   saltAlice,
		NftTokenId:  nftTokenId,
		FloorPrice:  floorPrice,
	})
	if err != nil {
		t.Fatalf("AuctionFinal: %v", err)
	}
	t.Logf("AuctionFinal OK — overall winner commitA=%s amount=%s", overallWinner.commit, overallWinner.amount)
	t.Logf("  public signals: %d elements", len(finalResult.PublicSignal))

	// ── Verify public signal contents ─────────────────────────────────────
	t.Log("── Verification ──────────────────────────────────────────────────────")

	// AuctionFinal public signal layout (38 elements):
	// [0]    StAuctionId
	// [1..10] StBatchWinnerCommit[0..9]
	// [11..20] StBatchWinnerPk[0..9]
	// [21..30] StBatchWinnerAmount[0..9]
	// [31] StOverallWinnerCommit
	// [32] StWinnerPk
	// [33] StWinnerCommitB
	// [34] StWinnerNftCommit
	// [35] StNftTokenId
	// [36] StWinningAmount
	// [37] StFloorPrice

	sig := finalResult.PublicSignal
	if len(sig) != 38 {
		t.Fatalf("AuctionFinal: expected 38 public signals, got %d", len(sig))
	}
	if sig[0].Cmp(auctionId) != 0 {
		t.Errorf("public signal[0] auctionId mismatch: got %s want %s", sig[0], auctionId)
	}
	if sig[31].Cmp(overallWinner.commit) != 0 {
		t.Errorf("public signal[31] overallWinnerCommit mismatch")
	}
	if sig[36].Cmp(overallWinnerBidder.amount) != 0 {
		t.Errorf("public signal[36] winningAmount mismatch: got %s want %s", sig[36], overallWinnerBidder.amount)
	}
	if sig[37].Cmp(floorPrice) != 0 {
		t.Errorf("public signal[37] floorPrice mismatch: got %s want %s", sig[37], floorPrice)
	}
	t.Logf("✓ auctionId, overallWinnerCommit, winningAmount, floorPrice all match expected values")
	t.Logf("✓ 252-bid auction test complete")
}
