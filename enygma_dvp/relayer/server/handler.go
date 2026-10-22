package server

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net"
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
	"github.com/ethereum/go-ethereum/rpc"
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
	// erc20VaultABI is Erc20CoinVault's: the partial-withdraw route calls the
	// vault directly rather than through EnygmaDvp.
	erc20VaultABI abi.ABI
	dvpAddr       common.Address
	auth          *bind.TransactOpts
	client        *ethclient.Client
	txMu          sync.Mutex // serializes on-chain submissions — prevents nonce races
	// txTimeout bounds how long transact waits for a receipt (0 = defaultTxTimeout).
	txTimeout time.Duration
	inFlight  sync.Map // key: "vault:treeNum:nullifier" — prevents concurrent double-spend

	feeSpendPubKey *big.Int // nil unless RELAYER_FEE_SPEND_PRIVATE_KEY is configured
	// feeNotes keeps the opening of every fee note this relayer is paid with
	// (see feeNoteStore); set whenever feeSpendPubKey is.
	feeNotes *feeNoteStore
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
	erc20VaultABI, err := loadABIFromArtifact("../artifacts/contracts/core/contracts/vaults/Erc20CoinVault.sol/Erc20CoinVault.json")
	if err != nil {
		return nil, fmt.Errorf("load Erc20CoinVault ABI: %w", err)
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
		erc20VaultABI:  erc20VaultABI,
		dvpAddr:        common.HexToAddress(dvpAddrStr),
		auth:           auth,
		client:         client,
		txTimeout:      cfg.TxTimeout,
		feeSpendPubKey: feeSpendPubKey,
		feeNotes:       newFeeNoteStore(cfg.FeeNotesPath),
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
// The receipt is a Payment (1-input) or Payment2in (2-input) statement.
// Validates the proof's Merkle root and nullifier, then calls dvp.payment().
func (h *Handler) RelayPayment(c *gin.Context) {
	var req RelayPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	parsed, err := parseReceipt(&req.Receipt, paymentLayout, payment2inLayout)
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

	receipt := buildProofReceipt(parsed)
	txReceipt, err := h.transact("payment", receipt, vaultId, ctBytes, encBytes)
	h.settleClaims(nfKeys, err)
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
// the relayer (output 2) whose amount is the public StFee signal, enforced
// on-chain by EnygmaDvp.paymentWithRelayerFee() against relayerFixedFeeAmount.
//
// Validation steps:
//  1. Confirm relayer-fee relaying is configured (RELAYER_FEE_SPEND_PRIVATE_KEY).
//  2. Parse the receipt as a PaymentRelayerFee statement and validate it
//     exactly like RelayPayment (root/nullifier).
//  3. Enforce the configured minimum fee (RELAYER_MIN_FEE) against StFee.
//  4. Confirm the fee note (output 2) is actually addressed to this relayer:
//     Poseidon(feeSpendPubKey, feeSalt, StFee, tokenId). checkFeeNote does
//     both.
//  5. Keep the fee note's opening (feeNoteStore): the salt reaches the relayer
//     only through this request, and without it the note cannot be spent.
//  6. Sign and submit to EnygmaDvp.paymentWithRelayerFee().
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

	parsed, err := parseReceipt(&req.Receipt, paymentRelayerFeeLayout)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("parse receipt: %s", err)})
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

	fee := parsed.get(paymentRelayerFeeLayout.fee)
	cmtRelayer := parsed.get(relayerFeeNote)
	if status, err := h.checkFeeNote(cmtRelayer, fee, feeSalt, tokenId); err != nil {
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	nfKeys, err := h.claimNullifiers(nullifierClaim{vaultAddr, parsed})
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	keepFeeNote := func() error {
		return h.feeNotes.recordSubmitted("paymentWithRelayerFee", vaultId, cmtRelayer, feeSalt, fee, tokenId)
	}
	receipt := buildProofReceipt(parsed)
	txReceipt, err := h.transactWith(h.dvpAddr, h.dvpABI, keepFeeNote, "paymentWithRelayerFee", receipt, vaultId, ctBytes, encBytes)
	h.settleClaims(nfKeys, err)
	if err != nil {
		relayError(c, "paymentWithRelayerFee", err)
		return
	}
	if err := h.feeNotes.recordMined("paymentWithRelayerFee", vaultId, cmtRelayer, txReceipt.TxHash.Hex(), txReceipt.BlockNumber.Uint64()); err != nil {
		log.Printf("[relay] fee note store: recording the mined fee note: %v (its opening is already kept)", err)
	}
	c.JSON(http.StatusOK, RelayResponse{
		TxHash:      txReceipt.TxHash.Hex(),
		BlockNumber: txReceipt.BlockNumber.Uint64(),
		GasUsed:     txReceipt.GasUsed,
	})
}

