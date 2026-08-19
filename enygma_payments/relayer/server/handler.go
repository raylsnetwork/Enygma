package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"sync"
	"time"

	"enygma_payments_relayer/config"
	enygma "enygma_payments_relayer/contracts"
	"enygma_payments_relayer/store"
	"enygma_payments_relayer/zkverify"

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
	Transfer(opts *bind.TransactOpts, commitmentDeltas []enygma.IEnygmaPoint, proof enygma.IEnygmaProof, participantIds []*big.Int) (*types.Transaction, error)
	TransferWithFee(opts *bind.TransactOpts, commitmentDeltas []enygma.IEnygmaPoint, proof enygma.IEnygmaFeeProof, participantIds []*big.Int) (*types.Transaction, error)

	// Simulate* dry-run the corresponding call above via eth_call against
	// current chain state — no broadcast, no gas spent by anyone. They
	// return the decoded on-chain revert error if the call would fail, nil
	// if it would succeed. Called before every broadcast: a Groth16 verify
	// that reverts costs real gas even though it changes no state, and only
	// Hardhat's dev RPC happens to reject a would-revert eth_sendTransaction
	// for free — production nodes generally mine (and charge for) it.
	SimulateTransfer(opts *bind.CallOpts, commitmentDeltas []enygma.IEnygmaPoint, proof enygma.IEnygmaProof, participantIds []*big.Int) error
	SimulateTransferWithFee(opts *bind.CallOpts, commitmentDeltas []enygma.IEnygmaPoint, proof enygma.IEnygmaFeeProof, participantIds []*big.Int) error
}

// ProofVerifier checks a Groth16 proof's validity against a public signal,
// independent of chain state — see package zkverify for the production
// implementation (native gnark verification, no network call) and its doc
// comment for how this differs from, and complements, the eth_call dry-run.
type ProofVerifier interface {
	Verify(proof [8]*big.Int, publicSignal []*big.Int) error
}

// ChainBackend is everything the relayer needs from the chain client beyond
// contract calls: waiting for receipts (bind.DeployBackend), readiness
// checks (BlockNumber, BalanceAt), and submission/resubmission support
// (SuggestGasPrice, PendingNonceAt). *ethclient.Client satisfies this
// directly; tests inject a mock.
type ChainBackend interface {
	bind.DeployBackend
	BlockNumber(ctx context.Context) (uint64, error)
	BalanceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (*big.Int, error)
	SuggestGasPrice(ctx context.Context) (*big.Int, error)
	PendingNonceAt(ctx context.Context, account common.Address) (uint64, error)
}

// defaultTxTimeout is the maximum time to wait for a transaction to be mined
// before attempting one gas-price-bumped resubmission. Overridable per
// Handler (HandlerDeps.TxTimeout) — tests use a much shorter value so a
// "stuck tx" scenario doesn't take 45+ real seconds to exercise.
const defaultTxTimeout = 45 * time.Second

// simulateTimeout is the maximum time to wait for the eth_call dry-run that
// precedes every broadcast. Short because it's a single read-only call, not
// a mined wait.
const simulateTimeout = 10 * time.Second

// readinessTimeout bounds GET /health/ready's chain calls — a slow RPC
// should make readiness fail fast, not hang the probe.
const readinessTimeout = 3 * time.Second

// gasBumpNumerator/Denominator: the minimum bump most nodes require to
// accept a same-nonce replacement transaction is 10%; 12% gives a small
// margin above that floor rather than sitting exactly on it. Because this is
// integer division, the percentage bump alone is a no-op for any gasPrice
// <= 8 wei (it rounds right back to the same value) — submitWithRetry backs
// that with an explicit "advance by at least 1 wei" floor.
const gasBumpNumerator, gasBumpDenominator = 112, 100

// submitTimeout bounds one nonce-claim-and-broadcast attempt (SuggestGasPrice
// plus the eth_sendTransaction call itself) — the part that must run under
// h.txMu, since NonceManager.release()'s safety depends on claims being
// strictly serialized. Previously this phase ran under context.Background()
// with no deadline at all, so a hung RPC node could block every other caller
// behind the lock indefinitely; now the lock is held for at most this long.
const submitTimeout = 15 * time.Second

