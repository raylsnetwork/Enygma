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

	"enygma_retail_payments/layout"
	"enygma_retail_payments/relayer/config"

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

// txTimeout is the maximum time to wait for a transaction to be mined.
// txMu must never be held across this wait (see each handler's submission
// step below) — holding it would let one slow-to-mine transaction block
// every other request across all /relay/* endpoints indefinitely, since
// txMu is shared process-wide.
var txTimeout = 45 * time.Second // a var only so tests can shorten it

// pendingClaimHold bounds how long a note stays claimed for a transaction
// that was still unmined when its request timed out.
const pendingClaimHold = time.Hour

// nullifierKey is the in-flight claim for one note. It is scoped by vault,
// since nullifiers are per vault (the same tree/nullifier in the ERC-20 and
// USDr vaults are different notes), and it is the same on every route, so a
// note cannot be relayed through two routes at once.
func nullifierKey(vault common.Address, tree, nullifier *big.Int) string {
	return "nf:" + vault.Hex() + ":" + tree.String() + ":" + nullifier.String()
}

// tagKey and channelKey are the in-flight claims for a tag publication and a
// channel setup. Each kind of claim has its own prefix (notes use "nf:"), so
// a caller-supplied value can never name another kind's claim.
func tagKey(tag [32]byte) string { return "tag:" + hex.EncodeToString(tag[:]) }

func channelKey(c1 []byte) string { return "chan:" + crypto.Keccak256Hash(c1).Hex() }

// decodeTag decodes a 32-byte hex tag (0x prefix optional, any case).
func decodeTag(s, field string) ([32]byte, error) {
	var tag [32]byte
	b, err := decodeHexField(s, field)
	if err != nil {
		return tag, err
	}
	if len(b) != 32 {
		return tag, fmt.Errorf("%s must be 32 bytes, got %d", field, len(b))
	}
	copy(tag[:], b)
	return tag, nil
}

// claim marks every key as in-flight, or none of them if any already is.
func (h *Handler) claim(keys ...string) bool {
	for i, k := range keys {
		if _, loaded := h.inFlight.LoadOrStore(k, struct{}{}); loaded {
			h.release(keys[:i])
			return false
		}
	}
	return true
}

func (h *Handler) release(keys []string) {
	for _, k := range keys {
		h.inFlight.Delete(k)
	}
}

// releaseWhenMined releases keys once tx is mined (or pendingClaimHold passes).
func (h *Handler) releaseWhenMined(tx *types.Transaction, keys []string) {
	go func() {
		holdCtx, holdCancel := context.WithTimeout(context.Background(), pendingClaimHold)
		defer holdCancel()
		_, _ = bind.WaitMined(holdCtx, h.client, tx)
		h.release(keys)
	}()
}

// waitMined waits up to txTimeout for tx and releases keys once its outcome
// is known. If tx is still unmined it answers 202 with the hash and keeps
// the keys claimed until tx is mined (or pendingClaimHold passes): released
// earlier, a retry of the same notes would pass the "already spent" check
// and be sent again, reverting on-chain at the relayer's cost. ok is false
// when a response has already been written.
func (h *Handler) waitMined(c *gin.Context, tx *types.Transaction, keys []string) (*types.Receipt, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), txTimeout)
	defer cancel()
	receipt, err := bind.WaitMined(ctx, h.client, tx)
	if errors.Is(err, context.DeadlineExceeded) {
		h.releaseWhenMined(tx, keys)
		c.JSON(http.StatusAccepted, gin.H{"txHash": tx.Hash().Hex(), "status": "pending"})
		return nil, false
	}
	h.release(keys)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("wait mined: %s", err)})
		return nil, false
	}
	return receipt, true
}

// Handler holds all dependencies for the relay endpoints.
type Handler struct {
	cfg                    *config.Config
	dvpABI                 abi.ABI
	vaultABI               abi.ABI
	tagRegistryABI         abi.ABI
	tagChannelRegistryABI  abi.ABI
	dvpAddr                common.Address
	vaultAddr              common.Address
	usdrVaultAddr          common.Address // zero address unless configured — POST /relay/payment_usdr_fee 503s without it
	tagRegistryAddr        common.Address
	tagChannelRegistryAddr common.Address
	auth                   *bind.TransactOpts
	client                 *ethclient.Client
	txMu                   sync.Mutex // serializes on-chain submissions — prevents nonce races
	inFlight               sync.Map   // prevents concurrent duplicate submissions
	feeSpendPubKey         *big.Int   // nil unless RELAYER_FEE_SPEND_PRIVATE_KEY is configured
	// feeNotes keeps the opening of every fee note this relayer is paid with
	// (see feeNoteStore): without it those notes could never be spent.
	feeNotes *feeNoteStore
}

