package tags_test

// TestPaymentFeeWithTagNotification — end-to-end test combining the fee payment circuit
// with the private tag notification layer.
//
// Same flow as TestPaymentWithTagNotification but uses paymentWithFee() on-chain:
//   - PaymentFee circuit: valueIn == Σ(valuesOut) + StFee
//   - 8-element statement includes StFee at index 7
//   - On-chain check: statement[7] == PROTOCOL_FEE
//   - Tag notification unchanged — Bob still receives via tag scan
//
// Constants:
//   PROTOCOL_FEE = 5
//   depositAmt   = 45   (payment=30, change=10, fee=5)
//   paymentAmt   = 30
//   changeAmt    = 10
//
// Prerequisites:
//   Terminal 1: cd enygma_dvp && npx hardhat node
//   Terminal 2: bash setup.sh                         (deploy + init with PaymentFee VK at slot 2)
//   Terminal 3: cd gnark_circuits && go run main.go   (gnark server on :8082)
//
// Run:
//   cd private_tags/test && CC=/usr/bin/clang go test -run TestPaymentFeeWithTagNotification -v -timeout 300s

import (
	"context"
	"math/big"
	"strings"
	"testing"

	tags   "github.com/raylsnetwork/enygma_retail_payments/private_tags/src"
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	feeTagProtocolFee = 5
	feeTagDepositAmt  = 45 // payment(30) + change(10) + fee(5)
	feeTagPaymentAmt  = 30
	feeTagChangeAmt   = 10
)

