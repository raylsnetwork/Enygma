package server

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"enygma_dvp/relayer/config"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-gonic/gin"
	"github.com/iden3/go-iden3-crypto/poseidon"
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
	inFlight sync.Map   // key: "vault:treeNum:nullifier" — prevents concurrent double-spend

	feeSpendPubKey *big.Int // nil unless RELAYER_FEE_SPEND_PRIVATE_KEY is configured
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

	// RELAYER_FEE_SPEND_PRIVATE_KEY is optional — derive the relayer's fee-note
	// spend pubkey once at startup if it's set. Same scheme as a user's
	// SpendKeyPair: PublicKey = Poseidon(PrivateKey).
	var feeSpendPubKey *big.Int
	if cfg.RelayerFeeSpendPrivateKey != nil {
		feeSpendPubKey, err = poseidon.Hash([]*big.Int{cfg.RelayerFeeSpendPrivateKey})
		if err != nil {
			return nil, fmt.Errorf("derive relayer fee spend pubkey: %w", err)
		}
	}

	return &Handler{
		cfg:            cfg,
		dvpABI:         dvpABI,
		vaultABI:       vaultABI,
		dvpAddr:        common.HexToAddress(dvpAddrStr),
		auth:           auth,
		client:         client,
		feeSpendPubKey: feeSpendPubKey,
	}, nil
}

// Info handles GET /relay/info — public, no auth required.
func (h *Handler) Info(c *gin.Context) {
	feeSpendPubKey := ""
	if h.feeSpendPubKey != nil {
		feeSpendPubKey = h.feeSpendPubKey.String()
	}
	c.JSON(http.StatusOK, InfoResponse{
		RelayerAddr:    h.auth.From.Hex(),
		FeeSpendPubKey: feeSpendPubKey,
	})
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
	nfKeys, err := h.claimNullifiers(nullifierClaim{vaultAddr, parsed})
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	defer h.releaseNullifiers(nfKeys)

	receipt := buildProofReceipt(parsed)
	txReceipt, err := h.transact("payment", receipt, vaultId, ctBytes, encBytes)
	if err != nil {
		relayError(c, "payment", err)
		return
	}
	c.JSON(http.StatusOK, RelayResponse{
		TxHash:      txReceipt.TxHash.Hex(),
		BlockNumber: txReceipt.BlockNumber.Uint64(),
		GasUsed:     txReceipt.GasUsed,
	})
}

