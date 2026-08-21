package server

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"enygma_relayer/config"
	"enygma_relayer/store"
	"enygma_relayer/zkverify"

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

// ProofVerifier checks a Groth16 proof's validity against a public signal,
// independent of chain state — see package zkverify for the production
// implementation (native gnark verification, no network call) and its doc
// comment for how this differs from, and complements, the eth_call dry-run.
type ProofVerifier interface {
	Verify(proof [8]*big.Int, publicSignal []*big.Int) error
}

const (
	// simulateTimeout is the maximum time to wait for the eth_call dry-run
	// that precedes every broadcast. Short because it's a single read-only
	// call, not a mined wait.
	simulateTimeout = 10 * time.Second

	// readinessTimeout bounds GET /health/ready's chain calls, and the
	// nonce manager's one-shot startup PendingNonceAt fetch — a slow RPC
	// should make readiness fail fast, not hang the probe or startup.
	readinessTimeout = 3 * time.Second

	// defaultTxTimeout is the maximum time to wait for a transaction to be
	// mined before attempting one gas-price-bumped resubmission.
	defaultTxTimeout = 45 * time.Second

	// submitTimeout bounds one nonce-claim-and-broadcast attempt
	// (SuggestGasPrice plus the eth_sendTransaction call itself) — the part
	// that must run under h.txMu, since NonceManager.release()'s safety
	// depends on claims being strictly serialized. Without a bound here, a
	// hung RPC node could block every other caller behind the lock
	// indefinitely.
	submitTimeout = 15 * time.Second

	// mineRecheckTimeout bounds the one-shot receipt check submitWithRetry
	// makes right before a gas-bump resubmit, to rule out the original tx
	// having actually mined in the same instant the mined-wait's deadline
	// fired.
	mineRecheckTimeout = 5 * time.Second

	// gasBumpNumerator/Denominator: the minimum bump most nodes require to
	// accept a same-nonce replacement transaction is 10%; 12% gives a small
	// margin above that floor rather than sitting exactly on it. Because
	// this is integer division, the percentage bump alone is a no-op for
	// any gasPrice <= 8 wei (it rounds right back to the same value) —
	// bumpGasPrice backs that with an explicit "advance by at least 1 wei"
	// floor.
	gasBumpNumerator, gasBumpDenominator = 112, 100
)

// relayerFeeNbPublic and paymentNbPublic are the exact public-signal counts
// their respective circuits declare — must match the verifying keys loaded
// from cfg.VKRelayerFeePath / cfg.VKPaymentPath.
const (
	relayerFeeNbPublic = 9
	paymentNbPublic    = 7
)

// ── handler ──────────────────────────────────────────────────────────────────

