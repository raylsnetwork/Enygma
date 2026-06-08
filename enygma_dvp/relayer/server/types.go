package server

// ── shared receipt payload ────────────────────────────────────────────────────

// ReceiptPayload carries a single Groth16 proof and its public signal.
// NumberOfInputs and NumberOfOutputs are required by the ProofReceipt struct
// on-chain and also drive pre-flight nullifier/root validation.
type ReceiptPayload struct {
	Proof           [8]string `json:"proof"           binding:"required"`
	PublicSignal    []string  `json:"publicSignal"    binding:"required"`
	NumberOfInputs  int       `json:"numberOfInputs"  binding:"required"`
	NumberOfOutputs int       `json:"numberOfOutputs" binding:"required"`
}

// ── payment ──────────────────────────────────────────────────────────────────

// RelayPaymentRequest is the body for POST /relay/payment.
// PublicSignal layout (non-interleaved, 1-in/2-out):
//
//	[msg, treeNum0, root0, nullifier0, commitment_bob, commitment_change]
type RelayPaymentRequest struct {
	VaultId    string  `json:"vaultId"    binding:"required"`
	Receipt    ReceiptPayload `json:"receipt"    binding:"required"`
	CipherText string  `json:"cipherText" binding:"required"` // Bob's ML-KEM capsule (hex)
	EncTxData  string  `json:"encTxData"  binding:"required"` // Bob's AES-GCM payload (hex)
}

// ── swap ─────────────────────────────────────────────────────────────────────

// RelaySwapRequest is the body for POST /relay/swap.
// Calls EnygmaDvp.swap(paymentReceipt, deliveryReceipt, paymentVaultId, deliveryVaultId).
type RelaySwapRequest struct {
	PaymentReceipt  ReceiptPayload `json:"paymentReceipt"  binding:"required"`
	DeliveryReceipt ReceiptPayload `json:"deliveryReceipt" binding:"required"`
	PaymentVaultId  string         `json:"paymentVaultId"  binding:"required"`
	DeliveryVaultId string         `json:"deliveryVaultId" binding:"required"`
}

// ── exchange ─────────────────────────────────────────────────────────────────

// RelayExchangeRequest is the body for POST /relay/exchange.
// Calls EnygmaDvp.exchange(receipt1, receipt2, vaultId1, vaultId2).
type RelayExchangeRequest struct {
	Receipt1 ReceiptPayload `json:"receipt1" binding:"required"`
	Receipt2 ReceiptPayload `json:"receipt2" binding:"required"`
	VaultId1 string         `json:"vaultId1" binding:"required"`
	VaultId2 string         `json:"vaultId2" binding:"required"`
}

// ── response ─────────────────────────────────────────────────────────────────

// RelayResponse is the body returned on success for all relay endpoints.
type RelayResponse struct {
	TxHash      string `json:"txHash"`
	BlockNumber uint64 `json:"blockNumber"`
	GasUsed     uint64 `json:"gasUsed"`
}
