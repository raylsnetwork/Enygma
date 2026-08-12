// Enygma DvP · NFT ↔ ERC20 Swap · Live Demo
//
// A local HTTP server that runs the full DvP (Delivery vs Payment) atomic
// swap flow step by step and streams each step as a Server-Sent Event to a
// live dashboard. Each of the 10 steps is triggered independently by its own
// button in the UI — press "Run" on a step to execute it and inspect the
// result before moving on to the next one.
//
// Scenario: Alice has 50 USDT (ERC20). Bob has an NFT ticket (ERC721).
// They swap atomically — Alice delivers 50 USDT to Bob, Bob delivers the
// ticket to Alice — using the two-leg submitPartialSettlement() flow (no
// relayer required; each party submits their own leg directly).
//
// Usage:
//
//	cd demo && bash run.sh
//	open http://localhost:9092
//
// Prerequisites (all must be running before clicking Run):
//
//  1. Hardhat node  :  npx hardhat node                       (from enygma_dvp/)
//  2. Deploy+init   :  see enygma_dvp/MEMORY.md / scripts/
//  3. Gnark server  :  cd gnark_circuits && go run main.go     (must include DvP keys)
package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/raylsnetwork/enygma_dvp/src/core"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

//go:embed index.html
var indexHTML string

// ── Config ─────────────────────────────────────────────────────────────────────

