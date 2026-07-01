package core

import (
	"encoding/json"
	"fmt"
	"math/big"
)

// AuctionProofResult holds the parsed {proof, publicSignal} response from the
// auction gnark server. Unlike the legacy DvP prover functions (which build the
// on-chain statement by hand because their circuits' public-input order differs
// from the contract's calldata order), every auction circuit's public signal is
// already laid out in exactly the order the corresponding EnygmaAuction.sol
// function expects — so PublicSignal can be passed straight through as the
// on-chain `statement` argument.
type AuctionProofResult struct {
	// Proof is the 8-element Groth16 proof [ax,ay,bx1,bx0,by1,by0,cx,cy].
	Proof []*big.Int
	// PublicSignal is the circuit's public statement, in on-chain calldata order.
	PublicSignal []*big.Int
}

// gnarkProofResponse mirrors the {proof, publicSignal} JSON shape returned by
// every /proof/auction* endpoint. json.Number tolerates either quoted-string
// or bare-number serialisation of the underlying *big.Int values.
type gnarkProofResponse struct {
	Proof        []json.Number `json:"proof"`
	PublicSignal []json.Number `json:"publicSignal"`
}

// postAndParse posts payload to endpoint and parses the standard auction
// circuit response shape into an AuctionProofResult.
func postAndParse(client *GnarkClient, endpoint string, payload map[string]interface{}) (*AuctionProofResult, error) {
	body, err := client.PostProof(endpoint, payload)
	if err != nil {
		return nil, err
	}

	var resp gnarkProofResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("%s: unmarshal response: %w (body=%s)", endpoint, err, body)
	}
	if len(resp.Proof) != 8 {
		return nil, fmt.Errorf("%s: expected 8 proof elements, got %d", endpoint, len(resp.Proof))
	}

	proof, err := jsonNumbersToBigInts(resp.Proof)
	if err != nil {
		return nil, fmt.Errorf("%s: proof: %w", endpoint, err)
	}
	signal, err := jsonNumbersToBigInts(resp.PublicSignal)
	if err != nil {
		return nil, fmt.Errorf("%s: publicSignal: %w", endpoint, err)
	}

	return &AuctionProofResult{Proof: proof, PublicSignal: signal}, nil
}

func jsonNumbersToBigInts(ns []json.Number) ([]*big.Int, error) {
	out := make([]*big.Int, len(ns))
	for i, n := range ns {
		v, ok := new(big.Int).SetString(n.String(), 10)
		if !ok {
			return nil, fmt.Errorf("invalid number %q at index %d", n.String(), i)
		}
		out[i] = v
	}
	return out, nil
}

// GetNullifierWithTree computes the nullifier used by the AuctionLock and
// AuctionBid circuits after the Vuln 14/15 fix. It folds the tree number into
// the global leaf index before hashing:
//
//	nf = Poseidon(sk, treeNumber*256 + pathIndex)
//
// Using a global index prevents a proof generated for tree T from being
// replayed under a different StTreeNumber when multiple trees share the same
// Merkle root (e.g., immediately after a tree roll-over).
func GetNullifierWithTree(sk, treeNumber, pathIndex *big.Int) (*big.Int, error) {
	globalIdx := new(big.Int).Add(
		new(big.Int).Mul(treeNumber, big.NewInt(256)),
		pathIndex,
	)
	return GetNullifier(sk, globalIdx)
}

// ── Phase 0a: AuctionLock ───────────────────────────────────────────────────

// AuctionLockParams holds the inputs needed to prove that Bob owns an ERC-721
// NFT note and locks it for auction.
type AuctionLockParams struct {
	Bob         *SpendKeyPair // Bob's spend key pair
	TokenId     *big.Int     // NFT token ID
	SaltIn      *big.Int     // salt of Bob's existing NFT note being spent
	TreeNumber  *big.Int     // NFT Merkle sub-tree index
	MerkleProof *MerkleProof // inclusion proof for Bob's existing NFT note
	SaltLocked  *big.Int     // fresh random salt for the locked commitment
	SaltRevert  *big.Int     // fresh random salt for the revert commitment (≠ SaltLocked)
}

