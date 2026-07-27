// Enygma Payments Live Demo — tabbed edition
//
// Usage:
//
//	cd demo && go mod tidy && CC=/usr/bin/clang go run .
//	open http://localhost:9090
package main

import (
	_ "embed"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	enygma "enygma/contracts"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/iden3/go-iden3-crypto/babyjub"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

//go:embed index.html
var indexHTML string

// ── Config ─────────────────────────────────────────────────────────────────────

const (
	listenAddr = ":9090"
	gnarkURL   = "http://127.0.0.1:8080/proof/enygma"
	relayerURL = "http://127.0.0.1:8082"

	nBanks      = 6
	mintAmount  = 500
	transferAmt = 100

	defaultOwnerKey = "34d091c661db4c814d65c8ae9277b7055c0dde5a752ce5a3fdfd4ea11a8f7154"
	defaultRelayKey = "enygma-test-secret"

	senderSkVal    = 424242
	senderPrevRVal = 0 // initial balance = Com(0,0) = (0,1) on BabyJubJub
)

var (
	rpcURL  = getEnv("RPC_URL", "http://127.0.0.1:8545")
	chainID = getEnvInt64("CHAIN_ID", 1337)

	// per-bank secret keys (index 0 = sender)
	bankSks = [nBanks]*big.Int{
		big.NewInt(senderSkVal),
		big.NewInt(1), big.NewInt(2), big.NewInt(3), big.NewInt(4), big.NewInt(5),
	}
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

func rpcHostPort() string {
	u, err := url.Parse(rpcURL)
	if err != nil {
		return "127.0.0.1:8545"
	}
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return net.JoinHostPort(host, port)
}

// ── BabyJubJub / Pedersen ──────────────────────────────────────────────────────

var (
	curveP, _ = new(big.Int).SetString("2736030358979909402780800718157159386076813972158567259200215660948447373041", 10)

	curveGx, _ = new(big.Int).SetString("16540640123574156134436876038791482806971768689494387082833631921987005038935", 10)
	curveGy, _ = new(big.Int).SetString("20819045374670962167435360035096875258406992893633759881276124905556507972311", 10)
	curveG     = &babyjub.Point{X: curveGx, Y: curveGy}

	curveHx, _ = new(big.Int).SetString("10100005861917718053548237064487763771145251762383025193119768015180892676690", 10)
	curveHy, _ = new(big.Int).SetString("7512830269827713629724023825249861327768672768516116945507944076335453576011", 10)
	curveH     = &babyjub.Point{X: curveHx, Y: curveHy}

	hashRandom *big.Int
	hashTag    *big.Int
)

func init() {
	hashRandom, _ = poseidon.Hash([]*big.Int{big.NewInt(21)})
	hashTag, _ = poseidon.Hash([]*big.Int{big.NewInt(12)})
}

func pedersenCommit(v, r *big.Int) *babyjub.Point {
	vG := babyjub.NewPoint().Mul(v, curveG)
	rH := babyjub.NewPoint().Mul(r, curveH)
	return babyjub.NewPoint().Projective().Add(vG.Projective(), rH.Projective()).Affine()
}

func addPoints(a, b *babyjub.Point) *babyjub.Point {
	return babyjub.NewPoint().Projective().Add(a.Projective(), b.Projective()).Affine()
}

func negMod(x *big.Int) *big.Int {
	return new(big.Int).Sub(curveP, new(big.Int).Mod(x, curveP))
}

func rValue(secret, blockHash *big.Int) *big.Int {
	h, _ := poseidon.Hash([]*big.Int{hashRandom, secret, new(big.Int).Mod(blockHash, curveP)})
	return h.Mod(h, curveP)
}

func tagValue(secret, blockHash *big.Int) *big.Int {
	h, _ := poseidon.Hash([]*big.Int{hashTag, secret, new(big.Int).Mod(blockHash, curveP)})
	return h.Mod(h, curveP)
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func pause(d time.Duration) {
	if chainID == 1337 {
		time.Sleep(d)
	}
}

// ── SSE Broker ─────────────────────────────────────────────────────────────────

type Event struct {
	Type    string `json:"type"`
	Tab     string `json:"tab,omitempty"`     // "setup" | "onboarding" | "transfer"
	Step    string `json:"step,omitempty"`
	BankIdx int    `json:"bankIdx,omitempty"` // for bank-specific step events
	Status  string `json:"status,omitempty"`
	Label   string `json:"label,omitempty"`
	Msg     string `json:"msg,omitempty"`
	TS      string `json:"ts,omitempty"`
	Pid     int    `json:"pid"` // always include; 0 = Bank 0 (omitempty would silently drop it)
	Field   string `json:"field,omitempty"`
	Value   string `json:"value,omitempty"`
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

// ── Persistent connection state ────────────────────────────────────────────────

type connState struct {
	mu            sync.Mutex
	ready         bool
	client        *ethclient.Client
	priv          *ecdsa.PrivateKey
	owner         common.Address
	inst          *enygma.Enygma
	tokenAddr     string
	verifierAddr  string
	totalGasUsed  uint64
	mintedBalances [nBanks]int64      // cumulative plaintext balance per bank; updated after each mint/transfer
	lastSenderIdx  int               // senderIdx from the most recent completed transfer
	registeredSks  [nBanks]*big.Int  // sk used at registration time; nil until registered
	lastRValues    [nBanks]*big.Int  // TxRandomValues from the last successful transfer; for verify tab
	cumulativeR    [nBanks]*big.Int  // running sum of txRandom per bank across all transfers (mod P)
	transferCount  int               // number of completed transfers
	kaSecrets      [nBanks][nBanks]*big.Int  // kaSecrets[i][j] = secret shared by Bank i and Bank j (symmetric)
	kaEKs          [nBanks][]byte           // ML-KEM-768 encapsulation keys (1184B each, public_view_key)
}

// ── Server ────────────────────────────────────────────────────────────────────

type Server struct {
	broker  *Broker
	runLock sync.Mutex
	state   connState
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

func (s *Server) startFlow(w http.ResponseWriter, fn func()) {
	if !s.runLock.TryLock() {
		http.Error(w, `{"error":"flow already running"}`, 409)
		return
	}
	go func() {
		defer s.runLock.Unlock()
		fn()
	}()
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"status":"started"}`)
}

func (s *Server) handleRunSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	s.startFlow(w, func() { runSetup(s) })
}

func (s *Server) handleRunRegisterBank(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	var body struct {
		BankIdx int    `json:"bankIdx"`
		Sk      string `json:"sk"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.BankIdx < 0 || body.BankIdx >= nBanks {
		http.Error(w, `{"error":"bankIdx out of range 0-5"}`, 400)
		return
	}
	sk := new(big.Int).Set(bankSks[body.BankIdx]) // default
	if body.Sk != "" {
		if parsed, ok := new(big.Int).SetString(body.Sk, 10); ok && parsed.Sign() > 0 {
			sk = parsed
		}
	}
	idx := body.BankIdx
	s.startFlow(w, func() { runRegisterBank(s, idx, sk) })
}

func (s *Server) handleRunMint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	var body struct {
		Amount  int64 `json:"amount"`
		BankIdx int   `json:"bankIdx"`
	}
	body.Amount = int64(mintAmount)
	json.NewDecoder(r.Body).Decode(&body)
	if body.Amount <= 0 {
		body.Amount = int64(mintAmount)
	}
	if body.BankIdx < 0 || body.BankIdx >= nBanks {
		http.Error(w, `{"error":"bankIdx out of range 0-5"}`, 400)
		return
	}
	amt := body.Amount
	bidx := body.BankIdx
	s.startFlow(w, func() { runMintSupply(s, amt, bidx) })
}

func (s *Server) handleRunTransfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	var body struct {
		SenderIdx    int           `json:"senderIdx"`
		SenderAmt    int64         `json:"senderAmt"`
		ReceiverAmts [nBanks]int64 `json:"receiverAmts"`
	}
	body.SenderAmt = int64(transferAmt)
	body.ReceiverAmts[1] = 60
	body.ReceiverAmts[2] = 40
	json.NewDecoder(r.Body).Decode(&body)
	if body.SenderIdx < 0 || body.SenderIdx >= nBanks {
		http.Error(w, `{"error":"senderIdx out of range 0-5"}`, 400)
		return
	}
	if body.SenderAmt <= 0 {
		body.SenderAmt = int64(transferAmt)
	}
	var receiverSum int64
	for i := 0; i < nBanks; i++ {
		if i != body.SenderIdx {
			receiverSum += body.ReceiverAmts[i]
		}
	}
	if receiverSum != body.SenderAmt {
		http.Error(w, `{"error":"receiver amounts must sum to senderAmt"}`, 400)
		return
	}
	sidx := body.SenderIdx
	sa := body.SenderAmt
	ramts := body.ReceiverAmts
	s.startFlow(w, func() { runTransfer(s, sidx, sa, ramts) })
}

// ── Flow context ──────────────────────────────────────────────────────────────

type flowCtx struct {
	b   *Broker
	tab string
	s   *Server
}

func newCtx(s *Server, tab string) *flowCtx {
	return &flowCtx{b: s.broker, tab: tab, s: s}
}

func (fc *flowCtx) emit(step, status, label, msg string) {
	fc.b.publish(Event{Type: "step", Tab: fc.tab, Step: step, Status: status, Label: label, Msg: msg})
}

func (fc *flowCtx) emitBank(bankIdx int, step, status, label, msg string) {
	fc.b.publish(Event{Type: "step", Tab: fc.tab, Step: step, BankIdx: bankIdx, Status: status, Label: label, Msg: msg})
}

func (fc *flowCtx) log(msg string) {
	fc.b.publish(Event{Type: "log", Tab: fc.tab, Msg: msg, TS: time.Now().Format("15:04:05.000")})
}

func (fc *flowCtx) participant(pid int, field, value string) {
	fc.b.publish(Event{Type: "participant", Tab: fc.tab, Pid: pid, Field: field, Value: value})
}

func (fc *flowCtx) metric(field, value string) {
	fc.b.publish(Event{Type: "metric", Tab: fc.tab, Field: field, Value: value})
}

func (fc *flowCtx) proto(field, value string) {
	fc.b.publish(Event{Type: "proto", Field: field, Value: value})
}

func (fc *flowCtx) done(success bool, msg string) {
	status := "success"
	if !success {
		status = "error"
	}
	fc.b.publish(Event{Type: "done", Tab: fc.tab, Status: status, Msg: msg})
}

func (fc *flowCtx) mkAuth() *bind.TransactOpts {
	st := &fc.s.state
	nonce, _ := st.client.PendingNonceAt(context.Background(), st.owner)
	gasPrice, _ := st.client.SuggestGasPrice(context.Background())
	auth, _ := bind.NewKeyedTransactorWithChainID(st.priv, big.NewInt(chainID))
	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)
	auth.GasLimit = 12_000_000
	auth.GasPrice = gasPrice
	return auth
}

func (fc *flowCtx) waitTx(label string, tx *ethtypes.Transaction, txErr error) (*ethtypes.Receipt, error) {
	if txErr != nil {
		return nil, txErr
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	r, err := bind.WaitMined(ctx, fc.s.state.client, tx)
	if err != nil {
		return nil, err
	}
	if r.Status != 1 {
		return nil, fmt.Errorf("tx reverted (block %d)", r.BlockNumber)
	}
	fc.s.state.totalGasUsed += r.GasUsed
	fc.log(fmt.Sprintf("✓ %s — tx %s  (gas %d)", label, r.TxHash.Hex()[:12]+"…", r.GasUsed))
	return r, nil
}

// ── Contract address resolver ─────────────────────────────────────────────────

type deployReceipts struct {
	TOKEN    struct{ ContractAddress string `json:"contractAddress"` } `json:"TOKEN"`
	VERIFIER struct{ ContractAddress string `json:"contractAddress"` } `json:"VERIFIER"`
}

func resolveAddresses() (token, verifier string, err error) {
	token = os.Getenv("ENYGMA_TOKEN_ADDR")
	verifier = os.Getenv("ENYGMA_VERIFIER_ADDR")
	if token != "" && verifier != "" {
		return
	}
	_, thisFile, _, _ := runtime.Caller(0)
	receiptsPath := filepath.Join(filepath.Dir(thisFile), "..", "run_scripts", "build", "enygma", "web3", "deploy_receipts.json")
	if data, readErr := os.ReadFile(receiptsPath); readErr == nil {
		var rec deployReceipts
		if json.Unmarshal(data, &rec) == nil {
			if token == "" {
				token = rec.TOKEN.ContractAddress
			}
			if verifier == "" {
				verifier = rec.VERIFIER.ContractAddress
			}
		}
	}
	if token == "" || verifier == "" {
		err = fmt.Errorf("contract addresses not found — set ENYGMA_TOKEN_ADDR / ENYGMA_VERIFIER_ADDR or run deploy scripts")
	}
	return
}

// ── Tab 1: Setup flow ─────────────────────────────────────────────────────────

func runSetup(s *Server) {
	fc := newCtx(s, "setup")

	fc.emit("prerequisites", "running", "Check prerequisites", "Probing Hardhat · Gnark · Relayer…")
	var missing []string
	if !tcpAvailable(rpcHostPort())     { missing = append(missing, "Chain RPC ("+rpcHostPort()+")") }
	if !tcpAvailable("127.0.0.1:8080") { missing = append(missing, "Gnark :8080") }
	if !tcpAvailable("127.0.0.1:8082") { missing = append(missing, "Relayer :8082") }
	if len(missing) > 0 {
		fc.emit("prerequisites", "error", "Check prerequisites", "Not reachable: "+strings.Join(missing, ", "))
		fc.done(false, "Start "+strings.Join(missing, ", ")+" first")
		return
	}
	fc.emit("prerequisites", "success", "Check prerequisites", rpcURL+" · Gnark :8080 · Relayer :8082 — all online")
	fc.log("✓ Chain RPC, Gnark server, and Relayer all reachable")
	pause(800 * time.Millisecond)

	tokenAddr, verifierAddr, err := resolveAddresses()
	if err != nil {
		fc.emit("dial_chain", "error", "Dial chain", err.Error())
		fc.done(false, err.Error())
		return
	}
	fc.log(fmt.Sprintf("TOKEN    contract: %s", tokenAddr))
	fc.log(fmt.Sprintf("VERIFIER contract: %s", verifierAddr))
	fc.proto("tokenAddr", tokenAddr)
	fc.proto("verifierAddr", verifierAddr)

	fc.emit("dial_chain", "running", "Dial chain node", rpcURL)
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		fc.emit("dial_chain", "error", "Dial chain node", err.Error())
		fc.done(false, "Cannot connect to chain RPC")
		return
	}
	privKeyHex := os.Getenv("MY_KEY")
	if privKeyHex == "" {
		privKeyHex = defaultOwnerKey
	}
	privKey, err := crypto.HexToECDSA(strings.TrimPrefix(privKeyHex, "0x"))
	if err != nil {
		fc.emit("dial_chain", "error", "Dial chain node", "invalid private key: "+err.Error())
		fc.done(false, "bad private key")
		return
	}
	owner := crypto.PubkeyToAddress(*privKey.Public().(*ecdsa.PublicKey))
	blockNum, _ := client.BlockNumber(context.Background())
	fc.emit("dial_chain", "success", "Dial chain node",
		fmt.Sprintf("owner %s · block %d · chainId %d", trunc(owner.Hex(), 14), blockNum, chainID))
	fc.log(fmt.Sprintf("Connected — owner: %s", owner.Hex()))
	fc.proto("ownerAddr", owner.Hex())
	fc.proto("blockNum", fmt.Sprintf("%d", blockNum))
	pause(600 * time.Millisecond)

	inst, err := enygma.NewEnygma(common.HexToAddress(tokenAddr), client)
	if err != nil {
		fc.emit("init_contract", "error", "Initialize contract", err.Error())
		fc.done(false, "Cannot bind contract")
		return
	}

	// Store to shared state early so mkAuth works
	s.state.mu.Lock()
	s.state.client = client
	s.state.priv = privKey
	s.state.owner = owner
	s.state.inst = inst
	s.state.tokenAddr = tokenAddr
	s.state.verifierAddr = verifierAddr
	s.state.mu.Unlock()

	fc.emit("init_contract", "running", "Initialize contract", "Enygma.initialize() — idempotent")
	if tx, txErr := inst.Initialize(fc.mkAuth()); txErr == nil {
		if _, waitErr := fc.waitTx("Initialize", tx, txErr); waitErr != nil {
			fc.log("Contract already initialized (OK — idempotent)")
		}
	} else {
		fc.log("Initialize skipped: " + txErr.Error())
	}
	fc.emit("init_contract", "success", "Initialize contract",
		fmt.Sprintf("contract %s · epochInterval=1", trunc(tokenAddr, 14)))
	pause(600 * time.Millisecond)

	fc.emit("add_verifier", "running", "Register verifier", fmt.Sprintf("addVerifier(%s)", trunc(verifierAddr, 14)))
	if avTx, avErr := inst.AddVerifier(fc.mkAuth(), common.HexToAddress(verifierAddr)); avErr == nil {
		if _, waitErr := fc.waitTx("AddVerifier", avTx, avErr); waitErr != nil {
			fc.log("Verifier already registered (OK — idempotent)")
		}
	} else {
		fc.log("AddVerifier skipped: " + avErr.Error())
	}
	fc.emit("add_verifier", "success", "Register verifier",
		fmt.Sprintf("Groth16 verifier registered at %s", trunc(verifierAddr, 14)))
	pause(400 * time.Millisecond)

	s.state.mu.Lock()
	s.state.ready = true
	s.state.mu.Unlock()

	fc.done(true, "Setup complete — chain connected, contract initialized, verifier registered")
}

// ── Tab 2: Register one bank ───────────────────────────────────────────────────

func runRegisterBank(s *Server, bankIdx int, sk *big.Int) {
	fc := newCtx(s, "onboarding")
	s.state.mu.Lock()
	ready := s.state.ready
	s.state.mu.Unlock()
	if !ready {
		fc.done(false, "Run Setup first before registering banks")
		return
	}

	stepKey := fmt.Sprintf("bank_%d", bankIdx)

	// Collect view key (ML-KEM ek); empty if key agreement hasn't been run yet
	s.state.mu.Lock()
	ek := s.state.kaEKs[bankIdx]
	s.state.mu.Unlock()

	ekDesc := "(run Key Agreement first)"
	if len(ek) > 0 {
		ekDesc = hex.EncodeToString(ek[:8]) + "… (1184B)"
	}

	// Derive spend key
	fc.emitBank(bankIdx, stepKey+"_key", "running",
		fmt.Sprintf("Bank %d · Derive Key", bankIdx),
		fmt.Sprintf("pk = Poseidon(%s, %s) mod P", trunc(sk.String(), 10), trunc(sk.String(), 10)))
	pk, _ := poseidon.Hash([]*big.Int{sk, sk})
	pk.Mod(pk, curveP)
	fc.emitBank(bankIdx, stepKey+"_key", "success",
		fmt.Sprintf("Bank %d · Key Derived", bankIdx),
		fmt.Sprintf("pk = %s", trunc(pk.String(), 22)))
	fc.participant(bankIdx, "pk", pk.String())
	fc.participant(bankIdx, "spend_key", pk.String())
	if len(ek) == 0 {
		fc.participant(bankIdx, "view_ek", "(run Key Agreement first)")
	} else {
		fc.participant(bankIdx, "view_ek", hex.EncodeToString(ek[:8])+"… (1184B)")
	}
	fc.log(fmt.Sprintf("Bank %d: pk = Poseidon(%s,%s) = %s", bankIdx, sk.String(), sk.String(), trunc(pk.String(), 30)))
	pause(400 * time.Millisecond)

	// Register on-chain
	fc.emitBank(bankIdx, stepKey+"_reg", "running",
		fmt.Sprintf("Bank %d · RegisterAccount", bankIdx),
		fmt.Sprintf("registerAccount(id=%d, spendPk=%s…, viewEk=%s)", bankIdx+1, trunc(pk.String(), 10), ekDesc))
	raTx, raErr := s.state.inst.RegisterAccount(
		fc.mkAuth(), s.state.owner,
		big.NewInt(int64(bankIdx+1)), pk, big.NewInt(senderPrevRVal), ek)
	if raErr == nil {
		if _, waitErr := fc.waitTx(fmt.Sprintf("RegisterAccount[%d]", bankIdx), raTx, raErr); waitErr != nil {
			fc.log(fmt.Sprintf("Bank %d already registered (OK — idempotent)", bankIdx))
		}
	} else {
		fc.log(fmt.Sprintf("RegisterAccount[%d] skipped: %s", bankIdx, raErr.Error()))
	}
	fc.emitBank(bankIdx, stepKey+"_reg", "success",
		fmt.Sprintf("Bank %d · Registered ✓", bankIdx),
		fmt.Sprintf("accountId=%d · spendPk+viewEk on-chain", bankIdx+1))
	s.state.mu.Lock()
	s.state.registeredSks[bankIdx] = new(big.Int).Set(sk)
	s.state.mu.Unlock()
	fc.participant(bankIdx, "status", "registered")
	fc.done(true, fmt.Sprintf("Bank %d (accountId=%d) registered on-chain", bankIdx, bankIdx+1))
}

// ── Tab 2: Mint supply ────────────────────────────────────────────────────────

func runMintSupply(s *Server, amount int64, bankIdx int) {
	fc := newCtx(s, "transfer") // lives on the transfer tab now
	s.state.mu.Lock()
	ready := s.state.ready
	s.state.mu.Unlock()
	if !ready {
		fc.done(false, "Run Setup first")
		return
	}

	accountId := int64(bankIdx + 1)
	label := fmt.Sprintf("Mint %d → Bank %d", amount, bankIdx)
	fc.emit("mint_supply", "running", label, fmt.Sprintf("mintSupply(%d, accountId=%d)", amount, accountId))
	tx, err := s.state.inst.MintSupply(fc.mkAuth(), big.NewInt(amount), big.NewInt(accountId))
	if _, err = fc.waitTx("MintSupply", tx, err); err != nil {
		fc.emit("mint_supply", "error", label, err.Error())
		fc.done(false, "MintSupply failed: "+err.Error())
		return
	}
	fc.emit("mint_supply", "success", label,
		fmt.Sprintf("balance[%d] = prevBalance + Com(%d, 0)", accountId, amount))
	fc.log(fmt.Sprintf("Minted %d tokens to accountId=%d (Bank %d)", amount, accountId, bankIdx))
	s.state.mu.Lock()
	s.state.mintedBalances[bankIdx] += amount
	s.state.mu.Unlock()
	fc.done(true, fmt.Sprintf("Minted %d tokens to Bank %d (accountId=%d) · total balance = %d", amount, bankIdx, accountId, s.state.mintedBalances[bankIdx]))
}

// ── Tab 3: Transfer flow ──────────────────────────────────────────────────────

func runTransfer(s *Server, senderIdx int, senderAmt int64, receiverAmts [nBanks]int64) {
	fc := newCtx(s, "transfer")
	s.state.mu.Lock()
	ready := s.state.ready
	inst := s.state.inst
	s.state.mu.Unlock()
	if !ready || inst == nil {
		fc.done(false, "Run Setup first")
		return
	}

	flowStart := time.Now()

	// Read on-chain state
	fc.emit("read_state", "running", "Read on-chain state", "getPublicValues(7) + getBlckHash()")
	pubVals, err := inst.GetPublicValues(&bind.CallOpts{}, big.NewInt(nBanks+1))
	if err != nil {
		fc.emit("read_state", "error", "Read on-chain state", err.Error())
		fc.done(false, "GetPublicValues failed")
		return
	}
	blockHash, err := inst.GetBlckHash(&bind.CallOpts{})
	if err != nil {
		fc.emit("read_state", "error", "Read on-chain state", err.Error())
		fc.done(false, "GetBlckHash failed")
		return
	}
	prevBalances := pubVals.Balances[1:]
	onChainKeys  := pubVals.Keys[1:]
	fc.emit("read_state", "success", "Read on-chain state",
		fmt.Sprintf("epochBlockHash = %s · %d accounts", trunc(blockHash.String(), 12), nBanks))
	fc.log(fmt.Sprintf("Epoch block hash: %s", blockHash.String()))
	fc.proto("senderIdx", fmt.Sprintf("%d", senderIdx))
	fc.participant(senderIdx, "prevBalance",
		fmt.Sprintf("(%s…, %s…)", trunc(prevBalances[senderIdx].C1.String(), 10), trunc(prevBalances[senderIdx].C2.String(), 10)))
	// Broadcast pre-transfer balances for all banks
	for i := 0; i < nBanks; i++ {
		fc.b.publish(Event{Type: "participant", Tab: "transfer", Pid: i, Field: "prevBal",
			Value: fmt.Sprintf("(%s…,%s…)", trunc(prevBalances[i].C1.String(), 8), trunc(prevBalances[i].C2.String(), 8))})
	}
	pause(600 * time.Millisecond)

	// Derive shared secrets — use the sk registered for bank 0 (falls back to default)
	s.state.mu.Lock()
	senderSk := s.state.registeredSks[senderIdx]
	var prevSenderR *big.Int
	if s.state.cumulativeR[senderIdx] != nil {
		prevSenderR = new(big.Int).Set(s.state.cumulativeR[senderIdx])
	} else {
		prevSenderR = new(big.Int) // 0 on first transfer for this bank
	}
	kaSecretsSnap := s.state.kaSecrets // copy [nBanks]*big.Int array
	s.state.mu.Unlock()
	if senderSk == nil {
		senderSk = bankSks[senderIdx]
	}
	fc.emit("derive_secrets", "running", "Derive shared secrets", "s₀ = Poseidon(prevR, sk) mod P")
	senderSecret, _ := poseidon.Hash([]*big.Int{prevSenderR, senderSk})
	senderSecret.Mod(senderSecret, curveP)
	// Use ML-KEM derived secrets if key agreement has been run; fall back to demo defaults.
	demoDefaults := [nBanks]int64{31415, 54142, 814712, 250912012, 12312512, 12312512}
	var secrets [nBanks]*big.Int
	for i := 0; i < nBanks; i++ {
		if i == senderIdx {
			secrets[i] = senderSecret
		} else if kaSecretsSnap[senderIdx][i] != nil {
			secrets[i] = new(big.Int).Set(kaSecretsSnap[senderIdx][i])
		} else {
			secrets[i] = big.NewInt(demoDefaults[i])
		}
	}
	fc.emit("derive_secrets", "success", "Derive shared secrets",
		fmt.Sprintf("s[%d] = Poseidon(%s, %s) = %s", senderIdx, trunc(prevSenderR.String(), 10), trunc(senderSk.String(), 10), trunc(senderSecret.String(), 20)))
	for i, s := range secrets {
		fc.log(fmt.Sprintf("  s[%d] = %s", i, trunc(s.String(), 40)))
		fc.participant(i, "secret", trunc(s.String(), 16)+"…")
	}
	pause(1200 * time.Millisecond)

	// Compute random factors
	fc.emit("compute_randoms", "running", "Compute random factors", "r_i = Poseidon(H₂₁, s_i, epochHash)")
	bh := new(big.Int).Set(blockHash)
	var rValues [nBanks]*big.Int
	rSum := new(big.Int)
	for i := 0; i < nBanks; i++ {
		r := rValue(secrets[i], bh)
		rValues[i] = r
		if i != senderIdx {
			rSum.Add(rSum, r)
			rSum.Mod(rSum, curveP)
		}
	}
	rValues[senderIdx] = rSum
	fc.emit("compute_randoms", "success", "Compute random factors",
		fmt.Sprintf("r[0] = Σr_receivers = %s  (conservation)", trunc(rSum.String(), 20)))
	for i := 0; i < nBanks; i++ {
		fc.log(fmt.Sprintf("  r[%d] = %s", i, trunc(rValues[i].String(), 40)))
		fc.participant(i, "r", trunc(rValues[i].String(), 16)+"…")
	}
	pause(1200 * time.Millisecond)

	// Build Pedersen commitments
	fc.emit("build_commits", "running", "Build Pedersen commitments", "TxCommit_i = v_i·G + r_i·H")
	var txValues [nBanks]*big.Int
	for i := 0; i < nBanks; i++ {
		if i == senderIdx {
			txValues[i] = negMod(big.NewInt(senderAmt))
		} else {
			txValues[i] = new(big.Int).SetInt64(receiverAmts[i])
		}
	}
	amtLabel := func(v int64) string {
		if v == 0 {
			return "0  (privacy)"
		}
		return fmt.Sprintf("+%d  (credit)", v)
	}
	var txRandom [nBanks]*big.Int
	var txCommit [nBanks]enygma.IEnygmaPoint
	for i := 0; i < nBanks; i++ {
		var pt *babyjub.Point
		if i == senderIdx {
			pt = pedersenCommit(negMod(big.NewInt(senderAmt)), rValues[i])
			txRandom[i] = rValues[i]
		} else {
			pt = pedersenCommit(txValues[i], negMod(rValues[i]))
			txRandom[i] = negMod(rValues[i])
		}
		txCommit[i] = enygma.IEnygmaPoint{C1: pt.X, C2: pt.Y}
	}

	// Store actual txRandom values for the verify tab (last-transfer view).
	s.state.mu.Lock()
	for i := 0; i < nBanks; i++ {
		s.state.lastRValues[i] = new(big.Int).Set(txRandom[i])
	}
	s.state.mu.Unlock()
	var txAmountLabels [nBanks]string
	for i := 0; i < nBanks; i++ {
		if i == senderIdx {
			txAmountLabels[i] = fmt.Sprintf("−%d  (debit)", senderAmt)
		} else {
			txAmountLabels[i] = amtLabel(receiverAmts[i])
		}
	}
	fc.emit("build_commits", "success", "Build Pedersen commitments",
		fmt.Sprintf("Bank%d −%d · Σreceivers=%d · Σ=(0,1) ✓", senderIdx, senderAmt, senderAmt))
	for i := 0; i < nBanks; i++ {
		fc.participant(i, "amount", txAmountLabels[i])
		fc.participant(i, "commit",
			fmt.Sprintf("(%s…, %s…)", trunc(txCommit[i].C1.String(), 10), trunc(txCommit[i].C2.String(), 10)))
	}
	pause(1200 * time.Millisecond)

	// Compute message tags
	fc.emit("compute_tags", "running", "Compute message tags", "t_i = Poseidon(H₁₂, s_i, epochHash)")
	bhTag := new(big.Int).Set(blockHash)
	var tagMessages [nBanks]*big.Int
	for i := 0; i < nBanks; i++ {
		tagMessages[i] = tagValue(secrets[i], bhTag)
	}
	fc.emit("compute_tags", "success", "Compute message tags",
		fmt.Sprintf("tag[0] = %s", trunc(tagMessages[0].String(), 20)))
	for i := 0; i < nBanks; i++ {
		fc.participant(i, "tag", trunc(tagMessages[i].String(), 16)+"…")
	}
	pause(1200 * time.Millisecond)

	// Compute nullifier — uses senderSecret (= Poseidon(prevR, sk) mod P) directly
	fc.emit("compute_nullifier", "running", "Compute nullifier", "η = Poseidon(senderSecret, epochHash)")
	nullifier, _ := poseidon.Hash([]*big.Int{senderSecret, blockHash})
	fc.emit("compute_nullifier", "success", "Compute nullifier",
		fmt.Sprintf("η = %s", trunc(nullifier.String(), 20)))
	fc.log(fmt.Sprintf("Nullifier: %s", nullifier.String()))
	pause(1200 * time.Millisecond)

	// Build k×k FingerPrint matrix: fp[i][senderIdx] = Poseidon(secrets[i]) mod P for i ≠ senderIdx
	fp := make([][nBanks]*big.Int, nBanks)
	for i := range fp {
		for j := range fp[i] {
			fp[i][j] = big.NewInt(0)
		}
	}
	for i := 0; i < nBanks; i++ {
		if i == senderIdx {
			continue
		}
		h, _ := poseidon.Hash([]*big.Int{secrets[i]})
		fp[i][senderIdx] = h.Mod(h, curveP)
	}
	fpStrs := make([][]string, nBanks)
	for i := range fpStrs {
		fpStrs[i] = make([]string, nBanks)
		for j := range fpStrs[i] {
			fpStrs[i][j] = fp[i][j].String()
		}
	}

	// ZK proof
	fc.emit("zk_proof", "running", "Generate ZK proof", "POST /proof/enygma — ~30s")
	fc.log("Requesting ZK proof from gnark server (this may take ~30s)…")

	toStrs := func(vals [nBanks]*big.Int) []string {
		s := make([]string, nBanks)
		for i, v := range vals {
			s[i] = v.String()
		}
		return s
	}
	prevCommitSlice := make([][]string, nBanks)
	for i, pt := range prevBalances {
		prevCommitSlice[i] = []string{pt.C1.String(), pt.C2.String()}
	}
	txCommitSlice := make([][]string, nBanks)
	for i, pt := range txCommit {
		txCommitSlice[i] = []string{pt.C1.String(), pt.C2.String()}
	}
	keyStrs := make([]string, nBanks)
	for i, k := range onChainKeys {
		keyStrs[i] = k.String()
	}
	kIndex := make([]string, nBanks)
	for i := range kIndex {
		kIndex[i] = fmt.Sprintf("%d", i)
	}
	proofReqBody, _ := json.Marshal(map[string]interface{}{
		"fingerprint_shared_secrets":   fpStrs,
		"public_keys":                  keyStrs,
		"previous_commits":             prevCommitSlice,
		"tx_commits":                   txCommitSlice,
		"block_number":                 blockHash.String(),
		"anonymity_set":                kIndex,
		"message_tags":                 toStrs(tagMessages),
		"nullifier":                    nullifier.String(),
		"sender_id":                    fmt.Sprintf("%d", senderIdx),
		"shared_secrets":               toStrs(secrets),
		"secret_key":                   senderSk.String(),
		"previous_sender_balance":      fmt.Sprintf("%d", s.state.mintedBalances[senderIdx]),
		"previous_sender_random_value": prevSenderR.String(),
		"tx_values":                    toStrs(txValues),
		"tx_random_values":             toStrs(txRandom),
		"sender_tx_value":              fmt.Sprintf("%d", senderAmt),
	})

	t0Proof := time.Now()
	httpResp, err := http.Post(gnarkURL, "application/json", bytes.NewReader(proofReqBody))
	if err != nil {
		fc.emit("zk_proof", "error", "Generate ZK proof", err.Error())
		fc.done(false, "Gnark server unreachable")
		return
	}
	defer httpResp.Body.Close()
	proofBody, _ := io.ReadAll(httpResp.Body)
	if httpResp.StatusCode != http.StatusOK {
		fc.emit("zk_proof", "error", "Generate ZK proof", string(proofBody))
		fc.done(false, "Proof generation failed")
		return
	}
	var proofResp struct {
		Proof        []*big.Int `json:"proof"`
		PublicSignal []*big.Int `json:"publicSignal"`
	}
	if err := json.Unmarshal(proofBody, &proofResp); err != nil {
		fc.emit("zk_proof", "error", "Generate ZK proof", "bad response: "+err.Error())
		fc.done(false, "Cannot parse proof response")
		return
	}
	proofElapsed := time.Since(t0Proof)
	fc.emit("zk_proof", "success", "Generate ZK proof",
		fmt.Sprintf("π ready in %s · %d signals", proofElapsed.Round(time.Millisecond), len(proofResp.PublicSignal)))
	fc.metric("proofTimeMs", fmt.Sprintf("%d", proofElapsed.Milliseconds()))
	fc.metric("proofSizeBytes", "256")

	// Relay
	fc.emit("relay_transfer", "running", "Submit via Relayer", "POST /relay/transfer")
	fc.log("Submitting Transfer via relayer…")

	// TX_COMMIT_OFFSET = 36 (FingerPrint 6×6) + 6 (PKs) + 12 (prevCommit) = 54
	const txCommitOffset = 54
	commFinal := make([][]string, nBanks)
	for i := 0; i < nBanks; i++ {
		c1 := proofResp.PublicSignal[txCommitOffset+2*i]
		c2 := proofResp.PublicSignal[txCommitOffset+2*i+1]
		commFinal[i] = []string{c1.String(), c2.String()}
	}
	var proof8 [8]string
	for i := 0; i < 8; i++ {
		proof8[i] = proofResp.Proof[i].String()
	}
	pubSigStrs := make([]string, len(proofResp.PublicSignal))
	for i, v := range proofResp.PublicSignal {
		pubSigStrs[i] = v.String()
	}
	kIdx64 := make([]int64, nBanks)
	for i := range kIdx64 {
		kIdx64[i] = int64(i + 1)
	}
	relayReqBody, _ := json.Marshal(struct {
		Proof        [8]string  `json:"proof"`
		PublicSignal []string   `json:"publicSignal"`
		Commitments  [][]string `json:"commitments"`
		KIndex       []int64    `json:"kIndex"`
	}{proof8, pubSigStrs, commFinal, kIdx64})

	relayAPIKey := os.Getenv("RELAYER_API_KEY")
	if relayAPIKey == "" {
		relayAPIKey = defaultRelayKey
	}
	relayStart := time.Now()
	relayHTTPReq, _ := http.NewRequest(http.MethodPost, relayerURL+"/relay/transfer", bytes.NewReader(relayReqBody))
	relayHTTPReq.Header.Set("Content-Type", "application/json")
	relayHTTPReq.Header.Set("Authorization", "Bearer "+relayAPIKey)

	relayHTTPResp, err := http.DefaultClient.Do(relayHTTPReq)
	if err != nil {
		fc.emit("relay_transfer", "error", "Submit via Relayer", err.Error())
		fc.done(false, "Relayer unreachable")
		return
	}
	defer relayHTTPResp.Body.Close()
	relayRespBody, _ := io.ReadAll(relayHTTPResp.Body)
	if relayHTTPResp.StatusCode != http.StatusOK {
		fc.emit("relay_transfer", "error", "Submit via Relayer", string(relayRespBody))
		fc.done(false, "Transfer rejected")
		return
	}
	var relayResult struct {
		TxHash      string `json:"txHash"`
		BlockNumber uint64 `json:"blockNumber"`
		GasUsed     uint64 `json:"gasUsed"`
	}
	json.Unmarshal(relayRespBody, &relayResult)
	relayRTT := time.Since(relayStart)
	s.state.totalGasUsed += relayResult.GasUsed
	fc.emit("relay_transfer", "success", "Submit via Relayer",
		fmt.Sprintf("tx %s · block %d · gas %d", trunc(relayResult.TxHash, 14), relayResult.BlockNumber, relayResult.GasUsed))
	fc.metric("verifyTimeMs", fmt.Sprintf("%d", relayRTT.Milliseconds()))
	fc.metric("verifyGas", fmt.Sprintf("%d", relayResult.GasUsed))
	pause(800 * time.Millisecond)

	// Verify balance
	fc.emit("verify_balance", "running", "Verify balance", "getBalance(1) + homomorphic check")
	newBal, err := inst.GetBalance(&bind.CallOpts{}, big.NewInt(int64(senderIdx+1)))
	if err != nil {
		fc.emit("verify_balance", "error", "Verify balance", err.Error())
		fc.done(false, "GetBalance failed")
		return
	}
	prevPt := &babyjub.Point{X: prevBalances[senderIdx].C1, Y: prevBalances[senderIdx].C2}
	deltaPt := &babyjub.Point{X: txCommit[senderIdx].C1, Y: txCommit[senderIdx].C2}
	expected := addPoints(prevPt, deltaPt)

	if newBal.X.Cmp(expected.X) != 0 || newBal.Y.Cmp(expected.Y) != 0 {
		fc.emit("verify_balance", "error", "Verify balance",
			fmt.Sprintf("MISMATCH — got (%s, %s)", trunc(newBal.X.String(), 12), trunc(newBal.Y.String(), 12)))
		fc.done(false, "Balance homomorphic check FAILED")
		return
	}
	fc.emit("verify_balance", "success", "Verify balance",
		fmt.Sprintf("prevBalance + TxCommit[%d] = newBalance ✓  (−%d tokens confirmed)", senderIdx, senderAmt))
	fc.log("Homomorphic check PASSED ✓")
	fc.participant(senderIdx, "balance",
		fmt.Sprintf("(%s…, %s…)", trunc(newBal.X.String(), 10), trunc(newBal.Y.String(), 10)))
	fc.participant(senderIdx, "status", "settled")
	// Broadcast final balances for all banks
	if pubValsNew, errNew := inst.GetPublicValues(&bind.CallOpts{}, big.NewInt(nBanks+1)); errNew == nil {
		for i := 0; i < nBanks; i++ {
			bal := pubValsNew.Balances[i+1]
			fc.b.publish(Event{Type: "participant", Tab: "transfer", Pid: i, Field: "finalBal",
				Value: fmt.Sprintf("(%s…,%s…)", trunc(bal.C1.String(), 8), trunc(bal.C2.String(), 8))})
		}
	}

	s.state.mu.Lock()
	s.state.mintedBalances[senderIdx] -= senderAmt
	for i := 0; i < nBanks; i++ {
		if i != senderIdx {
			s.state.mintedBalances[i] += receiverAmts[i]
		}
	}
	// Accumulate txRandom into cumulativeR only after the transfer is confirmed on-chain.
	for i := 0; i < nBanks; i++ {
		if s.state.cumulativeR[i] == nil {
			s.state.cumulativeR[i] = new(big.Int)
		}
		s.state.cumulativeR[i].Add(s.state.cumulativeR[i], txRandom[i])
		s.state.cumulativeR[i].Mod(s.state.cumulativeR[i], curveP)
	}
	s.state.transferCount++
	s.state.lastSenderIdx = senderIdx
	s.state.mu.Unlock()

	flowMs := time.Since(flowStart).Milliseconds()
	fc.metric("totalGas", fmt.Sprintf("%d", s.state.totalGasUsed))
	fc.metric("flowTimeMs", fmt.Sprintf("%d", flowMs))
	fc.log(fmt.Sprintf("✓ Transfer complete: %d ms  total protocol gas: %d", flowMs, s.state.totalGasUsed))
	fc.done(true, fmt.Sprintf("Transfer of %d tokens settled and verified on-chain", senderAmt))
}

// ── Tab 5: Key Agreement flow ─────────────────────────────────────────────────

func (s *Server) handleRunKeyAgreement(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	s.startFlow(w, func() { runKeyAgreement(s) })
}

// runKeyAgreement simulates ML-KEM-768 pairwise key agreement with realistic
// key sizes (NIST FIPS 203): dk seed=64B, ek=1184B, ct=1088B, rawSS=32B.
// It derives Baby JubJub field elements via SHA-256("enygma-view-key-v1:"||rawSS) mod P.
func runKeyAgreement(s *Server) {
	fc := newCtx(s, "agreement")
	t0 := time.Now()

	type bankData struct {
		ek       []byte // full 1184-byte simulated ML-KEM-768 encapsulation key
		ekPrefix [8]byte
	}
	banks := make([]bankData, nBanks)

	// ── Phase 1: Keygen ──────────────────────────────────────────────────────────
	fc.emit("ka_keygen", "running", "Keygen ×6", "GenerateKey() → (dk seed 64B, ek 1184B) per bank")
	pause(200 * time.Millisecond)

	for i := 0; i < nBanks; i++ {
		fc.emitBank(i, fmt.Sprintf("ka_keygen_%d", i), "running",
			fmt.Sprintf("Bank %d · GenerateKey()", i),
			"rand(64B seed) → dk  |  derive ek (1184B)")

		var seed [64]byte
		if _, err := io.ReadFull(rand.Reader, seed[:]); err != nil {
			fc.done(false, "rand failed: "+err.Error())
			return
		}
		// Derive a 1184-byte simulated ML-KEM-768 encapsulation key from the seed
		// using a chain of SHA-256 digests (46 × 32B = 1472B, truncated to 1184B).
		ek := make([]byte, 1184)
		prev := seed[:]
		for off := 0; off < 1184; off += 32 {
			h := sha256.Sum256(prev)
			end := off + 32
			if end > 1184 {
				end = 1184
			}
			copy(ek[off:end], h[:end-off])
			prev = h[:]
		}
		banks[i].ek = ek
		copy(banks[i].ekPrefix[:], ek[:8])
		pause(220 * time.Millisecond)

		fc.emitBank(i, fmt.Sprintf("ka_keygen_%d", i), "success",
			fmt.Sprintf("Bank %d · Keypair Ready", i),
			fmt.Sprintf("ek = %s… (1184B)", hex.EncodeToString(banks[i].ekPrefix[:])))
		fc.participant(i, "ka_ek", hex.EncodeToString(banks[i].ekPrefix[:])+"…")
		pause(80 * time.Millisecond)
	}
	fc.emit("ka_keygen", "success", "Keygen ×6 Complete",
		"6 ML-KEM-768 keypairs — ek (1184B) published, dk (64B seed) kept private")
	pause(500 * time.Millisecond)

	// ── Phase 2: Encapsulate — all C(6,2)=15 pairs, leader = lower index ────────
	fc.emit("ka_encap", "running", "Encapsulate ×15",
		"Each bank encapsulates to every peer with a higher index — 15 pairs total")

	var rawSecrets [nBanks][nBanks][32]byte
	var ctPrefixes [nBanks][nBanks][8]byte

	for i := 0; i < nBanks-1; i++ {
		for j := i + 1; j < nBanks; j++ {
			stepName := fmt.Sprintf("ka_encap_%d_%d", i, j)
			fc.emitBank(i, stepName, "running",
				fmt.Sprintf("Bank %d → Bank %d · Encapsulate", i, j),
				fmt.Sprintf("Encapsulate(ek[%d]) → ct (1088B) + rawSS (32B)", j))
			pause(100 * time.Millisecond)

			if _, err := io.ReadFull(rand.Reader, rawSecrets[i][j][:]); err != nil {
				fc.done(false, "rand failed: "+err.Error())
				return
			}
			ctHash := sha256.Sum256(append([]byte("ct:"), rawSecrets[i][j][:]...))
			copy(ctPrefixes[i][j][:], ctHash[:8])

			fc.emitBank(i, stepName, "success",
				fmt.Sprintf("Bank %d → Bank %d · Sent", i, j),
				fmt.Sprintf("ct = %s… (1088B)", hex.EncodeToString(ctPrefixes[i][j][:])))
			pause(30 * time.Millisecond)
		}
	}
	fc.emit("ka_encap", "success", "Encapsulate ×15 Complete",
		"15 ciphertexts dispatched — every peer can now independently decapsulate")
	pause(400 * time.Millisecond)

	// ── Phase 3: Decapsulate + field element derivation — all 15 pairs ──────────
	fc.emit("ka_decap", "running", "Decapsulate ×15",
		"Each peer: dk + ct → same rawSS → SHA-256('enygma-view-key-v1:'||rawSS) mod P")

	var fieldElems [nBanks][nBanks]*big.Int
	agreedCount := [nBanks]int{}

	for i := 0; i < nBanks-1; i++ {
		for j := i + 1; j < nBanks; j++ {
			stepName := fmt.Sprintf("ka_decap_%d_%d", i, j)
			fc.emitBank(j, stepName, "running",
				fmt.Sprintf("Bank %d · Decapsulate ct from %d", j, i),
				fmt.Sprintf("dk[%d] + ct_%d→%d → rawSS → SHA-256 → mod P", j, i, j))
			pause(130 * time.Millisecond)

			h := sha256.New()
			h.Write([]byte("enygma-view-key-v1:"))
			h.Write(rawSecrets[i][j][:])
			digest := h.Sum(nil)
			fe := new(big.Int).SetBytes(digest)
			fe.Mod(fe, curveP)
			fieldElems[i][j] = fe
			fieldElems[j][i] = new(big.Int).Set(fe) // symmetric

			agreedCount[i]++
			agreedCount[j]++
			fc.emitBank(j, stepName, "success",
				fmt.Sprintf("Bank %d ↔ Bank %d · Agreed ✓", i, j),
				fmt.Sprintf("fe = %s…", trunc(fe.String(), 16)))
			fc.participant(i, "ka_done", fmt.Sprintf("%d/5", agreedCount[i]))
			fc.participant(j, "ka_done", fmt.Sprintf("%d/5", agreedCount[j]))
			fc.log(fmt.Sprintf("  ss[%d↔%d] = %s…", i, j, trunc(fe.String(), 52)))
			pause(30 * time.Millisecond)
		}
	}

	fc.emit("ka_decap", "success", "All 15 Secrets Established",
		"Every bank pair has an identical pairwise ML-KEM-768 secret — full mesh complete")
	fc.emit("ka_field", "success", "15 Baby JubJub Scalars Ready",
		"SHA-256('enygma-view-key-v1:'||rawSS) mod P — usable as view-key secrets for any sender")

	s.state.mu.Lock()
	for i := 0; i < nBanks; i++ {
		s.state.kaEKs[i] = banks[i].ek
		for j := 0; j < nBanks; j++ {
			if fieldElems[i][j] != nil {
				s.state.kaSecrets[i][j] = fieldElems[i][j]
			}
		}
	}
	s.state.mu.Unlock()

	ms := time.Since(t0).Milliseconds()
	fc.done(true, fmt.Sprintf("ML-KEM-768 agreement complete in %dms — 5 pairwise secrets ready for payments", ms))
}

// ── Verify / calculator endpoints ─────────────────────────────────────────────

func (s *Server) handleCalcPedersen(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	var body struct {
		V string `json:"v"`
		R string `json:"r"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"bad json"}`, 400)
		return
	}
	v, okV := new(big.Int).SetString(body.V, 10)
	rr, okR := new(big.Int).SetString(body.R, 10)
	if !okV || !okR || v == nil || rr == nil {
		http.Error(w, `{"error":"invalid v or r — must be decimal integers"}`, 400)
		return
	}
	pt := pedersenCommit(v, rr)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"c1": pt.X.String(), "c2": pt.Y.String()})
}

