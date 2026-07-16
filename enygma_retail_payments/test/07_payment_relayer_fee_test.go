package tests

// TestRetailErc20_PaymentRelayerFee exercises the 3-output PaymentRelayer circuit.
//
// Fee model:
//
//	Alice pays Bob AND leaves a spendable fee note for the relayer (Carol).
//	  valueIn = paymentAmt + changeAmt + relayerFeeAmt
//	  Σ(valuesOut) = all three outputs — no value burned
//
//	depositAmt    = 45 (payment(30) + change(10) + relayerFee(5))
//	paymentAmt    = 30 (to Bob)
//	changeAmt     = 10 (back to Alice)
//	relayerFeeAmt =  5 (spendable note for relayer / Carol)
//
// Signal layout (1-in / 3-out, 8 elements):
//
//	signal[0]  StMessage             = 0
//	signal[1]  StTreeNumbers[0]      = tree index
//	signal[2]  StMerkleRoots[0]      = Merkle root
//	signal[3]  StNullifiers[0]       = nullifier
//	signal[4]  StCommitmentsOut[0]   = Bob's commitment   (output 0)
//	signal[5]  StCommitmentsOut[1]   = Alice's change     (output 1, constrained to senderPk)
//	signal[6]  StCommitmentsOut[2]   = Relayer fee note   (output 2)
//	signal[7]  StContractAddress     = vault address
//
// Prerequisites:
//
//	cd ../enygma_dvp && npx hardhat node
//	bash setup.sh                                     (from enygma_retail_payments/)
//	cd gnark_circuits && go run generation.go         (regenerate PaymentRelayer keys)
//	cd gnark_circuits && go run main.go               (start gnark server :8082)
//
// Run:
//
//	cd test && go test -run TestRetailErc20_PaymentRelayerFee -v -timeout 300s

import (
	"context"
	"math/big"
	"strings"
	"testing"

	dvpcore "github.com/raylsnetwork/enygma_dvp/src/core"
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	relayerFeeDeposit    = 45 // paymentAmt(30) + changeAmt(10) + relayerFeeAmt(5)
	relayerFeePayAmt     = 30
	relayerFeeChangeAmt  = 10
	relayerFeeRelayerAmt = 5
)

