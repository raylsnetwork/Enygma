package server

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"

	"enygma_dvp_relayer/config"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-gonic/gin"
)

// ── Solidity struct mirrors (must match IEnygmaDvp ABI exactly) ──────────────

type g1Point struct {
	X *big.Int `abi:"x"`
	Y *big.Int `abi:"y"`
}
type g2Point struct {
	X [2]*big.Int `abi:"x"`
	Y [2]*big.Int `abi:"y"`
}
type snarkProof struct {
	A g1Point `abi:"a"`
	B g2Point `abi:"b"`
	C g1Point `abi:"c"`
}
type proofReceipt struct {
	Proof           snarkProof `abi:"proof"`
	Statement       []*big.Int `abi:"statement"`
	NumberOfInputs  *big.Int   `abi:"numberOfInputs"`
	NumberOfOutputs *big.Int   `abi:"numberOfOutputs"`
}

// ── handler ──────────────────────────────────────────────────────────────────

// Handler holds all dependencies for the relay endpoints.
type Handler struct {
	cfg      *config.Config
	dvpABI   abi.ABI
	vaultABI abi.ABI
	dvpAddr  common.Address
	auth     *bind.TransactOpts
	client   *ethclient.Client
	txMu     sync.Mutex // serializes on-chain submissions — prevents nonce races
	inFlight sync.Map   // key: "treeNum:nullifier" — prevents concurrent double-spend
}

// NewHandler wires up the handler from config.
func NewHandler(cfg *config.Config) (*Handler, error) {
	client, err := ethclient.Dial(cfg.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("ethclient.Dial(%s): %w", cfg.RPCURL, err)
	}

	privKey, err := crypto.HexToECDSA(cfg.RelayerPrivateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("parse relayer private key: %w", err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(privKey, cfg.ChainID)
	if err != nil {
		return nil, fmt.Errorf("build transactor: %w", err)
	}
	auth.GasLimit = 8_000_000

	dvpAddrStr := cfg.EnygmaDvpAddr
	if dvpAddrStr == "" {
		receipts, err := loadReceipts(cfg.ReceiptsPath)
		if err != nil {
			return nil, fmt.Errorf("load receipts: %w", err)
		}
		dvpAddrStr = receipts["EnygmaDvp"].ContractAddress
	}
	if dvpAddrStr == "" {
		return nil, fmt.Errorf("EnygmaDvp address must be set via RELAYER_DVP_ADDR or receipts.json")
	}

	dvpABI, err := loadABIFromArtifact("../artifacts/contracts/core/contracts/EnygmaDvp.sol/EnygmaDvp.json")
	if err != nil {
		return nil, fmt.Errorf("load EnygmaDvp ABI: %w", err)
	}
	vaultABI, err := loadABIFromArtifact("../artifacts/contracts/core/contracts/vaults/AbstractCoinVault.sol/AbstractCoinVault.json")
	if err != nil {
		return nil, fmt.Errorf("load AbstractCoinVault ABI: %w", err)
	}

	return &Handler{
		cfg:      cfg,
		dvpABI:   dvpABI,
		vaultABI: vaultABI,
		dvpAddr:  common.HexToAddress(dvpAddrStr),
		auth:     auth,
		client:   client,
	}, nil
}

// ── endpoint handlers ─────────────────────────────────────────────────────────

// RelayPayment handles POST /relay/payment.
// Validates the proof's Merkle root and nullifier, then calls dvp.payment().
func (h *Handler) RelayPayment(c *gin.Context) {
	var req RelayPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	parsed, err := parseReceipt(&req.Receipt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("parse receipt: %s", err)})
		return
	}
	vaultId, ok := new(big.Int).SetString(req.VaultId, 10)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid vaultId: %q", req.VaultId)})
		return
	}
	ctBytes, err := decodeHex(req.CipherText, "cipherText")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	encBytes, err := decodeHex(req.EncTxData, "encTxData")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	vaultAddr, err := h.resolveVault(vaultId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := h.validateReceipt(vaultAddr, parsed); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	nfKeys, err := h.claimNullifiers(parsed)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	defer h.releaseNullifiers(nfKeys)

	receipt := buildProofReceipt(parsed)
	txReceipt, err := h.transact("payment", receipt, vaultId, ctBytes, encBytes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("payment(): %s", err)})
		return
	}
	c.JSON(http.StatusOK, RelayResponse{
		TxHash:      txReceipt.TxHash.Hex(),
		BlockNumber: txReceipt.BlockNumber.Uint64(),
		GasUsed:     txReceipt.GasUsed,
	})
}

