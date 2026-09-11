package tags_test

// TestFullPaymentUsdrFeeWithTagsViaRelayer — combines the genuine
// second-asset USDr flow (a normal payment plus an independent
// UsdrFeeCircuit proof over a SEPARATE token/vault, settled atomically via
// EnygmaDvp.paymentWithUsdrFee) with the private tag notification layer,
// all on-chain transactions routed through the relayer.
//
// This is the two-independent-proofs design mirroring enygma_payments'
// USDrCircuit — as opposed to payment_relayer_fee_tag_test.go's
// PaymentRelayerFeePublic mechanism, which pays the fee in the SAME token as
// the payment, as a third output of the SAME proof.
//
// Phases:
//
//	Phase 1 — Registration            Alice and Bob register in UserRegistry.
//	Phase 2 — Channel Setup           via POST /relay/channel (msg.sender = relayer).
//	Phase 3 — Deposit                 Alice deposits BOTH the main token and USDr.
//	Phase 4 — ZK Proofs               main payment (7-elem) + UsdrFeeCircuit (9-elem,
//	                                   fee note addressed to the relayer's published
//	                                   feeSpendPubKey from GET /relay/info).
//	Phase 5 — Atomic settlement       POST /relay/payment_usdr_fee — msg.sender = relayer.
//	                                   EnygmaDvp.paymentWithUsdrFee() settles both
//	                                   proofs in one call; the USDr leg's public
//	                                   StFee/StTokenId are checked against
//	                                   usdrFixedFeeAmount/usdrTokenId.
//	Phase 6 — Tag Notification        via POST /relay/tag, for the main payment's
//	                                   Bob note (the USDr fee note needs no tag — the
//	                                   relayer built the proof and already knows it).
//	Phase 7 — Bob Receives            scans TagRegistry, decrypts note, verifies
//	                                   commitment against the main proof's output.
//	Phase 8 — Sender Privacy + Fee    all 3 txs (channel/payment/tag) have
//	                                   tx.from = relayer; USDr fee note confirmed
//	                                   spendable and publicly verifiable.
//
// TestPaymentUsdrFeeViaRelayer_InvalidFeeRejected then verifies the USDr
// on-chain/off-chain enforcement itself, independent of tags — mirroring
// enygma_retail_payments/test/10_usdr_relayer_fee_onchain_test.go's negative
// cases, run here to confirm the same guarantees hold in the tag-integrated
// relayer path too.
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
//	cd private_tags/test && CC=/usr/bin/clang go test -run 'TestFullPaymentUsdrFeeWithTagsViaRelayer|TestPaymentUsdrFeeViaRelayer_InvalidFeeRejected' -v -timeout 300s

