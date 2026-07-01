package tests

// On-chain integration test for the DvP (Delivery vs Payment) swap flow.
//
// Scenario:
//   Alice has 30 USDT (ERC20) in the fungible vault.
//   Bob   has an ERC721 ticket (tokenId=42, amount=1) in the NFT vault.
//   They swap: Alice delivers 30 USDT to Bob; Bob delivers the ticket to Alice.
//
// Step 1 — Alice deposits 30 USDT into the ERC20 vault (ML-KEM deposit).
// Step 2 — Bob deposits ERC721 ticket (tokenId=42) into the NFT vault.
// Step 3 — DvPInitiator: Alice runs DvPInitiatorCircuit proof.
//             - Proves ownership of her 30 USDT note (valueIn=30, tokenIdIn=0).
//             - COMMIT_B  = Bob receives 30 USDT   (amount=30, tokenId=0).
//             - COMMIT_A  = Alice receives the ticket (amount=1, tokenId=42).
//             - REVERT_COMMIT_A = Alice's 30 USDT back if timeout.
//             - Encapsulates Bob's view key → CTXT + ENC_TX_DATA.
// Step 4 — Bob scans: decapsulates CTXT, verifies COMMIT_B and COMMIT_A.
// Step 5 — DvPDestination: Bob runs DvPDestinationCircuit proof.
//             - Proves ownership of his ticket note (valueIn=1, tokenIdIn=42).
//             - Proves COMMIT_A encodes the same amount/token as his note.
//
// Prerequisites (all must be running/completed before this test):
//   1. Hardhat node:      npx hardhat node
//   2. Deploy + init:     see MEMORY.md or 10_v2_erc20_payment_test.go header
//   3. Generate DvP keys: cd gnark_circuits && go run generation.go
//   4. Gnark server:      cd gnark_circuits && go run main.go
//
// Run with:
//   cd test && CC=/usr/bin/clang go test -run TestV2DvP -v -timeout 600s