// NewHandler wires up the handler from config.
// Returns an error if the config is invalid or contracts can't be loaded.
func NewHandler(cfg *config.Config) (*Handler, error) {
	// Dial the chain.
	client, err := ethclient.Dial(cfg.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("ethclient.Dial(%s): %w", cfg.RPCURL, err)
	}

	// Load signing key.
	privKey, err := crypto.HexToECDSA(cfg.RelayerPrivateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("parse relayer private key: %w", err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(privKey, cfg.ChainID)
	if err != nil {
		return nil, fmt.Errorf("build transactor: %w", err)
	}
	auth.GasLimit = 8_000_000

	// Resolve contract addresses: explicit env vars take priority over receipts.json.
	dvpAddrStr := cfg.EnygmaDvpAddr
	vaultAddrStr := cfg.Erc20VaultAddr
	if dvpAddrStr == "" || vaultAddrStr == "" {
		receipts, err := loadReceipts(cfg.ReceiptsPath)
		if err != nil {
			return nil, fmt.Errorf("load receipts: %w", err)
		}
		if dvpAddrStr == "" {
			dvpAddrStr = receipts["EnygmaDvp"].ContractAddress
		}
		if vaultAddrStr == "" {
			vaultAddrStr = receipts["Erc20CoinVault"].ContractAddress
		}
	}
	if dvpAddrStr == "" || vaultAddrStr == "" {
		return nil, fmt.Errorf("EnygmaDvp and Erc20CoinVault addresses must be set via env or receipts.json")
	}

	// Resolve UsdrCoinVault address — optional, mirrors TagRegistry's pattern:
	// POST /relay/payment_usdr_fee returns 503 if it's never configured.
	usdrVaultAddrStr := cfg.UsdrVaultAddr
	if usdrVaultAddrStr == "" {
		receipts, err := loadReceipts(cfg.ReceiptsPath)
		if err == nil {
			usdrVaultAddrStr = receipts["UsdrCoinVault"].ContractAddress
		}
	}

	// Resolve TagRegistry address.
	tagRegistryAddrStr := cfg.TagRegistryAddr
	if tagRegistryAddrStr == "" {
		receipts, err := loadReceipts(cfg.ReceiptsPath)
		if err == nil {
			tagRegistryAddrStr = receipts["TagRegistry"].ContractAddress
		}
	}
	// TagRegistry is optional — if not configured the /relay/tag endpoint returns 503.

	// Load ABIs.
	dvpABI, err := loadABIFromFile("../contracts/abis/EnygmaDvp.json")
	if err != nil {
		return nil, fmt.Errorf("load EnygmaDvp ABI: %w", err)
	}
	vaultABI, err := loadABIFromFile("../contracts/abis/Erc20CoinVault.json")
	if err != nil {
		return nil, fmt.Errorf("load Erc20CoinVault ABI: %w", err)
	}
	tagRegistryABI, err := loadABIFromFile("../private_tags/contracts/TagRegistry.json")
	if err != nil {
		// Non-fatal: tag relay will be unavailable but payment relay still works.
		tagRegistryABI = abi.ABI{}
	}

	tagChannelRegistryABI, err := loadABIFromFile("../private_tags/contracts/TagChannelRegistry.json")
	if err != nil {
		// Non-fatal: channel relay will be unavailable but other endpoints still work.
		tagChannelRegistryABI = abi.ABI{}
	}

	// Resolve TagChannelRegistry address.
	tagChannelRegistryAddrStr := cfg.TagChannelRegistryAddr
	if tagChannelRegistryAddrStr == "" {
		receipts, err := loadReceipts(cfg.ReceiptsPath)
		if err == nil {
			tagChannelRegistryAddrStr = receipts["TagChannelRegistry"].ContractAddress
		}
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
		cfg:                    cfg,
		dvpABI:                 dvpABI,
		vaultABI:               vaultABI,
		tagRegistryABI:         tagRegistryABI,
		tagChannelRegistryABI:  tagChannelRegistryABI,
		dvpAddr:                common.HexToAddress(dvpAddrStr),
		vaultAddr:              common.HexToAddress(vaultAddrStr),
		usdrVaultAddr:          common.HexToAddress(usdrVaultAddrStr),
		tagRegistryAddr:        common.HexToAddress(tagRegistryAddrStr),
		tagChannelRegistryAddr: common.HexToAddress(tagChannelRegistryAddrStr),
		auth:                   auth,
		client:                 client,
		feeSpendPubKey:         feeSpendPubKey,
		feeNotes:               newFeeNoteStore(cfg.FeeNotesPath),
	}, nil
}

// Info is the gin handler for GET /relay/info.
// Returns the relayer's configured contract addresses and chain ID.
// This endpoint is public (no auth) so clients can discover addresses
// without needing them pre-configured out-of-band.
func (h *Handler) Info(c *gin.Context) {
	feeSpendPubKey := ""
	if h.feeSpendPubKey != nil {
		feeSpendPubKey = h.feeSpendPubKey.String()
	}
	c.JSON(http.StatusOK, InfoResponse{
		RelayerAddr:            h.auth.From.Hex(),
		TagRegistryAddr:        h.tagRegistryAddr.Hex(),
		TagChannelRegistryAddr: h.tagChannelRegistryAddr.Hex(),
		ChainID:                h.cfg.ChainID.Int64(),
		FeeSpendPubKey:         feeSpendPubKey,
	})
}

// RelayPayment is the gin handler for POST /relay/payment.
//
// Validation steps:
//  1. Parse and decode all fields from the request body.
//  2. Check the Merkle root is known to the vault (rootHistory).
//  3. Check the nullifier has not been spent (nullifiers mapping).
//  4. Simulate the call, then sign and submit to EnygmaDvp.payment().
func (h *Handler) RelayPayment(c *gin.Context) {
	var req RelayPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Step 1 — decode all big.Int fields.
	p, err := parseRequest(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("parse: %s", err)})
		return
	}

	if !h.requireVault(c, "vaultId", p.vaultId, h.vaultAddr) {
		return
	}

	treeNum := p.publicSignal[layout.PaymentTreeNumbersOffset]
	root := p.publicSignal[layout.PaymentMerkleRootsOffset]
	nullifier := p.publicSignal[layout.PaymentNullifiersOffset]

	vault := bind.NewBoundContract(h.vaultAddr, h.vaultABI, h.client, h.client, h.client)

	// Step 2 — Merkle root must be in the vault's rootHistory.
	var rootResult []interface{}
	if err := vault.Call(&bind.CallOpts{}, &rootResult, "rootHistory", treeNum, root); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("rootHistory check: %s", err)})
		return
	}
	rootKnown, ok := rootResult[0].(bool)
	if !ok || !rootKnown {
		c.JSON(http.StatusBadRequest, gin.H{"error": "merkle root is not a known vault root"})
		return
	}

	// Step 3 — Nullifier must not already be spent.
	var nfResult []interface{}
	if err := vault.Call(&bind.CallOpts{}, &nfResult, "nullifiers", treeNum, nullifier); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("nullifiers check: %s", err)})
		return
	}
	nfSpent, ok := nfResult[0].(bool)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unexpected type from nullifiers()"})
		return
	}
	if nfSpent {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nullifier already spent"})
		return
	}

	// Step 4 — claim the note as in-flight to block concurrent duplicate
	// submissions, on this route or any other (see nullifierKey).
	nfKeys := []string{nullifierKey(h.vaultAddr, treeNum, nullifier)}
	if !h.claim(nfKeys...) {
		c.JSON(http.StatusConflict, gin.H{"error": "nullifier already in-flight"})
		return
	}

	// Step 5 — build ProofReceipt and simulate it: nothing is sent for a
	// request the chain would reject.
	receipt := buildProofReceipt(p)
	if !h.simulate(c, nfKeys, h.dvpAddr, h.dvpABI, "payment", receipt, p.vaultId, p.cipherText, p.encTxData) {
		return
	}

	// Step 6 — serialize submission to prevent nonce races under concurrency.
	// txMu guards only the submission itself, not the wait below — see
	// txTimeout's doc comment.
	h.txMu.Lock()
	dvp := bind.NewBoundContract(h.dvpAddr, h.dvpABI, h.client, h.client, h.client)
	tx, err := dvp.Transact(h.auth, "payment", receipt, p.vaultId, p.cipherText, p.encTxData)
	h.txMu.Unlock()
	if err != nil {
		h.release(nfKeys)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("payment(): %s", err)})
		return
	}
	txReceipt, ok := h.waitMined(c, tx, nfKeys)
	if !ok {
		return
	}
	if txReceipt.Status == types.ReceiptStatusFailed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "transaction reverted on-chain"})
		return
	}

	c.JSON(http.StatusOK, RelayPaymentResponse{
		TxHash:      txReceipt.TxHash.Hex(),
		BlockNumber: txReceipt.BlockNumber.Uint64(),
		GasUsed:     txReceipt.GasUsed,
	})
}

