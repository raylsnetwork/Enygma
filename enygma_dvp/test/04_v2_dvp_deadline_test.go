package tests

// On-chain integration tests for DvP with deadline / swap_id feature.
//
// Scenario: Alice swaps 50 USDT (ERC20) for Bob's NFT ticket (ERC721).
//
// Three sub-tests (each uses a fresh NFT tokenId to avoid minting conflicts):
//
//   FullSwap        — happy path: both legs submitted on time, swap settles atomically.
//   DeadlineExpired — Alice submits, Bob never responds, deadline passes (1 min),
//                     Alice calls claimSwapTimeout → revert commitment recoverable.
//   InvalidProof    — Alice submits, Bob submits an all-zero (invalid) proof (rejected),
//                     deadline passes, Alice calls claimSwapTimeout → revert commitment recoverable.
//
// Cross-commitment layout (matches EnygmaDvp.submitPartialSettlement cross-referencing):
//
//   Alice ERC20 DvP Initiator (1-in, NumberOfOutputs=1 on-chain, 7-element statement):
//     statement = [stMsg=commitA, treeNum, root, nf_A, commitB, commitA, revertCommitA]
//     receiptMessage  = statement[0] = commitA  (stMessage; set internally by DvPInitiatorProof)
//     receiptUniqueId = statement[4] = commitB  (first output; only this is inserted into ERC20 vault)
//   → stored: _pendingTransactions[commitB].targetReceiptId = commitA, deadline = deadline
//   → VK: VK_ID_DVP_INITIATOR (slot 23); ERC20 vault detects numberOfInputs==1
//
//   Bob ERC721 DvP Destination (1-in/1-out, 5-element statement):
//     statement = [stMsg=commitB, treeNum, root, nf_B, commitA]
//     receiptMessage  = statement[0] = commitB
//     receiptUniqueId = statement[4] = commitA
//   → check: _pendingTransactions[commitB].targetReceiptId == commitA ✓ → settle
//   → VK: VK_ID_DVP_DESTINATION (slot 24); ERC721 vault tries ownership VK, falls back to DvP VK
//
// claimSwapTimeout(commitB) is the revert entry point after deadline.
//
// Prerequisites:
//   1. npx hardhat node
//   2. deploy + init (see MEMORY.md)
//   3. cd gnark_circuits && go run main.go   (keys must include DvP circuits)
//
// Run:
//   cd test && CC=/usr/bin/clang go test -run TestV2DvP_WithDeadline -v -timeout 600s

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/raylsnetwork/enygma_dvp/src/core"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ── constants ─────────────────────────────────────────────────────────────────

const (
	dvpErc20Amount  = int64(50) // Alice delivers 50 USDT
	dvpErc20TokenId = int64(0)  // ERC20 tokenId convention
	dvpTicketAmount = int64(1)

	// Unique ERC721 tokenIds — chosen to not collide with other tests (25, 42)
	dvpTokenIdFull     = int64(100)
	dvpTokenIdDeadline = int64(101)
	dvpTokenIdInvalid  = int64(102)

	// Deadline window for timeout scenarios (seconds)
	dvpDeadlineSeconds = int64(60)
)

// ── shared setup ──────────────────────────────────────────────────────────────

type dvpTestContext struct {
	ctx            context.Context
	client         *ethclient.Client
	auth           *bind.TransactOpts
	gnarkClient    *core.GnarkClient
	merkleDepth    int
	erc20Vault     *bind.BoundContract
	erc20Token     *bind.BoundContract
	erc721Vault    *bind.BoundContract
	erc721Token    *bind.BoundContract
	dvp            *bind.BoundContract
	erc20VaultAddr common.Address
	erc721VaultAddr common.Address

	// event topic sigs
	commitmentSig    common.Hash
	nullifierSig     common.Hash
	swapTimedOutSig  common.Hash
	swapInitiatedSig common.Hash
}

