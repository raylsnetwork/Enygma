package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
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
	// bankTag (Fix H-09): the caller's per-bank credential identifier
	// (Fix H-06's bankID), passed through so the chain's own
	// RelayAttribution event can attribute this specific transaction —
	// previously only the relayer's own logs knew which bank asked for
	// a given submission.
	Transfer(opts *bind.TransactOpts, commitmentDeltas []enygma.IEnygmaPoint, proof enygma.IEnygmaProof, participantIds []*big.Int, bankTag string) (*types.Transaction, error)
	TransferWithFee(opts *bind.TransactOpts, commitmentDeltas []enygma.IEnygmaPoint, proof enygma.IEnygmaFeeProof, participantIds []*big.Int, bankTag string) (*types.Transaction, error)
}

// txTimeout is the maximum time to wait for a transaction to be mined.
const txTimeout = 45 * time.Second

// maxParticipants is the exact commitmentDeltas/participantIds length every
// Enygma transfer path requires on-chain (Enygma.sol's DEFAULT_SIZE — see
// _updateBalancesForTransfer's callers, which all revert InvalidParticipantCount
// / VerifierNotFound for any other length). Rejecting any other length here,
// before a transaction is ever built, is part of Fix H-10: a caller-supplied
// commitments/kIndex array with an attacker-chosen length was previously
// unbounded, so an oversized-but-guaranteed-to-revert payload could still be
// signed and broadcast, paying intrinsic gas for calldata that could not
// possibly succeed.
const maxParticipants = 6

// Fix L-05: field-order bounds. Every numeric field this relayer accepts
// is a decimal string parsed straight into a *big.Int and handed to
// abigen with no check against the field it's actually an element of —
// "nothing checked against BN254 r or q, the Baby Jubjub base field, or
// _totalRegisteredParties". A value at or above the relevant modulus is
// not a valid element of that field: for the SNARK's own public inputs
// (publicSignal) and the Baby Jubjub curve points this protocol embeds
// inside BN254's scalar field (commitments), that's bn254Fr; for the
// proof's own G1/G2 coordinates (proof[8]) it's bn254Fq, the curve's
// base field. Rejecting out-of-range values here — before any encoding,
// signing or broadcast — is strictly cheaper than letting Solidity's
// pairing precompile revert on them (or, worse, silently accepting a
// value Solidity's ABI encoder or the precompile reduces differently
// than the prover intended).
var (
	// bn254Fr is the BN254 scalar field order (21888242871839275222246405745257275088548364400416034343698204186575808495617).
	bn254Fr, _ = new(big.Int).SetString("21888242871839275222246405745257275088548364400416034343698204186575808495617", 10)
	// bn254Fq is the BN254 base field order (21888242871839275222246405745257275088696311157297823662689037894645226208583).
	bn254Fq, _ = new(big.Int).SetString("21888242871839275222246405745257275088696311157297823662689037894645226208583", 10)
)

// checkFieldElement rejects a decimal string that doesn't parse, is
// negative, or is >= modulus.
func checkFieldElement(label, s string, modulus *big.Int) (*big.Int, error) {
	n, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return nil, fmt.Errorf("%s: invalid decimal %q", label, s)
	}
	if n.Sign() < 0 {
		return nil, fmt.Errorf("%s: negative value %q is not a valid field element", label, s)
	}
	if n.Cmp(modulus) >= 0 {
		return nil, fmt.Errorf("%s: %q is >= the field modulus — not a valid field element", label, s)
	}
	return n, nil
}

// chainProbe is the subset of *ethclient.Client the Health handler uses to
// report chain liveness and the relayer's on-chain balance (Fix M-09).
// Nil in every test Handler (constructed via NewHandlerWithDeps without a
// real chain) — Health degrades gracefully rather than panicking.
type chainProbe interface {
	BlockNumber(ctx context.Context) (uint64, error)
	BalanceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (*big.Int, error)
}

