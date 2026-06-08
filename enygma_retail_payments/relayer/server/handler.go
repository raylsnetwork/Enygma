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

	"enygma_relayer/config"

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
	txMu                   sync.Mutex // serializes on-chain submissions — prevents nonce races
	inFlight               sync.Map   // prevents concurrent duplicate submissions
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
	}, nil
}

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
	})
}

// RelayPayment is the gin handler for POST /relay/payment.
//
// Validation steps:
//  1. Parse and decode all fields from the request body.
//  2. Check the Merkle root is known to the vault (rootHistory).
//  3. Check the nullifier has not been spent (nullifiers mapping).
//  4. Sign and submit to EnygmaDvp.payment().
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

	// Step 4 — claim nullifier as in-flight to block concurrent duplicate submissions.
	nfKey := p.publicSignal[1].String() + ":" + p.publicSignal[3].String()
	if _, loaded := h.inFlight.LoadOrStore(nfKey, struct{}{}); loaded {
		c.JSON(http.StatusConflict, gin.H{"error": "nullifier already in-flight"})
		return
	}
	defer h.inFlight.Delete(nfKey)

	// Step 5 — serialize submission to prevent nonce races under concurrency.
	h.txMu.Lock()
	defer h.txMu.Unlock()

	// Step 6 — build ProofReceipt and submit.
	receipt := buildProofReceipt(p)

	dvp := bind.NewBoundContract(h.dvpAddr, h.dvpABI, h.client, h.client, h.client)
	tx, err := dvp.Transact(h.auth, "payment", receipt, p.vaultId, p.cipherText, p.encTxData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("payment(): %s", err)})
		return
	}
	txReceipt, err := bind.WaitMined(context.Background(), h.client, tx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("wait mined: %s", err)})
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

	// Step 3 — in-flight dedup key.
	inFlightKey := req.Tag
	if windowMode {
		inFlightKey = req.Tags[0] // keyed by first window tag
	}
	if _, loaded := h.inFlight.LoadOrStore(inFlightKey, struct{}{}); loaded {
		c.JSON(http.StatusConflict, gin.H{"error": "tag already in-flight"})
		return
	}
	defer h.inFlight.Delete(inFlightKey)

	// Step 4 — serialize submission.
	h.txMu.Lock()
	defer h.txMu.Unlock()

	// Step 5 — submit. Window mode retries up to 3 times on block drift.
	registry := bind.NewBoundContract(h.tagRegistryAddr, h.tagRegistryABI, h.client, h.client, h.client)
	const maxAttempts = 3

	for attempt := 0; attempt < maxAttempts; attempt++ {
		var tag [32]byte

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
			tagBytes, err := decodeHexField(req.Tags[idx], fmt.Sprintf("tags[%d]", idx))
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if len(tagBytes) != 32 {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("tags[%d] must be 32 bytes, got %d", idx, len(tagBytes))})
				return
			}
			copy(tag[:], tagBytes)
		} else {
			tag = singleTag
		}

		tx, err := registry.Transact(h.auth, "publishTag", tag, ctxt)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("publishTag(): %s", err)})
			return
		}
		txReceipt, err := bind.WaitMined(context.Background(), h.client, tx)
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
	c1Key := req.C1[:min(34, len(req.C1))]
	if _, loaded := h.inFlight.LoadOrStore(c1Key, struct{}{}); loaded {
		c.JSON(http.StatusConflict, gin.H{"error": "channel setup already in-flight"})
		return
	}
	defer h.inFlight.Delete(c1Key)

	// Step 4 — serialize submission to prevent nonce races.
	h.txMu.Lock()
	defer h.txMu.Unlock()

	// Step 5 — submit openChannel(c1, c2, bitmap).
	registry := bind.NewBoundContract(h.tagChannelRegistryAddr, h.tagChannelRegistryABI, h.client, h.client, h.client)
	tx, err := registry.Transact(h.auth, "openChannel", c1, c2, bitmap)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("openChannel(): %s", err)})
		return
	}
	txReceipt, err := bind.WaitMined(context.Background(), h.client, tx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("wait mined: %s", err)})
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