func newDvpTestContext(t *testing.T) *dvpTestContext {
	t.Helper()
	ctx := context.Background()

	client, err := ethclient.Dial(hardhatRPC)
	if err != nil {
		t.Fatalf("ethclient.Dial: %v", err)
	}

	receipts := loadOnchainReceipts(t)
	erc20VaultAddr  := common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress)
	erc721VaultAddr := common.HexToAddress(receipts["Erc721CoinVault"].ContractAddress)
	dvpAddr         := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)
	erc20Addr       := common.HexToAddress(receipts["ERC20"].ContractAddress)
	erc721Addr      := common.HexToAddress(receipts["ERC721"].ContractAddress)

	erc20VaultABI  := loadOnchainABI(t, "core/contracts/vaults/Erc20CoinVault.sol/Erc20CoinVault.json")
	erc721VaultABI := loadOnchainABI(t, "core/contracts/vaults/Erc721CoinVault.sol/Erc721CoinVault.json")
	erc20ABI       := loadOnchainABI(t, "erc20/contracts/RaylsERC20.sol/RaylsERC20.json")
	erc721ABI      := loadOnchainABI(t, "erc721/contracts/RaylsERC721.sol/RaylsERC721.json")
	dvpABI         := loadOnchainABI(t, "core/contracts/EnygmaDvp.sol/EnygmaDvp.json")

	return &dvpTestContext{
		ctx:             ctx,
		client:          client,
		auth:            hardhatAuth(t, client),
		gnarkClient:     core.NewGnarkClient("http://localhost:8081"),
		merkleDepth:     8,
		erc20Vault:      bind.NewBoundContract(erc20VaultAddr, erc20VaultABI, client, client, client),
		erc20Token:      bind.NewBoundContract(erc20Addr, erc20ABI, client, client, client),
		erc721Vault:     bind.NewBoundContract(erc721VaultAddr, erc721VaultABI, client, client, client),
		erc721Token:     bind.NewBoundContract(erc721Addr, erc721ABI, client, client, client),
		dvp:             bind.NewBoundContract(dvpAddr, dvpABI, client, client, client),
		erc20VaultAddr:  erc20VaultAddr,
		erc721VaultAddr: erc721VaultAddr,
		commitmentSig:   crypto.Keccak256Hash([]byte("Commitment(uint256,uint256)")),
		nullifierSig:    crypto.Keccak256Hash([]byte("Nullifier(uint256,uint256,uint256)")),
		swapTimedOutSig:  crypto.Keccak256Hash([]byte("SwapTimedOut(uint256)")),
		swapInitiatedSig: crypto.Keccak256Hash([]byte("SwapInitiated(uint256,uint256,uint256,uint256)")),
	}
}

// dvpDeposits sets up Alice's ERC20 note and Bob's ERC721 note.
// Returns the generated key pairs, commitments, salts, and Merkle proofs.
type dvpDeposits struct {
	aliceSpend      *core.SpendKeyPair
	aliceView       *core.ViewKeyPair
	aliceSaltField  *big.Int
	aliceSaltBytes  []byte
	aliceCommitment *big.Int

	bobSpend        *core.SpendKeyPair
	bobView         *core.ViewKeyPair
	bobTicketSalt   *big.Int
	bobCommitment   *big.Int

	aliceMerkleProof *core.MerkleProof
	bobMerkleProof   *core.MerkleProof

	erc20Amount  *big.Int
	erc20TokenId *big.Int
	nftTokenId   *big.Int
	nftAmount    *big.Int
}

