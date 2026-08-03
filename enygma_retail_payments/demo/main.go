// Enygma Retail Payments · Live Demo
// https://claude.ai/code/artifact/c1d25015-0c70-4df5-8d01-5dc31d9b3a17
// A local HTTP server that runs the full retail payment protocol flow
// and streams each step as a Server-Sent Event to a live dashboard.
//
// Two modes are supported:
//
//	plain — Alice → relayer → Bob (no private tag notification)
//	tag   — adds ML-KEM channel setup and tag notification so Bob discovers
//	        the payment via TagRegistry without polling the vault directly.
//
// Usage:
//
//	cd demo && bash run.sh
//	open http://localhost:9091
//
// Prerequisites (all must be running before clicking Run):
//
//	1. Hardhat node  :  cd ../enygma_dvp && npx hardhat node
//	2. Deploy+init   :  bash setup.sh    (from enygma_retail_payments/)
//	3. Gnark server  :  cd gnark_circuits && go run main.go
//	4. Relayer       :  cd relayer && RELAYER_PRIVATE_KEY=... RELAYER_API_KEY=test-api-key-dev-only go run main.go
package main

import (
	_ "embed"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	dvpcore "github.com/raylsnetwork/enygma_dvp/src/core"
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"
	tags "github.com/raylsnetwork/enygma_retail_payments/private_tags/src"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

//go:embed index.html
var indexHTML string

// ── Config ─────────────────────────────────────────────────────────────────────

const (
	listenAddr    = ":9091"
	relayerURL    = "http://localhost:8090"
	relayerAPIKey = "test-api-key-dev-only"

	// Hardhat account[0] — Alice (deposits into the vault)
	alicePrivKeyHex = "34d091c661db4c814d65c8ae9277b7055c0dde5a752ce5a3fdfd4ea11a8f7154"
	// Hardhat account[1] — Bob (receives payment)
	bobPrivKeyHex = "69b5623bd1cfe22983c8849d155ca641238c18ab1b2e34c5ae943ed2ce4716b7"

	depositAmt  = int64(40)
	paymentAmt  = int64(30)
	changeAmt   = int64(10)
	merkleDepth = 8
)

var (
	rpcURL  = getEnv("RPC_URL", "http://127.0.0.1:8545")
	chainID = getEnvInt64("CHAIN_ID", 1337)
)

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt64(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

// pause adds a short delay between steps so dashboard animations are readable.
// On non-Hardhat chains the natural block time provides pacing.
func pause(d time.Duration) {
	if chainID == 1337 {
		time.Sleep(d)
	}
}

// ── SSE Broker ─────────────────────────────────────────────────────────────────

type Event struct {
	Type   string `json:"type"`
	Step   string `json:"step,omitempty"`
	Status string `json:"status,omitempty"`
	Label  string `json:"label,omitempty"`
	Msg    string `json:"msg,omitempty"`
	TS     string `json:"ts,omitempty"`
	Cat    string `json:"cat,omitempty"` // log category: key|chain|zk|tag
	// Participant panel
	Pid   int    `json:"pid,omitempty"`
	Field string `json:"field,omitempty"`
	Value string `json:"value,omitempty"`
}

type Broker struct {
	mu      sync.Mutex
	clients map[chan string]struct{}
}

func newBroker() *Broker { return &Broker{clients: make(map[chan string]struct{})} }

func (b *Broker) subscribe() chan string {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan string, 256)
	b.clients[ch] = struct{}{}
	return ch
}

func (b *Broker) unsubscribe(ch chan string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.clients, ch)
}

func (b *Broker) publish(e Event) {
	data, _ := json.Marshal(e)
	line := "data: " + string(data) + "\n\n"
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.clients {
		select {
		case ch <- line:
		default:
		}
	}
}

// ── Server ────────────────────────────────────────────────────────────────────

