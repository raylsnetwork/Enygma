package tags_test

// TestPaymentWithTagNotification — end-to-end integration test combining
// the ZK retail payment layer with the private tag notification layer.
//
// This test implements the full protocol described in:
//   - protocol_description.md (payment layer)
//   - "Catch Me If You KEM" paper (private tag notification layer)
//
// Flow:
//
//	Phase 1 — Registration
//	  Alice and Bob generate ZK key pairs and register on-chain (UserRegistry).
//
//	Phase 2 — Channel Setup
//	  Alice establishes a private channel with Bob via ML-KEM (TagChannelRegistry).
//	  Bob scans and recovers the shared secret.
//
//	Phase 3 — Deposit
//	  Alice deposits 40 ERC-20 tokens into the private vault.
//
//	Phase 4 — ZK Payment
//	  Alice generates a Groth16 payment proof: 30 to Bob, 10 change to Alice.
//	  Alice submits the proof on-chain (EnygmaDvp.payment()).
//
//	Phase 5 — Tag Notification
//	  Alice encrypts Bob's note {amount, tokenId, saltB} with the channel key
//	  and publishes a private tag on-chain (TagRegistry.publishTag).
//
//	Phase 6 — Bob Receives via Tag Scan
//	  Bob scans the TagRegistry, finds his tag, decrypts the note, and
//	  verifies the commitment matches what landed on-chain.
//
// Prerequisites:
//
//	Terminal 1: cd enygma_dvp && npx hardhat node          (fresh node)
//	Terminal 2: bash setup.sh                               (deploy + init contracts)
//	Terminal 3: cd gnark_circuits && go run main.go         (gnark server on :8082)
//
// Run:
//
//	cd private_tags/test && CC=/usr/bin/clang go test -run TestPaymentWithTagNotification -v -timeout 300s

import (
	"context"
	"encoding/json"
	"math/big"
	"net"
	"os"
	"strings"
	"testing"

	tags   "github.com/raylsnetwork/enygma_retail_payments/private_tags/src"
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ── Prerequisites check ───────────────────────────────────────────────────────

func gnarkAvailable() bool {
	conn, err := net.Dial("tcp", "localhost:8082")
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// ── Receipt / ABI helpers (payment contracts) ─────────────────────────────────

type payReceiptEntry struct {
	ContractAddress string `json:"contractAddress"`
}

func loadPaymentReceipts(t *testing.T) map[string]payReceiptEntry {
	t.Helper()
	data, err := os.ReadFile("../../build/receipts.json")
	if err != nil {
		t.Fatalf("read build/receipts.json: %v — run: bash setup.sh", err)
	}
	var r map[string]payReceiptEntry
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatalf("parse receipts.json: %v", err)
	}
	return r
}

func loadPaymentABI(t *testing.T, contractName string) abi.ABI {
	t.Helper()
	data, err := os.ReadFile("../../contracts/abis/" + contractName + ".json")
	if err != nil {
		t.Fatalf("read ABI %s: %v", contractName, err)
	}
	var artifact struct {
		ABI json.RawMessage `json:"abi"`
	}
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatalf("parse artifact: %v", err)
	}
	parsed, err := abi.JSON(strings.NewReader(string(artifact.ABI)))
	if err != nil {
		t.Fatalf("parse ABI: %v", err)
	}
	return parsed
}

func bindPayContract(t *testing.T, client *ethclient.Client, addr common.Address, a abi.ABI) *bind.BoundContract {
	t.Helper()
	return bind.NewBoundContract(addr, a, client, client, client)
}

// ── On-chain ProofReceipt struct (must match EnygmaDvp ABI exactly) ───────────

type payG1Point struct {
	X *big.Int `abi:"x"`
	Y *big.Int `abi:"y"`
}
type payG2Point struct {
	X [2]*big.Int `abi:"x"`
	Y [2]*big.Int `abi:"y"`
}
type paySnarkProof struct {
	A payG1Point `abi:"a"`
	B payG2Point `abi:"b"`
	C payG1Point `abi:"c"`
}
type payProofReceipt struct {
	Proof           paySnarkProof `abi:"proof"`
	Statement       []*big.Int    `abi:"statement"`
	NumberOfInputs  *big.Int      `abi:"numberOfInputs"`
	NumberOfOutputs *big.Int      `abi:"numberOfOutputs"`
}