func dvpSetupDeposits(t *testing.T, tc *dvpTestContext, nftTokenId int64) *dvpDeposits {
	t.Helper()
	ctx := tc.ctx
	auth := tc.auth
	addr := auth.From

	erc20Amount  := big.NewInt(dvpErc20Amount)
	erc20TokenId := big.NewInt(dvpErc20TokenId)
	nftTokId     := big.NewInt(nftTokenId)
	nftAmount    := big.NewInt(dvpTicketAmount)
	contractAddr := big.NewInt(0)
	_ = contractAddr

	// ── Alice key pairs & ERC20 deposit ───────────────────────────────────────
	aliceSpend, err := core.NewSpendKeyPair()
	if err != nil { t.Fatalf("Alice NewSpendKeyPair: %v", err) }
	aliceView, err := core.NewViewKeyPair()
	if err != nil { t.Fatalf("Alice NewViewKeyPair: %v", err) }

	ss, capsule, err := core.Encapsulate(aliceView.EncapsKey)
	if err != nil { t.Fatalf("Alice Encapsulate: %v", err) }
	saltBytes, err := core.DerivePaymentSalt(ss)
	if err != nil { t.Fatalf("Alice DerivePaymentSalt: %v", err) }
	saltField := core.SaltBToField(saltBytes)

	aliceCommitment, err := core.Erc20CommitmentV2(aliceSpend.PublicKey, saltField, erc20Amount, erc20TokenId)
	if err != nil { t.Fatalf("Alice Erc20CommitmentV2: %v", err) }
	aliceEncKey, err := core.DerivePaymentKey(ss)
	if err != nil { t.Fatalf("Alice DerivePaymentKey: %v", err) }
	aliceEncData, err := core.EncryptPayload(aliceEncKey, erc20TokenId, erc20Amount)
	if err != nil { t.Fatalf("Alice EncryptPayload: %v", err) }

	mintTx, err := tc.erc20Token.Transact(auth, "mint", addr, new(big.Int).Mul(erc20Amount, big.NewInt(10)))
	if err != nil { t.Fatalf("ERC20.mint: %v", err) }
	if _, err := bind.WaitMined(ctx, tc.client, mintTx); err != nil { t.Fatalf("wait ERC20 mint: %v", err) }

	approveTx, err := tc.erc20Token.Transact(auth, "approve", tc.erc20VaultAddr, erc20Amount)
	if err != nil { t.Fatalf("ERC20.approve: %v", err) }
	if _, err := bind.WaitMined(ctx, tc.client, approveTx); err != nil { t.Fatalf("wait ERC20 approve: %v", err) }

	// C-1 fix: contract computes commitment on-chain from key material.
	// Send [amount, pkSpend, salt, tokenId] instead of [amount, commitment].
	depositTx, err := tc.erc20Vault.Transact(auth, "depositV2",
		[]*big.Int{erc20Amount, aliceSpend.PublicKey, saltField, erc20TokenId}, capsule, aliceEncData)
	if err != nil { t.Fatalf("erc20Vault.depositV2: %v", err) }
	if _, err := bind.WaitMined(ctx, tc.client, depositTx); err != nil { t.Fatalf("wait ERC20 depositV2: %v", err) }
	t.Logf("Alice deposited %s USDT — commitment: %s", erc20Amount, aliceCommitment)

	// ── Bob key pairs & ERC721 deposit ────────────────────────────────────────
	bobSpend, err := core.NewSpendKeyPair()
	if err != nil { t.Fatalf("Bob NewSpendKeyPair: %v", err) }
	bobView, err := core.NewViewKeyPair()
	if err != nil { t.Fatalf("Bob NewViewKeyPair: %v", err) }

	bobSalt, err := core.RandomInField()
	if err != nil { t.Fatalf("Bob RandomInField: %v", err) }
	bobCommitment, err := core.Erc721Commitment(nftTokId, bobSpend.PublicKey, bobSalt)
	if err != nil { t.Fatalf("Bob Erc721Commitment: %v", err) }

	mintNftTx, err := tc.erc721Token.Transact(auth, "mint", addr, nftTokId)
	if err != nil { t.Fatalf("ERC721.mint tokenId=%d: %v", nftTokenId, err) }
	if _, err := bind.WaitMined(ctx, tc.client, mintNftTx); err != nil { t.Fatalf("wait ERC721 mint: %v", err) }

	approveNftTx, err := tc.erc721Token.Transact(auth, "approve", tc.erc721VaultAddr, nftTokId)
	if err != nil { t.Fatalf("ERC721.approve: %v", err) }
	if _, err := bind.WaitMined(ctx, tc.client, approveNftTx); err != nil { t.Fatalf("wait ERC721 approve: %v", err) }

	// C-1 fix: contract now computes commitment = Poseidon4(pkSpend, salt, 1, tokenId) on-chain.
	depositNftTx, err := tc.erc721Vault.Transact(auth, "deposit", []*big.Int{nftTokId, bobSpend.PublicKey, bobSalt})
	if err != nil { t.Fatalf("erc721Vault.deposit: %v", err) }
	if _, err := bind.WaitMined(ctx, tc.client, depositNftTx); err != nil { t.Fatalf("wait ERC721 deposit: %v", err) }
	t.Logf("Bob deposited ticket tokenId=%d — commitment: %s", nftTokenId, bobCommitment)

	// ── Merkle proofs ─────────────────────────────────────────────────────────
	mt20 := loadVaultMerkleTree(t, tc.client, tc.erc20VaultAddr, tc.merkleDepth)
	aliceProof, err := mt20.GenerateProof(aliceCommitment)
	if err != nil { t.Fatalf("GenerateProof Alice ERC20: %v", err) }

	mt721 := loadVaultMerkleTree(t, tc.client, tc.erc721VaultAddr, tc.merkleDepth)
	bobProof, err := mt721.GenerateProof(bobCommitment)
	if err != nil { t.Fatalf("GenerateProof Bob ERC721: %v", err) }

	return &dvpDeposits{
		aliceSpend:       aliceSpend,
		aliceView:        aliceView,
		aliceSaltField:   saltField,
		aliceSaltBytes:   saltBytes,
		aliceCommitment:  aliceCommitment,
		bobSpend:         bobSpend,
		bobView:          bobView,
		bobTicketSalt:    bobSalt,
		bobCommitment:    bobCommitment,
		aliceMerkleProof: aliceProof,
		bobMerkleProof:   bobProof,
		erc20Amount:      erc20Amount,
		erc20TokenId:     erc20TokenId,
		nftTokenId:       nftTokId,
		nftAmount:        nftAmount,
	}
}

// dvpGenerateProofs generates Alice's DvP Initiator proof and Bob's DvP Destination proof.
// Returns commitmentA, commitmentB, revertCommitA and the on-chain receipts.
type dvpProofs struct {
	commitA        *big.Int
	commitB        *big.Int
	revertCommitA  *big.Int

	aliceReceipt   onchainProofReceipt
	bobReceipt     onchainProofReceipt
}

