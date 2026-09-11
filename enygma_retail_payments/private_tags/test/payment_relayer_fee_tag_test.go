package tags_test

// TestFullPaymentRelayerFeeWithTagsViaRelayer — combines the PaymentRelayerFeePublic
// circuit (1-in/3-out: pay Bob, change to Alice, spendable fee note to the relayer)
// with the private tag notification layer, all on-chain transactions routed through
// the relayer — same sender-privacy property as TestFullPaymentWithTagsViaRelayer,
// but exercising the newer relayer-fee mechanism (paymentWithRelayerFee() +
// relayerFixedFeeAmount on-chain enforcement + POST /relay/payment_relayer_fee)
// instead of plain payment().
//
// Flow:
//
//	Phase 1 — Registration            Alice and Bob register in UserRegistry.
//	Phase 2 — Channel Setup           via POST /relay/channel (msg.sender = relayer).
//	Phase 3 — Deposit                 Alice deposits 45 tokens (30 pay + 10 change + 5 fee).
//	Phase 4 — ZK Proof                PaymentRelayerFeePublic: pay=30, change=10, fee=5
//	                                   (fee note addressed to the relayer's published
//	                                   feeSpendPubKey from GET /relay/info).
//	Phase 5 — Relay                   POST /relay/payment_relayer_fee — msg.sender = relayer.
//	                                   On-chain: paymentWithRelayerFee() checks the
//	                                   proof's public StFee against relayerFixedFeeAmount.
//	Phase 6 — Tag Notification        via POST /relay/tag — msg.sender = relayer.
//	Phase 7 — Bob Receives            scans TagRegistry, decrypts note, verifies commitment.
//	Phase 8 — Sender Privacy          all 3 txs (channel/payment/tag) have tx.from = relayer.
//
// TestPaymentRelayerFeeViaRelayer_InvalidFeeRejected then verifies the fixed-fee
// mechanism itself, independent of tags: a fee amount that doesn't match the
// contract's configured relayerFixedFeeAmount is rejected on-chain (InvalidRelayerFee),
// and a fee note not addressed to the relayer's own key is rejected off-chain (402)
// before ever touching chain.
//
// Prerequisites:
//
//	Terminal 1: cd enygma_dvp && npx hardhat node
//	Terminal 2: bash setup.sh
//	Terminal 3: cd gnark_circuits && go run main.go
//	Terminal 4: cd relayer && \
//	              RELAYER_PRIVATE_KEY=9883c26cc126a37158c4ffcc9d401d3ffa41187d9b1a18ce4912398d22597cda \
//	              RELAYER_API_KEY=test-api-key-dev-only \
//	              RELAYER_FEE_SPEND_PRIVATE_KEY=123456789 \
//	              RELAYER_TAG_CHANNEL_REGISTRY_ADDR=<addr> \
//	              RELAYER_TAG_REGISTRY_ADDR=<addr> \
//	              go run main.go
//
// Run:
//
//	cd private_tags/test && CC=/usr/bin/clang go test -run 'TestFullPaymentRelayerFeeWithTagsViaRelayer|TestPaymentRelayerFeeViaRelayer_InvalidFeeRejected' -v -timeout 300s

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"testing"

	dvpcore "github.com/raylsnetwork/enygma_dvp/src/core"
	tags "github.com/raylsnetwork/enygma_retail_payments/private_tags/src"
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	rfTagPaymentAmt = 30
	rfTagChangeAmt  = 10
	rfTagFeeAmt     = 5
)

// ── gnark proof response (POST /proof/paymentRelayerFeePublic) ────────────────

type rfGnarkProofResp struct {
	Proof        []*big.Int `json:"proof"`
	PublicSignal []*big.Int `json:"publicSignal"`
	Error        string     `json:"error"`
}

