package templates

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/test"
	"github.com/iden3/go-iden3-crypto/poseidon"

	"enygma_gnark_shared/valuebound"
)

const bidTestDepth = 8

// bidHash is Poseidon over xs, failing t on error.
func bidHash(t *testing.T, xs ...*big.Int) *big.Int {
	t.Helper()
	h, err := poseidon.Hash(xs)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// emptyAuctionBid is the AuctionBid circuit at the deployed depth.
func emptyAuctionBid() *AuctionBidCircuit {
	return &AuctionBidCircuit{
		Config:         AuctionBidCircuitConfig{TmMerkleTreeDepth: bidTestDepth},
		WtPathElements: make([]frontend.Variable, bidTestDepth),
	}
}

// auctionBid returns an AuctionBid witness for a bid of amount from a note of
// that amount, against an auction whose seller payout key is sellerPk, with
// commitB paying payTo.
func auctionBid(t *testing.T, amount, sellerPk, payTo *big.Int) *AuctionBidCircuit {
	t.Helper()
	sk := big.NewInt(31337)
	pkA := bidHash(t, sk)
	token := big.NewInt(5)
	saltIn, saltA, saltB, saltRevert := big.NewInt(1), big.NewInt(2), big.NewInt(3), big.NewInt(4)
	root := bidHash(t, pkA, saltIn, amount, token)
	path := make([]frontend.Variable, bidTestDepth)
	for i := range path {
		sib := big.NewInt(int64(500 + i))
		path[i] = sib
		root = bidHash(t, root, sib)
	}
	auctionId, ctxtHash := big.NewInt(77), big.NewInt(88)

	w := emptyAuctionBid()
	w.StAuctionId = auctionId
	w.StTreeNumber = 0
	w.StMerkleRoot = root
	w.StNullifier = bidHash(t, sk, big.NewInt(0))
	w.StCommitA = bidHash(t, pkA, saltA, amount, token)
	w.StCommitB = bidHash(t, payTo, saltB, amount, token)
	w.StRevertCommit = bidHash(t, pkA, saltRevert, amount, token)
	w.StCtxtHash = ctxtHash
	w.StFloorPrice = 1
	w.StSellerPk = sellerPk
	w.WtAuctionId = auctionId
	w.WtTreeNumber = 0
	w.WtSpendKey = sk
	w.WtAmount = amount
	w.WtTokenId = token
	w.WtSaltIn = saltIn
	copy(w.WtPathElements, path)
	w.WtPathIndex = 0
	w.WtSaltA = saltA
	w.WtSaltB = saltB
	w.WtBidAmount = amount
	w.WtSaltRevert = saltRevert
	w.WtCtxtHash = ctxtHash
	return w
}

// TestAuctionBid_CommitBMustPaySellerPk: a bid whose commitB pays the
// auction's seller key solves; one whose commitB pays the bidder (or any key
// but StSellerPk) does not, so the winner cannot be paid their own bid.
func TestAuctionBid_CommitBMustPaySellerPk(t *testing.T) {
	sellerPk := big.NewInt(4242)
	bidderPk := bidHash(t, big.NewInt(31337))
	f := ecc.BN254.ScalarField()

	if err := test.IsSolved(emptyAuctionBid(), auctionBid(t, big.NewInt(100), sellerPk, sellerPk), f); err != nil {
		t.Fatalf("a bid paying the seller's key did not solve: %v", err)
	}
	for name, payTo := range map[string]*big.Int{"the bidder": bidderPk, "another key": big.NewInt(4243)} {
		if err := test.IsSolved(emptyAuctionBid(), auctionBid(t, big.NewInt(100), sellerPk, payTo), f); err == nil {
			t.Fatalf("a bid whose commitB pays %s solved against the seller's key", name)
		}
	}
}

// TestAuctionBid_AmountBound: a bid of 10^36 - 1 solves; 0, 10^36 and p - 1
// do not.
func TestAuctionBid_AmountBound(t *testing.T) {
	sellerPk := big.NewInt(4242)
	f := ecc.BN254.ScalarField()
	top := new(big.Int).Sub(valuebound.Bound(), big.NewInt(1))
	if err := test.IsSolved(emptyAuctionBid(), auctionBid(t, top, sellerPk, sellerPk), f); err != nil {
		t.Fatalf("a 10^36 - 1 bid did not solve: %v", err)
	}
	for _, v := range []*big.Int{big.NewInt(0), valuebound.Bound(), new(big.Int).Sub(f, big.NewInt(1))} {
		if err := test.IsSolved(emptyAuctionBid(), auctionBid(t, v, sellerPk, sellerPk), f); err == nil {
			t.Fatalf("a bid of %s solved", v)
		}
	}
}