func buildPayProofReceipt(result *rpcore.PaymentResult) payProofReceipt {
	strs := result.Proof
	vals := make([]*big.Int, 8)
	for i, s := range strs {
		vals[i], _ = new(big.Int).SetString(s, 10)
	}
	stmt := result.ContractStatement()
	return payProofReceipt{
		Proof: paySnarkProof{
			A: payG1Point{X: vals[0], Y: vals[1]},
			B: payG2Point{X: [2]*big.Int{vals[2], vals[3]}, Y: [2]*big.Int{vals[4], vals[5]}},
			C: payG1Point{X: vals[6], Y: vals[7]},
		},
		Statement:       stmt,
		NumberOfInputs:  big.NewInt(int64(result.NumberOfInputs)),
		NumberOfOutputs: big.NewInt(int64(result.NumberOfOutputs)),
	}
}

// ── Merkle tree loader ────────────────────────────────────────────────────────

func loadPaymentMerkleTree(t *testing.T, client *ethclient.Client, vaultAddr common.Address, depth int) *rpcore.MerkleTree {
	t.Helper()
	commitmentSig := crypto.Keccak256Hash([]byte("Commitment(uint256,uint256)"))
	logs, err := client.FilterLogs(context.Background(), ethereum.FilterQuery{
		Addresses: []common.Address{vaultAddr},
		Topics:    [][]common.Hash{{commitmentSig}},
	})
	if err != nil {
		t.Fatalf("FilterLogs: %v", err)
	}
	mt := rpcore.NewMerkleTree(depth)
	for _, log := range logs {
		if len(log.Topics) >= 3 {
			mt.InsertLeaf(log.Topics[2].Big())
		}
	}
	return mt
}

// ── Main test ──────────────────────────────────────────────────────────────────