func postGnarkProof(url string, body interface{}) (*rfGnarkProofResp, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("http.Post: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gnark server %d: %s", resp.StatusCode, string(raw))
	}
	var out rfGnarkProofResp
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if out.Error != "" {
		return nil, fmt.Errorf("gnark error: %s", out.Error)
	}
	return &out, nil
}

// ── relay request/response (POST /relay/payment_relayer_fee) ──────────────────

type relayRelayerFeeReq struct {
	VaultId      string    `json:"vaultId"`
	Proof        [8]string `json:"proof"`
	PublicSignal [9]string `json:"publicSignal"`
	CipherText   string    `json:"cipherText"`
	EncTxData    string    `json:"encTxData"`
	FeeSalt      string    `json:"feeSalt"`
	TokenId      string    `json:"tokenId"`
}

type relayRelayerFeeResp struct {
	TxHash      string `json:"txHash"`
	BlockNumber uint64 `json:"blockNumber"`
	GasUsed     uint64 `json:"gasUsed"`
	Error       string `json:"error,omitempty"`
}

// ── shared witness-building helper ─────────────────────────────────────────────

// relayerFeeWitness holds everything needed to submit and verify one
// PaymentRelayerFeePublic proof: the gnark response plus the locally-computed
// values (commitments/salts) used to cross-check the public signal.
type relayerFeeWitness struct {
	proof        [8]string
	publicSignal [9]string
	ctxtBob      []byte
	ctxtIIBob    []byte
	feeSalt      *big.Int
	saltA        *big.Int
	saltBobField *big.Int
	tokenId      *big.Int
	cmtBob       *big.Int
	cmtChange    *big.Int
	cmtRelayer   *big.Int
}