// AuctionLockProof generates a proof for the AuctionLock circuit.
//
// Returns PublicSignal = [StAuctionId, StTreeNumber, StMerkleRoot, StNullifier,
// StCommitLocked, StNftTokenId, StRevertCommit] — exactly the `statement`
// argument initAuction() expects on-chain.
func (c *AuctionClient) AuctionLockProof(p AuctionLockParams) (*AuctionProofResult, error) {
	nullifier, err := GetNullifierWithTree(p.Bob.PrivateKey, p.TreeNumber, p.MerkleProof.Indices)
	if err != nil {
		return nil, fmt.Errorf("nullifier: %w", err)
	}

	commitLocked, err := Erc721Commitment(p.TokenId, p.Bob.PublicKey, p.SaltLocked)
	if err != nil {
		return nil, fmt.Errorf("commitLocked: %w", err)
	}

	auctionId, err := GetAuctionId(commitLocked)
	if err != nil {
		return nil, fmt.Errorf("auctionId: %w", err)
	}

	revertCommit, err := Erc721Commitment(p.TokenId, p.Bob.PublicKey, p.SaltRevert)
	if err != nil {
		return nil, fmt.Errorf("revertCommit: %w", err)
	}

	payload := map[string]interface{}{
		"stAuctionId":    auctionId.String(),
		"stTreeNumber":   p.TreeNumber.String(),
		"stMerkleRoot":   p.MerkleProof.Root.String(),
		"stNullifier":    nullifier.String(),
		"stCommitLocked": commitLocked.String(),
		"stNftTokenId":   p.TokenId.String(),
		"stRevertCommit": revertCommit.String(),
		"wtTreeNumber":   p.TreeNumber.String(),
		"wtSpendKey":     p.Bob.PrivateKey.String(),
		"wtTokenId":      p.TokenId.String(),
		"wtSaltIn":       p.SaltIn.String(),
		"wtPathElements": bigIntSliceToStrings(p.MerkleProof.Elements),
		"wtPathIndex":    p.MerkleProof.Indices.String(),
		"wtSaltLocked":   p.SaltLocked.String(),
		"wtSaltRevert":   p.SaltRevert.String(),
	}

	return postAndParse(c.GnarkClient, "/proof/auctionLock", payload)
}

// ── Phase 0b: AuctionBid ────────────────────────────────────────────────────

// AuctionBidParams holds the inputs needed to prove a sealed all-in USDC bid.
type AuctionBidParams struct {
	AuctionId   *big.Int     // target auction
	Bidder      *SpendKeyPair // bidder's spend key pair
	BidAmount   *big.Int     // bid amount — must equal the input note's full amount (all-in)
	TokenId     *big.Int     // USDC token ID
	SaltIn      *big.Int     // salt of the bidder's existing USDC note being spent
	TreeNumber  *big.Int     // USDC Merkle sub-tree index
	MerkleProof *MerkleProof // inclusion proof for the bidder's existing USDC note
	BobPk       *big.Int     // seller's spend public key (commitB's owner)
	SaltA       *big.Int     // fresh random salt for the locked bid commitment (commitA)
	SaltB       *big.Int     // salt for the seller's payout commitment (commitB) — HKDF(ss, "note salt")
	SaltRevert  *big.Int     // fresh random salt for the revert commitment (≠ SaltA)
}

// AuctionBidProof generates a proof for the AuctionBid circuit.
//
// Returns PublicSignal = [StAuctionId, StTreeNumber, StMerkleRoot, StNullifier,
// StCommitA, StCommitB, StRevertCommit] — exactly the `statement` argument
// submitBid() expects on-chain.
func (c *AuctionClient) AuctionBidProof(p AuctionBidParams) (*AuctionProofResult, error) {
	nullifier, err := GetNullifierWithTree(p.Bidder.PrivateKey, p.TreeNumber, p.MerkleProof.Indices)
	if err != nil {
		return nil, fmt.Errorf("nullifier: %w", err)
	}

	commitA, err := Erc20CommitmentV2(p.Bidder.PublicKey, p.SaltA, p.BidAmount, p.TokenId)
	if err != nil {
		return nil, fmt.Errorf("commitA: %w", err)
	}

	commitB, err := Erc20CommitmentV2(p.BobPk, p.SaltB, p.BidAmount, p.TokenId)
	if err != nil {
		return nil, fmt.Errorf("commitB: %w", err)
	}

	revertCommit, err := Erc20CommitmentV2(p.Bidder.PublicKey, p.SaltRevert, p.BidAmount, p.TokenId)
	if err != nil {
		return nil, fmt.Errorf("revertCommit: %w", err)
	}

	payload := map[string]interface{}{
		"stAuctionId":    p.AuctionId.String(),
		"stTreeNumber":   p.TreeNumber.String(),
		"stMerkleRoot":   p.MerkleProof.Root.String(),
		"stNullifier":    nullifier.String(),
		"stCommitA":      commitA.String(),
		"stCommitB":      commitB.String(),
		"stRevertCommit": revertCommit.String(),
		"wtAuctionId":    p.AuctionId.String(),
		"wtTreeNumber":   p.TreeNumber.String(),
		"wtSpendKey":     p.Bidder.PrivateKey.String(),
		"wtAmount":       p.BidAmount.String(),
		"wtTokenId":      p.TokenId.String(),
		"wtSaltIn":       p.SaltIn.String(),
		"wtPathElements": bigIntSliceToStrings(p.MerkleProof.Elements),
		"wtPathIndex":    p.MerkleProof.Indices.String(),
		"wtSpendPkBob":   p.BobPk.String(),
		"wtSaltA":        p.SaltA.String(),
		"wtSaltB":        p.SaltB.String(),
		"wtBidAmount":    p.BidAmount.String(),
		"wtSaltRevert":   p.SaltRevert.String(),
	}

	return postAndParse(c.GnarkClient, "/proof/auctionBid", payload)
}

