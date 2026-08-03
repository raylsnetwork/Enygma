// Enygma DvP · NFT ↔ ERC20 Swap · Live Demo
//
// A local HTTP server that runs the full DvP (Delivery vs Payment) atomic
// swap flow and streams each step as a Server-Sent Event to a live dashboard.
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
//	1. Hardhat node  :  npx hardhat node                       (from enygma_dvp/)
//	2. Deploy+init   :  see enygma_dvp/MEMORY.md / scripts/
//	3. Gnark server  :  cd gnark_circuits && go run main.go     (must include DvP keys)
package main

import (
	_ "embed"
	"context"
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
	go func() {
		defer s.runLock.Unlock()
		runFlow(s.broker)
	}()
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"status":"started"}`)
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

// ── Main flow ─────────────────────────────────────────────────────────────────

func runFlow(b *Broker) {
	b.publish(Event{Type: "reset"})
	time.Sleep(80 * time.Millisecond)

	ctx := context.Background()

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

	var totalGasUsed uint64
	var proofGenMs int64
	flowStart := time.Now()

	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Dir(thisFile)

	// ── Step: Prerequisites ───────────────────────────────────────────────────
	emit("prereqs", "running", "Check prerequisites", "Probing chain · gnark server…")
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
		fail("prereqs", "Check prerequisites", fmt.Errorf("services not running: %s", strings.Join(missing, ", ")))
		return
	}
	logMsg("", "✓ chain and gnark server are both reachable")

	receipts, err := loadReceipts(dir)
	if err != nil {
		fail("prereqs", "Check prerequisites", err)
		return
	}
	erc20VaultAddr := common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress)
	erc20Addr := common.HexToAddress(receipts["ERC20"].ContractAddress)
	erc721VaultAddr := common.HexToAddress(receipts["Erc721CoinVault"].ContractAddress)
	erc721Addr := common.HexToAddress(receipts["ERC721"].ContractAddress)
	dvpAddr := common.HexToAddress(receipts["EnygmaDvp"].ContractAddress)

	proto("erc20VaultAddr", erc20VaultAddr.Hex())
	proto("erc721VaultAddr", erc721VaultAddr.Hex())
	proto("dvpAddr", dvpAddr.Hex())
	logMsg("chain", fmt.Sprintf("Erc20CoinVault:  %s", shortHex(erc20VaultAddr.Hex(), 18)))
	logMsg("chain", fmt.Sprintf("Erc721CoinVault: %s", shortHex(erc721VaultAddr.Hex(), 18)))
	logMsg("chain", fmt.Sprintf("EnygmaDvp:       %s", shortHex(dvpAddr.Hex(), 18)))

	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		fail("prereqs", "Check prerequisites", fmt.Errorf("ethclient.Dial: %w", err))
		return
	}
	defer client.Close()

	erc20VaultABI, err := loadABI(dir, "core/contracts/vaults/Erc20CoinVault.sol/Erc20CoinVault.json")
	if err != nil {
		fail("prereqs", "Check prerequisites", err)
		return
	}
	erc20ABI, err := loadABI(dir, "erc20/contracts/RaylsERC20.sol/RaylsERC20.json")
	if err != nil {
		fail("prereqs", "Check prerequisites", err)
		return
	}
	erc721VaultABI, err := loadABI(dir, "core/contracts/vaults/Erc721CoinVault.sol/Erc721CoinVault.json")
	if err != nil {
		fail("prereqs", "Check prerequisites", err)
		return
	}
	erc721ABI, err := loadABI(dir, "erc721/contracts/RaylsERC721.sol/RaylsERC721.json")
	if err != nil {
		fail("prereqs", "Check prerequisites", err)
		return
	}
	dvpABI, err := loadABI(dir, "core/contracts/EnygmaDvp.sol/EnygmaDvp.json")
	if err != nil {
		fail("prereqs", "Check prerequisites", err)
		return
	}

	erc20Vault := bind.NewBoundContract(erc20VaultAddr, erc20VaultABI, client, client, client)
	erc20Token := bind.NewBoundContract(erc20Addr, erc20ABI, client, client, client)
	erc721Vault := bind.NewBoundContract(erc721VaultAddr, erc721VaultABI, client, client, client)
	erc721Token := bind.NewBoundContract(erc721Addr, erc721ABI, client, client, client)
	dvp := bind.NewBoundContract(dvpAddr, dvpABI, client, client, client)

	auth, err := makeAuth()
	if err != nil {
		fail("prereqs", "Check prerequisites", err)
		return
	}
	panel(0, "ethAddr", auth.From.Hex())
	panel(1, "ethAddr", auth.From.Hex())

	gnarkClient := core.NewGnarkClient(gnarkURL)
	emit("prereqs", "success", "Check prerequisites", "All services running")
	pause(1200 * time.Millisecond)

	// Randomize the NFT tokenId each run so repeated demo runs never collide.
	nftTokenIdInt := int64(1_000_000 + rand.Intn(900_000))
	erc20AmountBig := big.NewInt(erc20Amount)
	erc20TokenIdBig := big.NewInt(erc20TokenId)
	nftTokenIdBig := big.NewInt(nftTokenIdInt)
	nftAmountBig := big.NewInt(nftAmount)

	// ── Step: Key generation ──────────────────────────────────────────────────
	emit("keygen", "running", "Generate key pairs", "Alice & Bob each generate spend+view keypairs…")
	pause(1800 * time.Millisecond)

	aliceSpend, err := core.NewSpendKeyPair()
	if err != nil {
		fail("keygen", "Generate key pairs", err)
		return
	}
	aliceView, err := core.NewViewKeyPair()
	if err != nil {
		fail("keygen", "Generate key pairs", err)
		return
	}
	bobSpend, err := core.NewSpendKeyPair()
	if err != nil {
		fail("keygen", "Generate key pairs", err)
		return
	}
	bobView, err := core.NewViewKeyPair()
	if err != nil {
		fail("keygen", "Generate key pairs", err)
		return
	}

	panel(0, "pkSpend", "0x"+aliceSpend.PublicKey.Text(16))
	panel(1, "pkSpend", "0x"+bobSpend.PublicKey.Text(16))
	panel(0, "pkView", "0x"+common.Bytes2Hex(aliceView.EncapsKey))
	panel(1, "pkView", "0x"+common.Bytes2Hex(bobView.EncapsKey))
	logMsg("key", fmt.Sprintf("Alice pk_spend: 0x%s…", aliceSpend.PublicKey.Text(16)[:16]))
	logMsg("key", fmt.Sprintf("Bob   pk_spend: 0x%s…", bobSpend.PublicKey.Text(16)[:16]))
	emit("keygen", "success", "Generate key pairs", "BabyJubJub spend keys + ML-KEM-768 view keypairs generated")
	pause(1600 * time.Millisecond)

	// ── Step: Alice deposits ERC20 ────────────────────────────────────────────
	emit("deposit_erc20", "running", "Alice Deposits ERC20", fmt.Sprintf("Alice mints and deposits %d USDT into the vault…", erc20Amount))

	ssAlice, capsuleAlice, err := core.Encapsulate(aliceView.EncapsKey)
	if err != nil {
		fail("deposit_erc20", "Alice Deposits ERC20", fmt.Errorf("Encapsulate: %w", err))
		return
	}
	aliceSaltBytes, err := core.DerivePaymentSalt(ssAlice)
	if err != nil {
		fail("deposit_erc20", "Alice Deposits ERC20", err)
		return
	}
	aliceEncKey, err := core.DerivePaymentKey(ssAlice)
	if err != nil {
		fail("deposit_erc20", "Alice Deposits ERC20", err)
		return
	}
	aliceSaltField := core.SaltBToField(aliceSaltBytes)

	aliceCommitment, err := core.Erc20CommitmentV2(aliceSpend.PublicKey, aliceSaltField, erc20AmountBig, erc20TokenIdBig)
	if err != nil {
		fail("deposit_erc20", "Alice Deposits ERC20", err)
		return
	}
	aliceDepositEnc, err := core.EncryptPayload(aliceEncKey, erc20TokenIdBig, erc20AmountBig)
	if err != nil {
		fail("deposit_erc20", "Alice Deposits ERC20", err)
		return
	}

	mintErc20Tx, err := erc20Token.Transact(auth, "mint", auth.From, new(big.Int).Mul(erc20AmountBig, big.NewInt(10)))
	if err != nil {
		fail("deposit_erc20", "Alice Deposits ERC20", fmt.Errorf("ERC20.mint: %w", err))
		return
	}
	if _, err := bind.WaitMined(ctx, client, mintErc20Tx); err != nil {
		fail("deposit_erc20", "Alice Deposits ERC20", fmt.Errorf("wait mint: %w", err))
		return
	}
	approveErc20Tx, err := erc20Token.Transact(auth, "approve", erc20VaultAddr, erc20AmountBig)
	if err != nil {
		fail("deposit_erc20", "Alice Deposits ERC20", fmt.Errorf("ERC20.approve: %w", err))
		return
	}
	if _, err := bind.WaitMined(ctx, client, approveErc20Tx); err != nil {
		fail("deposit_erc20", "Alice Deposits ERC20", fmt.Errorf("wait approve: %w", err))
		return
	}
	depositErc20Tx, err := erc20Vault.Transact(auth, "depositV2",
		[]*big.Int{erc20AmountBig, aliceSpend.PublicKey, aliceSaltField, erc20TokenIdBig}, capsuleAlice, aliceDepositEnc)
	if err != nil {
		fail("deposit_erc20", "Alice Deposits ERC20", fmt.Errorf("depositV2: %w", err))
		return
	}
	depositErc20Receipt, err := bind.WaitMined(ctx, client, depositErc20Tx)
	if err != nil {
		fail("deposit_erc20", "Alice Deposits ERC20", fmt.Errorf("wait depositV2: %w", err))
		return
	}
	totalGasUsed += depositErc20Receipt.GasUsed
	logMsg("chain", fmt.Sprintf("Erc20CoinVault.depositV2() → block %d  gas %s", depositErc20Receipt.BlockNumber, formatNum(depositErc20Receipt.GasUsed)))
	logMsg("key", fmt.Sprintf("Alice commitment: 0x%s…", aliceCommitment.Text(16)[:16]))
	panel(0, "commitment", "0x"+aliceCommitment.Text(16))
	panel(0, "balance", fmt.Sprintf("%d USDT (deposited)", erc20Amount))
	emit("deposit_erc20", "success", "Alice Deposits ERC20", fmt.Sprintf("Alice deposited %d USDT — Poseidon commitment recorded on-chain (block %d)", erc20Amount, depositErc20Receipt.BlockNumber))
	pause(1600 * time.Millisecond)

	// ── Step: Bob deposits NFT ────────────────────────────────────────────────
	emit("deposit_nft", "running", "Bob Deposits NFT", fmt.Sprintf("Bob mints and deposits ticket tokenId=%d into the NFT vault…", nftTokenIdInt))

	bobNftSalt, err := core.RandomInField()
	if err != nil {
		fail("deposit_nft", "Bob Deposits NFT", err)
		return
	}
	bobCommitment, err := core.Erc721Commitment(nftTokenIdBig, bobSpend.PublicKey, bobNftSalt)
	if err != nil {
		fail("deposit_nft", "Bob Deposits NFT", err)
		return
	}

	mintNftTx, err := erc721Token.Transact(auth, "mint", auth.From, nftTokenIdBig)
	if err != nil {
		fail("deposit_nft", "Bob Deposits NFT", fmt.Errorf("ERC721.mint: %w", err))
		return
	}
	if _, err := bind.WaitMined(ctx, client, mintNftTx); err != nil {
		fail("deposit_nft", "Bob Deposits NFT", fmt.Errorf("wait mint: %w", err))
		return
	}
	approveNftTx, err := erc721Token.Transact(auth, "approve", erc721VaultAddr, nftTokenIdBig)
	if err != nil {
		fail("deposit_nft", "Bob Deposits NFT", fmt.Errorf("ERC721.approve: %w", err))
		return
	}
	if _, err := bind.WaitMined(ctx, client, approveNftTx); err != nil {
		fail("deposit_nft", "Bob Deposits NFT", fmt.Errorf("wait approve: %w", err))
		return
	}
	depositNftTx, err := erc721Vault.Transact(auth, "deposit", []*big.Int{nftTokenIdBig, bobSpend.PublicKey, bobNftSalt})
	if err != nil {
		fail("deposit_nft", "Bob Deposits NFT", fmt.Errorf("deposit: %w", err))
		return
	}
	depositNftReceipt, err := bind.WaitMined(ctx, client, depositNftTx)
	if err != nil {
		fail("deposit_nft", "Bob Deposits NFT", fmt.Errorf("wait deposit: %w", err))
		return
	}
	totalGasUsed += depositNftReceipt.GasUsed
	logMsg("chain", fmt.Sprintf("Erc721CoinVault.deposit() → block %d  gas %s", depositNftReceipt.BlockNumber, formatNum(depositNftReceipt.GasUsed)))
	logMsg("key", fmt.Sprintf("Bob commitment: 0x%s…", bobCommitment.Text(16)[:16]))
	panel(1, "commitment", "0x"+bobCommitment.Text(16))
	panel(1, "balance", fmt.Sprintf("ticket #%d (deposited)", nftTokenIdInt))
	emit("deposit_nft", "success", "Bob Deposits NFT", fmt.Sprintf("Bob deposited ticket #%d — Poseidon commitment recorded on-chain (block %d)", nftTokenIdInt, depositNftReceipt.BlockNumber))
	pause(1600 * time.Millisecond)

	// ── Merkle proofs (one tree per vault) ────────────────────────────────────
	erc20Mt, err := loadVaultMerkleTree(ctx, client, erc20VaultAddr)
	if err != nil {
		fail("deposit_nft", "Bob Deposits NFT", err)
		return
	}
	aliceMerkleProof, err := erc20Mt.GenerateProof(aliceCommitment)
	if err != nil {
		fail("deposit_nft", "Bob Deposits NFT", fmt.Errorf("GenerateProof (Alice ERC20): %w", err))
		return
	}
	nftMt, err := loadVaultMerkleTree(ctx, client, erc721VaultAddr)
	if err != nil {
		fail("deposit_nft", "Bob Deposits NFT", err)
		return
	}
	bobMerkleProof, err := nftMt.GenerateProof(bobCommitment)
	if err != nil {
		fail("deposit_nft", "Bob Deposits NFT", fmt.Errorf("GenerateProof (Bob NFT): %w", err))
		return
	}
	panel(0, "merkleRoot", "0x"+aliceMerkleProof.Root.Text(16))
	panel(1, "merkleRoot", "0x"+bobMerkleProof.Root.Text(16))

	// ── Step: DvP Initiator (Alice) ───────────────────────────────────────────
	emit("dvp_initiator", "running", "DvP Initiator Proof", "Alice proves ownership of her USDT note and locks in the swap terms…")

	proofStart := time.Now()
	initiatorResult, err := gnarkClient.DvPInitiatorProof(
		core.KeyPair{PrivateKey: aliceSpend.PrivateKey, PublicKey: aliceSpend.PublicKey},
		aliceSaltField,
		erc20AmountBig, erc20TokenIdBig,
		bobSpend.PublicKey, bobView.EncapsKey,
		nftAmountBig, nftTokenIdBig,
		big.NewInt(0),
		aliceMerkleProof, merkleDepth,
	)
	if err != nil {
		fail("dvp_initiator", "DvP Initiator Proof", fmt.Errorf("DvPInitiatorProof: %w", err))
		return
	}
	if len(initiatorResult.Proof) != 8 {
		fail("dvp_initiator", "DvP Initiator Proof", fmt.Errorf("expected 8-element proof, got %d", len(initiatorResult.Proof)))
		return
	}
	proofGenMs = time.Since(proofStart).Milliseconds()
	logMsg("zk", fmt.Sprintf("DvP Initiator proof generated in %d ms", proofGenMs))
	logMsg("zk", fmt.Sprintf("commitB (Bob receives %d USDT):  0x%s…", erc20Amount, initiatorResult.CommitB.Text(16)[:16]))
	logMsg("zk", fmt.Sprintf("commitA (Alice receives ticket): 0x%s…", initiatorResult.CommitA.Text(16)[:16]))
	proto("proofTimeMs", fmt.Sprintf("%d", proofGenMs))
	panel(0, "commitB", "0x"+initiatorResult.CommitB.Text(16))
	panel(0, "commitA", "0x"+initiatorResult.CommitA.Text(16))
	emit("dvp_initiator", "success", "DvP Initiator Proof", fmt.Sprintf("Groth16 proof generated in %d ms — swap terms locked, note encrypted for Bob", proofGenMs))
	pause(1800 * time.Millisecond)

	// ── Step: Bob scans and verifies ──────────────────────────────────────────
	emit("bob_scan", "running", "Bob Scans & Verifies", "Bob decapsulates Alice's message and verifies the swap terms…")
	pause(1400 * time.Millisecond)

	ssBob, err := core.Decapsulate(bobView.DecapsKey, initiatorResult.CipherText)
	if err != nil {
		fail("bob_scan", "Bob Scans & Verifies", fmt.Errorf("Decapsulate: %w", err))
		return
	}
	bobSaltBBytes, err := core.DerivePaymentSalt(ssBob)
	if err != nil {
		fail("bob_scan", "Bob Scans & Verifies", err)
		return
	}
	bobSaltABytes, err := core.DeriveDvpSaltInit(ssBob)
	if err != nil {
		fail("bob_scan", "Bob Scans & Verifies", err)
		return
	}
	bobEncKey, err := core.DerivePaymentKey(ssBob)
	if err != nil {
		fail("bob_scan", "Bob Scans & Verifies", err)
		return
	}
	saltBField := core.SaltBToField(bobSaltBBytes)
	saltAField := core.SaltBToField(bobSaltABytes)

	decTokenId, decAmount, err := core.DecryptPayload(bobEncKey, initiatorResult.EncTxData)
	if err != nil {
		fail("bob_scan", "Bob Scans & Verifies", fmt.Errorf("DecryptPayload: %w", err))
		return
	}
	logMsg("key", fmt.Sprintf("Bob decrypted: tokenId=%s amount=%s (Alice's USDT)", decTokenId, decAmount))

	expectedCommitB, err := core.Erc20CommitmentV2(bobSpend.PublicKey, saltBField, decAmount, decTokenId)
	if err != nil {
		fail("bob_scan", "Bob Scans & Verifies", err)
		return
	}
	if expectedCommitB.Cmp(initiatorResult.CommitB) != 0 {
		fail("bob_scan", "Bob Scans & Verifies", fmt.Errorf("commitB mismatch"))
		return
	}
	expectedCommitA, err := core.Erc20CommitmentV2(aliceSpend.PublicKey, saltAField, nftAmountBig, nftTokenIdBig)
	if err != nil {
		fail("bob_scan", "Bob Scans & Verifies", err)
		return
	}
	if expectedCommitA.Cmp(initiatorResult.CommitA) != 0 {
		fail("bob_scan", "Bob Scans & Verifies", fmt.Errorf("commitA mismatch"))
		return
	}
	logMsg("key", "commitB and commitA both verified ✓ — swap terms match")
	emit("bob_scan", "success", "Bob Scans & Verifies", fmt.Sprintf("Bob confirmed he'll receive %s USDT, and Alice will receive the ticket", decAmount))
	pause(1400 * time.Millisecond)

	// ── Step: DvP Destination (Bob) ───────────────────────────────────────────
	emit("dvp_destination", "running", "DvP Destination Proof", "Bob proves ownership of his ticket and accepts the swap…")

	destProofStart := time.Now()
	destinationResult, err := gnarkClient.DvPDestinationProof(
		core.KeyPair{PrivateKey: bobSpend.PrivateKey, PublicKey: bobSpend.PublicKey},
		bobNftSalt, nftAmountBig, nftTokenIdBig,
		aliceSpend.PublicKey, saltAField, saltBField,
		decAmount, decTokenId,
		initiatorResult.CommitA,
		big.NewInt(0),
		bobMerkleProof, merkleDepth,
	)
	if err != nil {
		fail("dvp_destination", "DvP Destination Proof", fmt.Errorf("DvPDestinationProof: %w", err))
		return
	}
	if len(destinationResult.Proof) != 8 {
		fail("dvp_destination", "DvP Destination Proof", fmt.Errorf("expected 8-element proof, got %d", len(destinationResult.Proof)))
		return
	}
	destProofMs := time.Since(destProofStart).Milliseconds()
	logMsg("zk", fmt.Sprintf("DvP Destination proof generated in %d ms", destProofMs))
	emit("dvp_destination", "success", "DvP Destination Proof", fmt.Sprintf("Groth16 proof generated in %d ms — Bob has accepted the swap", destProofMs))
	pause(1600 * time.Millisecond)

	// ── Settlement receipts ────────────────────────────────────────────────────
	aliceProof8, err := proofStringsToOnchain(initiatorResult.Proof)
	if err != nil {
		fail("dvp_destination", "DvP Destination Proof", err)
		return
	}
	aliceReceipt := onchainProofReceipt{
		Proof:           aliceProof8,
		Statement:       initiatorResult.Statement,
		NumberOfInputs:  big.NewInt(int64(initiatorResult.NumberOfInputs)),
		NumberOfOutputs: big.NewInt(int64(initiatorResult.NumberOfOutputs)),
	}
	bobProof8, err := proofStringsToOnchain(destinationResult.Proof)
	if err != nil {
		fail("dvp_destination", "DvP Destination Proof", err)
		return
	}
	bobReceipt := onchainProofReceipt{
		Proof:           bobProof8,
		Statement:       destinationResult.Statement,
		NumberOfInputs:  big.NewInt(int64(destinationResult.NumberOfInputs)),
		NumberOfOutputs: big.NewInt(int64(destinationResult.NumberOfOutputs)),
	}

	// ── Step: Alice submits leg 1 ─────────────────────────────────────────────
	emit("settle_alice", "running", "Alice Submits (Leg 1/2)", "Alice submits her ERC20 leg — locks in the swap on-chain…")

	deadline := new(big.Int).SetInt64(time.Now().Unix() + deadlineWindowSeconds)
	aliceTx, err := dvp.Transact(auth, "submitPartialSettlement", aliceReceipt, big.NewInt(vaultIdErc20), big.NewInt(groupFungible), deadline)
	if err != nil {
		fail("settle_alice", "Alice Submits (Leg 1/2)", fmt.Errorf("submitPartialSettlement: %w", err))
		return
	}
	aliceTxReceipt, err := bind.WaitMined(ctx, client, aliceTx)
	if err != nil {
		fail("settle_alice", "Alice Submits (Leg 1/2)", fmt.Errorf("wait submitPartialSettlement: %w", err))
		return
	}
	totalGasUsed += aliceTxReceipt.GasUsed
	proto("aliceTxHash", aliceTx.Hash().Hex())
	proto("aliceBlock", fmt.Sprintf("%d", aliceTxReceipt.BlockNumber))
	proto("aliceGas", fmt.Sprintf("%d", aliceTxReceipt.GasUsed))
	logMsg("chain", fmt.Sprintf("EnygmaDvp.submitPartialSettlement() [Alice] → block %d  gas %s  tx %s…",
		aliceTxReceipt.BlockNumber, formatNum(aliceTxReceipt.GasUsed), shortHex(aliceTx.Hash().Hex(), 18)))
	emit("settle_alice", "success", "Alice Submits (Leg 1/2)", fmt.Sprintf("SwapInitiated on-chain (block %d) — waiting for Bob's leg, deadline in %ds", aliceTxReceipt.BlockNumber, deadlineWindowSeconds))
	pause(1800 * time.Millisecond)

	// ── Step: Bob submits leg 2 — atomic settlement ───────────────────────────
	emit("settle_bob", "running", "Bob Submits (Leg 2/2)", "Bob submits his ERC721 leg — settles the swap atomically…")

	bobTx, err := dvp.Transact(auth, "submitPartialSettlement", bobReceipt, big.NewInt(vaultIdErc721), big.NewInt(groupNonFung), big.NewInt(0))
	if err != nil {
		fail("settle_bob", "Bob Submits (Leg 2/2)", fmt.Errorf("submitPartialSettlement: %w", err))
		return
	}
	bobTxReceipt, err := bind.WaitMined(ctx, client, bobTx)
	if err != nil {
		fail("settle_bob", "Bob Submits (Leg 2/2)", fmt.Errorf("wait submitPartialSettlement: %w", err))
		return
	}
	totalGasUsed += bobTxReceipt.GasUsed
	proto("bobTxHash", bobTx.Hash().Hex())
	proto("bobBlock", fmt.Sprintf("%d", bobTxReceipt.BlockNumber))
	proto("bobGas", fmt.Sprintf("%d", bobTxReceipt.GasUsed))
	proto("totalGas", fmt.Sprintf("%d", totalGasUsed))
	logMsg("chain", fmt.Sprintf("EnygmaDvp.submitPartialSettlement() [Bob] → block %d  gas %s  tx %s…",
		bobTxReceipt.BlockNumber, formatNum(bobTxReceipt.GasUsed), shortHex(bobTx.Hash().Hex(), 18)))
	emit("settle_bob", "success", "Bob Submits (Leg 2/2)", fmt.Sprintf("Swap settled atomically on-chain (block %d)", bobTxReceipt.BlockNumber))
	pause(1200 * time.Millisecond)

	// ── Step: Verify ───────────────────────────────────────────────────────────
	emit("verify", "running", "Verify Settlement", "Checking on-chain events confirm both sides settled…")
	pause(800 * time.Millisecond)

	commitmentSig := crypto.Keccak256Hash([]byte("Commitment(uint256,uint256)"))
	nullifierSig := crypto.Keccak256Hash([]byte("Nullifier(uint256,uint256,uint256)"))
	foundCommitA, foundCommitB, nullifierCount := false, false, 0
	for _, l := range bobTxReceipt.Logs {
		if len(l.Topics) == 0 {
			continue
		}
		switch l.Topics[0] {
		case commitmentSig:
			if len(l.Topics) < 3 {
				continue
			}
			cmt := l.Topics[2].Big()
			if cmt.Cmp(initiatorResult.CommitB) == 0 {
				foundCommitB = true
			} else if cmt.Cmp(initiatorResult.CommitA) == 0 {
				foundCommitA = true
			}
		case nullifierSig:
			nullifierCount++
		}
	}
	if !foundCommitA || !foundCommitB {
		fail("verify", "Verify Settlement", fmt.Errorf("expected commitments not found in settlement events (commitA=%v commitB=%v)", foundCommitA, foundCommitB))
		return
	}
	logMsg("chain", fmt.Sprintf("✓ commitB inserted (Bob's USDT note) · commitA inserted (Alice's ticket note) · %d nullifier(s) spent", nullifierCount))
	panel(1, "balance", fmt.Sprintf("%d USDT (received)", erc20Amount))
	panel(0, "balance", fmt.Sprintf("ticket #%d (received)", nftTokenIdInt))
	emit("verify", "success", "Verify Settlement", "Both commitments confirmed on-chain — swap complete")

	flowMs := time.Since(flowStart).Milliseconds()
	proto("flowTimeMs", fmt.Sprintf("%d", flowMs))
	logMsg("", fmt.Sprintf("✓ Total flow: %d ms  total protocol gas: %s", flowMs, formatNum(totalGasUsed)))
	b.publish(Event{
		Type:   "done",
		Status: "success",
		Msg:    fmt.Sprintf("Swap complete: Alice's %d USDT ↔ Bob's ticket #%d", erc20Amount, nftTokenIdInt),
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

	log.Printf("Enygma DvP · NFT ↔ ERC20 Swap Demo listening on http://localhost%s", listenAddr)
	log.Printf("Open http://localhost%s in your browser", listenAddr)
	if err := http.ListenAndServe(listenAddr, mux); err != nil {
		log.Fatal(err)
	}
}