// RelayPaymentRelayerFee is the gin handler for POST /relay/payment_relayer_fee.
//
// Submits a PaymentRelayerFeePublic proof (1 input / 3 outputs): Alice pays
// Bob (output 0), keeps change (output 1), and leaves a spendable fee note
// for the relayer (output 2) whose amount is the public StFee signal,
// enforced on-chain in EnygmaDvp.paymentWithRelayerFee() against
// relayerFixedFeeAmount.
//
// Validation steps:
//  1. Parse and decode all fields from the request body.
//  2. Confirm relayer-fee relaying is configured (RELAYER_FEE_SPEND_PRIVATE_KEY).
//  3. Check the Merkle root is known to the vault (rootHistory).
//  4. Check the nullifier has not been spent (nullifiers mapping).
//  5. Enforce the configured minimum fee (RELAYER_MIN_FEE) against the public fee.
//  6. Confirm the fee note is actually addressed to this relayer: recompute
//     Erc20CommitmentV2(feeSpendPubKey, feeSalt, fee, tokenId) and compare
//     against the fee note's commitment (output 2). This is an off-chain sanity check for the
//     relayer's own benefit — the fee AMOUNT is enforced on-chain by the
//     contract (relayerFixedFeeAmount); this step only confirms the relayer
//     can actually spend the note it's about to pay gas to help create.
//  7. Simulate the call, keep the fee note's opening (feeNoteStore), then
//     sign and submit to EnygmaDvp.paymentWithRelayerFee().
func (h *Handler) RelayPaymentRelayerFee(c *gin.Context) {
	// Step 2 — relayer-fee relaying must be configured.
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

	// Step 1 — decode all big.Int fields.
	p, err := parseRelayerFeeRequest(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("parse: %s", err)})
		return
	}

	// Steps 3–6.
	if !h.checkRelayerFeeLeg(c, p) {
		return
	}

	treeNum := p.publicSignal[layout.PaymentRelayerFeeTreeNumbersOffset]
	nullifier := p.publicSignal[layout.PaymentRelayerFeeNullifiersOffset]

	// Step 7 — claim nullifier as in-flight to block concurrent duplicate submissions.
	// Keyed per note, not per route: the same note relayed concurrently
	// through /relay/payment and this route used to pass both checks.
	nfKeys := []string{nullifierKey(h.vaultAddr, treeNum, nullifier)}
	if !h.claim(nfKeys...) {
		c.JSON(http.StatusConflict, gin.H{"error": "nullifier already in-flight"})
		return
	}

	// Step 8 — simulate: nothing is sent, or recorded, for a request the chain
	// would reject.
	receipt := buildRelayerFeeProofReceipt(p)
	if !h.simulate(c, nfKeys, h.dvpAddr, h.dvpABI, "paymentWithRelayerFee", receipt, p.vaultId, p.cipherText, p.encTxData) {
		return
	}

	// Step 9 — keep the fee note's opening before sending: the salt reaches
	// this relayer only through the request.
	cmtRelayer, fee := p.publicSignal[relayerFeeNoteOffset], p.publicSignal[layout.PaymentRelayerFeeFeeOffset]
	if err := h.feeNotes.recordSubmitted("paymentWithRelayerFee", p.vaultId, cmtRelayer, p.feeSalt, fee, p.tokenId); err != nil {
		h.release(nfKeys)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("could not keep the fee note's opening, so it would be unspendable; not relaying: %s", err)})
		return
	}

	// Step 10 — serialize submission to prevent nonce races under concurrency.
	// txMu guards only the submission itself, not the wait below — see
	// txTimeout's doc comment.
	h.txMu.Lock()
	dvp := bind.NewBoundContract(h.dvpAddr, h.dvpABI, h.client, h.client, h.client)
	tx, err := dvp.Transact(h.auth, "paymentWithRelayerFee", receipt, p.vaultId, p.cipherText, p.encTxData)
	h.txMu.Unlock()
	if err != nil {
		h.release(nfKeys)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("paymentWithRelayerFee(): %s", err)})
		return
	}
	txReceipt, ok := h.waitMined(c, tx, nfKeys)
	if !ok {
		return
	}
	if txReceipt.Status == types.ReceiptStatusFailed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "transaction reverted on-chain"})
		return
	}
	h.recordFeeNoteMined("paymentWithRelayerFee", p.vaultId, cmtRelayer, txReceipt)

	c.JSON(http.StatusOK, RelayPaymentRelayerFeeResponse{
		TxHash:      txReceipt.TxHash.Hex(),
		BlockNumber: txReceipt.BlockNumber.Uint64(),
		GasUsed:     txReceipt.GasUsed,
	})
}

// checkRelayerFeeLeg runs the checks a PaymentRelayerFeePublic receipt needs
// before this relayer pays gas for it (RelayPaymentRelayerFee's steps 3–6,
// shared with RelayPartialWithdraw): the Merkle root is known, the nullifier
// unspent, the fee at least RELAYER_MIN_FEE, and the fee note addressed to this
// relayer, after requireVault. It writes the error response and returns false
// if any check fails.
func (h *Handler) checkRelayerFeeLeg(c *gin.Context, p *parsedRelayerFee) bool {
	if !h.requireVault(c, "vaultId", p.vaultId, h.vaultAddr) {
		return false
	}

	treeNum := p.publicSignal[layout.PaymentRelayerFeeTreeNumbersOffset]
	root := p.publicSignal[layout.PaymentRelayerFeeMerkleRootsOffset]
	nullifier := p.publicSignal[layout.PaymentRelayerFeeNullifiersOffset]
	cmtRelayer := p.publicSignal[relayerFeeNoteOffset]
	fee := p.publicSignal[layout.PaymentRelayerFeeFeeOffset]

	vault := bind.NewBoundContract(h.vaultAddr, h.vaultABI, h.client, h.client, h.client)

	// Step 3 — Merkle root must be in the vault's rootHistory.
	var rootResult []interface{}
	if err := vault.Call(&bind.CallOpts{}, &rootResult, "rootHistory", treeNum, root); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("rootHistory check: %s", err)})
		return false
	}
	rootKnown, ok := rootResult[0].(bool)
	if !ok || !rootKnown {
		c.JSON(http.StatusBadRequest, gin.H{"error": "merkle root is not a known vault root"})
		return false
	}

	// Step 4 — Nullifier must not already be spent.
	var nfResult []interface{}
	if err := vault.Call(&bind.CallOpts{}, &nfResult, "nullifiers", treeNum, nullifier); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("nullifiers check: %s", err)})
		return false
	}
	nfSpent, ok := nfResult[0].(bool)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unexpected type from nullifiers()"})
		return false
	}
	if nfSpent {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nullifier already spent"})
		return false
	}

	// Step 5 — minimum fee floor.
	if h.cfg.MinFee.Sign() > 0 && fee.Cmp(h.cfg.MinFee) < 0 {
		c.JSON(http.StatusPaymentRequired, gin.H{
			"error": fmt.Sprintf("fee %s is below the relayer's minimum %s", fee, h.cfg.MinFee),
		})
		return false
	}

	// Step 6 — the fee note must actually be addressed to this relayer's key.
	expectedFeeCmt, err := poseidon.Hash([]*big.Int{h.feeSpendPubKey, p.feeSalt, fee, p.tokenId})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("compute expected fee commitment: %s", err)})
		return false
	}
	if expectedFeeCmt.Cmp(cmtRelayer) != 0 {
		c.JSON(http.StatusPaymentRequired, gin.H{
			"error": fmt.Sprintf("fee note is not addressed to this relayer's published spend key "+
				"(GET /relay/info) — publicSignal[%d] does not match "+
				"Poseidon(relayerFeeSpendPubKey, feeSalt, StFee, tokenId)", relayerFeeNoteOffset),
		})
		return false
	}

	return true
}

// simulateTimeout bounds the pre-flight simulation of a submission.
const simulateTimeout = 15 * time.Second

// simulate runs the call against the current chain state, at the gas limit the
// transaction will be sent with, before anything is sent or recorded. A request
// with a bad proof or a stale root would otherwise be sent and revert on-chain:
// a failed pairing check uses all the gas it is given, paid by the relayer. On
// failure it releases keys, writes the response (422 if the call reverts, 500
// if the node could not run it) and returns false.
func (h *Handler) simulate(c *gin.Context, keys []string, to common.Address, contractABI abi.ABI, method string, args ...interface{}) bool {
	data, err := contractABI.Pack(method, args...)
	if err != nil {
		h.release(keys)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("pack %s: %s", method, err)})
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), simulateTimeout)
	defer cancel()
	if _, err := h.client.CallContract(ctx, ethereum.CallMsg{From: h.auth.From, To: &to, Gas: h.auth.GasLimit, Data: data}, nil); err != nil {
		h.release(keys)
		if isRevertError(err) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("%s() would revert: %s", method, err)})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("simulate %s(): %s", method, err)})
		}
		return false
	}
	return true
}

