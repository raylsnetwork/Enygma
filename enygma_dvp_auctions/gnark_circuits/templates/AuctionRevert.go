package templates

import (
	"github.com/consensys/gnark/frontend"
	"gnark_server/primitives"
)

// AuctionRevertCircuit proves that the caller owns the locked NFT note behind
// a given auctionId and mints it back to themselves under a brand-new salt.
//
// commitLocked is never inserted into a Merkle tree — it only ever exists as
// a public scalar recorded by the contract at initAuction() time (and as
// StAuctionId = Poseidon(commitLocked)) — so no Merkle membership proof is
// needed here, just a preimage check.
//
// Critically, StRevertedCommit uses a fresh witness-only salt (WtSaltOut),
// never StCommitLocked or WtSaltLocked themselves. Both of those values are
// already public (emitted in AuctionInitialized), so reusing them as the new
// spendable leaf would permanently and publicly tie the reverted note back to
// this specific canceled auction. Minting fresh keeps the reverted note
// indistinguishable from any other ordinary NFT note.
//
// Public statement (4 elements):
//
//	[StAuctionId, StCommitLocked, StNftTokenId, StRevertedCommit]
type AuctionRevertCircuit struct {
	// --- public inputs ---
	StAuctionId      frontend.Variable `gnark:",public"` // Poseidon(commitLocked); must match auctions[id]
	StCommitLocked   frontend.Variable `gnark:",public"` // recorded by initAuction(); cross-checked by contract
	StNftTokenId     frontend.Variable `gnark:",public"` // recorded by initAuction(); cross-checked by contract
	StRevertedCommit frontend.Variable `gnark:",public"` // new commitment re-inserted into the NFT tree

	// --- private witnesses ---
	WtSpendKey   frontend.Variable // Bob's spend secret key
	WtTokenId    frontend.Variable // NFT token ID (constrained == StNftTokenId)
	WtSaltLocked frontend.Variable // the original salt used to form StCommitLocked at lock time
	WtSaltOut    frontend.Variable // fresh random salt for the reverted output commitment
}

func (circuit *AuctionRevertCircuit) Define(api frontend.API) error {
	// 1. Derive Bob's spend public key: pk_B = Poseidon(sk_B).
	pkBob := primitives.PublicKey(api, circuit.WtSpendKey)

	// 2. Re-derive the locked commitment from its preimage and check it
	// matches the value the contract recorded at initAuction().
	commitLocked := primitives.Erc721Commitment(api, circuit.WtTokenId, pkBob, circuit.WtSaltLocked)
	api.AssertIsEqual(commitLocked, circuit.StCommitLocked)

	// 3. Auction ID consistency.
	auctionId := primitives.AuctionId(api, circuit.StCommitLocked)
	api.AssertIsEqual(auctionId, circuit.StAuctionId)

	// 4. TokenId consistency.
	api.AssertIsEqual(circuit.WtTokenId, circuit.StNftTokenId)

	// 5. Mint the reverted commitment with a fresh salt, same owner/tokenId.
	revertedCommit := primitives.Erc721Commitment(api, circuit.WtTokenId, pkBob, circuit.WtSaltOut)
	api.AssertIsEqual(revertedCommit, circuit.StRevertedCommit)

	// 6. The fresh salt must actually be fresh — guards against accidentally
	// (or maliciously) reusing the already-public WtSaltLocked, which would
	// reintroduce the exact privacy leak this circuit exists to avoid.
	api.AssertIsDifferent(circuit.WtSaltOut, circuit.WtSaltLocked)

	return nil
}