// withdrawMessage is the StMessage of a withdrawal proof: WITHDRAW_MESSAGE in
// IEnygmaDvp.sol, MessageWithdraw in the gnark Payment templates.
const withdrawMessage = 1

// RelayPartialWithdraw handles POST /relay/partial_withdraw.
//
// Withdraws part of a note for its owner, who pays this relayer from the note
// instead of paying gas: a PaymentRelayerFeePublic proof whose outputs are the
// withdrawal (to Recipient), the owner's change note and this relayer's fee
// note, settled with Erc20CoinVault.partialWithdrawWithRelayerFee.
//
//  1. Confirm relayer-fee relaying is configured.
//  2. Parse the receipt as a PaymentRelayerFee statement; require the
//     withdrawal StMessage.
//  3. Check output[0] opens to Poseidon4(uint160(Recipient), 0, Amount,
//     TokenId), so the request says what the proof pays out.
//  4. checkFeeNote: enforce RELAYER_MIN_FEE against the public StFee and
//     confirm the fee note (output 2) is addressed to this relayer.
//  5. Check the root and nullifier, claim the nullifier, keep the fee note's
//     opening, then simulate and submit.
func (h *Handler) RelayPartialWithdraw(c *gin.Context) {
	if h.feeSpendPubKey == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "relayer fee not configured — set RELAYER_FEE_SPEND_PRIVATE_KEY",
		})
		return
	}

	var req RelayPartialWithdrawRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	parsed, err := parseReceipt(&req.Receipt, paymentRelayerFeeLayout)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("parse receipt: %s", err)})
		return
	}
	// A withdrawal proof carries StMessage withdrawMessage (the vault's
	// WITHDRAW_MESSAGE); a payment proof (0) would be refused on-chain.
	if msg := parsed.get(paymentRelayerFeeLayout.message); msg.Cmp(big.NewInt(withdrawMessage)) != 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("StMessage is %s; a withdrawal proof must carry %d", msg, withdrawMessage)})
		return
	}
	vaultId, ok := new(big.Int).SetString(req.VaultId, 10)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid vaultId: %q", req.VaultId)})
		return
	}
	if !common.IsHexAddress(req.Recipient) {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid recipient address: %q", req.Recipient)})
		return
	}
	recipient := common.HexToAddress(req.Recipient)
	amount, ok := new(big.Int).SetString(req.Amount, 10)
	if !ok || amount.Sign() <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid amount: %q (must be a positive integer)", req.Amount)})
		return
	}
	tokenId, ok := new(big.Int).SetString(req.TokenId, 10)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid tokenId: %q", req.TokenId)})
		return
	}
	feeSalt, ok := new(big.Int).SetString(req.FeeSalt, 10)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid feeSalt: %q", req.FeeSalt)})
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

	withdrawCmt, err := poseidon.Hash([]*big.Int{new(big.Int).SetBytes(recipient.Bytes()), big.NewInt(0), amount, tokenId})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("compute withdrawal commitment: %s", err)})
		return
	}
	if withdrawCmt.Cmp(parsed.get(withdrawalNote)) != 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "output 0 does not match Poseidon(uint160(recipient), 0, amount, tokenId) — " +
				"the proof does not withdraw this amount to this recipient",
		})
		return
	}

	fee := parsed.get(paymentRelayerFeeLayout.fee)
	cmtRelayer := parsed.get(relayerFeeNote)
	if status, err := h.checkFeeNote(cmtRelayer, fee, feeSalt, tokenId); err != nil {
		c.JSON(status, gin.H{"error": err.Error()})
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

	const method = "partialWithdrawWithRelayerFee"
	keepFeeNote := func() error {
		return h.feeNotes.recordSubmitted(method, vaultId, cmtRelayer, feeSalt, fee, tokenId)
	}
	receipt := buildProofReceipt(parsed)
	txReceipt, err := h.transactWith(vaultAddr, h.erc20VaultABI, keepFeeNote, method,
		[]*big.Int{amount, tokenId}, recipient, receipt, ctBytes, encBytes)
	h.settleClaims(nfKeys, err)
	if err != nil {
		relayError(c, method, err)
		return
	}
	if err := h.feeNotes.recordMined(method, vaultId, cmtRelayer, txReceipt.TxHash.Hex(), txReceipt.BlockNumber.Uint64()); err != nil {
		log.Printf("[relay] fee note store: recording the mined fee note: %v (its opening is already kept)", err)
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
//  2. Parse the main receipt as a Payment or Payment2in statement and the USDr
//     receipt as a UsdrFee statement, and validate both (root/nullifier, each
//     against its own vault).
//  3. Enforce RELAYER_MIN_FEE against the USDr leg's public StFee.
//  4. Confirm the USDr fee note (output 0) is addressed to this relayer:
//     Poseidon(feeSpendPubKey, feeSalt, StFee, StTokenId), with StFee and
//     StTokenId read straight from the USDr leg's own statement, both public
//     there. checkFeeNote does both.
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

	parsed, err := parseReceipt(&req.Receipt, paymentLayout, payment2inLayout)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("parse receipt: %s", err)})
		return
	}
	usdrParsed, err := parseReceipt(&req.UsdrReceipt, usdrFeeLayout)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("parse usdrReceipt: %s", err)})
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

	fee := usdrParsed.get(usdrFeeLayout.fee)
	tokenId := usdrParsed.get(usdrFeeLayout.tokenId)
	cmtFee := usdrParsed.get(usdrFeeNote)
	if status, err := h.checkFeeNote(cmtFee, fee, usdrFeeSalt, tokenId); err != nil {
		c.JSON(status, gin.H{"error": "USDr leg: " + err.Error()})
		return
	}

	nfKeys, err := h.claimNullifiers(nullifierClaim{vaultAddr, parsed}, nullifierClaim{usdrVaultAddr, usdrParsed})
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	keepFeeNote := func() error {
		return h.feeNotes.recordSubmitted("paymentWithUsdrFee", usdrVaultId, cmtFee, usdrFeeSalt, fee, tokenId)
	}
	receipt := buildProofReceipt(parsed)
	usdrReceipt := buildProofReceipt(usdrParsed)
	txReceipt, err := h.transactWith(h.dvpAddr, h.dvpABI, keepFeeNote, "paymentWithUsdrFee",
		receipt, vaultId, ctBytes, encBytes,
		usdrReceipt, usdrVaultId, usdrCtBytes, usdrEncBytes)
	h.settleClaims(nfKeys, err)
	if err != nil {
		relayError(c, "paymentWithUsdrFee", err)
		return
	}
	if err := h.feeNotes.recordMined("paymentWithUsdrFee", usdrVaultId, cmtFee, txReceipt.TxHash.Hex(), txReceipt.BlockNumber.Uint64()); err != nil {
		log.Printf("[relay] fee note store: recording the mined fee note: %v (its opening is already kept)", err)
	}
	c.JSON(http.StatusOK, RelayResponse{
		TxHash:      txReceipt.TxHash.Hex(),
		BlockNumber: txReceipt.BlockNumber.Uint64(),
		GasUsed:     txReceipt.GasUsed,
	})
}