// Handler holds all dependencies for the relay endpoints.
type Handler struct {
	cfg                    *config.Config
	dvpABI                 abi.ABI
	vaultABI               abi.ABI
	tagRegistryABI         abi.ABI
	tagChannelRegistryABI  abi.ABI
	dvpAddr                common.Address
	vaultAddr              common.Address
	tagRegistryAddr        common.Address
	tagChannelRegistryAddr common.Address
	auth                   *bind.TransactOpts
	client                 *ethclient.Client
	vPayment               ProofVerifier // local Groth16 verify for the plain Payment circuit
	vRelayerFee            ProofVerifier // local Groth16 verify for the PaymentRelayerFeePublic circuit
	store                  *store.Store  // persistent idempotency state — survives a crash mid-flight
	nonces                 *NonceManager // local next-nonce tracking with a release path on unused claims
	txTimeout              time.Duration // per-attempt mined-wait budget before a gas-bump retry
	txMu                   sync.Mutex    // serializes on-chain submissions — prevents nonce races
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

	// Load the Groth16 verifying keys for local, in-process verification —
	// no network call — of both circuits the relayer accepts proofs for.
	// Fails closed at startup — same posture as a bad ABI path or an
	// unparsable private key: better to refuse to start than to run with a
	// local check silently absent.
	vPayment, err := zkverify.Load(cfg.VKPaymentPath, paymentNbPublic)
	if err != nil {
		return nil, fmt.Errorf("load payment verifying key: %w", err)
	}
	vRelayerFee, err := zkverify.Load(cfg.VKRelayerFeePath, relayerFeeNbPublic)
	if err != nil {
		return nil, fmt.Errorf("load relayer-fee verifying key: %w", err)
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

	return &Handler{
		cfg:                    cfg,
		dvpABI:                 dvpABI,
		vaultABI:               vaultABI,
		tagRegistryABI:         tagRegistryABI,
		tagChannelRegistryABI:  tagChannelRegistryABI,
		dvpAddr:                common.HexToAddress(dvpAddrStr),
		vaultAddr:              common.HexToAddress(vaultAddrStr),
		tagRegistryAddr:        common.HexToAddress(tagRegistryAddrStr),
		tagChannelRegistryAddr: common.HexToAddress(tagChannelRegistryAddrStr),
		auth:                   auth,
		client:                 client,
		vPayment:               vPayment,
		vRelayerFee:            vRelayerFee,
		store:                  st,
		nonces:                 nonces,
		txTimeout:              defaultTxTimeout,
	}, nil
}

// Close releases the handler's persistent store. main.go calls this on
// shutdown via defer — bbolt fsyncs each write regardless, so data isn't
// lost even on a hard exit, but this releases the file lock cleanly instead
// of leaving it held until the OS reclaims it.
func (h *Handler) Close() error {
	return h.store.Close()
}

// ── Info ─────────────────────────────────────────────────────────────────────

// Info is the gin handler for GET /relay/info.
// Returns the relayer's configured contract addresses and chain ID.
// This endpoint is public (no auth) so clients can discover addresses
// without needing them pre-configured out-of-band.
func (h *Handler) Info(c *gin.Context) {
	c.JSON(http.StatusOK, InfoResponse{
		RelayerAddr:            h.auth.From.Hex(),
		TagRegistryAddr:        h.tagRegistryAddr.Hex(),
		TagChannelRegistryAddr: h.tagChannelRegistryAddr.Hex(),
		ChainID:                h.cfg.ChainID.Int64(),
		MinFee:                 h.cfg.MinFee.String(),
	})
}

// ── Ready ────────────────────────────────────────────────────────────────────

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

// ── Payment ──────────────────────────────────────────────────────────────────

// RelayPayment is the gin handler for POST /relay/payment.
//
// Validation steps:
//  1. Parse and decode all fields from the request body.
//  2. Dedup check (durable, survives a restart).
//  3. Local Groth16 verify against the Payment verifying key — no network
//     call, rejects a forged/malformed proof in ~1ms.
//  4. Check the Merkle root is known to the vault (rootHistory).
//  5. Check the nullifier has not been spent (nullifiers mapping).
//  6. Dry-run payment() via eth_call — no gas spent if it would revert.
//  7. Sign and submit to EnygmaDvp.payment(), with a gas-bumped retry on a
//     stuck tx.
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

	// Public signal layout: [msg, treeNum0, root0, nullifier0, cmt0, cmt1]
	treeNum := p.publicSignal[1]
	root := p.publicSignal[2]
	nullifier := p.publicSignal[3]

	// Step 2 — dedup check: a cheap, durable bbolt lookup that rejects an
	// exact duplicate/replay before paying for two RPC reads, a dry-run,
	// and a broadcast.
	//
	// Keyed as "nullifier:" — deliberately shared with RelayPaymentFee's
	// dedup key for the same (treeNum, nullifier), not a "payment:"-only
	// prefix. The same input note can be proven against either the plain
	// Payment circuit or the PaymentRelayerFeePublic circuit, producing the
	// same nullifier either way — an endpoint-specific prefix would let the
	// same nullifier be claimed as "free" on both endpoints simultaneously,
	// letting both dry-run successfully (neither has broadcast yet) and
	// both reach submitWithRetry, so one mines and the other burns gas
	// reverting on-chain against an already-spent nullifier. Sharing the
	// dedup namespace closes that gap: whichever endpoint claims the key
	// first blocks the other exactly like a same-endpoint retry would.
	dedupKey := "nullifier:" + treeNum.String() + ":" + nullifier.String()
	blocked, cached, err := h.store.TryBeginPending(dedupKey, h.cfg.DedupStaleness)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("dedup store: %v", err)})
		return
	}
	if blocked {
		if cached != nil {
			c.JSON(http.StatusOK, RelayPaymentResponse{TxHash: cached.TxHash, BlockNumber: cached.BlockNumber, GasUsed: cached.GasUsed})
			return
		}
		c.JSON(http.StatusConflict, gin.H{"error": "nullifier already in-flight"})
		return
	}

	// From here on we own dedupKey's Pending claim and must finalize it one
	// way or another — even on a panic below (e.g. a nil tx from a
	// misbehaving binding), which gin.Recovery() converts to a 500 but
	// would otherwise skip the markMined/markFailed calls entirely, leaving
	// the key stuck Pending for the full dedup staleness window instead of
	// free for an immediate retry.
	finalized := false
	defer func() {
		if !finalized {
			h.markFailed(dedupKey)
		}
	}()

	// Step 3 — local Groth16 verify, before anything else touches the
	// network: no point pricing an RPC round-trip on a proof that isn't
	// even valid.
	if err := h.vPayment.Verify(p.proof, p.publicSignal[:]); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid zk proof (local verification failed): %v", err)})
		return
	}

	vault := bind.NewBoundContract(h.vaultAddr, h.vaultABI, h.client, h.client, h.client)

	// Step 4 — Merkle root must be in the vault's rootHistory.
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

	// Step 5 — nullifier must not already be spent.
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

	receipt := buildProofReceipt(p)

	// Step 6 — dry-run before touching the mutex/chain state: a proof that
	// would revert has burned nothing here, versus real gas if broadcast.
	simCtx, simCancel := context.WithTimeout(context.Background(), simulateTimeout)
	simErr := h.simulateDvpCall(simCtx, "payment", receipt, p.vaultId, p.cipherText, p.encTxData)
	simCancel()
	if simErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("payment would revert on-chain (dry-run, no gas spent): %v", simErr)})
		return
	}

	// Step 7 — submit, with a gas-bumped retry on a stuck tx.
	dvp := bind.NewBoundContract(h.dvpAddr, h.dvpABI, h.client, h.client, h.client)
	txReceipt, err := h.submitWithRetry(context.Background(), func(auth *bind.TransactOpts) (*types.Transaction, error) {
		return dvp.Transact(auth, "payment", receipt, p.vaultId, p.cipherText, p.encTxData)
	})
	if err != nil {
		h.finalizeSubmitError(dedupKey, err)
		finalized = true
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("payment(): %s", err)})
		return
	}
	if txReceipt.Status == types.ReceiptStatusFailed {
		h.markFailed(dedupKey)
		finalized = true
		c.JSON(http.StatusBadRequest, gin.H{"error": "transaction reverted on-chain"})
		return
	}

	h.markMined(dedupKey, txReceipt)
	finalized = true
	c.JSON(http.StatusOK, RelayPaymentResponse{
		TxHash:      txReceipt.TxHash.Hex(),
		BlockNumber: txReceipt.BlockNumber.Uint64(),
		GasUsed:     txReceipt.GasUsed,
	})
}