// depositAndBuildRelayerFeeWitness deposits a fresh note for Alice and builds a
// PaymentRelayerFeePublic proof (pay 30 to Bob, 10 change to Alice, a fee note
// of relayerFeeAmt addressed to feeRecipientPk). depositAmt is computed from
// the actual amounts so the circuit's conservation law always holds, even when
// a test deliberately picks a relayerFeeAmt that mismatches the contract's
// configured relayerFixedFeeAmount.
func depositAndBuildRelayerFeeWitness(
	t *testing.T,
	ctx context.Context,
	client *ethclient.Client,
	vault, erc20 *bind.BoundContract,
	vaultAddr common.Address,
	aliceAuth *bind.TransactOpts,
	bobSpendPk *big.Int,
	bobViewEncapsKey []byte,
	relayerFeeAmt, feeRecipientPk *big.Int,
) relayerFeeWitness {
	t.Helper()

	merkleDepth := 8
	tokenId := big.NewInt(0)
	paymentAmt := big.NewInt(rfTagPaymentAmt)
	changeAmt := big.NewInt(rfTagChangeAmt)
	depositAmt := new(big.Int).Add(new(big.Int).Add(paymentAmt, changeAmt), relayerFeeAmt)
	vaultAddrBig := new(big.Int).SetBytes(vaultAddr.Bytes())

	aliceSpend, err := rpcore.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("Alice NewSpendKeyPair: %v", err)
	}
	aliceView, err := rpcore.NewViewKeyPair()
	if err != nil {
		t.Fatalf("Alice NewViewKeyPair: %v", err)
	}

	mintTx, err := erc20.Transact(aliceAuth, "mint", aliceAuth.From, new(big.Int).Mul(depositAmt, big.NewInt(10)))
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
	if _, err := bind.WaitMined(ctx, client, depositTx); err != nil {
		t.Fatalf("wait depositV2: %v", err)
	}

	mt := loadPaymentMerkleTree(t, client, vaultAddr, merkleDepth)
	aliceProof, err := mt.GenerateProof(aliceCommitment)
	if err != nil {
		t.Fatalf("GenerateProof: %v", err)
	}

	nullifier, err := dvpcore.GetNullifier(aliceSpend.PrivateKey, aliceProof.Indices)
	if err != nil {
		t.Fatalf("GetNullifier: %v", err)
	}

	ssBob, ctxtBob, err := rpcore.Encapsulate(bobViewEncapsKey)
	if err != nil {
		t.Fatalf("Encapsulate (Bob): %v", err)
	}
	saltBobRaw, err := rpcore.DerivePaymentSalt(ssBob)
	if err != nil {
		t.Fatalf("DerivePaymentSalt (Bob): %v", err)
	}
	encKeyBob, err := rpcore.DerivePaymentKey(ssBob)
	if err != nil {
		t.Fatalf("DerivePaymentKey (Bob): %v", err)
	}
	ctxtIIBob, err := rpcore.EncryptPayload(encKeyBob, tokenId, paymentAmt)
	if err != nil {
		t.Fatalf("EncryptPayload (Bob): %v", err)
	}
	saltBobField := rpcore.SaltBToField(saltBobRaw)
	cmtBob, err := rpcore.Erc20CommitmentV2(bobSpendPk, saltBobField, paymentAmt, tokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 (Bob): %v", err)
	}

	saltA, err := dvpcore.RandomInField()
	if err != nil {
		t.Fatalf("RandomInField (change): %v", err)
	}
	cmtChange, err := rpcore.Erc20CommitmentV2(aliceSpend.PublicKey, saltA, changeAmt, tokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 (change): %v", err)
	}

	feeSalt, err := dvpcore.RandomInField()
	if err != nil {
		t.Fatalf("RandomInField (fee): %v", err)
	}
	cmtRelayer, err := rpcore.Erc20CommitmentV2(feeRecipientPk, feeSalt, relayerFeeAmt, tokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 (fee): %v", err)
	}

	var pathElements [8]string
	for j, elem := range aliceProof.Elements {
		if j >= 8 {
			break
		}
		pathElements[j] = elem.String()
	}

	reqBody := map[string]interface{}{
		"stMessage":            "0",
		"stTreeNumbers":        [1]string{"0"},
		"stMerkleRoots":        [1]string{aliceProof.Root.String()},
		"stNullifiers":         [1]string{nullifier.String()},
		"stCommitmentsOut":     [3]string{cmtBob.String(), cmtChange.String(), cmtRelayer.String()},
		"stContractAddress":    vaultAddrBig.String(),
		"stFee":                relayerFeeAmt.String(),
		"wtPrivateKeysIn":      [1]string{aliceSpend.PrivateKey.String()},
		"wtValuesIn":           [1]string{depositAmt.String()},
		"wtSaltsIn":            [1]string{aliceSaltBField.String()},
		"wtPathElements":       [1][8]string{pathElements},
		"wtPathIndices":        [1]string{aliceProof.Indices.String()},
		"wtTokenId":            tokenId.String(),
		"wtSpendPublicKeysOut": [3]string{bobSpendPk.String(), aliceSpend.PublicKey.String(), feeRecipientPk.String()},
		"wtValuesOut":          [3]string{paymentAmt.String(), changeAmt.String(), relayerFeeAmt.String()},
		"wtSaltsOut":           [3]string{saltBobField.String(), saltA.String(), feeSalt.String()},
	}

	proofResp, err := postGnarkProof(gnarkURLRelayerFee+"/proof/paymentRelayerFeePublic", reqBody)
	if err != nil {
		t.Fatalf("postGnarkProof paymentRelayerFeePublic: %v", err)
	}
	if len(proofResp.PublicSignal) != 9 {
		t.Fatalf("expected 9 public signals, got %d", len(proofResp.PublicSignal))
	}

	var proofArr [8]string
	for i, v := range proofResp.Proof {
		proofArr[i] = v.String()
	}
	var sigArr [9]string
	for i, v := range proofResp.PublicSignal {
		sigArr[i] = v.String()
	}

	return relayerFeeWitness{
		proof:        proofArr,
		publicSignal: sigArr,
		ctxtBob:      ctxtBob,
		ctxtIIBob:    ctxtIIBob,
		feeSalt:      feeSalt,
		saltA:        saltA,
		saltBobField: saltBobField,
		tokenId:      tokenId,
		cmtBob:       cmtBob,
		cmtChange:    cmtChange,
		cmtRelayer:   cmtRelayer,
	}
}

