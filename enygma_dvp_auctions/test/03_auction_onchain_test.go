package tests

// End-to-end integration test for the EnygmaAuction contract.
//
// This test mirrors the structure of enygma_dvp's integration tests:
//   1. Running Hardhat node  (npx hardhat node, from enygma_dvp_auctions/)
//   2. Deployed contracts    (scripts/deploy.go → build/receipts.json)
//   3. Initialised VKs/roles (scripts/init.go)
//   4. Auction gnark server  (gnark_circuits/main.go on :8083)
//
// Scenario (single bidder, 1 batch, direct settle):
//   Bob locks an ERC-721 NFT (tokenId=42) with floorPrice=50.
//   Alice submits one USDC bid (amount=100, above floor).
//   Auctioneer runs one Phase-1 batch (Alice wins her batch).
//   Auctioneer runs Phase-2 final — auction settles.
//   State checked: SETTLED, overallWinnerCommit = Alice's commitA.
//
// Run:
//   npx hardhat node &
//   cd scripts && CC=/usr/bin/clang go build -o /tmp/deploy deploy.go init.go && cd .. && /tmp/deploy
//   cd gnark_circuits && go run ./cmd/export_vk_init_auction/ ../build
//   cd scripts && CC=/usr/bin/clang go build -o /tmp/init deploy.go init.go && cd .. && /tmp/init
//   cd gnark_circuits && go run main.go &
//   cd test && CC=/usr/bin/clang go test -run TestAuction_OnChain -v -timeout 600s

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/raylsnetwork/enygma_dvp_auctions/src/core"
)

const (
	hardhatRPC    = "http://localhost:8545"
	onchainChain  = "31337"
	ownerPrivKey  = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

	onchainNftTokenId  = 42
	onchainUsdcTokenId = 0
	onchainFloorPrice  = 50
	onchainBidAmount   = 100
)