// mineRecheckTimeout bounds the one-shot receipt check submitWithRetry makes
// right before a gas-bump resubmit, to rule out the original tx having
// actually mined in the same instant the mined-wait's deadline fired.
const mineRecheckTimeout = 5 * time.Second

// bumpGasPrice returns a gas price strictly greater than gasPrice, for a
// same-nonce replacement transaction. The 12% bump alone isn't enough at low
// values — gasPrice*112/100 is integer division, which rounds right back to
// gasPrice for any gasPrice <= 8 wei (floor(gasPrice*112/100) only exceeds
// gasPrice once gasPrice >= 9) — so this falls back to gasPrice+1 whenever
// the percentage bump would be a no-op, guaranteeing forward progress
// regardless of how low gasPrice is (plausible on a near-zero-gas
// permissioned chain).
func bumpGasPrice(gasPrice *big.Int) *big.Int {
	bumped := new(big.Int).Mul(gasPrice, big.NewInt(gasBumpNumerator))
	bumped.Div(bumped, big.NewInt(gasBumpDenominator))
	if bumped.Cmp(gasPrice) <= 0 {
		return new(big.Int).Add(gasPrice, big.NewInt(1))
	}
	return bumped
}

// Exact public-signal counts the two circuits declare — must match the
// verifying keys loaded from cfg.VKTransferPath / cfg.VKFeePath.
const (
	transferNbPublic = 80 // enygma (FingerPrint) circuit
	feeNbPublic      = 54 // enygma_fee circuit
)

// Handler holds all dependencies for the relay endpoints.
type Handler struct {
	cfg          *config.Config
	contractAddr string
	auth         *bind.TransactOpts // relayer's signing key — never leaves this process
	client       ChainBackend       // chain reads, receipt waits, gas/nonce lookups; tests inject a mock
	instance     EnygmaContract     // Enygma contract binding; tests inject a mock
	vTransfer    ProofVerifier      // local Groth16 verify for the enygma (transfer) circuit
	vFee         ProofVerifier      // local Groth16 verify for the enygma_fee circuit
	store        *store.Store       // persistent idempotency state — survives a crash mid-flight
	nonces       *NonceManager      // local next-nonce tracking with a release path on unused claims
	txTimeout    time.Duration      // per-attempt mined-wait budget before a gas-bump retry
	txMu         sync.Mutex         // serializes on-chain submissions to prevent nonce races
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

	// Build a transactor. Nonce and GasPrice are set explicitly per
	// submission (see submitWithRetry) rather than left for go-ethereum to
	// fetch automatically — that's what makes a same-nonce gas-bumped
	// resubmission possible. GasLimit comes from config.
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

	// Load Groth16 verifying keys for local (in-process, no-network) proof
	// verification. Fails closed at startup — same posture as a bad ABI path
	// or an unparsable private key: better to refuse to start than to run
	// with the local check silently absent.
	vTransfer, err := zkverify.Load(cfg.VKTransferPath, transferNbPublic)
	if err != nil {
		return nil, fmt.Errorf("load transfer verifying key: %w", err)
	}
	vFee, err := zkverify.Load(cfg.VKFeePath, feeNbPublic)
	if err != nil {
		return nil, fmt.Errorf("load fee verifying key: %w", err)
	}

	// Open the persistent idempotency store. Fails closed — a relayer that
	// can't durably remember what it's already submitted shouldn't start.
	st, err := store.Open(cfg.StorePath)
	if err != nil {
		return nil, fmt.Errorf("open idempotency store: %w", err)
	}

	nonceCtx, nonceCancel := context.WithTimeout(context.Background(), readinessTimeout)
	defer nonceCancel()
	nonces, err := newNonceManager(nonceCtx, client, auth.From)
	if err != nil {
		st.Close()
		return nil, fmt.Errorf("init nonce manager: %w", err)
	}

	return newHandlerDeps(HandlerDeps{
		Cfg:          cfg,
		ContractAddr: contractAddrStr,
		Auth:         auth,
		Client:       client,
		Instance:     instance,
		VTransfer:    vTransfer,
		VFee:         vFee,
		Store:        st,
		Nonces:       nonces,
	}), nil
}