// Handler holds all dependencies for the relay endpoints.
type Handler struct {
	cfg          *config.Config
	contractAddr string
	auth         *bind.TransactOpts // relayer's signing key — never leaves this process
	client       bind.DeployBackend // used only for bind.WaitMined; tests inject a mock
	chain        chainProbe         // used only by Health for liveness/balance; nil in tests (Fix M-09)
	instance     EnygmaContract     // Enygma contract binding; tests inject a mock
	txMu         sync.Mutex         // serializes only the on-chain *submission* (nonce assignment) — see RelayTransfer (Fix H-10)
	inFlight     sync.Map           // deduplicates concurrent identical submissions
	pendingTxs   int64              // count of submitted-but-not-yet-mined transactions (Fix M-09 /health signal), accessed via atomic
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

	// Build a transactor. Nonce is left nil so go-ethereum fetches it
	// per-submission; GasLimit is left at cfg.GasLimit (0 by default, i.e.
	// "auto-estimate via eth_estimateGas" — see config.Config.GasLimit's
	// doc for why that matters, Fix H-10).
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

	h := newHandlerDeps(cfg, contractAddrStr, auth, client, instance)
	h.chain = client // Fix M-09: real *ethclient.Client also satisfies chainProbe.
	return h, nil
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
// In production use NewHandler instead. The resulting Handler has no chain
// probe (Health reports chain liveness as "unknown" — see Health).
func NewHandlerWithDeps(cfg *config.Config, contractAddr string, auth *bind.TransactOpts, client bind.DeployBackend, contract EnygmaContract) *Handler {
	return newHandlerDeps(cfg, contractAddr, auth, client, contract)
}

// SetInFlight marks key as in-flight in the deduplication map.
// Exported so external test packages can simulate in-progress duplicate requests.
func (h *Handler) SetInFlight(key string, val any) { h.inFlight.Store(key, val) }

// DeleteInFlight removes key from the deduplication map.
// Exported for test cleanup after SetInFlight.
func (h *Handler) DeleteInFlight(key string) { h.inFlight.Delete(key) }

// ── Health ────────────────────────────────────────────────────────────────────

// HealthResponse is returned by GET /health.
//
// Fix M-09: previously this endpoint returned a static {"status":"ok"} no
// matter what — even with the chain unreachable or the relayer account
// out of gas, monitoring against /health alone would never fire. It now
// reports what it can about actual chain liveness, the relayer's own
// balance, and how many transactions are currently outstanding.
type HealthResponse struct {
	Status              string `json:"status"`
	ChainReachable      bool   `json:"chainReachable"`
	ChainError          string `json:"chainError,omitempty"`
	LatestBlock         uint64 `json:"latestBlock,omitempty"`
	RelayerBalanceWei   string `json:"relayerBalanceWei,omitempty"`
	PendingTransactions int64  `json:"pendingTransactions"`
}

// healthCheckTimeout bounds how long /health will wait on the chain before
// answering — a health probe must never itself hang.
const healthCheckTimeout = 3 * time.Second

// Health handles GET /health.
func (h *Handler) Health(c *gin.Context) {
	resp := HealthResponse{
		Status:              "ok",
		PendingTransactions: atomic.LoadInt64(&h.pendingTxs),
	}

	if h.chain == nil {
		// No chain probe configured (always true in tests — see
		// NewHandlerWithDeps). Still 200: the process itself is alive,
		// which is the one thing /health can vouch for here.
		c.JSON(http.StatusOK, resp)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), healthCheckTimeout)
	defer cancel()

	block, err := h.chain.BlockNumber(ctx)
	if err != nil {
		resp.ChainReachable = false
		resp.ChainError = err.Error()
		// The process is up but cannot reach the chain — still HTTP 200
		// (the endpoint itself works) but ChainReachable:false is the
		// signal monitoring should actually key off, not the status code.
		c.JSON(http.StatusOK, resp)
		return
	}
	resp.ChainReachable = true
	resp.LatestBlock = block

	if bal, err := h.chain.BalanceAt(ctx, h.auth.From, nil); err == nil {
		resp.RelayerBalanceWei = bal.String()
	}

	c.JSON(http.StatusOK, resp)
}

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
// Calls Enygma.transfer(commitmentDeltas, proof, participantIds).
// Used for confidential Enygma-to-Enygma balance updates (the enygma circuit).
// The public signal array must have exactly 81 elements (FingerPrint 6×6
// layout plus the Fix L-01 domain separator in the last slot). The domain
// separator itself is supplied by the caller (part of req.PublicSignal,
// like every other signal) — the relayer does not compute or validate it; the
// contract's own _expectedDomainId() check is what actually enforces it.
//
// Fix L-05: a short publicSignal used to be silently zero-padded up to
// 81, rather than rejected. Groth16 verification over the full 81-element
// vector happens before any state-dependent check, and the verifier's
// public-input MSM commits to every slot, so padding could never forge a
// different-but-accepted statement — but it did mean a malformed or
// truncated request was signed and broadcast anyway (auth.GasLimit != 0
// suppresses the local eth_estimateGas pre-flight — Fix H-10 — so this
// specific class of guaranteed-revert payload wasn't caught by that
// safeguard either), paying real gas for a transaction that could only
// ever revert. Requiring the exact length here is a free, local rejection
// of exactly that payload shape.
func (h *Handler) RelayTransfer(c *gin.Context) {
	var req RelayTransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	bankID := bankIDFromContext(c)

	proof8, err := parseProof8(req.Proof)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("proof: %v", err)})
		return
	}
	if len(req.PublicSignal) != 81 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("publicSignal: enygma circuit requires exactly 81 elements, got %d", len(req.PublicSignal))})
		return
	}

	var pubSig80 [81]*big.Int
	for i, s := range req.PublicSignal {
		n, err := checkFieldElement(fmt.Sprintf("publicSignal[%d]", i), s, bn254Fr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		pubSig80[i] = n
	}

	commitments, err := parseCommitments(req.Commitments)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("commitments: %v", err)})
		return
	}
	kIndex, err := parseParticipantIds(req.KIndex)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("kIndex: %v", err)})
		return
	}
	if err := checkParticipantCount(len(commitments), len(kIndex)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	transferProof := enygma.IEnygmaProof{
		Proof:        proof8,
		PublicSignal: pubSig80,
	}

	dedupKey, err := requestDedupKey("transfer", req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("dedup key: %v", err)})
		return
	}
	if _, loaded := h.inFlight.LoadOrStore(dedupKey, struct{}{}); loaded {
		c.JSON(http.StatusConflict, gin.H{"error": "duplicate transfer already in-flight"})
		return
	}
	defer h.inFlight.Delete(dedupKey)

	log.Printf("[relay] bank=%s transfer: submitting", bankID)

	// Fix H-10 (mechanism 4 of 4): txMu now guards only the submission
	// itself — the part that actually needs serializing, since auth.Nonce
	// is nil and go-ethereum resolves it per-submission from the node's
	// pending pool (concurrent submissions racing for the same "next"
	// nonce is exactly what the lock prevents). It no longer wraps
	// WaitMined, so one slow-to-mine transaction can no longer hold every
	// other bank's request queued behind it for up to txTimeout.
	h.txMu.Lock()
	tx, err := h.instance.Transfer(h.auth, commitments, transferProof, kIndex, bankID) // Fix H-09
	h.txMu.Unlock()
	if err != nil {
		log.Printf("[relay] bank=%s transfer: submit failed: %v", bankID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("transfer(): %v", err)})
		return
	}

	atomic.AddInt64(&h.pendingTxs, 1)
	defer atomic.AddInt64(&h.pendingTxs, -1)

	log.Printf("[relay] bank=%s transfer: submitted tx=%s, waiting for confirmation", bankID, tx.Hash().Hex())
	ctx, cancel := context.WithTimeout(context.Background(), txTimeout)
	defer cancel()
	receipt, err := bind.WaitMined(ctx, h.client, tx)
	if err != nil {
		log.Printf("[relay] bank=%s transfer: tx=%s wait mined failed: %v", bankID, tx.Hash().Hex(), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("wait mined: %v", err)})
		return
	}
	if receipt.Status != 1 {
		log.Printf("[relay] bank=%s transfer: tx=%s reverted on-chain", bankID, tx.Hash().Hex())
		c.JSON(http.StatusBadRequest, gin.H{"error": "transfer transaction reverted on-chain"})
		return
	}

	log.Printf("[relay] bank=%s transfer: tx=%s mined in block %d (gas used %d)",
		bankID, tx.Hash().Hex(), receipt.BlockNumber.Uint64(), receipt.GasUsed)
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
// ZK proof (enygma_fee circuit, 55-element public signal — 54 real signals
// plus the Fix L-01 domain separator in the last slot, supplied by the
// caller like every other signal).
// The fee amount is at publicSignal[50] and is visible to the relayer without
// revealing any other private information.
func (h *Handler) RelayTransferFee(c *gin.Context) {
	var req RelayTransferFeeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	bankID := bankIDFromContext(c)

	proof8, err := parseProof8(req.Proof)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("proof: %v", err)})
		return
	}
	if len(req.PublicSignal) != 55 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("publicSignal: fee circuit requires exactly 55 elements, got %d", len(req.PublicSignal))})
		return
	}

	var pubSig54 [55]*big.Int
	for i, s := range req.PublicSignal {
		n, err := checkFieldElement(fmt.Sprintf("publicSignal[%d]", i), s, bn254Fr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		pubSig54[i] = n
	}

	commitments, err := parseCommitments(req.Commitments)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("commitments: %v", err)})
		return
	}
	kIndex, err := parseParticipantIds(req.KIndex)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("kIndex: %v", err)})
		return
	}
	if err := checkParticipantCount(len(commitments), len(kIndex)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	feeProof := enygma.IEnygmaFeeProof{
		Proof:        proof8,
		PublicSignal: pubSig54,
	}

	dedupKey, err := requestDedupKey("transfer_fee", req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("dedup key: %v", err)})
		return
	}
	if _, loaded := h.inFlight.LoadOrStore(dedupKey, struct{}{}); loaded {
		c.JSON(http.StatusConflict, gin.H{"error": "duplicate fee transfer already in-flight"})
		return
	}
	defer h.inFlight.Delete(dedupKey)

	log.Printf("[relay] bank=%s transfer_fee: submitting", bankID)

	h.txMu.Lock()
	tx, err := h.instance.TransferWithFee(h.auth, commitments, feeProof, kIndex, bankID) // Fix H-09
	h.txMu.Unlock()
	if err != nil {
		log.Printf("[relay] bank=%s transfer_fee: submit failed: %v", bankID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("transferWithFee(): %v", err)})
		return
	}

	atomic.AddInt64(&h.pendingTxs, 1)
	defer atomic.AddInt64(&h.pendingTxs, -1)

	log.Printf("[relay] bank=%s transfer_fee: submitted tx=%s, waiting for confirmation", bankID, tx.Hash().Hex())
	ctx, cancel := context.WithTimeout(context.Background(), txTimeout)
	defer cancel()
	receipt, err := bind.WaitMined(ctx, h.client, tx)
	if err != nil {
		log.Printf("[relay] bank=%s transfer_fee: tx=%s wait mined failed: %v", bankID, tx.Hash().Hex(), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("wait mined: %v", err)})
		return
	}
	if receipt.Status != 1 {
		log.Printf("[relay] bank=%s transfer_fee: tx=%s reverted on-chain", bankID, tx.Hash().Hex())
		c.JSON(http.StatusBadRequest, gin.H{"error": "transferWithFee transaction reverted on-chain"})
		return
	}

	log.Printf("[relay] bank=%s transfer_fee: tx=%s mined in block %d (gas used %d)",
		bankID, tx.Hash().Hex(), receipt.BlockNumber.Uint64(), receipt.GasUsed)
	c.JSON(http.StatusOK, RelayResponse{
		TxHash:      receipt.TxHash.Hex(),
		BlockNumber: receipt.BlockNumber.Uint64(),
		GasUsed:     receipt.GasUsed,
	})
}