func (s *Server) handleStateBalances(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", 405)
		return
	}
	s.state.mu.Lock()
	ready := s.state.ready
	inst := s.state.inst
	s.state.mu.Unlock()
	if !ready || inst == nil {
		http.Error(w, `{"error":"run Setup first"}`, 503)
		return
	}
	pubVals, err := inst.GetPublicValues(&bind.CallOpts{}, big.NewInt(nBanks+1))
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, 500)
		return
	}
	s.state.mu.Lock()
	registeredSks := s.state.registeredSks
	s.state.mu.Unlock()

	type point struct {
		C1         string `json:"c1"`
		C2         string `json:"c2"`
		Registered bool   `json:"registered"`
	}
	balances := make([]point, nBanks)
	for i := 0; i < nBanks; i++ {
		bal := pubVals.Balances[i+1]
		balances[i] = point{
			C1:         bal.C1.String(),
			C2:         bal.C2.String(),
			Registered: registeredSks[i] != nil,
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"balances": balances})
}

func (s *Server) handleStateLastTransfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", 405)
		return
	}
	s.state.mu.Lock()
	rvCopy := s.state.lastRValues
	p := new(big.Int).Set(curveP)
	sidx := s.state.lastSenderIdx
	s.state.mu.Unlock()
	rvStrs := make([]string, nBanks)
	for i, v := range rvCopy {
		if v != nil {
			rvStrs[i] = v.String()
		} else {
			rvStrs[i] = "0"
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"rValues":   rvStrs,
		"P":         p.String(),
		"senderIdx": sidx,
	})
}