// HandlerDeps bundles everything NewHandlerWithDeps needs. Exported as a
// struct (rather than a long positional parameter list) so external test
// packages can set only what a given test cares about; unset TxTimeout gets
// defaultTxTimeout.
type HandlerDeps struct {
	Cfg          *config.Config
	ContractAddr string
	Auth         *bind.TransactOpts
	Client       ChainBackend
	Instance     EnygmaContract
	VTransfer    ProofVerifier
	VFee         ProofVerifier
	Store        *store.Store
	Nonces       *NonceManager
	TxTimeout    time.Duration
}

// newHandlerDeps constructs a Handler from pre-built dependencies (internal).
func newHandlerDeps(d HandlerDeps) *Handler {
	txTimeout := d.TxTimeout
	if txTimeout == 0 {
		txTimeout = defaultTxTimeout
	}
	// Defensive defaults for external callers of the exported
	// NewHandlerWithDeps that build a config.Config by hand instead of
	// going through config.Load() (which always populates these fields) —
	// Handler code calls MinFee.Sign()/MinBalanceWei.Sign() unconditionally,
	// so a nil here would panic on the first request instead of just
	// meaning "no minimum".
	if d.Cfg != nil {
		if d.Cfg.MinFee == nil {
			d.Cfg.MinFee = big.NewInt(0)
		}
		if d.Cfg.MinBalanceWei == nil {
			d.Cfg.MinBalanceWei = big.NewInt(0)
		}
	}
	return &Handler{
		cfg:          d.Cfg,
		contractAddr: d.ContractAddr,
		auth:         d.Auth,
		client:       d.Client,
		instance:     d.Instance,
		vTransfer:    d.VTransfer,
		vFee:         d.VFee,
		store:        d.Store,
		nonces:       d.Nonces,
		txTimeout:    txTimeout,
	}
}

// NewHandlerWithDeps constructs a Handler with pre-built dependencies.
// Exported for external test packages that inject mock contracts, backends,
// verifiers, and a store. In production use NewHandler instead.
func NewHandlerWithDeps(d HandlerDeps) *Handler {
	return newHandlerDeps(d)
}

// Close releases the handler's persistent store. main.go does not currently
// call this on shutdown (there's no signal-handling / graceful-shutdown
// path yet) — bbolt fsyncs each write regardless, so data isn't lost, but
// the file lock isn't released cleanly on a hard exit. Tests should call
// this in cleanup to avoid leaking file handles across a run.
func (h *Handler) Close() error {
	return h.store.Close()
}

// ── Info ──────────────────────────────────────────────────────────────────────

// Info handles GET /relay/info.
func (h *Handler) Info(c *gin.Context) {
	c.JSON(http.StatusOK, InfoResponse{
		RelayerAddr:  h.auth.From.Hex(),
		ContractAddr: h.contractAddr,
		ChainID:      h.cfg.ChainID.Int64(),
		MinFee:       h.cfg.MinFee.String(),
	})
}

// ── Ready ─────────────────────────────────────────────────────────────────────

// Ready handles GET /health/ready.
//
// Unlike /health (a pure liveness probe — deliberately dependency-free, so
// an RPC hiccup can't make an orchestrator think the process itself is dead
// and restart it for no reason), this checks whether the relayer can
// actually do its job right now: reach the chain, and — if
// RELAYER_MIN_BALANCE_WEI is set — still have enough gas budget. Returns
// 503 with per-check detail if not, so a load balancer can stop routing
// traffic here without anyone needing to read logs to find out why.
func (h *Handler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), readinessTimeout)
	defer cancel()

	ready := true
	checks := gin.H{}

	blockNumber, err := h.client.BlockNumber(ctx)
	if err != nil {
		ready = false
		checks["rpc"] = gin.H{"ok": false, "error": err.Error()}
	} else {
		checks["rpc"] = gin.H{"ok": true, "blockNumber": blockNumber}
	}

	balance, err := h.client.BalanceAt(ctx, h.auth.From, nil)
	switch {
	case err != nil:
		ready = false
		checks["balance"] = gin.H{"ok": false, "error": err.Error()}
	case h.cfg.MinBalanceWei.Sign() > 0 && balance.Cmp(h.cfg.MinBalanceWei) < 0:
		ready = false
		checks["balance"] = gin.H{"ok": false, "wei": balance.String(), "minRequired": h.cfg.MinBalanceWei.String()}
	default:
		checks["balance"] = gin.H{"ok": true, "wei": balance.String()}
	}

	status := http.StatusOK
	if !ready {
		status = http.StatusServiceUnavailable
	}
	c.JSON(status, gin.H{"ready": ready, "checks": checks})
}