// ── Payment via relayer, with fee ────────────────────────────────────────────

// RelayPaymentFee is the gin handler for POST /relay/payment_fee.
//
// This is the fee-paying counterpart to RelayPayment: it's how a payer
// compensates the relayer for sponsoring gas, as opposed to self-submitting
// payment() directly with their own key and no fee. It calls
// EnygmaDvp.paymentWithRelayerFee() using the PaymentRelayerFeePublic
// circuit (1 input / 3 outputs, 9-element public signal) — Alice's proof
// commits to a third output note (StCommitmentsOut[2]) that the relayer
// receives and can later spend, rather than a value silently burned from
// the shielded pool. StFee (publicSignal[8]) is the publicly verifiable
// amount of that note; RELAYER_MIN_FEE is enforced against it before any
// chain interaction.
//
// Validation steps:
//  1. Parse and decode all fields from the request body.
//  2. Dedup check (durable, survives a restart).
//  3. Local Groth16 verify against the PaymentRelayerFeePublic verifying
//     key — no network call, rejects a forged/malformed proof in ~1ms.
//  4. Enforce cfg.MinFee against StFee (below floor → 402, before any chain call).
//  5. Check the Merkle root is known to the vault (rootHistory).
//  6. Check the nullifier has not been spent (nullifiers mapping).
//  7. Dry-run paymentWithRelayerFee() via eth_call — no gas spent if it would revert.
//  8. Sign and submit to EnygmaDvp.paymentWithRelayerFee(), with a
//     gas-bumped retry on a stuck tx.
func (h *Handler) RelayPaymentFee(c *gin.Context) {
	var req RelayPaymentFeeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Step 1 — decode all big.Int fields.
	p, err := parseFeeRequest(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("parse: %s", err)})
		return
	}

	// Public signal layout: [msg, treeNum0, root0, nullifier0, cmtBob, cmtChange, cmtFee, contractAddr, fee]
	treeNum := p.publicSignal[1]
	root := p.publicSignal[2]
	nullifier := p.publicSignal[3]

	// Step 2 — dedup check first — see RelayPayment. Cheapest check
	// available, so it runs before local verify, the fee check, and the
	// on-chain reads.
	//
	// Shares RelayPayment's "nullifier:"-prefixed key namespace (see its
	// comment) rather than a "fee:"-only prefix, so the same input note
	// can't be claimed as free on both /relay/payment and
	// /relay/payment_fee at once.
	dedupKey := "nullifier:" + treeNum.String() + ":" + nullifier.String()
	blocked, cached, err := h.store.TryBeginPending(dedupKey, h.cfg.DedupStaleness)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("dedup store: %v", err)})
		return
	}
	if blocked {
		if cached != nil {
			c.JSON(http.StatusOK, RelayPaymentResponse{TxHash: cached.TxHash, BlockNumber: cached.BlockNumber, GasUsed: cached.GasUsed})
			return
		}
		c.JSON(http.StatusConflict, gin.H{"error": "nullifier already in-flight"})
		return
	}

	// See RelayPayment — guarantees dedupKey is freed even on a panic below.
	finalized := false
	defer func() {
		if !finalized {
			h.markFailed(dedupKey)
		}
	}()

	// Step 3 — local Groth16 verify, before anything else touches the
	// network: no point pricing or dry-running a proof that isn't even valid.
	if err := h.vRelayerFee.Verify(p.proof, p.publicSignal[:]); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid zk proof (local verification failed): %v", err)})
		return
	}

	// Step 4 — enforce the relayer's minimum fee before touching chain
	// state. publicSignal[8] is the PaymentRelayerFeePublic circuit's
	// public StFee slot — the amount of the spendable note Alice's proof
	// commits to leaving the relayer. A disabled minimum (0) skips the check.
	fee := p.publicSignal[8]
	if h.cfg.MinFee.Sign() > 0 && fee.Cmp(h.cfg.MinFee) < 0 {
		c.JSON(http.StatusPaymentRequired, gin.H{
			"error": fmt.Sprintf("fee %s below relayer minimum %s", fee, h.cfg.MinFee),
		})
		return
	}

	vault := bind.NewBoundContract(h.vaultAddr, h.vaultABI, h.client, h.client, h.client)

	// Step 5 — Merkle root must be in the vault's rootHistory.
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

	// Step 6 — nullifier must not already be spent.
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

	receipt := buildPaymentFeeReceipt(p)

	// Step 7 — dry-run before touching the mutex/chain state: a proof that
	// would revert has burned nothing here, versus real gas if broadcast.
	simCtx, simCancel := context.WithTimeout(context.Background(), simulateTimeout)
	simErr := h.simulateDvpCall(simCtx, "paymentWithRelayerFee", receipt, p.vaultId, p.cipherText, p.encTxData)
	simCancel()
	if simErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("paymentWithRelayerFee would revert on-chain (dry-run, no gas spent): %v", simErr)})
		return
	}

	// Step 8 — submit, with a gas-bumped retry on a stuck tx.
	dvp := bind.NewBoundContract(h.dvpAddr, h.dvpABI, h.client, h.client, h.client)
	txReceipt, err := h.submitWithRetry(context.Background(), func(auth *bind.TransactOpts) (*types.Transaction, error) {
		return dvp.Transact(auth, "paymentWithRelayerFee", receipt, p.vaultId, p.cipherText, p.encTxData)
	})
	if err != nil {
		h.finalizeSubmitError(dedupKey, err)
		finalized = true
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("paymentWithRelayerFee(): %s", err)})
		return
	}
	if txReceipt.Status == types.ReceiptStatusFailed {
		h.markFailed(dedupKey)
		finalized = true
		c.JSON(http.StatusBadRequest, gin.H{"error": "transaction reverted on-chain"})
		return
	}

	h.markMined(dedupKey, txReceipt)
	finalized = true
	c.JSON(http.StatusOK, RelayPaymentResponse{
		TxHash:      txReceipt.TxHash.Hex(),
		BlockNumber: txReceipt.BlockNumber.Uint64(),
		GasUsed:     txReceipt.GasUsed,
	})
}

