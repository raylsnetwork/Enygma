package tests

// TestAuditedPaymentViaRelayer is the full end-to-end test combining the
// relayer payment flow with auditor view-key recovery and note scanning.
//
// Flow:
//  1.  Generate auditor ML-KEM-768 keypair.
//  2.  Alice and Bob generate spend + view keypairs.
//  3.  Encrypt each user's sk_view for the auditor (mlKemCt as AEAD AAD).
//  4.  Register Alice and Bob on-chain with audit ciphertexts.
//  5.  Alice deposits 40 ERC-20 tokens into the private vault.
//  6.  Alice requests a ZK proof: pay 30 to Bob, keep 10 change.
//  7.  Alice sends the unsigned proof to the relayer (POST /relay/payment).
//      Relayer submits the transaction — Alice's Ethereum address is never exposed.
//  8.  Verify Payment and Nullifier events on-chain.
//  9.  Bob scans his note directly using his own view key (baseline).
//  10. Auditor reads (mlKemCt, aesCt) for every registered user from UserRegistry.
//  11. Auditor recovers each user's DecapsKey via AuditorDecryptViewKey.
//  12. Auditor performs O(N) scan:
//        - Alice's recovered key → no notes (change has no on-chain ciphertext).
//        - Bob's recovered key   → 1 note, amount 30 ✓
//  13. Verify auditor's note matches Bob's direct scan.