// ── Transfer ──────────────────────────────────────────────────────────────────

// RelayTransfer handles POST /relay/transfer.
//
// Calls Enygma.transfer(commitmentDeltas, proof, participantIds).
// Used for confidential Enygma-to-Enygma balance updates (the enygma circuit).
// The public signal array supports up to 80 elements (FingerPrint 6×6 layout);
// unused slots are zero-padded to fill the fixed [80]*big.Int expected by the contract.
// Dry-runs via eth_call before broadcasting — a proof that would revert is
// rejected here at no gas cost instead of being discovered from a mined receipt.
// Deduplication is durable across restarts (see package store); a request
// identical to one already mined gets the cached result replayed rather
// than resubmitted or rejected.
func (h *Handler) RelayTransfer(c *gin.Context) {
	var req RelayTransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Dedup check first: a cheap, in-process bbolt lookup that rejects an
	// exact duplicate/replay before paying for a pairing verification and an
	// RPC eth_call dry-run on every request, including ones about to be
	// rejected anyway.
	dedupKey := "transfer:" + req.Proof[0]
	blocked, cached, err := h.store.TryBeginPending(dedupKey, h.cfg.DedupStaleness)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("dedup store: %v", err)})
		return
	}
	if blocked {
		if cached != nil {
			c.JSON(http.StatusOK, RelayResponse{TxHash: cached.TxHash, BlockNumber: cached.BlockNumber, GasUsed: cached.GasUsed})
			return
		}
		c.JSON(http.StatusConflict, gin.H{"error": "duplicate transfer already in-flight"})
		return
	}

	// From here on we own dedupKey's Pending claim and must finalize it one
	// way or another — even on a panic below (e.g. a nil tx from a
	// misbehaving binding), which gin.Recovery() converts to a 500 but would
	// otherwise skip the markMined/markFailed calls entirely, leaving the
	// key stuck Pending for the full dedup staleness window instead of free
	// for an immediate retry. Mirrors the guarantee the old in-memory
	// sync.Map cleanup gave via `defer h.inFlight.Delete(dedupKey)`.
	finalized := false
	defer func() {
		if !finalized {
			h.markFailed(dedupKey)
		}
	}()

	proof8, err := parseProof8(req.Proof)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("proof: %v", err)})
		return
	}
	if len(req.PublicSignal) > 80 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("publicSignal: %d elements exceeds maximum of 80", len(req.PublicSignal))})
		return
	}

	// Zero-pad to [80]*big.Int; the circuit only uses the first N slots.
	var pubSig80 [80]*big.Int
	for i := range pubSig80 {
		pubSig80[i] = big.NewInt(0)
	}
	for i, s := range req.PublicSignal {
		n, ok := new(big.Int).SetString(s, 10)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("publicSignal[%d]: invalid decimal %q", i, s)})
			return
		}
		pubSig80[i] = n
	}

	// Local Groth16 verify: no network call, milliseconds of pairing
	// arithmetic. Catches a malformed/forged proof before the request ever
	// touches the RPC node — the dry-run below separately catches a
	// genuinely valid proof that's stale against current chain state.
	if err := h.vTransfer.Verify(proof8, pubSig80[:]); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid zk proof (local verification failed): %v", err)})
		return
	}

	commitments, err := parseCommitments(req.Commitments)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("commitments: %v", err)})
		return
	}
	kIndex := int64sToBI(req.KIndex)

	transferProof := enygma.IEnygmaProof{
		Proof:        proof8,
		PublicSignal: pubSig80,
	}

	// Dry-run before touching dedup/mutex/chain state: a proof that would
	// revert has already burned nothing here, versus real gas if broadcast.
	// Pending:true evaluates against the relayer's own still-unmined
	// broadcasts, not just the latest mined block — otherwise a transfer
	// whose proof assumes an earlier, already-broadcast-but-not-yet-mined
	// transfer already applied would be dry-run against stale pre-transfer
	// state and rejected even though it would succeed once mined in order.
	simCtx, simCancel := context.WithTimeout(context.Background(), simulateTimeout)
	simErr := h.instance.SimulateTransfer(&bind.CallOpts{From: h.auth.From, Context: simCtx, Pending: true}, commitments, transferProof, kIndex)
	simCancel()
	if simErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("transfer would revert on-chain (dry-run, no gas spent): %v", simErr)})
		return
	}

	receipt, err := h.submitWithRetry(context.Background(), func(auth *bind.TransactOpts) (*types.Transaction, error) {
		return h.instance.Transfer(auth, commitments, transferProof, kIndex)
	})
	if err != nil {
		h.finalizeSubmitError(dedupKey, err)
		finalized = true
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("transfer(): %v", err)})
		return
	}
	if receipt.Status != 1 {
		h.markFailed(dedupKey)
		finalized = true
		c.JSON(http.StatusBadRequest, gin.H{"error": "transfer transaction reverted on-chain"})
		return
	}

	h.markMined(dedupKey, receipt)
	finalized = true
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
// ZK proof (enygma_fee circuit, 54-element public signal).
// The fee amount is at publicSignal[50] and is visible to the relayer without
// revealing any other private information; requests below cfg.MinFee are
// rejected with 402 before any chain interaction.
// Dry-runs via eth_call before broadcasting, and dedup is durable across
// restarts — see RelayTransfer.
func (h *Handler) RelayTransferFee(c *gin.Context) {
	var req RelayTransferFeeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Dedup check first — see RelayTransfer. Cheapest check available, so it
	// runs before local verify, the fee check, and the dry-run.
	dedupKey := "transfer_fee:" + req.Proof[0]
	blocked, cached, err := h.store.TryBeginPending(dedupKey, h.cfg.DedupStaleness)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("dedup store: %v", err)})
		return
	}
	if blocked {
		if cached != nil {
			c.JSON(http.StatusOK, RelayResponse{TxHash: cached.TxHash, BlockNumber: cached.BlockNumber, GasUsed: cached.GasUsed})
			return
		}
		c.JSON(http.StatusConflict, gin.H{"error": "duplicate fee transfer already in-flight"})
		return
	}

	// See RelayTransfer — guarantees dedupKey is freed even on a panic below.
	finalized := false
	defer func() {
		if !finalized {
			h.markFailed(dedupKey)
		}
	}()

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

	// Local Groth16 verify — see RelayTransfer. Runs before the fee check
	// below: no point pricing a proof that isn't even valid.
	if err := h.vFee.Verify(proof8, pubSig54[:]); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid zk proof (local verification failed): %v", err)})
		return
	}

	// Enforce the relayer's minimum fee before touching dedup/mutex/chain state.
	// publicSignal[50] is the enygma_fee circuit's public Fee slot — the amount
	// the sender's proof commits to paying. A disabled minimum (0) skips the check.
	fee := pubSig54[50]
	if h.cfg.MinFee.Sign() > 0 && fee.Cmp(h.cfg.MinFee) < 0 {
		c.JSON(http.StatusPaymentRequired, gin.H{
			"error": fmt.Sprintf("fee %s below relayer minimum %s", fee, h.cfg.MinFee),
		})
		return
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

	// Dry-run before touching dedup/mutex/chain state — see SimulateTransfer.
	simCtx, simCancel := context.WithTimeout(context.Background(), simulateTimeout)
	simErr := h.instance.SimulateTransferWithFee(&bind.CallOpts{From: h.auth.From, Context: simCtx, Pending: true}, commitments, feeProof, kIndex)
	simCancel()
	if simErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("transferWithFee would revert on-chain (dry-run, no gas spent): %v", simErr)})
		return
	}

	receipt, err := h.submitWithRetry(context.Background(), func(auth *bind.TransactOpts) (*types.Transaction, error) {
		return h.instance.TransferWithFee(auth, commitments, feeProof, kIndex)
	})
	if err != nil {
		h.finalizeSubmitError(dedupKey, err)
		finalized = true
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("transferWithFee(): %v", err)})
		return
	}
	if receipt.Status != 1 {
		h.markFailed(dedupKey)
		finalized = true
		c.JSON(http.StatusBadRequest, gin.H{"error": "transferWithFee transaction reverted on-chain"})
		return
	}

	h.markMined(dedupKey, receipt)
	finalized = true
	c.JSON(http.StatusOK, RelayResponse{
		TxHash:      receipt.TxHash.Hex(),
		BlockNumber: receipt.BlockNumber.Uint64(),
		GasUsed:     receipt.GasUsed,
	})
}