// ── Phase 1: AuctionBatch ───────────────────────────────────────────────────

// AuctionBatchSize is the maximum number of bids processed in one Phase-1 proof.
const AuctionBatchSize = 100

// AuctionBatchSlot is one bidder's decrypted bid data within a batch.
// Inactive slots (Active == false) may leave the other fields nil/zero.
type AuctionBatchSlot struct {
	Active  bool
	CommitA *big.Int // on-chain registered commitA for this bid (0 for inactive slots)
	Pk      *big.Int // bidder's spend public key
	SaltA   *big.Int // salt used in commitA's preimage
	Amount  *big.Int // bid amount
	TokenId *big.Int // USDC token ID
}

// AuctionBatchParams holds the inputs needed to prove a Phase-1 batch winner.
// Slots must contain exactly AuctionBatchSize (100) entries; pad unused slots
// with Active: false.
type AuctionBatchParams struct {
	AuctionId *big.Int
	Slots     [AuctionBatchSize]AuctionBatchSlot
}

// AuctionBatchProof generates a proof for the AuctionBatch circuit. The batch
// winner (highest Amount among Active slots) is determined automatically.
//
// Returns PublicSignal = [StAuctionId, StBidCommitA[0..99], StBatchWinnerCommit,
// StBatchWinnerPk, StBatchWinnerAmount] — exactly the `statement` argument
// submitBatch() expects.
func (c *AuctionClient) AuctionBatchProof(p AuctionBatchParams) (*AuctionProofResult, error) {
	winnerIdx := -1
	for i, s := range p.Slots {
		if !s.Active {
			continue
		}
		if winnerIdx == -1 || s.Amount.Cmp(p.Slots[winnerIdx].Amount) > 0 {
			winnerIdx = i
		}
	}
	if winnerIdx == -1 {
		return nil, fmt.Errorf("auctionBatch: no active slots — at least one bid is required")
	}

	stBidCommitA := make([]string, AuctionBatchSize)
	wtActive := make([]string, AuctionBatchSize)
	wtPkBidder := make([]string, AuctionBatchSize)
	wtSaltA := make([]string, AuctionBatchSize)
	wtAmount := make([]string, AuctionBatchSize)
	wtTokenId := make([]string, AuctionBatchSize)

	for i, s := range p.Slots {
		if s.Active {
			stBidCommitA[i] = s.CommitA.String()
			wtActive[i] = "1"
			wtPkBidder[i] = s.Pk.String()
			wtSaltA[i] = s.SaltA.String()
			wtAmount[i] = s.Amount.String()
			wtTokenId[i] = s.TokenId.String()
		} else {
			stBidCommitA[i] = "0"
			wtActive[i] = "0"
			wtPkBidder[i] = "0"
			wtSaltA[i] = "0"
			wtAmount[i] = "0"
			wtTokenId[i] = "0"
		}
	}

	winner := p.Slots[winnerIdx]
	payload := map[string]interface{}{
		"stAuctionId":         p.AuctionId.String(),
		"wtAuctionId":         p.AuctionId.String(),
		"stBidCommitA":        stBidCommitA,
		"stBatchWinnerCommit": winner.CommitA.String(),
		"stBatchWinnerPk":     winner.Pk.String(),
		"stBatchWinnerAmount": winner.Amount.String(),
		"wtActive":            wtActive,
		"wtPkBidder":          wtPkBidder,
		"wtSaltA":             wtSaltA,
		"wtAmount":            wtAmount,
		"wtTokenId":           wtTokenId,
		"wtWinnerIdx":         fmt.Sprintf("%d", winnerIdx),
	}

	return postAndParse(c.GnarkClient, "/proof/auctionBatch", payload)
}