// ── Tag relay ─────────────────────────────────────────────────────────────────

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
//  3. Dedup check (durable, survives a restart).
//  4. Submit publishTag() — in window mode, retry with a different tag on
//     block drift; each attempt goes through the shared nonce manager and
//     gets one gas-bumped retry on a stuck tx.
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

	ctxt, err := decodeHexField(req.Ctxt, "ctxt")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate single tag now; window tags are validated on each attempt below.
	var singleTag [32]byte
	if !windowMode {
		tagBytes, err := decodeHexField(req.Tag, "tag")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if len(tagBytes) != 32 {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("tag must be 32 bytes, got %d", len(tagBytes))})
			return
		}
		copy(singleTag[:], tagBytes)
	}

	// Step 2 — TagRegistry must be configured.
	if (h.tagRegistryAddr == common.Address{}) || len(h.tagRegistryABI.Methods) == 0 {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "tag registry not configured — set RELAYER_TAG_REGISTRY_ADDR",
		})
		return
	}

	// Step 3 — dedup check.
	inFlightKey := req.Tag
	if windowMode {
		inFlightKey = req.Tags[0] // keyed by first window tag
	}
	dedupKey := "tag:" + inFlightKey
	blocked, cached, err := h.store.TryBeginPending(dedupKey, h.cfg.DedupStaleness)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("dedup store: %v", err)})
		return
	}
	if blocked {
		if cached != nil {
			c.JSON(http.StatusOK, RelayTagResponse{TxHash: cached.TxHash, BlockNumber: cached.BlockNumber, GasUsed: cached.GasUsed})
			return
		}
		c.JSON(http.StatusConflict, gin.H{"error": "tag already in-flight"})
		return
	}
	finalized := false
	defer func() {
		if !finalized {
			h.markFailed(dedupKey)
		}
	}()

	// Step 4 — submit. Window mode retries up to 3 times on block drift.
	registry := bind.NewBoundContract(h.tagRegistryAddr, h.tagRegistryABI, h.client, h.client, h.client)
	const maxAttempts = 3

	for attempt := 0; attempt < maxAttempts; attempt++ {
		var tag [32]byte

		if windowMode {
			// Pick the tag for the block this tx is expected to land in.
			currentBlock, err := h.client.BlockNumber(context.Background())
			if err != nil {
				h.markFailed(dedupKey)
				finalized = true
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("blockNumber: %s", err)})
				return
			}
			targetBlock := currentBlock + 1
			idx := int64(targetBlock) - int64(req.StartBlock)
			if idx < 0 || int(idx) >= len(req.Tags) {
				h.markFailed(dedupKey)
				finalized = true
				c.JSON(http.StatusConflict, gin.H{
					"error": fmt.Sprintf("window [%d, %d) does not cover expected block %d — resubmit with a new window",
						req.StartBlock, req.StartBlock+uint64(len(req.Tags)), targetBlock),
				})
				return
			}
			tagBytes, err := decodeHexField(req.Tags[idx], fmt.Sprintf("tags[%d]", idx))
			if err != nil {
				h.markFailed(dedupKey)
				finalized = true
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if len(tagBytes) != 32 {
				h.markFailed(dedupKey)
				finalized = true
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("tags[%d] must be 32 bytes, got %d", idx, len(tagBytes))})
				return
			}
			copy(tag[:], tagBytes)
		} else {
			tag = singleTag
		}

		txReceipt, err := h.submitWithRetry(context.Background(), func(auth *bind.TransactOpts) (*types.Transaction, error) {
			return registry.Transact(auth, "publishTag", tag, ctxt)
		})
		if err != nil {
			h.finalizeSubmitError(dedupKey, err)
			finalized = true
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("publishTag(): %s", err)})
			return
		}
		if txReceipt.Status == types.ReceiptStatusFailed {
			h.markFailed(dedupKey)
			finalized = true
			c.JSON(http.StatusBadRequest, gin.H{"error": "transaction reverted on-chain (tag may already exist in this block)"})
			return
		}

		actualBlock := txReceipt.BlockNumber.Uint64()

		// In window mode, check whether the tx landed in the block we targeted.
		// If not, the tag we just published is mined and permanent (there's
		// no undoing publishTag) but it's for the wrong block, so this
		// request isn't satisfied yet — retry with the correct tag for the
		// block we actually landed in.
		if windowMode {
			tagIdx := int64(actualBlock) - int64(req.StartBlock)
			if tagIdx < 0 || int(tagIdx) >= len(req.Tags) {
				// Landed outside the window entirely.
				h.markFailed(dedupKey)
				finalized = true
				c.JSON(http.StatusConflict, gin.H{
					"error": fmt.Sprintf("tx landed in block %d which is outside window [%d, %d) — resubmit",
						actualBlock, req.StartBlock, req.StartBlock+uint64(len(req.Tags))),
				})
				return
			}
			// The correct tag for the actual block is tags[tagIdx] — compare
			// against what we just submitted. submittedCorrect defaults to
			// false (not true): a decode error or wrong length here means
			// this entry can't be confirmed to match what was published, and
			// the mined publishTag is on-chain and permanent regardless — so
			// an unverifiable entry must fall through to the "not confirmed
			// correct" path below rather than be persisted via markMined as
			// a verified success it never actually was.
			correctTagBytes, decodeErr := decodeHexField(req.Tags[tagIdx], "")
			submittedCorrect := decodeErr == nil && len(correctTagBytes) == 32
			if submittedCorrect {
				for i, b := range correctTagBytes {
					if tag[i] != b {
						submittedCorrect = false
						break
					}
				}
			}
			if submittedCorrect {
				h.markMined(dedupKey, txReceipt)
				finalized = true
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

		h.markMined(dedupKey, txReceipt)
		finalized = true
		c.JSON(http.StatusOK, RelayTagResponse{
			TxHash:      txReceipt.TxHash.Hex(),
			BlockNumber: actualBlock,
			GasUsed:     txReceipt.GasUsed,
		})
		return
	}

	h.markFailed(dedupKey)
	finalized = true
	c.JSON(http.StatusConflict, gin.H{"error": "exceeded retry limit — resubmit with a new window starting from the current block"})
}