type Server struct {
	broker  *Broker
	runLock sync.Mutex
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, indexHTML)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := s.broker.subscribe()
	defer s.broker.unsubscribe(ch)

	tick := time.NewTicker(20 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
			fmt.Fprint(w, ": ping\n\n")
			fl.Flush()
		case msg, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprint(w, msg)
			fl.Flush()
		}
	}
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	if !s.runLock.TryLock() {
		http.Error(w, `{"error":"flow already running"}`, 409)
		return
	}
	mode := r.URL.Query().Get("mode")
	withTag := mode == "tag"
	go func() {
		defer s.runLock.Unlock()
		runFlow(s.broker, withTag)
	}()
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"status":"started"}`)
}

// ── Relay API types ────────────────────────────────────────────────────────────

type relayInfoResponse struct {
	RelayerAddr            string `json:"relayerAddr"`
	TagRegistryAddr        string `json:"tagRegistryAddr"`
	TagChannelRegistryAddr string `json:"tagChannelRegistryAddr"`
	ChainID                int64  `json:"chainId"`
}

type relayPaymentRequest struct {
	VaultId      string    `json:"vaultId"`
	Proof        [8]string `json:"proof"`
	PublicSignal [7]string `json:"publicSignal"`
	CipherText   string    `json:"cipherText"`
	EncTxData    string    `json:"encTxData"`
}

type relayPaymentResponse struct {
	TxHash      string `json:"txHash"`
	BlockNumber uint64 `json:"blockNumber"`
	GasUsed     uint64 `json:"gasUsed"`
	Error       string `json:"error,omitempty"`
}

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

type relayTagRequest struct {
	Tags       []string `json:"tags,omitempty"`
	StartBlock uint64   `json:"startBlock,omitempty"`
	Ctxt       string   `json:"ctxt"`
}

type relayTagResponse struct {
	TxHash      string `json:"txHash"`
	BlockNumber uint64 `json:"blockNumber"`
	GasUsed     uint64 `json:"gasUsed"`
	Error       string `json:"error,omitempty"`
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func tcpAvailable(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

type receiptEntry struct {
	ContractAddress string `json:"contractAddress"`
}

func loadReceipts(srcDir string) (map[string]receiptEntry, error) {
	path := filepath.Join(srcDir, "..", "build", "receipts.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read receipts.json: %w (run deploy+init first)", err)
	}
	var r map[string]receiptEntry
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parse receipts.json: %w", err)
	}
	return r, nil
}

func loadContractABI(path string) (abi.ABI, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return abi.ABI{}, fmt.Errorf("read ABI %s: %w", path, err)
	}
	var artifact struct {
		ABI json.RawMessage `json:"abi"`
	}
	if err := json.Unmarshal(data, &artifact); err != nil {
		return abi.ABI{}, fmt.Errorf("parse artifact JSON: %w", err)
	}
	return abi.JSON(strings.NewReader(string(artifact.ABI)))
}

func makeAuth(privHex string) (*bind.TransactOpts, error) {
	key, err := crypto.HexToECDSA(privHex)
	if err != nil {
		return nil, fmt.Errorf("HexToECDSA: %w", err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(chainID))
	if err != nil {
		return nil, fmt.Errorf("NewKeyedTransactorWithChainID: %w", err)
	}
	auth.GasLimit = 8_000_000
	return auth, nil
}

func waitMined(ctx context.Context, client *ethclient.Client, tx *ethtypes.Transaction) (*ethtypes.Receipt, error) {
	receipt, err := bind.WaitMined(ctx, client, tx)
	if err != nil {
		return nil, err
	}
	if receipt.Status != ethtypes.ReceiptStatusSuccessful {
		return nil, fmt.Errorf("tx reverted (block %d)", receipt.BlockNumber)
	}
	return receipt, nil
}

func postRelayer(path string, body interface{}, result interface{}) (int, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequest(http.MethodPost, relayerURL+path, bytes.NewReader(data))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+relayerAPIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("POST %s: %w", relayerURL+path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if result != nil {
		_ = json.Unmarshal(raw, result)
	}
	return resp.StatusCode, nil
}

func buildRelayPaymentRequest(result *dvpcore.PaymentResult) relayPaymentRequest {
	var proof [8]string
	copy(proof[:], result.Proof)
	stmt := result.ContractStatement()
	var sig [7]string
	for i, v := range stmt {
		sig[i] = v.String()
	}
	return relayPaymentRequest{
		VaultId:      "0",
		Proof:        proof,
		PublicSignal: sig,
		CipherText:   "0x" + hex.EncodeToString(result.CipherText),
		EncTxData:    "0x" + hex.EncodeToString(result.EncTxData),
	}
}

func buildVaultMerkleTree(ctx context.Context, client *ethclient.Client, vaultAddr common.Address) (*rpcore.MerkleTree, error) {
	commitmentSig := crypto.Keccak256Hash([]byte("Commitment(uint256,uint256)"))
	query := ethereum.FilterQuery{
		Addresses: []common.Address{vaultAddr},
		Topics:    [][]common.Hash{{commitmentSig}},
	}
	logs, err := client.FilterLogs(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("FilterLogs (vault Commitment): %w", err)
	}
	mt := rpcore.NewMerkleTree(merkleDepth)
	for _, l := range logs {
		if len(l.Topics) >= 3 {
			mt.InsertLeaf(l.Topics[2].Big())
		}
	}
	return mt, nil
}

func toHex(b []byte) string { return "0x" + hex.EncodeToString(b) }

// shortHex returns the first n chars of a hex string followed by "…".
func shortHex(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// formatNum formats a uint64 with comma separators, e.g. 243018 → "243,018".
func formatNum(n uint64) string {
	s := strconv.FormatUint(n, 10)
	out := make([]byte, 0, len(s)+len(s)/3)
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(c))
	}
	return string(out)
}

// ── Main flow ─────────────────────────────────────────────────────────────────

func runFlow(b *Broker, withTag bool) {
	b.publish(Event{Type: "reset"})
	time.Sleep(80 * time.Millisecond)

	ctx := context.Background()

	// Convenience closures over b.
	emit := func(step, status, label, msg string) {
		b.publish(Event{Type: "step", Step: step, Status: status, Label: label, Msg: msg})
	}
	logMsg := func(cat, msg string) {
		b.publish(Event{Type: "log", Cat: cat, Msg: msg, TS: time.Now().Format("15:04:05.000")})
	}
	panel := func(pid int, field, value string) {
		b.publish(Event{Type: "participant", Pid: pid, Field: field, Value: value})
	}
	proto := func(field, value string) {
		b.publish(Event{Type: "proto", Field: field, Value: value})
	}
	fail := func(step, label string, err error) {
		emit(step, "error", label, err.Error())
		b.publish(Event{Type: "done", Status: "error", Msg: err.Error()})
	}

	// Metric accumulators — emitted as proto events at flow completion.
	var totalGasUsed uint64
	var proofGenMs, relayRTTms int64
	flowStart := time.Now()

	// Resolve the source directory for reading local files.
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Dir(thisFile)

	// ── Step: Prerequisites ───────────────────────────────────────────────────
	emit("prereqs", "running", "Check prerequisites", "Probing chain · gnark server · relayer…")
	pause(1400 * time.Millisecond)

	chainOK   := tcpAvailable("127.0.0.1:8545")
	gnarkOK   := tcpAvailable("localhost:8082")
	relayerOK := tcpAvailable("localhost:8090")

	var missing []string
	if !chainOK   { missing = append(missing, "Hardhat (8545)") }
	if !gnarkOK   { missing = append(missing, "gnark server (8082)") }
	if !relayerOK { missing = append(missing, "relayer (8090)") }
	if len(missing) > 0 {
		fail("prereqs", "Check prerequisites", fmt.Errorf("services not running: %s", strings.Join(missing, ", ")))
		return
	}
	logMsg("", "✓ chain · gnark server · relayer are all reachable")

	// Always fetch /relay/info to get relayer address (and tag addrs in with-tag mode).
	var tagRegistryAddr, channelRegistryAddr common.Address
	{
		resp, err := http.Get(relayerURL + "/relay/info")
		if err == nil {
			var info relayInfoResponse
			json.NewDecoder(resp.Body).Decode(&info)
			resp.Body.Close()
			if info.RelayerAddr != "" {
				proto("relayerAddr", info.RelayerAddr)
				logMsg("chain", fmt.Sprintf("relayer: %s", shortHex(info.RelayerAddr, 18)))
			}
			if withTag {
				zeroAddr := common.Address{}.Hex()
				if info.TagRegistryAddr == zeroAddr || info.TagChannelRegistryAddr == zeroAddr {
					fail("prereqs", "Check prerequisites", fmt.Errorf(
						"relayer TagRegistry/TagChannelRegistry not configured — restart with RELAYER_TAG_REGISTRY_ADDR and RELAYER_TAG_CHANNEL_REGISTRY_ADDR"))
					return
				}
				tagRegistryAddr     = common.HexToAddress(info.TagRegistryAddr)
				channelRegistryAddr = common.HexToAddress(info.TagChannelRegistryAddr)
				proto("tagRegistryAddr", info.TagRegistryAddr)
				proto("tagChannelAddr", info.TagChannelRegistryAddr)
				logMsg("chain", fmt.Sprintf("TagRegistry: %s", shortHex(info.TagRegistryAddr, 18)))
				logMsg("chain", fmt.Sprintf("TagChannelRegistry: %s", shortHex(info.TagChannelRegistryAddr, 18)))
			}
		} else if withTag {
			fail("prereqs", "Check prerequisites", fmt.Errorf("GET /relay/info: %w", err))
			return
		}
	}

	emit("prereqs", "success", "Check prerequisites", "All services running")
	pause(2000 * time.Millisecond)

	// ── Load contract addresses ────────────────────────────────────────────────
	receipts, err := loadReceipts(dir)
	if err != nil {
		fail("prereqs", "Check prerequisites", err)
		return
	}
	vaultAddr    := common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress)
	erc20Addr    := common.HexToAddress(receipts["ERC20"].ContractAddress)
	registryAddr := common.HexToAddress(receipts["UserRegistry"].ContractAddress)
	proto("vaultAddr", vaultAddr.Hex())
	proto("registryAddr", registryAddr.Hex())
	logMsg("chain", fmt.Sprintf("Erc20CoinVault: %s", shortHex(vaultAddr.Hex(), 18)))
	logMsg("chain", fmt.Sprintf("UserRegistry:   %s", shortHex(registryAddr.Hex(), 18)))

	// ── Connect to chain ──────────────────────────────────────────────────────
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		fail("prereqs", "Check prerequisites", fmt.Errorf("ethclient.Dial: %w", err))
		return
	}
	defer client.Close()

	// ── Load ABIs ─────────────────────────────────────────────────────────────
	vaultABI, err := loadContractABI(filepath.Join(dir, "..", "contracts", "abis", "Erc20CoinVault.json"))
	if err != nil {
		fail("prereqs", "Check prerequisites", err)
		return
	}
	erc20ABI, err := loadContractABI(filepath.Join(dir, "..", "contracts", "abis", "RaylsERC20.json"))
	if err != nil {
		fail("prereqs", "Check prerequisites", err)
		return
	}

	vault := bind.NewBoundContract(vaultAddr, vaultABI, client, client, client)
	erc20 := bind.NewBoundContract(erc20Addr, erc20ABI, client, client, client)

	// ── Build authenticators ──────────────────────────────────────────────────
	aliceAuth, err := makeAuth(alicePrivKeyHex)
	if err != nil {
		fail("prereqs", "Check prerequisites", err)
		return
	}
	bobAuth, err := makeAuth(bobPrivKeyHex)
	if err != nil {
		fail("prereqs", "Check prerequisites", err)
		return
	}
	panel(0, "ethAddr", aliceAuth.From.Hex())
	panel(1, "ethAddr", bobAuth.From.Hex())

	gnarkClient := rpcore.NewPaymentClient("")
	tokenId     := big.NewInt(0)

	// ── Step: Key generation ──────────────────────────────────────────────────
	emit("keygen", "running", "Generate key pairs", "Alice & Bob each generate spend+view keypairs…")
	pause(2000 * time.Millisecond)

	aliceSpend, err := rpcore.NewSpendKeyPair()
	if err != nil { fail("keygen", "Generate key pairs", err); return }
	aliceView, err  := rpcore.NewViewKeyPair()
	if err != nil { fail("keygen", "Generate key pairs", err); return }
	bobSpend, err   := rpcore.NewSpendKeyPair()
	if err != nil { fail("keygen", "Generate key pairs", err); return }
	bobView, err    := rpcore.NewViewKeyPair()
	if err != nil { fail("keygen", "Generate key pairs", err); return }

	panel(0, "pkSpend", "0x"+aliceSpend.PublicKey.Text(16))
	panel(1, "pkSpend", "0x"+bobSpend.PublicKey.Text(16))
	panel(0, "pkView", "0x"+hex.EncodeToString(aliceView.EncapsKey))
	panel(1, "pkView", "0x"+hex.EncodeToString(bobView.EncapsKey))
	logMsg("key", fmt.Sprintf("Alice pk_spend: 0x%s…", aliceSpend.PublicKey.Text(16)[:16]))
	logMsg("key", fmt.Sprintf("Bob   pk_spend: 0x%s…", bobSpend.PublicKey.Text(16)[:16]))
	logMsg("key", fmt.Sprintf("Alice ML-KEM-768 ek: 0x%s…", hex.EncodeToString(aliceView.EncapsKey)[:16]))
	logMsg("key", fmt.Sprintf("Bob   ML-KEM-768 ek: 0x%s…", hex.EncodeToString(bobView.EncapsKey)[:16]))
	emit("keygen", "success", "Generate key pairs", "BabyJubJub spend keys + ML-KEM-768 view keypairs generated")
	pause(2000 * time.Millisecond)

	// ── Step: Registration ────────────────────────────────────────────────────
	emit("register", "running", "Register on-chain", "Publishing keys to UserRegistry…")

	// Use empty audit ciphertexts (demo doesn't exercise the auditor path).
	stubMlKemCt := make([]byte, 1088)
	stubAesCt   := make([]byte, 92)

	if err := rpcore.Register(client, aliceAuth, registryAddr,
		aliceSpend.PublicKey, aliceView.EncapsKey, stubMlKemCt, stubAesCt); err != nil {
		if !strings.Contains(err.Error(), "AlreadyRegistered") && !strings.Contains(err.Error(), "45ed80e9") {
			fail("register", "Register on-chain", fmt.Errorf("Alice Register: %w", err))
			return
		}
		logMsg("chain", "Alice already registered in UserRegistry — skipping")
	} else {
		logMsg("chain", fmt.Sprintf("UserRegistry.register(Alice) → tx mined"))
	}
	panel(0, "status", "Registered")

	if err := rpcore.Register(client, bobAuth, registryAddr,
		bobSpend.PublicKey, bobView.EncapsKey, stubMlKemCt, stubAesCt); err != nil {
		if !strings.Contains(err.Error(), "AlreadyRegistered") && !strings.Contains(err.Error(), "45ed80e9") {
			fail("register", "Register on-chain", fmt.Errorf("Bob Register: %w", err))
			return
		}
		logMsg("chain", "Bob already registered in UserRegistry — skipping")
	} else {
		logMsg("chain", fmt.Sprintf("UserRegistry.register(Bob) → tx mined"))
	}
	panel(1, "status", "Registered")
	emit("register", "success", "Register on-chain", "Alice & Bob public keys committed to UserRegistry")
	pause(2000 * time.Millisecond)

	// ── Step: Channel setup (with-tag mode only) ──────────────────────────────
	var channelSS []byte
	if withTag {
		emit("channel", "running", "Channel Setup", "Alice establishes ML-KEM channel with Bob via relayer…")

		totalUsers, err := tags.GetUserCount(client, registryAddr)
		if err != nil { fail("channel", "Channel Setup", fmt.Errorf("GetUserCount: %w", err)); return }
		bobIdx, err := tags.GetRecipientIndex(client, registryAddr, bobAuth.From)
		if err != nil { fail("channel", "Channel Setup", fmt.Errorf("GetRecipientIndex: %w", err)); return }
		logMsg("tag", fmt.Sprintf("UserRegistry: %d users registered, Bob index: %d", totalUsers, bobIdx))

		var c1, c2, bitmap []byte
		channelSS, c1, c2, bitmap, err = tags.PrepareChannelSetup(
			bobView.EncapsKey,
			[]byte("Payment channel ready"),
			[]byte(aliceAuth.From.Hex()),
			tags.PrivacyFull,
			totalUsers, bobIdx, nil,
		)
		if err != nil { fail("channel", "Channel Setup", fmt.Errorf("PrepareChannelSetup: %w", err)); return }
		logMsg("tag", fmt.Sprintf("ML-KEM-768 encapsulation → c1=%dB  c2=%dB  bitmap=%dB (%s)", len(c1), len(c2), len(bitmap), toHex(bitmap)))

		var chResp relayChannelResponse
		status, err := postRelayer("/relay/channel", relayChannelRequest{
			C1:     toHex(c1),
			C2:     toHex(c2),
			Bitmap: toHex(bitmap),
		}, &chResp)
		if err != nil { fail("channel", "Channel Setup", err); return }
		if status != http.StatusOK {
			fail("channel", "Channel Setup", fmt.Errorf("POST /relay/channel %d: %s", status, chResp.Error))
			return
		}
		totalGasUsed += chResp.GasUsed
		proto("chTxHash", chResp.TxHash)
		proto("chBlock", fmt.Sprintf("%d", chResp.BlockNumber))
		proto("chGas", fmt.Sprintf("%d", chResp.GasUsed))
		logMsg("chain", fmt.Sprintf("TagChannelRegistry.openChannel() → block %d  gas %s  tx %s…",
			chResp.BlockNumber, formatNum(chResp.GasUsed), shortHex(chResp.TxHash, 18)))

		// Bob scans to recover the shared secret.
		bobFound, err := tags.ScanChannels(client, channelRegistryAddr, bobView.DecapsKey, chResp.ChannelIdx, 1)
		if err != nil { fail("channel", "Channel Setup", fmt.Errorf("ScanChannels (Bob): %w", err)); return }
		if len(bobFound) != 1 {
			fail("channel", "Channel Setup", fmt.Errorf("Bob found %d channels, want 1", len(bobFound)))
			return
		}
		logMsg("tag", fmt.Sprintf("Bob decapsulated shared secret: 0x%x…", bobFound[0].SharedSecret[:8]))
		panel(1, "channel", fmt.Sprintf("Channel #%d established", chResp.ChannelIdx))
		emit("channel", "success", "Channel Setup", fmt.Sprintf("ML-KEM shared secret established (idx=%d, block=%d)", chResp.ChannelIdx, chResp.BlockNumber))
		pause(2000 * time.Millisecond)
	}

	// ── Step: Deposit ─────────────────────────────────────────────────────────
	emit("deposit", "running", "Deposit ERC-20", fmt.Sprintf("Alice mints and deposits %d tokens into the private vault…", depositAmt))

	// Mint tokens to Alice.
	mintTx, err := erc20.Transact(aliceAuth, "mint", aliceAuth.From, big.NewInt(depositAmt*10))
	if err != nil { fail("deposit", "Deposit ERC-20", fmt.Errorf("ERC20.mint: %w", err)); return }
	if _, err := waitMined(ctx, client, mintTx); err != nil {
		fail("deposit", "Deposit ERC-20", fmt.Errorf("wait mint: %w", err)); return
	}
	logMsg("chain", fmt.Sprintf("ERC20.mint(%d tokens) → Alice", depositAmt*10))

	approveTx, err := erc20.Transact(aliceAuth, "approve", vaultAddr, big.NewInt(depositAmt))
	if err != nil { fail("deposit", "Deposit ERC-20", fmt.Errorf("ERC20.approve: %w", err)); return }
	if _, err := waitMined(ctx, client, approveTx); err != nil {
		fail("deposit", "Deposit ERC-20", fmt.Errorf("wait approve: %w", err)); return
	}

	// Encapsulate to derive deposit salt and encryption key.
	ss, capsule, err := rpcore.Encapsulate(aliceView.EncapsKey)
	if err != nil { fail("deposit", "Deposit ERC-20", fmt.Errorf("Encapsulate: %w", err)); return }
	aliceSaltB, err := rpcore.DerivePaymentSalt(ss)
	if err != nil { fail("deposit", "Deposit ERC-20", fmt.Errorf("DerivePaymentSalt: %w", err)); return }
	aliceEncKey, err := rpcore.DerivePaymentKey(ss)
	if err != nil { fail("deposit", "Deposit ERC-20", fmt.Errorf("DerivePaymentKey: %w", err)); return }
	aliceSaltBField := rpcore.SaltBToField(aliceSaltB)

	aliceCommitment, err := rpcore.Erc20CommitmentV2(aliceSpend.PublicKey, aliceSaltBField, big.NewInt(depositAmt), tokenId)
	if err != nil { fail("deposit", "Deposit ERC-20", fmt.Errorf("Erc20CommitmentV2: %w", err)); return }

	depositCtxt, err := rpcore.EncryptPayload(aliceEncKey, tokenId, big.NewInt(depositAmt))
	if err != nil { fail("deposit", "Deposit ERC-20", fmt.Errorf("EncryptPayload: %w", err)); return }

	depositTx, err := vault.Transact(aliceAuth, "depositV2",
		[]*big.Int{big.NewInt(depositAmt), aliceSpend.PublicKey, aliceSaltBField, tokenId},
		capsule, depositCtxt)
	if err != nil { fail("deposit", "Deposit ERC-20", fmt.Errorf("depositV2: %w", err)); return }
	depositReceipt, err := waitMined(ctx, client, depositTx)
	if err != nil { fail("deposit", "Deposit ERC-20", fmt.Errorf("wait depositV2: %w", err)); return }

	totalGasUsed += depositReceipt.GasUsed
	logMsg("chain", fmt.Sprintf("Erc20CoinVault.depositV2() → block %d  gas %s",
		depositReceipt.BlockNumber, formatNum(depositReceipt.GasUsed)))
	logMsg("key", fmt.Sprintf("Poseidon commitment: 0x%s…", aliceCommitment.Text(16)[:16]))
	proto("depositBlock", fmt.Sprintf("%d", depositReceipt.BlockNumber))
	proto("depositGas", fmt.Sprintf("%d", depositReceipt.GasUsed))
	panel(0, "balance", fmt.Sprintf("%d tokens (deposited)", depositAmt))
	panel(0, "commitment", "0x"+aliceCommitment.Text(16))
	emit("deposit", "success", "Deposit ERC-20", fmt.Sprintf("Alice deposited %d tokens — Poseidon commitment recorded on-chain (block %d)", depositAmt, depositReceipt.BlockNumber))
	pause(2000 * time.Millisecond)

	// ── Step: Merkle proof ────────────────────────────────────────────────────
	emit("merkle", "running", "Build Merkle Proof", "Scanning vault Commitment events to reconstruct the Merkle tree…")

	mt, err := buildVaultMerkleTree(ctx, client, vaultAddr)
	if err != nil { fail("merkle", "Build Merkle Proof", err); return }
	aliceProof, err := mt.GenerateProof(aliceCommitment)
	if err != nil { fail("merkle", "Build Merkle Proof", fmt.Errorf("GenerateProof: %w", err)); return }

	logMsg("key", fmt.Sprintf("Merkle root: 0x%s…", aliceProof.Root.Text(16)[:16]))
	panel(0, "merkleRoot", "0x"+aliceProof.Root.Text(16))
	emit("merkle", "success", "Build Merkle Proof", fmt.Sprintf("Merkle inclusion proof for Alice's deposit (depth=%d, root=0x%s…)", merkleDepth, aliceProof.Root.Text(16)[:12]))
	pause(2000 * time.Millisecond)

	// ── Step: ZK proof ────────────────────────────────────────────────────────
	emit("zkproof", "running", "Generate ZK Proof",
		fmt.Sprintf("Gnark server: proving pay=%d to Bob, change=%d to Alice…", paymentAmt, changeAmt))

	vaultAddrBig := new(big.Int).SetBytes(vaultAddr.Bytes())
	proofGenStart := time.Now()
	paymentResult, err := gnarkClient.BoundPaymentProof(
		vaultAddrBig,
		big.NewInt(0),
		[]*big.Int{big.NewInt(depositAmt)},
		[]rpcore.KeyPair{{PrivateKey: aliceSpend.PrivateKey, PublicKey: aliceSpend.PublicKey}},
		[]*big.Int{aliceSaltBField},
		[]*big.Int{big.NewInt(paymentAmt), big.NewInt(changeAmt)},
		[]*big.Int{bobSpend.PublicKey, aliceSpend.PublicKey},
		[][]byte{bobView.EncapsKey, aliceView.EncapsKey},
		merkleDepth,
		[]*rpcore.MerkleProof{aliceProof},
		[]*big.Int{big.NewInt(0)},
		tokenId,
	)
	proofGenMs = time.Since(proofGenStart).Milliseconds()
	if err != nil { fail("zkproof", "Generate ZK Proof", fmt.Errorf("BoundPaymentProof: %w", err)); return }

	stmt := paymentResult.ContractStatement()
	logMsg("zk", fmt.Sprintf("proof generation: %d ms  size: 256 bytes (Groth16: 8 × 32-byte field elements)", proofGenMs))
	logMsg("zk", fmt.Sprintf("nullifier: 0x%s…", stmt[3].Text(16)[:16]))
	logMsg("zk", fmt.Sprintf("Bob commitment:   0x%s…", stmt[4].Text(16)[:16]))
	logMsg("zk", fmt.Sprintf("Alice change cmt: 0x%s…", stmt[5].Text(16)[:16]))
	logMsg("zk", fmt.Sprintf("Bob saltB: 0x%s…", paymentResult.SaltB.Text(16)[:16]))
	panel(0, "nullifier", "0x"+stmt[3].Text(16))
	panel(0, "proof", "0x"+paymentResult.Proof[0])
	panel(1, "commitment", "0x"+stmt[4].Text(16))
	proto("proofTimeMs", fmt.Sprintf("%d", proofGenMs))
	proto("proofSizeBytes", "256")
	emit("zkproof", "success", "Generate ZK Proof", fmt.Sprintf("Groth16 proof generated in %d ms — nullifier and output commitments revealed, inputs remain private", proofGenMs))
	pause(2000 * time.Millisecond)

	// ── Step: Relay payment ───────────────────────────────────────────────────
	emit("relay", "running", "Relay Payment", "Alice sends unsigned proof to relayer (POST /relay/payment)…")

	relayReq := buildRelayPaymentRequest(paymentResult)
	var payResp relayPaymentResponse
	relayCallStart := time.Now()
	status, err := postRelayer("/relay/payment", relayReq, &payResp)
	relayRTTms = time.Since(relayCallStart).Milliseconds()
	if err != nil { fail("relay", "Relay Payment", err); return }
	if status != http.StatusOK {
		fail("relay", "Relay Payment", fmt.Errorf("relayer %d: %s", status, payResp.Error))
		return
	}

	totalGasUsed += payResp.GasUsed
	proto("payTxHash", payResp.TxHash)
	proto("payBlock", fmt.Sprintf("%d", payResp.BlockNumber))
	proto("payGas", fmt.Sprintf("%d", payResp.GasUsed))
	proto("verifyTimeMs", fmt.Sprintf("%d", relayRTTms))
	logMsg("chain", fmt.Sprintf("EnygmaDvp.payment() → block %d  gas %s  tx %s…",
		payResp.BlockNumber, formatNum(payResp.GasUsed), shortHex(payResp.TxHash, 18)))
	logMsg("zk", fmt.Sprintf("relay round-trip (submit→confirmed): %d ms  verify gas: %s", relayRTTms, formatNum(payResp.GasUsed)))
	panel(0, "balance", fmt.Sprintf("0 tokens (note spent at block %d)", payResp.BlockNumber))
	emit("relay", "success", "Relay Payment", fmt.Sprintf("Relayer submitted payment() on-chain (block %d, gas %s)", payResp.BlockNumber, formatNum(payResp.GasUsed)))
	pause(2000 * time.Millisecond)

	// ── Step: Tag notification (with-tag mode) ────────────────────────────────
	var tagBlock uint64
	if withTag {
		emit("tag", "running", "Publish Tag", "Alice notifies Bob via private tag (POST /relay/tag)…")

		startBlock, windowTags, noteCtxt, err := tags.PreparePaymentTag(
			client, 3,
			bobSpend.PublicKey,
			channelSS,
			big.NewInt(paymentAmt), tokenId, paymentResult.SaltB,
		)
		if err != nil { fail("tag", "Publish Tag", fmt.Errorf("PreparePaymentTag: %w", err)); return }

		hexTags := make([]string, len(windowTags))
		for i, wt := range windowTags {
			wt := wt
			hexTags[i] = toHex(wt[:])
		}

		var tagResp relayTagResponse
		status, err := postRelayer("/relay/tag", relayTagRequest{
			Tags:       hexTags,
			StartBlock: startBlock,
			Ctxt:       toHex(noteCtxt),
		}, &tagResp)
		if err != nil { fail("tag", "Publish Tag", err); return }
		if status != http.StatusOK {
			fail("tag", "Publish Tag", fmt.Errorf("relayer %d: %s", status, tagResp.Error))
			return
		}
		totalGasUsed += tagResp.GasUsed
		tagBlock = tagResp.BlockNumber
		proto("tagTxHash", tagResp.TxHash)
		proto("tagBlock", fmt.Sprintf("%d", tagBlock))
		proto("tagGas", fmt.Sprintf("%d", tagResp.GasUsed))
		logMsg("chain", fmt.Sprintf("TagRegistry.publishTag() → block %d  gas %s  tx %s…",
			tagBlock, formatNum(tagResp.GasUsed), shortHex(tagResp.TxHash, 18)))
		emit("tag", "success", "Publish Tag", fmt.Sprintf("Encrypted payment note published to TagRegistry (block %d)", tagBlock))
		pause(2000 * time.Millisecond)

		// ── Step: Bob scans TagRegistry ───────────────────────────────────────
		emit("scan_tag", "running", "Bob Scans Tag", "Bob scans TagRegistry and decrypts payment note…")

		bobChannels := []tags.Channel{{SharedSecret: channelSS, PkSpend: bobSpend.PublicKey}}
		cursor := tags.NewScanCursor()
		matches, _, err := tags.ScanBlocksFromCursor(client, tagRegistryAddr, bobChannels, cursor, tagBlock)
		if err != nil { fail("scan_tag", "Bob Scans Tag", fmt.Errorf("ScanBlocksFromCursor: %w", err)); return }
		if len(matches) == 0 {
			fail("scan_tag", "Bob Scans Tag", fmt.Errorf("Bob found 0 tags in registry"))
			return
		}
		note, err := tags.DecryptPaymentNote(channelSS, matches[0].Entry.Ctxt)
		if err != nil { fail("scan_tag", "Bob Scans Tag", fmt.Errorf("DecryptPaymentNote: %w", err)); return }

		logMsg("tag", fmt.Sprintf("Bob found tag in TagRegistry → decrypting note"))
		logMsg("tag", fmt.Sprintf("note.amount=%s  note.tokenId=%s  note.salt=0x%s…",
			note.Amount, note.TokenId, note.Salt.Text(16)[:16]))

		// Verify Bob's commitment matches the on-chain output.
		bobCmt, err := rpcore.Erc20CommitmentV2(bobSpend.PublicKey, note.Salt, note.Amount, note.TokenId)
		if err != nil { fail("scan_tag", "Bob Scans Tag", fmt.Errorf("Erc20CommitmentV2 (verify): %w", err)); return }
		if bobCmt.Cmp(stmt[4]) != 0 {
			fail("scan_tag", "Bob Scans Tag", fmt.Errorf("commitment mismatch: got %s want %s", bobCmt.Text(10)[:12], stmt[4].Text(10)[:12]))
			return
		}
		logMsg("key", fmt.Sprintf("Bob commitment: 0x%s… ✓ matches on-chain", bobCmt.Text(16)[:16]))
		panel(1, "balance", fmt.Sprintf("%d tokens (discovered via tag)", note.Amount))
		panel(1, "salt", "0x"+note.Salt.Text(16))
		emit("scan_tag", "success", "Bob Scans Tag", fmt.Sprintf("Bob decrypted note from TagRegistry: %s tokens", note.Amount))
		pause(1600 * time.Millisecond)

		// ── Step: Verify ─────────────────────────────────────────────────────
		emit("verify", "running", "Verify Commitment", "Checking Bob and Alice commitments match on-chain state…")
		pause(1200 * time.Millisecond)

		aliceChangeCmt, err := rpcore.Erc20CommitmentV2(aliceSpend.PublicKey, paymentResult.SaltA, big.NewInt(changeAmt), tokenId)
		if err != nil { fail("verify", "Verify Commitment", err); return }
		if aliceChangeCmt.Cmp(stmt[5]) != 0 {
			fail("verify", "Verify Commitment", fmt.Errorf("Alice change commitment mismatch"))
			return
		}
		logMsg("key", fmt.Sprintf("Alice change commitment: 0x%s… ✓ matches on-chain", aliceChangeCmt.Text(16)[:16]))
		panel(0, "balance", fmt.Sprintf("%d tokens change (spendable)", changeAmt))
		emit("verify", "success", "Verify Commitment",
			fmt.Sprintf("All commitments verified on-chain: Bob received %d tokens, Alice holds %d tokens change", paymentAmt, changeAmt))

	} else {
		// ── Step: Bob scans (plain mode) ──────────────────────────────────────
		emit("scan", "running", "Bob Scans Note", "Bob decrypts his note from the ML-KEM ciphertext in the Payment event…")
		pause(1800 * time.Millisecond)

		bobEvents := []dvpcore.OnChainErc20Event{{
			Commitment: stmt[4],
			CipherText: paymentResult.CipherText,
			EncTxData:  paymentResult.EncTxData,
		}}
		bobNotes, err := dvpcore.ScanForErc20Notes(bobView.DecapsKey, bobSpend.PublicKey, bobEvents)
		if err != nil { fail("scan", "Bob Scans Note", fmt.Errorf("ScanForErc20Notes: %w", err)); return }
		if len(bobNotes) == 0 {
			fail("scan", "Bob Scans Note", fmt.Errorf("Bob found 0 notes"))
			return
		}
		bobNote := bobNotes[0]
		if bobNote.Amount.Cmp(big.NewInt(paymentAmt)) != 0 {
			fail("scan", "Bob Scans Note", fmt.Errorf("amount mismatch: got %s want %d", bobNote.Amount, paymentAmt))
			return
		}

		// Verify Alice's change commitment locally.
		aliceChangeCmt, err := rpcore.Erc20CommitmentV2(aliceSpend.PublicKey, paymentResult.SaltA, big.NewInt(changeAmt), tokenId)
		if err != nil { fail("scan", "Bob Scans Note", err); return }
		if aliceChangeCmt.Cmp(stmt[5]) != 0 {
			fail("scan", "Bob Scans Note", fmt.Errorf("Alice change commitment mismatch"))
			return
		}
		logMsg("key", fmt.Sprintf("Bob decrypted note via ML-KEM: amount=%s tokenId=%s", bobNote.Amount, bobNote.TokenId))
		logMsg("key", fmt.Sprintf("Alice change commitment: 0x%s… ✓", aliceChangeCmt.Text(16)[:16]))
		panel(1, "balance", fmt.Sprintf("%d tokens (received)", bobNote.Amount))
		panel(1, "salt", "0x"+paymentResult.SaltB.Text(16))
		panel(0, "balance", fmt.Sprintf("%d tokens change (spendable)", changeAmt))
		emit("scan", "success", "Bob Scans Note",
			fmt.Sprintf("Bob decrypted note via ML-KEM view key: %d tokens received", paymentAmt))
	}

	// ── Done ──────────────────────────────────────────────────────────────────
	flowMs := time.Since(flowStart).Milliseconds()
	proto("totalGas", fmt.Sprintf("%d", totalGasUsed))
	proto("flowTimeMs", fmt.Sprintf("%d", flowMs))
	logMsg("", fmt.Sprintf("✓ Total flow: %d ms  total protocol gas: %s", flowMs, formatNum(totalGasUsed)))
	mode := "plain"
	if withTag { mode = "with private tag" }
	b.publish(Event{
		Type:   "done",
		Status: "success",
		Msg:    fmt.Sprintf("Payment complete (%s): Alice → %d tokens → Bob", mode, paymentAmt),
	})
}

// ── Entry point ───────────────────────────────────────────────────────────────

func main() {
	broker := newBroker()
	srv := &Server{broker: broker}

	mux := http.NewServeMux()
	mux.HandleFunc("/", srv.handleIndex)
	mux.HandleFunc("/events", srv.handleEvents)
	mux.HandleFunc("/run", srv.handleRun)

	log.Printf("Enygma Retail Payments Demo listening on http://localhost%s", listenAddr)
	log.Printf("Open http://localhost%s in your browser", listenAddr)
	if err := http.ListenAndServe(listenAddr, mux); err != nil {
		log.Fatal(err)
	}
}


