// Package core implements the Go prover library for the Enygma two-phase ZK
// sealed-bid NFT auction protocol. It builds on top of the shared Enygma DvP
// primitives (Poseidon commitments, nullifiers, Merkle trees, spend keys)
// rather than reimplementing them, and adds the auction-specific high-level
// proof-generation functions in prover_auction.go.
package core

import (
	dvpcore "github.com/raylsnetwork/enygma_dvp/src/core"
)

// ── Re-exported DvP primitives — callers only need to import this package ─────

type (
	SpendKeyPair = dvpcore.SpendKeyPair
	MerkleProof  = dvpcore.MerkleProof
	MerkleTree   = dvpcore.MerkleTree
	GnarkClient  = dvpcore.GnarkClient
)

var (
	NewSpendKeyPair   = dvpcore.NewSpendKeyPair
	NewMerkleTree     = dvpcore.NewMerkleTree
	GetNullifier      = dvpcore.GetNullifier
	GetAuctionId      = dvpcore.GetAuctionId
	Erc721Commitment  = dvpcore.Erc721Commitment
	Erc20CommitmentV2 = dvpcore.Erc20CommitmentV2
	RandomInField     = dvpcore.RandomInField

	// SNARK_SCALAR_FIELD is the BN254 scalar field modulus, re-exported for
	// callers that need to reduce raw randomness before use.
	SNARK_SCALAR_FIELD = dvpcore.SNARK_SCALAR_FIELD
)

// AuctionMerkleDepth is the fixed Merkle tree depth used by the auction
// circuits (AuctionLock, AuctionBid) — matches gnark_circuits' merkleDepth.
const AuctionMerkleDepth = 8

// AuctionClient is an HTTP client for the auction gnark proof server.
type AuctionClient struct {
	*dvpcore.GnarkClient
}

// NewAuctionClient creates an AuctionClient targeting the auction gnark
// server. Defaults to http://localhost:8083 if baseURL is empty — the port
// the auction server listens on (distinct from the DvP server's :8081 and
// the retail payments server's :8082).
func NewAuctionClient(baseURL string) *AuctionClient {
	if baseURL == "" {
		baseURL = "http://localhost:8083"
	}
	return &AuctionClient{GnarkClient: dvpcore.NewGnarkClient(baseURL)}
}