// ── Channel relay (sender privacy §6.3) ──────────────────────────────────────

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
//  3. Dedup check (durable, survives a restart).
//  4. Submit openChannel(c1, c2, bitmap), with a gas-bumped retry on a
//     stuck tx, and parse the ChannelOpened event to return the assigned
//     channelIdx.
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

	// Step 3 — dedup check (keyed by the first 34 chars of c1's hex string).
	c1Key := req.C1[:min(34, len(req.C1))]
	dedupKey := "channel:" + c1Key
	blocked, cached, err := h.store.TryBeginPending(dedupKey, h.cfg.DedupStaleness)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("dedup store: %v", err)})
		return
	}
	if blocked {
		if cached != nil {
			c.JSON(http.StatusOK, RelayChannelResponse{
				ChannelIdx:  cached.ChannelIdx,
				TxHash:      cached.TxHash,
				BlockNumber: cached.BlockNumber,
				GasUsed:     cached.GasUsed,
			})
			return
		}
		c.JSON(http.StatusConflict, gin.H{"error": "channel setup already in-flight"})
		return
	}
	finalized := false
	defer func() {
		if !finalized {
			h.markFailed(dedupKey)
		}
	}()

	// Step 4 — submit openChannel(c1, c2, bitmap).
	registry := bind.NewBoundContract(h.tagChannelRegistryAddr, h.tagChannelRegistryABI, h.client, h.client, h.client)
	txReceipt, err := h.submitWithRetry(context.Background(), func(auth *bind.TransactOpts) (*types.Transaction, error) {
		return registry.Transact(auth, "openChannel", c1, c2, bitmap)
	})
	if err != nil {
		h.finalizeSubmitError(dedupKey, err)
		finalized = true
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("openChannel(): %s", err)})
		return
	}
	if txReceipt.Status == types.ReceiptStatusFailed {
		h.markFailed(dedupKey)
		finalized = true
		c.JSON(http.StatusBadRequest, gin.H{"error": "transaction reverted on-chain"})
		return
	}

	// Parse ChannelOpened event to extract the assigned channelIdx.
	var channelIdx uint64
	channelOpenedSig := crypto.Keccak256Hash([]byte("ChannelOpened(uint256,address)"))
	for _, log := range txReceipt.Logs {
		if log.Topics[0] == channelOpenedSig {
			channelIdx = new(big.Int).SetBytes(log.Topics[1].Bytes()).Uint64()
			break
		}
	}

	h.markMinedChannel(dedupKey, txReceipt, channelIdx)
	finalized = true
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

