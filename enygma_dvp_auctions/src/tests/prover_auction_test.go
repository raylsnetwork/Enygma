package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/raylsnetwork/enygma_dvp_auctions/src/core"
)

// --- Helpers: mock gnark auction server ---

// mockAuctionServer returns numSignals dummy public-signal values and a valid
// 8-element dummy proof, regardless of the request body.
func mockAuctionServer(t *testing.T, expectedPath string, numSignals int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != expectedPath {
			t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
		}
		writeProofResponse(w, numSignals)
	}))
}

// mockAuctionServerCapture is like mockAuctionServer but also captures the
// request payload so the test can assert on individual fields.
func mockAuctionServerCapture(t *testing.T, expectedPath string, numSignals int) (*httptest.Server, *map[string]interface{}) {
	t.Helper()
	var captured map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != expectedPath {
			t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &captured); err != nil {
			t.Fatalf("failed to unmarshal captured payload: %v", err)
		}
		writeProofResponse(w, numSignals)
	}))
	return server, &captured
}

func writeProofResponse(w http.ResponseWriter, numSignals int) {
	proof := make([]string, 8)
	for i := range proof {
		proof[i] = fmt.Sprintf("%d", i+1)
	}
	signal := make([]string, numSignals)
	for i := range signal {
		signal[i] = fmt.Sprintf("%d", i+1000)
	}
	w.WriteHeader(200)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"proof": proof, "publicSignal": signal})
}

// --- Helpers: fixtures ---

func makeMerkleProof(depth int) *core.MerkleProof {
	elements := make([]*big.Int, depth)
	for i := range elements {
		elements[i] = big.NewInt(int64(i + 100))
	}
	return &core.MerkleProof{
		Element:  big.NewInt(42),
		Elements: elements,
		Indices:  big.NewInt(3),
		Root:     big.NewInt(999),
	}
}

func makeSpendKeyPair(priv, pub int64) *core.SpendKeyPair {
	return &core.SpendKeyPair{PrivateKey: big.NewInt(priv), PublicKey: big.NewInt(pub)}
}

// --- AuctionLockProof tests ---