// isRevertError reports whether a simulation failed because the call reverted
// (the request's fault) rather than because the node could not run it: the
// JSON-RPC execution-error code 3, or a node that only says so in its message.
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

// requireVault checks that the vaultId a request names is the vault this
// relayer checks its roots and nullifiers against (want). EnygmaDvp settles
// against vaultById(vaultId), so with another id the request would be checked
// against one vault and settled against a different one: it would revert at
// the relayer's cost, and a fee note would be recorded under the wrong vault.
// It writes the error response and returns false on a mismatch.
func (h *Handler) requireVault(c *gin.Context, field string, vaultId *big.Int, want common.Address) bool {
	dvp := bind.NewBoundContract(h.dvpAddr, h.dvpABI, h.client, h.client, h.client)
	var out []interface{}
	if err := dvp.Call(&bind.CallOpts{}, &out, "vaultById", vaultId); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("vaultById(%s): %s", vaultId, err)})
		return false
	}
	got, ok := out[0].(common.Address)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unexpected type from vaultById()"})
		return false
	}
	if got != want {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("%s %s is vault %s, but this relayer relays for vault %s", field, vaultId, got.Hex(), want.Hex()),
		})
		return false
	}
	return true
}

// recordFeeNoteMined notes the transaction that inserted a fee note. Best
// effort: the opening is already kept by recordSubmitted.
func (h *Handler) recordFeeNoteMined(route string, vaultId, commitment *big.Int, r *types.Receipt) {
	if err := h.feeNotes.recordMined(route, vaultId, commitment, r.TxHash.Hex(), r.BlockNumber.Uint64()); err != nil {
		log.Printf("[relay] fee note store: recording the mined fee note: %v (its opening is already kept)", err)
	}
}

// withdrawMessage is the StMessage of a withdrawal proof: WITHDRAW_MESSAGE in
// enygma_dvp's IEnygmaDvp.sol, MessageWithdraw in the gnark Payment templates.
const withdrawMessage = 1

// relayerFeeNoteOffset is where a PaymentRelayerFee statement holds the
// relayer's fee note commitment: output 2, after the payment (or withdrawal)
// and the change.
const relayerFeeNoteOffset = layout.PaymentRelayerFeeCommitmentsOutOffset + 2

// RelayPartialWithdraw is the gin handler for POST /relay/partial_withdraw.
//
// Withdraws part of a note for its owner, who pays this relayer from the note
// instead of paying gas: a PaymentRelayerFeePublic proof whose outputs are the
// withdrawal (to Recipient), the owner's change note and this relayer's fee
// note, settled with Erc20CoinVault.partialWithdrawWithRelayerFee.
//
//  1. Confirm relayer-fee relaying is configured, and parse the request.
//  2. Require the withdrawal StMessage, and that output[0] opens to
//     Poseidon4(uint160(Recipient), 0, Amount, TokenId), so the request says
//     what the proof pays out.
//  3. The checks RelayPaymentRelayerFee makes (checkRelayerFeeLeg): known
//     root, unspent nullifier, RELAYER_MIN_FEE, fee note addressed to this
//     relayer.
//  4. Claim the nullifier, simulate the call, then submit it to the vault.
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
	p, err := parseRelayerFeeRequest(&req.RelayPaymentRelayerFeeRequest)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("parse: %s", err)})
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

	message := p.publicSignal[layout.PaymentRelayerFeeMessageOffset]
	if message.Cmp(big.NewInt(withdrawMessage)) != 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("publicSignal[%d] (StMessage) is %s; a withdrawal proof must carry %d", layout.PaymentRelayerFeeMessageOffset, message, withdrawMessage)})
		return
	}
	withdrawCmt, err := poseidon.Hash([]*big.Int{new(big.Int).SetBytes(recipient.Bytes()), big.NewInt(0), amount, p.tokenId})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("compute withdrawal commitment: %s", err)})
		return
	}
	if withdrawCmt.Cmp(p.publicSignal[layout.PaymentRelayerFeeCommitmentsOutOffset]) != 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("publicSignal[%d] does not match Poseidon(uint160(recipient), 0, amount, tokenId) — "+
				"the proof does not withdraw this amount to this recipient", layout.PaymentRelayerFeeCommitmentsOutOffset),
		})
		return
	}

	if !h.checkRelayerFeeLeg(c, p) {
		return
	}

	treeNum := p.publicSignal[layout.PaymentRelayerFeeTreeNumbersOffset]
	nullifier := p.publicSignal[layout.PaymentRelayerFeeNullifiersOffset]
	nfKeys := []string{nullifierKey(h.vaultAddr, treeNum, nullifier)}
	if !h.claim(nfKeys...) {
		c.JSON(http.StatusConflict, gin.H{"error": "nullifier already in-flight"})
		return
	}

	const method = "partialWithdrawWithRelayerFee"
	receipt := buildRelayerFeeProofReceipt(p)
	args := []interface{}{[]*big.Int{amount, p.tokenId}, recipient, receipt, p.cipherText, p.encTxData}

	if !h.simulate(c, nfKeys, h.vaultAddr, h.vaultABI, method, args...) {
		return
	}
	cmtRelayer, fee := p.publicSignal[relayerFeeNoteOffset], p.publicSignal[layout.PaymentRelayerFeeFeeOffset]
	if err := h.feeNotes.recordSubmitted(method, p.vaultId, cmtRelayer, p.feeSalt, fee, p.tokenId); err != nil {
		h.release(nfKeys)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("could not keep the fee note's opening, so it would be unspendable; not relaying: %s", err)})
		return
	}

	h.txMu.Lock()
	vault := bind.NewBoundContract(h.vaultAddr, h.vaultABI, h.client, h.client, h.client)
	tx, err := vault.Transact(h.auth, method, args...)
	h.txMu.Unlock()
	if err != nil {
		h.release(nfKeys)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("%s(): %s", method, err)})
		return
	}
	txReceipt, ok := h.waitMined(c, tx, nfKeys)
	if !ok {
		return
	}
	if txReceipt.Status == types.ReceiptStatusFailed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "transaction reverted on-chain"})
		return
	}
	h.recordFeeNoteMined(method, p.vaultId, cmtRelayer, txReceipt)
	c.JSON(http.StatusOK, RelayPaymentRelayerFeeResponse{
		TxHash:      txReceipt.TxHash.Hex(),
		BlockNumber: txReceipt.BlockNumber.Uint64(),
		GasUsed:     txReceipt.GasUsed,
	})
}

