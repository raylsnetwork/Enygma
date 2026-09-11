package server

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"sync"
	"time"

	"enygma_payments_relayer/config"
	enygma "enygma_payments_relayer/contracts"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-gonic/gin"
)

// EnygmaContract is the subset of *contracts.Enygma used by the Handler.
// Exported so external test packages can inject a mock without importing the handler internals.
// The concrete *enygma.Enygma satisfies this interface.
type EnygmaContract interface {
	Transfer(opts *bind.TransactOpts, commitmentDeltas []enygma.IEnygmaPoint, proof enygma.IEnygmaProof, usdrCommitmentDeltas []enygma.IEnygmaPoint, usdrProof enygma.IEnygmaUsdrProof, participantIds []*big.Int) (*types.Transaction, error)
	TransferWithFee(opts *bind.TransactOpts, commitmentDeltas []enygma.IEnygmaPoint, proof enygma.IEnygmaFeeProof, participantIds []*big.Int) (*types.Transaction, error)
}

// txTimeout is the maximum time to wait for a transaction to be mined.
const txTimeout = 45 * time.Second

// Handler holds all dependencies for the relay endpoints.
type Handler struct {
	cfg          *config.Config
	contractAddr string
	auth         *bind.TransactOpts // relayer's signing key — never leaves this process
	client       bind.DeployBackend // used only for bind.WaitMined; tests inject a mock
	instance     EnygmaContract     // Enygma contract binding; tests inject a mock
	txMu         sync.Mutex         // serializes on-chain submissions to prevent nonce races
	inFlight     sync.Map           // deduplicates concurrent identical submissions
}

// NewHandler wires up the handler: dials the chain, loads the signing key,
// resolves the contract address, and creates the bound contract instance.
func NewHandler(cfg *config.Config) (*Handler, error) {
	// Dial the chain.
	client, err := ethclient.Dial(cfg.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("ethclient.Dial(%s): %w", cfg.RPCURL, err)
	}

	// Load the relayer's signing key.
	privKey, err := crypto.HexToECDSA(cfg.RelayerPrivateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("parse relayer private key: %w", err)
	}

	// Build a transactor. Nonce and GasPrice are left nil so go-ethereum
	// fetches them automatically on each submission; GasLimit comes from config.
	auth, err := bind.NewKeyedTransactorWithChainID(privKey, cfg.ChainID)
	if err != nil {
		return nil, fmt.Errorf("build transactor: %w", err)
	}
	auth.Value = big.NewInt(0)
	auth.GasLimit = cfg.GasLimit

	// Resolve the contract address: env var takes priority, then address.json.
	contractAddrStr := cfg.ContractAddr
	if contractAddrStr == "" {
		contractAddrStr, err = readAddressJSON(cfg.AddressJSONPath)
		if err != nil {
			return nil, fmt.Errorf("resolve contract address: %w", err)
		}
	}

	// Bind the Enygma contract.
	instance, err := enygma.NewEnygma(common.HexToAddress(contractAddrStr), client)
	if err != nil {
		return nil, fmt.Errorf("bind Enygma contract at %s: %w", contractAddrStr, err)
	}

	return newHandlerDeps(cfg, contractAddrStr, auth, client, instance), nil
}

// newHandlerDeps constructs a Handler from pre-built dependencies (internal).
func newHandlerDeps(cfg *config.Config, contractAddr string, auth *bind.TransactOpts, client bind.DeployBackend, instance EnygmaContract) *Handler {
	return &Handler{
		cfg:          cfg,
		contractAddr: contractAddr,
		auth:         auth,
		client:       client,
		instance:     instance,
	}
}

// NewHandlerWithDeps constructs a Handler with pre-built dependencies.
// Exported for external test packages that inject mock contracts and backends.
// In production use NewHandler instead.
func NewHandlerWithDeps(cfg *config.Config, contractAddr string, auth *bind.TransactOpts, client bind.DeployBackend, contract EnygmaContract) *Handler {
	return newHandlerDeps(cfg, contractAddr, auth, client, contract)
}

// SetInFlight marks key as in-flight in the deduplication map.
// Exported so external test packages can simulate in-progress duplicate requests.
func (h *Handler) SetInFlight(key string, val any) { h.inFlight.Store(key, val) }

// DeleteInFlight removes key from the deduplication map.
// Exported for test cleanup after SetInFlight.
func (h *Handler) DeleteInFlight(key string) { h.inFlight.Delete(key) }

// ── Info ──────────────────────────────────────────────────────────────────────

// Info handles GET /relay/info.
func (h *Handler) Info(c *gin.Context) {
	c.JSON(http.StatusOK, InfoResponse{
		RelayerAddr:  h.auth.From.Hex(),
		ContractAddr: h.contractAddr,
		ChainID:      h.cfg.ChainID.Int64(),
	})
}