// RelaySwap handles POST /relay/swap.
// The payment receipt is a DvpInitiator statement and the delivery receipt a
// DvpDestination one. Validates both receipts, then calls dvp.swap().
func (h *Handler) RelaySwap(c *gin.Context) {
	var req RelaySwapRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	payParsed, err := parseReceipt(&req.PaymentReceipt, dvpInitiatorLayout)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("parse paymentReceipt: %s", err)})
		return
	}
	delParsed, err := parseReceipt(&req.DeliveryReceipt, dvpDestinationLayout)
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

	payReceipt := buildProofReceipt(payParsed)
	delReceipt := buildProofReceipt(delParsed)
	txReceipt, err := h.transact("swap", payReceipt, delReceipt, payVaultId, delVaultId)
	h.settleClaims(nfKeys, err)
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
// Both receipts are DvpInitiator statements. Validates both receipts, then
// calls dvp.exchange().
func (h *Handler) RelayExchange(c *gin.Context) {
	var req RelayExchangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	parsed1, err := parseReceipt(&req.Receipt1, dvpInitiatorLayout)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("parse receipt1: %s", err)})
		return
	}
	parsed2, err := parseReceipt(&req.Receipt2, dvpInitiatorLayout)
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

	r1 := buildProofReceipt(parsed1)
	r2 := buildProofReceipt(parsed2)
	txReceipt, err := h.transact("exchange", r1, r2, vaultId1, vaultId2)
	h.settleClaims(nfKeys, err)
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

