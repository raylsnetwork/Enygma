package tests

// Integration test for the auction cancel/revert path: Bob locks an NFT,
// nobody ever bids, and Bob proves ownership of the locked commitment to
// reclaim it under a brand-new, unlinkable salt — exercising the privacy
// property the revert circuit exists to provide (see AuctionRevert.go):
// the reverted commitment must never reuse the already-public commitLocked
// value, or the reclaimed note would be permanently linkable back to this
// specific canceled auction.
//
// Prerequisites: auction gnark server running on :8083 (see 01_auction_252_bids_test.go).
//
// Run with:
//   cd test && CC=/usr/bin/clang go test -run TestAuction_Revert -v -timeout 120s

import (
	"math/big"
	"testing"

	"github.com/raylsnetwork/enygma_dvp_auctions/src/core"
)

func TestAuction_Revert(t *testing.T) {
	if !serverAvailable(serverAddr) {
		t.Skip("auction gnark server not running on " + serverAddr + " — skipping")
	}

	client := core.NewAuctionClient("")
	nftTokenId := big.NewInt(888)

	// ── Bob locks his NFT ──────────────────────────────────────────────────
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
		t.Fatalf("GenerateProof: %v", err)
	}

	saltLocked, err := core.RandomInField()
	if err != nil {
		t.Fatalf("RandomInField: %v", err)
	}
	saltRevert, err := core.RandomInField()
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
		SaltRevert:  saltRevert,
	})
	if err != nil {
		t.Fatalf("AuctionLock: %v", err)
	}

	auctionId := lockResult.PublicSignal[0]
	commitLocked := lockResult.PublicSignal[4]
	t.Logf("auctionId    = %s", auctionId)
	t.Logf("commitLocked = %s", commitLocked)

	// ── Nobody ever bids — Bob reverts ────────────────────────────────────
	saltOut, err := core.RandomInField()
	if err != nil {
		t.Fatalf("RandomInField: %v", err)
	}

	revertResult, err := client.AuctionRevertProof(core.AuctionRevertParams{
		Bob:        bob,
		TokenId:    nftTokenId,
		SaltLocked: saltLocked,
		SaltOut:    saltOut,
	})
	if err != nil {
		t.Fatalf("AuctionRevert: %v", err)
	}

	// AuctionRevert public signal layout (4 elements):
	// [0] StAuctionId  [1] StCommitLocked  [2] StNftTokenId  [3] StRevertedCommit
	sig := revertResult.PublicSignal
	if len(sig) != 4 {
		t.Fatalf("expected 4 public signals, got %d", len(sig))
	}
	if sig[0].Cmp(auctionId) != 0 {
		t.Errorf("public signal[0] auctionId mismatch: got %s want %s", sig[0], auctionId)
	}
	if sig[1].Cmp(commitLocked) != 0 {
		t.Errorf("public signal[1] commitLocked mismatch: got %s want %s", sig[1], commitLocked)
	}
	if sig[2].Cmp(nftTokenId) != 0 {
		t.Errorf("public signal[2] nftTokenId mismatch: got %s want %s", sig[2], nftTokenId)
	}
	revertedCommit := sig[3]

	// Privacy property: the reverted commitment must NOT equal the already
	// publicly-known commitLocked value.
	if revertedCommit.Cmp(commitLocked) == 0 {
		t.Fatalf("revertedCommit must differ from commitLocked (privacy leak), got equal value %s", revertedCommit)
	}

	// The reverted commitment must independently re-derive from (tokenId, bob.PublicKey, saltOut).
	wantReverted, err := core.Erc721Commitment(nftTokenId, bob.PublicKey, saltOut)
	if err != nil {
		t.Fatalf("Erc721Commitment: %v", err)
	}
	if revertedCommit.Cmp(wantReverted) != 0 {
		t.Errorf("revertedCommit mismatch: got %s want %s", revertedCommit, wantReverted)
	}

	t.Logf("✓ revertedCommit = %s (fresh, unlinkable to commitLocked)", revertedCommit)
	t.Logf("✓ auction revert test complete")
}