// recheckMined does one bounded, one-shot receipt lookup for tx — used right
// before deciding a mined-wait's deadline means "stuck" and something needs
// resubmitting. Catches the tx having actually mined in the same instant the
// wait's deadline fired, so a submission that already succeeded on-chain
// isn't reported as failed and resubmitted as a conflicting same-nonce
// replacement.
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
// to h.txTimeout for it to be mined with the lock released, so a slow
// mined-wait never blocks other callers from claiming their own nonces and
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
	// so a submission that already succeeded on-chain isn't reported as a
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
		// "nonce too low" is not ambiguous at all, unlike the other errors
		// handled below: the node validates and rejects a stale nonce
		// synchronously, before the tx ever enters the mempool — a
		// definite non-send. This specific case arises whenever something
		// outside this relayer process advances the chain's nonce for its
		// own address between our nonce manager's last sync and now (an
		// operator sending a manual tx from the same key, a test suite
		// that happens to reuse it, ...) — our purely-local counter has no
		// way to observe that on its own. Resync and retry once with the
		// corrected nonce rather than give up: the caller's dedup
		// (store.TryBeginPending) already guarantees this is the only
		// attempt at this logical request, so retrying it here with a
		// different nonce can't create a duplicate submission.
		if isNonceTooLowError(err) {
			if resyncErr := h.nonces.resync(subCtx, h.client, h.auth.From); resyncErr != nil {
				log.Printf("relayer: nonce %d rejected as too low, and resync failed: %v", nonce, resyncErr)
				return nil, nil, 0, fmt.Errorf("%w: %w", err, errAmbiguousOutcome)
			}
			retryNonce := h.nonces.take()
			log.Printf("relayer: nonce %d rejected as too low — resynced and retrying once with nonce %d", nonce, retryNonce)
			auth.Nonce = new(big.Int).SetUint64(retryNonce)
			retryTx, retryErr := doCall(&auth)
			if retryErr == nil {
				return retryTx, gasPrice, retryNonce, nil
			}
			err = retryErr
			nonce = retryNonce
		}

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

