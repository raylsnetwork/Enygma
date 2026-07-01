package tags_test

// TestFullPaymentWithTagsViaRelayer — complete end-to-end test combining
// the ZK retail payment layer with the private tag notification layer,
// with ALL on-chain transactions routed through the relayer.
//
// This means neither Alice's nor Bob's Ethereum address ever appears as
// msg.sender for any on-chain transaction — full sender privacy.
//
// Flow:
//
//   Phase 1 — Registration
//     Alice and Bob generate ZK key pairs and register in UserRegistry.
//
//   Phase 2 — Channel Setup via POST /relay/channel
//     Alice prepares (c1, c2, bitmap) locally and sends to relayer.
//     Relayer submits openChannel() — msg.sender = relayer, not Alice.
//     Bob scans TagChannelRegistry and recovers ss_channel.
//
//   Phase 3 — Deposit
//     Alice deposits 40 ERC-20 tokens into the vault.
//     (Direct — Alice depositing her own tokens needs no privacy.)
//
//   Phase 4 — ZK Payment Proof
//     Alice generates a Groth16 proof: pay 30 to Bob, keep 10 change.
//
//   Phase 5 — Payment via POST /relay/payment
//     Alice sends the unsigned proof to the relayer.
//     Relayer validates root + nullifier, submits payment() — msg.sender = relayer.
//
//   Phase 6 — Tag Notification via POST /relay/tag
//     Alice encrypts Bob's note {amount, tokenId, saltB} with channelKey.
//     Alice sends tag window to relayer.
//     Relayer submits publishTag() — msg.sender = relayer, not Alice.
//
//   Phase 7 — Bob Receives via Tag Scan
//     Bob scans TagRegistry, finds his tag, decrypts note, verifies commitment.
//
//   Phase 8 — Sender Privacy Verification
//     All three on-chain transactions (channel, payment, tag) confirm
//     msg.sender = relayer address, never Alice or Bob.
//
// Prerequisites:
//
//	Terminal 1: cd enygma_dvp && npx hardhat node              (fresh node)
//	Terminal 2: bash setup.sh                                   (deploy + init contracts)
//	Terminal 3: cd gnark_circuits && go run main.go             (gnark server :8082)
//	Terminal 4: cd relayer && \
//	              RELAYER_PRIVATE_KEY=9883c26cc126a37158c4ffcc9d401d3ffa41187d9b1a18ce4912398d22597cda \
//	              RELAYER_API_KEY=test-api-key-dev-only \
//	              RELAYER_TAG_CHANNEL_REGISTRY_ADDR=<addr> \
//	              RELAYER_TAG_REGISTRY_ADDR=<addr> \
//	              go run main.go
//
// Run:
//
//	cd private_tags/test && CC=/usr/bin/clang go test -run TestFullPaymentWithTagsViaRelayer -v -timeout 300s