// RelayPaymentUsdrFee is the gin handler for POST /relay/payment_usdr_fee.
//
// Settles two independent proofs atomically in one call
// (EnygmaDvp.paymentWithUsdrFee): a normal payment against the main vault —
// a Payment or, merging two notes, a Payment2in proof — and a UsdrFeeCircuit
// proof — a second, independent relayer-fee asset with its own
// token/vault/circuit — against the USDr vault.
//
// Validation steps:
//  1. Confirm the USDr vault is configured (503 if not).
//  2. Parse and decode both legs' fields from the request body; the main
//     leg's statement length picks its layout (any other length is a 400).
//  3. Check every input's Merkle root is known and nullifier unspent, on
//     both legs.
//  4. Enforce the configured minimum fee (RELAYER_MIN_FEE) against the USDr
//     leg's public StFee.
//  5. Confirm the USDr fee note is addressed to this relayer: recompute
//     Poseidon(feeSpendPubKey, feeSalt, StFee, StTokenId) — both StFee and
//     StTokenId read straight from the USDr leg's own public signal — and
//     compare against the fee note's commitment (output 0 of the
//     UsdrFeeCircuit).
//  6. Simulate the call, keep the fee note's opening (feeNoteStore), then
//     sign and submit both receipts to EnygmaDvp.paymentWithUsdrFee().
func (h *Handler) RelayPaymentUsdrFee(c *gin.Context) {
	if (h.usdrVaultAddr == common.Address{}) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "USDr vault not configured — set RELAYER_USDR_VAULT_ADDR",
		})
		return
	}
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

	p, err := parseUsdrFeeRequest(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("parse: %s", err)})
		return
	}

	if !h.requireVault(c, "vaultId", p.vaultId, h.vaultAddr) ||
		!h.requireVault(c, "usdrVaultId", p.usdrVaultId, h.usdrVaultAddr) {
		return
	}

	usdrTreeNum := p.usdrPublicSignal[layout.UsdrFeeTreeNumbersOffset]
	usdrRoot := p.usdrPublicSignal[layout.UsdrFeeMerkleRootsOffset]
	usdrNullifier := p.usdrPublicSignal[layout.UsdrFeeNullifiersOffset]
	cmtFee := p.usdrPublicSignal[layout.UsdrFeeCommitmentsOutOffset] // output 0: the fee note
	fee := p.usdrPublicSignal[layout.UsdrFeeFeeOffset]
	tokenId := p.usdrPublicSignal[layout.UsdrFeeTokenIdOffset]

	vault := bind.NewBoundContract(h.vaultAddr, h.vaultABI, h.client, h.client, h.client)
	usdrVault := bind.NewBoundContract(h.usdrVaultAddr, h.vaultABI, h.client, h.client, h.client)

	// Step 3 — root/nullifier checks, every input of both legs.
	type input struct {
		name      string
		vault     *bind.BoundContract
		vaultAddr common.Address
		treeNum   *big.Int
		root      *big.Int
		nullifier *big.Int
	}
	var inputs []input
	for i := 0; i < p.mainLayout.inputs; i++ {
		name := "main"
		if p.mainLayout.inputs > 1 {
			name = fmt.Sprintf("main input %d", i)
		}
		inputs = append(inputs, input{name, vault, h.vaultAddr,
			p.publicSignal[p.mainLayout.treeNumbersOffset+i],
			p.publicSignal[p.mainLayout.merkleRootsOffset+i],
			p.publicSignal[p.mainLayout.nullifiersOffset+i]})
	}
	inputs = append(inputs, input{"usdr", usdrVault, h.usdrVaultAddr, usdrTreeNum, usdrRoot, usdrNullifier})
	for _, in := range inputs {
		var rootResult []interface{}
		if err := in.vault.Call(&bind.CallOpts{}, &rootResult, "rootHistory", in.treeNum, in.root); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("%s rootHistory check: %s", in.name, err)})
			return
		}
		if rootKnown, ok := rootResult[0].(bool); !ok || !rootKnown {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("%s: merkle root is not a known vault root", in.name)})
			return
		}

		var nfResult []interface{}
		if err := in.vault.Call(&bind.CallOpts{}, &nfResult, "nullifiers", in.treeNum, in.nullifier); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("%s nullifiers check: %s", in.name, err)})
			return
		}
		nfSpent, ok := nfResult[0].(bool)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("%s: unexpected type from nullifiers()", in.name)})
			return
		}
		if nfSpent {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("%s: nullifier already spent", in.name)})
			return
		}
	}

	// Step 4 — minimum fee floor.
	if h.cfg.MinFee.Sign() > 0 && fee.Cmp(h.cfg.MinFee) < 0 {
		c.JSON(http.StatusPaymentRequired, gin.H{
			"error": fmt.Sprintf("fee %s is below the relayer's minimum %s", fee, h.cfg.MinFee),
		})
		return
	}

	// Step 5 — the USDr fee note must actually be addressed to this relayer's key.
	expectedFeeCmt, err := poseidon.Hash([]*big.Int{h.feeSpendPubKey, p.usdrFeeSalt, fee, tokenId})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("compute expected fee commitment: %s", err)})
		return
	}
	if expectedFeeCmt.Cmp(cmtFee) != 0 {
		c.JSON(http.StatusPaymentRequired, gin.H{
			"error": fmt.Sprintf("USDr fee note is not addressed to this relayer's published spend key "+
				"(GET /relay/info) — usdrPublicSignal[%d] does not match "+
				"Poseidon(relayerFeeSpendPubKey, usdrFeeSalt, StFee, StTokenId)", layout.UsdrFeeCommitmentsOutOffset),
		})
		return
	}

	// Step (dedup) — claim every note as in-flight, each in its own vault.
	// One key per note (not one for the request) so each note is also
	// blocked from being relayed concurrently through another route.
	var nfKeys []string
	for _, in := range inputs {
		nfKeys = append(nfKeys, nullifierKey(in.vaultAddr, in.treeNum, in.nullifier))
	}
	if !h.claim(nfKeys...) {
		c.JSON(http.StatusConflict, gin.H{"error": "nullifier already in-flight"})
		return
	}

	// Simulate, then keep the USDr fee note's opening before sending: the salt
	// reaches this relayer only through the request.
	receipt, usdrReceipt := buildUsdrFeeProofReceipts(p)
	if !h.simulate(c, nfKeys, h.dvpAddr, h.dvpABI, "paymentWithUsdrFee",
		receipt, p.vaultId, p.cipherText, p.encTxData,
		usdrReceipt, p.usdrVaultId, p.usdrCipherText, p.usdrEncTxData) {
		return
	}
	if err := h.feeNotes.recordSubmitted("paymentWithUsdrFee", p.usdrVaultId, cmtFee, p.usdrFeeSalt, fee, tokenId); err != nil {
		h.release(nfKeys)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("could not keep the fee note's opening, so it would be unspendable; not relaying: %s", err)})
		return
	}

	// txMu guards only the submission itself, not the wait below — see
	// txTimeout's doc comment.
	h.txMu.Lock()

	// Step 6 — submit both receipts atomically.
	dvp := bind.NewBoundContract(h.dvpAddr, h.dvpABI, h.client, h.client, h.client)
	tx, err := dvp.Transact(h.auth, "paymentWithUsdrFee",
		receipt, p.vaultId, p.cipherText, p.encTxData,
		usdrReceipt, p.usdrVaultId, p.usdrCipherText, p.usdrEncTxData)
	h.txMu.Unlock()
	if err != nil {
		h.release(nfKeys)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("paymentWithUsdrFee(): %s", err)})
		return
	}
	txReceipt, ok := h.waitMined(c, tx, nfKeys)
	if !ok {
		return
	}
	if txReceipt.Status == types.ReceiptStatusFailed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "transaction reverted on-chain"})
		return
	}
	h.recordFeeNoteMined("paymentWithUsdrFee", p.usdrVaultId, cmtFee, txReceipt)

	c.JSON(http.StatusOK, RelayPaymentUsdrFeeResponse{
		TxHash:      txReceipt.TxHash.Hex(),
		BlockNumber: txReceipt.BlockNumber.Uint64(),
		GasUsed:     txReceipt.GasUsed,
	})
}