func TestAuctionLockProof_Success(t *testing.T) {
	server := mockAuctionServer(t, "/proof/auctionLock", 7)
	defer server.Close()

	client := core.NewAuctionClient(server.URL)
	result, err := client.AuctionLockProof(core.AuctionLockParams{
		Bob:         makeSpendKeyPair(10, 20),
		TokenId:     big.NewInt(77),
		SaltIn:      big.NewInt(5),
		TreeNumber:  big.NewInt(0),
		MerkleProof: makeMerkleProof(core.AuctionMerkleDepth),
		SaltLocked:  big.NewInt(123),
		SaltRevert:  big.NewInt(456),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Proof) != 8 {
		t.Errorf("expected 8 proof elements, got %d", len(result.Proof))
	}
	if len(result.PublicSignal) != 7 {
		t.Errorf("expected 7 public signal elements, got %d", len(result.PublicSignal))
	}
}

func TestAuctionLockProof_PayloadFields(t *testing.T) {
	server, captured := mockAuctionServerCapture(t, "/proof/auctionLock", 7)
	defer server.Close()

	client := core.NewAuctionClient(server.URL)
	bob := makeSpendKeyPair(10, 20)
	tokenId := big.NewInt(77)
	saltLocked := big.NewInt(123)
	saltRevert := big.NewInt(456)

	_, err := client.AuctionLockProof(core.AuctionLockParams{
		Bob:         bob,
		TokenId:     tokenId,
		SaltIn:      big.NewInt(5),
		TreeNumber:  big.NewInt(0),
		MerkleProof: makeMerkleProof(core.AuctionMerkleDepth),
		SaltLocked:  saltLocked,
		SaltRevert:  saltRevert,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	payload := *captured
	if payload["wtSpendKey"] != "10" {
		t.Errorf("expected wtSpendKey=10, got %v", payload["wtSpendKey"])
	}
	if payload["wtTokenId"] != "77" {
		t.Errorf("expected wtTokenId=77, got %v", payload["wtTokenId"])
	}
	if payload["stNftTokenId"] != "77" {
		t.Errorf("expected stNftTokenId=77, got %v", payload["stNftTokenId"])
	}

	// stCommitLocked / stAuctionId must be independently derivable.
	wantCommitLocked, _ := core.Erc721Commitment(tokenId, bob.PublicKey, saltLocked)
	if payload["stCommitLocked"] != wantCommitLocked.String() {
		t.Errorf("stCommitLocked mismatch: got %v want %s", payload["stCommitLocked"], wantCommitLocked)
	}
	wantAuctionId, _ := core.GetAuctionId(wantCommitLocked)
	if payload["stAuctionId"] != wantAuctionId.String() {
		t.Errorf("stAuctionId mismatch: got %v want %s", payload["stAuctionId"], wantAuctionId)
	}

	// stRevertCommit must be independently derivable from saltRevert.
	wantRevertCommit, _ := core.Erc721Commitment(tokenId, bob.PublicKey, saltRevert)
	if payload["stRevertCommit"] != wantRevertCommit.String() {
		t.Errorf("stRevertCommit mismatch: got %v want %s", payload["stRevertCommit"], wantRevertCommit)
	}
	if payload["wtSaltRevert"] != saltRevert.String() {
		t.Errorf("wtSaltRevert mismatch: got %v want %s", payload["wtSaltRevert"], saltRevert)
	}
}

// --- AuctionBidProof tests ---

func TestAuctionBidProof_Success(t *testing.T) {
	server := mockAuctionServer(t, "/proof/auctionBid", 7)
	defer server.Close()

	client := core.NewAuctionClient(server.URL)
	result, err := client.AuctionBidProof(core.AuctionBidParams{
		AuctionId:   big.NewInt(1),
		Bidder:      makeSpendKeyPair(30, 40),
		BidAmount:   big.NewInt(100),
		TokenId:     big.NewInt(0),
		SaltIn:      big.NewInt(7),
		TreeNumber:  big.NewInt(0),
		MerkleProof: makeMerkleProof(core.AuctionMerkleDepth),
		BobPk:       big.NewInt(20),
		SaltA:       big.NewInt(11),
		SaltB:       big.NewInt(22),
		SaltRevert:  big.NewInt(33),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.PublicSignal) != 7 {
		t.Errorf("expected 7 public signal elements, got %d", len(result.PublicSignal))
	}
}

func TestAuctionBidProof_PayloadFields(t *testing.T) {
	server, captured := mockAuctionServerCapture(t, "/proof/auctionBid", 7)
	defer server.Close()

	client := core.NewAuctionClient(server.URL)
	bidder := makeSpendKeyPair(30, 40)
	bobPk := big.NewInt(20)
	amount := big.NewInt(100)
	tokenId := big.NewInt(0)
	saltA := big.NewInt(11)
	saltB := big.NewInt(22)
	saltRevert := big.NewInt(33)

	_, err := client.AuctionBidProof(core.AuctionBidParams{
		AuctionId:   big.NewInt(1),
		Bidder:      bidder,
		BidAmount:   amount,
		TokenId:     tokenId,
		SaltIn:      big.NewInt(7),
		TreeNumber:  big.NewInt(0),
		MerkleProof: makeMerkleProof(core.AuctionMerkleDepth),
		BobPk:       bobPk,
		SaltA:       saltA,
		SaltB:       saltB,
		SaltRevert:  saltRevert,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	payload := *captured
	// All-in bid: wtAmount must equal wtBidAmount.
	if payload["wtAmount"] != payload["wtBidAmount"] {
		t.Errorf("expected wtAmount == wtBidAmount (all-in), got %v vs %v",
			payload["wtAmount"], payload["wtBidAmount"])
	}

	wantCommitA, _ := core.Erc20CommitmentV2(bidder.PublicKey, saltA, amount, tokenId)
	if payload["stCommitA"] != wantCommitA.String() {
		t.Errorf("stCommitA mismatch: got %v want %s", payload["stCommitA"], wantCommitA)
	}
	wantCommitB, _ := core.Erc20CommitmentV2(bobPk, saltB, amount, tokenId)
	if payload["stCommitB"] != wantCommitB.String() {
		t.Errorf("stCommitB mismatch: got %v want %s", payload["stCommitB"], wantCommitB)
	}

	wantRevertCommit, _ := core.Erc20CommitmentV2(bidder.PublicKey, saltRevert, amount, tokenId)
	if payload["stRevertCommit"] != wantRevertCommit.String() {
		t.Errorf("stRevertCommit mismatch: got %v want %s", payload["stRevertCommit"], wantRevertCommit)
	}
	if payload["wtSaltRevert"] != saltRevert.String() {
		t.Errorf("wtSaltRevert mismatch: got %v want %s", payload["wtSaltRevert"], saltRevert)
	}
}

// --- AuctionBatchProof tests ---

func makeBatchSlots(active map[int][4]int64) [core.AuctionBatchSize]core.AuctionBatchSlot {
	var slots [core.AuctionBatchSize]core.AuctionBatchSlot
	for i, v := range active {
		// v = [commitA, pk, saltA, amount]; tokenId fixed at 0 for simplicity.
		slots[i] = core.AuctionBatchSlot{
			Active:  true,
			CommitA: big.NewInt(v[0]),
			Pk:      big.NewInt(v[1]),
			SaltA:   big.NewInt(v[2]),
			Amount:  big.NewInt(v[3]),
			TokenId: big.NewInt(0),
		}
	}
	return slots
}

func TestAuctionBatchProof_PicksHighestActiveBid(t *testing.T) {
	server, captured := mockAuctionServerCapture(t, "/proof/auctionBatch", core.AuctionBatchSize+4)
	defer server.Close()

	client := core.NewAuctionClient(server.URL)
	slots := makeBatchSlots(map[int][4]int64{
		0: {101, 201, 301, 10}, // amount 10
		1: {102, 202, 302, 20}, // amount 20 — winner
		5: {103, 203, 303, 5},  // amount 5
	})

	_, err := client.AuctionBatchProof(core.AuctionBatchParams{
		AuctionId: big.NewInt(1),
		Slots:     slots,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	payload := *captured
	if payload["wtWinnerIdx"] != "1" {
		t.Errorf("expected wtWinnerIdx=1, got %v", payload["wtWinnerIdx"])
	}
	if payload["stBatchWinnerAmount"] != "20" {
		t.Errorf("expected stBatchWinnerAmount=20, got %v", payload["stBatchWinnerAmount"])
	}
	if payload["stBatchWinnerCommit"] != "102" {
		t.Errorf("expected stBatchWinnerCommit=102, got %v", payload["stBatchWinnerCommit"])
	}

	// Inactive slots must be zero-filled in both stBidCommitA and wtActive.
	stBidCommitA, ok := payload["stBidCommitA"].([]interface{})
	if !ok || len(stBidCommitA) != core.AuctionBatchSize {
		t.Fatalf("expected stBidCommitA array of length %d", core.AuctionBatchSize)
	}
	if stBidCommitA[2] != "0" {
		t.Errorf("expected inactive slot 2 to be 0, got %v", stBidCommitA[2])
	}
	if stBidCommitA[1] != "102" {
		t.Errorf("expected active slot 1 to be 102, got %v", stBidCommitA[1])
	}
}

func TestAuctionBatchProof_NoActiveSlots_Error(t *testing.T) {
	client := core.NewAuctionClient("http://localhost:1") // unreachable — must not even be dialed
	var slots [core.AuctionBatchSize]core.AuctionBatchSlot

	_, err := client.AuctionBatchProof(core.AuctionBatchParams{
		AuctionId: big.NewInt(1),
		Slots:     slots,
	})
	if err == nil {
		t.Fatal("expected error when no slots are active")
	}
}

// --- AuctionFinalProof tests ---

func TestAuctionFinalProof_PicksHighestActiveBatch(t *testing.T) {
	server, captured := mockAuctionServerCapture(t, "/proof/auctionFinal", 3*core.AuctionFinalBatches+8)
	defer server.Close()

	client := core.NewAuctionClient(server.URL)
	var batches [core.AuctionFinalBatches]core.AuctionFinalBatchResult
	batches[0] = core.AuctionFinalBatchResult{Active: true, Commit: big.NewInt(11), Pk: big.NewInt(21), Amount: big.NewInt(100)}
	batches[1] = core.AuctionFinalBatchResult{Active: true, Commit: big.NewInt(12), Pk: big.NewInt(22), Amount: big.NewInt(200)} // winner
	batches[2] = core.AuctionFinalBatchResult{Active: true, Commit: big.NewInt(13), Pk: big.NewInt(23), Amount: big.NewInt(50)}

	bobPk := big.NewInt(99)
	saltB := big.NewInt(7)
	usdcTokenId := big.NewInt(0)
	saltAlice := big.NewInt(8)
	nftTokenId := big.NewInt(77)
	floorPrice := big.NewInt(10)

	result, err := client.AuctionFinalProof(core.AuctionFinalParams{
		AuctionId:   big.NewInt(1),
		Batches:     batches,
		BobPk:       bobPk,
		SaltB:       saltB,
		UsdcTokenId: usdcTokenId,
		SaltAlice:   saltAlice,
		NftTokenId:  nftTokenId,
		FloorPrice:  floorPrice,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.PublicSignal) != 3*core.AuctionFinalBatches+8 {
		t.Errorf("expected %d public signal elements, got %d", 3*core.AuctionFinalBatches+8, len(result.PublicSignal))
	}

	payload := *captured
	if payload["wtWinnerBatchIdx"] != "1" {
		t.Errorf("expected wtWinnerBatchIdx=1, got %v", payload["wtWinnerBatchIdx"])
	}
	if payload["stWinningAmount"] != "200" {
		t.Errorf("expected stWinningAmount=200, got %v", payload["stWinningAmount"])
	}
	if payload["stOverallWinnerCommit"] != "12" {
		t.Errorf("expected stOverallWinnerCommit=12, got %v", payload["stOverallWinnerCommit"])
	}
	if payload["stFloorPrice"] != "10" {
		t.Errorf("expected stFloorPrice=10, got %v", payload["stFloorPrice"])
	}

	wantCommitB, _ := core.Erc20CommitmentV2(bobPk, saltB, big.NewInt(200), usdcTokenId)
	if payload["stWinnerCommitB"] != wantCommitB.String() {
		t.Errorf("stWinnerCommitB mismatch: got %v want %s", payload["stWinnerCommitB"], wantCommitB)
	}
	wantNftCommit, _ := core.Erc721Commitment(nftTokenId, batches[1].Pk, saltAlice)
	if payload["stWinnerNftCommit"] != wantNftCommit.String() {
		t.Errorf("stWinnerNftCommit mismatch: got %v want %s", payload["stWinnerNftCommit"], wantNftCommit)
	}
}

func TestAuctionFinalProof_NoActiveBatches_Error(t *testing.T) {
	client := core.NewAuctionClient("http://localhost:1")
	var batches [core.AuctionFinalBatches]core.AuctionFinalBatchResult

	_, err := client.AuctionFinalProof(core.AuctionFinalParams{
		AuctionId:   big.NewInt(1),
		Batches:     batches,
		BobPk:       big.NewInt(1),
		SaltB:       big.NewInt(1),
		UsdcTokenId: big.NewInt(0),
		SaltAlice:   big.NewInt(1),
		NftTokenId:  big.NewInt(1),
		FloorPrice:  big.NewInt(0),
	})
	if err == nil {
		t.Fatal("expected error when no batches are active")
	}
}

// --- AuctionRevertProof tests ---

func TestAuctionRevertProof_Success(t *testing.T) {
	server := mockAuctionServer(t, "/proof/auctionRevert", 4)
	defer server.Close()

	client := core.NewAuctionClient(server.URL)
	result, err := client.AuctionRevertProof(core.AuctionRevertParams{
		Bob:        makeSpendKeyPair(10, 20),
		TokenId:    big.NewInt(77),
		SaltLocked: big.NewInt(123),
		SaltOut:    big.NewInt(456),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.PublicSignal) != 4 {
		t.Errorf("expected 4 public signal elements, got %d", len(result.PublicSignal))
	}
}

func TestAuctionRevertProof_PayloadFields(t *testing.T) {
	server, captured := mockAuctionServerCapture(t, "/proof/auctionRevert", 4)
	defer server.Close()

	client := core.NewAuctionClient(server.URL)
	bob := makeSpendKeyPair(10, 20)
	tokenId := big.NewInt(77)
	saltLocked := big.NewInt(123)
	saltOut := big.NewInt(456)

	_, err := client.AuctionRevertProof(core.AuctionRevertParams{
		Bob:        bob,
		TokenId:    tokenId,
		SaltLocked: saltLocked,
		SaltOut:    saltOut,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	payload := *captured
	wantCommitLocked, _ := core.Erc721Commitment(tokenId, bob.PublicKey, saltLocked)
	if payload["stCommitLocked"] != wantCommitLocked.String() {
		t.Errorf("stCommitLocked mismatch: got %v want %s", payload["stCommitLocked"], wantCommitLocked)
	}
	wantRevertedCommit, _ := core.Erc721Commitment(tokenId, bob.PublicKey, saltOut)
	if payload["stRevertedCommit"] != wantRevertedCommit.String() {
		t.Errorf("stRevertedCommit mismatch: got %v want %s", payload["stRevertedCommit"], wantRevertedCommit)
	}
	// The fresh salt must actually differ from the locked salt in the request.
	if payload["wtSaltOut"] == payload["wtSaltLocked"] {
		t.Errorf("wtSaltOut must differ from wtSaltLocked, both were %v", payload["wtSaltOut"])
	}
}

// --- Error handling ---

func TestAuctionLockProof_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := core.NewAuctionClient(server.URL)
	_, err := client.AuctionLockProof(core.AuctionLockParams{
		Bob:         makeSpendKeyPair(10, 20),
		TokenId:     big.NewInt(77),
		SaltIn:      big.NewInt(5),
		TreeNumber:  big.NewInt(0),
		MerkleProof: makeMerkleProof(core.AuctionMerkleDepth),
		SaltLocked:  big.NewInt(123),
		SaltRevert:  big.NewInt(456),
	})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestAuctionRevertProof_ConnectionRefused(t *testing.T) {
	client := core.NewAuctionClient("http://localhost:1")
	_, err := client.AuctionRevertProof(core.AuctionRevertParams{
		Bob:        makeSpendKeyPair(10, 20),
		TokenId:    big.NewInt(77),
		SaltLocked: big.NewInt(123),
		SaltOut:    big.NewInt(456),
	})
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

func TestAuctionBidProof_MalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"proof":[1,2,3],"publicSignal":[1]}`)) // only 3 proof elements
	}))
	defer server.Close()

	client := core.NewAuctionClient(server.URL)
	_, err := client.AuctionBidProof(core.AuctionBidParams{
		AuctionId:   big.NewInt(1),
		Bidder:      makeSpendKeyPair(30, 40),
		BidAmount:   big.NewInt(100),
		TokenId:     big.NewInt(0),
		SaltIn:      big.NewInt(7),
		TreeNumber:  big.NewInt(0),
		MerkleProof: makeMerkleProof(core.AuctionMerkleDepth),
		BobPk:       big.NewInt(20),
		SaltA:       big.NewInt(11),
		SaltB:       big.NewInt(22),
		SaltRevert:  big.NewInt(33),
	})
	if err == nil {
		t.Fatal("expected error for malformed proof (wrong element count)")
	}
}