// checkFeeNote is the claim check every fee route makes on the note it is paid
// with: the fee must meet RELAYER_MIN_FEE, and the note must be
// Poseidon(feeSpendPubKey, salt, fee, tokenId), i.e. addressed to this
// relayer's published spend key. A refusal comes with its HTTP status.
func (h *Handler) checkFeeNote(feeNote, fee, salt, tokenId *big.Int) (int, error) {
	if h.cfg.MinFee.Sign() > 0 && fee.Cmp(h.cfg.MinFee) < 0 {
		return http.StatusPaymentRequired, fmt.Errorf("fee %s is below the relayer's minimum %s", fee, h.cfg.MinFee)
	}
	expected, err := poseidon.Hash([]*big.Int{h.feeSpendPubKey, salt, fee, tokenId})
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("compute expected fee commitment: %s", err)
	}
	if expected.Cmp(feeNote) != 0 {
		return http.StatusPaymentRequired, errors.New("fee note is not addressed to this relayer's published spend key " +
			"(GET /relay/info): it does not match Poseidon(relayerFeeSpendPubKey, feeSalt, StFee, tokenId)")
	}
	return 0, nil
}

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
func (h *Handler) validateReceipt(vaultAddr common.Address, p *parsedReceipt) error {
	vault := bind.NewBoundContract(vaultAddr, h.vaultABI, h.client, h.client, h.client)
	treeNumbers := p.layout.treeNumbers.in(p.signal)
	roots := p.layout.merkleRoots.in(p.signal)

	for i, nullifier := range p.layout.nullifiers.in(p.signal) {
		treeNum, root := treeNumbers[i], roots[i]

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
	layout statementLayout
}

// parseReceipt parses r as a statement with whichever accepted layout its
// numberOfInputs/numberOfOutputs name. A publicSignal that is not that
// layout's statement length is refused before anything else is parsed.
func parseReceipt(r *ReceiptPayload, accepted ...statementLayout) (*parsedReceipt, error) {
	c, err := layoutFor(r.NumberOfInputs, r.NumberOfOutputs, len(r.PublicSignal), accepted...)
	if err != nil {
		return nil, err
	}

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

	return &parsedReceipt{proof: proof, signal: sig, layout: c}, nil
}

// get returns the receipt's single-signal field f.
func (p *parsedReceipt) get(f field) *big.Int { return f.one(p.signal) }

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
		NumberOfInputs:  big.NewInt(int64(p.layout.nIn)),
		NumberOfOutputs: big.NewInt(int64(p.layout.nOut)),
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
		treeNumbers := r.layout.treeNumbers.in(r.signal)
		for i, nullifier := range r.layout.nullifiers.in(r.signal) {
			if nullifier.Sign() == 0 {
				continue
			}
			treeNum := treeNumbers[i]
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

// pendingClaimHold bounds how long nullifier claims stay held for a
// transaction that was still unmined when its request timed out.
const pendingClaimHold = time.Hour

// settleClaims releases a request's nullifier claims once its submission is
// over. If the transaction was sent but is still pending (errTxPending), the
// claims stay held until it is mined (or pendingClaimHold passes), so the same
// notes cannot be submitted again while the first transaction can still land.
func (h *Handler) settleClaims(keys []string, err error) {
	var pending *errTxPending
	if !errors.As(err, &pending) {
		h.releaseNullifiers(keys)
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), pendingClaimHold)
		defer cancel()
		_, _ = bind.WaitMined(ctx, h.client, pending.tx)
		h.releaseNullifiers(keys)
	}()
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

// defaultTxTimeout is used when the handler was built without a TxTimeout.
const defaultTxTimeout = 2 * time.Minute

// errTxPending reports that the transaction was sent but not mined within the
// timeout. It may still be mined later; the caller gets its hash to follow it.
type errTxPending struct{ tx *types.Transaction }

func (e *errTxPending) Error() string {
	return "transaction " + e.tx.Hash().Hex() + " sent but not mined yet"
}

// isRevertError reports whether an eth_call error came from the EVM rejecting the
// call, as opposed to a transport or node failure. In order:
//  1. a network error (connection refused, timeout, context deadline) is never a
//     revert, whatever its text says;
//  2. an RPC error with code 3 is the standard "execution reverted" code (geth,
//     Nethermind, most L2 nodes);
//  3. otherwise fall back to the message, which is how Hardhat reports it
//     ("VM Exception while processing transaction: reverted ...").
func isRevertError(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return false
	}
	var rpcErr rpc.Error
	if errors.As(err, &rpcErr) && rpcErr.ErrorCode() == 3 {
		return true
	}
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
	// Sent but not mined within RELAYER_TX_TIMEOUT: accepted, outcome unknown.
	var pending *errTxPending
	if errors.As(err, &pending) {
		c.JSON(http.StatusAccepted, gin.H{"txHash": pending.tx.Hash().Hex(), "status": "pending"})
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
	return h.transactTo(h.dvpAddr, h.dvpABI, method, args...)
}

// transactTo is transact against any contract: the partial-withdraw route
// calls a vault, not EnygmaDvp.
func (h *Handler) transactTo(to common.Address, contractABI abi.ABI, method string, args ...interface{}) (*types.Receipt, error) {
	return h.transactWith(to, contractABI, nil, method, args...)
}

// errFeeNoteNotKept reports that a fee note's opening could not be saved, so
// the transaction was NOT sent: the relayer would be paid with a note it can
// never spend.
type errFeeNoteNotKept struct{ cause error }

func (e *errFeeNoteNotKept) Error() string {
	return "could not keep the fee note's opening, so it would be unspendable; not relaying: " + e.cause.Error()
}
func (e *errFeeNoteNotKept) Unwrap() error { return e.cause }

// transactWith is transactTo with a step that runs once the simulation has
// passed and before the transaction is sent; if it fails, nothing is sent.
// The fee routes save the fee note's opening there, so a request whose
// simulation reverts leaves no record of a note that will never exist.
func (h *Handler) transactWith(to common.Address, contractABI abi.ABI, beforeSend func() error, method string, args ...interface{}) (*types.Receipt, error) {
	h.txMu.Lock()
	defer h.txMu.Unlock()

	data, err := contractABI.Pack(method, args...)
	if err != nil {
		return nil, fmt.Errorf("pack %s: %w", method, err)
	}

	simCtx, cancel := context.WithTimeout(context.Background(), simulateTimeout)
	defer cancel()
	if _, err := h.client.CallContract(simCtx, ethereum.CallMsg{
		From: h.auth.From,
		To:   &to,
		Gas:  h.auth.GasLimit,
		Data: data,
	}, nil); err != nil {
		if isRevertError(err) {
			return nil, &errWouldRevert{cause: err}
		}
		return nil, fmt.Errorf("simulate %s: %w", method, err)
	}
	if beforeSend != nil {
		if err := beforeSend(); err != nil {
			return nil, &errFeeNoteNotKept{cause: err}
		}
	}

	target := bind.NewBoundContract(to, contractABI, h.client, h.client, h.client)
	tx, err := target.Transact(h.auth, method, args...)
	if err != nil {
		return nil, err
	}
	// Bounded wait: an unbounded one held txMu forever if the transaction got
	// stuck (e.g. underpriced), blocking every later submission.
	timeout := h.txTimeout
	if timeout <= 0 {
		timeout = defaultTxTimeout
	}
	waitCtx, cancelWait := context.WithTimeout(context.Background(), timeout)
	defer cancelWait()
	receipt, err := bind.WaitMined(waitCtx, h.client, tx)
	if errors.Is(err, context.DeadlineExceeded) {
		return nil, &errTxPending{tx: tx}
	}
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