// RelayTag is the gin handler for POST /relay/tag.
//
// Supports two modes:
//
// Single-tag mode: client pre-computes tag for a predicted block.
// Window mode: client provides tags for multiple blocks; the relayer picks
// the correct one and retries (up to 3 times) if the tx drifts.
//
// Validation steps:
//  1. Decode ctxt and validate tag input (single or window).
//  2. Verify TagRegistry is configured (503 if not).
//  3. Claim in-flight lock to prevent concurrent duplicate submissions.
//  4. Serialize with txMu to prevent nonce races.
//  5. Submit publishTag() — in window mode, retry on block drift.
func (h *Handler) RelayTag(c *gin.Context) {
	var req RelayTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Step 1 — validate: either single tag or window, not both.
	windowMode := len(req.Tags) > 0
	if !windowMode && req.Tag == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "either tag (single) or tags+startBlock (window) is required"})
		return
	}
	if windowMode && req.Tag != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provide either tag or tags, not both"})
		return
	}
	if windowMode && len(req.Tags) > 16 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tags window too large (max 16)"})
		return
	}

	// The payload is under a per-block key, so it travels with its tag: one
	// ctxt in single mode, one per tag in window mode.
	var singleCtxt []byte
	var windowCtxts [][]byte
	if !windowMode {
		if req.Ctxt == "" || len(req.Ctxts) > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "single mode takes ctxt (not ctxts)"})
			return
		}
		ctxt, err := decodeHexField(req.Ctxt, "ctxt")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		singleCtxt = ctxt
	} else {
		if req.Ctxt != "" || len(req.Ctxts) != len(req.Tags) {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf(
				"window mode takes ctxts, one per tag (got %d tags, %d ctxts)", len(req.Tags), len(req.Ctxts))})
			return
		}
		windowCtxts = make([][]byte, len(req.Ctxts))
		for i, s := range req.Ctxts {
			ctxt, err := decodeHexField(s, fmt.Sprintf("ctxts[%d]", i))
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			windowCtxts[i] = ctxt
		}
	}

	// Decode and validate every tag up front: the claims below are keyed on
	// the decoded bytes, so all of them must be well-formed before claiming.
	var singleTag [32]byte
	var windowTags [][32]byte
	if !windowMode {
		tag, err := decodeTag(req.Tag, "tag")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		singleTag = tag
	} else {
		windowTags = make([][32]byte, len(req.Tags))
		for i, t := range req.Tags {
			tag, err := decodeTag(t, fmt.Sprintf("tags[%d]", i))
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			windowTags[i] = tag
		}
	}

	// Step 2 — TagRegistry must be configured.
	if (h.tagRegistryAddr == common.Address{}) || len(h.tagRegistryABI.Methods) == 0 {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "tag registry not configured — set RELAYER_TAG_REGISTRY_ADDR",
		})
		return
	}

	// Step 3 — claim the tag(s) as in-flight, keyed on the decoded bytes so
	// the same tag cannot slip past in another hex spelling (0x prefix,
	// letter case). In window mode every tag of the window is claimed, not
	// just the first: a shifted window that still covers the tag being
	// published collides on it.
	var claimKeys []string
	if windowMode {
		for _, t := range windowTags {
			claimKeys = append(claimKeys, tagKey(t))
		}
	} else {
		claimKeys = []string{tagKey(singleTag)}
	}
	if !h.claim(claimKeys...) {
		c.JSON(http.StatusConflict, gin.H{"error": "tag already in-flight"})
		return
	}
	// Released when the request ends, unless an attempt is still unmined:
	// then only once it is mined. TagRegistry only rejects a repeated tag
	// within one block, so a retry released early and landing in another
	// block would publish the same tag a second time at the relayer's cost.
	var pendingTx *types.Transaction
	defer func() {
		if pendingTx != nil {
			h.releaseWhenMined(pendingTx, claimKeys)
			return
		}
		h.release(claimKeys)
	}()

	// Step 5 — submit. Window mode retries up to 3 times on block drift.
	// txMu is locked/unlocked per attempt below, around the submission
	// only — not held across WaitMined (see txTimeout's doc comment) and
	// not held across the whole retry loop, so one slow-to-mine attempt
	// can't block every other request while this handler retries.
	registry := bind.NewBoundContract(h.tagRegistryAddr, h.tagRegistryABI, h.client, h.client, h.client)
	const maxAttempts = 3

	for attempt := 0; attempt < maxAttempts; attempt++ {
		var tag [32]byte
		var ctxt []byte

		if windowMode {
			// Pick the tag for the block this tx is expected to land in.
			currentBlock, err := h.client.BlockNumber(context.Background())
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("blockNumber: %s", err)})
				return
			}
			targetBlock := currentBlock + 1
			idx := int64(targetBlock) - int64(req.StartBlock)
			if idx < 0 || int(idx) >= len(req.Tags) {
				c.JSON(http.StatusConflict, gin.H{
					"error": fmt.Sprintf("window [%d, %d) does not cover expected block %d — resubmit with a new window",
						req.StartBlock, req.StartBlock+uint64(len(req.Tags)), targetBlock),
				})
				return
			}
			tag, ctxt = windowTags[idx], windowCtxts[idx]
		} else {
			tag, ctxt = singleTag, singleCtxt
		}

		h.txMu.Lock()
		tx, err := registry.Transact(h.auth, "publishTag", tag, ctxt)
		h.txMu.Unlock()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("publishTag(): %s", err)})
			return
		}
		waitCtx, cancel := context.WithTimeout(context.Background(), txTimeout)
		txReceipt, err := bind.WaitMined(waitCtx, h.client, tx)
		cancel()
		if errors.Is(err, context.DeadlineExceeded) {
			// Outcome unknown: the block-drift check below has not run, so in
			// window mode the client must confirm from the receipt that the
			// transaction landed in the block its tag was for.
			pendingTx = tx
			c.JSON(http.StatusAccepted, gin.H{"txHash": tx.Hash().Hex(), "status": "pending"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("wait mined: %s", err)})
			return
		}
		if txReceipt.Status == types.ReceiptStatusFailed {
			c.JSON(http.StatusBadRequest, gin.H{"error": "transaction reverted on-chain (tag may already exist in this block)"})
			return
		}

		actualBlock := txReceipt.BlockNumber.Uint64()

		// In window mode, check whether the tx landed in the block we targeted.
		// If not, retry using the correct tag for the actual block (which is in
		// the window since we just mined it).
		if windowMode {
			expectedBlock := actualBlock // we'll re-query on next attempt
			_ = expectedBlock
			// Verify the tag we submitted was for the correct block.
			// Re-query to check: the tag we used was for (startBlock + idx).
			// If the tx landed in a different block, the tag is wrong — retry.
			tagIdx := int64(actualBlock) - int64(req.StartBlock)
			chosenIdx := int64(0) // index we submitted
			// Re-derive the chosen index from the pre-submission query.
			// Since we can't recover it easily, we just check if actual block is
			// within the window and let the retry loop pick the right tag.
			if tagIdx < 0 || int(tagIdx) >= len(req.Tags) {
				// Landed outside the window entirely.
				c.JSON(http.StatusConflict, gin.H{
					"error": fmt.Sprintf("tx landed in block %d which is outside window [%d, %d) — resubmit",
						actualBlock, req.StartBlock, req.StartBlock+uint64(len(req.Tags))),
				})
				return
			}
			_ = chosenIdx
			// The correct tag for the actual block is tags[tagIdx].
			// Check if we already submitted the correct tag.
			correctTagBytes, _ := decodeHexField(req.Tags[tagIdx], "")
			if len(correctTagBytes) == 32 && correctTagBytes[0] == tag[0] {
				// Quick check: likely the right tag (exact check below is fine).
			}
			// Deep check: compare submitted tag with correct tag for actualBlock.
			submittedCorrect := true
			if len(correctTagBytes) == 32 {
				for i, b := range correctTagBytes {
					if tag[i] != b {
						submittedCorrect = false
						break
					}
				}
			}
			if submittedCorrect {
				c.JSON(http.StatusOK, RelayTagResponse{
					TxHash:      txReceipt.TxHash.Hex(),
					BlockNumber: actualBlock,
					GasUsed:     txReceipt.GasUsed,
				})
				return
			}
			// Wrong tag was submitted for this block — retry with the correct one.
			continue
		}

		c.JSON(http.StatusOK, RelayTagResponse{
			TxHash:      txReceipt.TxHash.Hex(),
			BlockNumber: actualBlock,
			GasUsed:     txReceipt.GasUsed,
		})
		return
	}

	c.JSON(http.StatusConflict, gin.H{"error": "exceeded retry limit — resubmit with a new window starting from the current block"})
}