func TestRetailErc20_PaymentRelayerFee(t *testing.T) {
	if !chainAvailable() {
		t.Skip("Hardhat node not running on localhost:8545 — skipping")
	}
	if !serverAvailable("localhost:8082") {
		t.Skip("gnark server not running on localhost:8082 — skipping")
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
	dvpAddr      := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)
	registryAddr := common.HexToAddress(receipts["UserRegistry"].ContractAddress)

	vaultABI := loadOnchainABI(t, "Erc20CoinVault")
	erc20ABI  := loadOnchainABI(t, "RaylsERC20")
	dvpABI    := loadOnchainABI(t, "EnygmaDvp")

	vault := bind.NewBoundContract(vaultAddr, vaultABI, client, client, client)
	erc20 := bind.NewBoundContract(erc20Addr, erc20ABI, client, client, client)
	dvp   := bind.NewBoundContract(dvpAddr, dvpABI, client, client, client)

	aliceAuth := hardhatAuth(t, client)
	bobAuth   := hardhatBobAuth(t, client)

	gnarkClient := rpcore.NewPaymentClient("")
	merkleDepth := 8
	tokenId     := big.NewInt(0)
	depositAmt  := big.NewInt(int64(relayerFeeDeposit))
	paymentAmt  := big.NewInt(int64(relayerFeePayAmt))
	changeAmt   := big.NewInt(int64(relayerFeeChangeAmt))
	relayerAmt  := big.NewInt(int64(relayerFeeRelayerAmt))

	// ── Step 1: Key generation ────────────────────────────────────────────────
	t.Log("Step 1 — generating ZK key pairs for Alice, Bob, and relayer (Carol)")

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
	// Relayer (Carol) only needs a spend key — their note is found via SaltRelayer off-chain.
	relayerSpend, err := rpcore.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("Relayer NewSpendKeyPair: %v", err)
	}

	t.Logf("  Alice   pk_spend: %s", aliceSpend.PublicKey)
	t.Logf("  Bob     pk_spend: %s", bobSpend.PublicKey)
	t.Logf("  Relayer pk_spend: %s", relayerSpend.PublicKey)

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

	// ── Step 2: Registration ─────────────────────────────────────────────────
	t.Log("Step 2 — registering keys on-chain (UserRegistry)")

	if err := rpcore.Register(client, aliceAuth, registryAddr,
		aliceSpend.PublicKey, aliceView.EncapsKey, aliceMlKemCt, aliceAesCt); err != nil {
		if !strings.Contains(err.Error(), "AlreadyRegistered") && !strings.Contains(err.Error(), "0x45ed80e9") {
			t.Fatalf("Alice Register: %v", err)
		}
		t.Log("  Alice already registered")
	} else {
		t.Logf("  Alice registered (%s)", aliceAuth.From.Hex())
	}

	if err := rpcore.Register(client, bobAuth, registryAddr,
		bobSpend.PublicKey, bobView.EncapsKey, bobMlKemCt, bobAesCt); err != nil {
		if !strings.Contains(err.Error(), "AlreadyRegistered") && !strings.Contains(err.Error(), "0x45ed80e9") {
			t.Fatalf("Bob Register: %v", err)
		}
		t.Log("  Bob already registered")
	} else {
		t.Logf("  Bob registered (%s)", bobAuth.From.Hex())
	}

	// Relayer does not need to register — their fee note is found via SaltRelayer.

	// ── Step 3: Deposit — Alice deposits 45 tokens ───────────────────────────
	t.Logf("Step 3 — Alice deposits %d tokens (payment=%d + change=%d + relayerFee=%d)",
		relayerFeeDeposit, relayerFeePayAmt, relayerFeeChangeAmt, relayerFeeRelayerAmt)

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

	aliceCommitment, err := rpcore.Erc20CommitmentV2(
		aliceSpend.PublicKey, aliceSaltBField, depositAmt, tokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 (deposit): %v", err)
	}

	aliceDepositCtxtII, err := rpcore.EncryptPayload(aliceEncKey, tokenId, depositAmt)
	if err != nil {
		t.Fatalf("EncryptPayload: %v", err)
	}

	depositTx, err := vault.Transact(aliceAuth, "depositV2",
		[]*big.Int{depositAmt, aliceSpend.PublicKey, aliceSaltBField, tokenId}, capsule, aliceDepositCtxtII)
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

	// ── Step 4: PaymentRelayer proof ─────────────────────────────────────────
	t.Logf("Step 4 — requesting PaymentRelayer proof (pay=%d to Bob, change=%d to Alice, fee=%d to relayer)",
		relayerFeePayAmt, relayerFeeChangeAmt, relayerFeeRelayerAmt)

	vaultAddrBig := new(big.Int).SetBytes(vaultAddr.Bytes())
	paymentResult, err := gnarkClient.BoundPaymentRelayerProof(
		vaultAddrBig,
		big.NewInt(0), // stMessage = 0
		[]*big.Int{depositAmt},
		[]rpcore.KeyPair{
			{PrivateKey: aliceSpend.PrivateKey, PublicKey: aliceSpend.PublicKey},
		},
		[]*big.Int{aliceSaltBField},
		[]*big.Int{paymentAmt, changeAmt, relayerAmt},
		relayerSpend.PublicKey,
		[]*big.Int{bobSpend.PublicKey, aliceSpend.PublicKey},
		bobView.EncapsKey,
		merkleDepth,
		[]*rpcore.MerkleProof{aliceProof},
		[]*big.Int{big.NewInt(0)},
		tokenId,
	)
	if err != nil {
		t.Fatalf("BoundPaymentRelayerProof: %v", err)
	}
	t.Log("  PaymentRelayer proof generated ✓")
	t.Logf("  SaltRelayer (for relayer to spend output[2]): %s", paymentResult.SaltRelayer)

	// ── Step 5: Verify public signals ────────────────────────────────────────
	t.Log("Step 5 — verifying public signals")

	stmt := paymentResult.ContractStatement()
	// Layout: [msg, treeNum[0], root[0], nf[0], cmt_bob, cmt_change, cmt_relayer, contractAddr]
	if len(stmt) != 8 {
		t.Fatalf("expected 8 statement elements, got %d", len(stmt))
	}
	t.Logf("  signal[0] msg           = %s", stmt[0])
	t.Logf("  signal[1] treeNum       = %s", stmt[1])
	t.Logf("  signal[2] root          = %s", stmt[2])
	t.Logf("  signal[3] nullifier     = %s", stmt[3])
	t.Logf("  signal[4] cmt_bob       = %s", stmt[4])
	t.Logf("  signal[5] cmt_change    = %s", stmt[5])
	t.Logf("  signal[6] cmt_relayer   = %s", stmt[6])
	t.Logf("  signal[7] contractAddr  = %s", stmt[7])

	if stmt[7].Cmp(vaultAddrBig) != 0 {
		t.Errorf("signal[7] contractAddr = %s, want %s", stmt[7], vaultAddrBig)
	} else {
		t.Log("  signal[7] == vault address ✓")
	}

	// ── Step 5b: Submit paymentWithRelayerFee on-chain ────────────────────────
	t.Log("Step 5b — submitting paymentWithRelayerFee on-chain (EnygmaDvp.paymentWithRelayerFee)")

	onchainReceipt := paymentResultToReceipt(t, paymentResult)
	payTx, err := dvp.Transact(aliceAuth, "paymentWithRelayerFee",
		onchainReceipt, big.NewInt(0), paymentResult.CipherText, paymentResult.EncTxData)
	if err != nil {
		t.Fatalf("EnygmaDvp.paymentWithRelayerFee: %v", err)
	}
	payReceipt, err := bind.WaitMined(ctx, client, payTx)
	if err != nil {
		t.Fatalf("wait paymentWithRelayerFee: %v", err)
	}
	t.Logf("  paymentWithRelayerFee mined (block %d, gas %d)",
		payReceipt.BlockNumber, payReceipt.GasUsed)

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
	if payEvents != 1 {
		t.Errorf("expected 1 Payment event (Bob's note), got %d", payEvents)
	}
	if nfEvents != 1 {
		t.Errorf("expected 1 Nullifier event (Alice's input burned), got %d", nfEvents)
	}
	t.Log("  on-chain paymentWithRelayerFee ✓")

	// ── Step 6: Verify Bob's note (ML-KEM scan) ───────────────────────────────
	t.Log("Step 6 — Bob scans his note via ML-KEM")

	bobEvents := []dvpcore.OnChainErc20Event{{
		Commitment: paymentResult.Statement[4],
		CipherText: paymentResult.CipherText,
		EncTxData:  paymentResult.EncTxData,
	}}
	bobNotes, err := dvpcore.ScanForErc20Notes(bobView.DecapsKey, bobSpend.PublicKey, bobEvents)
	if err != nil {
		t.Fatalf("ScanForErc20Notes: %v", err)
	}
	if len(bobNotes) != 1 {
		t.Fatalf("Bob expected 1 note, got %d", len(bobNotes))
	}
	if bobNotes[0].Amount.Cmp(paymentAmt) != 0 {
		t.Errorf("Bob's note amount: got %s, want %s", bobNotes[0].Amount, paymentAmt)
	}
	t.Logf("  Bob's note: amount=%s tokenId=%s ✓", bobNotes[0].Amount, bobNotes[0].TokenId)

	// ── Step 7: Verify Alice's change commitment ──────────────────────────────
	t.Log("Step 7 — Alice verifies her change commitment locally")

	aliceChangeCmt, err := rpcore.Erc20CommitmentV2(
		aliceSpend.PublicKey, paymentResult.SaltA, changeAmt, tokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 (change verify): %v", err)
	}
	if aliceChangeCmt.Cmp(paymentResult.ContractStatement()[5]) != 0 {
		t.Errorf("Alice's change commitment mismatch: got %s, want %s",
			aliceChangeCmt, paymentResult.ContractStatement()[5])
	}
	t.Logf("  Alice's change note: amount=%s saltA=%s ✓", changeAmt, paymentResult.SaltA)

	// ── Step 8: Verify relayer fee commitment ────────────────────────────────
	t.Log("Step 8 — relayer verifies fee commitment locally (using SaltRelayer)")

	relayerFeeCmt, err := rpcore.Erc20CommitmentV2(
		relayerSpend.PublicKey, paymentResult.SaltRelayer, relayerAmt, tokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 (relayer verify): %v", err)
	}
	if relayerFeeCmt.Cmp(paymentResult.ContractStatement()[6]) != 0 {
		t.Errorf("Relayer fee commitment mismatch: got %s, want %s",
			relayerFeeCmt, paymentResult.ContractStatement()[6])
	}
	t.Logf("  Relayer fee note: amount=%s saltRelayer=%s ✓", relayerAmt, paymentResult.SaltRelayer)

	t.Logf("=== PAYMENT RELAYER FEE FLOW COMPLETE ===")
	t.Logf("    Alice paid %s tokens to Bob", paymentAmt)
	t.Logf("    Alice kept %s tokens as change", changeAmt)
	t.Logf("    Relayer earned spendable note of %s tokens", relayerAmt)
}