// ── Submission (nonce + gas-bump retry) ───────────────────────────────────────

// errAmbiguousOutcome marks a submitWithRetry error where the underlying
// transaction may actually have been broadcast (or even mined) despite the
// error — an RPC ack-loss, a mined-wait that failed for a reason other than
// a clean timeout, or a gas-bump resubmit whose own send failed after the
// original tx was already live. Callers must not treat this the same as a
// definite failure (a dry-run rejection or a clean on-chain revert): marking
// the dedup key outright failed and freeing it for an immediate identical
// retry risks a double-submission, or a confusing rejection against a
// nullifier the ambiguous attempt may have already consumed. Left as a
// Pending record instead, so the existing staleness window governs recovery.
var errAmbiguousOutcome = errors.New("submission outcome ambiguous — transaction may have been broadcast")

// recheckMined does one bounded, one-shot receipt lookup for tx — used right
// before deciding a mined-wait's deadline means "stuck" and something needs
// resubmitting. Catches the tx having actually mined in the same instant the
// wait's deadline fired, so a transfer that already succeeded on-chain isn't
// reported as failed and resubmitted as a conflicting same-nonce replacement.
func (h *Handler) recheckMined(tx *types.Transaction) *types.Receipt {
	ctx, cancel := context.WithTimeout(context.Background(), mineRecheckTimeout)
	defer cancel()
	receipt, err := h.client.TransactionReceipt(ctx, tx.Hash())
	if err != nil || receipt == nil {
		return nil
	}
	return receipt
}

