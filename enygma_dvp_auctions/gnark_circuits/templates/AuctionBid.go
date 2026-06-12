package templates

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/cmp"
	"gnark_server/primitives"
)

// AuctionBidCircuitConfig holds compile-time parameters.
type AuctionBidCircuitConfig struct {
	TmMerkleTreeDepth int
	TmRange           frontend.Variable // upper bound for range checks (e.g. 2^128)
}

// AuctionBidCircuit proves that Alice owns a USDC note, nullifies it, and
// creates two output commitments:
//
//   - commitA = Erc20CommitmentV2(pk_A, saltA, bidAmount, tokenId)
//     Alice's locked bid — held by the auction contract. Released back to
//     Alice if she loses; released to Bob as payment if she wins.
//
//   - commitB = Erc20CommitmentV2(pk_B, saltB, bidAmount, tokenId)
//     Bob's USDC destination — becomes spendable by Bob after auction
//     settlement. saltB = HKDF(ss, "note salt") where ss is from
//     ML-KEM.Encapsulate(pk_auction), so only the auctioneer can read the bid.
//
// The bid is all-in: bidAmount == Alice's full note amount (no change output).
// The auctioneer decrypts ctxt_1/ctxt_2 (submitted alongside the proof) to
// verify commitB's preimage and determine the winning bid.
//
// Public statement (6 elements):
//
//	[stAuctionId, stTreeNumber, stMerkleRoot, stNullifier, stCommitA, stCommitB]
type AuctionBidCircuit struct {
	Config AuctionBidCircuitConfig

	// --- public inputs ---
	StAuctionId  frontend.Variable `gnark:",public"` // identifies which auction this bid targets
	StTreeNumber frontend.Variable `gnark:",public"` // Alice's USDC Merkle sub-tree index
	StMerkleRoot frontend.Variable `gnark:",public"` // current Merkle root
	StNullifier  frontend.Variable `gnark:",public"` // nf_A = Poseidon(sk_A, leafIndex)
	StCommitA    frontend.Variable `gnark:",public"` // Alice's locked bid commitment
	StCommitB    frontend.Variable `gnark:",public"` // Bob's USDC payout destination commitment

	// --- private witnesses: Alice's input USDC note ---
	WtSpendKey     frontend.Variable   // Alice's spend secret key
	WtAmount       frontend.Variable   // USDC amount in Alice's note (== bidAmount, all-in)
	WtTokenId      frontend.Variable   // USDC token ID
	WtSaltIn       frontend.Variable   // saltB of Alice's current note
	WtPathElements []frontend.Variable // Merkle path (TmMerkleTreeDepth elements)
	WtPathIndex    frontend.Variable   // leaf index in tree

	// --- private witnesses: output notes ---
	WtSpendPkBob frontend.Variable // Bob's spend public key (from his registration)
	WtSaltA      frontend.Variable // salt for commitA (fresh random)
	WtSaltB      frontend.Variable // salt for commitB = HKDF(ss, "note salt")
	WtBidAmount  frontend.Variable // bid amount — constrained == WtAmount (all-in)
}

func (circuit *AuctionBidCircuit) Define(api frontend.API) error {
	// 1. Derive Alice's spend public key: pk_A = Poseidon(sk_A).
	pkAlice := primitives.PublicKey(api, circuit.WtSpendKey)

	// 2. Nullifier: nf_A = Poseidon(sk_A, leafIndex) — burns Alice's USDC note.
	nullifier := primitives.Nullifier(api, circuit.WtSpendKey, circuit.WtPathIndex)
	api.AssertIsEqual(nullifier, circuit.StNullifier)

	// 3. Alice's current USDC input commitment.
	commitIn := primitives.Erc20CommitmentV2(api, pkAlice, circuit.WtSaltIn, circuit.WtAmount, circuit.WtTokenId)

	// 4. Merkle membership.
	pathElems := make([]frontend.Variable, circuit.Config.TmMerkleTreeDepth)
	for j := 0; j < circuit.Config.TmMerkleTreeDepth; j++ {
		pathElems[j] = circuit.WtPathElements[j]
	}
	root := primitives.MerkleProof(api, commitIn, circuit.WtPathIndex, pathElems)
	api.AssertIsEqual(root, circuit.StMerkleRoot)

	// 5. commitA: Alice's locked bid — she remains the owner (pk_A), fresh salt.
	commitA := primitives.Erc20CommitmentV2(api, pkAlice, circuit.WtSaltA, circuit.WtBidAmount, circuit.WtTokenId)
	api.AssertIsEqual(commitA, circuit.StCommitA)

	// 6. commitB: Bob's USDC payout destination — saltB derived from ML-KEM ss.
	commitB := primitives.Erc20CommitmentV2(api, circuit.WtSpendPkBob, circuit.WtSaltB, circuit.WtBidAmount, circuit.WtTokenId)
	api.AssertIsEqual(commitB, circuit.StCommitB)

	// 7. All-in bid: bid amount must equal Alice's full note amount (no change).
	api.AssertIsEqual(circuit.WtBidAmount, circuit.WtAmount)

	// 8. Range check: 0 < bidAmount < TmRange (prevents zero bids and overflow).
	isPositive := cmp.IsLess(api, 0, circuit.WtBidAmount)
	api.AssertIsEqual(isPositive, 1)

	isInRange := cmp.IsLess(api, circuit.WtBidAmount, circuit.Config.TmRange)
	api.AssertIsEqual(isInRange, 1)

	return nil
}