func TestAuction_OnChain(t *testing.T) {
	// Skip guards: both the gnark server AND Hardhat node must be up.
	if !serverAvailable(serverAddr) {
		t.Skipf("auction gnark server not running on %s — skipping", serverAddr)
	}
	if !tcpAvailable("localhost:8545") {
		t.Skip("Hardhat node not running on localhost:8545 — skipping")
	}

	// ─── Setup ────────────────────────────────────────────────────────────────

	projectRoot := findProjectRoot(t)

	receipts := loadOnChainReceipts(t, projectRoot)
	ethClient := dialEth(t)
	defer ethClient.Close()

	chainID, _ := new(big.Int).SetString(onchainChain, 10)
	ownerAuth := makeAuth(t, ownerPrivKey, chainID)

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

	gnarkClient := core.NewAuctionClient("") // defaults to :8083

	nftTokenId  := big.NewInt(onchainNftTokenId)
	usdcTokenId := big.NewInt(onchainUsdcTokenId)
	floorPrice  := big.NewInt(onchainFloorPrice)
	bidAmount   := big.NewInt(onchainBidAmount)

	// ─── Key pairs ────────────────────────────────────────────────────────────

	bob, err := core.NewSpendKeyPair()
	checkErr(t, "NewSpendKeyPair(bob)", err)

	alice, err := core.NewSpendKeyPair()
	checkErr(t, "NewSpendKeyPair(alice)", err)

	// ─── NFT commitment ───────────────────────────────────────────────────────

	bobSaltIn, err := core.RandomInField()
	checkErr(t, "RandomInField(bobSaltIn)", err)

	bobNftCommitIn, err := core.Erc721Commitment(nftTokenId, bob.PublicKey, bobSaltIn)
	checkErr(t, "Erc721Commitment", err)

	t.Logf("Bob NFT commitIn  = %s", bobNftCommitIn)
	t.Logf("Bob public key    = %s", bob.PublicKey)

	// ─── Seed NFT note into NftVault ──────────────────────────────────────────

	t.Log("Seeding Bob's NFT note into NftVault...")
	seedTx, err := nftVaultContract.Transact(ownerAuth, "seedDeposit", bobNftCommitIn)
	checkErr(t, "seedDeposit(nft)", err)
	waitTx(t, ethClient, seedTx)

	nftRoot := vaultRoot(t, nftVaultContract)
	t.Logf("NftVault root after seed = %s", nftRoot)

	// Build local NFT tree mirroring the vault (depth 8, same Poseidon zero value).
	nftTree := core.NewMerkleTree(core.AuctionMerkleDepth)
	nftTree.InsertLeaf(bobNftCommitIn)
	nftProof, err := nftTree.GenerateProof(bobNftCommitIn)
	checkErr(t, "GenerateProof(nft)", err)

	if nftProof.Root.Cmp(nftRoot) != 0 {
		t.Fatalf("NFT local tree root %s ≠ vault root %s", nftProof.Root, nftRoot)
	}

	// ─── USDC commitment ──────────────────────────────────────────────────────

	aliceSaltIn, err := core.RandomInField()
	checkErr(t, "RandomInField(aliceSaltIn)", err)

	aliceUsdcCommitIn, err := core.Erc20CommitmentV2(alice.PublicKey, aliceSaltIn, bidAmount, usdcTokenId)
	checkErr(t, "Erc20CommitmentV2", err)

	t.Logf("Alice USDC commitIn = %s", aliceUsdcCommitIn)

	// ─── Seed USDC note into UsdcVault ───────────────────────────────────────

	t.Log("Seeding Alice's USDC note into UsdcVault...")
	seedTx, err = usdcVaultContract.Transact(ownerAuth, "seedDeposit", aliceUsdcCommitIn)
	checkErr(t, "seedDeposit(usdc)", err)
	waitTx(t, ethClient, seedTx)

	usdcRoot := vaultRoot(t, usdcVaultContract)
	t.Logf("UsdcVault root after seed = %s", usdcRoot)

	usdcTree := core.NewMerkleTree(core.AuctionMerkleDepth)
	usdcTree.InsertLeaf(aliceUsdcCommitIn)
	usdcProof, err := usdcTree.GenerateProof(aliceUsdcCommitIn)
	checkErr(t, "GenerateProof(usdc)", err)

	if usdcProof.Root.Cmp(usdcRoot) != 0 {
		t.Fatalf("USDC local tree root %s ≠ vault root %s", usdcProof.Root, usdcRoot)
	}

	// ─── Phase 0a: initAuction ────────────────────────────────────────────────

	t.Log("── Phase 0a: AuctionLock / initAuction ──────────────────────────────────")

	saltLocked, err := core.RandomInField()
	checkErr(t, "RandomInField(saltLocked)", err)
	saltRevertBob, err := core.RandomInField()
	checkErr(t, "RandomInField(saltRevertBob)", err)

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

	// deadline = current block timestamp + 3600 seconds
	// settlementDeadline = deadline + 7200 seconds (auctioneer has 2 hours to settle after bidding closes)
	header, err := ethClient.HeaderByNumber(context.Background(), nil)
	checkErr(t, "HeaderByNumber", err)
	deadline           := new(big.Int).Add(new(big.Int).SetUint64(header.Time), big.NewInt(3600))
	settlementDeadline := new(big.Int).Add(deadline, big.NewInt(7200))

	proof8Lock  := toBigArr8(lockResult.Proof)
	signal7Lock := toBigArr7(lockResult.PublicSignal)

	initTx, err := auctionContract.Transact(ownerAuth, "initAuction",
		proof8Lock, signal7Lock, deadline, settlementDeadline, floorPrice,
	)
	checkErr(t, "initAuction tx", err)
	waitTx(t, ethClient, initTx)
	t.Log("initAuction: OK")

	// ─── Phase 0b: submitBid ──────────────────────────────────────────────────

	t.Log("── Phase 0b: AuctionBid / submitBid ─────────────────────────────────────")

	saltA, err := core.RandomInField()
	checkErr(t, "RandomInField(saltA)", err)
	saltB, err := core.RandomInField()
	checkErr(t, "RandomInField(saltB)", err)
	saltRevertAlice, err := core.RandomInField()
	checkErr(t, "RandomInField(saltRevertAlice)", err)

	bidResult, err := gnarkClient.AuctionBidProof(core.AuctionBidParams{
		AuctionId:   auctionId,
		Bidder:      alice,
		BidAmount:   bidAmount,
		TokenId:     usdcTokenId,
		SaltIn:      aliceSaltIn,
		TreeNumber:  big.NewInt(0),
		MerkleProof: usdcProof,
		BobPk:       bob.PublicKey,
		SaltA:       saltA,
		SaltB:       saltB,
		SaltRevert:  saltRevertAlice,
	})
	checkErr(t, "AuctionBidProof", err)

	aliceCommitA := bidResult.PublicSignal[4]
	aliceCommitB := bidResult.PublicSignal[5]
	t.Logf("Alice commitA = %s", aliceCommitA)
	t.Logf("Alice commitB = %s", aliceCommitB)

	proof8Bid  := toBigArr8(bidResult.Proof)
	signal7Bid := toBigArr7(bidResult.PublicSignal)

	// Dummy ML-KEM ciphertext (content not verified on-chain, only stored for auctioneer)
	ctxt1 := []byte("dummy-mlkem-capsule")
	ctxt2 := []byte("dummy-aead-ciphertext")

	bidTx, err := auctionContract.Transact(ownerAuth, "submitBid",
		proof8Bid, signal7Bid, ctxt1, ctxt2,
	)
	checkErr(t, "submitBid tx", err)
	waitTx(t, ethClient, bidTx)
	t.Log("submitBid: OK")

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
	t.Log("Time advanced: OK")

	// ─── Phase 1: submitBatch ─────────────────────────────────────────────────

	t.Log("── Phase 1: AuctionBatch / submitBatch ──────────────────────────────────")

	var slots [core.AuctionBatchSize]core.AuctionBatchSlot
	slots[0] = core.AuctionBatchSlot{
		Active:  true,
		CommitA: aliceCommitA,
		Pk:      alice.PublicKey,
		SaltA:   saltA,
		Amount:  bidAmount,
		TokenId: usdcTokenId,
	}
	// Slots 1..99 are zero (inactive).

	batchResult, err := gnarkClient.AuctionBatchProof(core.AuctionBatchParams{
		AuctionId: auctionId,
		Slots:     slots,
	})
	checkErr(t, "AuctionBatchProof", err)

	batchWinnerCommit := batchResult.PublicSignal[101]
	batchWinnerPk     := batchResult.PublicSignal[102]
	batchWinnerAmount := batchResult.PublicSignal[103]
	t.Logf("Batch winner commitA=%s amount=%s", batchWinnerCommit, batchWinnerAmount)

	batchId      := big.NewInt(1)
	proof8Batch  := toBigArr8(batchResult.Proof)
	signal104    := toBigArr104(batchResult.PublicSignal)

	batchTx, err := auctionContract.Transact(ownerAuth, "submitBatch",
		batchId, proof8Batch, signal104,
	)
	checkErr(t, "submitBatch tx", err)
	waitTx(t, ethClient, batchTx)
	t.Log("submitBatch: OK")

	// ─── Phase 2: settle ──────────────────────────────────────────────────────

	t.Log("── Phase 2: AuctionFinal / settle ───────────────────────────────────────")

	var batchResults [core.AuctionFinalBatches]core.AuctionFinalBatchResult
	batchResults[0] = core.AuctionFinalBatchResult{
		Active: true,
		Commit: batchWinnerCommit,
		Pk:     batchWinnerPk,
		Amount: batchWinnerAmount,
	}
	// Slots 1..9 are zero (inactive).

	saltAlice, err := core.RandomInField()
	checkErr(t, "RandomInField(saltAlice)", err)

	finalResult, err := gnarkClient.AuctionFinalProof(core.AuctionFinalParams{
		AuctionId:   auctionId,
		Batches:     batchResults,
		BobPk:       bob.PublicKey,
		SaltB:       saltB,
		UsdcTokenId: usdcTokenId,
		SaltAlice:   saltAlice,
		NftTokenId:  nftTokenId,
		FloorPrice:  floorPrice,
	})
	checkErr(t, "AuctionFinalProof", err)

	t.Logf("AuctionFinal signals: %d", len(finalResult.PublicSignal))
	t.Logf("  overallWinnerCommit = %s", finalResult.PublicSignal[31])
	t.Logf("  winningAmount       = %s", finalResult.PublicSignal[36])

	proof8Final  := toBigArr8(finalResult.Proof)
	signal38     := toBigArr38(finalResult.PublicSignal)

	// batchIds[i] maps statement slot i to the on-chain batchId submitted by submitBatch.
	var batchIds [10]*big.Int
	batchIds[0] = batchId
	for i := 1; i < 10; i++ {
		batchIds[i] = big.NewInt(0)
	}

	settleTx, err := auctionContract.Transact(ownerAuth, "settleOptimistic",
		proof8Final, signal38, batchIds,
	)
	checkErr(t, "settleOptimistic tx", err)
	waitTx(t, ethClient, settleTx)
	t.Log("settleOptimistic: OK — challenge window started")

	// Advance time past the 1-day challenge window so finalizeSettlement can be called.
	var timeResult2 interface{}
	if err := rpcClient.Call(&timeResult2, "evm_increaseTime", 86401); err != nil {
		t.Fatalf("evm_increaseTime (challenge window): %v", err)
	}
	if err := rpcClient.Call(&timeResult2, "evm_mine"); err != nil {
		t.Fatalf("evm_mine: %v", err)
	}

	finalizeTx, err := auctionContract.Transact(ownerAuth, "finalizeSettlement", auctionId)
	checkErr(t, "finalizeSettlement tx", err)
	waitTx(t, ethClient, finalizeTx)
	t.Log("finalizeSettlement: OK")

	// ─── Verify final state ───────────────────────────────────────────────────

	t.Log("── Verification ──────────────────────────────────────────────────────────")

	// getAuctionState → uint8 (SETTLED = 3)
	var stateResults []interface{}
	err = auctionContract.Call(&bind.CallOpts{}, &stateResults, "getAuctionState", auctionId)
	checkErr(t, "getAuctionState", err)
	state := stateResults[0].(uint8)
	if state != 3 {
		t.Errorf("expected AuctionState.SETTLED (3), got %d", state)
	} else {
		t.Logf("✓ AuctionState = SETTLED (%d)", state)
	}

	// getAuctionCore → check overallWinnerCommit and bidCount
	var coreResults []interface{}
	err = auctionContract.Call(&bind.CallOpts{}, &coreResults, "getAuctionCore", auctionId)
	checkErr(t, "getAuctionCore", err)
	// Return order: state, nftTokenId, commitLocked, revertCommit, overallWinnerCommit,
	//               batchCount, deadline, settlementDeadline, floorPrice, bidCount
	overallWinnerCommit := coreResults[4].(*big.Int)
	bidCount            := coreResults[9].(*big.Int)
	t.Logf("  overallWinnerCommit = %s", overallWinnerCommit)
	t.Logf("  bidCount            = %s", bidCount)

	if overallWinnerCommit.Cmp(aliceCommitA) != 0 {
		t.Errorf("overallWinnerCommit mismatch: got %s, want %s", overallWinnerCommit, aliceCommitA)
	} else {
		t.Log("✓ overallWinnerCommit == Alice's commitA")
	}
	if bidCount.Cmp(big.NewInt(1)) != 0 {
		t.Errorf("bidCount mismatch: got %s, want 1", bidCount)
	} else {
		t.Log("✓ bidCount == 1")
	}

	// Check that Alice's bid is marked claimed (winner cannot double-spend).
	var bidResults []interface{}
	err = auctionContract.Call(&bind.CallOpts{}, &bidResults, "getBid", auctionId, aliceCommitA)
	checkErr(t, "getBid", err)
	// BidData: active bool, claimed bool, commitB uint256, nullifier uint256, treeNumber uint256
	type bidData struct {
		Active     bool
		Claimed    bool
		CommitB    *big.Int
		Nullifier  *big.Int
		TreeNumber *big.Int
	}
	// bind returns struct fields as a struct or separate values depending on ABI
	// For named structs from ABI, results come back as map or struct.
	// Use struct binding via ABI call output binding.
	if len(bidResults) > 0 {
		if bd, ok := bidResults[0].(struct {
			Active       bool     `abi:"active"`
			Claimed      bool     `abi:"claimed"`
			CommitB      *big.Int `abi:"commitB"`
			RevertCommit *big.Int `abi:"revertCommit"`
			Nullifier    *big.Int `abi:"nullifier"`
			TreeNumber   *big.Int `abi:"treeNumber"`
		}); ok {
			if !bd.Claimed {
				t.Error("expected Alice's bid to be claimed after settle")
			} else {
				t.Log("✓ Alice's bid is claimed")
			}
		}
	}

	t.Log("✓ On-chain auction end-to-end test complete")
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func tcpAvailable(addr string) bool {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func findProjectRoot(t *testing.T) string {
	t.Helper()
	// Walk up from the test directory until we find enygma_auction.config.json.
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "enygma_auction.config.json")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find enygma_auction.config.json — run from inside the project")
		}
		dir = parent
	}
}

