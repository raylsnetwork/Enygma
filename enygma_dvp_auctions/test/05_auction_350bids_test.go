package tests

// End-to-end integration test: 350 bids across 4 batches.
//
// Scenario:
//   Bob locks an ERC-721 NFT (tokenId=42, floorPrice=50).
//   350 bidders each submit a USDC bid:
//     - 349 bidders bid 100 USDC
//     - Bidder at index 150 bids 500 USDC (the clear winner, in batch 2)
//   Auctioneer runs 4 Phase-1 batches (batch capacity = 100):
//     batch 1: bids   0..99  → winner = bidder 0   (amount 100, first in batch)
//     batch 2: bids 100..199 → winner = bidder 150  (amount 500)
//     batch 3: bids 200..299 → winner = bidder 200  (amount 100, first in batch)
//     batch 4: bids 300..349 → winner = bidder 300  (amount 100, first in batch; 50 inactive slots)
//   Auctioneer runs Phase-2 final → overall winner = bidder 150 (500 > 100).
//   Assertions: SETTLED, overallWinnerCommit == bidder150.commitA, bidCount == 350.

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"

	"github.com/raylsnetwork/enygma_dvp_auctions/src/core"
)

const (
	numBids350   = 350
	winnerIdx350 = 150 // bidder 150 bids 500; all others bid 100
	winnerAmt350 = 500
	loserAmt350  = 100
)