import (
	"bytes"
	"context"
	"math/big"
	"net/http"
	"strings"
	"testing"

	dvpcore "github.com/raylsnetwork/enygma_dvp/src/core"
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

func TestAuditedPaymentViaRelayer(t *testing.T) {
	if !chainAvailable() {
		t.Skip("Hardhat node not running on localhost:8545 — skipping")
	}
	if !serverAvailable("localhost:8082") {
		t.Skip("gnark server not running on localhost:8082 — skipping")
	}
	if !serverAvailable("localhost:8090") {
		t.Skip("relayer not running on localhost:8090 — skipping")
	}

	ctx := context.Background()

	client, err := ethclient.Dial(hardhatRPC)
	if err != nil {
		t.Fatalf("ethclient.Dial: %v", err)
	}
	defer client.Close()

	receipts := loadOnchainReceipts(t)
	vaultAddr    := common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress)
	erc20Addr    := common.HexToAddress(receipts["ERC20"].ContractAddress)
	registryAddr := common.HexToAddress(receipts["UserRegistry"].ContractAddress)

	vaultABI := loadOnchainABI(t, "Erc20CoinVault")
	erc20ABI  := loadOnchainABI(t, "RaylsERC20")

	vault := bind.NewBoundContract(vaultAddr, vaultABI, client, client, client)
	erc20 := bind.NewBoundContract(erc20Addr, erc20ABI, client, client, client)

	aliceAuth := hardhatAuth(t, client)
	bobAuth   := hardhatBobAuth(t, client)

	gnarkClient := rpcore.NewPaymentClient("")
	merkleDepth := 8
	tokenId      := big.NewInt(0)
	depositAmt   := big.NewInt(40)
	paymentAmt   := big.NewInt(30)
	changeAmt    := big.NewInt(10)

	// ── Step 1: Auditor keypair ───────────────────────────────────────────────
	t.Log("Step 1 — generating auditor ML-KEM-768 keypair")

	auditorPair, err := rpcore.NewAuditorKeyPair()
	if err != nil {
		t.Fatalf("NewAuditorKeyPair: %v", err)
	}
	t.Logf("  auditor encapsulation key: %d bytes", len(auditorPair.EncapsKey))

	// ── Step 2: User keypairs ─────────────────────────────────────────────────
	t.Log("Step 2 — generating Alice and Bob's spend and view keypairs")

	aliceSpend, err := rpcore.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("NewSpendKeyPair (Alice): %v", err)
	}
	aliceView, err := rpcore.NewViewKeyPair()
	if err != nil {
		t.Fatalf("NewViewKeyPair (Alice): %v", err)
	}
	bobSpend, err := rpcore.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("NewSpendKeyPair (Bob): %v", err)
	}
	bobView, err := rpcore.NewViewKeyPair()
	if err != nil {
		t.Fatalf("NewViewKeyPair (Bob): %v", err)
	}
	t.Logf("  Alice pk_spend: %s", aliceSpend.PublicKey)
	t.Logf("  Bob   pk_spend: %s", bobSpend.PublicKey)

	// ── Step 3: Encrypt view keys for auditor ────────────────────────────────
	t.Log("Step 3 — encrypting view secret keys for auditor (mlKemCt as AEAD AAD)")

	aliceMlKemCt, aliceAesCt, err := rpcore.EncryptViewKeyForAuditor(auditorPair.EncapsKey, aliceView.DecapsKey)
	if err != nil {
		t.Fatalf("EncryptViewKeyForAuditor (Alice): %v", err)
	}
	bobMlKemCt, bobAesCt, err := rpcore.EncryptViewKeyForAuditor(auditorPair.EncapsKey, bobView.DecapsKey)
	if err != nil {
		t.Fatalf("EncryptViewKeyForAuditor (Bob): %v", err)
	}
	t.Logf("  Alice: mlKemCt=%d bytes  aesCt=%d bytes", len(aliceMlKemCt), len(aliceAesCt))
	t.Logf("  Bob:   mlKemCt=%d bytes  aesCt=%d bytes", len(bobMlKemCt), len(bobAesCt))

	// ── Step 4: Registration ──────────────────────────────────────────────────
	t.Log("Step 4 — registering Alice and Bob on-chain with audit ciphertexts")

	if err := rpcore.Register(client, aliceAuth, registryAddr,
		aliceSpend.PublicKey, aliceView.EncapsKey, aliceMlKemCt, aliceAesCt); err != nil {
		if !strings.Contains(err.Error(), "AlreadyRegistered") && !strings.Contains(err.Error(), "45ed80e9") {
			t.Fatalf("Register (Alice): %v", err)
		}
		t.Logf("  Alice already registered — skipping")
	} else {
		t.Logf("  Alice registered (%s)", aliceAuth.From.Hex())
	}

	if err := rpcore.Register(client, bobAuth, registryAddr,
		bobSpend.PublicKey, bobView.EncapsKey, bobMlKemCt, bobAesCt); err != nil {
		if !strings.Contains(err.Error(), "AlreadyRegistered") && !strings.Contains(err.Error(), "45ed80e9") {
			t.Fatalf("Register (Bob): %v", err)
		}
		t.Logf("  Bob already registered — skipping")
	} else {
		t.Logf("  Bob registered (%s)", bobAuth.From.Hex())
	}

	// ── Step 5: Alice deposits 40 tokens ────────────────────────────────────
	t.Log("Step 5 — Alice deposits 40 tokens into the private vault")

	mintTx, err := erc20.Transact(aliceAuth, "mint", aliceAuth.From,
		new(big.Int).Mul(depositAmt, big.NewInt(10)))
	if err != nil {
		t.Fatalf("ERC20.mint: %v", err)
	}
	if _, err := bind.WaitMined(ctx, client, mintTx); err != nil {
		t.Fatalf("wait mint: %v", err)
	}

	approveTx, err := erc20.Transact(aliceAuth, "approve", vaultAddr, depositAmt)
	if err != nil {
		t.Fatalf("ERC20.approve: %v", err)
	}
	if _, err := bind.WaitMined(ctx, client, approveTx); err != nil {
		t.Fatalf("wait approve: %v", err)
	}

	ss, capsule, err := rpcore.Encapsulate(aliceView.EncapsKey)
	if err != nil {
		t.Fatalf("Encapsulate: %v", err)
	}
	aliceSaltB, err := rpcore.DerivePaymentSalt(ss)
	if err != nil {
		t.Fatalf("DerivePaymentSalt: %v", err)
	}
	aliceEncKey, err := rpcore.DerivePaymentKey(ss)
	if err != nil {
		t.Fatalf("DerivePaymentKey: %v", err)
	}
	aliceSaltBField := rpcore.SaltBToField(aliceSaltB)
	aliceCommitment, err := rpcore.Erc20CommitmentV2(
		aliceSpend.PublicKey, aliceSaltBField, depositAmt, tokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2: %v", err)
	}
	aliceDepositCtxt, err := rpcore.EncryptPayload(aliceEncKey, tokenId, depositAmt)
	if err != nil {
		t.Fatalf("EncryptPayload: %v", err)
	}

	depositTx, err := vault.Transact(aliceAuth, "depositV2",
		[]*big.Int{depositAmt, aliceCommitment}, capsule, aliceDepositCtxt)
	if err != nil {
		t.Fatalf("vault.depositV2: %v", err)
	}
	depositReceipt, err := bind.WaitMined(ctx, client, depositTx)
	if err != nil {
		t.Fatalf("wait depositV2: %v", err)
	}
	t.Logf("  depositV2 mined (block %d, gas %d, commitment %s)",
		depositReceipt.BlockNumber, depositReceipt.GasUsed, aliceCommitment)

	mt := loadVaultMerkleTree(t, client, vaultAddr, merkleDepth)
	aliceProof, err := mt.GenerateProof(aliceCommitment)
	if err != nil {
		t.Fatalf("GenerateProof: %v", err)
	}
	t.Logf("  Merkle root: %s", aliceProof.Root)

	// ── Step 6: ZK proof ──────────────────────────────────────────────────────
	t.Log("Step 6 — requesting ZK proof: pay 30 to Bob, keep 10 change")

	vaultAddrBig := new(big.Int).SetBytes(vaultAddr.Bytes())
	paymentResult, err := gnarkClient.BoundPaymentProof(
		vaultAddrBig,
		big.NewInt(0),
		[]*big.Int{depositAmt},
		[]rpcore.KeyPair{
			{PrivateKey: aliceSpend.PrivateKey, PublicKey: aliceSpend.PublicKey},
		},
		[]*big.Int{aliceSaltBField},
		[]*big.Int{paymentAmt, changeAmt},
		[]*big.Int{bobSpend.PublicKey, aliceSpend.PublicKey},
		[][]byte{bobView.EncapsKey, aliceView.EncapsKey},
		merkleDepth,
		[]*rpcore.MerkleProof{aliceProof},
		[]*big.Int{big.NewInt(0)},
		tokenId,
	)
	if err != nil {
		t.Fatalf("BoundPaymentProof: %v", err)
	}
	bobCommitment   := paymentResult.ContractStatement()[4]
	aliceChangeCmt  := paymentResult.ContractStatement()[5]
	t.Logf("  proof generated")
	t.Logf("  Bob's output commitment:   %s", bobCommitment)
	t.Logf("  Alice's change commitment: %s", aliceChangeCmt)

	// ── Step 7: Relay ─────────────────────────────────────────────────────────
	t.Log("Step 7 — Alice sends unsigned proof to relayer (POST /relay/payment)")

	relayReq := buildRelayRequest(t, paymentResult, "0")
	relayResp, statusCode, err := postToRelayer(t, relayerAPIKey, relayReq)
	if err != nil {
		t.Fatalf("postToRelayer: %v", err)
	}
	if statusCode != http.StatusOK {
		t.Fatalf("relayer returned %d: %s", statusCode, relayResp.Error)
	}
	t.Logf("  relayer response: txHash=%s block=%d gas=%d",
		relayResp.TxHash, relayResp.BlockNumber, relayResp.GasUsed)

	// ── Step 8: Verify on-chain events ────────────────────────────────────────
	t.Log("Step 8 — verifying on-chain Payment and Nullifier events")

	txHash := common.HexToHash(relayResp.TxHash)
	txReceipt, err := client.TransactionReceipt(ctx, txHash)
	if err != nil {
		t.Fatalf("TransactionReceipt: %v", err)
	}

	paymentSig   := crypto.Keccak256Hash([]byte("Payment(uint256,uint256,bytes,bytes)"))
	nullifierSig := crypto.Keccak256Hash([]byte("Nullifier(uint256,uint256,uint256)"))

	var paymentCount, nullifierCount int
	for _, log := range txReceipt.Logs {
		switch log.Topics[0] {
		case paymentSig:
			paymentCount++
			t.Logf("  Payment event: commitment=%s", log.Topics[2].Big())
		case nullifierSig:
			nullifierCount++
			t.Logf("  Nullifier event: nullifier=%s", log.Topics[3].Big())
		}
	}
	if paymentCount != 1 {
		t.Errorf("expected 1 Payment event, got %d", paymentCount)
	}
	if nullifierCount != 1 {
		t.Errorf("expected 1 Nullifier event, got %d", nullifierCount)
	}

	// ── Step 9: Bob scans directly (baseline) ────────────────────────────────
	t.Log("Step 9 — Bob scans his note directly with his own view key (baseline)")

	onChainEvents := []dvpcore.OnChainErc20Event{{
		Commitment: bobCommitment,
		CipherText: paymentResult.CipherText,
		EncTxData:  paymentResult.EncTxData,
	}}

	bobDirect, err := dvpcore.ScanForErc20Notes(bobView.DecapsKey, bobSpend.PublicKey, onChainEvents)
	if err != nil {
		t.Fatalf("ScanForErc20Notes (Bob direct): %v", err)
	}
	if len(bobDirect) != 1 || bobDirect[0].Amount.Cmp(paymentAmt) != 0 {
		t.Fatalf("Bob direct scan: expected 1 note of %s, got %d notes", paymentAmt, len(bobDirect))
	}
	t.Logf("  Bob direct scan ✓ — amount=%s tokenId=%s", bobDirect[0].Amount, bobDirect[0].TokenId)

	// ── Step 10: Auditor reads audit ciphertexts from chain ───────────────────
	t.Log("Step 10 — auditor reads audit ciphertexts for all registered users")

	aliceAddr := common.HexToAddress(hardhatAliceAddr)
	bobAddr   := common.HexToAddress(hardhatBobAddr)

	gotAliceMlKemCt, gotAliceAesCt, err := rpcore.LookupAuditCiphertexts(client, registryAddr, aliceAddr)
	if err != nil {
		t.Fatalf("LookupAuditCiphertexts (Alice): %v", err)
	}
	gotBobMlKemCt, gotBobAesCt, err := rpcore.LookupAuditCiphertexts(client, registryAddr, bobAddr)
	if err != nil {
		t.Fatalf("LookupAuditCiphertexts (Bob): %v", err)
	}
	t.Logf("  Alice: mlKemCt=%d bytes  aesCt=%d bytes", len(gotAliceMlKemCt), len(gotAliceAesCt))
	t.Logf("  Bob:   mlKemCt=%d bytes  aesCt=%d bytes", len(gotBobMlKemCt), len(gotBobAesCt))

	// Also read each user's pk_spend from the registry so the auditor can
	// verify commitments without knowing the users' private spend keys.
	aliceOnChainKeys, err := rpcore.LookupKeys(client, registryAddr, aliceAddr)
	if err != nil {
		t.Fatalf("LookupKeys (Alice): %v", err)
	}
	bobOnChainKeys, err := rpcore.LookupKeys(client, registryAddr, bobAddr)
	if err != nil {
		t.Fatalf("LookupKeys (Bob): %v", err)
	}

	// ── Step 11: Auditor recovers view keys ───────────────────────────────────
	t.Log("Step 11 — auditor recovers view secret keys via AuditorDecryptViewKey")

	recoveredAliceDK, err := rpcore.AuditorDecryptViewKey(auditorPair.DecapsKey, gotAliceMlKemCt, gotAliceAesCt)
	if err != nil {
		t.Fatalf("AuditorDecryptViewKey (Alice): %v", err)
	}
	recoveredBobDK, err := rpcore.AuditorDecryptViewKey(auditorPair.DecapsKey, gotBobMlKemCt, gotBobAesCt)
	if err != nil {
		t.Fatalf("AuditorDecryptViewKey (Bob): %v", err)
	}

	// Verify the recovered keys match the originals (encapsulation key round-trip).
	if !bytes.Equal(recoveredAliceDK.EncapsulationKey().Bytes(), aliceView.EncapsKey) {
		t.Error("Alice encapsulation key mismatch after auditor recovery")
	}
	if !bytes.Equal(recoveredBobDK.EncapsulationKey().Bytes(), bobView.EncapsKey) {
		t.Error("Bob encapsulation key mismatch after auditor recovery")
	}
	t.Logf("  Alice view key recovered ✓")
	t.Logf("  Bob   view key recovered ✓")

	// ── Step 12: Auditor performs O(N) scan ───────────────────────────────────
	// The auditor tries every registered user's recovered view key against
	// the on-chain payment event. Only the note owner's key will decrypt.
	t.Log("Step 12 — auditor scans payment event with each user's recovered view key")

	type userEntry struct {
		name   string
		dk     interface{ Decapsulate([]byte) ([]byte, error) }
		pkSpend *big.Int
	}

	// Try Alice's recovered key — should find nothing (change has no on-chain ciphertext).
	aliceAuditNotes, err := dvpcore.ScanForErc20Notes(recoveredAliceDK, aliceOnChainKeys.SpendKey, onChainEvents)
	if err != nil {
		t.Fatalf("auditor ScanForErc20Notes (Alice key): %v", err)
	}
	if len(aliceAuditNotes) != 0 {
		t.Errorf("auditor: expected 0 notes for Alice's key, got %d", len(aliceAuditNotes))
	}
	t.Logf("  Alice's key → %d notes (expected 0 — change has no on-chain ciphertext) ✓",
		len(aliceAuditNotes))

	// Try Bob's recovered key — should find the 30-token note.
	bobAuditNotes, err := dvpcore.ScanForErc20Notes(recoveredBobDK, bobOnChainKeys.SpendKey, onChainEvents)
	if err != nil {
		t.Fatalf("auditor ScanForErc20Notes (Bob key): %v", err)
	}
	if len(bobAuditNotes) != 1 {
		t.Fatalf("auditor: expected 1 note for Bob's key, got %d", len(bobAuditNotes))
	}
	t.Logf("  Bob's key   → %d note(s) ✓", len(bobAuditNotes))

	// ── Step 13: Verify auditor sees same note as Bob ─────────────────────────
	t.Log("Step 13 — verifying auditor's note matches Bob's direct scan")

	auditorNote := bobAuditNotes[0]
	directNote  := bobDirect[0]

	if auditorNote.Amount.Cmp(directNote.Amount) != 0 {
		t.Errorf("amount mismatch: auditor=%s direct=%s", auditorNote.Amount, directNote.Amount)
	}
	if auditorNote.TokenId.Cmp(directNote.TokenId) != 0 {
		t.Errorf("tokenId mismatch: auditor=%s direct=%s", auditorNote.TokenId, directNote.TokenId)
	}
	if auditorNote.Commitment.Cmp(directNote.Commitment) != 0 {
		t.Errorf("commitment mismatch: auditor=%s direct=%s", auditorNote.Commitment, directNote.Commitment)
	}
	if auditorNote.Amount.Cmp(paymentAmt) != 0 {
		t.Errorf("expected amount %s, got %s", paymentAmt, auditorNote.Amount)
	}

	t.Logf("  auditor note: amount=%s tokenId=%s commitment=%s",
		auditorNote.Amount, auditorNote.TokenId, auditorNote.Commitment)
	t.Logf("  matches Bob's direct scan ✓")

	t.Log("=== AUDITED PAYMENT VIA RELAYER COMPLETE ===")
	t.Logf("    Alice paid %s tokens to Bob via relayer (Alice's address never exposed)",
		paymentAmt)
	t.Logf("    Auditor recovered Bob's view key from on-chain ciphertexts")
	t.Logf("    Auditor scanned payment event and confirmed: amount=%s tokenId=%s",
		auditorNote.Amount, auditorNote.TokenId)
}
