package templates

import (
	"github.com/consensys/gnark/frontend"
	"gnark_server/primitives"
)

// AuctionLockCircuitConfig holds compile-time parameters.
type AuctionLockCircuitConfig struct {
	TmMerkleTreeDepth int
}

// AuctionLockCircuit proves that Bob owns an ERC-721 NFT note and locks it
// for an auction.
//
// Bob spends his current NFT note (nullifies it) and creates a new "locked"
// commitment held by the auction contract. The auction ID is derived from the
// locked commitment so it is uniquely bound to this specific NFT lock.
//
// Off-chain Bob generates a fresh WtSaltLocked using ML-KEM or random bytes.
// The contract records stCommitLocked as the locked NFT commitment.
//
// Public statement (6 elements):
//
//	[stAuctionId, stTreeNumber, stMerkleRoot, stNullifier, stCommitLocked, stNftTokenId]
type AuctionLockCircuit struct {
	Config AuctionLockCircuitConfig

	// --- public inputs ---
	StAuctionId    frontend.Variable `gnark:",public"` // Poseidon(commitLocked) — unique auction ID
	StTreeNumber   frontend.Variable `gnark:",public"` // ERC-721 Merkle sub-tree index
	StMerkleRoot   frontend.Variable `gnark:",public"` // current Merkle root
	StNullifier    frontend.Variable `gnark:",public"` // nf_B = Poseidon(sk_B, leafIndex)
	StCommitLocked frontend.Variable `gnark:",public"` // Erc721Commitment(tokenId, pk_B, saltLocked)
	StNftTokenId   frontend.Variable `gnark:",public"` // public tokenId so bidders know what is up for auction

	// --- private witnesses: Bob's input NFT note ---
	WtSpendKey     frontend.Variable   // Bob's spend secret key
	WtTokenId      frontend.Variable   // NFT token ID (constrained == StNftTokenId)
	WtSaltIn       frontend.Variable   // saltB of Bob's current note
	WtPathElements []frontend.Variable // Merkle path (TmMerkleTreeDepth elements)
	WtPathIndex    frontend.Variable   // leaf index in tree

	// --- private witness: auction-locked output ---
	WtSaltLocked frontend.Variable // fresh random salt for the locked commitment
}

func (circuit *AuctionLockCircuit) Define(api frontend.API) error {
	// 1. Derive Bob's spend public key: pk_B = Poseidon(sk_B).
	pkBob := primitives.PublicKey(api, circuit.WtSpendKey)

	// 2. Nullifier: nf_B = Poseidon(sk_B, leafIndex) — burns Bob's original note.
	nullifier := primitives.Nullifier(api, circuit.WtSpendKey, circuit.WtPathIndex)
	api.AssertIsEqual(nullifier, circuit.StNullifier)

	// 3. Bob's current NFT input commitment.
	commitIn := primitives.Erc721Commitment(api, circuit.WtTokenId, pkBob, circuit.WtSaltIn)

	// 4. Merkle membership: commitIn must be a leaf in the current tree.
	pathElems := make([]frontend.Variable, circuit.Config.TmMerkleTreeDepth)
	for j := 0; j < circuit.Config.TmMerkleTreeDepth; j++ {
		pathElems[j] = circuit.WtPathElements[j]
	}
	root := primitives.MerkleProof(api, commitIn, circuit.WtPathIndex, pathElems)
	api.AssertIsEqual(root, circuit.StMerkleRoot)

	// 5. Locked commitment: same owner (pk_B), same tokenId, fresh salt.
	commitLocked := primitives.Erc721Commitment(api, circuit.WtTokenId, pkBob, circuit.WtSaltLocked)
	api.AssertIsEqual(commitLocked, circuit.StCommitLocked)

	// 6. Auction ID derived from the locked commitment — uniquely identifies
	// this auction on-chain without revealing pk_B or saltLocked.
	auctionId := primitives.AuctionId(api, circuit.StCommitLocked)
	api.AssertIsEqual(auctionId, circuit.StAuctionId)

	// 7. Public tokenId must match the private witness — bidders verify what
	// they are bidding on before the commitment preimage is revealed.
	api.AssertIsEqual(circuit.WtTokenId, circuit.StNftTokenId)

	return nil
}