// ── Conversion helpers ────────────────────────────────────────────────────────

// bankIDFromContext reads the bank identifier bearerAuth attached to the
// request. Falls back to "unknown" (never expected on an authenticated
// route, but logging must never panic on a missing context value).
func bankIDFromContext(c *gin.Context) string {
	if v, ok := c.Get("bankID"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return "unknown"
}

// DedupKey exposes requestDedupKey to external test packages that need to
// pre-seed h.SetInFlight with the exact key RelayTransfer/RelayTransferFee
// will compute for a given request body, to simulate a concurrent duplicate.
func DedupKey(kind string, req any) (string, error) { return requestDedupKey(kind, req) }

// requestDedupKey hashes the entire request body (not just proof[0]) so an
// attacker cannot evade deduplication by changing one unrelated field while
// keeping proof[0] fixed, or force a false-positive 409 against an unrelated
// request that happens to share the same proof[0] value (Fix H-10, mechanism
// 4). req's JSON encoding is deterministic (a fixed struct, not a map), so
// this hash is stable across repeated marshaling of an identical request.
func requestDedupKey(kind string, req any) (string, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return kind + ":" + hex.EncodeToString(sum[:]), nil
}

// parseParticipantIds converts the request's []int64 kIndex into []*big.Int,
// rejecting negative values.
//
// Fix H-10 (mechanism 2 of 4): a negative int64 packed into a uint256 ABI
// argument does not fail — go-ethereum's ABI encoder two's-complements it
// into a huge unsigned value (e.g. -1 becomes 2^256-1), which is not what
// any caller means by a negative participant id and is never a valid
// on-chain account id (H-07 already requires ids to be registered and > 0).
// Rejecting negative values here, before any encoding happens, closes that
// silent reinterpretation.
func parseParticipantIds(ids []int64) ([]*big.Int, error) {
	out := make([]*big.Int, len(ids))
	for i, id := range ids {
		if id < 0 {
			return nil, fmt.Errorf("[%d]: negative id %d is not a valid account id", i, id)
		}
		out[i] = big.NewInt(id)
	}
	return out, nil
}

// checkParticipantCount rejects any commitments/kIndex length other than
// exactly maxParticipants (Fix H-10, mechanism 2): every Enygma transfer
// path requires exactly DEFAULT_SIZE (6) participants on-chain, so any
// other length is a guaranteed on-chain revert. Catching it here means the
// relayer never signs or broadcasts that transaction — see maxParticipants'
// doc for why that specifically matters (paced, but real, gas cost).
func checkParticipantCount(nCommitments, nKIndex int) error {
	if nCommitments != nKIndex {
		return fmt.Errorf("commitments has %d elements but kIndex has %d — they must match", nCommitments, nKIndex)
	}
	if nCommitments != maxParticipants {
		return fmt.Errorf("commitments/kIndex must contain exactly %d elements (Enygma's fixed participant count), got %d", maxParticipants, nCommitments)
	}
	return nil
}

// parseProof8 converts an 8-element decimal string array into [8]*big.Int.
// Element order from gnark: [Ax, Ay, B00, B01, B10, B11, Cx, Cy] — all
// BN254 G1/G2 coordinates, i.e. elements of the base field Fq (Fix L-05).
func parseProof8(raw [8]string) ([8]*big.Int, error) {
	var out [8]*big.Int
	for i, s := range raw {
		n, err := checkFieldElement(fmt.Sprintf("proof[%d]", i), s, bn254Fq)
		if err != nil {
			return out, err
		}
		out[i] = n
	}
	return out, nil
}

// parseCommitments converts [][]string pairs [C1x, C2y] into []IEnygmaPoint.
// These are Baby Jubjub curve point coordinates, which — Baby Jubjub being
// embedded inside BN254's scalar field — are elements of Fr, not Fq
// (Fix L-05).
func parseCommitments(raw [][]string) ([]enygma.IEnygmaPoint, error) {
	out := make([]enygma.IEnygmaPoint, len(raw))
	for i, pair := range raw {
		if len(pair) != 2 {
			return nil, fmt.Errorf("[%d]: expected [C1, C2], got %d elements", i, len(pair))
		}
		c1, err := checkFieldElement(fmt.Sprintf("commitments[%d].C1", i), pair[0], bn254Fr)
		if err != nil {
			return nil, err
		}
		c2, err := checkFieldElement(fmt.Sprintf("commitments[%d].C2", i), pair[1], bn254Fr)
		if err != nil {
			return nil, err
		}
		out[i] = enygma.IEnygmaPoint{C1: c1, C2: c2}
	}
	return out, nil
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