// RelaySwap handles POST /relay/swap.
// Validates both receipts, then calls dvp.swap().
func (h *Handler) RelaySwap(c *gin.Context) {
	var req RelaySwapRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	payParsed, err := parseReceipt(&req.PaymentReceipt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("parse paymentReceipt: %s", err)})
		return
	}
	delParsed, err := parseReceipt(&req.DeliveryReceipt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("parse deliveryReceipt: %s", err)})
		return
	}
	payVaultId, ok := new(big.Int).SetString(req.PaymentVaultId, 10)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid paymentVaultId: %q", req.PaymentVaultId)})
		return
	}
	delVaultId, ok := new(big.Int).SetString(req.DeliveryVaultId, 10)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid deliveryVaultId: %q", req.DeliveryVaultId)})
		return
	}

	payVaultAddr, err := h.resolveVault(payVaultId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("resolve paymentVault: %s", err)})
		return
	}
	delVaultAddr, err := h.resolveVault(delVaultId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("resolve deliveryVault: %s", err)})
		return
	}
	if err := h.validateReceipt(payVaultAddr, payParsed); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("payment side: %s", err)})
		return
	}
	if err := h.validateReceipt(delVaultAddr, delParsed); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("delivery side: %s", err)})
		return
	}
	nfKeys, err := h.claimNullifiers(payParsed, delParsed)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	defer h.releaseNullifiers(nfKeys)

	payReceipt := buildProofReceipt(payParsed)
	delReceipt := buildProofReceipt(delParsed)
	txReceipt, err := h.transact("swap", payReceipt, delReceipt, payVaultId, delVaultId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("swap(): %s", err)})
		return
	}
	c.JSON(http.StatusOK, RelayResponse{
		TxHash:      txReceipt.TxHash.Hex(),
		BlockNumber: txReceipt.BlockNumber.Uint64(),
		GasUsed:     txReceipt.GasUsed,
	})
}

// RelayExchange handles POST /relay/exchange.
// Validates both receipts, then calls dvp.exchange().
func (h *Handler) RelayExchange(c *gin.Context) {
	var req RelayExchangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	parsed1, err := parseReceipt(&req.Receipt1)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("parse receipt1: %s", err)})
		return
	}
	parsed2, err := parseReceipt(&req.Receipt2)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("parse receipt2: %s", err)})
		return
	}
	vaultId1, ok := new(big.Int).SetString(req.VaultId1, 10)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid vaultId1: %q", req.VaultId1)})
		return
	}
	vaultId2, ok := new(big.Int).SetString(req.VaultId2, 10)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid vaultId2: %q", req.VaultId2)})
		return
	}

	vaultAddr1, err := h.resolveVault(vaultId1)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("resolve vault1: %s", err)})
		return
	}
	vaultAddr2, err := h.resolveVault(vaultId2)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("resolve vault2: %s", err)})
		return
	}
	if err := h.validateReceipt(vaultAddr1, parsed1); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("receipt1: %s", err)})
		return
	}
	if err := h.validateReceipt(vaultAddr2, parsed2); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("receipt2: %s", err)})
		return
	}
	nfKeys, err := h.claimNullifiers(parsed1, parsed2)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	defer h.releaseNullifiers(nfKeys)

	r1 := buildProofReceipt(parsed1)
	r2 := buildProofReceipt(parsed2)
	txReceipt, err := h.transact("exchange", r1, r2, vaultId1, vaultId2)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("exchange(): %s", err)})
		return
	}
	c.JSON(http.StatusOK, RelayResponse{
		TxHash:      txReceipt.TxHash.Hex(),
		BlockNumber: txReceipt.BlockNumber.Uint64(),
		GasUsed:     txReceipt.GasUsed,
	})
}