// submitWithRetry claims a nonce and broadcasts via doCall (both under
// h.txMu, bounded by submitTimeout — see claimAndBroadcast), then waits up
// to h.txTimeout for it to be mined with the lock released, so a slow mined-
// wait never blocks other callers from claiming their own nonces and
// submitting. If that wait times out — the tx is stuck, most likely
// underpriced for current chain conditions — it resubmits the SAME nonce
// once with a bumped gas price and waits again. This is a single retry, not
// an unbounded loop: the goal is recovering from a transient gas spike, not
// fighting a chain that's fundamentally stuck (that needs an operator, not a
// retry loop).
func (h *Handler) submitWithRetry(ctx context.Context, doCall func(auth *bind.TransactOpts) (*types.Transaction, error)) (*types.Receipt, error) {
	tx, gasPrice, nonce, err := h.claimAndBroadcast(ctx, doCall)
	if err != nil {
		return nil, err
	}

	receipt, waitErr := h.waitMined(ctx, tx)
	if waitErr == nil {
		return receipt, nil
	}
	if !errors.Is(waitErr, context.DeadlineExceeded) {
		// tx was successfully broadcast — a wait failure that isn't a clean
		// deadline (an RPC error while polling, say) leaves its actual
		// on-chain outcome unknown, not definitely failed.
		return nil, fmt.Errorf("wait mined (tx %s): %w: %w", tx.Hash().Hex(), waitErr, errAmbiguousOutcome)
	}

	// The wait context's deadline may have fired in the same instant the
	// node actually mined tx — check once more before treating it as stuck,
	// so a transfer that already succeeded on-chain isn't reported as a
	// failure and resubmitted as a conflicting same-nonce replacement.
	if r := h.recheckMined(tx); r != nil {
		log.Printf("relayer: tx %s was mined right at the wait deadline; skipping gas-bump resubmit", tx.Hash().Hex())
		return r, nil
	}

	log.Printf("relayer: tx %s not mined within %s, resubmitting nonce %d with bumped gas price", tx.Hash().Hex(), h.txTimeout, nonce)

	bumped := bumpGasPrice(gasPrice)

	tx2, err := h.broadcast(ctx, doCall, nonce, bumped)
	if err != nil {
		// We no longer know where the chain's nonce state stands: the
		// original tx may or may not still be pending, and this replacement
		// never broadcast either. Re-sync from the chain rather than let
		// every future submission keep signing a nonce the chain may never
		// accept in order — that would otherwise stall the relayer until an
		// operator restarts it.
		if resyncErr := h.resyncNonce(ctx); resyncErr != nil {
			log.Printf("relayer: nonce resync after failed gas-bump resubmit also failed: %v", resyncErr)
		}
		return nil, fmt.Errorf("gas-bump resubmit failed (original tx %s may still be pending): %w: %w", tx.Hash().Hex(), err, errAmbiguousOutcome)
	}
	receipt, waitErr = h.waitMined(ctx, tx2)
	if waitErr != nil {
		// Same deadline-vs-mined race as the first wait, for the
		// gas-bumped replacement.
		if r := h.recheckMined(tx2); r != nil {
			log.Printf("relayer: gas-bumped tx %s was mined right at the wait deadline", tx2.Hash().Hex())
			return r, nil
		}
		return nil, fmt.Errorf("stuck after gas-bump retry (tx %s): %w: %w", tx2.Hash().Hex(), waitErr, errAmbiguousOutcome)
	}
	return receipt, nil
}