func dvpGenerateProofs(t *testing.T, tc *dvpTestContext, d *dvpDeposits) *dvpProofs {
	t.Helper()

	// ── Alice: DvP Initiator proof ────────────────────────────────────────────
	//
	// DvPInitiatorProof internally:
	//   1. Runs ML-KEM.Encapsulate(bob_view_pk) → (ss, cipherText)
	//   2. Derives saltB = HKDF(ss, "note salt"), saltA = HKDF(ss, "Init Salt")
	//   3. Computes commitB = Poseidon4(bobSpendPk, saltB, erc20Amount, erc20TokenId)
	//   4. Computes commitA = Poseidon4(aliceSpendPk, saltA, nftAmount, nftTokenId)
	//                       = Erc721Commitment(nftTokenId, aliceSpendPk, saltA)
	//   5. Computes revertCommitA = Poseidon4(aliceSpendPk, revertSalt, erc20Amount, erc20TokenId)
	//   6. Sets stMessage = commitA (for on-chain cross-referencing)
	//   7. Generates ZK proof with DvP Initiator circuit (VK slot 23)
	//
	// Statement: [commitA, treeNum, root, nf_A, commitB, commitA, revertCommitA] (7 elements)
	// NumberOfInputs=1, NumberOfOutputs=1 (only commitB is treated as payment output on-chain)
	aliceResult, err := tc.gnarkClient.DvPInitiatorProof(
		core.KeyPair{PrivateKey: d.aliceSpend.PrivateKey, PublicKey: d.aliceSpend.PublicKey},
		d.aliceSaltField,     // salt of Alice's input ERC20 note
		d.erc20Amount,        // Alice sends 50 USDT
		d.erc20TokenId,       // tokenId=0 for ERC20
		d.bobSpend.PublicKey,
		d.bobView.EncapsKey,
		d.nftAmount,          // Bob delivers 1 NFT (valueBob for commitA)
		d.nftTokenId,         // Bob delivers the ticket NFT (tokenIdBob for commitA)
		big.NewInt(0),        // stTreeNumber
		d.aliceMerkleProof,
		tc.merkleDepth,
	)
	if err != nil { t.Fatalf("Alice DvPInitiatorProof: %v", err) }
	if len(aliceResult.Proof) != 8 {
		t.Fatalf("expected 8-element DvP Initiator proof, got %d", len(aliceResult.Proof))
	}

	commitA       := aliceResult.CommitA
	commitB       := aliceResult.CommitB
	revertCommitA := aliceResult.RevertCommitA

	// Verify DvP Initiator statement layout:
	// [stMsg=commitA, treeNum, root, nf_A, commitB, commitA, revertCommitA]
	if aliceResult.Statement[0].Cmp(commitA) != 0 {
		t.Fatalf("Alice DvP Initiator: statement[0] != commitA")
	}
	if aliceResult.Statement[4].Cmp(commitB) != 0 {
		t.Fatalf("Alice DvP Initiator: statement[4] != commitB")
	}
	if aliceResult.Statement[5].Cmp(commitA) != 0 {
		t.Fatalf("Alice DvP Initiator: statement[5] != commitA")
	}
	if aliceResult.Statement[6].Cmp(revertCommitA) != 0 {
		t.Fatalf("Alice DvP Initiator: statement[6] != revertCommitA")
	}
	t.Logf("commitA (Alice expects ERC721 tokenId=%s): %s", d.nftTokenId, commitA)
	t.Logf("commitB (Bob receives %s USDT):            %s", d.erc20Amount, commitB)
	t.Logf("revertCommitA (Alice fallback note):        %s", revertCommitA)
	t.Logf("Alice DvP Initiator proof verified ✓")

	aliceOnchainProof := proofStringsToOnchain(t, aliceResult.Proof)
	aliceReceipt := onchainProofReceipt{
		Proof:           aliceOnchainProof,
		Statement:       aliceResult.Statement,
		NumberOfInputs:  big.NewInt(int64(aliceResult.NumberOfInputs)),
		NumberOfOutputs: big.NewInt(int64(aliceResult.NumberOfOutputs)),
	}

	// ── Bob: DvP Destination proof ─────────────────────────────────────────────
	//
	// stMessage = commitB — the key in _pendingTransactions that matches Alice's stored receipt.
	// Proves: Bob's ERC721 input note is in the tree, and
	//   commitA = Poseidon4(aliceSpendPk, saltA, 1, nftTokenId)
	//          = Erc721Commitment(nftTokenId, aliceSpendPk, saltA)
	// where saltA was derived from Alice's KEM and is passed here off-chain.
	//
	// Statement: [commitB, treeNum, root, nf_B, commitA] (5 elements)
	// NumberOfInputs=1, NumberOfOutputs=1 (commitA goes into ERC721 vault during settlement)
	// Bob decapsulates Alice's ML-KEM ciphertext to derive saltB and Alice's payment details.
	ssBobLocal, err := core.Decapsulate(d.bobView.DecapsKey, aliceResult.CipherText)
	if err != nil { t.Fatalf("Bob Decapsulate: %v", err) }
	saltBBytesLocal, err := core.DerivePaymentSalt(ssBobLocal)
	if err != nil { t.Fatalf("Bob DerivePaymentSalt: %v", err) }
	saltBFieldLocal := core.SaltBToField(saltBBytesLocal)
	bobEncKeyLocal, err := core.DerivePaymentKey(ssBobLocal)
	if err != nil { t.Fatalf("Bob DerivePaymentKey: %v", err) }
	decTokenIdLocal, decAmountLocal, err := core.DecryptPayload(bobEncKeyLocal, aliceResult.EncTxData)
	if err != nil { t.Fatalf("Bob DecryptPayload: %v", err) }

	bobResult, err := tc.gnarkClient.DvPDestinationProof(
		core.KeyPair{PrivateKey: d.bobSpend.PrivateKey, PublicKey: d.bobSpend.PublicKey},
		d.bobTicketSalt,      // salt of Bob's input ERC721 note
		d.nftAmount,          // Bob delivers 1 NFT
		d.nftTokenId,         // Bob delivers the ticket NFT
		d.aliceSpend.PublicKey,
		aliceResult.SaltA,    // HKDF-derived from KEM; salt for Alice's NFT output commitment
		saltBFieldLocal,      // Bob's HKDF payment salt; salt for Bob's ERC20 output commitment
		decAmountLocal,       // Alice's ERC20 amount Bob receives
		decTokenIdLocal,      // Alice's ERC20 tokenId Bob receives
		commitA,              // Alice's expected output commitment (pre-computed by DvPInitiatorProof)
		big.NewInt(0),        // stTreeNumber
		d.bobMerkleProof,
		tc.merkleDepth,
	)
	if err != nil { t.Fatalf("Bob DvPDestinationProof: %v", err) }
	if len(bobResult.Proof) != 8 {
		t.Fatalf("expected 8-element DvP Destination proof, got %d", len(bobResult.Proof))
	}

	// Verify DvP Destination statement layout:
	// [stMsg=commitB, treeNum, root, nf_B, commitA]
	if bobResult.Statement[0].Cmp(commitB) != 0 {
		t.Fatalf("Bob DvP Destination: statement[0] != commitB")
	}
	if bobResult.Statement[4].Cmp(commitA) != 0 {
		t.Fatalf("Bob DvP Destination: statement[4] != commitA")
	}

	// Cross-commitment consistency:
	// Alice stMsg (statement[0]) == Bob output (statement[4]) == commitA ✓
	// Bob stMsg (statement[0]) == Alice first output (statement[4]) == commitB ✓
	if aliceResult.Statement[0].Cmp(bobResult.Statement[4]) != 0 {
		t.Fatalf("cross-check FAILED: Alice stMsg (commitA) != Bob output (commitA)")
	}
	if bobResult.Statement[0].Cmp(aliceResult.Statement[4]) != 0 {
		t.Fatalf("cross-check FAILED: Bob stMsg (commitB) != Alice output (commitB)")
	}
	t.Logf("Bob DvP Destination proof & cross-commitment consistency verified ✓")

	bobOnchainProof := proofStringsToOnchain(t, bobResult.Proof)
	bobReceipt := onchainProofReceipt{
		Proof:           bobOnchainProof,
		Statement:       bobResult.Statement,
		NumberOfInputs:  big.NewInt(int64(bobResult.NumberOfInputs)),
		NumberOfOutputs: big.NewInt(int64(bobResult.NumberOfOutputs)),
	}

	return &dvpProofs{
		commitA:       commitA,
		commitB:       commitB,
		revertCommitA: revertCommitA,
		aliceReceipt:  aliceReceipt,
		bobReceipt:    bobReceipt,
	}
}