func TestPaymentWithTagNotification(t *testing.T) {
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
	depositAmt  := big.NewInt(40)
	paymentAmt  := big.NewInt(30)
	changeAmt   := big.NewInt(10)

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

	if err := rpcore.Register(client, aliceAuth, registryAddr, aliceSpend.PublicKey, aliceView.EncapsKey); err != nil {
		if !strings.Contains(err.Error(), "AlreadyRegistered") && !strings.Contains(err.Error(), "45ed80e9") {
			t.Fatalf("Alice Register: %v", err)
		}
		t.Log("  Alice already registered")
	} else {
		t.Logf("  Alice registered (%s)", aliceAuth.From.Hex())
	}

	if err := rpcore.Register(client, bobAuth, registryAddr, bobSpend.PublicKey, bobView.EncapsKey); err != nil {
		if !strings.Contains(err.Error(), "AlreadyRegistered") && !strings.Contains(err.Error(), "45ed80e9") {
			t.Fatalf("Bob Register: %v", err)
		}
		t.Log("  Bob already registered")
	} else {
		t.Logf("  Bob registered (%s)", bobAuth.From.Hex())
	}

	// ── Step 5: Alice sets up a channel with Bob (Phase 2) ───────────────────
	t.Log("Step 5 — Alice sets up a channel with Bob (ML-KEM, Phase 2)")
	t.Log("         (test-only: direct publish — in production use /relay/channel)")

	// Alice encapsulates Bob's view key to establish the channel.
	initialMsg := []byte("Payment channel ready")
	aliceSenderId := []byte(aliceAddr)

	channelSS, c1, c2, bitmap, err := tags.PrepareChannelSetup(
		bobView.EncapsKey,
		initialMsg,
		aliceSenderId,
		tags.PrivacyFull,
		2,   // totalUsers
		1,   // recipientIdx: Bob is user 1
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
	t.Logf("  message: %q  sender: %q", string(bobChannelFound[0].Message), string(bobChannelFound[0].SenderId))

	// Verify both sides share the same secret.
	if new(big.Int).SetBytes(channelSS).Cmp(new(big.Int).SetBytes(bobChannelSS)) != 0 {
		t.Fatal("shared secret mismatch between Alice and Bob")
	}
	t.Log("  shared secrets match ✓")

	// ── Step 7: Alice deposits 40 tokens ─────────────────────────────────────
	t.Log("Step 7 — Alice mints and deposits 40 ERC-20 tokens")

	mintTx, err := erc20.Transact(aliceAuth, "mint", aliceAuth.From,
		new(big.Int).Mul(depositAmt, big.NewInt(10)))
	if err != nil { t.Fatalf("ERC20.mint: %v", err) }
	if _, err := bind.WaitMined(ctx, client, mintTx); err != nil { t.Fatalf("wait mint: %v", err) }

	approveTx, err := erc20.Transact(aliceAuth, "approve", vaultAddr, depositAmt)
	if err != nil { t.Fatalf("ERC20.approve: %v", err) }
	if _, err := bind.WaitMined(ctx, client, approveTx); err != nil { t.Fatalf("wait approve: %v", err) }

	// Alice encapsulates her own view key to create her deposit note.
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
		[]*big.Int{depositAmt, aliceCommitment}, capsule, depositCtxt)
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

	// ── Step 9: Generate ZK payment proof ────────────────────────────────────
	t.Log("Step 9 — generating ZK payment proof (30 to Bob, 10 change to Alice)")

	vaultAddrBig := new(big.Int).SetBytes(vaultAddr.Bytes())
	paymentResult, err := gnarkClient.BoundPaymentProof(
		vaultAddrBig,
		big.NewInt(0),
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
	if err != nil { t.Fatalf("BoundPaymentProof: %v", err) }
	t.Logf("  proof generated")
	t.Logf("  Bob's commitment   (output 0): %s", paymentResult.Statement[4])
	t.Logf("  Alice's change cmt (output 1): %s", paymentResult.Statement[5])
	t.Logf("  Bob's saltB:                   %s", paymentResult.SaltB)

	// ── Step 10: Submit payment on-chain ─────────────────────────────────────
	t.Log("Step 10 — submitting payment on-chain (EnygmaDvp.payment)")

	receipt := buildPayProofReceipt(paymentResult)
	payTx, err := dvp.Transact(aliceAuth, "payment",
		receipt, big.NewInt(0), paymentResult.CipherText, paymentResult.EncTxData)
	if err != nil { t.Fatalf("EnygmaDvp.payment: %v", err) }
	payReceipt, err := bind.WaitMined(ctx, client, payTx)
	if err != nil { t.Fatalf("wait payment: %v", err) }
	t.Logf("  payment mined (block %d, gas %d)", payReceipt.BlockNumber, payReceipt.GasUsed)

	// Verify on-chain events.
	paymentSig  := crypto.Keccak256Hash([]byte("Payment(uint256,uint256,bytes,bytes)"))
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
	t.Log("          (test-only: direct publish — in production use /relay/tag)")

	// Use window size 3: covers startBlock, startBlock+1, startBlock+2.
	startBlock, windowTags, noteCtxt, err := tags.PreparePaymentTag(
		client, 3,
		bobSpend.PublicKey,
		channelSS,
		paymentAmt, tokenId, paymentResult.SaltB,
	)
	if err != nil { t.Fatalf("PreparePaymentTag: %v", err) }
	t.Logf("  tag window: blocks [%d, %d)  ctxt=%d bytes", startBlock, startBlock+3, len(noteCtxt))

	// Publish using windowTags[0] (expected to land in startBlock).
	tagBlock := publishTagDirect(t, client, aliceAuth, tagRegAddr, windowTags[0], noteCtxt)

	// If the block drifted, republish with the correct tag from the window.
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

	// Bob recomputes his commitment from the decrypted note.
	bobRecomputedCmt, err := rpcore.Erc20CommitmentV2(
		bobSpend.PublicKey, note.Salt, note.Amount, note.TokenId)
	if err != nil { t.Fatalf("Erc20CommitmentV2 (Bob verify): %v", err) }

	expectedBobCmt := paymentResult.Statement[4] // on-chain commitment for output 0
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
	t.Log("=== PAYMENT WITH TAG NOTIFICATION COMPLETE ===")
	t.Logf("    Alice paid %s tokens to Bob (kept %s as change)", paymentAmt, changeAmt)
	t.Logf("    Bob found his note via tag scan — no Payment event decryption needed")
	t.Logf("    Bob's commitment verified: %s", bobRecomputedCmt)
	t.Logf("    Bob can now generate a Merkle proof and withdraw")
}
