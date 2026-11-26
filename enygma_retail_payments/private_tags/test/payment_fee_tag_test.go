package tags_test

// TestPaymentFeeWithTagNotification — end-to-end test combining the fee payment circuit
// with the private tag notification layer.
//
// Same flow as TestPaymentWithTagNotification but uses channelPaymentWithFee()
// on-chain:
//   - PaymentFee circuit: valueIn == Σ(valuesOut) + StFee
//   - the statement carries a public StFee
//   - On-chain check: StFee == PROTOCOL_FEE
//   - Bob's note is a channel payment as before: tagged for the next block,
//     sealed under its key, and bound by the proof's StCiphertextHash; Bob
//     finds it by its tag among the ChannelPayment events
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
//   Terminal 3: cd gnark_circuits && go run ./cmd/server   (gnark server on :8082)
//
// Run:
//   cd private_tags/test && CC=/usr/bin/clang go test -run TestPaymentFeeWithTagNotification -v -timeout 300s

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"enygma_retail_payments/layout"
	dvpcore "github.com/raylsnetwork/enygma_dvp/src/core"
	tags "github.com/raylsnetwork/enygma_retail_payments/private_tags/src"
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
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
		t.Skip("gnark server not running on localhost:8082 — run: cd gnark_circuits && go run ./cmd/server")
	}

	ctx := context.Background()

	client := mustDial(t)
	defer client.Close()

	aliceAuth := hardhatAuthFromKey(t, alicePrivKeyHex)
	bobAuth := hardhatAuthFromKey(t, bobPrivKeyHex)

	// ── Step 1: Deploy a fresh TagChannelRegistry ────────────────────────────
	t.Log("Step 1 — deploying a fresh TagChannelRegistry")

	chRegAddr, err := tags.DeployTagChannelRegistry(client, aliceAuth, "../contracts/TagChannelRegistry.json")
	if err != nil {
		t.Fatalf("DeployTagChannelRegistry: %v", err)
	}
	t.Logf("  TagChannelRegistry: %s", chRegAddr.Hex())

	// ── Step 2: Load payment contract addresses ───────────────────────────────
	t.Log("Step 2 — loading payment contract addresses")

	receipts := loadPaymentReceipts(t)
	vaultAddr := common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress)
	erc20Addr := common.HexToAddress(receipts["ERC20"].ContractAddress)
	dvpAddr := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)
	registryAddr := common.HexToAddress(receipts["UserRegistry"].ContractAddress)

	vaultABI := loadPaymentABI(t, "Erc20CoinVault")
	erc20ABI := loadPaymentABI(t, "RaylsERC20")
	dvpABI := loadPaymentABI(t, "EnygmaDvp")

	vault := bindPayContract(t, client, vaultAddr, vaultABI)
	erc20 := bindPayContract(t, client, erc20Addr, erc20ABI)
	dvp := bindPayContract(t, client, dvpAddr, dvpABI)

	merkleDepth := 8
	tokenId := big.NewInt(0)
	depositAmt := big.NewInt(int64(feeTagDepositAmt))
	paymentAmt := big.NewInt(int64(feeTagPaymentAmt))
	changeAmt := big.NewInt(int64(feeTagChangeAmt))
	fee := big.NewInt(int64(feeTagProtocolFee))

	// ── Step 3: Generate ZK key pairs ────────────────────────────────────────
	t.Log("Step 3 — generating ZK key pairs (spend + view) for Alice and Bob")

	aliceSpend, err := rpcore.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("Alice NewSpendKeyPair: %v", err)
	}
	aliceView, err := rpcore.NewViewKeyPair()
	if err != nil {
		t.Fatalf("Alice NewViewKeyPair: %v", err)
	}
	bobSpend, err := rpcore.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("Bob NewSpendKeyPair: %v", err)
	}
	bobView, err := rpcore.NewViewKeyPair()
	if err != nil {
		t.Fatalf("Bob NewViewKeyPair: %v", err)
	}

	t.Logf("  Alice pk_spend: %s", aliceSpend.PublicKey)
	t.Logf("  Bob   pk_spend: %s", bobSpend.PublicKey)

	// ── Step 4: Register Alice and Bob in UserRegistry ───────────────────────
	t.Log("Step 4 — registering Alice and Bob in UserRegistry")

	auditorPair, err := rpcore.NewAuditorKeyPair()
	if err != nil {
		t.Fatalf("NewAuditorKeyPair: %v", err)
	}
	aliceMlKemCt, aliceAesCt, err := rpcore.EncryptViewKeyForAuditor(auditorPair.EncapsKey, aliceView.DecapsKey)
	if err != nil {
		t.Fatalf("EncryptViewKeyForAuditor (Alice): %v", err)
	}
	bobMlKemCt, bobAesCt, err := rpcore.EncryptViewKeyForAuditor(auditorPair.EncapsKey, bobView.DecapsKey)
	if err != nil {
		t.Fatalf("EncryptViewKeyForAuditor (Bob): %v", err)
	}

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
		t.Fatalf("Encapsulate (deposit): %v", err)
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

	aliceCommitment, err := rpcore.Erc20CommitmentV2(aliceSpend.PublicKey, aliceSaltBField, depositAmt, tokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 (deposit): %v", err)
	}

	depositCtxt, err := rpcore.EncryptPayload(aliceEncKey, tokenId, depositAmt)
	if err != nil {
		t.Fatalf("EncryptPayload (deposit): %v", err)
	}

	depositTx, err := vault.Transact(aliceAuth, "depositV2",
		[]*big.Int{depositAmt, aliceSpend.PublicKey, aliceSaltBField, tokenId}, capsule, depositCtxt)
	if err != nil {
		t.Fatalf("vault.depositV2: %v", err)
	}
	depositReceipt, err := bind.WaitMined(ctx, client, depositTx)
	if err != nil {
		t.Fatalf("wait depositV2: %v", err)
	}
	t.Logf("  deposit mined (block %d, commitment %s)", depositReceipt.BlockNumber, aliceCommitment)

	// ── Step 8: Build Merkle proof for Alice's input note ────────────────────
	t.Log("Step 8 — building Merkle proof for Alice's input note")

	mt := loadPaymentMerkleTree(t, client, vaultAddr, merkleDepth)
	aliceProof, err := mt.GenerateProof(aliceCommitment)
	if err != nil {
		t.Fatalf("GenerateProof (Alice): %v", err)
	}
	t.Logf("  Merkle root: %s", aliceProof.Root)

	// ── Step 9: Tag the payment and prove it (PaymentFee) bound to the tag ──
	t.Logf("Step 9 — tagging the payment for the next block and proving it with PaymentFee (fee=%d, pay=%d → Bob, change=%d → Alice)",
		feeTagProtocolFee, feeTagPaymentAmt, feeTagChangeAmt)

	b := nextBlock(t, client)
	ch, err := tags.PrepareChannelPayment(channelSS, b, bobSpend.PublicKey, paymentAmt, tokenId)
	if err != nil {
		t.Fatalf("PrepareChannelPayment: %v", err)
	}
	nullifier, err := dvpcore.GetNullifierWithTree(aliceSpend.PrivateKey, big.NewInt(int64(aliceProof.TreeNumber)), aliceProof.Indices, merkleDepth)
	if err != nil {
		t.Fatalf("GetNullifierWithTree: %v", err)
	}
	// The circuit requires the change salt Poseidon(sk, nullifier).
	changeSalt, err := rpcore.ChangeSalt(aliceSpend.PrivateKey, nullifier)
	if err != nil {
		t.Fatalf("ChangeSalt: %v", err)
	}
	bobCmt, err := rpcore.Erc20CommitmentV2(bobSpend.PublicKey, ch.Salt, paymentAmt, tokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 (Bob): %v", err)
	}
	changeCmt, err := rpcore.Erc20CommitmentV2(aliceSpend.PublicKey, changeSalt, changeAmt, tokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 (change): %v", err)
	}
	var pathElements [8]string
	for j, el := range aliceProof.Elements {
		if j < len(pathElements) {
			pathElements[j] = el.String()
		}
	}
	vaultAddrBig := new(big.Int).SetBytes(vaultAddr.Bytes())
	proofResp, err := postGnarkProof(gnarkURLRelayerFee+"/proof/paymentFee", map[string]interface{}{
		"stMessage":            "0",
		"stTreeNumbers":        [1]string{big.NewInt(int64(aliceProof.TreeNumber)).String()},
		"stMerkleRoots":        [1]string{aliceProof.Root.String()},
		"stNullifiers":         [1]string{nullifier.String()},
		"stCommitmentsOut":     [2]string{bobCmt.String(), changeCmt.String()},
		"stContractAddress":    vaultAddrBig.String(),
		"stCiphertextHash":     rpcore.ChannelPaymentCiphertextHash(b, ch.Tag, ch.Ctxt).String(),
		"stFee":                fee.String(),
		"wtPrivateKeysIn":      [1]string{aliceSpend.PrivateKey.String()},
		"wtValuesIn":           [1]string{depositAmt.String()},
		"wtSaltsIn":            [1]string{aliceSaltBField.String()},
		"wtPathElements":       [1][8]string{pathElements},
		"wtPathIndices":        [1]string{aliceProof.Indices.String()},
		"wtTokenId":            tokenId.String(),
		"wtSpendPublicKeysOut": [2]string{bobSpend.PublicKey.String(), aliceSpend.PublicKey.String()},
		"wtValuesOut":          [2]string{paymentAmt.String(), changeAmt.String()},
		"wtSaltsOut":           [2]string{ch.Salt.String(), changeSalt.String()},
	})
	if err != nil {
		t.Fatalf("prove paymentFee: %v", err)
	}
	t.Logf("  proof generated, bound to block %d's tag %x... ✓", b, ch.Tag[:8])

	stmt := proofResp.PublicSignal
	if len(stmt) != layout.PaymentFeeLen {
		t.Fatalf("expected %d statement elements, got %d", layout.PaymentFeeLen, len(stmt))
	}
	if stmt[layout.PaymentFeeFeeOffset].Cmp(fee) != 0 {
		t.Errorf("statement fee = %s, want %s", stmt[layout.PaymentFeeFeeOffset], fee)
	} else {
		t.Logf("  statement fee = %d == PROTOCOL_FEE ✓", feeTagProtocolFee)
	}

	// ── Step 10: Settle channelPaymentWithFee on-chain ───────────────────────
	t.Log("Step 10 — settling the payment on-chain (EnygmaDvp.channelPaymentWithFee)")

	proofStrs := make([]string, len(proofResp.Proof))
	for i, v := range proofResp.Proof {
		proofStrs[i] = v.String()
	}
	receipt := buildPayProofReceipt(&rpcore.PaymentResult{Proof: proofStrs, Statement: stmt, NumberOfInputs: 1, NumberOfOutputs: 2})
	payTx, err := dvp.Transact(aliceAuth, "channelPaymentWithFee",
		receipt, big.NewInt(0), fee, new(big.Int).SetUint64(b), ch.Tag, ch.Ctxt)
	if err != nil {
		t.Fatalf("EnygmaDvp.channelPaymentWithFee: %v", err)
	}
	payReceipt, err := bind.WaitMined(ctx, client, payTx)
	if err != nil || payReceipt.Status != 1 {
		t.Fatalf("channelPaymentWithFee did not settle (err=%v)", err)
	}
	t.Logf("  channelPaymentWithFee mined (block %d, gas %d)", payReceipt.BlockNumber, payReceipt.GasUsed)
	expectOneChannelPayment(t, payReceipt, b, ch.Tag, 0)

	// ── Step 11: Bob finds the payment by its tag and opens his note ─────────
	t.Log("Step 11 — Bob scans the ChannelPayment events for his tag and opens his note")

	bobOpensChannelPayment(t, client, dvpAddr, b, bobChannelSS, bobSpend.PublicKey, paymentAmt, tokenId,
		stmt[layout.PaymentFeeCommitmentsOutOffset])

	// ── Step 12: Alice verifies her change note ───────────────────────────────
	t.Log("Step 12 — Alice verifies her change note")

	aliceChangeCmt, err := rpcore.Erc20CommitmentV2(
		aliceSpend.PublicKey, changeSalt, changeAmt, tokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 (Alice change): %v", err)
	}

	if aliceChangeCmt.Cmp(stmt[layout.PaymentFeeCommitmentsOutOffset+1]) != 0 {
		t.Errorf("Alice's change commitment mismatch:\n  got:  %s\n  want: %s",
			aliceChangeCmt, stmt[layout.PaymentFeeCommitmentsOutOffset+1])
	}
	t.Logf("  Alice change commitment: %s ✓", aliceChangeCmt)

	t.Log("")
	t.Log("=== PAYMENT FEE WITH TAG NOTIFICATION COMPLETE ===")
	t.Logf("    Alice paid %s tokens to Bob (kept %s as change, protocol fee %s)",
		paymentAmt, changeAmt, fee)
	t.Logf("    channelPaymentWithFee verified on-chain with fee check ✓")
	t.Logf("    Bob found his note by its tag among the ChannelPayment events — no capsule ✓")
	t.Logf("    Bob's commitment verified: %s", bobCmt)
}