// ── Transfer ──────────────────────────────────────────────────────────────────

// RelayTransfer handles POST /relay/transfer.
//
// Calls Enygma.transfer(commitmentDeltas, proof, usdrCommitmentDeltas,
// usdrProof, participantIds). Used for confidential Enygma-to-Enygma
// balance updates (the enygma circuit), plus a second, independent USDr
// proof paying the relayer a fee, settled atomically in the same call.
// The main proof's public signal supports up to 80 elements (FingerPrint
// 6×6 layout); the USDr proof's supports up to 81 (one more — its public
// FeeAmount signal, appended last). Unused slots are zero-padded to fill
// the fixed-size arrays the contract expects. Both proofs share KIndex
// (the same k=6 anonymity-set participantIds).
func (h *Handler) RelayTransfer(c *gin.Context) {
	var req RelayTransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	proof8, err := parseProof8(req.Proof)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("proof: %v", err)})
		return
	}
	pubSig80, err := padPublicSignal80(req.PublicSignal)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("publicSignal: %v", err)})
		return
	}

	usdrProof8, err := parseProof8(req.UsdrProof)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("usdrProof: %v", err)})
		return
	}
	usdrPubSig81, err := padPublicSignal81(req.UsdrPublicSignal)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("usdrPublicSignal: %v", err)})
		return
	}

	commitments, err := parseCommitments(req.Commitments)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("commitments: %v", err)})
		return
	}
	usdrCommitments, err := parseCommitments(req.UsdrCommitments)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("usdrCommitments: %v", err)})
		return
	}
	kIndex := int64sToBI(req.KIndex)

	transferProof := enygma.IEnygmaProof{
		Proof:        proof8,
		PublicSignal: pubSig80,
	}
	usdrTransferProof := enygma.IEnygmaUsdrProof{
		Proof:        usdrProof8,
		PublicSignal: usdrPubSig81,
	}

	// Both proofs' first element in the dedup key — a resubmission of only
	// one leg (e.g. same main proof, different usdrProof) must not be
	// treated as identical to an in-flight submission of the pair.
	dedupKey := "transfer:" + req.Proof[0] + ":" + req.UsdrProof[0]
	if _, loaded := h.inFlight.LoadOrStore(dedupKey, struct{}{}); loaded {
		c.JSON(http.StatusConflict, gin.H{"error": "duplicate transfer already in-flight"})
		return
	}
	defer h.inFlight.Delete(dedupKey)

	h.txMu.Lock()
	defer h.txMu.Unlock()

	tx, err := h.instance.Transfer(h.auth, commitments, transferProof, usdrCommitments, usdrTransferProof, kIndex)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("transfer(): %v", err)})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), txTimeout)
	defer cancel()
	receipt, err := bind.WaitMined(ctx, h.client, tx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("wait mined: %v", err)})
		return
	}
	if receipt.Status != 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "transfer transaction reverted on-chain"})
		return
	}

	c.JSON(http.StatusOK, RelayResponse{
		TxHash:      receipt.TxHash.Hex(),
		BlockNumber: receipt.BlockNumber.Uint64(),
		GasUsed:     receipt.GasUsed,
	})
}

// ── TransferFee ───────────────────────────────────────────────────────────────

// RelayTransferFee handles POST /relay/transfer_fee.
//
// Calls Enygma.transferWithFee(commitmentDeltas, proof, participantIds).
// Used for confidential transfers where the user embeds a public fee in the
// ZK proof (enygma_fee circuit, 51-element public signal).
// The fee amount is at publicSignal[50] and is visible to the relayer without
// revealing any other private information.
func (h *Handler) RelayTransferFee(c *gin.Context) {
	var req RelayTransferFeeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	proof8, err := parseProof8(req.Proof)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("proof: %v", err)})
		return
	}
	if len(req.PublicSignal) != 54 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("publicSignal: fee circuit requires exactly 54 elements, got %d", len(req.PublicSignal))})
		return
	}

	var pubSig54 [54]*big.Int
	for i := range pubSig54 {
		pubSig54[i] = big.NewInt(0)
	}
	for i, s := range req.PublicSignal {
		n, ok := new(big.Int).SetString(s, 10)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("publicSignal[%d]: invalid decimal %q", i, s)})
			return
		}
		pubSig54[i] = n
	}

	commitments, err := parseCommitments(req.Commitments)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("commitments: %v", err)})
		return
	}
	kIndex := int64sToBI(req.KIndex)

	feeProof := enygma.IEnygmaFeeProof{
		Proof:        proof8,
		PublicSignal: pubSig54,
	}

	dedupKey := "transfer_fee:" + req.Proof[0]
	if _, loaded := h.inFlight.LoadOrStore(dedupKey, struct{}{}); loaded {
		c.JSON(http.StatusConflict, gin.H{"error": "duplicate fee transfer already in-flight"})
		return
	}
	defer h.inFlight.Delete(dedupKey)

	h.txMu.Lock()
	defer h.txMu.Unlock()

	tx, err := h.instance.TransferWithFee(h.auth, commitments, feeProof, kIndex)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("transferWithFee(): %v", err)})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), txTimeout)
	defer cancel()
	receipt, err := bind.WaitMined(ctx, h.client, tx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("wait mined: %v", err)})
		return
	}
	if receipt.Status != 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "transferWithFee transaction reverted on-chain"})
		return
	}

	c.JSON(http.StatusOK, RelayResponse{
		TxHash:      receipt.TxHash.Hex(),
		BlockNumber: receipt.BlockNumber.Uint64(),
		GasUsed:     receipt.GasUsed,
	})
}