// RelayPaymentRelayerFee handles POST /relay/payment_relayer_fee.
//
// Same-token relayer fee (PaymentRelayerFeePublic circuit): Alice pays Bob
// (output 0), keeps change (output 1), and leaves a spendable fee note for
// the relayer (output 2) whose amount is the public StFee signal
// (statement[8]), enforced on-chain by EnygmaDvp.paymentWithRelayerFee()
// against relayerFixedFeeAmount.
//
// Validation steps:
//  1. Confirm relayer-fee relaying is configured (RELAYER_FEE_SPEND_PRIVATE_KEY).
//  2. Parse and validate the receipt exactly like RelayPayment (root/nullifier).
//  3. Enforce the configured minimum fee (RELAYER_MIN_FEE) against statement[8].
//  4. Confirm the fee note is actually addressed to this relayer: recompute
//     Poseidon(feeSpendPubKey, feeSalt, StFee, tokenId) and compare against
//     statement[6] (the fee note's commitment, output 2).
//  5. Sign and submit to EnygmaDvp.paymentWithRelayerFee().
func (h *Handler) RelayPaymentRelayerFee(c *gin.Context) {
	if h.feeSpendPubKey == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "relayer fee not configured — set RELAYER_FEE_SPEND_PRIVATE_KEY",
		})
		return
	}

	var req RelayPaymentRelayerFeeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	parsed, err := parseReceipt(&req.Receipt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("parse receipt: %s", err)})
		return
	}
	if parsed.nIn != 1 || parsed.nOut != 3 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "expected a 1-input/3-output PaymentRelayerFeePublic receipt"})
		return
	}
	// parseReceipt's generic minExpected (1+3*nIn+nOut = 7 for nIn=1/nOut=3)
	// doesn't account for this circuit's extra StContractAddress/StFee
	// signals (9 elements total: [msg, treeNum0, root0, nf0, cmtBob,
	// cmtChange, cmtRelayer, contractAddr, fee]) — check explicitly before
	// indexing signal[6]/[8] below, or a short-but->=7 signal panics instead
	// of returning a clean 400.
	if len(parsed.signal) < 9 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("publicSignal too short for PaymentRelayerFeePublic: got %d, need 9", len(parsed.signal))})
		return
	}
	vaultId, ok := new(big.Int).SetString(req.VaultId, 10)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid vaultId: %q", req.VaultId)})
		return
	}
	feeSalt, ok := new(big.Int).SetString(req.FeeSalt, 10)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid feeSalt: %q", req.FeeSalt)})
		return
	}
	tokenId, ok := new(big.Int).SetString(req.TokenId, 10)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid tokenId: %q", req.TokenId)})
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

	// Statement: [msg, treeNum0, root0, nf0, cmtBob, cmtChange, cmtRelayer, contractAddr, fee]
	fee := parsed.signal[8]
	cmtRelayer := parsed.signal[6]

	if h.cfg.MinFee.Sign() > 0 && fee.Cmp(h.cfg.MinFee) < 0 {
		c.JSON(http.StatusPaymentRequired, gin.H{
			"error": fmt.Sprintf("fee %s is below the relayer's minimum %s", fee, h.cfg.MinFee),
		})
		return
	}
	expectedFeeCmt, err := poseidon.Hash([]*big.Int{h.feeSpendPubKey, feeSalt, fee, tokenId})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("compute expected fee commitment: %s", err)})
		return
	}
	if expectedFeeCmt.Cmp(cmtRelayer) != 0 {
		c.JSON(http.StatusPaymentRequired, gin.H{
			"error": "fee note is not addressed to this relayer's published spend key " +
				"(GET /relay/info) — statement[6] does not match " +
				"Poseidon(relayerFeeSpendPubKey, feeSalt, StFee, tokenId)",
		})
		return
	}

	nfKeys, err := h.claimNullifiers(nullifierClaim{vaultAddr, parsed})
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	defer h.releaseNullifiers(nfKeys)

	receipt := buildProofReceipt(parsed)
	txReceipt, err := h.transact("paymentWithRelayerFee", receipt, vaultId, ctBytes, encBytes)
	if err != nil {
		relayError(c, "paymentWithRelayerFee", err)
		return
	}
	c.JSON(http.StatusOK, RelayResponse{
		TxHash:      txReceipt.TxHash.Hex(),
		BlockNumber: txReceipt.BlockNumber.Uint64(),
		GasUsed:     txReceipt.GasUsed,
	})
}