// resyncNonce re-fetches the next nonce from the chain under h.txMu.
// NonceManager.resync itself only locks its own internal counter, not
// h.txMu — calling it directly here (outside the lock claimAndBroadcast and
// broadcast hold for their own claim+send) would let its PendingNonceAt read
// race a concurrent claimAndBroadcast: if a concurrent claim lands and
// broadcasts a higher nonce while this read is in flight, the stale read
// completing afterward would rewind nm.next back below it, and the next
// take() would hand out a nonce that's already in use. Holding h.txMu here
// closes that window, restoring the "all claims are serialized" invariant
// NonceManager's own doc comment assumes.
func (h *Handler) resyncNonce(ctx context.Context) error {
	h.txMu.Lock()
	defer h.txMu.Unlock()
	return h.nonces.resync(ctx, h.client, h.auth.From)
}

// claimAndBroadcast claims the next nonce and submits doCall — the only part
// that must be serialized (NonceManager.release()'s safety depends on claims
// being strictly ordered) and so the only part that briefly blocks other
// callers. Bounded by submitTimeout so one hung RPC call can't hold h.txMu —
// and therefore every other caller — forever.
func (h *Handler) claimAndBroadcast(ctx context.Context, doCall func(auth *bind.TransactOpts) (*types.Transaction, error)) (tx *types.Transaction, gasPrice *big.Int, nonce uint64, err error) {
	h.txMu.Lock()
	defer h.txMu.Unlock()

	subCtx, cancel := context.WithTimeout(ctx, submitTimeout)
	defer cancel()

	gasPrice, err = h.client.SuggestGasPrice(subCtx)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("suggest gas price: %w", err)
	}

	nonce = h.nonces.take()
	auth := *h.auth
	auth.Nonce = new(big.Int).SetUint64(nonce)
	auth.GasPrice = gasPrice
	auth.Context = subCtx

	tx, err = doCall(&auth)
	if err != nil {
		// A doCall error doesn't guarantee the node never received or
		// accepted the transaction — it could equally be an ack-loss (the
		// RPC response was lost, e.g. to a timeout or connection reset,
		// after the node already accepted the tx into its mempool).
		// Releasing the nonce unconditionally here would risk handing it
		// out again to a different transaction, colliding with one that's
		// actually live. Confirm the chain still agrees the nonce is
		// unused first; a stale local counter is recoverable (resync, or
		// an operator restart) — a nonce collision on-chain is not.
		if pending, pErr := h.client.PendingNonceAt(subCtx, h.auth.From); pErr != nil {
			log.Printf("relayer: could not verify nonce %d is unused before releasing (pending-nonce check failed: %v) — not releasing", nonce, pErr)
			return nil, nil, 0, fmt.Errorf("%w: %w", err, errAmbiguousOutcome)
		} else if pending > nonce {
			log.Printf("relayer: doCall for nonce %d errored, but the chain already shows next-pending nonce %d — not releasing (possible ack-loss, not a definite non-send)", nonce, pending)
			return nil, nil, 0, fmt.Errorf("%w: %w", err, errAmbiguousOutcome)
		}
		h.nonces.release(nonce)
		return nil, nil, 0, err
	}
	return tx, gasPrice, nonce, nil
}