// ── Conversion helpers ────────────────────────────────────────────────────────

// padPublicSignal80 zero-pads a variable-length decimal-string public
// signal (up to 80 elements) into the fixed [80]*big.Int the contract
// expects; the circuit only uses the first N slots.
func padPublicSignal80(signal []string) ([80]*big.Int, error) {
	var out [80]*big.Int
	if len(signal) > 80 {
		return out, fmt.Errorf("%d elements exceeds maximum of 80", len(signal))
	}
	for i := range out {
		out[i] = big.NewInt(0)
	}
	for i, s := range signal {
		n, ok := new(big.Int).SetString(s, 10)
		if !ok {
			return out, fmt.Errorf("[%d]: invalid decimal %q", i, s)
		}
		out[i] = n
	}
	return out, nil
}

// padPublicSignal81 is padPublicSignal80's counterpart for USDr proofs,
// which carry one extra public signal (FeeAmount, appended last — see
// USDrCircuit.Define / IEnygma.UsdrProof) beyond the main proof's 80.
func padPublicSignal81(signal []string) ([81]*big.Int, error) {
	var out [81]*big.Int
	if len(signal) > 81 {
		return out, fmt.Errorf("%d elements exceeds maximum of 81", len(signal))
	}
	for i := range out {
		out[i] = big.NewInt(0)
	}
	for i, s := range signal {
		n, ok := new(big.Int).SetString(s, 10)
		if !ok {
			return out, fmt.Errorf("[%d]: invalid decimal %q", i, s)
		}
		out[i] = n
	}
	return out, nil
}

// parseProof8 converts an 8-element decimal string array into [8]*big.Int.
// Element order from gnark: [Ax, Ay, B00, B01, B10, B11, Cx, Cy].
func parseProof8(raw [8]string) ([8]*big.Int, error) {
	var out [8]*big.Int
	for i, s := range raw {
		n, ok := new(big.Int).SetString(s, 10)
		if !ok {
			return out, fmt.Errorf("[%d]: invalid decimal %q", i, s)
		}
		out[i] = n
	}
	return out, nil
}

// parseCommitments converts [][]string pairs [C1x, C2y] into []IEnygmaPoint.
func parseCommitments(raw [][]string) ([]enygma.IEnygmaPoint, error) {
	out := make([]enygma.IEnygmaPoint, len(raw))
	for i, pair := range raw {
		if len(pair) != 2 {
			return nil, fmt.Errorf("[%d]: expected [C1, C2], got %d elements", i, len(pair))
		}
		c1, ok := new(big.Int).SetString(pair[0], 10)
		if !ok {
			return nil, fmt.Errorf("[%d].C1: invalid decimal %q", i, pair[0])
		}
		c2, ok := new(big.Int).SetString(pair[1], 10)
		if !ok {
			return nil, fmt.Errorf("[%d].C2: invalid decimal %q", i, pair[1])
		}
		out[i] = enygma.IEnygmaPoint{C1: c1, C2: c2}
	}
	return out, nil
}

// int64sToBI converts []int64 participant IDs into []*big.Int.
func int64sToBI(ids []int64) []*big.Int {
	out := make([]*big.Int, len(ids))
	for i, id := range ids {
		out[i] = big.NewInt(id)
	}
	return out
}

// ── I/O helpers ───────────────────────────────────────────────────────────────

// readAddressJSON reads the contract address from a Hardhat deploy address.json file.
func readAddressJSON(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	var f struct {
		Address string `json:"address"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return "", fmt.Errorf("parse %s: %w", path, err)
	}
	if f.Address == "" {
		return "", fmt.Errorf("%s: address field is empty", path)
	}
	return f.Address, nil
}