const gnarkURLRelayerFee = "http://localhost:8082"

// ── Main happy-path test ──────────────────────────────────────────────────────

func TestFullPaymentRelayerFeeWithTagsViaRelayer(t *testing.T) {
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
	bobAuth := hardhatAuthFromKey(t, bobPrivKeyHex)
	_ = bobAuth

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
		t.Logf("  RELAYER_FEE_SPEND_PRIVATE_KEY=123456789 \\")
		t.Logf("  RELAYER_TAG_CHANNEL_REGISTRY_ADDR=%s \\", chAddr.Hex())
		t.Logf("  RELAYER_TAG_REGISTRY_ADDR=%s \\", trAddr.Hex())
		t.Logf("  go run main.go")
		t.Skip("relayer not running on localhost:8090")
	}

	var info relayInfoResp
	if status := getJSON(t, "/relay/info", &info); status != http.StatusOK {
		t.Fatalf("GET /relay/info: status=%d", status)
	}
	t.Logf("  relayerAddr:    %s", info.RelayerAddr)
	t.Logf("  feeSpendPubKey: %s", info.FeeSpendPubKey)

	zeroAddr := common.Address{}.Hex()
	if info.TagChannelRegistryAddr == zeroAddr || info.TagRegistryAddr == zeroAddr {
		chAddr, _ := tags.DeployTagChannelRegistry(client, aliceAuth, "../contracts/TagChannelRegistry.json")
		trAddr, _ := tags.DeployTagRegistry(client, aliceAuth, "../contracts/TagRegistry.json")
		t.Skipf("Restart relayer with:\n  RELAYER_TAG_CHANNEL_REGISTRY_ADDR=%s\n  RELAYER_TAG_REGISTRY_ADDR=%s",
			chAddr.Hex(), trAddr.Hex())
	}
	if info.FeeSpendPubKey == "" {
		t.Skip("relayer is not configured with RELAYER_FEE_SPEND_PRIVATE_KEY — skipping")
	}
	relayerFeePubKey, ok := new(big.Int).SetString(info.FeeSpendPubKey, 10)
	if !ok {
		t.Fatalf("invalid feeSpendPubKey from /relay/info: %q", info.FeeSpendPubKey)
	}

	channelRegistryAddr := common.HexToAddress(info.TagChannelRegistryAddr)
	tagRegistryAddr := common.HexToAddress(info.TagRegistryAddr)
	relayerEthAddr := common.HexToAddress(info.RelayerAddr)

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

	fixedFee := big.NewInt(rfTagFeeAmt)

	// ═════════════════════════════════════════════════════════════════════════
	// PHASE 0 — owner configures the fixed relayer fee on-chain
	// ═════════════════════════════════════════════════════════════════════════
	t.Logf("── Phase 0: owner sets relayerFixedFeeAmount = %s ──", fixedFee)
	setFeeTx, err := dvp.Transact(aliceAuth, "setRelayerFixedFee", fixedFee)
	if err != nil {
		t.Fatalf("setRelayerFixedFee: %v", err)
	}
	if r, err := bind.WaitMined(ctx, client, setFeeTx); err != nil || r.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("wait setRelayerFixedFee: receipt=%+v err=%v", r, err)
	}

	// ═════════════════════════════════════════════════════════════════════════
	// PHASE 1 — Registration
	// ═════════════════════════════════════════════════════════════════════════
	t.Log("── Phase 1: Registration ──")

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

	if err := rpcore.Register(client, aliceAuth, registryAddr, aliceSpend.PublicKey, aliceView.EncapsKey, make([]byte, 1088), make([]byte, 92)); err != nil {
		if !strings.Contains(err.Error(), "AlreadyRegistered") && !strings.Contains(err.Error(), "45ed80e9") {
			t.Fatalf("Alice Register: %v", err)
		}
	}
	if err := rpcore.Register(client, bobAuth, registryAddr, bobSpend.PublicKey, bobView.EncapsKey, make([]byte, 1088), make([]byte, 92)); err != nil {
		if !strings.Contains(err.Error(), "AlreadyRegistered") && !strings.Contains(err.Error(), "45ed80e9") {
			t.Fatalf("Bob Register: %v", err)
		}
	}
	t.Logf("  Alice pk_spend: %s", aliceSpend.PublicKey)
	t.Logf("  Bob   pk_spend: %s", bobSpend.PublicKey)

	// ═════════════════════════════════════════════════════════════════════════
	// PHASE 2 — Channel Setup via /relay/channel
	// ═════════════════════════════════════════════════════════════════════════
	t.Log("── Phase 2: Channel Setup via /relay/channel ──")

	totalUsers, err := tags.GetUserCount(client, registryAddr)
	if err != nil {
		t.Fatalf("GetUserCount: %v", err)
	}
	bobIdx, err := tags.GetRecipientIndex(client, registryAddr, bobAuth.From)
	if err != nil {
		t.Fatalf("GetRecipientIndex (Bob): %v", err)
	}

	channelSS, c1, c2, bitmap, err := tags.PrepareChannelSetup(
		bobView.EncapsKey,
		[]byte("RelayerFee channel ready"),
		[]byte(aliceAddr),
		tags.PrivacyFull,
		totalUsers,
		bobIdx,
		nil,
	)
	if err != nil {
		t.Fatalf("PrepareChannelSetup: %v", err)
	}

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
	chSender := txSender(t, client, chResp.TxHash)
	t.Logf("  channel tx.from = %s", chSender.Hex())

	bobFound, err := tags.ScanChannels(client, channelRegistryAddr, bobView.DecapsKey, chResp.ChannelIdx, 1)
	if err != nil {
		t.Fatalf("ScanChannels (Bob): %v", err)
	}
	if len(bobFound) != 1 {
		t.Fatalf("Bob expected 1 channel, found %d", len(bobFound))
	}
	bobChannelSS := bobFound[0].SharedSecret
	if new(big.Int).SetBytes(channelSS).Cmp(new(big.Int).SetBytes(bobChannelSS)) != 0 {
		t.Fatal("shared secret mismatch between Alice and Bob")
	}
	t.Log("  shared secrets match ✓")

	// ═════════════════════════════════════════════════════════════════════════
	// PHASE 3+4 — Deposit + ZK PaymentRelayerFeePublic proof
	// ═════════════════════════════════════════════════════════════════════════
	t.Logf("── Phase 3+4: deposit + PaymentRelayerFeePublic proof (pay=%d, change=%d, fee=%d) ──",
		rfTagPaymentAmt, rfTagChangeAmt, rfTagFeeAmt)

	w := depositAndBuildRelayerFeeWitness(t, ctx, client, vault, erc20, vaultAddr, aliceAuth,
		bobSpend.PublicKey, bobView.EncapsKey, fixedFee, relayerFeePubKey)
	t.Log("  proof generated ✓")
	t.Logf("  cmt_bob=%s cmt_relayer=%s StFee=%s", w.cmtBob, w.cmtRelayer, w.publicSignal[8])

	// ═════════════════════════════════════════════════════════════════════════
	// PHASE 5 — Relay via /relay/payment_relayer_fee
	// ═════════════════════════════════════════════════════════════════════════
	t.Log("── Phase 5: Payment via /relay/payment_relayer_fee ──")

	var payResp relayRelayerFeeResp
	status = postJSON(t, "/relay/payment_relayer_fee", relayRelayerFeeReq{
		VaultId:      "0",
		Proof:        w.proof,
		PublicSignal: w.publicSignal,
		CipherText:   toHex(w.ctxtBob),
		EncTxData:    toHex(w.ctxtIIBob),
		FeeSalt:      w.feeSalt.String(),
		TokenId:      w.tokenId.String(),
	}, &payResp)
	if status != http.StatusOK {
		t.Fatalf("POST /relay/payment_relayer_fee: status=%d error=%s", status, payResp.Error)
	}
	t.Logf("  paymentWithRelayerFee mined: block=%d gas=%d tx=%s...",
		payResp.BlockNumber, payResp.GasUsed, payResp.TxHash[:10])

	paySender := txSender(t, client, payResp.TxHash)
	t.Logf("  payment tx.from = %s", paySender.Hex())

	paymentSig := crypto.Keccak256Hash([]byte("Payment(uint256,uint256,bytes,bytes)"))
	nullifierSig := crypto.Keccak256Hash([]byte("Nullifier(uint256,uint256,uint256)"))
	txReceipt, err := client.TransactionReceipt(ctx, common.HexToHash(payResp.TxHash))
	if err != nil {
		t.Fatalf("TransactionReceipt: %v", err)
	}
	var payEvents, nfEvents int
	for _, log := range txReceipt.Logs {
		switch log.Topics[0] {
		case paymentSig:
			payEvents++
		case nullifierSig:
			nfEvents++
		}
	}
	if payEvents != 1 {
		t.Errorf("expected 1 Payment event, got %d", payEvents)
	}
	if nfEvents != 1 {
		t.Errorf("expected 1 Nullifier event, got %d", nfEvents)
	}

	// ═════════════════════════════════════════════════════════════════════════
	// PHASE 6 — Tag Notification via /relay/tag
	// ═════════════════════════════════════════════════════════════════════════
	t.Log("── Phase 6: Tag Notification via /relay/tag (window mode) ──")

	paymentAmt := big.NewInt(rfTagPaymentAmt)
	tokenId := w.tokenId
	startBlock, windowTags, noteCtxt, err := tags.PreparePaymentTag(
		client, 3, bobSpend.PublicKey, channelSS, paymentAmt, tokenId, w.saltBobField,
	)
	if err != nil {
		t.Fatalf("PreparePaymentTag: %v", err)
	}
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
	tagSender := txSender(t, client, tagResp.TxHash)
	t.Logf("  tag tx.from = %s (block %d)", tagSender.Hex(), tagBlock)

	// ═════════════════════════════════════════════════════════════════════════
	// PHASE 7 — Bob Receives via Tag Scan
	// ═════════════════════════════════════════════════════════════════════════
	t.Log("── Phase 7: Bob scans TagRegistry ──")

	bobChannels := []tags.Channel{{SharedSecret: bobChannelSS, PkSpend: bobSpend.PublicKey}}
	cursor := tags.NewScanCursor()
	matches, _, err := tags.ScanBlocksFromCursor(client, tagRegistryAddr, bobChannels, cursor, tagBlock)
	if err != nil {
		t.Fatalf("ScanBlocksFromCursor (Bob): %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("Bob expected 1 matching tag, got %d", len(matches))
	}
	note, err := tags.DecryptPaymentNote(bobChannelSS, matches[0].Entry.Ctxt)
	if err != nil {
		t.Fatalf("DecryptPaymentNote: %v", err)
	}
	if note.Amount.Cmp(paymentAmt) != 0 {
		t.Errorf("amount mismatch: got %s, want %s", note.Amount, paymentAmt)
	}
	bobRecomputedCmt, err := rpcore.Erc20CommitmentV2(bobSpend.PublicKey, note.Salt, note.Amount, note.TokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 (Bob verify): %v", err)
	}
	if bobRecomputedCmt.Cmp(w.cmtBob) != 0 {
		t.Errorf("Bob's recomputed commitment mismatch: got %s, want %s", bobRecomputedCmt, w.cmtBob)
	}
	t.Logf("  Bob found and verified his note via tag scan ✓ (amount=%s)", note.Amount)

	// ═════════════════════════════════════════════════════════════════════════
	// PHASE 8 — Sender Privacy + Fee Verification
	// ═════════════════════════════════════════════════════════════════════════
	t.Log("── Phase 8: Sender Privacy + Fee Verification ──")

	aliceEthAddr := common.HexToAddress(aliceAddr)
	bobEthAddr := common.HexToAddress(bobAddr)
	for _, check := range []struct {
		name   string
		sender common.Address
	}{
		{"channel", chSender},
		{"payment_relayer_fee", paySender},
		{"tag", tagSender},
	} {
		if check.sender == aliceEthAddr || check.sender == bobEthAddr {
			t.Errorf("%s: tx.from == Alice or Bob — sender privacy violated", check.name)
		}
		if check.sender != relayerEthAddr {
			t.Errorf("%s: tx.from=%s is not the relayer %s", check.name, check.sender.Hex(), relayerEthAddr.Hex())
		}
	}
	t.Log("  all 3 txs: tx.from = relayer ✓")

	recomputedFeeCmt, err := rpcore.Erc20CommitmentV2(relayerFeePubKey, w.feeSalt, fixedFee, tokenId)
	if err != nil {
		t.Fatalf("recompute relayer fee commitment: %v", err)
	}
	if recomputedFeeCmt.Cmp(w.cmtRelayer) != 0 {
		t.Errorf("relayer fee commitment mismatch: got %s, want %s", recomputedFeeCmt, w.cmtRelayer)
	}
	if w.publicSignal[8] != fixedFee.String() {
		t.Errorf("public StFee signal = %s, want %s", w.publicSignal[8], fixedFee)
	}
	t.Logf("  relayer fee note (amount=%s) confirmed spendable and publicly verifiable ✓", fixedFee)

	t.Log("")
	t.Log("=== FULL PAYMENT RELAYER FEE WITH TAGS VIA RELAYER COMPLETE ===")
}