const (
	listenAddr = ":9092"
	gnarkURL   = "http://localhost:8081"

	// Hardhat account[0] — same address settles both legs on-chain; Alice and
	// Bob are distinguished by their off-chain spend/view keypairs, not by
	// Ethereum address (this mirrors how the underlying test suite operates).
	hardhatPrivKeyHex = "34d091c661db4c814d65c8ae9277b7055c0dde5a752ce5a3fdfd4ea11a8f7154"

	merkleDepth  = 8
	erc20Amount  = int64(50) // Alice delivers 50 USDT
	erc20TokenId = int64(0)
	nftAmount    = int64(1)

	vaultIdErc20  = 0 // VAULT_ID_ERC20
	vaultIdErc721 = 1 // VAULT_ID_ERC721
	groupFungible = 0 // GROUP_ID_FUNGIBLES
	groupNonFung  = 1 // GROUP_ID_NON_FUNGIBLES

	deadlineWindowSeconds = 3600 // 1 hour — plenty of time for a live demo
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
	Cat    string `json:"cat,omitempty"` // log category: key|chain|zk
	Pid    int    `json:"pid,omitempty"`
	Field  string `json:"field,omitempty"`
	Value  string `json:"value,omitempty"`
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

// emitStep publishes a step status transition (running / success / error).
func emitStep(b *Broker, step, status, label, msg string) {
	b.publish(Event{Type: "step", Step: step, Status: status, Label: label, Msg: msg})
}

// logMsg publishes a free-form event-log line, optionally tagged with a category.
func logMsg(b *Broker, cat, msg string) {
	b.publish(Event{Type: "log", Cat: cat, Msg: msg, TS: time.Now().Format("15:04:05.000")})
}

// panelSet publishes a field update for one of the two participant panels (0=Alice, 1=Bob).
func panelSet(b *Broker, pid int, field, value string) {
	b.publish(Event{Type: "participant", Pid: pid, Field: field, Value: value})
}

// protoSet publishes a field update for the shared protocol/metrics panel.
func protoSet(b *Broker, field, value string) {
	b.publish(Event{Type: "proto", Field: field, Value: value})
}

// ── Flow state ────────────────────────────────────────────────────────────────
//
// FlowState holds everything one step hands off to the next. It lives on the
// Server and is replaced wholesale by /reset. Because the HTTP layer only
// ever runs one step at a time (guarded by Server.runLock, held for the
// entire duration of a step — including on /reset), fields need no locking
// of their own beyond that single mutex.

// stepOrder is the fixed sequence a swap must be walked through. A step can
// only be started once every step before it in this list has completed.
var stepOrder = []string{
	"prereqs", "keygen", "deposit_erc20", "deposit_nft",
	"dvp_initiator", "bob_scan", "dvp_destination",
	"settle_alice", "settle_bob", "verify",
}

var stepIndex = func() map[string]int {
	m := make(map[string]int, len(stepOrder))
	for i, s := range stepOrder {
		m[s] = i
	}
	return m
}()

type FlowState struct {
	broker *Broker
	done   map[string]bool

	// set by prereqs
	ctx                                                          context.Context
	client                                                       *ethclient.Client
	dir                                                          string
	erc20VaultAddr, erc721VaultAddr, dvpAddr                     common.Address
	erc20Addr, erc721Addr                                        common.Address
	erc20Vault, erc20Token, erc721Vault, erc721Token, dvp        *bind.BoundContract
	auth                                                         *bind.TransactOpts
	gnarkClient                                                  *core.GnarkClient
	nftTokenIdInt                                                int64
	erc20AmountBig, erc20TokenIdBig, nftTokenIdBig, nftAmountBig *big.Int
	flowStart                                                    time.Time
	totalGasUsed                                                 uint64

	// keygen
	aliceSpend, bobSpend *core.SpendKeyPair
	aliceView, bobView   *core.ViewKeyPair

	// deposit_erc20
	ssAlice, capsuleAlice []byte
	aliceSaltField        *big.Int
	aliceCommitment       *big.Int

	// deposit_nft
	bobNftSalt    *big.Int
	bobCommitment *big.Int

	// merkle proofs, computed at the start of dvp_initiator (after both deposits)
	aliceMerkleProof, bobMerkleProof *core.MerkleProof

	// dvp_initiator
	initiatorResult *core.DvPInitiatorResult
	proofGenMs      int64

	// bob_scan
	saltAField, saltBField *big.Int
	decTokenId, decAmount  *big.Int

	// dvp_destination
	destinationResult *core.DvPDestinationResult
	aliceReceipt      onchainProofReceipt
	bobReceipt        onchainProofReceipt

	// settle_alice / settle_bob
	deadline       *big.Int
	aliceTxReceipt *types.Receipt
	bobTxReceipt   *types.Receipt
}

func newFlowState(b *Broker) *FlowState {
	return &FlowState{broker: b, done: make(map[string]bool), ctx: context.Background()}
}

// ── Server ────────────────────────────────────────────────────────────────────

type Server struct {
	broker  *Broker
	runLock sync.Mutex
	fs      *FlowState
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

// stepFns maps each step id to the function that executes it. Registered in
// init() below, once all the step functions are declared.
var stepFns map[string]func(*FlowState)

// handleStep runs exactly one step, identified by the URL path /step/{id}.
// It refuses to start a step whose prerequisites haven't completed yet, and
// refuses to start a second step while one is already running.
func (s *Server) handleStep(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/step/")
	fn, ok := stepFns[id]
	if !ok {
		http.Error(w, `{"error":"unknown step"}`, 404)
		return
	}
	if !s.runLock.TryLock() {
		http.Error(w, `{"error":"a step is already running"}`, 409)
		return
	}
	idx := stepIndex[id]
	for _, prev := range stepOrder[:idx] {
		if !s.fs.done[prev] {
			s.runLock.Unlock()
			http.Error(w, fmt.Sprintf(`{"error":"run %q first"}`, prev), 409)
			return
		}
	}
	if s.fs.done[id] {
		s.runLock.Unlock()
		http.Error(w, `{"error":"step already completed — Reset to run the flow again"}`, 409)
		return
	}
	go func() {
		defer s.runLock.Unlock()
		fn(s.fs)
	}()
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"status":"started"}`)
}

// handleReset clears all flow state and tells every connected client to
// blank their UI. Refuses to run while a step is in flight.
func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	if !s.runLock.TryLock() {
		http.Error(w, `{"error":"a step is running — wait for it to finish"}`, 409)
		return
	}
	defer s.runLock.Unlock()
	s.fs = newFlowState(s.broker)
	s.broker.publish(Event{Type: "reset"})
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"status":"reset"}`)
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

func loadReceipts(dir string) (map[string]receiptEntry, error) {
	path := filepath.Join(dir, "..", "build", "receipts.json")
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

func loadABI(dir, artifactRelPath string) (abi.ABI, error) {
	path := filepath.Join(dir, "..", "artifacts", "contracts", artifactRelPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return abi.ABI{}, fmt.Errorf("read artifact %s: %w", artifactRelPath, err)
	}
	var artifact struct {
		ABI json.RawMessage `json:"abi"`
	}
	if err := json.Unmarshal(data, &artifact); err != nil {
		return abi.ABI{}, fmt.Errorf("parse artifact JSON: %w", err)
	}
	return abi.JSON(strings.NewReader(string(artifact.ABI)))
}

func makeAuth() (*bind.TransactOpts, error) {
	key, err := crypto.HexToECDSA(hardhatPrivKeyHex)
	if err != nil {
		return nil, fmt.Errorf("HexToECDSA: %w", err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(chainID))
	if err != nil {
		return nil, fmt.Errorf("NewKeyedTransactorWithChainID: %w", err)
	}
	auth.GasLimit = 6_000_000
	return auth, nil
}

func shortHex(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

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

// ── On-chain proof / receipt structs (mirrors IEnygmaDvp.sol) ──────────────────

type onchainG1Point struct {
	X *big.Int `abi:"x"`
	Y *big.Int `abi:"y"`
}

type onchainG2Point struct {
	X [2]*big.Int `abi:"x"`
	Y [2]*big.Int `abi:"y"`
}

type onchainSnarkProof struct {
	A onchainG1Point `abi:"a"`
	B onchainG2Point `abi:"b"`
	C onchainG1Point `abi:"c"`
}

type onchainProofReceipt struct {
	Proof           onchainSnarkProof `abi:"proof"`
	Statement       []*big.Int        `abi:"statement"`
	NumberOfInputs  *big.Int          `abi:"numberOfInputs"`
	NumberOfOutputs *big.Int          `abi:"numberOfOutputs"`
}

// proofStringsToOnchain converts the gnark server's 8 decimal-string proof
// elements into the EIP-197 G1/G2 point encoding the contract expects.
//
// Gnark handler output order: [Ax, Ay, BX_A1(imag), BX_A0(real), BY_A1(imag), BY_A0(real), Cx, Cy]
func proofStringsToOnchain(proof []string) (onchainSnarkProof, error) {
	if len(proof) != 8 {
		return onchainSnarkProof{}, fmt.Errorf("expected 8 proof elements, got %d", len(proof))
	}
	vals := make([]*big.Int, 8)
	for i, s := range proof {
		n, ok := new(big.Int).SetString(s, 10)
		if !ok {
			return onchainSnarkProof{}, fmt.Errorf("invalid proof element %d: %q", i, s)
		}
		vals[i] = n
	}
	return onchainSnarkProof{
		A: onchainG1Point{X: vals[0], Y: vals[1]},
		B: onchainG2Point{
			X: [2]*big.Int{vals[2], vals[3]},
			Y: [2]*big.Int{vals[4], vals[5]},
		},
		C: onchainG1Point{X: vals[6], Y: vals[7]},
	}, nil
}

// loadVaultMerkleTree replays historical Commitment events to rebuild a vault's tree.
func loadVaultMerkleTree(ctx context.Context, client *ethclient.Client, vaultAddr common.Address) (*core.MerkleTree, error) {
	commitmentSig := crypto.Keccak256Hash([]byte("Commitment(uint256,uint256)"))
	query := ethereum.FilterQuery{
		Addresses: []common.Address{vaultAddr},
		Topics:    [][]common.Hash{{commitmentSig}},
	}
	logs, err := client.FilterLogs(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("FilterLogs (vault Commitment events): %w", err)
	}
	mt := core.NewMerkleTree(merkleDepth)
	for _, l := range logs {
		if len(l.Topics) >= 3 {
			mt.InsertLeaf(l.Topics[2].Big())
		}
	}
	return mt, nil
}

// ── Step 1: Prerequisites ───────────────────────────────────────────────────

func stepPrereqs(fs *FlowState) {
	b := fs.broker
	fs.flowStart = time.Now()
	emitStep(b, "prereqs", "running", "Check prerequisites", "Probing chain · gnark server…")
	pause(1200 * time.Millisecond)

	chainOK := tcpAvailable("127.0.0.1:8545")
	gnarkOK := tcpAvailable("localhost:8081")
	var missing []string
	if !chainOK {
		missing = append(missing, "Hardhat (8545)")
	}
	if !gnarkOK {
		missing = append(missing, "gnark server (8081)")
	}
	if len(missing) > 0 {
		emitStep(b, "prereqs", "error", "Check prerequisites", fmt.Errorf("services not running: %s", strings.Join(missing, ", ")).Error())
		return
	}
	logMsg(b, "", "✓ chain and gnark server are both reachable")

	_, thisFile, _, _ := runtime.Caller(0)
	fs.dir = filepath.Dir(thisFile)

	receipts, err := loadReceipts(fs.dir)
	if err != nil {
		emitStep(b, "prereqs", "error", "Check prerequisites", err.Error())
		return
	}
	fs.erc20VaultAddr = common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress)
	fs.erc20Addr = common.HexToAddress(receipts["ERC20"].ContractAddress)
	fs.erc721VaultAddr = common.HexToAddress(receipts["Erc721CoinVault"].ContractAddress)
	fs.erc721Addr = common.HexToAddress(receipts["ERC721"].ContractAddress)
	fs.dvpAddr = common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)

	protoSet(b, "erc20VaultAddr", fs.erc20VaultAddr.Hex())
	protoSet(b, "erc721VaultAddr", fs.erc721VaultAddr.Hex())
	protoSet(b, "dvpAddr", fs.dvpAddr.Hex())
	logMsg(b, "chain", fmt.Sprintf("Erc20CoinVault:  %s", shortHex(fs.erc20VaultAddr.Hex(), 18)))
	logMsg(b, "chain", fmt.Sprintf("Erc721CoinVault: %s", shortHex(fs.erc721VaultAddr.Hex(), 18)))
	logMsg(b, "chain", fmt.Sprintf("EnygmaDvp:       %s", shortHex(fs.dvpAddr.Hex(), 18)))

	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		emitStep(b, "prereqs", "error", "Check prerequisites", fmt.Errorf("ethclient.Dial: %w", err).Error())
		return
	}
	fs.client = client

	erc20VaultABI, err := loadABI(fs.dir, "core/contracts/vaults/Erc20CoinVault.sol/Erc20CoinVault.json")
	if err != nil {
		emitStep(b, "prereqs", "error", "Check prerequisites", err.Error())
		return
	}
	erc20ABI, err := loadABI(fs.dir, "erc20/contracts/RaylsERC20.sol/RaylsERC20.json")
	if err != nil {
		emitStep(b, "prereqs", "error", "Check prerequisites", err.Error())
		return
	}
	erc721VaultABI, err := loadABI(fs.dir, "core/contracts/vaults/Erc721CoinVault.sol/Erc721CoinVault.json")
	if err != nil {
		emitStep(b, "prereqs", "error", "Check prerequisites", err.Error())
		return
	}
	erc721ABI, err := loadABI(fs.dir, "erc721/contracts/RaylsERC721.sol/RaylsERC721.json")
	if err != nil {
		emitStep(b, "prereqs", "error", "Check prerequisites", err.Error())
		return
	}
	dvpABI, err := loadABI(fs.dir, "core/contracts/EnygmaDvp.sol/EnygmaDvp.json")
	if err != nil {
		emitStep(b, "prereqs", "error", "Check prerequisites", err.Error())
		return
	}

	fs.erc20Vault = bind.NewBoundContract(fs.erc20VaultAddr, erc20VaultABI, client, client, client)
	fs.erc20Token = bind.NewBoundContract(fs.erc20Addr, erc20ABI, client, client, client)
	fs.erc721Vault = bind.NewBoundContract(fs.erc721VaultAddr, erc721VaultABI, client, client, client)
	fs.erc721Token = bind.NewBoundContract(fs.erc721Addr, erc721ABI, client, client, client)
	fs.dvp = bind.NewBoundContract(fs.dvpAddr, dvpABI, client, client, client)

	auth, err := makeAuth()
	if err != nil {
		emitStep(b, "prereqs", "error", "Check prerequisites", err.Error())
		return
	}
	fs.auth = auth
	panelSet(b, 0, "ethAddr", auth.From.Hex())
	panelSet(b, 1, "ethAddr", auth.From.Hex())

	fs.gnarkClient = core.NewGnarkClient(gnarkURL)

	// Randomize the NFT tokenId each run so repeated demo runs never collide.
	fs.nftTokenIdInt = int64(1_000_000 + rand.Intn(900_000))
	fs.erc20AmountBig = big.NewInt(erc20Amount)
	fs.erc20TokenIdBig = big.NewInt(erc20TokenId)
	fs.nftTokenIdBig = big.NewInt(fs.nftTokenIdInt)
	fs.nftAmountBig = big.NewInt(nftAmount)

	emitStep(b, "prereqs", "success", "Check prerequisites", "All services running")
	fs.done["prereqs"] = true
}

// ── Step 2: Key generation ──────────────────────────────────────────────────

func stepKeygen(fs *FlowState) {
	b := fs.broker
	emitStep(b, "keygen", "running", "Generate key pairs", "Alice & Bob each generate spend+view keypairs…")
	pause(1800 * time.Millisecond)

	aliceSpend, err := core.NewSpendKeyPair()
	if err != nil {
		emitStep(b, "keygen", "error", "Generate key pairs", err.Error())
		return
	}
	aliceView, err := core.NewViewKeyPair()
	if err != nil {
		emitStep(b, "keygen", "error", "Generate key pairs", err.Error())
		return
	}
	bobSpend, err := core.NewSpendKeyPair()
	if err != nil {
		emitStep(b, "keygen", "error", "Generate key pairs", err.Error())
		return
	}
	bobView, err := core.NewViewKeyPair()
	if err != nil {
		emitStep(b, "keygen", "error", "Generate key pairs", err.Error())
		return
	}
	fs.aliceSpend, fs.aliceView, fs.bobSpend, fs.bobView = aliceSpend, aliceView, bobSpend, bobView

	panelSet(b, 0, "pkSpend", "0x"+aliceSpend.PublicKey.Text(16))
	panelSet(b, 1, "pkSpend", "0x"+bobSpend.PublicKey.Text(16))
	panelSet(b, 0, "pkView", "0x"+common.Bytes2Hex(aliceView.EncapsKey))
	panelSet(b, 1, "pkView", "0x"+common.Bytes2Hex(bobView.EncapsKey))
	logMsg(b, "key", fmt.Sprintf("Alice pk_spend: 0x%s…", aliceSpend.PublicKey.Text(16)[:16]))
	logMsg(b, "key", fmt.Sprintf("Bob   pk_spend: 0x%s…", bobSpend.PublicKey.Text(16)[:16]))
	emitStep(b, "keygen", "success", "Generate key pairs", "BabyJubJub spend keys + ML-KEM-768 view keypairs generated")
	fs.done["keygen"] = true
}

// ── Step 3: Alice deposits ERC20 ────────────────────────────────────────────

func stepDepositErc20(fs *FlowState) {
	b := fs.broker
	ctx := fs.ctx
	client := fs.client
	emitStep(b, "deposit_erc20", "running", "Alice Deposits ERC20", fmt.Sprintf("Alice mints and deposits %d USDT into the vault…", erc20Amount))

	ssAlice, capsuleAlice, err := core.Encapsulate(fs.aliceView.EncapsKey)
	if err != nil {
		emitStep(b, "deposit_erc20", "error", "Alice Deposits ERC20", fmt.Errorf("Encapsulate: %w", err).Error())
		return
	}
	fs.ssAlice, fs.capsuleAlice = ssAlice, capsuleAlice
	aliceSaltBytes, err := core.DerivePaymentSalt(ssAlice)
	if err != nil {
		emitStep(b, "deposit_erc20", "error", "Alice Deposits ERC20", err.Error())
		return
	}
	aliceEncKey, err := core.DerivePaymentKey(ssAlice)
	if err != nil {
		emitStep(b, "deposit_erc20", "error", "Alice Deposits ERC20", err.Error())
		return
	}
	fs.aliceSaltField = core.SaltBToField(aliceSaltBytes)

	aliceCommitment, err := core.Erc20CommitmentV2(fs.aliceSpend.PublicKey, fs.aliceSaltField, fs.erc20AmountBig, fs.erc20TokenIdBig)
	if err != nil {
		emitStep(b, "deposit_erc20", "error", "Alice Deposits ERC20", err.Error())
		return
	}
	fs.aliceCommitment = aliceCommitment
	aliceDepositEnc, err := core.EncryptPayload(aliceEncKey, fs.erc20TokenIdBig, fs.erc20AmountBig)
	if err != nil {
		emitStep(b, "deposit_erc20", "error", "Alice Deposits ERC20", err.Error())
		return
	}

	mintErc20Tx, err := fs.erc20Token.Transact(fs.auth, "mint", fs.auth.From, new(big.Int).Mul(fs.erc20AmountBig, big.NewInt(10)))
	if err != nil {
		emitStep(b, "deposit_erc20", "error", "Alice Deposits ERC20", fmt.Errorf("ERC20.mint: %w", err).Error())
		return
	}
	if _, err := bind.WaitMined(ctx, client, mintErc20Tx); err != nil {
		emitStep(b, "deposit_erc20", "error", "Alice Deposits ERC20", fmt.Errorf("wait mint: %w", err).Error())
		return
	}
	approveErc20Tx, err := fs.erc20Token.Transact(fs.auth, "approve", fs.erc20VaultAddr, fs.erc20AmountBig)
	if err != nil {
		emitStep(b, "deposit_erc20", "error", "Alice Deposits ERC20", fmt.Errorf("ERC20.approve: %w", err).Error())
		return
	}
	if _, err := bind.WaitMined(ctx, client, approveErc20Tx); err != nil {
		emitStep(b, "deposit_erc20", "error", "Alice Deposits ERC20", fmt.Errorf("wait approve: %w", err).Error())
		return
	}
	depositErc20Tx, err := fs.erc20Vault.Transact(fs.auth, "depositV2",
		[]*big.Int{fs.erc20AmountBig, fs.aliceSpend.PublicKey, fs.aliceSaltField, fs.erc20TokenIdBig}, capsuleAlice, aliceDepositEnc)
	if err != nil {
		emitStep(b, "deposit_erc20", "error", "Alice Deposits ERC20", fmt.Errorf("depositV2: %w", err).Error())
		return
	}
	depositErc20Receipt, err := bind.WaitMined(ctx, client, depositErc20Tx)
	if err != nil {
		emitStep(b, "deposit_erc20", "error", "Alice Deposits ERC20", fmt.Errorf("wait depositV2: %w", err).Error())
		return
	}
	fs.totalGasUsed += depositErc20Receipt.GasUsed
	logMsg(b, "chain", fmt.Sprintf("Erc20CoinVault.depositV2() → block %d  gas %s", depositErc20Receipt.BlockNumber, formatNum(depositErc20Receipt.GasUsed)))
	logMsg(b, "key", fmt.Sprintf("Alice commitment: 0x%s…", aliceCommitment.Text(16)[:16]))
	panelSet(b, 0, "commitment", "0x"+aliceCommitment.Text(16))
	panelSet(b, 0, "balance", fmt.Sprintf("%d USDT (deposited)", erc20Amount))
	emitStep(b, "deposit_erc20", "success", "Alice Deposits ERC20", fmt.Sprintf("Alice deposited %d USDT — Poseidon commitment recorded on-chain (block %d)", erc20Amount, depositErc20Receipt.BlockNumber))
	fs.done["deposit_erc20"] = true
}

// ── Step 4: Bob deposits NFT ────────────────────────────────────────────────

func stepDepositNft(fs *FlowState) {
	b := fs.broker
	ctx := fs.ctx
	client := fs.client
	emitStep(b, "deposit_nft", "running", "Bob Deposits NFT", fmt.Sprintf("Bob mints and deposits ticket tokenId=%d into the NFT vault…", fs.nftTokenIdInt))

	bobNftSalt, err := core.RandomInField()
	if err != nil {
		emitStep(b, "deposit_nft", "error", "Bob Deposits NFT", err.Error())
		return
	}
	fs.bobNftSalt = bobNftSalt
	bobCommitment, err := core.Erc721Commitment(fs.nftTokenIdBig, fs.bobSpend.PublicKey, bobNftSalt)
	if err != nil {
		emitStep(b, "deposit_nft", "error", "Bob Deposits NFT", err.Error())
		return
	}
	fs.bobCommitment = bobCommitment

	mintNftTx, err := fs.erc721Token.Transact(fs.auth, "mint", fs.auth.From, fs.nftTokenIdBig)
	if err != nil {
		emitStep(b, "deposit_nft", "error", "Bob Deposits NFT", fmt.Errorf("ERC721.mint: %w", err).Error())
		return
	}
	if _, err := bind.WaitMined(ctx, client, mintNftTx); err != nil {
		emitStep(b, "deposit_nft", "error", "Bob Deposits NFT", fmt.Errorf("wait mint: %w", err).Error())
		return
	}
	approveNftTx, err := fs.erc721Token.Transact(fs.auth, "approve", fs.erc721VaultAddr, fs.nftTokenIdBig)
	if err != nil {
		emitStep(b, "deposit_nft", "error", "Bob Deposits NFT", fmt.Errorf("ERC721.approve: %w", err).Error())
		return
	}
	if _, err := bind.WaitMined(ctx, client, approveNftTx); err != nil {
		emitStep(b, "deposit_nft", "error", "Bob Deposits NFT", fmt.Errorf("wait approve: %w", err).Error())
		return
	}
	depositNftTx, err := fs.erc721Vault.Transact(fs.auth, "deposit", []*big.Int{fs.nftTokenIdBig, fs.bobSpend.PublicKey, bobNftSalt})
	if err != nil {
		emitStep(b, "deposit_nft", "error", "Bob Deposits NFT", fmt.Errorf("deposit: %w", err).Error())
		return
	}
	depositNftReceipt, err := bind.WaitMined(ctx, client, depositNftTx)
	if err != nil {
		emitStep(b, "deposit_nft", "error", "Bob Deposits NFT", fmt.Errorf("wait deposit: %w", err).Error())
		return
	}
	fs.totalGasUsed += depositNftReceipt.GasUsed
	logMsg(b, "chain", fmt.Sprintf("Erc721CoinVault.deposit() → block %d  gas %s", depositNftReceipt.BlockNumber, formatNum(depositNftReceipt.GasUsed)))
	logMsg(b, "key", fmt.Sprintf("Bob commitment: 0x%s…", bobCommitment.Text(16)[:16]))
	panelSet(b, 1, "commitment", "0x"+bobCommitment.Text(16))
	panelSet(b, 1, "balance", fmt.Sprintf("ticket #%d (deposited)", fs.nftTokenIdInt))
	emitStep(b, "deposit_nft", "success", "Bob Deposits NFT", fmt.Sprintf("Bob deposited ticket #%d — Poseidon commitment recorded on-chain (block %d)", fs.nftTokenIdInt, depositNftReceipt.BlockNumber))
	fs.done["deposit_nft"] = true
}

// ── Step 5: DvP Initiator (Alice) ───────────────────────────────────────────

func stepDvpInitiator(fs *FlowState) {
	b := fs.broker
	ctx := fs.ctx
	client := fs.client
	emitStep(b, "dvp_initiator", "running", "DvP Initiator Proof", "Alice proves ownership of her USDT note and locks in the swap terms…")

	// Merkle proofs — computed now, against the tree state after both deposits.
	erc20Mt, err := loadVaultMerkleTree(ctx, client, fs.erc20VaultAddr)
	if err != nil {
		emitStep(b, "dvp_initiator", "error", "DvP Initiator Proof", err.Error())
		return
	}
	aliceMerkleProof, err := erc20Mt.GenerateProof(fs.aliceCommitment)
	if err != nil {
		emitStep(b, "dvp_initiator", "error", "DvP Initiator Proof", fmt.Errorf("GenerateProof (Alice ERC20): %w", err).Error())
		return
	}
	nftMt, err := loadVaultMerkleTree(ctx, client, fs.erc721VaultAddr)
	if err != nil {
		emitStep(b, "dvp_initiator", "error", "DvP Initiator Proof", err.Error())
		return
	}
	bobMerkleProof, err := nftMt.GenerateProof(fs.bobCommitment)
	if err != nil {
		emitStep(b, "dvp_initiator", "error", "DvP Initiator Proof", fmt.Errorf("GenerateProof (Bob NFT): %w", err).Error())
		return
	}
	fs.aliceMerkleProof, fs.bobMerkleProof = aliceMerkleProof, bobMerkleProof
	panelSet(b, 0, "merkleRoot", "0x"+aliceMerkleProof.Root.Text(16))
	panelSet(b, 1, "merkleRoot", "0x"+bobMerkleProof.Root.Text(16))

	proofStart := time.Now()
	initiatorResult, err := fs.gnarkClient.DvPInitiatorProof(
		core.KeyPair{PrivateKey: fs.aliceSpend.PrivateKey, PublicKey: fs.aliceSpend.PublicKey},
		fs.aliceSaltField,
		fs.erc20AmountBig, fs.erc20TokenIdBig,
		fs.bobSpend.PublicKey, fs.bobView.EncapsKey,
		fs.nftAmountBig, fs.nftTokenIdBig,
		big.NewInt(0),
		aliceMerkleProof, merkleDepth,
	)
	if err != nil {
		emitStep(b, "dvp_initiator", "error", "DvP Initiator Proof", fmt.Errorf("DvPInitiatorProof: %w", err).Error())
		return
	}
	if len(initiatorResult.Proof) != 8 {
		emitStep(b, "dvp_initiator", "error", "DvP Initiator Proof", fmt.Errorf("expected 8-element proof, got %d", len(initiatorResult.Proof)).Error())
		return
	}
	fs.initiatorResult = initiatorResult
	fs.proofGenMs = time.Since(proofStart).Milliseconds()
	logMsg(b, "zk", fmt.Sprintf("DvP Initiator proof generated in %d ms", fs.proofGenMs))
	logMsg(b, "zk", fmt.Sprintf("commitB (Bob receives %d USDT):  0x%s…", erc20Amount, initiatorResult.CommitB.Text(16)[:16]))
	logMsg(b, "zk", fmt.Sprintf("commitA (Alice receives ticket): 0x%s…", initiatorResult.CommitA.Text(16)[:16]))
	protoSet(b, "proofTimeMs", fmt.Sprintf("%d", fs.proofGenMs))
	panelSet(b, 0, "commitB", "0x"+initiatorResult.CommitB.Text(16))
	panelSet(b, 0, "commitA", "0x"+initiatorResult.CommitA.Text(16))
	emitStep(b, "dvp_initiator", "success", "DvP Initiator Proof", fmt.Sprintf("Groth16 proof generated in %d ms — swap terms locked, note encrypted for Bob", fs.proofGenMs))
	fs.done["dvp_initiator"] = true
}

// ── Step 6: Bob scans and verifies ──────────────────────────────────────────

func stepBobScan(fs *FlowState) {
	b := fs.broker
	emitStep(b, "bob_scan", "running", "Bob Scans & Verifies", "Bob decapsulates Alice's message and verifies the swap terms…")
	pause(600 * time.Millisecond)

	ssBob, err := core.Decapsulate(fs.bobView.DecapsKey, fs.initiatorResult.CipherText)
	if err != nil {
		emitStep(b, "bob_scan", "error", "Bob Scans & Verifies", fmt.Errorf("Decapsulate: %w", err).Error())
		return
	}
	bobSaltBBytes, err := core.DerivePaymentSalt(ssBob)
	if err != nil {
		emitStep(b, "bob_scan", "error", "Bob Scans & Verifies", err.Error())
		return
	}
	bobSaltABytes, err := core.DeriveDvpSaltInit(ssBob)
	if err != nil {
		emitStep(b, "bob_scan", "error", "Bob Scans & Verifies", err.Error())
		return
	}
	bobEncKey, err := core.DerivePaymentKey(ssBob)
	if err != nil {
		emitStep(b, "bob_scan", "error", "Bob Scans & Verifies", err.Error())
		return
	}
	fs.saltBField = core.SaltBToField(bobSaltBBytes)
	fs.saltAField = core.SaltBToField(bobSaltABytes)

	decTokenId, decAmount, err := core.DecryptPayload(bobEncKey, fs.initiatorResult.EncTxData)
	if err != nil {
		emitStep(b, "bob_scan", "error", "Bob Scans & Verifies", fmt.Errorf("DecryptPayload: %w", err).Error())
		return
	}
	fs.decTokenId, fs.decAmount = decTokenId, decAmount
	logMsg(b, "key", fmt.Sprintf("Bob decrypted: tokenId=%s amount=%s (Alice's USDT)", decTokenId, decAmount))

	expectedCommitB, err := core.Erc20CommitmentV2(fs.bobSpend.PublicKey, fs.saltBField, decAmount, decTokenId)
	if err != nil {
		emitStep(b, "bob_scan", "error", "Bob Scans & Verifies", err.Error())
		return
	}
	if expectedCommitB.Cmp(fs.initiatorResult.CommitB) != 0 {
		emitStep(b, "bob_scan", "error", "Bob Scans & Verifies", "commitB mismatch")
		return
	}
	expectedCommitA, err := core.Erc20CommitmentV2(fs.aliceSpend.PublicKey, fs.saltAField, fs.nftAmountBig, fs.nftTokenIdBig)
	if err != nil {
		emitStep(b, "bob_scan", "error", "Bob Scans & Verifies", err.Error())
		return
	}
	if expectedCommitA.Cmp(fs.initiatorResult.CommitA) != 0 {
		emitStep(b, "bob_scan", "error", "Bob Scans & Verifies", "commitA mismatch")
		return
	}
	logMsg(b, "key", "commitB and commitA both verified ✓ — swap terms match")
	emitStep(b, "bob_scan", "success", "Bob Scans & Verifies", fmt.Sprintf("Bob confirmed he'll receive %s USDT, and Alice will receive the ticket", decAmount))
	fs.done["bob_scan"] = true
}

// ── Step 7: DvP Destination (Bob) ───────────────────────────────────────────

func stepDvpDestination(fs *FlowState) {
	b := fs.broker
	emitStep(b, "dvp_destination", "running", "DvP Destination Proof", "Bob proves ownership of his ticket and accepts the swap…")

	destProofStart := time.Now()
	destinationResult, err := fs.gnarkClient.DvPDestinationProof(
		core.KeyPair{PrivateKey: fs.bobSpend.PrivateKey, PublicKey: fs.bobSpend.PublicKey},
		fs.bobNftSalt, fs.nftAmountBig, fs.nftTokenIdBig,
		fs.aliceSpend.PublicKey, fs.saltAField, fs.saltBField,
		fs.decAmount, fs.decTokenId,
		fs.initiatorResult.CommitA,
		big.NewInt(0),
		fs.bobMerkleProof, merkleDepth,
	)
	if err != nil {
		emitStep(b, "dvp_destination", "error", "DvP Destination Proof", fmt.Errorf("DvPDestinationProof: %w", err).Error())
		return
	}
	if len(destinationResult.Proof) != 8 {
		emitStep(b, "dvp_destination", "error", "DvP Destination Proof", fmt.Errorf("expected 8-element proof, got %d", len(destinationResult.Proof)).Error())
		return
	}
	fs.destinationResult = destinationResult
	destProofMs := time.Since(destProofStart).Milliseconds()
	logMsg(b, "zk", fmt.Sprintf("DvP Destination proof generated in %d ms", destProofMs))

	// Build both on-chain receipts now so settle_alice / settle_bob can submit directly.
	aliceProof8, err := proofStringsToOnchain(fs.initiatorResult.Proof)
	if err != nil {
		emitStep(b, "dvp_destination", "error", "DvP Destination Proof", err.Error())
		return
	}
	fs.aliceReceipt = onchainProofReceipt{
		Proof:           aliceProof8,
		Statement:       fs.initiatorResult.Statement,
		NumberOfInputs:  big.NewInt(int64(fs.initiatorResult.NumberOfInputs)),
		NumberOfOutputs: big.NewInt(int64(fs.initiatorResult.NumberOfOutputs)),
	}
	bobProof8, err := proofStringsToOnchain(destinationResult.Proof)
	if err != nil {
		emitStep(b, "dvp_destination", "error", "DvP Destination Proof", err.Error())
		return
	}
	fs.bobReceipt = onchainProofReceipt{
		Proof:           bobProof8,
		Statement:       destinationResult.Statement,
		NumberOfInputs:  big.NewInt(int64(destinationResult.NumberOfInputs)),
		NumberOfOutputs: big.NewInt(int64(destinationResult.NumberOfOutputs)),
	}

	emitStep(b, "dvp_destination", "success", "DvP Destination Proof", fmt.Sprintf("Groth16 proof generated in %d ms — Bob has accepted the swap", destProofMs))
	fs.done["dvp_destination"] = true
}

// ── Step 8: Alice submits leg 1 ─────────────────────────────────────────────

func stepSettleAlice(fs *FlowState) {
	b := fs.broker
	ctx := fs.ctx
	client := fs.client
	emitStep(b, "settle_alice", "running", "Alice Submits (Leg 1/2)", "Alice submits her ERC20 leg — locks in the swap on-chain…")

	fs.deadline = new(big.Int).SetInt64(time.Now().Unix() + deadlineWindowSeconds)
	aliceTx, err := fs.dvp.Transact(fs.auth, "submitPartialSettlement", fs.aliceReceipt, big.NewInt(vaultIdErc20), big.NewInt(groupFungible), fs.deadline)
	if err != nil {
		emitStep(b, "settle_alice", "error", "Alice Submits (Leg 1/2)", fmt.Errorf("submitPartialSettlement: %w", err).Error())
		return
	}
	aliceTxReceipt, err := bind.WaitMined(ctx, client, aliceTx)
	if err != nil {
		emitStep(b, "settle_alice", "error", "Alice Submits (Leg 1/2)", fmt.Errorf("wait submitPartialSettlement: %w", err).Error())
		return
	}
	fs.aliceTxReceipt = aliceTxReceipt
	fs.totalGasUsed += aliceTxReceipt.GasUsed
	protoSet(b, "aliceTxHash", aliceTx.Hash().Hex())
	protoSet(b, "aliceBlock", fmt.Sprintf("%d", aliceTxReceipt.BlockNumber))
	protoSet(b, "aliceGas", fmt.Sprintf("%d", aliceTxReceipt.GasUsed))
	logMsg(b, "chain", fmt.Sprintf("EnygmaDvp.submitPartialSettlement() [Alice] → block %d  gas %s  tx %s…",
		aliceTxReceipt.BlockNumber, formatNum(aliceTxReceipt.GasUsed), shortHex(aliceTx.Hash().Hex(), 18)))
	emitStep(b, "settle_alice", "success", "Alice Submits (Leg 1/2)", fmt.Sprintf("SwapInitiated on-chain (block %d) — waiting for Bob's leg, deadline in %ds", aliceTxReceipt.BlockNumber, deadlineWindowSeconds))
	fs.done["settle_alice"] = true
}

// ── Step 9: Bob submits leg 2 — atomic settlement ───────────────────────────

func stepSettleBob(fs *FlowState) {
	b := fs.broker
	ctx := fs.ctx
	client := fs.client
	emitStep(b, "settle_bob", "running", "Bob Submits (Leg 2/2)", "Bob submits his ERC721 leg — settles the swap atomically…")

	bobTx, err := fs.dvp.Transact(fs.auth, "submitPartialSettlement", fs.bobReceipt, big.NewInt(vaultIdErc721), big.NewInt(groupNonFung), big.NewInt(0))
	if err != nil {
		emitStep(b, "settle_bob", "error", "Bob Submits (Leg 2/2)", fmt.Errorf("submitPartialSettlement: %w", err).Error())
		return
	}
	bobTxReceipt, err := bind.WaitMined(ctx, client, bobTx)
	if err != nil {
		emitStep(b, "settle_bob", "error", "Bob Submits (Leg 2/2)", fmt.Errorf("wait submitPartialSettlement: %w", err).Error())
		return
	}
	fs.bobTxReceipt = bobTxReceipt
	fs.totalGasUsed += bobTxReceipt.GasUsed
	protoSet(b, "bobTxHash", bobTx.Hash().Hex())
	protoSet(b, "bobBlock", fmt.Sprintf("%d", bobTxReceipt.BlockNumber))
	protoSet(b, "bobGas", fmt.Sprintf("%d", bobTxReceipt.GasUsed))
	protoSet(b, "totalGas", fmt.Sprintf("%d", fs.totalGasUsed))
	logMsg(b, "chain", fmt.Sprintf("EnygmaDvp.submitPartialSettlement() [Bob] → block %d  gas %s  tx %s…",
		bobTxReceipt.BlockNumber, formatNum(bobTxReceipt.GasUsed), shortHex(bobTx.Hash().Hex(), 18)))
	emitStep(b, "settle_bob", "success", "Bob Submits (Leg 2/2)", fmt.Sprintf("Swap settled atomically on-chain (block %d)", bobTxReceipt.BlockNumber))
	fs.done["settle_bob"] = true
}

// ── Step 10: Verify ───────────────────────────────────────────────────────────

func stepVerify(fs *FlowState) {
	b := fs.broker
	emitStep(b, "verify", "running", "Verify Settlement", "Checking on-chain events confirm both sides settled…")
	pause(500 * time.Millisecond)

	commitmentSig := crypto.Keccak256Hash([]byte("Commitment(uint256,uint256)"))
	nullifierSig := crypto.Keccak256Hash([]byte("Nullifier(uint256,uint256,uint256)"))
	foundCommitA, foundCommitB, nullifierCount := false, false, 0
	for _, l := range fs.bobTxReceipt.Logs {
		if len(l.Topics) == 0 {
			continue
		}
		switch l.Topics[0] {
		case commitmentSig:
			if len(l.Topics) < 3 {
				continue
			}
			cmt := l.Topics[2].Big()
			if cmt.Cmp(fs.initiatorResult.CommitB) == 0 {
				foundCommitB = true
			} else if cmt.Cmp(fs.initiatorResult.CommitA) == 0 {
				foundCommitA = true
			}
		case nullifierSig:
			nullifierCount++
		}
	}
	if !foundCommitA || !foundCommitB {
		emitStep(b, "verify", "error", "Verify Settlement", fmt.Errorf("expected commitments not found in settlement events (commitA=%v commitB=%v)", foundCommitA, foundCommitB).Error())
		return
	}
	logMsg(b, "chain", fmt.Sprintf("✓ commitB inserted (Bob's USDT note) · commitA inserted (Alice's ticket note) · %d nullifier(s) spent", nullifierCount))
	panelSet(b, 1, "balance", fmt.Sprintf("%d USDT (received)", erc20Amount))
	panelSet(b, 0, "balance", fmt.Sprintf("ticket #%d (received)", fs.nftTokenIdInt))
	emitStep(b, "verify", "success", "Verify Settlement", "Both commitments confirmed on-chain — swap complete")
	fs.done["verify"] = true

	flowMs := time.Since(fs.flowStart).Milliseconds()
	protoSet(b, "flowTimeMs", fmt.Sprintf("%d", flowMs))
	logMsg(b, "", fmt.Sprintf("✓ Total flow: %d ms  total protocol gas: %s", flowMs, formatNum(fs.totalGasUsed)))
	b.publish(Event{
		Type:   "done",
		Status: "success",
		Msg:    fmt.Sprintf("Swap complete: Alice's %d USDT ↔ Bob's ticket #%d", erc20Amount, fs.nftTokenIdInt),
	})
}

func init() {
	stepFns = map[string]func(*FlowState){
		"prereqs":         stepPrereqs,
		"keygen":          stepKeygen,
		"deposit_erc20":   stepDepositErc20,
		"deposit_nft":     stepDepositNft,
		"dvp_initiator":   stepDvpInitiator,
		"bob_scan":        stepBobScan,
		"dvp_destination": stepDvpDestination,
		"settle_alice":    stepSettleAlice,
		"settle_bob":      stepSettleBob,
		"verify":          stepVerify,
	}
}

// ── Entry point ───────────────────────────────────────────────────────────────

func main() {
	broker := newBroker()
	srv := &Server{broker: broker, fs: newFlowState(broker)}

	mux := http.NewServeMux()
	mux.HandleFunc("/", srv.handleIndex)
	mux.HandleFunc("/events", srv.handleEvents)
	mux.HandleFunc("/step/", srv.handleStep)
	mux.HandleFunc("/reset", srv.handleReset)

	log.Printf("Enygma DvP · NFT ↔ ERC20 Swap Demo (step-by-step) listening on http://localhost%s", listenAddr)
	log.Printf("Open http://localhost%s in your browser", listenAddr)
	if err := http.ListenAndServe(listenAddr, mux); err != nil {
		log.Fatal(err)
	}
}