func TestPaymentFeeWithTagNotification(t *testing.T) {
	if !chainAvailable() {
		t.Skip("Hardhat node not running on localhost:8545 — skipping")
	}
	if !gnarkAvailable() {
		t.Skip("gnark server not running on localhost:8082 — run: cd gnark_circuits && go run main.go")
	}

	ctx := context.Background()

	client := mustDial(t)
	defer client.Close()

	aliceAuth := hardhatAuthFromKey(t, alicePrivKeyHex)
	bobAuth   := hardhatAuthFromKey(t, bobPrivKeyHex)

	// ── Step 1: Deploy fresh TagRegistry and TagChannelRegistry ──────────────
	t.Log("Step 1 — deploying fresh TagRegistry and TagChannelRegistry")

	tagRegAddr, err := tags.DeployTagRegistry(client, aliceAuth, "../contracts/TagRegistry.json")
	if err != nil {
		t.Fatalf("DeployTagRegistry: %v", err)
	}
	chRegAddr, err := tags.DeployTagChannelRegistry(client, aliceAuth, "../contracts/TagChannelRegistry.json")
	if err != nil {
		t.Fatalf("DeployTagChannelRegistry: %v", err)
	}
	t.Logf("  TagRegistry:        %s", tagRegAddr.Hex())
	t.Logf("  TagChannelRegistry: %s", chRegAddr.Hex())

	// ── Step 2: Load payment contract addresses ───────────────────────────────
	t.Log("Step 2 — loading payment contract addresses")

	receipts     := loadPaymentReceipts(t)
	vaultAddr    := common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress)
	erc20Addr    := common.HexToAddress(receipts["ERC20"].ContractAddress)
	dvpAddr      := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)
	registryAddr := common.HexToAddress(receipts["UserRegistry"].ContractAddress)

	vaultABI := loadPaymentABI(t, "Erc20CoinVault")
	erc20ABI := loadPaymentABI(t, "RaylsERC20")
	dvpABI   := loadPaymentABI(t, "EnygmaDvp")

	vault := bindPayContract(t, client, vaultAddr, vaultABI)
	erc20 := bindPayContract(t, client, erc20Addr, erc20ABI)
	dvp   := bindPayContract(t, client, dvpAddr, dvpABI)

	gnarkClient := rpcore.NewPaymentClient("")
	merkleDepth := 8
	tokenId     := big.NewInt(0)
	depositAmt  := big.NewInt(int64(feeTagDepositAmt))
	paymentAmt  := big.NewInt(int64(feeTagPaymentAmt))
	changeAmt   := big.NewInt(int64(feeTagChangeAmt))
	fee         := big.NewInt(int64(feeTagProtocolFee))

	// ── Step 3: Generate ZK key pairs ────────────────────────────────────────
	t.Log("Step 3 — generating ZK key pairs (spend + view) for Alice and Bob")

	aliceSpend, err := rpcore.NewSpendKeyPair()
	if err != nil { t.Fatalf("Alice NewSpendKeyPair: %v", err) }
	aliceView, err := rpcore.NewViewKeyPair()
	if err != nil { t.Fatalf("Alice NewViewKeyPair: %v", err) }
	bobSpend, err := rpcore.NewSpendKeyPair()
	if err != nil { t.Fatalf("Bob NewSpendKeyPair: %v", err) }
	bobView, err := rpcore.NewViewKeyPair()
	if err != nil { t.Fatalf("Bob NewViewKeyPair: %v", err) }

	t.Logf("  Alice pk_spend: %s", aliceSpend.PublicKey)
	t.Logf("  Bob   pk_spend: %s", bobSpend.PublicKey)

	// ── Step 4: Register Alice and Bob in UserRegistry ───────────────────────
	t.Log("Step 4 — registering Alice and Bob in UserRegistry")

	auditorPair, err := rpcore.NewAuditorKeyPair()
	if err != nil { t.Fatalf("NewAuditorKeyPair: %v", err) }
	aliceMlKemCt, aliceAesCt, err := rpcore.EncryptViewKeyForAuditor(auditorPair.EncapsKey, aliceView.DecapsKey)
	if err != nil { t.Fatalf("EncryptViewKeyForAuditor (Alice): %v", err) }
	bobMlKemCt, bobAesCt, err := rpcore.EncryptViewKeyForAuditor(auditorPair.EncapsKey, bobView.DecapsKey)
	if err != nil { t.Fatalf("EncryptViewKeyForAuditor (Bob): %v", err) }

	if err := rpcore.Register(client, aliceAuth, registryAddr, aliceSpend.PublicKey, aliceView.EncapsKey, aliceMlKemCt, aliceAesCt); err != nil {
		if !strings.Contains(err.Error(), "AlreadyRegistered") && !strings.Contains(err.Error(), "45ed80e9") {
			t.Fatalf("Alice Register: %v", err)
		}
		t.Log("  Alice already registered")
	} else {
		t.Logf("  Alice registered (%s)", aliceAuth.From.Hex())
	}

	if err := rpcore.Register(client, bobAuth, registryAddr, bobSpend.PublicKey, bobView.EncapsKey, bobMlKemCt, bobAesCt); err != nil {
		if !strings.Contains(err.Error(), "AlreadyRegistered") && !strings.Contains(err.Error(), "45ed80e9") {
			t.Fatalf("Bob Register: %v", err)
		}
		t.Log("  Bob already registered")
	} else {
		t.Logf("  Bob registered (%s)", bobAuth.From.Hex())
	}

	// ── Step 5: Alice sets up a channel with Bob ──────────────────────────────
	t.Log("Step 5 — Alice sets up a channel with Bob (ML-KEM)")

	initialMsg := []byte("FeePayment channel ready")
	aliceSenderId := []byte(aliceAddr)
	channelSS, c1, c2, bitmap, err := tags.PrepareChannelSetup(
		bobView.EncapsKey,
		initialMsg,
		aliceSenderId,
		tags.PrivacyFull,
		2,
		1,
		nil,
	)
	if err != nil {
		t.Fatalf("PrepareChannelSetup: %v", err)
	}
	openChannelDirect(t, client, aliceAuth, chRegAddr, c1, c2, bitmap)
	t.Logf("  channel published — ss_channel: %x...", channelSS[:8])

	// ── Step 6: Bob scans channels and recovers shared secret ────────────────
	t.Log("Step 6 — Bob scans TagChannelRegistry for his channel")

	bobChannelFound, err := tags.ScanChannels(client, chRegAddr, bobView.DecapsKey, 0, 100)
	if err != nil {
		t.Fatalf("ScanChannels (Bob): %v", err)
	}
	if len(bobChannelFound) != 1 {
		t.Fatalf("Bob expected 1 channel, found %d", len(bobChannelFound))
	}
	bobChannelSS := bobChannelFound[0].SharedSecret
	t.Logf("  Bob found channel — recovered ss: %x...", bobChannelSS[:8])

	if new(big.Int).SetBytes(channelSS).Cmp(new(big.Int).SetBytes(bobChannelSS)) != 0 {
		t.Fatal("shared secret mismatch between Alice and Bob")
	}
	t.Log("  shared secrets match ✓")

	// ── Step 7: Alice deposits 45 tokens (payment + change + fee) ────────────
	t.Logf("Step 7 — Alice mints and deposits %d ERC-20 tokens (pay=%d + change=%d + fee=%d)",
		feeTagDepositAmt, feeTagPaymentAmt, feeTagChangeAmt, feeTagProtocolFee)

	mintTx, err := erc20.Transact(aliceAuth, "mint", aliceAuth.From,
		new(big.Int).Mul(depositAmt, big.NewInt(10)))
	if err != nil { t.Fatalf("ERC20.mint: %v", err) }
	if _, err := bind.WaitMined(ctx, client, mintTx); err != nil { t.Fatalf("wait mint: %v", err) }

	approveTx, err := erc20.Transact(aliceAuth, "approve", vaultAddr, depositAmt)
	if err != nil { t.Fatalf("ERC20.approve: %v", err) }
	if _, err := bind.WaitMined(ctx, client, approveTx); err != nil { t.Fatalf("wait approve: %v", err) }

	ss, capsule, err := rpcore.Encapsulate(aliceView.EncapsKey)
	if err != nil { t.Fatalf("Encapsulate (deposit): %v", err) }
	aliceSaltB, err := rpcore.DerivePaymentSalt(ss)
	if err != nil { t.Fatalf("DerivePaymentSalt: %v", err) }
	aliceEncKey, err := rpcore.DerivePaymentKey(ss)
	if err != nil { t.Fatalf("DerivePaymentKey: %v", err) }
	aliceSaltBField := rpcore.SaltBToField(aliceSaltB)

	aliceCommitment, err := rpcore.Erc20CommitmentV2(aliceSpend.PublicKey, aliceSaltBField, depositAmt, tokenId)
	if err != nil { t.Fatalf("Erc20CommitmentV2 (deposit): %v", err) }

	depositCtxt, err := rpcore.EncryptPayload(aliceEncKey, tokenId, depositAmt)
	if err != nil { t.Fatalf("EncryptPayload (deposit): %v", err) }

	depositTx, err := vault.Transact(aliceAuth, "depositV2",
		[]*big.Int{depositAmt, aliceSpend.PublicKey, aliceSaltBField, tokenId}, capsule, depositCtxt)
	if err != nil { t.Fatalf("vault.depositV2: %v", err) }
	depositReceipt, err := bind.WaitMined(ctx, client, depositTx)
	if err != nil { t.Fatalf("wait depositV2: %v", err) }
	t.Logf("  deposit mined (block %d, commitment %s)", depositReceipt.BlockNumber, aliceCommitment)

	// ── Step 8: Build Merkle proof for Alice's input note ────────────────────
	t.Log("Step 8 — building Merkle proof for Alice's input note")

	mt := loadPaymentMerkleTree(t, client, vaultAddr, merkleDepth)
	aliceProof, err := mt.GenerateProof(aliceCommitment)
	if err != nil { t.Fatalf("GenerateProof (Alice): %v", err) }
	t.Logf("  Merkle root: %s", aliceProof.Root)

	// ── Step 9: Generate ZK PaymentFee proof ─────────────────────────────────
	t.Logf("Step 9 — generating PaymentFee proof (fee=%d, pay=%d → Bob, change=%d → Alice)",
		feeTagProtocolFee, feeTagPaymentAmt, feeTagChangeAmt)

	vaultAddrBig := new(big.Int).SetBytes(vaultAddr.Bytes())
	paymentResult, err := gnarkClient.BoundPaymentFeeProof(
		vaultAddrBig,
		fee,
		big.NewInt(0), // stMessage = 0
		[]*big.Int{depositAmt},
		[]rpcore.KeyPair{{PrivateKey: aliceSpend.PrivateKey, PublicKey: aliceSpend.PublicKey}},
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
		t.Fatalf("BoundPaymentFeeProof: %v", err)
	}
	t.Log("  proof generated ✓")

	// Verify fee signal.
	stmt := paymentResult.ContractStatement()
	if len(stmt) != 8 {
		t.Fatalf("expected 8 statement elements, got %d", len(stmt))
	}
	if stmt[7].Cmp(fee) != 0 {
		t.Errorf("statement[7] fee = %s, want %s", stmt[7], fee)
	} else {
		t.Logf("  statement[7] = %d == PROTOCOL_FEE ✓", feeTagProtocolFee)
	}

	// ── Step 10: Submit paymentWithFee on-chain ───────────────────────────────
	t.Log("Step 10 — submitting paymentWithFee on-chain (EnygmaDvp.paymentWithFee)")

	receipt := buildPayProofReceipt(paymentResult)
	payTx, err := dvp.Transact(aliceAuth, "paymentWithFee",
		receipt, big.NewInt(0), fee, paymentResult.CipherText, paymentResult.EncTxData)
	if err != nil {
		t.Fatalf("EnygmaDvp.paymentWithFee: %v", err)
	}
	payReceipt, err := bind.WaitMined(ctx, client, payTx)
	if err != nil {
		t.Fatalf("wait paymentWithFee: %v", err)
	}
	t.Logf("  paymentWithFee mined (block %d, gas %d)", payReceipt.BlockNumber, payReceipt.GasUsed)

	// Verify on-chain events.
	paymentSig   := crypto.Keccak256Hash([]byte("Payment(uint256,uint256,bytes,bytes)"))
	nullifierSig := crypto.Keccak256Hash([]byte("Nullifier(uint256,uint256,uint256)"))
	var payEvents, nfEvents int
	for _, log := range payReceipt.Logs {
		switch log.Topics[0] {
		case paymentSig:
			payEvents++
			t.Logf("  Payment event: commitment=%s", log.Topics[2].Big())
		case nullifierSig:
			nfEvents++
			t.Logf("  Nullifier event: nullifier=%s", log.Topics[3].Big())
		}
	}
	if payEvents != 1 { t.Errorf("expected 1 Payment event, got %d", payEvents) }
	if nfEvents != 1  { t.Errorf("expected 1 Nullifier event, got %d", nfEvents) }

	// ── Step 11: Alice prepares tag notification for Bob ─────────────────────
	t.Log("Step 11 — Alice prepares payment tag notification")

	startBlock, windowTags, noteCtxt, err := tags.PreparePaymentTag(
		client, 3,
		bobSpend.PublicKey,
		channelSS,
		paymentAmt, tokenId, paymentResult.SaltB,
	)
	if err != nil { t.Fatalf("PreparePaymentTag: %v", err) }
	t.Logf("  tag window: blocks [%d, %d)  ctxt=%d bytes", startBlock, startBlock+3, len(noteCtxt))

	tagBlock := publishTagDirect(t, client, aliceAuth, tagRegAddr, windowTags[0], noteCtxt)

	if tagBlock != startBlock {
		offset := int(tagBlock) - int(startBlock)
		if offset < 0 || offset >= len(windowTags) {
			t.Fatalf("tag drifted outside window: block %d not in [%d, %d)",
				tagBlock, startBlock, startBlock+uint64(len(windowTags)))
		}
		t.Logf("  block drift %d→%d — republishing with window tag[%d]", startBlock, tagBlock, offset)
		tagBlock = publishTagDirect(t, client, aliceAuth, tagRegAddr, windowTags[offset], noteCtxt)
	}
	t.Logf("  tag published at block %d ✓", tagBlock)

	// ── Step 12: Bob scans for his tag ───────────────────────────────────────
	t.Log("Step 12 — Bob scans TagRegistry for his payment tag")

	bobChannels := []tags.Channel{{SharedSecret: bobChannelSS, PkSpend: bobSpend.PublicKey}}
	cursor := tags.NewScanCursor()
	matches, _, err := tags.ScanBlocksFromCursor(client, tagRegAddr, bobChannels, cursor, tagBlock)
	if err != nil { t.Fatalf("ScanBlocksFromCursor (Bob): %v", err) }
	if len(matches) != 1 {
		t.Fatalf("Bob expected 1 matching tag, got %d", len(matches))
	}
	t.Logf("  Bob found tag at block %d ✓", matches[0].BlockNumber)

	// ── Step 13: Bob decrypts the tag and verifies the payment note ───────────
	t.Log("Step 13 — Bob decrypts tag and verifies his payment note")

	note, err := tags.DecryptPaymentNote(bobChannelSS, matches[0].Entry.Ctxt)
	if err != nil { t.Fatalf("DecryptPaymentNote: %v", err) }

	t.Logf("  note.Amount:  %s  (want %s)", note.Amount, paymentAmt)
	t.Logf("  note.TokenId: %s  (want %s)", note.TokenId, tokenId)
	t.Logf("  note.Salt:    %s", note.Salt)

	if note.Amount.Cmp(paymentAmt) != 0 {
		t.Errorf("amount mismatch: got %s, want %s", note.Amount, paymentAmt)
	}
	if note.TokenId.Cmp(tokenId) != 0 {
		t.Errorf("tokenId mismatch: got %s, want %s", note.TokenId, tokenId)
	}
	if note.Salt.Cmp(paymentResult.SaltB) != 0 {
		t.Errorf("saltB mismatch: got %s, want %s", note.Salt, paymentResult.SaltB)
	}
	t.Log("  amount ✓  tokenId ✓  salt ✓")

	bobRecomputedCmt, err := rpcore.Erc20CommitmentV2(
		bobSpend.PublicKey, note.Salt, note.Amount, note.TokenId)
	if err != nil { t.Fatalf("Erc20CommitmentV2 (Bob verify): %v", err) }

	expectedBobCmt := paymentResult.Statement[4]
	if bobRecomputedCmt.Cmp(expectedBobCmt) != 0 {
		t.Errorf("Bob's recomputed commitment mismatch:\n  got:  %s\n  want: %s",
			bobRecomputedCmt, expectedBobCmt)
	}
	t.Logf("  Bob recomputed commitment: %s ✓", bobRecomputedCmt)

	// ── Step 14: Alice verifies her change note ───────────────────────────────
	t.Log("Step 14 — Alice verifies her change note")

	aliceChangeCmt, err := rpcore.Erc20CommitmentV2(
		aliceSpend.PublicKey, paymentResult.SaltA, changeAmt, tokenId)
	if err != nil { t.Fatalf("Erc20CommitmentV2 (Alice change): %v", err) }

	if aliceChangeCmt.Cmp(paymentResult.Statement[5]) != 0 {
		t.Errorf("Alice's change commitment mismatch:\n  got:  %s\n  want: %s",
			aliceChangeCmt, paymentResult.Statement[5])
	}
	t.Logf("  Alice change commitment: %s ✓", aliceChangeCmt)

	t.Log("")
	t.Log("=== PAYMENT FEE WITH TAG NOTIFICATION COMPLETE ===")
	t.Logf("    Alice paid %s tokens to Bob (kept %s as change, protocol fee %s)",
		paymentAmt, changeAmt, fee)
	t.Logf("    paymentWithFee verified on-chain with fee check ✓")
	t.Logf("    Bob found his note via tag scan — no Payment event decryption needed ✓")
	t.Logf("    Bob's commitment verified: %s", bobRecomputedCmt)
}