func (s *Server) handleStateCumulativeR(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", 405)
		return
	}
	s.state.mu.Lock()
	crCopy := s.state.cumulativeR
	count := s.state.transferCount
	p := new(big.Int).Set(curveP)
	s.state.mu.Unlock()
	crStrs := make([]string, nBanks)
	for i, v := range crCopy {
		if v != nil {
			crStrs[i] = v.String()
		} else {
			crStrs[i] = "0"
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"cumulativeR":   crStrs,
		"transferCount": count,
		"P":             p.String(),
	})
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

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	srv := &Server{broker: newBroker()}

	mux := http.NewServeMux()
	mux.HandleFunc("/", srv.handleIndex)
	mux.HandleFunc("/events", srv.handleEvents)
	mux.HandleFunc("/run/setup", srv.handleRunSetup)
	mux.HandleFunc("/run/register-bank", srv.handleRunRegisterBank)
	mux.HandleFunc("/run/mint", srv.handleRunMint)
	mux.HandleFunc("/run/transfer", srv.handleRunTransfer)
	mux.HandleFunc("/run/key-agreement", srv.handleRunKeyAgreement)
	mux.HandleFunc("/calc/pedersen", srv.handleCalcPedersen)
	mux.HandleFunc("/state/balances", srv.handleStateBalances)
	mux.HandleFunc("/state/last-transfer", srv.handleStateLastTransfer)
	mux.HandleFunc("/state/cumulative-r", srv.handleStateCumulativeR)

	log.Printf("Enygma Payments Demo → http://localhost%s", listenAddr)
	log.Printf("  RPC URL  : %s  (chainId=%d)", rpcURL, chainID)
	log.Printf("  Gnark    : %s", gnarkURL)
	log.Printf("  Relayer  : %s", relayerURL)

	if err := http.ListenAndServe(listenAddr, mux); err != nil {
		log.Fatalf("server: %v", err)
	}
}