// RelayChannel is the gin handler for POST /relay/channel.
//
// The client constructs (c1, c2, bitmap) locally via SetupChannel logic and
// sends them to the relayer. The relayer submits openChannel() on-chain using
// its own private key — msg.sender = relayer, not the actual sender.
//
// This provides sender privacy (paper §6.3): on-chain records show only the
// relayer address, so observers cannot link any channel to a specific user.
//
// Validation steps:
//  1. Decode c1 (must be exactly 1088 bytes), c2, and bitmap from hex.
//  2. Verify TagChannelRegistry is configured (503 if not).
//  3. Claim c1 hash as in-flight (409 if a duplicate is already processing).
//  4. Serialize submission with txMu to prevent nonce races.
//  5. Submit openChannel(c1, c2, bitmap) and parse the ChannelOpened event
//     to return the assigned channelIdx.
func (h *Handler) RelayChannel(c *gin.Context) {
	var req RelayChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Step 1 — decode and validate fields.
	c1, err := decodeHexField(req.C1, "c1")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(c1) != 1088 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("c1 must be 1088 bytes (ML-KEM-768), got %d", len(c1))})
		return
	}
	c2, err := decodeHexField(req.C2, "c2")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(c2) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "c2 must not be empty"})
		return
	}
	bitmap, err := decodeHexField(req.Bitmap, "bitmap")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Step 2 — TagChannelRegistry must be configured.
	if (h.tagChannelRegistryAddr == common.Address{}) || len(h.tagChannelRegistryABI.Methods) == 0 {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "tag channel registry not configured — set RELAYER_TAG_CHANNEL_REGISTRY_ADDR",
		})
		return
	}

	// Step 3 — claim c1 as in-flight (keyed by first 16 bytes of c1 hex).
	// Keyed on the decoded c1, so another hex spelling of it collides.
	claimKeys := []string{channelKey(c1)}
	if !h.claim(claimKeys...) {
		c.JSON(http.StatusConflict, gin.H{"error": "channel setup already in-flight"})
		return
	}

	// Step 4 — serialize submission to prevent nonce races. txMu guards
	// only the submission itself, not the wait below — see txTimeout's
	// doc comment.
	h.txMu.Lock()

	// Step 5 — submit openChannel(c1, c2, bitmap).
	registry := bind.NewBoundContract(h.tagChannelRegistryAddr, h.tagChannelRegistryABI, h.client, h.client, h.client)
	tx, err := registry.Transact(h.auth, "openChannel", c1, c2, bitmap)
	h.txMu.Unlock()
	if err != nil {
		h.release(claimKeys)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("openChannel(): %s", err)})
		return
	}
	// Held past a timeout until mined, so a retry is not published twice.
	txReceipt, ok := h.waitMined(c, tx, claimKeys)
	if !ok {
		return
	}
	if txReceipt.Status == types.ReceiptStatusFailed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "transaction reverted on-chain"})
		return
	}

	// Parse ChannelOpened event to extract the assigned channelIdx.
	var channelIdx uint64
	channelOpenedSig := crypto.Keccak256Hash([]byte("ChannelOpened(uint256,address)"))
	for _, log := range txReceipt.Logs {
		if log.Address != h.tagChannelRegistryAddr || len(log.Topics) < 2 {
			continue
		}
		if log.Topics[0] == channelOpenedSig {
			channelIdx = new(big.Int).SetBytes(log.Topics[1].Bytes()).Uint64()
			break
		}
	}

	c.JSON(http.StatusOK, RelayChannelResponse{
		ChannelIdx:  channelIdx,
		TxHash:      txReceipt.TxHash.Hex(),
		BlockNumber: txReceipt.BlockNumber.Uint64(),
		GasUsed:     txReceipt.GasUsed,
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ── helpers ───────────────────────────────────────────────────────────────────

func parseRequest(req *RelayPaymentRequest) (*parsed, error) {
	vaultId, ok := new(big.Int).SetString(req.VaultId, 10)
	if !ok {
		return nil, fmt.Errorf("invalid vaultId: %q", req.VaultId)
	}

	var proof [8]*big.Int
	for i, s := range req.Proof {
		n, ok := new(big.Int).SetString(s, 10)
		if !ok {
			return nil, fmt.Errorf("invalid proof[%d]: %q", i, s)
		}
		proof[i] = n
	}

	var sig [layout.PaymentLen]*big.Int
	for i, s := range req.PublicSignal {
		n, ok := new(big.Int).SetString(s, 10)
		if !ok {
			return nil, fmt.Errorf("invalid publicSignal[%d]: %q", i, s)
		}
		sig[i] = n
	}

	ctBytes, err := decodeHexField(req.CipherText, "cipherText")
	if err != nil {
		return nil, err
	}
	encBytes, err := decodeHexField(req.EncTxData, "encTxData")
	if err != nil {
		return nil, err
	}

	return &parsed{
		vaultId:      vaultId,
		proof:        proof,
		publicSignal: sig,
		cipherText:   ctBytes,
		encTxData:    encBytes,
	}, nil
}

func parseRelayerFeeRequest(req *RelayPaymentRelayerFeeRequest) (*parsedRelayerFee, error) {
	vaultId, ok := new(big.Int).SetString(req.VaultId, 10)
	if !ok {
		return nil, fmt.Errorf("invalid vaultId: %q", req.VaultId)
	}

	var proof [8]*big.Int
	for i, s := range req.Proof {
		n, ok := new(big.Int).SetString(s, 10)
		if !ok {
			return nil, fmt.Errorf("invalid proof[%d]: %q", i, s)
		}
		proof[i] = n
	}

	var sig [layout.PaymentRelayerFeeLen]*big.Int
	for i, s := range req.PublicSignal {
		n, ok := new(big.Int).SetString(s, 10)
		if !ok {
			return nil, fmt.Errorf("invalid publicSignal[%d]: %q", i, s)
		}
		sig[i] = n
	}

	feeSalt, ok := new(big.Int).SetString(req.FeeSalt, 10)
	if !ok {
		return nil, fmt.Errorf("invalid feeSalt: %q", req.FeeSalt)
	}
	tokenId, ok := new(big.Int).SetString(req.TokenId, 10)
	if !ok {
		return nil, fmt.Errorf("invalid tokenId: %q", req.TokenId)
	}

	ctBytes, err := decodeHexField(req.CipherText, "cipherText")
	if err != nil {
		return nil, err
	}
	encBytes, err := decodeHexField(req.EncTxData, "encTxData")
	if err != nil {
		return nil, err
	}

	return &parsedRelayerFee{
		vaultId:      vaultId,
		proof:        proof,
		publicSignal: sig,
		cipherText:   ctBytes,
		encTxData:    encBytes,
		feeSalt:      feeSalt,
		tokenId:      tokenId,
	}, nil
}

func parseUsdrFeeRequest(req *RelayPaymentUsdrFeeRequest) (*parsedUsdrFee, error) {
	vaultId, ok := new(big.Int).SetString(req.VaultId, 10)
	if !ok {
		return nil, fmt.Errorf("invalid vaultId: %q", req.VaultId)
	}
	usdrVaultId, ok := new(big.Int).SetString(req.UsdrVaultId, 10)
	if !ok {
		return nil, fmt.Errorf("invalid usdrVaultId: %q", req.UsdrVaultId)
	}

	var proof [8]*big.Int
	for i, s := range req.Proof {
		n, ok := new(big.Int).SetString(s, 10)
		if !ok {
			return nil, fmt.Errorf("invalid proof[%d]: %q", i, s)
		}
		proof[i] = n
	}
	var usdrProof [8]*big.Int
	for i, s := range req.UsdrProof {
		n, ok := new(big.Int).SetString(s, 10)
		if !ok {
			return nil, fmt.Errorf("invalid usdrProof[%d]: %q", i, s)
		}
		usdrProof[i] = n
	}

	mainLayout, ok := usdrFeeMainLegLayouts[len(req.PublicSignal)]
	if !ok {
		return nil, fmt.Errorf("publicSignal has %d signals; want a Payment (%d) or Payment2in (%d) statement",
			len(req.PublicSignal), layout.PaymentLen, layout.Payment2inLen)
	}
	sig := make([]*big.Int, len(req.PublicSignal))
	for i, s := range req.PublicSignal {
		n, ok := new(big.Int).SetString(s, 10)
		if !ok {
			return nil, fmt.Errorf("invalid publicSignal[%d]: %q", i, s)
		}
		sig[i] = n
	}
	var usdrSig [layout.UsdrFeeLen]*big.Int
	for i, s := range req.UsdrPublicSignal {
		n, ok := new(big.Int).SetString(s, 10)
		if !ok {
			return nil, fmt.Errorf("invalid usdrPublicSignal[%d]: %q", i, s)
		}
		usdrSig[i] = n
	}

	usdrFeeSalt, ok := new(big.Int).SetString(req.UsdrFeeSalt, 10)
	if !ok {
		return nil, fmt.Errorf("invalid usdrFeeSalt: %q", req.UsdrFeeSalt)
	}

	ctBytes, err := decodeHexField(req.CipherText, "cipherText")
	if err != nil {
		return nil, err
	}
	encBytes, err := decodeHexField(req.EncTxData, "encTxData")
	if err != nil {
		return nil, err
	}
	usdrCtBytes, err := decodeHexField(req.UsdrCipherText, "usdrCipherText")
	if err != nil {
		return nil, err
	}
	usdrEncBytes, err := decodeHexField(req.UsdrEncTxData, "usdrEncTxData")
	if err != nil {
		return nil, err
	}

	return &parsedUsdrFee{
		vaultId:          vaultId,
		proof:            proof,
		publicSignal:     sig,
		mainLayout:       mainLayout,
		cipherText:       ctBytes,
		encTxData:        encBytes,
		usdrVaultId:      usdrVaultId,
		usdrProof:        usdrProof,
		usdrPublicSignal: usdrSig,
		usdrCipherText:   usdrCtBytes,
		usdrEncTxData:    usdrEncBytes,
		usdrFeeSalt:      usdrFeeSalt,
	}, nil
}

func decodeHexField(s, fieldName string) ([]byte, error) {
	s = strings.TrimPrefix(s, "0x")
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("invalid hex in %s: %w", fieldName, err)
	}
	return b, nil
}

// buildProofReceipt maps the parsed proof + public signal into the
// ProofReceipt struct that EnygmaDvp.payment() expects.
// for a Payment proof.
func buildProofReceipt(p *parsed) proofReceipt {
	// Proof element order from gnark: [Ax, Ay, BX_imag, BX_real, BY_imag, BY_real, Cx, Cy]
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
		Statement:       append(make([]*big.Int, 0, layout.PaymentLen), p.publicSignal[:]...),
		NumberOfInputs:  big.NewInt(layout.PaymentNullifiersSize),
		NumberOfOutputs: big.NewInt(layout.PaymentCommitmentsOutSize),
	}
}