import (
	"context"
	"math/big"
	"testing"

	"github.com/raylsnetwork/enygma_dvp/src/core"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

func TestV2DvP(t *testing.T) {
	if !chainAvailable() {
		t.Skip("Hardhat node not running on localhost:8545 — skipping on-chain test")
	}
	if !serverAvailable("localhost:8081") {
		t.Skip("gnark server not running on localhost:8081 — skipping on-chain test")
	}

	ctx := context.Background()

	// ── Connect ───────────────────────────────────────────────────────────────
	client, err := ethclient.Dial(hardhatRPC)
	if err != nil {
		t.Fatalf("ethclient.Dial: %v", err)
	}
	defer client.Close()

	// ── Load contract addresses ───────────────────────────────────────────────
	receipts := loadOnchainReceipts(t)
	erc20VaultAddr := common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress)
	erc20Addr      := common.HexToAddress(receipts["ERC20"].ContractAddress)
	nftVaultAddr   := common.HexToAddress(receipts["Erc721CoinVault"].ContractAddress)
	erc721Addr     := common.HexToAddress(receipts["ERC721"].ContractAddress)

	// ── ABIs ──────────────────────────────────────────────────────────────────
	erc20VaultABI := loadOnchainABI(t, "core/contracts/vaults/Erc20CoinVault.sol/Erc20CoinVault.json")
	erc20ABI      := loadOnchainABI(t, "erc20/contracts/RaylsERC20.sol/RaylsERC20.json")
	nftVaultABI   := loadOnchainABI(t, "core/contracts/vaults/Erc721CoinVault.sol/Erc721CoinVault.json")
	erc721ABI     := loadOnchainABI(t, "erc721/contracts/RaylsERC721.sol/RaylsERC721.json")

	erc20Vault := bind.NewBoundContract(erc20VaultAddr, erc20VaultABI, client, client, client)
	erc20      := bind.NewBoundContract(erc20Addr,      erc20ABI,      client, client, client)
	nftVault   := bind.NewBoundContract(nftVaultAddr,   nftVaultABI,   client, client, client)
	erc721     := bind.NewBoundContract(erc721Addr,     erc721ABI,     client, client, client)

	auth := hardhatAuth(t, client)
	addr := auth.From

	gnarkClient := core.NewGnarkClient("http://localhost:8081")
	merkleDepth := 8

	// Alice delivers: 30 USDT (ERC20), tokenId=0
	erc20Amount  := big.NewInt(30)
	erc20TokenId := big.NewInt(0)

	// Bob delivers: ERC721 ticket tokenId=42, amount=1
	nftTokenId := big.NewInt(42)
	nftAmount  := big.NewInt(1)

	// ─────────────────────────────────────────────────────────────────────────
	// Step 1 — Alice deposits 30 USDT into the ERC20 vault
	// ─────────────────────────────────────────────────────────────────────────
	aliceSpend, err := core.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("Alice NewSpendKeyPair: %v", err)
	}
	aliceView, err := core.NewViewKeyPair()
	if err != nil {
		t.Fatalf("Alice NewViewKeyPair: %v", err)
	}

	// ERC20 deposit uses ML-KEM so Alice can later derive her note's salt.
	ssAlice, capsuleAlice, err := core.Encapsulate(aliceView.EncapsKey)
	if err != nil {
		t.Fatalf("Encapsulate (Alice deposit): %v", err)
	}
	aliceSaltBytes, err := core.DerivePaymentSalt(ssAlice)
	if err != nil {
		t.Fatalf("DerivePaymentSalt (Alice): %v", err)
	}
	aliceEncKey, err := core.DerivePaymentKey(ssAlice)
	if err != nil {
		t.Fatalf("DerivePaymentKey (Alice): %v", err)
	}
	aliceSaltField := core.SaltBToField(aliceSaltBytes)

	aliceCommitment, err := core.Erc20CommitmentV2(aliceSpend.PublicKey, aliceSaltField, erc20Amount, erc20TokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 (Alice): %v", err)
	}
	aliceDepositEnc, err := core.EncryptPayload(aliceEncKey, erc20TokenId, erc20Amount)
	if err != nil {
		t.Fatalf("EncryptPayload (Alice deposit): %v", err)
	}

	mintErc20Tx, err := erc20.Transact(auth, "mint", addr, new(big.Int).Mul(erc20Amount, big.NewInt(10)))
	if err != nil {
		t.Fatalf("ERC20.mint: %v", err)
	}
	if _, err := bind.WaitMined(ctx, client, mintErc20Tx); err != nil {
		t.Fatalf("wait ERC20 mint: %v", err)
	}
	approveErc20Tx, err := erc20.Transact(auth, "approve", erc20VaultAddr, erc20Amount)
	if err != nil {
		t.Fatalf("ERC20.approve: %v", err)
	}
	if _, err := bind.WaitMined(ctx, client, approveErc20Tx); err != nil {
		t.Fatalf("wait ERC20 approve: %v", err)
	}
	// C-1 fix: contract computes commitment on-chain from key material.
	depositErc20Tx, err := erc20Vault.Transact(auth, "depositV2",
		[]*big.Int{erc20Amount, aliceSpend.PublicKey, aliceSaltField, erc20TokenId}, capsuleAlice, aliceDepositEnc)
	if err != nil {
		t.Fatalf("erc20Vault.depositV2: %v", err)
	}
	if _, err := bind.WaitMined(ctx, client, depositErc20Tx); err != nil {
		t.Fatalf("wait ERC20 depositV2: %v", err)
	}
	t.Logf("Step 1 — Alice deposited %s USDT (commitment %s)", erc20Amount, aliceCommitment)

	// ─────────────────────────────────────────────────────────────────────────
	// Step 2 — Bob deposits ERC721 ticket (tokenId=42) into the NFT vault
	// ─────────────────────────────────────────────────────────────────────────
	bobSpend, err := core.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("Bob NewSpendKeyPair: %v", err)
	}
	bobView, err := core.NewViewKeyPair()
	if err != nil {
		t.Fatalf("Bob NewViewKeyPair: %v", err)
	}

	// ERC721 deposit uses a plain random salt (no ML-KEM).
	bobNftSalt, err := core.RandomInField()
	if err != nil {
		t.Fatalf("Bob RandomInField (salt): %v", err)
	}

	// Commitment = Poseidon4(bobSpendPK, bobNftSalt, 1, nftTokenId)
	bobCommitment, err := core.Erc721Commitment(nftTokenId, bobSpend.PublicKey, bobNftSalt)
	if err != nil {
		t.Fatalf("Erc721Commitment (Bob): %v", err)
	}

	mintNftTx, err := erc721.Transact(auth, "mint", addr, nftTokenId)
	if err != nil {
		t.Fatalf("ERC721.mint: %v", err)
	}
	if _, err := bind.WaitMined(ctx, client, mintNftTx); err != nil {
		t.Fatalf("wait ERC721 mint: %v", err)
	}
	approveNftTx, err := erc721.Transact(auth, "approve", nftVaultAddr, nftTokenId)
	if err != nil {
		t.Fatalf("ERC721.approve: %v", err)
	}
	if _, err := bind.WaitMined(ctx, client, approveNftTx); err != nil {
		t.Fatalf("wait ERC721 approve: %v", err)
	}
	// C-1 fix: contract computes commitment = Poseidon4(pkSpend, salt, 1, tokenId) on-chain.
	depositNftTx, err := nftVault.Transact(auth, "deposit", []*big.Int{nftTokenId, bobSpend.PublicKey, bobNftSalt})
	if err != nil {
		t.Fatalf("nftVault.deposit: %v", err)
	}
	if _, err := bind.WaitMined(ctx, client, depositNftTx); err != nil {
		t.Fatalf("wait NFT deposit: %v", err)
	}
	t.Logf("Step 2 — Bob deposited ticket tokenId=%s (commitment %s)", nftTokenId, bobCommitment)

	// Build separate Merkle trees — one per vault.
	erc20Mt := loadVaultMerkleTree(t, client, erc20VaultAddr, merkleDepth)
	nftMt   := loadVaultMerkleTree(t, client, nftVaultAddr,   merkleDepth)

	aliceProof, err := erc20Mt.GenerateProof(aliceCommitment)
	if err != nil {
		t.Fatalf("GenerateProof (Alice ERC20): %v", err)
	}
	bobProof, err := nftMt.GenerateProof(bobCommitment)
	if err != nil {
		t.Fatalf("GenerateProof (Bob NFT): %v", err)
	}
	t.Logf("ERC20 Merkle root: %s", aliceProof.Root)
	t.Logf("NFT  Merkle root:  %s", bobProof.Root)

	// ─────────────────────────────────────────────────────────────────────────
	// Step 3 — Alice runs DvPInitiatorCircuit proof
	//
	// Alice delivers 30 USDT (amount=30, tokenId=0).
	// Alice expects the ticket (amount=1, tokenId=42) from Bob.
	// ─────────────────────────────────────────────────────────────────────────
	initiatorResult, err := gnarkClient.DvPInitiatorProof(
		core.KeyPair{PrivateKey: aliceSpend.PrivateKey, PublicKey: aliceSpend.PublicKey},
		aliceSaltField,  // Alice's ERC20 deposit salt
		erc20Amount,     // Alice delivers amount=30
		erc20TokenId,    // Alice delivers tokenId=0
		bobSpend.PublicKey,
		bobView.EncapsKey,
		nftAmount,   // Alice expects amount=1
		nftTokenId,  // Alice expects tokenId=42
		big.NewInt(0),
		aliceProof,
		merkleDepth,
	)
	if err != nil {
		t.Fatalf("DvPInitiatorProof: %v", err)
	}
	if len(initiatorResult.Proof) != 8 {
		t.Fatalf("expected 8-element proof, got %d", len(initiatorResult.Proof))
	}
	t.Logf("Step 3 — DvPInitiator proof generated")
	t.Logf("  COMMIT_B  (Bob gets 30 USDT):          %s", initiatorResult.CommitB)
	t.Logf("  COMMIT_A  (Alice gets ticket tokenId=%s): %s", nftTokenId, initiatorResult.CommitA)
	t.Logf("  REVERT_COMMIT_A:                        %s", initiatorResult.RevertCommitA)

	// ─────────────────────────────────────────────────────────────────────────
	// Step 4 — Bob scans and verifies COMMIT_B and COMMIT_A
	//
	// Bob decapsulates Alice's CTXT to recover ss_B, then re-derives all salts.
	// ─────────────────────────────────────────────────────────────────────────
	ssBobDerived, err := core.Decapsulate(bobView.DecapsKey, initiatorResult.CipherText)
	if err != nil {
		t.Fatalf("Bob Decapsulate: %v", err)
	}
	bobSaltBDerived, err := core.DerivePaymentSalt(ssBobDerived)
	if err != nil {
		t.Fatalf("Bob DerivePaymentSalt: %v", err)
	}
	bobSaltADerived, err := core.DeriveDvpSaltInit(ssBobDerived)
	if err != nil {
		t.Fatalf("Bob DeriveDvpSaltInit: %v", err)
	}
	bobEncKeyDerived, err := core.DerivePaymentKey(ssBobDerived)
	if err != nil {
		t.Fatalf("Bob DerivePaymentKey: %v", err)
	}

	saltBDerivedField := core.SaltBToField(bobSaltBDerived)
	saltADerivedField := core.SaltBToField(bobSaltADerived)

	// Decrypt ENC_TX_DATA → Alice is delivering tokenId=0, amount=30 (USDT)
	decTokenId, decAmount, err := core.DecryptPayload(bobEncKeyDerived, initiatorResult.EncTxData)
	if err != nil {
		t.Fatalf("Bob DecryptPayload: %v", err)
	}
	t.Logf("Step 4 — Bob decrypted: tokenId=%s amount=%s (Alice's USDT)", decTokenId, decAmount)

	// Verify COMMIT_B: Bob will receive 30 USDT.
	expectedCommitB, err := core.Erc20CommitmentV2(bobSpend.PublicKey, saltBDerivedField, decAmount, decTokenId)
	if err != nil {
		t.Fatalf("Bob Erc20CommitmentV2 (COMMIT_B): %v", err)
	}
	if expectedCommitB.Cmp(initiatorResult.CommitB) != 0 {
		t.Fatalf("COMMIT_B mismatch: got %s, want %s", expectedCommitB, initiatorResult.CommitB)
	}
	t.Logf("Step 4 — COMMIT_B verified (Bob receives %s USDT)", decAmount)

	// Verify COMMIT_A: Alice will receive the ticket (amount=1, tokenId=42).
	expectedCommitA, err := core.Erc20CommitmentV2(aliceSpend.PublicKey, saltADerivedField, nftAmount, nftTokenId)
	if err != nil {
		t.Fatalf("Bob Erc20CommitmentV2 (COMMIT_A): %v", err)
	}
	if expectedCommitA.Cmp(initiatorResult.CommitA) != 0 {
		t.Fatalf("COMMIT_A mismatch: got %s, want %s", expectedCommitA, initiatorResult.CommitA)
	}
	t.Logf("Step 4 — COMMIT_A verified (Alice receives ticket tokenId=%s)", nftTokenId)

	// ─────────────────────────────────────────────────────────────────────────
	// Step 5 — Bob runs DvPDestinationCircuit proof
	//
	// Bob proves ownership of his ticket note and that COMMIT_A encodes
	// the same amount and tokenId as his note.
	// ─────────────────────────────────────────────────────────────────────────
	destinationResult, err := gnarkClient.DvPDestinationProof(
		core.KeyPair{PrivateKey: bobSpend.PrivateKey, PublicKey: bobSpend.PublicKey},
		bobNftSalt,    // Bob's NFT deposit salt (plain random)
		nftAmount,     // Bob delivers amount=1
		nftTokenId,    // Bob delivers tokenId=42
		aliceSpend.PublicKey,
		saltADerivedField,       // HKDF(ss_B, "Init Salt") — salt for Alice's output (she receives NFT)
		saltBDerivedField,       // HKDF(ss_B, "note salt") — salt for Bob's output (he receives USDT)
		decAmount,               // Alice's ERC20 amount that Bob receives
		decTokenId,              // Alice's ERC20 tokenId that Bob receives
		initiatorResult.CommitA, // COMMIT_A from Alice's proof
		big.NewInt(0),
		bobProof,
		merkleDepth,
	)
	if err != nil {
		t.Fatalf("DvPDestinationProof: %v", err)
	}
	if len(destinationResult.Proof) != 8 {
		t.Fatalf("expected 8-element proof, got %d", len(destinationResult.Proof))
	}
	t.Logf("Step 5 — DvPDestination proof generated")
	t.Logf("  statement: %v", destinationResult.Statement)

	t.Logf("=== DvP COMPLETE: Alice delivers %s USDT → Bob | Bob delivers ticket (tokenId=%s) → Alice ===",
		erc20Amount, nftTokenId)
}