// ── Fee enforcement (no tags needed) ───────────────────────────────────────────

// TestPaymentRelayerFeeViaRelayer_InvalidFeeRejected verifies the two enforcement
// layers of the relayer-fee mechanism directly (independent of tag notification):
//   - on-chain: a fee amount that doesn't match relayerFixedFeeAmount reverts
//     with InvalidRelayerFee, even though the note is correctly addressed;
//   - off-chain: a fee note addressed to a non-relayer key is rejected 402 by
//     the relayer before it ever reaches chain.
func TestPaymentRelayerFeeViaRelayer_InvalidFeeRejected(t *testing.T) {
	if !chainAvailable() {
		t.Skip("Hardhat node not running on localhost:8545 — skipping")
	}
	if !gnarkAvailable() {
		t.Skip("gnark server not running on localhost:8082 — skipping")
	}
	if !relayerAvailable() {
		t.Skip("relayer not running on localhost:8090 — skipping")
	}

	var info relayInfoResp
	if status := getJSON(t, "/relay/info", &info); status != http.StatusOK {
		t.Fatalf("GET /relay/info: status=%d", status)
	}
	if info.FeeSpendPubKey == "" {
		t.Skip("relayer is not configured with RELAYER_FEE_SPEND_PRIVATE_KEY — skipping")
	}
	relayerFeePubKey, ok := new(big.Int).SetString(info.FeeSpendPubKey, 10)
	if !ok {
		t.Fatalf("invalid feeSpendPubKey from /relay/info: %q", info.FeeSpendPubKey)
	}

	ctx := context.Background()
	client := mustDial(t)
	defer client.Close()

	aliceAuth := hardhatAuthFromKey(t, alicePrivKeyHex)
	bobAuth := hardhatAuthFromKey(t, bobPrivKeyHex)
	_ = bobAuth

	receipts := loadPaymentReceipts(t)
	vaultAddr := common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress)
	erc20Addr := common.HexToAddress(receipts["ERC20"].ContractAddress)
	dvpAddr := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)

	vaultABI := loadPaymentABI(t, "Erc20CoinVault")
	erc20ABI := loadPaymentABI(t, "RaylsERC20")
	dvpABI := loadPaymentABI(t, "EnygmaDvp")

	vault := bindPayContract(t, client, vaultAddr, vaultABI)
	erc20 := bindPayContract(t, client, erc20Addr, erc20ABI)
	dvp := bindPayContract(t, client, dvpAddr, dvpABI)

	fixedFee := big.NewInt(rfTagFeeAmt)
	setFeeTx, err := dvp.Transact(aliceAuth, "setRelayerFixedFee", fixedFee)
	if err != nil {
		t.Fatalf("setRelayerFixedFee: %v", err)
	}
	if r, err := bind.WaitMined(ctx, client, setFeeTx); err != nil || r.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("wait setRelayerFixedFee: receipt=%+v err=%v", r, err)
	}

	bobSpend, err := rpcore.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("Bob NewSpendKeyPair: %v", err)
	}
	bobView, err := rpcore.NewViewKeyPair()
	if err != nil {
		t.Fatalf("Bob NewViewKeyPair: %v", err)
	}

	t.Run("on-chain: fee mismatches relayerFixedFeeAmount", func(t *testing.T) {
		mismatchedFee := new(big.Int).Add(fixedFee, big.NewInt(1))
		w := depositAndBuildRelayerFeeWitness(t, ctx, client, vault, erc20, vaultAddr, aliceAuth,
			bobSpend.PublicKey, bobView.EncapsKey, mismatchedFee, relayerFeePubKey)

		var resp relayRelayerFeeResp
		status := postJSON(t, "/relay/payment_relayer_fee", relayRelayerFeeReq{
			VaultId:      "0",
			Proof:        w.proof,
			PublicSignal: w.publicSignal,
			CipherText:   toHex(w.ctxtBob),
			EncTxData:    toHex(w.ctxtIIBob),
			FeeSalt:      w.feeSalt.String(),
			TokenId:      w.tokenId.String(),
		}, &resp)

		if status == http.StatusOK {
			t.Fatalf("expected failure (mismatched fee should revert on-chain), got 200: %+v", resp)
		}
		t.Logf("  relayer responded %d: %s", status, resp.Error)
		// Hardhat simulates before accepting a tx, so the revert surfaces as a
		// 500 from dvp.Transact() itself, carrying the exact revert reason.
		if !strings.Contains(resp.Error, "InvalidRelayerFee") {
			t.Errorf("expected the InvalidRelayerFee revert reason, got: %s", resp.Error)
		} else {
			t.Log("  confirmed the specific InvalidRelayerFee revert fired ✓")
		}
	})

	t.Run("off-chain: fee note addressed to a non-relayer key", func(t *testing.T) {
		strangerSpend, err := rpcore.NewSpendKeyPair()
		if err != nil {
			t.Fatalf("stranger NewSpendKeyPair: %v", err)
		}
		w := depositAndBuildRelayerFeeWitness(t, ctx, client, vault, erc20, vaultAddr, aliceAuth,
			bobSpend.PublicKey, bobView.EncapsKey, fixedFee, strangerSpend.PublicKey)

		var resp relayRelayerFeeResp
		status := postJSON(t, "/relay/payment_relayer_fee", relayRelayerFeeReq{
			VaultId:      "0",
			Proof:        w.proof,
			PublicSignal: w.publicSignal,
			CipherText:   toHex(w.ctxtBob),
			EncTxData:    toHex(w.ctxtIIBob),
			FeeSalt:      w.feeSalt.String(),
			TokenId:      w.tokenId.String(),
		}, &resp)

		if status != http.StatusPaymentRequired {
			t.Fatalf("expected 402 (off-chain ownership check), got %d: %+v", status, resp)
		}
		t.Logf("  relayer responded 402 as expected: %s", resp.Error)
	})
}