// RelayPaymentUsdrFee handles POST /relay/payment_usdr_fee.
//
// Settles two independent proofs atomically in one call
// (EnygmaDvp.paymentWithUsdrFee): a normal payment against VaultId, and a
// UsdrFeeCircuit proof — a second, independent relayer-fee asset with its
// own token/vault/circuit — against UsdrVaultId.
//
// Validation steps mirror RelayPaymentRelayerFee, applied to the USDr leg,
// plus validating and settling the main leg in the same call:
//  1. Confirm relayer-fee relaying is configured.
//  2. Parse and validate BOTH receipts (root/nullifier, each against its own vault).
//  3. Enforce RELAYER_MIN_FEE against the USDr leg's public StFee (statement[7]).
//  4. Confirm the USDr fee note is addressed to this relayer: recompute
//     Poseidon(feeSpendPubKey, feeSalt, StFee, StTokenId) — StFee and
//     StTokenId read straight from the USDr leg's own public signal, both
//     public there — and compare against statement[4] (output 0).
//  5. Sign and submit both receipts to EnygmaDvp.paymentWithUsdrFee().
func (h *Handler) RelayPaymentUsdrFee(c *gin.Context) {
	if h.feeSpendPubKey == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "relayer fee not configured — set RELAYER_FEE_SPEND_PRIVATE_KEY",
		})
		return
	}

	var req RelayPaymentUsdrFeeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	parsed, err := parseReceipt(&req.Receipt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("parse receipt: %s", err)})
		return
	}
	// Every plain-payment circuit in this family (Payment, Payment2in) has
	// exactly 2 outputs (recipient + change); nIn (1 or 2) is left
	// unconstrained here since either is a valid main leg. Unlike
	// RelayPaymentRelayerFee's/UsdrFee's shape check below, this isn't
	// guarding an out-of-range signal index (the plain-payment wire shape
	// has no extra elements beyond parseReceipt's generic length check) —
	// it's defense-in-depth against a receipt built for a differently-shaped
	// circuit being submitted as this route's main leg.
	if parsed.nOut != 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "expected a 2-output payment receipt for the main leg"})
		return
	}
	usdrParsed, err := parseReceipt(&req.UsdrReceipt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("parse usdrReceipt: %s", err)})
		return
	}
	if usdrParsed.nIn != 1 || usdrParsed.nOut != 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "expected a 1-input/2-output UsdrFee receipt"})
		return
	}
	// parseReceipt's generic minExpected (1+3*nIn+nOut = 6 for nIn=1/nOut=2)
	// doesn't account for UsdrFee's extra StContractAddress/StFee/StTokenId
	// signals (9 elements total: [msg, treeNum0, root0, nf0, cmtFee,
	// cmtChange, contractAddr, fee, tokenId]) — check explicitly before
	// indexing signal[4]/[7]/[8] below, or a short-but->=6 signal panics
	// instead of returning a clean 400.
	if len(usdrParsed.signal) < 9 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("publicSignal too short for UsdrFee: got %d, need 9", len(usdrParsed.signal))})
		return
	}
	vaultId, ok := new(big.Int).SetString(req.VaultId, 10)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid vaultId: %q", req.VaultId)})
		return
	}
	usdrVaultId, ok := new(big.Int).SetString(req.UsdrVaultId, 10)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid usdrVaultId: %q", req.UsdrVaultId)})
		return
	}
	usdrFeeSalt, ok := new(big.Int).SetString(req.UsdrFeeSalt, 10)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid usdrFeeSalt: %q", req.UsdrFeeSalt)})
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
	usdrCtBytes, err := decodeHex(req.UsdrCipherText, "usdrCipherText")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	usdrEncBytes, err := decodeHex(req.UsdrEncTxData, "usdrEncTxData")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	vaultAddr, err := h.resolveVault(vaultId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("resolve vault: %s", err)})
		return
	}
	usdrVaultAddr, err := h.resolveVault(usdrVaultId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("resolve usdrVault: %s", err)})
		return
	}
	if err := h.validateReceipt(vaultAddr, parsed); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("main receipt: %s", err)})
		return
	}
	if err := h.validateReceipt(usdrVaultAddr, usdrParsed); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("usdr receipt: %s", err)})
		return
	}

	// USDr statement: [msg, treeNum0, root0, nf0, cmtFee, cmtChange, contractAddr, fee, tokenId]
	fee := usdrParsed.signal[7]
	tokenId := usdrParsed.signal[8]
	cmtFee := usdrParsed.signal[4]

	if h.cfg.MinFee.Sign() > 0 && fee.Cmp(h.cfg.MinFee) < 0 {
		c.JSON(http.StatusPaymentRequired, gin.H{
			"error": fmt.Sprintf("fee %s is below the relayer's minimum %s", fee, h.cfg.MinFee),
		})
		return
	}
	expectedFeeCmt, err := poseidon.Hash([]*big.Int{h.feeSpendPubKey, usdrFeeSalt, fee, tokenId})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("compute expected fee commitment: %s", err)})
		return
	}
	if expectedFeeCmt.Cmp(cmtFee) != 0 {
		c.JSON(http.StatusPaymentRequired, gin.H{
			"error": "USDr fee note is not addressed to this relayer's published spend key " +
				"(GET /relay/info) — usdrReceipt.publicSignal[4] does not match " +
				"Poseidon(relayerFeeSpendPubKey, usdrFeeSalt, StFee, StTokenId)",
		})
		return
	}

	nfKeys, err := h.claimNullifiers(nullifierClaim{vaultAddr, parsed}, nullifierClaim{usdrVaultAddr, usdrParsed})
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	defer h.releaseNullifiers(nfKeys)

	receipt := buildProofReceipt(parsed)
	usdrReceipt := buildProofReceipt(usdrParsed)
	txReceipt, err := h.transact("paymentWithUsdrFee",
		receipt, vaultId, ctBytes, encBytes,
		usdrReceipt, usdrVaultId, usdrCtBytes, usdrEncBytes)
	if err != nil {
		relayError(c, "paymentWithUsdrFee", err)
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
	nfKeys, err := h.claimNullifiers(nullifierClaim{payVaultAddr, payParsed}, nullifierClaim{delVaultAddr, delParsed})
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	defer h.releaseNullifiers(nfKeys)

	payReceipt := buildProofReceipt(payParsed)
	delReceipt := buildProofReceipt(delParsed)
	txReceipt, err := h.transact("swap", payReceipt, delReceipt, payVaultId, delVaultId)
	if err != nil {
		relayError(c, "swap", err)
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
	nfKeys, err := h.claimNullifiers(nullifierClaim{vaultAddr1, parsed1}, nullifierClaim{vaultAddr2, parsed2})
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	defer h.releaseNullifiers(nfKeys)

	r1 := buildProofReceipt(parsed1)
	r2 := buildProofReceipt(parsed2)
	txReceipt, err := h.transact("exchange", r1, r2, vaultId1, vaultId2)
	if err != nil {
		relayError(c, "exchange", err)
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

// nullifierClaim pairs a parsed receipt with the vault it's being submitted
// against, so claimNullifiers can scope its dedup key per-vault.
type nullifierClaim struct {
	vault   common.Address
	receipt *parsedReceipt
}

// claimNullifiers atomically marks all nullifiers across the given
// (vault, receipt) pairs as in-flight. Returns the claimed keys so the
// caller can release them with defer. Returns an error (HTTP 409) if any
// nullifier is already in-flight.
//
// The dedup key includes the vault address because GetNullifier(WithTree)
// deliberately has no vault component (nullifiers are scoped per-vault
// on-chain, in each vault's own nullifiers mapping) — the SAME spend key's
// first note in two DIFFERENT vaults can land at the same (treeNumber,
// pathIndex) and so produce the identical nullifier value in both. Without
// the vault in the key, two legitimate, unrelated concurrent relay calls
// against different vaults could spuriously reject each other with 409.
func (h *Handler) claimNullifiers(claims ...nullifierClaim) ([]string, error) {
	var claimed []string
	for _, c := range claims {
		r := c.receipt
		for i := 0; i < r.nIn; i++ {
			nullifier := r.signal[1+2*r.nIn+i]
			if nullifier.Sign() == 0 {
				continue
			}
			treeNum := r.signal[1+i]
			key := c.vault.Hex() + ":" + treeNum.String() + ":" + nullifier.String()
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

// simulateTimeout bounds the pre-flight simulation of a submission.
const simulateTimeout = 60 * time.Second

// errWouldRevert reports that the node's simulation of a submission reverted:
// the transaction was NOT sent. It is the caller's request that is at fault
// (bad proof, spent nullifier, wrong fee, ...), not the relayer.
type errWouldRevert struct{ cause error }

func (e *errWouldRevert) Error() string { return "would revert: " + e.cause.Error() }
func (e *errWouldRevert) Unwrap() error { return e.cause }

// isRevertError reports whether an eth_estimateGas / eth_call error came from
// the EVM rejecting the call, as opposed to a transport or node failure.
func isRevertError(err error) bool {
	m := strings.ToLower(err.Error())
	return strings.Contains(m, "revert") || strings.Contains(m, "vm exception") ||
		strings.Contains(m, "invalid opcode") || strings.Contains(m, "out of gas")
}

// relayError writes the response for a failed submission. A submission that the
// pre-flight simulation showed would revert is a client error (422); anything
// else is a relayer or node failure (500).
func relayError(c *gin.Context, method string, err error) {
	var wr *errWouldRevert
	if errors.As(err, &wr) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("%s(): %s", method, err)})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("%s(): %s", method, err)})
}

// transact simulates the call against the current chain state and sends it only
// if the simulation succeeds.
//
// The relayer used to send every validated request with a fixed 8,000,000 gas
// limit and no simulation. A request with a bad proof was sent anyway and
// reverted on-chain: an invalid proof makes the pairing precompile fail, which
// consumes all the gas passed to it, and each such request cost the relayer
// about 7.5M gas. Anyone holding the API key could drain the relayer's funds with
// junk proofs. eth_call runs the call once, at the same gas cap the transaction
// will use, and fails on a revert, so nothing is sent for a request that cannot
// succeed. The limit stays at the cap: a transaction that succeeds only pays for
// the gas it uses, so a high limit costs nothing extra, and one call is much
// cheaper than eth_estimateGas's repeated executions of two Groth16 verifications.
func (h *Handler) transact(method string, args ...interface{}) (*types.Receipt, error) {
	h.txMu.Lock()
	defer h.txMu.Unlock()

	data, err := h.dvpABI.Pack(method, args...)
	if err != nil {
		return nil, fmt.Errorf("pack %s: %w", method, err)
	}

	simCtx, cancel := context.WithTimeout(context.Background(), simulateTimeout)
	defer cancel()
	if _, err := h.client.CallContract(simCtx, ethereum.CallMsg{
		From: h.auth.From,
		To:   &h.dvpAddr,
		Gas:  h.auth.GasLimit,
		Data: data,
	}, nil); err != nil {
		if isRevertError(err) {
			return nil, &errWouldRevert{cause: err}
		}
		return nil, fmt.Errorf("simulate %s: %w", method, err)
	}

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