// buildRelayerFeeProofReceipt maps the parsed proof + public signal into the
// ProofReceipt struct that EnygmaDvp.paymentWithRelayerFee() expects, for a
// PaymentRelayerFeePublic proof.
func buildRelayerFeeProofReceipt(p *parsedRelayerFee) proofReceipt {
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
		Statement:       append(make([]*big.Int, 0, layout.PaymentRelayerFeeLen), p.publicSignal[:]...),
		NumberOfInputs:  big.NewInt(layout.PaymentRelayerFeeNullifiersSize),
		NumberOfOutputs: big.NewInt(layout.PaymentRelayerFeeCommitmentsOutSize),
	}
}

// buildUsdrFeeProofReceipts maps a parsedUsdrFee into the two ProofReceipt
// structs EnygmaDvp.paymentWithUsdrFee() expects — the main leg (a Payment
// or Payment2in proof, per p.mainLayout) and the USDr leg (a UsdrFee proof).
func buildUsdrFeeProofReceipts(p *parsedUsdrFee) (receipt, usdrReceipt proofReceipt) {
	sp := snarkProof{
		A: g1Point{X: p.proof[0], Y: p.proof[1]},
		B: g2Point{
			X: [2]*big.Int{p.proof[2], p.proof[3]},
			Y: [2]*big.Int{p.proof[4], p.proof[5]},
		},
		C: g1Point{X: p.proof[6], Y: p.proof[7]},
	}
	receipt = proofReceipt{
		Proof:           sp,
		Statement:       p.publicSignal,
		NumberOfInputs:  big.NewInt(int64(p.mainLayout.inputs)),
		NumberOfOutputs: big.NewInt(int64(p.mainLayout.outputs)),
	}

	usdrSp := snarkProof{
		A: g1Point{X: p.usdrProof[0], Y: p.usdrProof[1]},
		B: g2Point{
			X: [2]*big.Int{p.usdrProof[2], p.usdrProof[3]},
			Y: [2]*big.Int{p.usdrProof[4], p.usdrProof[5]},
		},
		C: g1Point{X: p.usdrProof[6], Y: p.usdrProof[7]},
	}
	usdrReceipt = proofReceipt{
		Proof:           usdrSp,
		Statement:       append(make([]*big.Int, 0, layout.UsdrFeeLen), p.usdrPublicSignal[:]...),
		NumberOfInputs:  big.NewInt(layout.UsdrFeeNullifiersSize),
		NumberOfOutputs: big.NewInt(layout.UsdrFeeCommitmentsOutSize),
	}

	return receipt, usdrReceipt
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

func loadABIFromFile(path string) (abi.ABI, error) {
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