import (
	"context"
	"math/big"
	"net/http"
	"strings"
	"testing"

	tags   "github.com/raylsnetwork/enygma_retail_payments/private_tags/src"
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// ── Payment relay request / response types ────────────────────────────────────

// relayPaymentReq is the JSON body sent to POST /relay/payment.
type relayPaymentReq struct {
	VaultId      string    `json:"vaultId"`
	Proof        [8]string `json:"proof"`
	PublicSignal [7]string `json:"publicSignal"`
	CipherText   string    `json:"cipherText"`
	EncTxData    string    `json:"encTxData"`
}

// relayPaymentResp is the JSON body returned by POST /relay/payment.
type relayPaymentResp struct {
	TxHash      string `json:"txHash"`
	BlockNumber uint64 `json:"blockNumber"`
	GasUsed     uint64 `json:"gasUsed"`
	Error       string `json:"error,omitempty"`
}

// buildPayRelayRequest converts a PaymentResult into the relayer's JSON format.
func buildPayRelayRequest(t *testing.T, result *rpcore.PaymentResult) relayPaymentReq {
	t.Helper()
	if len(result.Proof) != 8 {
		t.Fatalf("expected 8 proof elements, got %d", len(result.Proof))
	}
	var proof [8]string
	copy(proof[:], result.Proof)

	// ContractStatement: non-interleaved [msg, treeNum, root, nullifier, cmt_bob, cmt_change, contractAddr]
	stmt := result.ContractStatement()
	if len(stmt) != 7 {
		t.Fatalf("expected 7 statement elements, got %d", len(stmt))
	}
	var sig [7]string
	for i, v := range stmt {
		sig[i] = v.String()
	}

	return relayPaymentReq{
		VaultId:      "0",
		Proof:        proof,
		PublicSignal: sig,
		CipherText:   toHex(result.CipherText),
		EncTxData:    toHex(result.EncTxData),
	}
}

// ── Main test ──────────────────────────────────────────────────────────────────

func TestFullPaymentWithTagsViaRelayer(t *testing.T) {
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

	// ── If relayer not running: deploy contracts and print start command ───────
	if !relayerAvailable() {
		chAddr, err := tags.DeployTagChannelRegistry(client, aliceAuth, "../contracts/TagChannelRegistry.json")
		if err != nil {
			t.Fatalf("DeployTagChannelRegistry: %v", err)
		}
		trAddr, err := tags.DeployTagRegistry(client, aliceAuth, "../contracts/TagRegistry.json")
		if err != nil {
			t.Fatalf("DeployTagRegistry: %v", err)
		}
		t.Logf("Relayer not running. Start it with:")
		t.Logf("")
		t.Logf("  cd ../../relayer")
		t.Logf("  RELAYER_PRIVATE_KEY=9883c26cc126a37158c4ffcc9d401d3ffa41187d9b1a18ce4912398d22597cda \\")
		t.Logf("  RELAYER_API_KEY=test-api-key-dev-only \\")
		t.Logf("  RELAYER_TAG_CHANNEL_REGISTRY_ADDR=%s \\", chAddr.Hex())
		t.Logf("  RELAYER_TAG_REGISTRY_ADDR=%s \\", trAddr.Hex())
		t.Logf("  go run main.go")
		t.Logf("")
		t.Logf("Then re-run: CC=/usr/bin/clang go test -run TestFullPaymentWithTagsViaRelayer -v -timeout 300s")
		t.Skip("relayer not running on localhost:8090")
	}

	// ── GET /relay/info — discover configured contract addresses ──────────────
	t.Log("── Discovering relayer configuration via GET /relay/info ──")

	var info relayInfoResp
	if status := getJSON(t, "/relay/info", &info); status != http.StatusOK {
		t.Fatalf("GET /relay/info: status=%d", status)
	}
	t.Logf("  relayerAddr:            %s", info.RelayerAddr)
	t.Logf("  tagRegistryAddr:        %s", info.TagRegistryAddr)
	t.Logf("  tagChannelRegistryAddr: %s", info.TagChannelRegistryAddr)

	zeroAddr := common.Address{}.Hex()
	if info.TagChannelRegistryAddr == zeroAddr || info.TagRegistryAddr == zeroAddr {
		chAddr, _ := tags.DeployTagChannelRegistry(client, aliceAuth, "../contracts/TagChannelRegistry.json")
		trAddr, _ := tags.DeployTagRegistry(client, aliceAuth, "../contracts/TagRegistry.json")
		t.Skipf("Restart relayer with:\n"+
			"  RELAYER_TAG_CHANNEL_REGISTRY_ADDR=%s\n"+
			"  RELAYER_TAG_REGISTRY_ADDR=%s", chAddr.Hex(), trAddr.Hex())
	}

	channelRegistryAddr := common.HexToAddress(info.TagChannelRegistryAddr)
	tagRegistryAddr     := common.HexToAddress(info.TagRegistryAddr)
	relayerEthAddr      := common.HexToAddress(info.RelayerAddr)

	// ── Load payment contracts from receipts.json ─────────────────────────────
	receipts     := loadPaymentReceipts(t)
	vaultAddr    := common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress)
	erc20Addr    := common.HexToAddress(receipts["ERC20"].ContractAddress)
	registryAddr := common.HexToAddress(receipts["UserRegistry"].ContractAddress)

	vaultABI := loadPaymentABI(t, "Erc20CoinVault")
	erc20ABI := loadPaymentABI(t, "RaylsERC20")

	vault := bindPayContract(t, client, vaultAddr, vaultABI)
	erc20 := bindPayContract(t, client, erc20Addr, erc20ABI)

	gnarkClient := rpcore.NewPaymentClient("")
	merkleDepth := 8
	tokenId     := big.NewInt(0)
	depositAmt  := big.NewInt(40)
	paymentAmt  := big.NewInt(30)
	changeAmt   := big.NewInt(10)

	// ═════════════════════════════════════════════════════════════════════════
	// PHASE 1 — Key Generation and Registration
	// ═════════════════════════════════════════════════════════════════════════
	t.Log("── Phase 1: Key Generation and Registration ──")

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

	if err := rpcore.Register(client, aliceAuth, registryAddr, aliceSpend.PublicKey, aliceView.EncapsKey, make([]byte, 1088), make([]byte, 92)); err != nil {
		if !strings.Contains(err.Error(), "AlreadyRegistered") && !strings.Contains(err.Error(), "45ed80e9") {
			t.Fatalf("Alice Register: %v", err)
		}
		t.Log("  Alice already registered")
	} else {
		t.Logf("  Alice registered (%s)", aliceAuth.From.Hex())
	}

	if err := rpcore.Register(client, bobAuth, registryAddr, bobSpend.PublicKey, bobView.EncapsKey, make([]byte, 1088), make([]byte, 92)); err != nil {
		if !strings.Contains(err.Error(), "AlreadyRegistered") && !strings.Contains(err.Error(), "45ed80e9") {
			t.Fatalf("Bob Register: %v", err)
		}
		t.Log("  Bob already registered")
	} else {
		t.Logf("  Bob registered (%s)", bobAuth.From.Hex())
	}

	// ═════════════════════════════════════════════════════════════════════════
	// PHASE 2 — Channel Setup via POST /relay/channel
	// msg.sender = relayer — Alice's Ethereum address never appears on-chain
	// ═════════════════════════════════════════════════════════════════════════
	t.Log("── Phase 2: Channel Setup via /relay/channel ──")

	// Alice looks up total users and Bob's index for the privacy bitmap.
	totalUsers, err := tags.GetUserCount(client, registryAddr)
	if err != nil { t.Fatalf("GetUserCount: %v", err) }
	bobIdx, err := tags.GetRecipientIndex(client, registryAddr, bobAuth.From)
	if err != nil { t.Fatalf("GetRecipientIndex (Bob): %v", err) }
	t.Logf("  total users: %d  Bob index: %d", totalUsers, bobIdx)

	// Alice prepares (c1, c2, bitmap) locally — no on-chain call yet.
	channelSS, c1, c2, bitmap, err := tags.PrepareChannelSetup(
		bobView.EncapsKey,
		[]byte("Payment channel ready"),
		[]byte(aliceAddr),        // Alice identifies herself inside c2
		tags.PrivacyFull,
		totalUsers,
		bobIdx,
		nil,
	)
	if err != nil { t.Fatalf("PrepareChannelSetup: %v", err) }
	t.Logf("  ss_channel: %x...  c1=%dB  c2=%dB  bitmap=%dB",
		channelSS[:8], len(c1), len(c2), len(bitmap))

	// Alice sends (c1, c2, bitmap) to the relayer.
	// Relayer calls openChannel() — msg.sender = relayer, not Alice.
	var chResp relayChannelResponse
	status := postJSON(t, "/relay/channel", relayChannelRequest{
		C1:     toHex(c1),
		C2:     toHex(c2),
		Bitmap: toHex(bitmap),
	}, &chResp)

	if status == http.StatusServiceUnavailable {
		t.Skipf("relayer TagChannelRegistry not configured — restart with RELAYER_TAG_CHANNEL_REGISTRY_ADDR=<addr>")
	}
	if status != http.StatusOK {
		t.Fatalf("POST /relay/channel: status=%d error=%s", status, chResp.Error)
	}
	t.Logf("  channel published: idx=%d  block=%d  gas=%d", chResp.ChannelIdx, chResp.BlockNumber, chResp.GasUsed)

	// Verify msg.sender = relayer (not Alice).
	chSender := txSender(t, client, chResp.TxHash)
	if chSender == common.HexToAddress(aliceAddr) {
		t.Error("channel tx.from == Alice — relayer did not sign")
	}
	t.Logf("  channel tx.from = %s (relayer, not Alice) ✓", chSender.Hex())

	// Bob scans TagChannelRegistry — scan only the channel just published.
	bobFound, err := tags.ScanChannels(client, channelRegistryAddr, bobView.DecapsKey, chResp.ChannelIdx, 1)
	if err != nil { t.Fatalf("ScanChannels (Bob): %v", err) }
	if len(bobFound) != 1 {
		t.Fatalf("Bob expected 1 channel, found %d", len(bobFound))
	}

	bobChannelSS := bobFound[0].SharedSecret
	t.Logf("  Bob found channel — recovered ss: %x...", bobChannelSS[:8])
	t.Logf("  message: %q  senderId: %q", string(bobFound[0].Message), string(bobFound[0].SenderId))

	// Both sides must share the same secret.
	if new(big.Int).SetBytes(channelSS).Cmp(new(big.Int).SetBytes(bobChannelSS)) != 0 {
		t.Fatal("shared secret mismatch between Alice and Bob")
	}
	t.Log("  shared secrets match ✓")

	// ═════════════════════════════════════════════════════════════════════════
	// PHASE 3 — Deposit
	// Alice deposits her own tokens directly — no privacy concern here.
	// ═════════════════════════════════════════════════════════════════════════
	t.Log("── Phase 3: Alice deposits 40 ERC-20 tokens ──")

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

	// ═════════════════════════════════════════════════════════════════════════
	// PHASE 4 — ZK Payment Proof
	// ═════════════════════════════════════════════════════════════════════════
	t.Log("── Phase 4: Generating ZK payment proof (30 to Bob, 10 change to Alice) ──")

	mt := loadPaymentMerkleTree(t, client, vaultAddr, merkleDepth)
	aliceProof, err := mt.GenerateProof(aliceCommitment)
	if err != nil { t.Fatalf("GenerateProof (Alice): %v", err) }
	t.Logf("  Merkle root: %s", aliceProof.Root)

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

	// ═════════════════════════════════════════════════════════════════════════
	// PHASE 5 — Payment via POST /relay/payment
	// Alice sends unsigned proof to relayer.
	// Relayer validates root + nullifier, submits payment() — msg.sender = relayer.
	// ═════════════════════════════════════════════════════════════════════════
	t.Log("── Phase 5: Payment via /relay/payment ──")

	var payResp relayPaymentResp
	status = postJSON(t, "/relay/payment", buildPayRelayRequest(t, paymentResult), &payResp)
	if status != http.StatusOK {
		t.Fatalf("POST /relay/payment: status=%d error=%s", status, payResp.Error)
	}
	t.Logf("  payment mined: block=%d  gas=%d  tx=%s...",
		payResp.BlockNumber, payResp.GasUsed, payResp.TxHash[:10])

	// Verify msg.sender = relayer (not Alice).
	paySender := txSender(t, client, payResp.TxHash)
	if paySender == common.HexToAddress(aliceAddr) {
		t.Error("payment tx.from == Alice — relayer did not sign")
	}
	t.Logf("  payment tx.from = %s (relayer, not Alice) ✓", paySender.Hex())

	// Verify on-chain events.
	paymentSig  := crypto.Keccak256Hash([]byte("Payment(uint256,uint256,bytes,bytes)"))
	nullifierSig := crypto.Keccak256Hash([]byte("Nullifier(uint256,uint256,uint256)"))
	txReceipt, err := client.TransactionReceipt(ctx, common.HexToHash(payResp.TxHash))
	if err != nil { t.Fatalf("TransactionReceipt: %v", err) }
	var payEvents, nfEvents int
	for _, log := range txReceipt.Logs {
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

	// ═════════════════════════════════════════════════════════════════════════
	// PHASE 6 — Tag Notification via POST /relay/tag
	// Alice sends tag window to relayer.
	// Relayer submits publishTag() — msg.sender = relayer, not Alice.
	// ═════════════════════════════════════════════════════════════════════════
	t.Log("── Phase 6: Tag Notification via /relay/tag (window mode) ──")

	startBlock, windowTags, noteCtxt, err := tags.PreparePaymentTag(
		client, 3,
		bobSpend.PublicKey,
		channelSS,
		paymentAmt, tokenId, paymentResult.SaltB,
	)
	if err != nil { t.Fatalf("PreparePaymentTag: %v", err) }
	t.Logf("  tag window: blocks [%d, %d)  ctxt=%d bytes", startBlock, startBlock+3, len(noteCtxt))

	hexTags := make([]string, len(windowTags))
	for i, wt := range windowTags {
		wt := wt
		hexTags[i] = toHex(wt[:])
	}

	var tagResp relayTagResp
	status = postJSON(t, "/relay/tag", relayTagReq{
		Tags:       hexTags,
		StartBlock: startBlock,
		Ctxt:       toHex(noteCtxt),
	}, &tagResp)

	if status == http.StatusServiceUnavailable {
		t.Skipf("relayer TagRegistry not configured — restart with RELAYER_TAG_REGISTRY_ADDR=<addr>")
	}
	if status != http.StatusOK {
		t.Fatalf("POST /relay/tag: status=%d error=%s", status, tagResp.Error)
	}
	tagBlock := tagResp.BlockNumber
	t.Logf("  tag published: block=%d  gas=%d  tx=%s...",
		tagBlock, tagResp.GasUsed, tagResp.TxHash[:10])

	// Verify msg.sender = relayer (not Alice).
	tagSender := txSender(t, client, tagResp.TxHash)
	if tagSender == common.HexToAddress(aliceAddr) {
		t.Error("tag tx.from == Alice — relayer did not sign")
	}
	t.Logf("  tag tx.from = %s (relayer, not Alice) ✓", tagSender.Hex())

	// ═════════════════════════════════════════════════════════════════════════
	// PHASE 7 — Bob Receives via Tag Scan
	// ═════════════════════════════════════════════════════════════════════════
	t.Log("── Phase 7: Bob scans TagRegistry for his payment tag ──")

	bobChannels := []tags.Channel{{SharedSecret: bobChannelSS, PkSpend: bobSpend.PublicKey}}
	cursor := tags.NewScanCursor()
	matches, _, err := tags.ScanBlocksFromCursor(client, tagRegistryAddr, bobChannels, cursor, tagBlock)
	if err != nil { t.Fatalf("ScanBlocksFromCursor (Bob): %v", err) }
	if len(matches) != 1 {
		t.Fatalf("Bob expected 1 matching tag, got %d", len(matches))
	}
	t.Logf("  Bob found tag at block %d ✓", matches[0].BlockNumber)

	// Bob decrypts the tag payload to recover the note.
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

	// Bob recomputes his commitment and verifies it matches what is on-chain.
	bobRecomputedCmt, err := rpcore.Erc20CommitmentV2(
		bobSpend.PublicKey, note.Salt, note.Amount, note.TokenId)
	if err != nil { t.Fatalf("Erc20CommitmentV2 (Bob verify): %v", err) }

	if bobRecomputedCmt.Cmp(paymentResult.Statement[4]) != 0 {
		t.Errorf("Bob's recomputed commitment mismatch:\n  got:  %s\n  want: %s",
			bobRecomputedCmt, paymentResult.Statement[4])
	}
	t.Logf("  Bob recomputed commitment: %s ✓", bobRecomputedCmt)
	t.Log("  Bob can now generate a Merkle proof and withdraw")

	// Alice verifies her change note.
	aliceChangeCmt, err := rpcore.Erc20CommitmentV2(
		aliceSpend.PublicKey, paymentResult.SaltA, changeAmt, tokenId)
	if err != nil { t.Fatalf("Erc20CommitmentV2 (Alice change): %v", err) }
	if aliceChangeCmt.Cmp(paymentResult.Statement[5]) != 0 {
		t.Errorf("Alice's change commitment mismatch")
	}
	t.Logf("  Alice change commitment: %s ✓", aliceChangeCmt)

	// ═════════════════════════════════════════════════════════════════════════
	// PHASE 8 — Sender Privacy Verification
	// All three on-chain transactions must have msg.sender = relayer.
	// ═════════════════════════════════════════════════════════════════════════
	t.Log("── Phase 8: Sender Privacy Verification ──")

	aliceEthAddr := common.HexToAddress(aliceAddr)
	bobEthAddr   := common.HexToAddress(bobAddr)

	for _, check := range []struct {
		name   string
		sender common.Address
	}{
		{"channel setup", chSender},
		{"payment",       paySender},
		{"tag",           tagSender},
	} {
		if check.sender == aliceEthAddr {
			t.Errorf("%s: tx.from == Alice — sender privacy violated", check.name)
		}
		if check.sender == bobEthAddr {
			t.Errorf("%s: tx.from == Bob — sender privacy violated", check.name)
		}
		if check.sender != relayerEthAddr {
			t.Errorf("%s: tx.from=%s is not the relayer %s",
				check.name, check.sender.Hex(), relayerEthAddr.Hex())
		}
		t.Logf("  %-15s tx.from = %s (relayer) ✓", check.name, check.sender.Hex())
	}

	t.Log("")
	t.Log("=== FULL PAYMENT WITH TAGS VIA RELAYER COMPLETE ===")
	t.Logf("  Phase 1 — Alice and Bob registered")
	t.Logf("  Phase 2 — Channel setup:  msg.sender = relayer ✓")
	t.Logf("  Phase 3 — Alice deposited %s tokens", depositAmt)
	t.Logf("  Phase 4 — ZK proof generated (gnark server)")
	t.Logf("  Phase 5 — Payment:        msg.sender = relayer ✓")
	t.Logf("  Phase 6 — Tag:            msg.sender = relayer ✓")
	t.Logf("  Phase 7 — Bob found note via tag scan, commitment verified")
	t.Logf("  Phase 8 — Alice's Ethereum address never appeared as tx.from ✓")
	t.Logf("            Bob's Ethereum address never appeared as tx.from ✓")
	t.Logf("            Relayer address (%s) was msg.sender for all 3 txs", relayerEthAddr.Hex())
}

// ── Privacy mode helpers ───────────────────────────────────────────────────────

func countBits(bitmap []byte, n int) int {
	count := 0
	for i := 0; i < n; i++ {
		if i/8 < len(bitmap) && bitmap[i/8]&(1<<(uint(i)%8)) != 0 {
			count++
		}
	}
	return count
}

func bitIsSet(bitmap []byte, i int) bool {
	return i/8 < len(bitmap) && bitmap[i/8]&(1<<(uint(i)%8)) != 0
}

// TestAllPrivacyModesViaRelayer verifies all four privacy modes (None, Subset,
// Rift, Full) with channel setup routed through the relayer.
//
// For each mode this test:
//  1. Prepares (c1, c2, bitmap) locally — bitmap encodes the privacy mode.
//  2. POSTs to /relay/channel — relayer submits openChannel(), msg.sender = relayer.
//  3. Verifies tx.from = relayer (not Bob who initiated the setup).
//  4. Verifies bitmap bit counts match the expected mode semantics.
//  5. Alice (recipient) scans and finds exactly her channel.
//  6. Decrypted message and senderId round-trip correctly.
//  7. Carol (wrong key) finds nothing.
//
// Prerequisites: same as TestFullPaymentWithTagsViaRelayer.
//
// Run:
//
//	cd private_tags/test && CC=/usr/bin/clang go test -run TestAllPrivacyModesViaRelayer -v -timeout 300s
func TestAllPrivacyModesViaRelayer(t *testing.T) {
	if !chainAvailable() {
		t.Skip("Hardhat node not running on localhost:8545 — skipping")
	}
	if !relayerAvailable() {
		t.Skip("relayer not running on localhost:8090 — skipping")
	}

	client := mustDial(t)
	defer client.Close()

	aliceAuth := hardhatAuthFromKey(t, alicePrivKeyHex)
	bobAuth   := hardhatAuthFromKey(t, bobPrivKeyHex)
	_ = bobAuth // Bob initiates channel but sends via relayer — auth not needed for tx

	// ── Discover registry addresses ───────────────────────────────────────────
	var info relayInfoResp
	if status := getJSON(t, "/relay/info", &info); status != http.StatusOK {
		t.Fatalf("GET /relay/info: status=%d", status)
	}
	zeroAddr := common.Address{}.Hex()
	if info.TagChannelRegistryAddr == zeroAddr {
		t.Skip("relayer TagChannelRegistry not configured — restart with RELAYER_TAG_CHANNEL_REGISTRY_ADDR=<addr>")
	}
	channelRegistryAddr := common.HexToAddress(info.TagChannelRegistryAddr)
	relayerEthAddr      := common.HexToAddress(info.RelayerAddr)

	// ── Load UserRegistry for bitmap parameters ───────────────────────────────
	receipts     := loadPaymentReceipts(t)
	registryAddr := common.HexToAddress(receipts["UserRegistry"].ContractAddress)

	// Alice and Carol generate ML-KEM keypairs.
	// Alice is the recipient in all modes; Carol has a completely different key.
	aliceDK, err := rpcore.NewViewKeyPair()
	if err != nil { t.Fatalf("NewViewKeyPair (Alice): %v", err) }
	carolDK, err := rpcore.NewViewKeyPair()
	if err != nil { t.Fatalf("NewViewKeyPair (Carol): %v", err) }

	// Register Alice so she has an index in UserRegistry for the bitmap.
	aliceSpend, err := rpcore.NewSpendKeyPair()
	if err != nil { t.Fatalf("NewSpendKeyPair (Alice): %v", err) }
	if err := rpcore.Register(client, aliceAuth, registryAddr,
		aliceSpend.PublicKey, aliceDK.EncapsKey, make([]byte, 1088), make([]byte, 92)); err != nil {
		if !strings.Contains(err.Error(), "AlreadyRegistered") &&
			!strings.Contains(err.Error(), "45ed80e9") {
			t.Fatalf("Alice Register: %v", err)
		}
	}

	// Look up Alice's registration index and total users for the bitmap.
	totalUsers, err := tags.GetUserCount(client, registryAddr)
	if err != nil { t.Fatalf("GetUserCount: %v", err) }
	aliceIdx, err := tags.GetRecipientIndex(client, registryAddr, aliceAuth.From)
	if err != nil { t.Fatalf("GetRecipientIndex (Alice): %v", err) }
	t.Logf("UserRegistry: totalUsers=%d  aliceIdx=%d", totalUsers, aliceIdx)

	// Excluded indices for PrivacyRift — all users except Alice and index 0.
	excludedForRift := []int{}
	for i := 0; i < totalUsers; i++ {
		if i != aliceIdx && i != 0 {
			excludedForRift = append(excludedForRift, i)
		}
	}

	// ── Define the four test cases ────────────────────────────────────────────

	type modeCase struct {
		mode        tags.PrivacyMode
		message     string
		senderId    string
		wantMinBits int
		wantMaxBits int // -1 = exact totalUsers
		wantRecipientSet bool
	}

	cases := []modeCase{
		{
			mode:             tags.PrivacyNone,
			message:          "none-mode: recipient only",
			senderId:         "sender-none",
			wantMinBits:      1,
			wantMaxBits:      1,
			wantRecipientSet: true,
		},
		{
			mode:             tags.PrivacySubset,
			message:          "subset-mode: k-anonymity",
			senderId:         "sender-subset",
			wantMinBits:      2,
			wantMaxBits:      totalUsers,
			wantRecipientSet: true,
		},
		{
			mode:             tags.PrivacyRift,
			message:          "rift-mode: all except excluded",
			senderId:         "sender-rift",
			wantMinBits:      totalUsers - len(excludedForRift),
			wantMaxBits:      totalUsers - len(excludedForRift),
			wantRecipientSet: true,
		},
		{
			mode:             tags.PrivacyFull,
			message:          "full-mode: all users",
			senderId:         "sender-full",
			wantMinBits:      totalUsers,
			wantMaxBits:      totalUsers,
			wantRecipientSet: true,
		},
	}

	// Track channel index cursor so each sub-test scans only its own channel.
	var channelCursor uint64

	for _, tc := range cases {
		tc := tc
		t.Run(tags.BitmapDescription(tc.mode)[:4], func(t *testing.T) {

			excluded := excludedForRift
			if tc.mode != tags.PrivacyRift {
				excluded = nil
			}

			// ── Bob prepares channel setup locally ───────────────────────────
			ssBob, c1, c2, bitmap, err := tags.PrepareChannelSetup(
				aliceDK.EncapsKey,
				[]byte(tc.message),
				[]byte(tc.senderId),
				tc.mode,
				totalUsers,
				aliceIdx,
				excluded,
			)
			if err != nil { t.Fatalf("PrepareChannelSetup: %v", err) }
			t.Logf("  c1=%dB  c2=%dB  bitmap=%dB", len(c1), len(c2), len(bitmap))

			// ── Verify bitmap structure before publishing ────────────────────
			setBits := countBits(bitmap, totalUsers)
			t.Logf("  bitmap set bits: %d (want %d..%d)",
				setBits, tc.wantMinBits, tc.wantMaxBits)

			if tc.wantRecipientSet && !bitIsSet(bitmap, aliceIdx) {
				t.Errorf("recipient bit %d must always be set", aliceIdx)
			}
			if setBits < tc.wantMinBits {
				t.Errorf("too few set bits: got %d, want >= %d", setBits, tc.wantMinBits)
			}
			if tc.wantMaxBits > 0 && setBits > tc.wantMaxBits {
				t.Errorf("too many set bits: got %d, want <= %d", setBits, tc.wantMaxBits)
			}
			if tc.mode == tags.PrivacyRift {
				for _, ex := range excludedForRift {
					if bitIsSet(bitmap, ex) {
						t.Errorf("PrivacyRift: excluded user %d should not be in bitmap", ex)
					}
				}
			}

			// ── POST /relay/channel ──────────────────────────────────────────
			// Bob sends (c1, c2, bitmap) to the relayer.
			// Relayer submits openChannel() — msg.sender = relayer, not Bob.
			var chResp relayChannelResponse
			status := postJSON(t, "/relay/channel", relayChannelRequest{
				C1:     toHex(c1),
				C2:     toHex(c2),
				Bitmap: toHex(bitmap),
			}, &chResp)

			if status != http.StatusOK {
				t.Fatalf("POST /relay/channel: status=%d error=%s", status, chResp.Error)
			}
			t.Logf("  channel published: idx=%d  block=%d  tx=%s...",
				chResp.ChannelIdx, chResp.BlockNumber, chResp.TxHash[:10])

			// ── Verify msg.sender = relayer ──────────────────────────────────
			sender := txSender(t, client, chResp.TxHash)
			if sender == common.HexToAddress(bobAddr) {
				t.Error("channel tx.from == Bob — relayer did not sign")
			}
			if sender == common.HexToAddress(aliceAddr) {
				t.Error("channel tx.from == Alice — relayer did not sign")
			}
			if sender != relayerEthAddr {
				t.Errorf("channel tx.from = %s, want relayer %s",
					sender.Hex(), relayerEthAddr.Hex())
			}
			t.Logf("  tx.from = %s (relayer) ✓", sender.Hex())

			// ── Alice scans — should find exactly her channel ─────────────────
			aliceFound, err := tags.ScanChannels(
				client, channelRegistryAddr, aliceDK.DecapsKey,
				channelCursor, 10,
			)
			if err != nil { t.Fatalf("ScanChannels (Alice): %v", err) }
			if len(aliceFound) != 1 {
				t.Fatalf("Alice expected 1 channel, found %d", len(aliceFound))
			}

			ch := aliceFound[0]
			t.Logf("  Alice found channel #%d ✓", ch.ChannelIdx)

			// ── Verify message and senderId ───────────────────────────────────
			if string(ch.Message) != tc.message {
				t.Errorf("message: got %q, want %q", ch.Message, tc.message)
			}
			if string(ch.SenderId) != tc.senderId {
				t.Errorf("senderId: got %q, want %q", ch.SenderId, tc.senderId)
			}
			t.Logf("  message=%q  senderId=%q ✓", ch.Message, ch.SenderId)

			// ── Verify shared secrets match ───────────────────────────────────
			if new(big.Int).SetBytes(ssBob).Cmp(
				new(big.Int).SetBytes(ch.SharedSecret)) != 0 {
				t.Error("shared secret mismatch between Bob and Alice")
			}
			t.Log("  shared secrets match ✓")

			// ── Carol (wrong key) finds nothing ───────────────────────────────
			carolFound, err := tags.ScanChannels(
				client, channelRegistryAddr, carolDK.DecapsKey,
				channelCursor, 10,
			)
			if err != nil { t.Fatalf("ScanChannels (Carol): %v", err) }
			if len(carolFound) != 0 {
				t.Errorf("Carol should find 0 channels, found %d", len(carolFound))
			}
			t.Log("  Carol (wrong key) finds nothing ✓")

			// Advance cursor past this channel for the next sub-test.
			channelCursor = ch.ChannelIdx + 1

			t.Logf("  [%s] PASSED — bitmap=%d bits, relayer signed, Alice found, Carol excluded",
				tags.BitmapDescription(tc.mode)[:4], setBits)
		})
	}

	t.Log("=== ALL PRIVACY MODES VIA RELAYER COMPLETE ===")
	t.Log("  None:   only recipient bit set — observer identifies recipient exactly")
	t.Log("  Subset: recipient + √N decoys — k-anonymity")
	t.Log("  Rift:   all except excluded — sender dissociates from specific users")
	t.Log("  Full:   all users — observer only knows two users are communicating")
	t.Log("  All four modes: msg.sender = relayer for every channel setup ✓")
}