// hardhatIncreaseTime advances Hardhat's clock by the given seconds and mines a block.
func hardhatIncreaseTime(t *testing.T, client *ethclient.Client, seconds int64) {
	t.Helper()
	var result interface{}
	if err := client.Client().CallContext(context.Background(), &result, "evm_increaseTime", seconds); err != nil {
		t.Fatalf("evm_increaseTime(%d): %v", seconds, err)
	}
	if err := client.Client().CallContext(context.Background(), &result, "evm_mine"); err != nil {
		t.Fatalf("evm_mine: %v", err)
	}
	t.Logf("Hardhat time advanced by %ds and a new block mined", seconds)
}

// currentBlockTimestamp returns the timestamp of the latest mined block.
func currentBlockTimestamp(t *testing.T, client *ethclient.Client) *big.Int {
	t.Helper()
	header, err := client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		t.Fatalf("HeaderByNumber: %v", err)
	}
	return new(big.Int).SetUint64(header.Time)
}

// ── Main test ─────────────────────────────────────────────────────────────────

func TestV2DvP_WithDeadline(t *testing.T) {
	if !chainAvailable() {
		t.Skip("Hardhat node not running on localhost:8545 — skipping on-chain test")
	}
	if !serverAvailable("localhost:8081") {
		t.Skip("gnark server not running on localhost:8081 — skipping on-chain test")
	}

	// ── Scenario 1: Full swap success ─────────────────────────────────────────
	t.Run("FullSwap", func(t *testing.T) {
		tc := newDvpTestContext(t)
		defer tc.client.Close()

		d := dvpSetupDeposits(t, tc, dvpTokenIdFull)
		p := dvpGenerateProofs(t, tc, d)

		// deadline = now + 1 hour (plenty of time)
		deadline := new(big.Int).Add(currentBlockTimestamp(t, tc.client), big.NewInt(3600))

		// ── Alice submits first leg ────────────────────────────────────────────
		// vaultId=0 (ERC20), groupId=0 (Fungibles)
		t.Logf("Alice submitting first leg (ERC20 JoinSplit) with deadline=%s", deadline)
		aliceTx, err := tc.dvp.Transact(tc.auth, "submitPartialSettlement",
			p.aliceReceipt,
			big.NewInt(0), // vaultId = VAULT_ID_ERC20
			big.NewInt(0), // groupId = GROUP_ID_FUNGIBLES
			deadline,
			// revertCommitA removed: contract now reads it from receipt.statement[6]
		)
		if err != nil { t.Fatalf("Alice submitPartialSettlement: %v", err) }
		aliceTxReceipt, err := bind.WaitMined(tc.ctx, tc.client, aliceTx)
		if err != nil { t.Fatalf("wait Alice submitPartialSettlement: %v", err) }
		t.Logf("Alice leg mined (block %d, gas %d)", aliceTxReceipt.BlockNumber, aliceTxReceipt.GasUsed)

		// Verify SwapInitiated event
		swapInitiated := false
		var emittedSwapId *big.Int
		for _, log := range aliceTxReceipt.Logs {
			if log.Topics[0] == tc.swapInitiatedSig {
				swapInitiated = true
				emittedSwapId = log.Topics[1].Big()
				t.Logf("  SwapInitiated — swapId=%s commitA=%s commitB=%s",
					emittedSwapId, log.Topics[2].Big(), log.Topics[3].Big())
			}
		}
		if !swapInitiated {
			t.Fatal("SwapInitiated event not found in Alice's TX receipt")
		}

		// ── Bob submits second leg ─────────────────────────────────────────────
		// vaultId=1 (ERC721), groupId=1 (NonFungibles)
		// deadline and revertCommitA are ignored on the second submission
		t.Logf("Bob submitting second leg (ERC721 Ownership)")
		bobTx, err := tc.dvp.Transact(tc.auth, "submitPartialSettlement",
			p.bobReceipt,
			big.NewInt(1), // vaultId = VAULT_ID_ERC721
			big.NewInt(1), // groupId = GROUP_ID_NON_FUNGIBLES
			big.NewInt(0), // deadline (ignored on second leg)
		)
		if err != nil { t.Fatalf("Bob submitPartialSettlement: %v", err) }
		bobTxReceipt, err := bind.WaitMined(tc.ctx, tc.client, bobTx)
		if err != nil { t.Fatalf("wait Bob submitPartialSettlement: %v", err) }
		t.Logf("Bob leg mined (block %d, gas %d) — swap settled atomically",
			bobTxReceipt.BlockNumber, bobTxReceipt.GasUsed)

		// ── Verify commitments inserted ───────────────────────────────────────
		foundCommitA, foundCommitB := false, false
		nullifierCount := 0
		for _, log := range bobTxReceipt.Logs {
			switch log.Topics[0] {
			case tc.commitmentSig:
				cmt := log.Topics[2].Big()
				if cmt.Cmp(p.commitB) == 0 {
					foundCommitB = true
					t.Logf("  commitB inserted (Bob's USDT note): %s", cmt)
				} else if cmt.Cmp(p.commitA) == 0 {
					foundCommitA = true
					t.Logf("  commitA inserted (Alice's ticket note): %s", cmt)
				}
			case tc.nullifierSig:
				nullifierCount++
			}
		}
		if !foundCommitA { t.Error("commitA (Alice's ticket) not found in settlement events") }
		if !foundCommitB { t.Error("commitB (Bob's USDT) not found in settlement events") }
		if nullifierCount == 0 { t.Error("expected at least one Nullifier event") }

		t.Logf("=== FullSwap COMPLETE: Alice burned 50 USDT → Bob | Bob burned ticket → Alice ===")
	})

	// ── Scenario 2: Deadline expired ─────────────────────────────────────────
	t.Run("DeadlineExpired", func(t *testing.T) {
		tc := newDvpTestContext(t)
		defer tc.client.Close()

		d := dvpSetupDeposits(t, tc, dvpTokenIdDeadline)
		p := dvpGenerateProofs(t, tc, d)

		// deadline = now + 60 seconds
		deadline := new(big.Int).Add(currentBlockTimestamp(t, tc.client), big.NewInt(dvpDeadlineSeconds))

		// ── Alice submits first leg ────────────────────────────────────────────
		t.Logf("Alice submitting first leg with 60s deadline=%s", deadline)
		aliceTx, err := tc.dvp.Transact(tc.auth, "submitPartialSettlement",
			p.aliceReceipt,
			big.NewInt(0),
			big.NewInt(0),
			deadline,
		)
		if err != nil { t.Fatalf("Alice submitPartialSettlement: %v", err) }
		aliceTxReceipt, err := bind.WaitMined(tc.ctx, tc.client, aliceTx)
		if err != nil { t.Fatalf("wait Alice submitPartialSettlement: %v", err) }
		t.Logf("Alice leg mined (block %d)", aliceTxReceipt.BlockNumber)

		// Verify SwapInitiated
		for _, log := range aliceTxReceipt.Logs {
			if log.Topics[0] == tc.swapInitiatedSig {
				t.Logf("  SwapInitiated — swapId=%s deadline=%s", log.Topics[1].Big(), deadline)
			}
		}

		// ── Bob does NOT submit. Advance time past deadline. ──────────────────
		t.Logf("Bob not responding. Advancing Hardhat time by %ds...", dvpDeadlineSeconds+5)
		hardhatIncreaseTime(t, tc.client, dvpDeadlineSeconds+5)

		// Verify that Bob's submission now fails (deadline expired)
		t.Logf("Verifying Bob's submission is rejected after deadline")
		_, errBob := tc.dvp.Transact(tc.auth, "submitPartialSettlement",
			p.bobReceipt,
			big.NewInt(1),
			big.NewInt(1),
			big.NewInt(0),
		)
		if errBob == nil {
			t.Error("expected Bob's post-deadline submission to fail, but it succeeded")
		} else {
			t.Logf("  Bob's post-deadline submission correctly rejected: %v", errBob)
		}

		// ── Alice claims timeout — revert commitment back to Alice ─────────────
		// pendingReceiptId = commitB (Alice's first output = the key in _pendingTransactions)
		t.Logf("Alice calling claimSwapTimeout(commitB=%s)", p.commitB)
		timeoutTx, err := tc.dvp.Transact(tc.auth, "claimSwapTimeout", p.commitB)
		if err != nil { t.Fatalf("claimSwapTimeout: %v", err) }
		timeoutReceipt, err := bind.WaitMined(tc.ctx, tc.client, timeoutTx)
		if err != nil { t.Fatalf("wait claimSwapTimeout: %v", err) }
		t.Logf("claimSwapTimeout mined (block %d, gas %d)",
			timeoutReceipt.BlockNumber, timeoutReceipt.GasUsed)

		// Verify SwapTimedOut event
		timedOut := false
		for _, log := range timeoutReceipt.Logs {
			if log.Topics[0] == tc.swapTimedOutSig {
				timedOut = true
				t.Logf("  SwapTimedOut event — pendingReceiptId=%s (= commitB)", log.Topics[1].Big())
			}
			if log.Topics[0] == tc.nullifierSig {
				t.Logf("  CoinUnlocked — nf=%s (Alice's nullifier is now free)", log.Topics[3].Big())
			}
		}
		if !timedOut {
			t.Error("SwapTimedOut event not found in timeout TX receipt")
		}

		// Verify that a second claimSwapTimeout fails (entry deleted)
		_, errDouble := tc.dvp.Transact(tc.auth, "claimSwapTimeout", p.commitB)
		if errDouble == nil {
			t.Error("second claimSwapTimeout should fail (SwapNotFound) but succeeded")
		} else {
			t.Logf("  Second claimSwapTimeout correctly rejected (swap cleaned up): %v", errDouble)
		}

		// Alice's original commitment is still in the ERC20 Merkle tree.
		// Now that nf_A is unlocked, Alice can spend her original note
		// and output revertCommitA. We verify the tree still contains aliceCommitment.
		mt20 := loadVaultMerkleTree(t, tc.client, tc.erc20VaultAddr, tc.merkleDepth)
		aliceProofAgain, err := mt20.GenerateProof(d.aliceCommitment)
		if err != nil {
			t.Errorf("Alice's original commitment no longer provable in tree: %v", err)
		} else {
			t.Logf("  Alice's commitment still in ERC20 tree (root=%s) — funds recoverable via revertCommitA",
				aliceProofAgain.Root)
		}

		t.Logf("=== DeadlineExpired COMPLETE: swap timed out, Alice's nullifier unlocked, revert commitment available ===")
		t.Logf("    revertCommitA = %s", p.revertCommitA)
	})

	// ── Scenario 3: Bob submits invalid proof, then Alice claims timeout ──────
	t.Run("InvalidProof", func(t *testing.T) {
		tc := newDvpTestContext(t)
		defer tc.client.Close()

		d := dvpSetupDeposits(t, tc, dvpTokenIdInvalid)
		p := dvpGenerateProofs(t, tc, d)

		deadline := new(big.Int).Add(currentBlockTimestamp(t, tc.client), big.NewInt(dvpDeadlineSeconds))

		// ── Alice submits first leg ────────────────────────────────────────────
		t.Logf("Alice submitting first leg with 60s deadline=%s", deadline)
		aliceTx, err := tc.dvp.Transact(tc.auth, "submitPartialSettlement",
			p.aliceReceipt,
			big.NewInt(0),
			big.NewInt(0),
			deadline,
		)
		if err != nil { t.Fatalf("Alice submitPartialSettlement: %v", err) }
		aliceTxReceipt, err := bind.WaitMined(tc.ctx, tc.client, aliceTx)
		if err != nil { t.Fatalf("wait Alice submitPartialSettlement: %v", err) }
		t.Logf("Alice leg mined (block %d)", aliceTxReceipt.BlockNumber)

		// ── Bob submits with invalid (all-zero) proof ─────────────────────────
		// The on-chain ZK verifier will reject the pairing check.
		invalidProof := onchainSnarkProof{
			A: onchainG1Point{X: big.NewInt(1), Y: big.NewInt(2)}, // minimal non-zero point
			B: onchainG2Point{
				X: [2]*big.Int{big.NewInt(0), big.NewInt(1)},
				Y: [2]*big.Int{big.NewInt(0), big.NewInt(1)},
			},
			C: onchainG1Point{X: big.NewInt(1), Y: big.NewInt(2)},
		}
		invalidBobReceipt := onchainProofReceipt{
			Proof:           invalidProof,
			Statement:       p.bobReceipt.Statement, // correct statement, wrong proof
			NumberOfInputs:  p.bobReceipt.NumberOfInputs,
			NumberOfOutputs: p.bobReceipt.NumberOfOutputs,
		}

		t.Logf("Bob submitting with invalid proof (pairing check will fail)")
		_, errInvalid := tc.dvp.Transact(tc.auth, "submitPartialSettlement",
			invalidBobReceipt,
			big.NewInt(1),
			big.NewInt(1),
			big.NewInt(0),
		)
		if errInvalid == nil {
			t.Error("expected Bob's invalid proof to be rejected, but TX was accepted")
		} else {
			t.Logf("  Bob's invalid proof correctly rejected by on-chain verifier: %v", errInvalid)
		}

		// Wait enough time between retries to avoid nonce issues
		time.Sleep(500 * time.Millisecond)

		// ── Advance time past deadline ─────────────────────────────────────────
		t.Logf("Advancing Hardhat time by %ds past deadline...", dvpDeadlineSeconds+5)
		hardhatIncreaseTime(t, tc.client, dvpDeadlineSeconds+5)

		// ── Alice claims timeout ───────────────────────────────────────────────
		t.Logf("Alice calling claimSwapTimeout(commitB=%s)", p.commitB)
		timeoutTx, err := tc.dvp.Transact(tc.auth, "claimSwapTimeout", p.commitB)
		if err != nil { t.Fatalf("claimSwapTimeout: %v", err) }
		timeoutReceipt, err := bind.WaitMined(tc.ctx, tc.client, timeoutTx)
		if err != nil { t.Fatalf("wait claimSwapTimeout: %v", err) }
		t.Logf("claimSwapTimeout mined (block %d, gas %d)",
			timeoutReceipt.BlockNumber, timeoutReceipt.GasUsed)

		timedOut := false
		for _, log := range timeoutReceipt.Logs {
			if log.Topics[0] == tc.swapTimedOutSig {
				timedOut = true
				t.Logf("  SwapTimedOut event — pendingReceiptId=%s", log.Topics[1].Big())
			}
			if log.Topics[0] == tc.nullifierSig {
				t.Logf("  CoinUnlocked — Alice's nullifier freed for re-use")
			}
		}
		if !timedOut {
			t.Error("SwapTimedOut event not found after invalid proof scenario")
		}

		// Alice's commitment is still in tree — she can spend with revertCommitA
		mt20 := loadVaultMerkleTree(t, tc.client, tc.erc20VaultAddr, tc.merkleDepth)
		_, err = mt20.GenerateProof(d.aliceCommitment)
		if err != nil {
			t.Errorf("Alice's commitment no longer provable in tree: %v", err)
		} else {
			t.Logf("  Alice's commitment still spendable — revertCommitA=%s recoverable", p.revertCommitA)
		}

		t.Logf("=== InvalidProof COMPLETE: Bob's proof rejected, swap timed out, Alice's funds recoverable ===")
		t.Logf("    revertCommitA = %s", p.revertCommitA)
	})
}