// ── validation ────────────────────────────────────────────────────────────────

// resolveVault calls dvp.vaultById(vaultId) to get the vault contract address.
func (h *Handler) resolveVault(vaultId *big.Int) (common.Address, error) {
	dvp := bind.NewBoundContract(h.dvpAddr, h.dvpABI, h.client, h.client, h.client)
	var result []interface{}
	if err := dvp.Call(&bind.CallOpts{}, &result, "vaultById", vaultId); err != nil {
		return common.Address{}, fmt.Errorf("vaultById(%s): %w", vaultId, err)
	}
	addr, ok := result[0].(common.Address)
	if !ok {
		return common.Address{}, fmt.Errorf("vaultById returned unexpected type")
	}
	if addr == (common.Address{}) {
		return common.Address{}, fmt.Errorf("vaultId %s is not registered", vaultId)
	}
	return addr, nil
}

// validateReceipt checks each input's Merkle root and nullifier against the vault.
//
// Non-interleaved statement layout:
//
//	[msg, treeNum[0..nIn-1], root[0..nIn-1], nullifier[0..nIn-1], cmt[0..nOut-1]]
func (h *Handler) validateReceipt(vaultAddr common.Address, p *parsedReceipt) error {
	vault := bind.NewBoundContract(vaultAddr, h.vaultABI, h.client, h.client, h.client)
	nIn := p.nIn
	sig := p.signal

	if len(sig) < 1+3*nIn {
		return fmt.Errorf("publicSignal too short for %d inputs: got %d elements", nIn, len(sig))
	}

	for i := 0; i < nIn; i++ {
		treeNum := sig[1+i]
		root := sig[1+nIn+i]
		nullifier := sig[1+2*nIn+i]

		var rootResult []interface{}
		if err := vault.Call(&bind.CallOpts{}, &rootResult, "rootHistory", treeNum, root); err != nil {
			return fmt.Errorf("rootHistory check (input %d): %w", i, err)
		}
		if known, ok := rootResult[0].(bool); !ok || !known {
			return fmt.Errorf("input %d: Merkle root is not a known vault root", i)
		}

		var nfResult []interface{}
		if err := vault.Call(&bind.CallOpts{}, &nfResult, "nullifiers", treeNum, nullifier); err != nil {
			return fmt.Errorf("nullifiers check (input %d): %w", i, err)
		}
		if spent, ok := nfResult[0].(bool); !ok {
			return fmt.Errorf("input %d: unexpected type from nullifiers()", i)
		} else if spent {
			return fmt.Errorf("input %d: nullifier already spent", i)
		}
	}
	return nil
}

// ── parsing ───────────────────────────────────────────────────────────────────

type parsedReceipt struct {
	proof  [8]*big.Int
	signal []*big.Int
	nIn    int
	nOut   int
}

func parseReceipt(r *ReceiptPayload) (*parsedReceipt, error) {
	var proof [8]*big.Int
	for i, s := range r.Proof {
		n, ok := new(big.Int).SetString(s, 10)
		if !ok {
			return nil, fmt.Errorf("invalid proof[%d]: %q", i, s)
		}
		proof[i] = n
	}

	sig := make([]*big.Int, len(r.PublicSignal))
	for i, s := range r.PublicSignal {
		n, ok := new(big.Int).SetString(s, 10)
		if !ok {
			return nil, fmt.Errorf("invalid publicSignal[%d]: %q", i, s)
		}
		sig[i] = n
	}

	if r.NumberOfInputs <= 0 {
		return nil, fmt.Errorf("numberOfInputs must be > 0")
	}
	if r.NumberOfOutputs <= 0 {
		return nil, fmt.Errorf("numberOfOutputs must be > 0")
	}
	// Cap inputs/outputs to prevent RPC amplification: validateReceipt makes
	// 2 RPC calls per input, so an unbounded nIn allows DoS via huge signals.
	if r.NumberOfInputs > 10 || r.NumberOfOutputs > 10 {
		return nil, fmt.Errorf("numberOfInputs/Outputs exceeds maximum of 10")
	}
	// DVP initiator proofs carry extra elements (commitA, revertCommitA) beyond
	// the standard 1+3*nIn+nOut layout — use >= so we don't reject them.
	minExpected := 1 + 3*r.NumberOfInputs + r.NumberOfOutputs
	if len(sig) < minExpected {
		return nil, fmt.Errorf("publicSignal too short: got %d, need at least %d (1+3*%d+%d)",
			len(sig), minExpected, r.NumberOfInputs, r.NumberOfOutputs)
	}

	return &parsedReceipt{
		proof:  proof,
		signal: sig,
		nIn:    r.NumberOfInputs,
		nOut:   r.NumberOfOutputs,
	}, nil
}