func TestAuction_OnChain_350Bids(t *testing.T) {
	if !serverAvailable(serverAddr) {
		t.Skipf("auction gnark server not running on %s — skipping", serverAddr)
	}
	if !tcpAvailable("localhost:8545") {
		t.Skip("Hardhat node not running on localhost:8545 — skipping")
	}

	// ─── Setup ────────────────────────────────────────────────────────────────

	projectRoot := findProjectRoot(t)
	receipts    := loadOnChainReceipts(t, projectRoot)
	ethClient   := dialEth(t)
	defer ethClient.Close()

	chainID, _ := new(big.Int).SetString(onchainChain, 10)
	ownerAuth   := makeAuth(t, ownerPrivKey, chainID)

	auctionABI := loadABI(t, projectRoot, "core/contracts/EnygmaAuction.sol/EnygmaAuction")
	vaultABI   := loadABI(t, projectRoot, "core/contracts/AuctionCoinVault.sol/AuctionCoinVault")

	auctionContract := bind.NewBoundContract(
		common.HexToAddress(receipts["EnygmaAuction"]),
		auctionABI, ethClient, ethClient, ethClient,
	)
	nftVaultContract := bind.NewBoundContract(
		common.HexToAddress(receipts["NftVault"]),
		vaultABI, ethClient, ethClient, ethClient,
	)
	usdcVaultContract := bind.NewBoundContract(
		common.HexToAddress(receipts["UsdcVault"]),
		vaultABI, ethClient, ethClient, ethClient,
	)

	// Guard: both vaults must be empty (fresh deployment).
	emptyRoot := core.NewMerkleTree(core.AuctionMerkleDepth).Root()
	if cur := vaultRoot(t, nftVaultContract); cur.Cmp(emptyRoot) != 0 {
		t.Skipf("NftVault is not empty (root=%s) — redeploy contracts on a fresh Hardhat node first", cur)
	}
	if cur := vaultRoot(t, usdcVaultContract); cur.Cmp(emptyRoot) != 0 {
		t.Skipf("UsdcVault is not empty (root=%s) — redeploy contracts on a fresh Hardhat node first", cur)
	}

	gnarkClient := core.NewAuctionClient("")

	nftTokenId  := big.NewInt(onchainNftTokenId)
	usdcTokenId := big.NewInt(onchainUsdcTokenId)
	floorPrice  := big.NewInt(onchainFloorPrice)

	// ─── Bob (NFT seller) ─────────────────────────────────────────────────────

	var err error
	bob, err := core.NewSpendKeyPair()
	checkErr(t, "NewSpendKeyPair(bob)", err)

	// ─── 350 bidders ──────────────────────────────────────────────────────────

	t.Logf("Generating %d bidder key pairs...", numBids350)
	bidders := make([]*core.SpendKeyPair, numBids350)
	for i := range bidders {
		bidders[i], err = core.NewSpendKeyPair()
		checkErr(t, fmt.Sprintf("NewSpendKeyPair(bidder %d)", i), err)
	}

	// ─── NFT commitment ───────────────────────────────────────────────────────

	bobSaltIn, err := core.RandomInField()
	checkErr(t, "RandomInField(bobSaltIn)", err)

	bobNftCommitIn, err := core.Erc721Commitment(nftTokenId, bob.PublicKey, bobSaltIn)
	checkErr(t, "Erc721Commitment", err)

	saltLocked, err := core.RandomInField()
	checkErr(t, "RandomInField(saltLocked)", err)
	saltRevertBob, err := core.RandomInField()
	checkErr(t, "RandomInField(saltRevertBob)", err)

	// ─── Seed NFT note ────────────────────────────────────────────────────────

	t.Log("Seeding Bob's NFT note into NftVault...")
	seedTx, err := nftVaultContract.Transact(ownerAuth, "seedDeposit", bobNftCommitIn)
	checkErr(t, "seedDeposit(nft)", err)
	waitTx(t, ethClient, seedTx)

	nftRoot := vaultRoot(t, nftVaultContract)
	nftTree := core.NewMerkleTree(core.AuctionMerkleDepth)
	nftTree.InsertLeaf(bobNftCommitIn)
	nftProof, err := nftTree.GenerateProof(bobNftCommitIn)
	checkErr(t, "GenerateProof(nft)", err)

	if nftProof.Root.Cmp(nftRoot) != 0 {
		t.Fatalf("NFT local tree root %s ≠ vault root %s", nftProof.Root, nftRoot)
	}

	// ─── USDC commitments for all 350 bidders ────────────────────────────────

	saltsIn     := make([]*big.Int, numBids350)
	amounts     := make([]*big.Int, numBids350)
	usdcCommits := make([]*big.Int, numBids350)

	for i := 0; i < numBids350; i++ {
		saltsIn[i], err = core.RandomInField()
		checkErr(t, fmt.Sprintf("RandomInField(saltIn %d)", i), err)

		amounts[i] = big.NewInt(loserAmt350)
		if i == winnerIdx350 {
			amounts[i] = big.NewInt(winnerAmt350)
		}

		usdcCommits[i], err = core.Erc20CommitmentV2(bidders[i].PublicKey, saltsIn[i], amounts[i], usdcTokenId)
		checkErr(t, fmt.Sprintf("Erc20CommitmentV2(bidder %d)", i), err)
	}

	// ─── Seed all 350 USDC notes ──────────────────────────────────────────────

	t.Logf("Seeding %d USDC notes into UsdcVault...", numBids350)
	for i := 0; i < numBids350; i++ {
		tx, err := usdcVaultContract.Transact(ownerAuth, "seedDeposit", usdcCommits[i])
		checkErr(t, fmt.Sprintf("seedDeposit(usdc %d)", i), err)
		waitTx(t, ethClient, tx)
		if (i+1)%50 == 0 {
			t.Logf("  seeded %d/%d", i+1, numBids350)
		}
	}

	usdcRoot := vaultRoot(t, usdcVaultContract)

	// Build local USDC tree, generate Merkle proofs for all bidders.
	usdcTree := core.NewMerkleTree(core.AuctionMerkleDepth)
	for i := 0; i < numBids350; i++ {
		usdcTree.InsertLeaf(usdcCommits[i])
	}
	usdcProofs := make([]*core.MerkleProof, numBids350)
	for i := 0; i < numBids350; i++ {
		usdcProofs[i], err = usdcTree.GenerateProof(usdcCommits[i])
		checkErr(t, fmt.Sprintf("GenerateProof(usdc %d)", i), err)
	}

	if usdcProofs[0].Root.Cmp(usdcRoot) != 0 {
		t.Fatalf("USDC local tree root %s ≠ vault root %s", usdcProofs[0].Root, usdcRoot)
	}

	// ─── Phase 0a: initAuction ────────────────────────────────────────────────

	t.Log("── Phase 0a: AuctionLock / initAuction ──────────────────────────────────")

	lockResult, err := gnarkClient.AuctionLockProof(core.AuctionLockParams{
		Bob:         bob,
		TokenId:     nftTokenId,
		SaltIn:      bobSaltIn,
		TreeNumber:  big.NewInt(0),
		MerkleProof: nftProof,
		SaltLocked:  saltLocked,
		SaltRevert:  saltRevertBob,
	})
	checkErr(t, "AuctionLockProof", err)

	auctionId := lockResult.PublicSignal[0]
	t.Logf("auctionId = %s", auctionId)

	header, err := ethClient.HeaderByNumber(context.Background(), nil)
	checkErr(t, "HeaderByNumber", err)
	deadline           := new(big.Int).Add(new(big.Int).SetUint64(header.Time), big.NewInt(3600))
	settlementDeadline := new(big.Int).Add(deadline, big.NewInt(7200))

	initTx, err := auctionContract.Transact(ownerAuth, "initAuction",
		toBigArr8(lockResult.Proof), toBigArr7(lockResult.PublicSignal), deadline, settlementDeadline, floorPrice,
	)
	checkErr(t, "initAuction tx", err)
	waitTx(t, ethClient, initTx)
	t.Log("initAuction: OK")

	// ─── Phase 0b: 350 bids ───────────────────────────────────────────────────

	t.Logf("── Phase 0b: Submitting %d bids ─────────────────────────────────────────", numBids350)

	type bidRecord struct {
		commitA *big.Int
		commitB *big.Int
		saltA   *big.Int
		saltB   *big.Int
	}
	bidsData := make([]bidRecord, numBids350)

	for i := 0; i < numBids350; i++ {
		saltA, err := core.RandomInField()
		checkErr(t, fmt.Sprintf("RandomInField(saltA %d)", i), err)
		saltB, err := core.RandomInField()
		checkErr(t, fmt.Sprintf("RandomInField(saltB %d)", i), err)
		saltRevert, err := core.RandomInField()
		checkErr(t, fmt.Sprintf("RandomInField(saltRevert %d)", i), err)

		bidResult, err := gnarkClient.AuctionBidProof(core.AuctionBidParams{
			AuctionId:   auctionId,
			Bidder:      bidders[i],
			BidAmount:   amounts[i],
			TokenId:     usdcTokenId,
			SaltIn:      saltsIn[i],
			TreeNumber:  big.NewInt(0),
			MerkleProof: usdcProofs[i],
			BobPk:       bob.PublicKey,
			SaltA:       saltA,
			SaltB:       saltB,
			SaltRevert:  saltRevert,
		})
		checkErr(t, fmt.Sprintf("AuctionBidProof(bidder %d)", i), err)

		bidsData[i] = bidRecord{
			commitA: bidResult.PublicSignal[4],
			commitB: bidResult.PublicSignal[5],
			saltA:   saltA,
			saltB:   saltB,
		}

		bidTx, err := auctionContract.Transact(ownerAuth, "submitBid",
			toBigArr8(bidResult.Proof),
			toBigArr7(bidResult.PublicSignal),
			[]byte("dummy-mlkem-capsule"),
			[]byte("dummy-aead-ciphertext"),
		)
		checkErr(t, fmt.Sprintf("submitBid tx (bidder %d)", i), err)
		waitTx(t, ethClient, bidTx)

		if (i+1)%50 == 0 {
			t.Logf("  submitted bid %d/%d", i+1, numBids350)
		}
	}
	t.Logf("All %d bids submitted", numBids350)

	// ─── Advance time past deadline ───────────────────────────────────────────

	t.Log("Advancing time past auction deadline...")
	rpcClient := ethClient.Client()
	var timeResult interface{}
	if err := rpcClient.Call(&timeResult, "evm_increaseTime", 7200); err != nil {
		t.Fatalf("evm_increaseTime: %v", err)
	}
	if err := rpcClient.Call(&timeResult, "evm_mine"); err != nil {
		t.Fatalf("evm_mine: %v", err)
	}

	// ─── Phase 1: 4 batch proofs ──────────────────────────────────────────────

	numBatches := (numBids350 + core.AuctionBatchSize - 1) / core.AuctionBatchSize // = 4
	t.Logf("── Phase 1: %d AuctionBatch proofs ──────────────────────────────────────", numBatches)

	type batchRecord struct {
		id     *big.Int
		commit *big.Int
		pk     *big.Int
		amount *big.Int
	}
	batchRecords := make([]batchRecord, numBatches)

	for b := 0; b < numBatches; b++ {
		start := b * core.AuctionBatchSize
		end   := start + core.AuctionBatchSize
		if end > numBids350 {
			end = numBids350
		}

		var slots [core.AuctionBatchSize]core.AuctionBatchSlot
		for i := start; i < end; i++ {
			slots[i-start] = core.AuctionBatchSlot{
				Active:  true,
				CommitA: bidsData[i].commitA,
				Pk:      bidders[i].PublicKey,
				SaltA:   bidsData[i].saltA,
				Amount:  amounts[i],
				TokenId: usdcTokenId,
			}
		}
		// Remaining slots are zero (inactive) — zero-valued by default.

		batchResult, err := gnarkClient.AuctionBatchProof(core.AuctionBatchParams{
			AuctionId: auctionId,
			Slots:     slots,
		})
		checkErr(t, fmt.Sprintf("AuctionBatchProof(batch %d)", b+1), err)

		// Public signal layout: [auctionId, bidCommitA[0..99], batchWinnerCommit, batchWinnerPk, batchWinnerAmount]
		batchWinnerCommit := batchResult.PublicSignal[101]
		batchWinnerPk     := batchResult.PublicSignal[102]
		batchWinnerAmount := batchResult.PublicSignal[103]
		t.Logf("  batch %d winner: commitA=%s amount=%s", b+1, batchWinnerCommit, batchWinnerAmount)

		batchId := big.NewInt(int64(b + 1))
		batchTx, err := auctionContract.Transact(ownerAuth, "submitBatch",
			batchId, toBigArr8(batchResult.Proof), toBigArr104(batchResult.PublicSignal),
		)
		checkErr(t, fmt.Sprintf("submitBatch tx (batch %d)", b+1), err)
		waitTx(t, ethClient, batchTx)
		t.Logf("  batch %d: OK", b+1)

		batchRecords[b] = batchRecord{
			id:     batchId,
			commit: batchWinnerCommit,
			pk:     batchWinnerPk,
			amount: batchWinnerAmount,
		}
	}

	// ─── Phase 2: AuctionFinal / settle ──────────────────────────────────────

	t.Log("── Phase 2: AuctionFinal / settle ───────────────────────────────────────")

	var finalBatches [core.AuctionFinalBatches]core.AuctionFinalBatchResult
	for b := 0; b < numBatches; b++ {
		finalBatches[b] = core.AuctionFinalBatchResult{
			Active: true,
			Commit: batchRecords[b].commit,
			Pk:     batchRecords[b].pk,
			Amount: batchRecords[b].amount,
		}
	}

	// Winner is bidder 150 (batch 2). Their saltB is bidsData[winnerIdx350].saltB.
	saltAlice, err := core.RandomInField()
	checkErr(t, "RandomInField(saltAlice)", err)

	finalResult, err := gnarkClient.AuctionFinalProof(core.AuctionFinalParams{
		AuctionId:   auctionId,
		Batches:     finalBatches,
		BobPk:       bob.PublicKey,
		SaltB:       bidsData[winnerIdx350].saltB,
		UsdcTokenId: usdcTokenId,
		SaltAlice:   saltAlice,
		NftTokenId:  nftTokenId,
		FloorPrice:  floorPrice,
	})
	checkErr(t, "AuctionFinalProof", err)

	overallWinnerCommit := finalResult.PublicSignal[31]
	winningAmount       := finalResult.PublicSignal[36]
	t.Logf("  overallWinnerCommit = %s", overallWinnerCommit)
	t.Logf("  winningAmount       = %s", winningAmount)

	var batchIds [10]*big.Int
	for b := 0; b < numBatches; b++ {
		batchIds[b] = batchRecords[b].id
	}
	for b := numBatches; b < 10; b++ {
		batchIds[b] = big.NewInt(0)
	}

	settleTx, err := auctionContract.Transact(ownerAuth, "settleOptimistic",
		toBigArr8(finalResult.Proof), toBigArr38(finalResult.PublicSignal), batchIds,
	)
	checkErr(t, "settleOptimistic tx", err)
	waitTx(t, ethClient, settleTx)
	t.Log("settleOptimistic: OK — challenge window started")

	// Advance time past the 1-day challenge window.
	var challengeTimeResult interface{}
	if err := rpcClient.Call(&challengeTimeResult, "evm_increaseTime", 86401); err != nil {
		t.Fatalf("evm_increaseTime (challenge window): %v", err)
	}
	if err := rpcClient.Call(&challengeTimeResult, "evm_mine"); err != nil {
		t.Fatalf("evm_mine: %v", err)
	}

	finalizeTx, err := auctionContract.Transact(ownerAuth, "finalizeSettlement", auctionId)
	checkErr(t, "finalizeSettlement tx", err)
	waitTx(t, ethClient, finalizeTx)
	t.Log("finalizeSettlement: OK")

	// ─── Verify final state ───────────────────────────────────────────────────

	t.Log("── Verification ──────────────────────────────────────────────────────────")

	var stateResults []interface{}
	err = auctionContract.Call(&bind.CallOpts{}, &stateResults, "getAuctionState", auctionId)
	checkErr(t, "getAuctionState", err)
	state := stateResults[0].(uint8)
	if state != 3 {
		t.Errorf("expected SETTLED (3), got %d", state)
	} else {
		t.Logf("✓ AuctionState = SETTLED (%d)", state)
	}

	var coreResults []interface{}
	err = auctionContract.Call(&bind.CallOpts{}, &coreResults, "getAuctionCore", auctionId)
	checkErr(t, "getAuctionCore", err)
	// fields: state, nftTokenId, commitLocked, revertCommit, overallWinnerCommit,
	//         batchCount, deadline, settlementDeadline, floorPrice, bidCount
	onChainWinner   := coreResults[4].(*big.Int)
	onChainBidCount := coreResults[9].(*big.Int)
	t.Logf("  overallWinnerCommit = %s", onChainWinner)
	t.Logf("  bidCount            = %s", onChainBidCount)

	if onChainWinner.Cmp(bidsData[winnerIdx350].commitA) != 0 {
		t.Errorf("overallWinnerCommit mismatch: got %s, want bidder%d's commitA %s",
			onChainWinner, winnerIdx350, bidsData[winnerIdx350].commitA)
	} else {
		t.Logf("✓ overallWinnerCommit == bidder %d's commitA (highest bid: %d USDC)", winnerIdx350, winnerAmt350)
	}

	if onChainBidCount.Cmp(big.NewInt(numBids350)) != 0 {
		t.Errorf("bidCount mismatch: got %s, want %d", onChainBidCount, numBids350)
	} else {
		t.Logf("✓ bidCount == %d", numBids350)
	}

	if winningAmount.Cmp(big.NewInt(winnerAmt350)) != 0 {
		t.Errorf("winningAmount mismatch: got %s, want %d", winningAmount, winnerAmt350)
	} else {
		t.Logf("✓ winningAmount == %d USDC", winnerAmt350)
	}

	t.Logf("✓ 350-bid on-chain auction test complete")
}