// ── Phase 2: AuctionFinal ───────────────────────────────────────────────────

// AuctionFinalBatches is the maximum number of Phase-1 batch results consumed
// by one Phase-2 final proof.
const AuctionFinalBatches = 10

// AuctionFinalBatchResult is one batch's Phase-1 output. Inactive slots
// (Active == false) must leave Commit/Pk/Amount nil — they are encoded as 0.
type AuctionFinalBatchResult struct {
	Active bool
	Commit *big.Int // batch winner's commitA
	Pk     *big.Int // batch winner's spend public key
	Amount *big.Int // batch winner's bid amount
}

// AuctionFinalParams holds the inputs needed to prove and settle the overall
// auction winner. Batches must contain exactly AuctionFinalBatches (10)
// entries; pad unused slots with Active: false.
type AuctionFinalParams struct {
	AuctionId   *big.Int
	Batches     [AuctionFinalBatches]AuctionFinalBatchResult
	BobPk       *big.Int // seller's spend public key, recovered from the winning bid's ML-KEM decryption
	SaltB       *big.Int // salt for the seller's USDC payout commitment
	UsdcTokenId *big.Int
	SaltAlice   *big.Int // fresh random salt for the winner's new NFT commitment
	NftTokenId  *big.Int
	FloorPrice  *big.Int // seller's reserve price; must match what was recorded at initAuction()
}

// AuctionFinalProof generates a proof for the AuctionFinal circuit. The
// overall winner (highest Amount among Active batches) is determined
// automatically.
//
// Returns PublicSignal = [StAuctionId, StBatchWinnerCommit[0..9],
// StBatchWinnerPk[0..9], StBatchWinnerAmount[0..9], StOverallWinnerCommit,
// StWinnerPk, StWinnerCommitB, StWinnerNftCommit, StNftTokenId,
// StWinningAmount, StFloorPrice] — exactly the `statement` argument
// settle()/settleOptimistic() expect.
func (c *AuctionClient) AuctionFinalProof(p AuctionFinalParams) (*AuctionProofResult, error) {
	winnerIdx := -1
	for i, b := range p.Batches {
		if !b.Active {
			continue
		}
		if winnerIdx == -1 || b.Amount.Cmp(p.Batches[winnerIdx].Amount) > 0 {
			winnerIdx = i
		}
	}
	if winnerIdx == -1 {
		return nil, fmt.Errorf("auctionFinal: no active batches — at least one settled batch is required")
	}
	winner := p.Batches[winnerIdx]

	commitB, err := Erc20CommitmentV2(p.BobPk, p.SaltB, winner.Amount, p.UsdcTokenId)
	if err != nil {
		return nil, fmt.Errorf("commitB: %w", err)
	}

	winnerNftCommit, err := Erc721Commitment(p.NftTokenId, winner.Pk, p.SaltAlice)
	if err != nil {
		return nil, fmt.Errorf("winnerNftCommit: %w", err)
	}

	stBatchWinnerCommit := make([]string, AuctionFinalBatches)
	stBatchWinnerPk := make([]string, AuctionFinalBatches)
	stBatchWinnerAmount := make([]string, AuctionFinalBatches)
	wtBatchActive := make([]string, AuctionFinalBatches)

	for i, b := range p.Batches {
		if b.Active {
			stBatchWinnerCommit[i] = b.Commit.String()
			stBatchWinnerPk[i] = b.Pk.String()
			stBatchWinnerAmount[i] = b.Amount.String()
			wtBatchActive[i] = "1"
		} else {
			stBatchWinnerCommit[i] = "0"
			stBatchWinnerPk[i] = "0"
			stBatchWinnerAmount[i] = "0"
			wtBatchActive[i] = "0"
		}
	}

	payload := map[string]interface{}{
		"stAuctionId":           p.AuctionId.String(),
		"wtAuctionId":           p.AuctionId.String(),
		"stBatchWinnerCommit":   stBatchWinnerCommit,
		"stBatchWinnerPk":       stBatchWinnerPk,
		"stBatchWinnerAmount":   stBatchWinnerAmount,
		"stOverallWinnerCommit": winner.Commit.String(),
		"stWinnerPk":            winner.Pk.String(),
		"stWinnerCommitB":       commitB.String(),
		"stWinnerNftCommit":     winnerNftCommit.String(),
		"stNftTokenId":          p.NftTokenId.String(),
		"stWinningAmount":       winner.Amount.String(),
		"stFloorPrice":          p.FloorPrice.String(),
		"wtBatchActive":         wtBatchActive,
		"wtWinnerBatchIdx":      fmt.Sprintf("%d", winnerIdx),
		"wtPkBob":               p.BobPk.String(),
		"wtSaltB":               p.SaltB.String(),
		"wtUsdcTokenId":         p.UsdcTokenId.String(),
		"wtSaltAlice":           p.SaltAlice.String(),
		"wtNftTokenId":          p.NftTokenId.String(),
	}

	return postAndParse(c.GnarkClient, "/proof/auctionFinal", payload)
}