// broadcast resubmits doCall with an already-claimed nonce and a new gas
// price — the gas-bump retry path. No new nonce claim, so no release path is
// needed on failure here; see submitWithRetry's resync call instead. Still
// serialized by h.txMu and bounded by submitTimeout for the same reason as
// claimAndBroadcast.
func (h *Handler) broadcast(ctx context.Context, doCall func(auth *bind.TransactOpts) (*types.Transaction, error), nonce uint64, gasPrice *big.Int) (*types.Transaction, error) {
	h.txMu.Lock()
	defer h.txMu.Unlock()

	subCtx, cancel := context.WithTimeout(ctx, submitTimeout)
	defer cancel()

	auth := *h.auth
	auth.Nonce = new(big.Int).SetUint64(nonce)
	auth.GasPrice = gasPrice
	auth.Context = subCtx

	return doCall(&auth)
}

// waitMined wraps bind.WaitMined with h.txTimeout. On timeout it returns
// context.DeadlineExceeded (bind.WaitMined already propagates ctx.Err()
// directly), which submitWithRetry uses to decide whether a gas-bump retry
// is worth attempting versus surfacing a genuine RPC error immediately.
func (h *Handler) waitMined(ctx context.Context, tx *types.Transaction) (*types.Receipt, error) {
	waitCtx, cancel := context.WithTimeout(ctx, h.txTimeout)
	defer cancel()
	return bind.WaitMined(waitCtx, h.client, tx)
}

// markMined and markFailed record a terminal outcome in the idempotency
// store. A store write failure is logged, not surfaced to the caller — the
// on-chain outcome already happened and is what the response reflects;
// losing this write only means a future identical retry might not replay
// cleanly (harmless: the nullifier is already consumed on-chain either way).
func (h *Handler) markMined(dedupKey string, receipt *types.Receipt) {
	if err := h.store.MarkMined(dedupKey, receipt.TxHash.Hex(), receipt.BlockNumber.Uint64(), receipt.GasUsed); err != nil {
		log.Printf("relayer: mark mined (key=%s): %v", dedupKey, err)
	}
}

func (h *Handler) markFailed(dedupKey string) {
	if err := h.store.MarkFailed(dedupKey); err != nil {
		log.Printf("relayer: mark failed (key=%s): %v", dedupKey, err)
	}
}

// finalizeSubmitError decides how to leave dedupKey's record after a
// submitWithRetry error. A definite failure (err doesn't wrap
// errAmbiguousOutcome — e.g. the nonce was cleanly released before any
// broadcast) is safe to mark failed: store.MarkFailed deletes the record
// outright, freeing the key for an immediate retry with no risk of
// collision. An ambiguous outcome is left untouched (still Pending) instead
// — we don't know whether the transaction actually broadcast or mined, so
// deleting the record and allowing a blind immediate retry risks a
// double-submission or a confusing rejection against a nullifier the
// ambiguous attempt may have already consumed. The existing staleness
// window governs recovery for that case instead.
func (h *Handler) finalizeSubmitError(dedupKey string, err error) {
	if errors.Is(err, errAmbiguousOutcome) {
		log.Printf("relayer: leaving dedup key %q as Pending — submission outcome ambiguous, not safe to mark failed for an immediate retry", dedupKey)
		return
	}
	h.markFailed(dedupKey)
}

// ── Conversion helpers ────────────────────────────────────────────────────────

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