func loadOnChainReceipts(t *testing.T, projectRoot string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(projectRoot, "build", "receipts.json"))
	if err != nil {
		t.Skipf("build/receipts.json not found — run scripts/deploy.go first: %v", err)
	}
	var raw map[string]struct {
		ContractAddress string `json:"contractAddress"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse receipts.json: %v", err)
	}
	m := make(map[string]string, len(raw))
	for k, v := range raw {
		m[k] = v.ContractAddress
	}
	return m
}

func dialEth(t *testing.T) *ethclient.Client {
	t.Helper()
	c, err := ethclient.Dial(hardhatRPC)
	if err != nil {
		t.Fatalf("ethclient.Dial: %v", err)
	}
	return c
}

func makeAuth(t *testing.T, privKeyHex string, chainID *big.Int) *bind.TransactOpts {
	t.Helper()
	privKeyHex = strings.TrimPrefix(privKeyHex, "0x")
	pk, err := crypto.HexToECDSA(privKeyHex)
	if err != nil {
		t.Fatalf("HexToECDSA: %v", err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(pk, chainID)
	if err != nil {
		t.Fatalf("NewKeyedTransactorWithChainID: %v", err)
	}
	return auth
}

func loadABI(t *testing.T, projectRoot, contractPath string) abi.ABI {
	t.Helper()
	artifactPath := filepath.Join(projectRoot, "artifacts", "contracts", contractPath+".json")
	data, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Skipf("artifact not found (%s) — run npx hardhat compile first: %v", artifactPath, err)
	}
	var artifact struct {
		ABI json.RawMessage `json:"abi"`
	}
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatalf("parse artifact: %v", err)
	}
	parsedABI, err := abi.JSON(strings.NewReader(string(artifact.ABI)))
	if err != nil {
		t.Fatalf("parse ABI: %v", err)
	}
	return parsedABI
}

func checkErr(t *testing.T, label string, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", label, err)
	}
}

func waitTx(t *testing.T, client *ethclient.Client, tx *types.Transaction) {
	t.Helper()
	rec, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil {
		t.Fatalf("WaitMined: %v", err)
	}
	if rec.Status == 0 {
		t.Fatalf("transaction %s reverted", rec.TxHash.Hex())
	}
}

func vaultRoot(t *testing.T, vault *bind.BoundContract) *big.Int {
	t.Helper()
	var results []interface{}
	if err := vault.Call(&bind.CallOpts{}, &results, "currentRoot"); err != nil {
		t.Fatalf("currentRoot: %v", err)
	}
	return results[0].(*big.Int)
}

// ─── Fixed-size array converters ──────────────────────────────────────────────

func toBigArr8(slice []*big.Int) [8]*big.Int {
	var arr [8]*big.Int
	for i := 0; i < 8 && i < len(slice); i++ {
		arr[i] = slice[i]
	}
	for i := len(slice); i < 8; i++ {
		arr[i] = big.NewInt(0)
	}
	return arr
}

func toBigArr6(slice []*big.Int) [6]*big.Int {
	var arr [6]*big.Int
	for i := 0; i < 6 && i < len(slice); i++ {
		arr[i] = slice[i]
	}
	for i := len(slice); i < 6; i++ {
		arr[i] = big.NewInt(0)
	}
	return arr
}

func toBigArr7(slice []*big.Int) [7]*big.Int {
	var arr [7]*big.Int
	for i := 0; i < 7 && i < len(slice); i++ {
		arr[i] = slice[i]
	}
	for i := len(slice); i < 7; i++ {
		arr[i] = big.NewInt(0)
	}
	return arr
}

func toBigArr38(slice []*big.Int) [38]*big.Int {
	var arr [38]*big.Int
	for i := 0; i < 38 && i < len(slice); i++ {
		arr[i] = slice[i]
	}
	for i := len(slice); i < 38; i++ {
		arr[i] = big.NewInt(0)
	}
	return arr
}

func toBigArr104(slice []*big.Int) [104]*big.Int {
	var arr [104]*big.Int
	for i := 0; i < 104 && i < len(slice); i++ {
		arr[i] = slice[i]
	}
	for i := len(slice); i < 104; i++ {
		arr[i] = big.NewInt(0)
	}
	return arr
}

func toBigArr10(slice []*big.Int) [10]*big.Int {
	var arr [10]*big.Int
	for i := 0; i < 10 && i < len(slice); i++ {
		arr[i] = slice[i]
	}
	for i := len(slice); i < 10; i++ {
		arr[i] = big.NewInt(0)
	}
	return arr
}

// formatAddr converts a hex string to a log-friendly short form.
func formatAddr(hex string) string {
	if len(hex) > 10 {
		return hex[:6] + "…" + hex[len(hex)-4:]
	}
	return hex
}

var _ = fmt.Sprintf // suppress "imported and not used"