// ── Phase 3: AuctionRevert ──────────────────────────────────────────────────

// AuctionRevertParams holds the inputs needed to prove that Bob owns the
// locked NFT note behind a canceled auction and reclaim it under a fresh,
// unlinkable commitment.
type AuctionRevertParams struct {
	Bob        *SpendKeyPair
	TokenId    *big.Int
	SaltLocked *big.Int // the original salt used at initAuction() time
	SaltOut    *big.Int // fresh random salt for the reverted output commitment
}

// AuctionRevertProof generates a proof for the AuctionRevert circuit.
//
// Returns PublicSignal = [StAuctionId, StCommitLocked, StNftTokenId,
// StRevertedCommit] — exactly the `statement` argument revertAuction() expects.
func (c *AuctionClient) AuctionRevertProof(p AuctionRevertParams) (*AuctionProofResult, error) {
	commitLocked, err := Erc721Commitment(p.TokenId, p.Bob.PublicKey, p.SaltLocked)
	if err != nil {
		return nil, fmt.Errorf("commitLocked: %w", err)
	}

	auctionId, err := GetAuctionId(commitLocked)
	if err != nil {
		return nil, fmt.Errorf("auctionId: %w", err)
	}

	revertedCommit, err := Erc721Commitment(p.TokenId, p.Bob.PublicKey, p.SaltOut)
	if err != nil {
		return nil, fmt.Errorf("revertedCommit: %w", err)
	}

	payload := map[string]interface{}{
		"stAuctionId":      auctionId.String(),
		"stCommitLocked":   commitLocked.String(),
		"stNftTokenId":     p.TokenId.String(),
		"stRevertedCommit": revertedCommit.String(),
		"wtSpendKey":       p.Bob.PrivateKey.String(),
		"wtTokenId":        p.TokenId.String(),
		"wtSaltLocked":     p.SaltLocked.String(),
		"wtSaltOut":        p.SaltOut.String(),
	}

	return postAndParse(c.GnarkClient, "/proof/auctionRevert", payload)
}

// ── Phase 0c: AuctionWithdraw ───────────────────────────────────────────────

// AuctionWithdrawParams holds the inputs needed to prove that a bidder owns
// the bid commitment they wish to withdraw, before the auction deadline.
type AuctionWithdrawParams struct {
	AuctionId *big.Int
	Bidder    *SpendKeyPair // bidder's spend key pair
	SaltA     *big.Int     // salt used when forming commitA at submitBid() time
	Amount    *big.Int     // USDC amount committed in commitA
	TokenId   *big.Int     // USDC token ID
}

// AuctionWithdrawProof generates a proof for the AuctionWithdraw circuit.
//
// Returns PublicSignal = [StAuctionId, StCommitA] — exactly the `statement`
// argument withdrawBid() expects on-chain.
func (c *AuctionClient) AuctionWithdrawProof(p AuctionWithdrawParams) (*AuctionProofResult, error) {
	commitA, err := Erc20CommitmentV2(p.Bidder.PublicKey, p.SaltA, p.Amount, p.TokenId)
	if err != nil {
		return nil, fmt.Errorf("commitA: %w", err)
	}

	payload := map[string]interface{}{
		"stAuctionId": p.AuctionId.String(),
		"stCommitA":   commitA.String(),
		"wtSpendKey":  p.Bidder.PrivateKey.String(),
		"wtSaltA":     p.SaltA.String(),
		"wtAmount":    p.Amount.String(),
		"wtTokenId":   p.TokenId.String(),
		"wtAuctionId": p.AuctionId.String(),
	}

	return postAndParse(c.GnarkClient, "/proof/auctionWithdraw", payload)
}

// ── internal helpers ─────────────────────────────────────────────────────────

func bigIntSliceToStrings(vals []*big.Int) []string {
	result := make([]string, len(vals))
	for i, v := range vals {
		result[i] = v.String()
	}
	return result
}