func buildProofReceipt(p *parsedReceipt) proofReceipt {
	sp := snarkProof{
		A: g1Point{X: p.proof[0], Y: p.proof[1]},
		B: g2Point{
			X: [2]*big.Int{p.proof[2], p.proof[3]},
			Y: [2]*big.Int{p.proof[4], p.proof[5]},
		},
		C: g1Point{X: p.proof[6], Y: p.proof[7]},
	}
	return proofReceipt{
		Proof:           sp,
		Statement:       p.signal,
		NumberOfInputs:  big.NewInt(int64(p.nIn)),
		NumberOfOutputs: big.NewInt(int64(p.nOut)),
	}
}

// ── nullifier in-flight tracking ──────────────────────────────────────────────

// claimNullifiers atomically marks all nullifiers across the given receipts as
// in-flight. Returns the claimed keys so the caller can release them with defer.
// Returns an error (HTTP 409) if any nullifier is already in-flight.
func (h *Handler) claimNullifiers(receipts ...*parsedReceipt) ([]string, error) {
	var claimed []string
	for _, r := range receipts {
		for i := 0; i < r.nIn; i++ {
			nullifier := r.signal[1+2*r.nIn+i]
			if nullifier.Sign() == 0 {
				continue
			}
			treeNum := r.signal[1+i]
			key := treeNum.String() + ":" + nullifier.String()
			if _, loaded := h.inFlight.LoadOrStore(key, struct{}{}); loaded {
				h.releaseNullifiers(claimed)
				return nil, fmt.Errorf("nullifier already in-flight: %s", key)
			}
			claimed = append(claimed, key)
		}
	}
	return claimed, nil
}

func (h *Handler) releaseNullifiers(keys []string) {
	for _, k := range keys {
		h.inFlight.Delete(k)
	}
}

// ── chain helpers ─────────────────────────────────────────────────────────────

func (h *Handler) transact(method string, args ...interface{}) (*types.Receipt, error) {
	h.txMu.Lock()
	defer h.txMu.Unlock()
	dvp := bind.NewBoundContract(h.dvpAddr, h.dvpABI, h.client, h.client, h.client)
	tx, err := dvp.Transact(h.auth, method, args...)
	if err != nil {
		return nil, err
	}
	receipt, err := bind.WaitMined(context.Background(), h.client, tx)
	if err != nil {
		return nil, fmt.Errorf("wait mined: %w", err)
	}
	if receipt.Status == types.ReceiptStatusFailed {
		return nil, fmt.Errorf("transaction reverted on-chain")
	}
	return receipt, nil
}

// ── file helpers ──────────────────────────────────────────────────────────────

type receiptEntry struct {
	ContractAddress string `json:"contractAddress"`
}

func loadReceipts(path string) (map[string]receiptEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r map[string]receiptEntry
	return r, json.Unmarshal(data, &r)
}

func loadABIFromArtifact(path string) (abi.ABI, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return abi.ABI{}, err
	}
	var artifact struct {
		ABI json.RawMessage `json:"abi"`
	}
	if err := json.Unmarshal(data, &artifact); err != nil {
		return abi.ABI{}, err
	}
	return abi.JSON(strings.NewReader(string(artifact.ABI)))
}

func decodeHex(s, fieldName string) ([]byte, error) {
	s = strings.TrimPrefix(s, "0x")
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("invalid hex in %s: %w", fieldName, err)
	}
	return b, nil
}
