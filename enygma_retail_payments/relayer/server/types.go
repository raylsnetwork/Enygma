package server

import "math/big"

// RelayPaymentRequest is the JSON body accepted by POST /relay/payment.
//
// The client (user wallet) submits everything the relayer needs to
// validate and submit the transaction. The relayer holds the signing key;
// the client never touches it.
//
// Proof element order (8 elements, decimal strings):
//
//	[Ax, Ay, BX_imag, BX_real, BY_imag, BY_real, Cx, Cy]
//
// PublicSignal layout for the Payment circuit (1 input / 2 outputs):
//
//	[msg, treeNum0, root0, nullifier0, commitment0, commitment1]
type RelayPaymentRequest struct {
	// VaultId identifies which Erc20CoinVault tree the input note belongs to.
	VaultId string `json:"vaultId" binding:"required"`
	// Proof is the 8-element Groth16 proof array (decimal strings).
	Proof [8]string `json:"proof" binding:"required"`
	// PublicSignal is the 7-element public witness array (decimal strings).
	// Elements: [msg, treeNum, root, nullifier, commitOut, commitChange, contractAddress]
	PublicSignal [7]string `json:"publicSignal" binding:"required"`
	// CipherText is hex-encoded bytes passed opaquely to EnygmaDvp.payment().
	// ML-KEM mode:    ML-KEM-768 capsule (1088 bytes).
	// Channel mode:   32-byte channelId identifying the shared channel.
	CipherText string `json:"cipherText" binding:"required"`
	// EncTxData is hex-encoded bytes passed opaquely to EnygmaDvp.payment().
	// ML-KEM mode:    AES-GCM note payload encrypted with ML-KEM-derived key.
	// Channel mode:   AES-256-GCM note payload encrypted with channel key.
	EncTxData string `json:"encTxData" binding:"required"`
}

// RelayPaymentResponse is returned on success.
type RelayPaymentResponse struct {
	TxHash      string `json:"txHash"`
	BlockNumber uint64 `json:"blockNumber"`
	GasUsed     uint64 `json:"gasUsed"`
}

// parsed holds the decoded form of a RelayPaymentRequest ready for validation
// and on-chain submission.
type parsed struct {
	vaultId      *big.Int
	proof        [8]*big.Int
	publicSignal [7]*big.Int
	cipherText   []byte
	encTxData    []byte
}

// ── Info ──────────────────────────────────────────────────────────────────────

// InfoResponse is returned by GET /relay/info.
// Clients call this to discover configured contract addresses before
// publishing tags or channels, avoiding the chicken-and-egg problem
// where addresses must be known before the relayer starts.
type InfoResponse struct {
	RelayerAddr            string `json:"relayerAddr"`
	TagRegistryAddr        string `json:"tagRegistryAddr"`
	TagChannelRegistryAddr string `json:"tagChannelRegistryAddr"`
	ChainID                int64  `json:"chainId"`
}

// ── Tag relay ─────────────────────────────────────────────────────────────────

// RelayTagRequest is the JSON body accepted by POST /relay/tag.
//
// Two modes:
//
// Single-tag mode — caller predicts the exact block (original API):
//
//	{ "tag": "0x<32-byte hex>", "ctxt": "0x..." }
//
// Window mode — caller provides a window of pre-derived tags so the relayer
// can pick the one matching the actual landing block, eliminating block-
// prediction mismatch:
//
//	{ "tags": ["0x...", "0x...", "0x..."], "startBlock": 42, "ctxt": "0x..." }
//
// In window mode the relayer queries the pending block, picks the appropriate
// tag, and retries (up to 3 times) if the tx lands in a later block that is
// still within the window.  Use tags.PrepareTagWithWindow to build the request.
type RelayTagRequest struct {
	// Single-tag mode: pre-computed tag for a specific block.
	Tag string `json:"tag,omitempty"`

	// Window mode: tags[i] is the Poseidon tag for block startBlock+i.
	// Must be used together with StartBlock.
	Tags       []string `json:"tags,omitempty"`
	StartBlock uint64   `json:"startBlock,omitempty"`

	// Ctxt is always required in both modes.
	Ctxt string `json:"ctxt" binding:"required"`
}

// RelayTagResponse is returned on successful tag publication.
type RelayTagResponse struct {
	TxHash      string `json:"txHash"`
	BlockNumber uint64 `json:"blockNumber"`
	GasUsed     uint64 `json:"gasUsed"`
}

// ── Channel relay (sender privacy §6.3) ──────────────────────────────────────

// RelayChannelRequest is the JSON body accepted by POST /relay/channel.
//
// The client performs ML-KEM encapsulation + AEAD encryption locally (steps 3–6
// of paper §3) and sends the blobs to the relayer. The relayer submits
// TagChannelRegistry.openChannel(c1, c2, bitmap) on-chain with its own key.
//
// Because msg.sender = relayer address, external observers cannot link the
// channel setup to the actual sender's Ethereum address (paper §6.3).
//
//	C1     — ML-KEM-768 ciphertext, exactly 1088 bytes (2176 hex chars + "0x")
//	C2     — AEAD.Encrypt(k, payload, c1), up to 4096 bytes
//	Bitmap — packed privacy-mode bitmap, up to 16384 bytes
type RelayChannelRequest struct {
	C1     string `json:"c1"     binding:"required"`
	C2     string `json:"c2"     binding:"required"`
	Bitmap string `json:"bitmap" binding:"required"`
}

// RelayChannelResponse is returned on successful channel publication.
type RelayChannelResponse struct {
	ChannelIdx  uint64 `json:"channelIdx"`
	TxHash      string `json:"txHash"`
	BlockNumber uint64 `json:"blockNumber"`
	GasUsed     uint64 `json:"gasUsed"`
}




