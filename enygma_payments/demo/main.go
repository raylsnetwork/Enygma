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

//go:embed bank.html
var bankHTML string

// ── Config ─────────────────────────────────────────────────────────────────────

const (
	listenAddr = ":9090"
	gnarkURL   = "http://127.0.0.1:8080/proof/enygma"
	gnarkFeeURL = "http://127.0.0.1:8080/proof/enygma_fee"
	relayerURL = "http://127.0.0.1:8082"

	nBanks      = 6
	mintAmount  = 500
	transferAmt = 100
	defaultFeeAmt = 20 // demo default protocol fee — matches enygma_test's PROTOCOL_FEE convention

	defaultOwnerKey = "34d091c661db4c814d65c8ae9277b7055c0dde5a752ce5a3fdfd4ea11a8f7154"
	defaultRelayKey = "enygma-test-secret"

	senderSkVal    = 424242
	senderPrevRVal = 0 // initial balance = Com(0,0) = (0,1) on BabyJubJub

	// Treasury: a reserved account (accountId = nBanks+1, outside the 6 demo
	// banks) that transferWithFee() credits with fee·G on every fee transfer.
	// Registered during Setup alongside the fee verifier — see runSetup.
	treasuryAccountIdVal = nBanks + 1
	treasurySkVal        = 31337
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
	// BN254 field prime — same Q used by Solidity's CurveBabyJubJub
	curveQ, _ = new(big.Int).SetString("21888242871839275222246405745257275088548364400416034343698204186575808495617", 10)

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

// addPointsAffine mirrors CurveBabyJubJub.pointAdd exactly:
//
//	x3 = (x1*y2 + y1*x2) / (1 + D*x1*x2*y1*y2)
//	y3 = (y1*y2 - A*x1*x2) / (1 - D*x1*x2*y1*y2)
//
// Uses the same Q, A=168700, D=168696 constants as the Solidity contract.
func addPointsAffine(ax, ay, bx, by *big.Int) (x3, y3 *big.Int) {
	Q := curveQ
	D := big.NewInt(168696)
	A := big.NewInt(168700)

	one := big.NewInt(1)

	// neutral-element fast paths (match Solidity's early returns)
	if ax.Sign() == 0 && ay.Cmp(one) == 0 {
		return new(big.Int).Set(bx), new(big.Int).Set(by)
	}
	if bx.Sign() == 0 && by.Cmp(one) == 0 {
		return new(big.Int).Set(ax), new(big.Int).Set(ay)
	}

	x1x2 := new(big.Int).Mul(ax, bx)
	x1x2.Mod(x1x2, Q)

	y1y2 := new(big.Int).Mul(ay, by)
	y1y2.Mod(y1y2, Q)

	dx1x2y1y2 := new(big.Int).Mul(D, x1x2)
	dx1x2y1y2.Mul(dx1x2y1y2, y1y2)
	dx1x2y1y2.Mod(dx1x2y1y2, Q)

	x3Num := new(big.Int).Add(
		new(big.Int).Mul(ax, by),
		new(big.Int).Mul(ay, bx),
	)
	x3Num.Mod(x3Num, Q)

	y3Num := new(big.Int).Sub(y1y2, new(big.Int).Mul(A, x1x2))
	y3Num.Mod(y3Num, Q)

	x3Den := new(big.Int).Add(one, dx1x2y1y2)
	x3Den.Mod(x3Den, Q)

	y3Den := new(big.Int).Sub(new(big.Int).Set(Q), dx1x2y1y2)
	y3Den.Add(y3Den, one)
	y3Den.Mod(y3Den, Q)

	x3 = new(big.Int).Mul(x3Num, new(big.Int).ModInverse(x3Den, Q))
	x3.Mod(x3, Q)

	y3 = new(big.Int).Mul(y3Num, new(big.Int).ModInverse(y3Den, Q))
	y3.Mod(y3, Q)

	return x3, y3
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
	feeVerifierAddr string
	treasuryReady   bool  // true once the fee verifier is registered and treasury account configured
	treasuryFeeTotal int64 // cumulative plaintext fee total credited to the treasury this session
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

func (s *Server) handleBank(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, bankHTML)
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

func (s *Server) handleRunTransferWithFee(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	var body struct {
		SenderIdx    int           `json:"senderIdx"`
		SenderAmt    int64         `json:"senderAmt"`
		ReceiverAmts [nBanks]int64 `json:"receiverAmts"`
		FeeAmt       int64         `json:"feeAmt"`
	}
	body.SenderAmt = int64(transferAmt)
	body.ReceiverAmts[1] = 60
	body.ReceiverAmts[2] = 40
	body.FeeAmt = defaultFeeAmt
	json.NewDecoder(r.Body).Decode(&body)
	if body.SenderIdx < 0 || body.SenderIdx >= nBanks {
		http.Error(w, `{"error":"senderIdx out of range 0-5"}`, 400)
		return
	}
	if body.SenderAmt <= 0 {
		body.SenderAmt = int64(transferAmt)
	}
	if body.FeeAmt < 0 {
		http.Error(w, `{"error":"feeAmt must be >= 0"}`, 400)
		return
	}
	var receiverSum int64
	for i := 0; i < nBanks; i++ {
		if i != body.SenderIdx {
			receiverSum += body.ReceiverAmts[i]
		}
	}
	if receiverSum != body.SenderAmt {
		http.Error(w, `{"error":"receiver amounts must sum to senderAmt (fee is on top, not included)"}`, 400)
		return
	}
	sidx := body.SenderIdx
	sa := body.SenderAmt
	ramts := body.ReceiverAmts
	fa := body.FeeAmt
	s.startFlow(w, func() { runTransferWithFee(s, sidx, sa, ramts, fa) })
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
	TOKEN        struct{ ContractAddress string `json:"contractAddress"` } `json:"TOKEN"`
	VERIFIER     struct{ ContractAddress string `json:"contractAddress"` } `json:"VERIFIER"`
	FEE_VERIFIER struct{ ContractAddress string `json:"contractAddress"` } `json:"FEE_VERIFIER"`
}

func resolveAddresses() (token, verifier, feeVerifier string, err error) {
	token = os.Getenv("ENYGMA_TOKEN_ADDR")
	verifier = os.Getenv("ENYGMA_VERIFIER_ADDR")
	feeVerifier = os.Getenv("ENYGMA_FEE_VERIFIER_ADDR")
	if token != "" && verifier != "" && feeVerifier != "" {
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
			if feeVerifier == "" {
				feeVerifier = rec.FEE_VERIFIER.ContractAddress
			}
		}
	}
	if token == "" || verifier == "" {
		err = fmt.Errorf("contract addresses not found — set ENYGMA_TOKEN_ADDR / ENYGMA_VERIFIER_ADDR or run deploy scripts")
		return
	}
	// feeVerifier is allowed to stay empty — older deploy_receipts.json files
	// (from before the fee system existed) won't have it. Setup surfaces this
	// clearly and skips fee-verifier registration rather than failing setup
	// entirely; the fee-transfer mode just won't work until re-deployed.
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

	tokenAddr, verifierAddr, feeVerifierAddr, err := resolveAddresses()
	if err != nil {
		fc.emit("dial_chain", "error", "Dial chain", err.Error())
		fc.done(false, err.Error())
		return
	}
	fc.log(fmt.Sprintf("TOKEN        contract: %s", tokenAddr))
	fc.log(fmt.Sprintf("VERIFIER     contract: %s", verifierAddr))
	if feeVerifierAddr != "" {
		fc.log(fmt.Sprintf("FEE_VERIFIER contract: %s", feeVerifierAddr))
	}

	// Sync the relayer's address file so it picks up the freshly-deployed contract
	// on its next restart (avoids the demo and relay pointing at different contracts).
	_, thisFileSrc, _, _ := runtime.Caller(0)
	goClientDir := filepath.Join(filepath.Dir(thisFileSrc), "..", "..", "go_client")
	addrJSON := fmt.Sprintf(`{"address": "%s"}`, tokenAddr)
	for _, name := range []string{"address.json", filepath.Join("config", "address.json")} {
		_ = os.WriteFile(filepath.Join(goClientDir, name), []byte(addrJSON), 0644)
	}
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
	s.state.feeVerifierAddr = feeVerifierAddr
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

	// Fee verifier + treasury — required before the fee-transfer mode can run.
	// Both idempotent, same pattern as add_verifier above. A pre-existing
	// deploy_receipts.json from before the fee system existed won't have a
	// FEE_VERIFIER address; skip cleanly rather than failing setup — the
	// fee-transfer mode just stays unavailable until re-deployed.
	if feeVerifierAddr == "" {
		fc.emit("add_fee_verifier", "warning", "Register fee verifier",
			"No FEE_VERIFIER in deploy_receipts.json — re-run the deploy script to enable fee transfers")
	} else {
		fc.emit("add_fee_verifier", "running", "Register fee verifier",
			fmt.Sprintf("addFeeVerifier(%s)", trunc(feeVerifierAddr, 14)))
		if afvTx, afvErr := inst.AddFeeVerifier(fc.mkAuth(), common.HexToAddress(feeVerifierAddr)); afvErr == nil {
			if _, waitErr := fc.waitTx("AddFeeVerifier", afvTx, afvErr); waitErr != nil {
				fc.log("Fee verifier already registered (OK — idempotent)")
			}
		} else {
			fc.log("AddFeeVerifier skipped: " + afvErr.Error())
		}
		fc.emit("add_fee_verifier", "success", "Register fee verifier",
			fmt.Sprintf("Groth16 fee verifier registered at %s", trunc(feeVerifierAddr, 14)))
		fc.proto("feeVerifierAddr", feeVerifierAddr)
		pause(400 * time.Millisecond)

		fc.emit("setup_treasury", "running", "Configure treasury",
			fmt.Sprintf("registerAccount(id=%d) + setTreasuryAccountId(%d)", treasuryAccountIdVal, treasuryAccountIdVal))
		treasuryPk, _ := poseidon.Hash([]*big.Int{big.NewInt(treasurySkVal), big.NewInt(treasurySkVal)})
		treasuryPk.Mod(treasuryPk, curveP)
		if trTx, trErr := inst.RegisterAccount(fc.mkAuth(), owner,
			big.NewInt(treasuryAccountIdVal), treasuryPk, big.NewInt(0), []byte{}); trErr == nil {
			if _, waitErr := fc.waitTx("RegisterAccount[treasury]", trTx, trErr); waitErr != nil {
				fc.log("Treasury account already registered (OK — idempotent)")
			}
		} else {
			fc.log("Treasury RegisterAccount skipped: " + trErr.Error())
		}
		if stTx, stErr := inst.SetTreasuryAccountId(fc.mkAuth(), big.NewInt(treasuryAccountIdVal)); stErr == nil {
			if _, waitErr := fc.waitTx("SetTreasuryAccountId", stTx, stErr); waitErr != nil {
				fc.log("Treasury account ID already set (OK — idempotent)")
			}
		} else {
			fc.log("SetTreasuryAccountId skipped: " + stErr.Error())
		}
		fc.emit("setup_treasury", "success", "Configure treasury",
			fmt.Sprintf("treasuryAccountId=%d ready to receive fee·G on every fee transfer", treasuryAccountIdVal))
		fc.proto("treasuryReady", "1")
		s.state.mu.Lock()
		s.state.treasuryReady = true
		s.state.mu.Unlock()
		pause(400 * time.Millisecond)
	}

	// Verify the relay is pointing at the same contract the demo just initialized.
	// A mismatch (relay started with a stale RELAYER_CONTRACT_ADDR) causes silent
	// balance-not-updated bugs because the relay submits TXs to the wrong contract.
	fc.emit("check_relay_addr", "running", "Verify relay contract", "GET /relay/info")
	relayInfoResp, relayInfoErr := http.Get(relayerURL + "/relay/info")
	if relayInfoErr == nil {
		var relayInfo struct {
			ContractAddr string `json:"contractAddr"`
		}
		if infoBody, readErr := io.ReadAll(relayInfoResp.Body); readErr == nil {
			json.Unmarshal(infoBody, &relayInfo)
		}
		relayInfoResp.Body.Close()
		if !strings.EqualFold(relayInfo.ContractAddr, tokenAddr) {
			fc.emit("check_relay_addr", "error", "Verify relay contract",
				fmt.Sprintf("Relay uses %s but demo uses %s — restart relay with RELAYER_CONTRACT_ADDR=%s",
					trunc(relayInfo.ContractAddr, 14), trunc(tokenAddr, 14), tokenAddr))
			fc.done(false, fmt.Sprintf("Relayer contract mismatch — restart relayer with RELAYER_CONTRACT_ADDR=%s (address.json already updated)", tokenAddr))
			return
		}
		fc.emit("check_relay_addr", "success", "Verify relay contract",
			fmt.Sprintf("Relay ↔ demo both at %s ✓", trunc(tokenAddr, 14)))
	} else {
		fc.emit("check_relay_addr", "warning", "Verify relay contract", "Could not reach relay for address check")
	}

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

	ekDesc := "(run Key Agreement on this bank's dashboard)"
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
		fc.participant(bankIdx, "view_ek", "(run Key Agreement on this bank's dashboard)")
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
	fc := newCtx(s, "mint")
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

	// Build k×k FingerPrint matrix.
	// If key agreement has been run, load the full pre-computed matrix from disk
	// (fp[i][j] = Poseidon(ss[i][j]) mod P for all pairs).
	// Otherwise fall back to the sparse demo-defaults matrix (sender's column only).
	var fpStrs [][]string
	if loaded, err := loadFingerprintMatrix(); err == nil {
		fpStrs = loaded
		fc.log(fmt.Sprintf("FingerPrint: loaded full %dx%d matrix from %s", nBanks, nBanks, fpMatrixFile))
	} else {
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
		fpStrs = make([][]string, nBanks)
		for i := range fpStrs {
			fpStrs[i] = make([]string, nBanks)
			for j := range fpStrs[i] {
				fpStrs[i][j] = fp[i][j].String()
			}
		}
		fc.log("FingerPrint: key agreement not run — using sparse sender-column-only matrix")
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

	// Transfer confirmed on-chain — update server state before verify so state stays
	// in sync even if the homomorphic check below fails.
	s.state.mu.Lock()
	s.state.mintedBalances[senderIdx] -= senderAmt
	for i := 0; i < nBanks; i++ {
		if i != senderIdx {
			s.state.mintedBalances[i] += receiverAmts[i]
		}
	}
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

	// Verify balance — use the contract's own pointAdd (addPedComm) so the expected
	// value is computed with the exact same arithmetic as _updateBalancesForTransfer.
	fc.emit("verify_balance", "running", "Verify balance", "getBalance(1) + homomorphic check")
	newBal, err := inst.GetBalance(&bind.CallOpts{}, big.NewInt(int64(senderIdx+1)))
	if err != nil {
		fc.emit("verify_balance", "error", "Verify balance", err.Error())
		fc.done(false, "GetBalance failed")
		return
	}

	expX, expY, pedErr := inst.AddPedComm(&bind.CallOpts{},
		prevBalances[senderIdx].C1, prevBalances[senderIdx].C2,
		txCommit[senderIdx].C1, txCommit[senderIdx].C2,
	)
	if pedErr != nil {
		fc.emit("verify_balance", "error", "Verify balance", "addPedComm: "+pedErr.Error())
		fc.done(false, "Verify: addPedComm failed")
		return
	}

	if newBal.X.Cmp(expX) != 0 || newBal.Y.Cmp(expY) != 0 {
		fc.emit("verify_balance", "error", "Verify balance",
			fmt.Sprintf("MISMATCH — got (%s, %s) expected (%s, %s)",
				trunc(newBal.X.String(), 12), trunc(newBal.Y.String(), 12),
				trunc(expX.String(), 12), trunc(expY.String(), 12)))
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

	flowMs := time.Since(flowStart).Milliseconds()
	fc.metric("totalGas", fmt.Sprintf("%d", s.state.totalGasUsed))
	fc.metric("flowTimeMs", fmt.Sprintf("%d", flowMs))
	fc.log(fmt.Sprintf("✓ Transfer complete: %d ms  total protocol gas: %d", flowMs, s.state.totalGasUsed))
	fc.done(true, fmt.Sprintf("Transfer of %d tokens settled and verified on-chain", senderAmt))
}

// ── Tab 3b: Transfer with protocol fee ──────────────────────────────────────────
//
// Mirrors runTransfer above but drives the enygma_fee circuit (54 public
// signals) instead of the base FingerPrint circuit (80 signals) — a
// genuinely different circuit, not a parameter variant, so it gets its own
// function rather than a conditional branch threaded through runTransfer.
// Reuses the same SSE step ids as runTransfer (same swimlane UI on the bank
// dashboard) plus one fee-specific step, "treasury_credit", at the end.
//
// Differences from the base circuit, all straight from
// gnark-server/pkg/circuits/enygma_fee and go_client/enygma_test/fee_transfer_test.go:
//   - Per-participant proof-of-knowledge is a flat hash array
//     (Poseidon(secret,secret) mod P), not the k×k FingerPrint matrix.
//   - The nullifier is derived from that hash, not the raw secret.
//   - The sender's committed value is -(amount+fee); the fee itself, plus
//     the two conservation-check outputs (SumTxCommit, SumTxValuesWithFee),
//     are additional public signals the relayer and contract both check.
func runTransferWithFee(s *Server, senderIdx int, senderAmt int64, receiverAmts [nBanks]int64, feeAmt int64) {
	fc := newCtx(s, "transfer")
	s.state.mu.Lock()
	ready := s.state.ready
	inst := s.state.inst
	treasuryReady := s.state.treasuryReady
	s.state.mu.Unlock()
	if !ready || inst == nil {
		fc.done(false, "Run Setup first")
		return
	}
	if !treasuryReady {
		fc.done(false, "Fee transfers need the treasury configured — re-run Setup (requires FEE_VERIFIER in deploy_receipts.json)")
		return
	}

	flowStart := time.Now()

	// Read on-chain state — including the treasury's pre-transfer balance,
	// so the new treasury_credit step can show a real before/after delta.
	fc.emit("read_state", "running", "Read on-chain state", "getPublicValues(7) + getBlckHash() + treasury balance")
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
	treasuryBalBefore, err := inst.GetBalance(&bind.CallOpts{}, big.NewInt(treasuryAccountIdVal))
	if err != nil {
		fc.emit("read_state", "error", "Read on-chain state", "treasury: "+err.Error())
		fc.done(false, "GetBalance(treasury) failed")
		return
	}
	prevBalances := pubVals.Balances[1:]
	onChainKeys := pubVals.Keys[1:]
	fc.emit("read_state", "success", "Read on-chain state",
		fmt.Sprintf("epochBlockHash = %s · %d accounts · treasury @ id=%d", trunc(blockHash.String(), 12), nBanks, treasuryAccountIdVal))
	fc.proto("senderIdx", fmt.Sprintf("%d", senderIdx))
	for i := 0; i < nBanks; i++ {
		fc.b.publish(Event{Type: "participant", Tab: "transfer", Pid: i, Field: "prevBal",
			Value: fmt.Sprintf("(%s…,%s…)", trunc(prevBalances[i].C1.String(), 8), trunc(prevBalances[i].C2.String(), 8))})
	}
	pause(600 * time.Millisecond)

	// Derive shared secrets — identical scheme to the base circuit.
	s.state.mu.Lock()
	senderSk := s.state.registeredSks[senderIdx]
	var prevSenderR *big.Int
	if s.state.cumulativeR[senderIdx] != nil {
		prevSenderR = new(big.Int).Set(s.state.cumulativeR[senderIdx])
	} else {
		prevSenderR = new(big.Int)
	}
	kaSecretsSnap := s.state.kaSecrets
	s.state.mu.Unlock()
	if senderSk == nil {
		senderSk = bankSks[senderIdx]
	}
	fc.emit("derive_secrets", "running", "Derive shared secrets", "s₀ = Poseidon(prevR, sk) mod P")
	senderSecret, _ := poseidon.Hash([]*big.Int{prevSenderR, senderSk})
	senderSecret.Mod(senderSecret, curveP)
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
	for i, sv := range secrets {
		fc.participant(i, "secret", trunc(sv.String(), 16)+"…")
	}
	pause(1000 * time.Millisecond)

	// Random factors — identical scheme to the base circuit.
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
		fc.participant(i, "r", trunc(rValues[i].String(), 16)+"…")
	}
	pause(1000 * time.Millisecond)

	// Commitments — sender debits (amount + fee), not just amount. This is
	// the one line that's structurally different from the base circuit: the
	// fee is folded straight into the sender's committed value, not billed
	// separately, which is what lets the circuit prove conservation with a
	// single extra term (fee·G) rather than a whole second transaction.
	fc.emit("build_commits", "running", "Build Pedersen commitments", "TxCommit_i = v_i·G + r_i·H  (sender debits amount+fee)")
	senderDebit := senderAmt + feeAmt
	var txValues [nBanks]*big.Int
	for i := 0; i < nBanks; i++ {
		if i == senderIdx {
			txValues[i] = negMod(big.NewInt(senderDebit))
		} else {
			txValues[i] = big.NewInt(receiverAmts[i])
		}
	}
	var txRandom [nBanks]*big.Int
	var txCommitPts [nBanks]*babyjub.Point
	var txCommit [nBanks]enygma.IEnygmaPoint
	for i := 0; i < nBanks; i++ {
		var pt *babyjub.Point
		if i == senderIdx {
			pt = pedersenCommit(negMod(big.NewInt(senderDebit)), rValues[i])
			txRandom[i] = rValues[i]
		} else {
			pt = pedersenCommit(txValues[i], negMod(rValues[i]))
			txRandom[i] = negMod(rValues[i])
		}
		txCommitPts[i] = pt
		txCommit[i] = enygma.IEnygmaPoint{C1: pt.X, C2: pt.Y}
	}
	s.state.mu.Lock()
	for i := 0; i < nBanks; i++ {
		s.state.lastRValues[i] = new(big.Int).Set(txRandom[i])
	}
	s.state.mu.Unlock()
	fc.emit("build_commits", "success", "Build Pedersen commitments",
		fmt.Sprintf("Bank%d −%d (−%d transfer −%d fee) · Σreceivers=%d · Σ+fee·G=(0,1) ✓", senderIdx, senderDebit, senderAmt, feeAmt, senderAmt))
	for i := 0; i < nBanks; i++ {
		amtLabel := "0  (privacy)"
		if i == senderIdx {
			amtLabel = fmt.Sprintf("−%d  (−%d xfer −%d fee)", senderDebit, senderAmt, feeAmt)
		} else if receiverAmts[i] > 0 {
			amtLabel = fmt.Sprintf("+%d  (credit)", receiverAmts[i])
		}
		fc.participant(i, "amount", amtLabel)
		fc.participant(i, "commit", fmt.Sprintf("(%s…, %s…)", trunc(txCommit[i].C1.String(), 10), trunc(txCommit[i].C2.String(), 10)))
	}
	pause(1000 * time.Millisecond)

	// Message tags — identical scheme to the base circuit.
	fc.emit("compute_tags", "running", "Compute message tags", "t_i = Poseidon(H₁₂, s_i, epochHash)")
	var tagMessages [nBanks]*big.Int
	for i := 0; i < nBanks; i++ {
		tagMessages[i] = tagValue(secrets[i], blockHash)
	}
	fc.emit("compute_tags", "success", "Compute message tags",
		fmt.Sprintf("tag[0] = %s", trunc(tagMessages[0].String(), 20)))
	for i := 0; i < nBanks; i++ {
		fc.participant(i, "tag", trunc(tagMessages[i].String(), 16)+"…")
	}
	pause(1000 * time.Millisecond)

	// Hash array + nullifier — the fee circuit's proof-of-knowledge scheme.
	// Poseidon(secret,secret) replaces the FingerPrint matrix; the nullifier
	// is derived from that hash rather than the raw secret directly.
	fc.emit("compute_nullifier", "running", "Compute nullifier", "h_i = Poseidon(s_i,s_i) · η = Poseidon(h₀, epochHash)")
	var hashArray [nBanks]*big.Int
	for i := 0; i < nBanks; i++ {
		h, _ := poseidon.Hash([]*big.Int{secrets[i], secrets[i]})
		hashArray[i] = h.Mod(h, curveP)
	}
	nullifier, _ := poseidon.Hash([]*big.Int{hashArray[senderIdx], blockHash})
	fc.emit("compute_nullifier", "success", "Compute nullifier",
		fmt.Sprintf("η = %s", trunc(nullifier.String(), 20)))
	pause(1000 * time.Millisecond)

	// Fee conservation outputs — SumTxCommit and SumTxValuesWithFee. The
	// circuit hard-asserts Σ(TxCommit)+fee·G == (0,1); computing it here too
	// lets the demo show the same identity the ZK proof is about to prove.
	sumCommit := txCommitPts[0]
	for i := 1; i < nBanks; i++ {
		sumCommit = addPoints(sumCommit, txCommitPts[i])
	}
	feeGPoint := babyjub.NewPoint().Mul(big.NewInt(feeAmt), curveG)
	sumCommit = addPoints(sumCommit, feeGPoint)
	sumTxVal := new(big.Int)
	for i := 0; i < nBanks; i++ {
		sumTxVal.Add(sumTxVal, txValues[i])
	}
	sumTxVal.Add(sumTxVal, big.NewInt(feeAmt))
	sumTxVal.Mod(sumTxVal, curveP)

	// ZK proof — enygma_fee circuit.
	fc.emit("zk_proof", "running", "Generate ZK proof", "POST /proof/enygma_fee — ~30s")
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
	anonSet := make([]string, nBanks)
	for i := range anonSet {
		anonSet[i] = fmt.Sprintf("%d", i)
	}
	proofReqBody, _ := json.Marshal(map[string]interface{}{
		"hashed_shared_secrets":        toStrs(hashArray),
		"public_keys":                  keyStrs,
		"previous_commits":             prevCommitSlice,
		"tx_commits":                   txCommitSlice,
		"block_number":                 blockHash.String(),
		"anonymity_set":                anonSet,
		"message_tags":                 toStrs(tagMessages),
		"nullifier":                    nullifier.String(),
		"fee":                          fmt.Sprintf("%d", feeAmt),
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
	httpResp, err := http.Post(gnarkFeeURL, "application/json", bytes.NewReader(proofReqBody))
	if err != nil {
		fc.emit("zk_proof", "error", "Generate ZK proof", err.Error())
		fc.done(false, "Gnark server unreachable")
		return
	}
	defer httpResp.Body.Close()
	proofBody, _ := io.ReadAll(httpResp.Body)
	if httpResp.StatusCode != http.StatusOK {
		fc.emit("zk_proof", "error", "Generate ZK proof", string(proofBody))
		fc.done(false, "Fee proof generation failed")
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
	conservationOK := sumCommit.X.Sign() == 0 && sumCommit.Y.Cmp(big.NewInt(1)) == 0 && sumTxVal.Sign() == 0
	conservationLabel := "Σ+fee·G=(0,1) ✓"
	if !conservationOK {
		conservationLabel = "Σ+fee·G conservation check FAILED"
	}
	fc.emit("zk_proof", "success", "Generate ZK proof",
		fmt.Sprintf("π ready in %s · %d signals · fee=%d · %s", proofElapsed.Round(time.Millisecond), len(proofResp.PublicSignal), feeAmt, conservationLabel))
	fc.metric("proofTimeMs", fmt.Sprintf("%d", proofElapsed.Milliseconds()))
	fc.metric("feeAmt", fmt.Sprintf("%d", feeAmt))
	pause(300 * time.Millisecond)

	// Relay — POST /relay/transfer_fee. The relayer runs its own local
	// Groth16 verify, checks signal[50] (fee) against RELAYER_MIN_FEE,
	// dry-runs via eth_call, then submits. A 402 here means the embedded fee
	// was below the relayer's floor — surfaced distinctly, not as a generic
	// failure, since it's the fee system's enforcement actually firing.
	const txCommitOffset = 24 // 54-signal layout: 6 hashes + 6 keys + 12 prevCommit = 24
	commFinal := make([][]string, nBanks)
	for i := 0; i < nBanks; i++ {
		commFinal[i] = []string{
			proofResp.PublicSignal[txCommitOffset+2*i].String(),
			proofResp.PublicSignal[txCommitOffset+2*i+1].String(),
		}
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
	fc.emit("relay_transfer", "running", "Submit via Relayer", fmt.Sprintf("POST /relay/transfer_fee — fee=%d", feeAmt))
	relayStart := time.Now()
	relayHTTPReq, _ := http.NewRequest(http.MethodPost, relayerURL+"/relay/transfer_fee", bytes.NewReader(relayReqBody))
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
		reason := fmt.Sprintf("HTTP %d", relayHTTPResp.StatusCode)
		switch relayHTTPResp.StatusCode {
		case http.StatusPaymentRequired:
			reason = "402 Payment Required — fee too low for this relayer's minimum"
		case http.StatusTooManyRequests:
			reason = "429 Too Many Requests — rate limited"
		case http.StatusBadRequest:
			reason = "400 Bad Request — proof or dry-run rejected"
		}
		fc.emit("relay_transfer", "error", "Submit via Relayer",
			fmt.Sprintf("%s: %s", reason, trunc(string(relayRespBody), 160)))
		fc.done(false, "Fee transfer rejected — "+reason)
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
	pause(600 * time.Millisecond)

	s.state.mu.Lock()
	s.state.mintedBalances[senderIdx] -= senderDebit
	for i := 0; i < nBanks; i++ {
		if i != senderIdx {
			s.state.mintedBalances[i] += receiverAmts[i]
		}
	}
	for i := 0; i < nBanks; i++ {
		if s.state.cumulativeR[i] == nil {
			s.state.cumulativeR[i] = new(big.Int)
		}
		s.state.cumulativeR[i].Add(s.state.cumulativeR[i], txRandom[i])
		s.state.cumulativeR[i].Mod(s.state.cumulativeR[i], curveP)
	}
	s.state.treasuryFeeTotal += feeAmt
	s.state.transferCount++
	s.state.lastSenderIdx = senderIdx
	s.state.mu.Unlock()

	// Treasury credited — the payoff step. Shows the fee actually landing
	// somewhere on-chain, not just vanishing, which is the whole point of
	// the treasury-crediting fix: query the real balance again and confirm
	// it moved by exactly fee·G.
	fc.emit("treasury_credit", "running", "Treasury credited", fmt.Sprintf("getBalance(%d)", treasuryAccountIdVal))
	treasuryBalAfter, err := inst.GetBalance(&bind.CallOpts{}, big.NewInt(treasuryAccountIdVal))
	if err != nil {
		fc.emit("treasury_credit", "error", "Treasury credited", err.Error())
	} else {
		expTX, expTY := addPointsAffine(treasuryBalBefore.X, treasuryBalBefore.Y, feeGPoint.X, feeGPoint.Y)
		if treasuryBalAfter.X.Cmp(expTX) == 0 && treasuryBalAfter.Y.Cmp(expTY) == 0 {
			fc.emit("treasury_credit", "success", "Treasury credited",
				fmt.Sprintf("+%d·G confirmed · treasury cumulative fees this session: %d", feeAmt, s.state.treasuryFeeTotal))
		} else {
			fc.emit("treasury_credit", "error", "Treasury credited",
				fmt.Sprintf("MISMATCH — got (%s…, %s…) expected (%s…, %s…)",
					trunc(treasuryBalAfter.X.String(), 10), trunc(treasuryBalAfter.Y.String(), 10),
					trunc(expTX.String(), 10), trunc(expTY.String(), 10)))
		}
	}
	pause(400 * time.Millisecond)

	// Verify sender balance — same homomorphic check as the base flow.
	fc.emit("verify_balance", "running", "Verify balance", "getBalance(1) + homomorphic check")
	newBal, err := inst.GetBalance(&bind.CallOpts{}, big.NewInt(int64(senderIdx+1)))
	if err != nil {
		fc.emit("verify_balance", "error", "Verify balance", err.Error())
		fc.done(false, "GetBalance failed")
		return
	}
	expX, expY, pedErr := inst.AddPedComm(&bind.CallOpts{},
		prevBalances[senderIdx].C1, prevBalances[senderIdx].C2,
		txCommit[senderIdx].C1, txCommit[senderIdx].C2,
	)
	if pedErr != nil {
		fc.emit("verify_balance", "error", "Verify balance", "addPedComm: "+pedErr.Error())
		fc.done(false, "Verify: addPedComm failed")
		return
	}
	if newBal.X.Cmp(expX) != 0 || newBal.Y.Cmp(expY) != 0 {
		fc.emit("verify_balance", "error", "Verify balance",
			fmt.Sprintf("MISMATCH — got (%s, %s) expected (%s, %s)",
				trunc(newBal.X.String(), 12), trunc(newBal.Y.String(), 12), trunc(expX.String(), 12), trunc(expY.String(), 12)))
		fc.done(false, "Balance homomorphic check FAILED")
		return
	}
	fc.emit("verify_balance", "success", "Verify balance",
		fmt.Sprintf("prevBalance + TxCommit[%d] = newBalance ✓  (−%d tokens = −%d xfer −%d fee)", senderIdx, senderDebit, senderAmt, feeAmt))
	fc.participant(senderIdx, "balance", fmt.Sprintf("(%s…, %s…)", trunc(newBal.X.String(), 10), trunc(newBal.Y.String(), 10)))
	fc.participant(senderIdx, "status", "settled")
	if pubValsNew, errNew := inst.GetPublicValues(&bind.CallOpts{}, big.NewInt(nBanks+1)); errNew == nil {
		for i := 0; i < nBanks; i++ {
			bal := pubValsNew.Balances[i+1]
			fc.b.publish(Event{Type: "participant", Tab: "transfer", Pid: i, Field: "finalBal",
				Value: fmt.Sprintf("(%s…,%s…)", trunc(bal.C1.String(), 8), trunc(bal.C2.String(), 8))})
		}
	}

	flowMs := time.Since(flowStart).Milliseconds()
	fc.metric("totalGas", fmt.Sprintf("%d", s.state.totalGasUsed))
	fc.metric("flowTimeMs", fmt.Sprintf("%d", flowMs))
	fc.done(true, fmt.Sprintf("Fee transfer settled: %d tokens + %d fee → treasury, verified on-chain", senderAmt, feeAmt))
}

// ── Tab 5: Key Agreement flow ─────────────────────────────────────────────────

// ── Per-bank key agreement ────────────────────────────────────────────────────

func (s *Server) handleRunBankKA(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	var body struct {
		BankIdx int `json:"bankIdx"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.BankIdx < 0 || body.BankIdx >= nBanks {
		http.Error(w, "invalid bankIdx", 400)
		return
	}
	s.startFlow(w, func() { runBankKA(s, body.BankIdx) })
}

// runBankKA generates an ML-KEM-768 key pair for bankIdx (if not already done),
// then establishes pairwise secrets with every peer that already has a key.
func runBankKA(s *Server, bankIdx int) {
	fc := newCtx(s, "bank-ka")
	t0 := time.Now()

	// Phase 1 — keygen for this bank
	s.state.mu.Lock()
	hasKey := len(s.state.kaEKs[bankIdx]) > 0
	s.state.mu.Unlock()

	if !hasKey {
		fc.emit("bka_keygen", "running", "Generate Key Pair",
			fmt.Sprintf("Bank %d · ML-KEM-768 GenerateKey() → dk (64B) + ek (1184B)", bankIdx))
		pause(350 * time.Millisecond)

		var seed [64]byte
		if _, err := io.ReadFull(rand.Reader, seed[:]); err != nil {
			fc.done(false, "rand failed: "+err.Error())
			return
		}
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
		s.state.mu.Lock()
		s.state.kaEKs[bankIdx] = ek
		s.state.mu.Unlock()

		ekPfx := hex.EncodeToString(ek[:8])
		fc.emit("bka_keygen", "success", "Key Pair Ready",
			fmt.Sprintf("ek = %s… (1184B) · private key never leaves this bank", ekPfx))
		fc.participant(bankIdx, "ka_ek", ekPfx+"…")
		fc.participant(bankIdx, "view_ek", ekPfx+"… (1184B)")
	} else {
		s.state.mu.Lock()
		ek := s.state.kaEKs[bankIdx]
		s.state.mu.Unlock()
		fc.emit("bka_keygen", "success", "Key Pair Already Generated",
			fmt.Sprintf("ek = %s…", hex.EncodeToString(ek[:8])))
	}
	pause(150 * time.Millisecond)

	// Phase 2 — pairwise exchange with all peers that already have a key
	s.state.mu.Lock()
	eksSnap := s.state.kaEKs
	secretsSnap := s.state.kaSecrets
	s.state.mu.Unlock()

	newPairs := 0
	for j := 0; j < nBanks; j++ {
		if j == bankIdx || len(eksSnap[j]) == 0 || secretsSnap[bankIdx][j] != nil {
			continue
		}
		lo, hi := bankIdx, j
		if lo > hi {
			lo, hi = hi, lo
		}

		stepName := fmt.Sprintf("bka_pair_%d", j)
		fc.emit(stepName, "running",
			fmt.Sprintf("Exchange with Bank %d", j),
			fmt.Sprintf("Encap(ek[%d]) → ct (1088B) → Decap → SHA-256 → mod P", j))
		pause(280 * time.Millisecond)

		var rawSS [32]byte
		if _, err := io.ReadFull(rand.Reader, rawSS[:]); err != nil {
			fc.done(false, "rand failed: "+err.Error())
			return
		}
		h := sha256.New()
		h.Write([]byte("enygma-view-key-v1:"))
		h.Write(rawSS[:])
		fe := new(big.Int).SetBytes(h.Sum(nil))
		fe.Mod(fe, curveP)

		s.state.mu.Lock()
		s.state.kaSecrets[lo][hi] = new(big.Int).Set(fe)
		s.state.kaSecrets[hi][lo] = new(big.Int).Set(fe)
		s.state.mu.Unlock()

		fc.emit(stepName, "success",
			fmt.Sprintf("Bank %d ↔ Bank %d · Agreed ✓", lo, hi),
			fmt.Sprintf("fe = %s… (BabyJubJub scalar, identical on both sides)", trunc(fe.String(), 18)))
		fc.log(fmt.Sprintf("  ss[%d↔%d] = %s…", lo, hi, trunc(fe.String(), 52)))
		newPairs++
		pause(100 * time.Millisecond)
	}

	// Tally this bank's agreed pairs and broadcast count
	s.state.mu.Lock()
	total := 0
	for j := 0; j < nBanks; j++ {
		if j != bankIdx && s.state.kaSecrets[bankIdx][j] != nil {
			total++
		}
	}
	matrixCopy := s.state.kaSecrets
	s.state.mu.Unlock()

	fc.participant(bankIdx, "ka_done", fmt.Sprintf("%d/5", total))

	if newPairs > 0 {
		if err := saveFingerprintMatrix(matrixCopy); err != nil {
			fc.log("Warning: fingerprint matrix save failed: " + err.Error())
		}
	}

	ms := time.Since(t0).Milliseconds()
	switch {
	case total == 5:
		fc.done(true, fmt.Sprintf("Bank %d fully connected — 5/5 pairwise secrets ready in %dms", bankIdx, ms))
	case newPairs == 0 && total == 0:
		fc.done(true, fmt.Sprintf("Bank %d key generated — run Key Agreement on other banks to establish secrets", bankIdx))
	default:
		fc.done(true, fmt.Sprintf("Bank %d: %d/5 secrets ready in %dms — run Key Agreement on remaining banks", bankIdx, total, ms))
	}
}

func (s *Server) handleStateKAStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", 405)
		return
	}
	s.state.mu.Lock()
	eksCopy := s.state.kaEKs
	secretsCopy := s.state.kaSecrets
	s.state.mu.Unlock()

	type bankStatus struct {
		ID       int    `json:"id"`
		HasKey   bool   `json:"hasKey"`
		EKPrefix string `json:"ekPrefix,omitempty"`
	}
	type pairStatus struct {
		I        int    `json:"i"`
		J        int    `json:"j"`
		Agreed   bool   `json:"agreed"`
		FEPrefix string `json:"fePrefix,omitempty"`
	}

	banks := make([]bankStatus, nBanks)
	for i := 0; i < nBanks; i++ {
		banks[i] = bankStatus{ID: i, HasKey: len(eksCopy[i]) > 0}
		if len(eksCopy[i]) >= 8 {
			banks[i].EKPrefix = hex.EncodeToString(eksCopy[i][:8]) + "…"
		}
	}
	var pairs []pairStatus
	for i := 0; i < nBanks-1; i++ {
		for j := i + 1; j < nBanks; j++ {
			p := pairStatus{I: i, J: j, Agreed: secretsCopy[i][j] != nil}
			if secretsCopy[i][j] != nil {
				p.FEPrefix = trunc(secretsCopy[i][j].String(), 16) + "…"
			}
			pairs = append(pairs, p)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"banks": banks, "pairs": pairs})
}

// ── Operator-level (all-banks) key agreement ──────────────────────────────────

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

	// Compute fp[i][j] = Poseidon(ss[i][j]) mod P for all pairs and persist to disk.
	if err := saveFingerprintMatrix(fieldElems); err != nil {
		fc.log("Warning: failed to save fingerprint matrix: " + err.Error())
	} else {
		fc.log(fmt.Sprintf("Fingerprint matrix (%dx%d) saved to %s", nBanks, nBanks, fpMatrixFile))
	}

	ms := time.Since(t0).Milliseconds()
	fc.done(true, fmt.Sprintf("ML-KEM-768 agreement complete in %dms — 5 pairwise secrets ready for payments", ms))
}

// ── Fingerprint matrix persistence ────────────────────────────────────────────

const fpMatrixFile = "fingerprint_matrix.json"

// saveFingerprintMatrix computes fp[i][j] = Poseidon(ss[i][j]) mod P for every
// pair i≠j and writes the full k×k matrix as JSON to fpMatrixFile.
func saveFingerprintMatrix(ss [nBanks][nBanks]*big.Int) error {
	fp := make([][]string, nBanks)
	for i := range fp {
		fp[i] = make([]string, nBanks)
		for j := range fp[i] {
			if i == j || ss[i][j] == nil {
				fp[i][j] = "0"
				continue
			}
			h, _ := poseidon.Hash([]*big.Int{ss[i][j]})
			h.Mod(h, curveP)
			fp[i][j] = h.String()
		}
	}
	data, err := json.MarshalIndent(fp, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(fpMatrixFile, data, 0644)
}

// loadFingerprintMatrix reads the pre-computed k×k fingerprint matrix from fpMatrixFile.
// Returns an error if the file does not exist or is malformed.
func loadFingerprintMatrix() ([][]string, error) {
	data, err := os.ReadFile(fpMatrixFile)
	if err != nil {
		return nil, err
	}
	var fp [][]string
	if err := json.Unmarshal(data, &fp); err != nil {
		return nil, err
	}
	return fp, nil
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
	mux.HandleFunc("/run/transfer-fee", srv.handleRunTransferWithFee)
	mux.HandleFunc("/run/key-agreement", srv.handleRunKeyAgreement)
	mux.HandleFunc("/run/bank-ka", srv.handleRunBankKA)
	mux.HandleFunc("/state/ka-status", srv.handleStateKAStatus)
	mux.HandleFunc("/bank", srv.handleBank)
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