// isNonceTooLowError reports whether err is a node's rejection of a stale
// nonce — a definite non-send, checked and rejected before the transaction
// ever enters the mempool. Matches both geth's wording ("nonce too low")
// and Hardhat's ("Nonce too low. Expected nonce to be X but got Y...").
func isNonceTooLowError(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "nonce too low")
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

// markMinedChannel is markMined plus the assigned channelIdx — see
// RelayChannel and store.Store.MarkMinedChannel.
func (h *Handler) markMinedChannel(dedupKey string, receipt *types.Receipt, channelIdx uint64) {
	if err := h.store.MarkMinedChannel(dedupKey, receipt.TxHash.Hex(), receipt.BlockNumber.Uint64(), receipt.GasUsed, channelIdx); err != nil {
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

	var sig [7]*big.Int
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

func parseFeeRequest(req *RelayPaymentFeeRequest) (*parsedFee, error) {
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

	var sig [9]*big.Int
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

	return &parsedFee{
		vaultId:      vaultId,
		proof:        proof,
		publicSignal: sig,
		cipherText:   ctBytes,
		encTxData:    encBytes,
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
//
// Payment circuit (1 input / 2 outputs) public signal layout:
//
//	[msg, treeNum0, root0, nullifier0, commitment0, commitment1]
//
// ProofReceipt.Statement must be in non-interleaved form:
//
//	[msg, treeNum0, root0, nullifier0, commitment0, commitment1]
//
// NumberOfInputs=1, NumberOfOutputs=2.
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
	statement := make([]*big.Int, 7)
	for i := range p.publicSignal {
		statement[i] = p.publicSignal[i]
	}
	return proofReceipt{
		Proof:           sp,
		Statement:       statement,
		NumberOfInputs:  big.NewInt(1),
		NumberOfOutputs: big.NewInt(2),
	}
}

// buildPaymentFeeReceipt maps the parsed fee proof + public signal into the
// ProofReceipt struct that EnygmaDvp.paymentWithRelayerFee() expects.
//
// PaymentRelayerFeePublic circuit (1 input / 3 outputs) public signal layout:
//
//	[msg, treeNum0, root0, nullifier0, commitBob, commitChange, commitFee, contractAddress, fee]
//
// ProofReceipt.Statement carries all 9 elements through untouched — the
// vault's checkReceiptConditions indexes into it relative to
// numberOfInputs/numberOfOutputs, so the trailing contractAddress/fee
// elements are inert to that logic beyond being part of the verified
// witness. NumberOfInputs=1, NumberOfOutputs=3.
func buildPaymentFeeReceipt(p *parsedFee) proofReceipt {
	// Proof element order from gnark: [Ax, Ay, BX_imag, BX_real, BY_imag, BY_real, Cx, Cy]
	sp := snarkProof{
		A: g1Point{X: p.proof[0], Y: p.proof[1]},
		B: g2Point{
			X: [2]*big.Int{p.proof[2], p.proof[3]},
			Y: [2]*big.Int{p.proof[4], p.proof[5]},
		},
		C: g1Point{X: p.proof[6], Y: p.proof[7]},
	}
	statement := make([]*big.Int, len(p.publicSignal))
	for i := range p.publicSignal {
		statement[i] = p.publicSignal[i]
	}
	return proofReceipt{
		Proof:           sp,
		Statement:       statement,
		NumberOfInputs:  big.NewInt(1),
		NumberOfOutputs: big.NewInt(3),
	}
}

// simulateDvpCall dry-runs an EnygmaDvp method via eth_call — no broadcast,
// no gas spent. Returns the decoded on-chain revert error if the call would
// fail, nil if it would succeed. Called before every broadcast: a Groth16
// verify that reverts costs real gas even though it changes no state.
//
// Pending:true evaluates against the relayer's own still-unmined broadcasts,
// not just the latest mined block — otherwise a payment whose proof assumes
// an earlier, already-broadcast-but-not-yet-mined payment already applied
// would be dry-run against stale pre-payment state and rejected even though
// it would succeed once mined in order.
func (h *Handler) simulateDvpCall(ctx context.Context, method string, args ...interface{}) error {
	dvp := bind.NewBoundContract(h.dvpAddr, h.dvpABI, h.client, h.client, h.client)
	var out []interface{}
	return dvp.Call(&bind.CallOpts{From: h.auth.From, Context: ctx, Pending: true}, &out, method, args...)
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