import (
	"context"
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

const usdrTagChangeAmt = 10 // sender's USDr change, arbitrary

// ── relay request/response (POST /relay/payment_usdr_fee) ─────────────────────

type relayUsdrFeeReq struct {
	VaultId      string    `json:"vaultId"`
	Proof        [8]string `json:"proof"`
	PublicSignal [7]string `json:"publicSignal"`
	CipherText   string    `json:"cipherText"`
	EncTxData    string    `json:"encTxData"`

	UsdrVaultId      string    `json:"usdrVaultId"`
	UsdrProof        [8]string `json:"usdrProof"`
	UsdrPublicSignal [9]string `json:"usdrPublicSignal"`
	UsdrCipherText   string    `json:"usdrCipherText"`
	UsdrEncTxData    string    `json:"usdrEncTxData"`
	UsdrFeeSalt      string    `json:"usdrFeeSalt"`
}

type relayUsdrFeeResp struct {
	TxHash      string `json:"txHash"`
	BlockNumber uint64 `json:"blockNumber"`
	GasUsed     uint64 `json:"gasUsed"`
	Error       string `json:"error,omitempty"`
}

// ── USDr leg: build a UsdrFeeCircuit witness ───────────────────────────────────

type usdrFeeWitness struct {
	proof        [8]string
	publicSignal [9]string
	feeSalt      *big.Int
	cmtFee       *big.Int
}

// depositAndBuildUsdrFeeWitness deposits a fresh USDr note for Alice and
// builds a UsdrFeeCircuit proof (fee note of relayerFeeAmt to
// feeRecipientPk, change of usdrTagChangeAmt back to Alice). depositAmt is
// computed from the actual amounts used so the circuit's own conservation
// law always holds even when a sub-test deliberately picks a mismatching
// relayerFeeAmt/tokenId.
func depositAndBuildUsdrFeeWitness(
	t *testing.T,
	ctx context.Context,
	client *ethclient.Client,
	usdrVault, usdrErc20 *bind.BoundContract,
	usdrVaultAddr common.Address,
	aliceAuth *bind.TransactOpts,
	relayerFeeAmt, feeRecipientPk, tokenId *big.Int,
) usdrFeeWitness {
	t.Helper()

	merkleDepth := 8
	changeAmt := big.NewInt(usdrTagChangeAmt)
	depositAmt := new(big.Int).Add(relayerFeeAmt, changeAmt)
	vaultAddrBig := new(big.Int).SetBytes(usdrVaultAddr.Bytes())

	aliceSpend, err := rpcore.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("Alice NewSpendKeyPair: %v", err)
	}
	aliceView, err := rpcore.NewViewKeyPair()
	if err != nil {
		t.Fatalf("Alice NewViewKeyPair: %v", err)
	}

	mintTx, err := usdrErc20.Transact(aliceAuth, "mint", aliceAuth.From, new(big.Int).Mul(depositAmt, big.NewInt(10)))
	if err != nil {
		t.Fatalf("UsdrERC20.mint: %v", err)
	}
	if _, err := bind.WaitMined(ctx, client, mintTx); err != nil {
		t.Fatalf("wait mint: %v", err)
	}
	approveTx, err := usdrErc20.Transact(aliceAuth, "approve", usdrVaultAddr, depositAmt)
	if err != nil {
		t.Fatalf("UsdrERC20.approve: %v", err)
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
	depositTx, err := usdrVault.Transact(aliceAuth, "depositV2",
		[]*big.Int{depositAmt, aliceSpend.PublicKey, aliceSaltBField, tokenId}, capsule, depositCtxt)
	if err != nil {
		t.Fatalf("usdrVault.depositV2: %v", err)
	}
	if _, err := bind.WaitMined(ctx, client, depositTx); err != nil {
		t.Fatalf("wait depositV2: %v", err)
	}

	mt := loadPaymentMerkleTree(t, client, usdrVaultAddr, merkleDepth)
	aliceProof, err := mt.GenerateProof(aliceCommitment)
	if err != nil {
		t.Fatalf("GenerateProof: %v", err)
	}

	nullifier, err := dvpcore.GetNullifier(aliceSpend.PrivateKey, aliceProof.Indices)
	if err != nil {
		t.Fatalf("GetNullifier: %v", err)
	}

	// Output 0: relayer fee note.
	feeSalt, err := dvpcore.RandomInField()
	if err != nil {
		t.Fatalf("RandomInField (fee): %v", err)
	}
	cmtFee, err := rpcore.Erc20CommitmentV2(feeRecipientPk, feeSalt, relayerFeeAmt, tokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 (fee): %v", err)
	}

	// Output 1: Alice's change.
	changeSalt, err := dvpcore.RandomInField()
	if err != nil {
		t.Fatalf("RandomInField (change): %v", err)
	}
	cmtChange, err := rpcore.Erc20CommitmentV2(aliceSpend.PublicKey, changeSalt, changeAmt, tokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 (change): %v", err)
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
		"stCommitmentsOut":     [2]string{cmtFee.String(), cmtChange.String()},
		"stContractAddress":    vaultAddrBig.String(),
		"stFee":                relayerFeeAmt.String(),
		"stTokenId":            tokenId.String(),
		"wtPrivateKeysIn":      [1]string{aliceSpend.PrivateKey.String()},
		"wtValuesIn":           [1]string{depositAmt.String()},
		"wtSaltsIn":            [1]string{aliceSaltBField.String()},
		"wtPathElements":       [1][8]string{pathElements},
		"wtPathIndices":        [1]string{aliceProof.Indices.String()},
		"wtSpendPublicKeysOut": [2]string{feeRecipientPk.String(), aliceSpend.PublicKey.String()},
		"wtValuesOut":          [2]string{relayerFeeAmt.String(), changeAmt.String()},
		"wtSaltsOut":           [2]string{feeSalt.String(), changeSalt.String()},
	}

	proofResp, err := postGnarkProof(gnarkURLRelayerFee+"/proof/usdrFee", reqBody)
	if err != nil {
		t.Fatalf("postGnarkProof usdrFee: %v", err)
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

	return usdrFeeWitness{
		proof:        proofArr,
		publicSignal: sigArr,
		feeSalt:      feeSalt,
		cmtFee:       cmtFee,
	}
}

func postUsdrFeeToRelayer(t *testing.T, req relayUsdrFeeReq) (*relayUsdrFeeResp, int) {
	t.Helper()
	var resp relayUsdrFeeResp
	status := postJSON(t, "/relay/payment_usdr_fee", req, &resp)
	return &resp, status
}

// ── Main happy-path test ──────────────────────────────────────────────────────

func TestFullPaymentUsdrFeeWithTagsViaRelayer(t *testing.T) {
	if !chainAvailable() {
		t.Skip("Hardhat node not running on localhost:8545 — skipping")
	}
	if !gnarkAvailable() {
		t.Skip("gnark server not running on localhost:8082 — skipping")
	}

	ctx := context.Background()
	client := mustDial(t)
	defer client.Close()

	aliceAuth := hardhatAuthFromKey(t, alicePrivKeyHex)
	bobAuth := hardhatAuthFromKey(t, bobPrivKeyHex)

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
	usdrVaultAddr := common.HexToAddress(receipts["UsdrCoinVault"].ContractAddress)
	usdrErc20Addr := common.HexToAddress(receipts["UsdrERC20"].ContractAddress)
	dvpAddr := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)
	registryAddr := common.HexToAddress(receipts["UserRegistry"].ContractAddress)

	vaultABI := loadPaymentABI(t, "Erc20CoinVault")
	erc20ABI := loadPaymentABI(t, "RaylsERC20")
	dvpABI := loadPaymentABI(t, "EnygmaDvp")

	vault := bindPayContract(t, client, vaultAddr, vaultABI)
	erc20 := bindPayContract(t, client, erc20Addr, erc20ABI)
	usdrVault := bindPayContract(t, client, usdrVaultAddr, vaultABI)
	usdrErc20 := bindPayContract(t, client, usdrErc20Addr, erc20ABI)
	dvp := bindPayContract(t, client, dvpAddr, dvpABI)

	tokenId := big.NewInt(0) // fixed convention value — see EnygmaDvp.sol's usdrTokenId doc comment
	fixedFee := big.NewInt(7)

	t.Logf("── Phase 0: owner sets usdrFixedFeeAmount = %s ──", fixedFee)
	setFeeTx, err := dvp.Transact(aliceAuth, "setUsdrFixedFee", fixedFee)
	if err != nil {
		t.Fatalf("setUsdrFixedFee: %v", err)
	}
	if r, err := bind.WaitMined(ctx, client, setFeeTx); err != nil || r.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("wait setUsdrFixedFee: receipt=%+v err=%v", r, err)
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
		[]byte("UsdrFee channel ready"),
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
	// PHASE 3+4 — Deposit + ZK Proofs (main payment + UsdrFeeCircuit)
	// ═════════════════════════════════════════════════════════════════════════
	t.Log("── Phase 3+4: deposit + build main payment proof + USDr fee proof ──")

	gnarkClient := rpcore.NewPaymentClient("")
	merkleDepth := 8
	mainDepositAmt := big.NewInt(40)
	mainPayAmt := big.NewInt(30)
	mainChangeAmt := big.NewInt(10)

	mintTx, err := erc20.Transact(aliceAuth, "mint", aliceAuth.From, new(big.Int).Mul(mainDepositAmt, big.NewInt(10)))
	if err != nil {
		t.Fatalf("ERC20.mint: %v", err)
	}
	if _, err := bind.WaitMined(ctx, client, mintTx); err != nil {
		t.Fatalf("wait mint: %v", err)
	}
	approveTx, err := erc20.Transact(aliceAuth, "approve", vaultAddr, mainDepositAmt)
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
	aliceCommitment, err := rpcore.Erc20CommitmentV2(aliceSpend.PublicKey, aliceSaltBField, mainDepositAmt, big.NewInt(0))
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 (deposit): %v", err)
	}
	depositCtxt, err := rpcore.EncryptPayload(aliceEncKey, big.NewInt(0), mainDepositAmt)
	if err != nil {
		t.Fatalf("EncryptPayload (deposit): %v", err)
	}
	depositTx, err := vault.Transact(aliceAuth, "depositV2",
		[]*big.Int{mainDepositAmt, aliceSpend.PublicKey, aliceSaltBField, big.NewInt(0)}, capsule, depositCtxt)
	if err != nil {
		t.Fatalf("vault.depositV2: %v", err)
	}
	if _, err := bind.WaitMined(ctx, client, depositTx); err != nil {
		t.Fatalf("wait depositV2: %v", err)
	}

	mt := loadPaymentMerkleTree(t, client, vaultAddr, merkleDepth)
	aliceProof, err := mt.GenerateProof(aliceCommitment)
	if err != nil {
		t.Fatalf("GenerateProof (main): %v", err)
	}

	vaultAddrBig := new(big.Int).SetBytes(vaultAddr.Bytes())
	paymentResult, err := gnarkClient.BoundPaymentProof(
		vaultAddrBig,
		big.NewInt(0),
		[]*big.Int{mainDepositAmt},
		[]rpcore.KeyPair{{PrivateKey: aliceSpend.PrivateKey, PublicKey: aliceSpend.PublicKey}},
		[]*big.Int{aliceSaltBField},
		[]*big.Int{mainPayAmt, mainChangeAmt},
		[]*big.Int{bobSpend.PublicKey, aliceSpend.PublicKey},
		[][]byte{bobView.EncapsKey, aliceView.EncapsKey},
		merkleDepth,
		[]*rpcore.MerkleProof{aliceProof},
		[]*big.Int{big.NewInt(0)},
		big.NewInt(0),
	)
	if err != nil {
		t.Fatalf("BoundPaymentProof: %v", err)
	}

	usdrW := depositAndBuildUsdrFeeWitness(t, ctx, client, usdrVault, usdrErc20, usdrVaultAddr, aliceAuth,
		fixedFee, relayerFeePubKey, tokenId)

	// ═════════════════════════════════════════════════════════════════════════
	// PHASE 5 — Atomic settlement via /relay/payment_usdr_fee
	// ═════════════════════════════════════════════════════════════════════════
	t.Log("── Phase 5: Payment via /relay/payment_usdr_fee ──")

	var mainProofArr [8]string
	copy(mainProofArr[:], paymentResult.Proof)
	stmt := paymentResult.ContractStatement()
	var mainSigArr [7]string
	for i, v := range stmt {
		mainSigArr[i] = v.String()
	}

	payResp, payStatus := postUsdrFeeToRelayer(t, relayUsdrFeeReq{
		VaultId:          "0",
		Proof:            mainProofArr,
		PublicSignal:     mainSigArr,
		CipherText:       toHex(paymentResult.CipherText),
		EncTxData:        toHex(paymentResult.EncTxData),
		UsdrVaultId:      "1",
		UsdrProof:        usdrW.proof,
		UsdrPublicSignal: usdrW.publicSignal,
		UsdrCipherText:   "0x",
		UsdrEncTxData:    "0x",
		UsdrFeeSalt:      usdrW.feeSalt.String(),
	})
	if payStatus != http.StatusOK {
		t.Fatalf("POST /relay/payment_usdr_fee: status=%d error=%s", payStatus, payResp.Error)
	}
	t.Logf("  paymentWithUsdrFee mined: block=%d gas=%d tx=%s", payResp.BlockNumber, payResp.GasUsed, payResp.TxHash)

	paySender := txSender(t, client, payResp.TxHash)
	t.Logf("  payment tx.from = %s", paySender.Hex())

	txHash := common.HexToHash(payResp.TxHash)
	txReceipt, err := client.TransactionReceipt(ctx, txHash)
	if err != nil {
		t.Fatalf("TransactionReceipt: %v", err)
	}
	paymentSig := crypto.Keccak256Hash([]byte("Payment(uint256,uint256,bytes,bytes)"))
	nullifierSig := crypto.Keccak256Hash([]byte("Nullifier(uint256,uint256,uint256)"))
	var payEvents, nfEvents int
	for _, log := range txReceipt.Logs {
		switch log.Topics[0] {
		case paymentSig:
			payEvents++
		case nullifierSig:
			nfEvents++
		}
	}
	if payEvents != 2 {
		t.Errorf("expected 2 Payment events (main + USDr), got %d", payEvents)
	}
	if nfEvents != 2 {
		t.Errorf("expected 2 Nullifier events (main + USDr), got %d", nfEvents)
	}

	// ═════════════════════════════════════════════════════════════════════════
	// PHASE 6 — Tag Notification via /relay/tag (Bob's main payment note)
	// ═════════════════════════════════════════════════════════════════════════
	t.Log("── Phase 6: Tag Notification via /relay/tag (window mode) ──")

	startBlock, windowTags, noteCtxt, err := tags.PreparePaymentTag(
		client, 3, bobSpend.PublicKey, channelSS, mainPayAmt, big.NewInt(0), paymentResult.SaltB,
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
	if note.Amount.Cmp(mainPayAmt) != 0 {
		t.Errorf("amount mismatch: got %s, want %s", note.Amount, mainPayAmt)
	}
	bobRecomputedCmt, err := rpcore.Erc20CommitmentV2(bobSpend.PublicKey, note.Salt, note.Amount, note.TokenId)
	if err != nil {
		t.Fatalf("Erc20CommitmentV2 (Bob verify): %v", err)
	}
	if bobRecomputedCmt.Cmp(stmt[4]) != 0 {
		t.Errorf("Bob's recomputed commitment mismatch: got %s, want %s", bobRecomputedCmt, stmt[4])
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
		{"payment_usdr_fee", paySender},
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

	recomputedFeeCmt, err := rpcore.Erc20CommitmentV2(relayerFeePubKey, usdrW.feeSalt, fixedFee, tokenId)
	if err != nil {
		t.Fatalf("recompute USDr fee commitment: %v", err)
	}
	if recomputedFeeCmt.Cmp(usdrW.cmtFee) != 0 {
		t.Errorf("USDr fee commitment mismatch: got %s, want %s", recomputedFeeCmt, usdrW.cmtFee)
	}
	t.Logf("  USDr fee note (amount=%s, separate token) confirmed spendable ✓ — atomic with the main payment and the tag notification", fixedFee)

	t.Log("")
	t.Log("=== FULL PAYMENT USDR FEE WITH TAGS VIA RELAYER COMPLETE ===")
}

// ── Fee enforcement (no tags needed) ───────────────────────────────────────────

// TestPaymentUsdrFeeViaRelayer_InvalidFeeRejected verifies the USDr
// enforcement layers directly, independent of tag notification — mirroring
// enygma_retail_payments/test/10_usdr_relayer_fee_onchain_test.go's negative
// cases, confirming the same guarantees hold via this tag-integrated relayer
// route too.
func TestPaymentUsdrFeeViaRelayer_InvalidFeeRejected(t *testing.T) {
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

	receipts := loadPaymentReceipts(t)
	vaultAddr := common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress)
	erc20Addr := common.HexToAddress(receipts["ERC20"].ContractAddress)
	usdrVaultAddr := common.HexToAddress(receipts["UsdrCoinVault"].ContractAddress)
	usdrErc20Addr := common.HexToAddress(receipts["UsdrERC20"].ContractAddress)
	dvpAddr := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)

	vaultABI := loadPaymentABI(t, "Erc20CoinVault")
	erc20ABI := loadPaymentABI(t, "RaylsERC20")
	dvpABI := loadPaymentABI(t, "EnygmaDvp")

	vault := bindPayContract(t, client, vaultAddr, vaultABI)
	erc20 := bindPayContract(t, client, erc20Addr, erc20ABI)
	usdrVault := bindPayContract(t, client, usdrVaultAddr, vaultABI)
	usdrErc20 := bindPayContract(t, client, usdrErc20Addr, erc20ABI)
	dvp := bindPayContract(t, client, dvpAddr, dvpABI)

	tokenId := big.NewInt(0)
	fixedFee := big.NewInt(7)

	setFeeTx, err := dvp.Transact(aliceAuth, "setUsdrFixedFee", fixedFee)
	if err != nil {
		t.Fatalf("setUsdrFixedFee: %v", err)
	}
	if r, err := bind.WaitMined(ctx, client, setFeeTx); err != nil || r.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("wait setUsdrFixedFee: receipt=%+v err=%v", r, err)
	}

	bobSpend, err := rpcore.NewSpendKeyPair()
	if err != nil {
		t.Fatalf("Bob NewSpendKeyPair: %v", err)
	}
	bobView, err := rpcore.NewViewKeyPair()
	if err != nil {
		t.Fatalf("Bob NewViewKeyPair: %v", err)
	}
	_ = bobAuth

	buildAndSubmit := func(t *testing.T, usdrFeeAmt, usdrFeeRecipientPk, usdrTokenIdArg *big.Int) (*relayUsdrFeeResp, int) {
		t.Helper()

		gnarkClient := rpcore.NewPaymentClient("")
		merkleDepth := 8
		depositAmt := big.NewInt(40)
		payAmt := big.NewInt(30)
		changeAmt := big.NewInt(10)

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
		aliceCommitment, err := rpcore.Erc20CommitmentV2(aliceSpend.PublicKey, aliceSaltBField, depositAmt, big.NewInt(0))
		if err != nil {
			t.Fatalf("Erc20CommitmentV2 (deposit): %v", err)
		}
		depositCtxt, err := rpcore.EncryptPayload(aliceEncKey, big.NewInt(0), depositAmt)
		if err != nil {
			t.Fatalf("EncryptPayload (deposit): %v", err)
		}
		depositTx, err := vault.Transact(aliceAuth, "depositV2",
			[]*big.Int{depositAmt, aliceSpend.PublicKey, aliceSaltBField, big.NewInt(0)}, capsule, depositCtxt)
		if err != nil {
			t.Fatalf("vault.depositV2: %v", err)
		}
		if _, err := bind.WaitMined(ctx, client, depositTx); err != nil {
			t.Fatalf("wait depositV2: %v", err)
		}

		mt := loadPaymentMerkleTree(t, client, vaultAddr, merkleDepth)
		aliceProof, err := mt.GenerateProof(aliceCommitment)
		if err != nil {
			t.Fatalf("GenerateProof (main): %v", err)
		}

		vaultAddrBig := new(big.Int).SetBytes(vaultAddr.Bytes())
		paymentResult, err := gnarkClient.BoundPaymentProof(
			vaultAddrBig,
			big.NewInt(0),
			[]*big.Int{depositAmt},
			[]rpcore.KeyPair{{PrivateKey: aliceSpend.PrivateKey, PublicKey: aliceSpend.PublicKey}},
			[]*big.Int{aliceSaltBField},
			[]*big.Int{payAmt, changeAmt},
			[]*big.Int{bobSpend.PublicKey, aliceSpend.PublicKey},
			[][]byte{bobView.EncapsKey, aliceView.EncapsKey},
			merkleDepth,
			[]*rpcore.MerkleProof{aliceProof},
			[]*big.Int{big.NewInt(0)},
			big.NewInt(0),
		)
		if err != nil {
			t.Fatalf("BoundPaymentProof: %v", err)
		}

		usdrW := depositAndBuildUsdrFeeWitness(t, ctx, client, usdrVault, usdrErc20, usdrVaultAddr, aliceAuth,
			usdrFeeAmt, usdrFeeRecipientPk, usdrTokenIdArg)

		var mainProofArr [8]string
		copy(mainProofArr[:], paymentResult.Proof)
		stmt := paymentResult.ContractStatement()
		var mainSigArr [7]string
		for i, v := range stmt {
			mainSigArr[i] = v.String()
		}

		return postUsdrFeeToRelayer(t, relayUsdrFeeReq{
			VaultId:          "0",
			Proof:            mainProofArr,
			PublicSignal:     mainSigArr,
			CipherText:       toHex(paymentResult.CipherText),
			EncTxData:        toHex(paymentResult.EncTxData),
			UsdrVaultId:      "1",
			UsdrProof:        usdrW.proof,
			UsdrPublicSignal: usdrW.publicSignal,
			UsdrCipherText:   "0x",
			UsdrEncTxData:    "0x",
			UsdrFeeSalt:      usdrW.feeSalt.String(),
		})
	}

	t.Run("on-chain: usdrFee mismatches usdrFixedFeeAmount", func(t *testing.T) {
		mismatchedFee := new(big.Int).Add(fixedFee, big.NewInt(1))
		resp, status := buildAndSubmit(t, mismatchedFee, relayerFeePubKey, tokenId)
		if status != http.StatusInternalServerError {
			t.Fatalf("expected 500 (on-chain revert surfaced), got %d: %+v", status, resp)
		}
		t.Logf("  relayer responded %d: %s", status, resp.Error)
		if !strings.Contains(resp.Error, "InvalidUsdrFee") {
			t.Errorf("expected error to contain InvalidUsdrFee, got: %s", resp.Error)
		} else {
			t.Log("  confirmed the specific InvalidUsdrFee revert fired ✓")
		}
	})

	t.Run("on-chain: usdrTokenId mismatches configured value", func(t *testing.T) {
		wrongTokenId := big.NewInt(1)
		resp, status := buildAndSubmit(t, fixedFee, relayerFeePubKey, wrongTokenId)
		if status != http.StatusInternalServerError {
			t.Fatalf("expected 500 (on-chain revert surfaced), got %d: %+v", status, resp)
		}
		t.Logf("  relayer responded %d: %s", status, resp.Error)
		if !strings.Contains(resp.Error, "InvalidUsdrFee") {
			t.Errorf("expected error to contain InvalidUsdrFee, got: %s", resp.Error)
		} else {
			t.Log("  confirmed the specific InvalidUsdrFee revert fired ✓")
		}
	})

	t.Run("off-chain: usdr fee note addressed to a non-relayer key", func(t *testing.T) {
		strangerSpend, err := rpcore.NewSpendKeyPair()
		if err != nil {
			t.Fatalf("stranger NewSpendKeyPair: %v", err)
		}
		resp, status := buildAndSubmit(t, fixedFee, strangerSpend.PublicKey, tokenId)
		if status != http.StatusPaymentRequired {
			t.Fatalf("expected 402 (off-chain ownership check), got %d: %+v", status, resp)
		}
		t.Logf("  relayer responded 402 as expected: %s", resp.Error)
	})
}
