package server

// ── shared receipt payload ────────────────────────────────────────────────────

// ReceiptPayload carries a single Groth16 proof and its public signal.
// NumberOfInputs and NumberOfOutputs are required by the ProofReceipt struct
// on-chain; they also pick which of the route's circuits the receipt is, and
// PublicSignal must then be exactly that circuit's statement.
type ReceiptPayload struct {
	Proof           [8]string `json:"proof"           binding:"required"`
	PublicSignal    []string  `json:"publicSignal"    binding:"required"`
	NumberOfInputs  int       `json:"numberOfInputs"  binding:"required"`
	NumberOfOutputs int       `json:"numberOfOutputs" binding:"required"`
}

// ── payment ──────────────────────────────────────────────────────────────────

// RelayPaymentRequest is the body for POST /relay/payment.
// Receipt is a Payment (1-in/2-out) or Payment2in (2-in/2-out) statement, in
// the order enygma_dvp/layout declares.
type RelayPaymentRequest struct {
	VaultId    string         `json:"vaultId"    binding:"required"`
	Receipt    ReceiptPayload `json:"receipt"    binding:"required"`
	CipherText string         `json:"cipherText" binding:"required"` // Bob's ML-KEM capsule (hex)
	EncTxData  string         `json:"encTxData"  binding:"required"` // Bob's AES-GCM payload (hex)
}

// ── relayer fee (same token — PaymentRelayerFeePublic circuit) ────────────────

// RelayPaymentRelayerFeeRequest is the body for POST /relay/payment_relayer_fee.
// Receipt is a PaymentRelayerFee (1-in/3-out) statement; its outputs are
// Bob's note, the change and the relayer's fee note.
//
// FeeSalt/TokenId are supplied so the relayer can independently recompute the
// fee note's commitment (Erc20CommitmentV2(feeSpendPubKey, feeSalt, fee,
// tokenId)) and confirm it matches output 2 — i.e. that the note is
// actually addressed to this relayer's own key — before ever submitting
// on-chain. TokenId is a private witness in this circuit (unlike UsdrFee's),
// so it must be supplied here rather than read from the public signal.
type RelayPaymentRelayerFeeRequest struct {
	VaultId    string         `json:"vaultId"    binding:"required"`
	Receipt    ReceiptPayload `json:"receipt"    binding:"required"`
	CipherText string         `json:"cipherText" binding:"required"`
	EncTxData  string         `json:"encTxData"  binding:"required"`
	FeeSalt    string         `json:"feeSalt"    binding:"required"`
	TokenId    string         `json:"tokenId"    binding:"required"`
}

// ── partial withdraw (PaymentRelayerFeePublic circuit) ────────────────────────

// RelayPartialWithdrawRequest is the body for POST /relay/partial_withdraw.
// Receipt is a PaymentRelayerFee (1-in/3-out) statement; its outputs are
// cmtWithdraw, cmtChange and cmtRelayer.
//
// cmtWithdraw must be Poseidon4(uint160(Recipient), 0, Amount, TokenId): the
// part paid out to Recipient. cmtChange is the owner's change note, and
// CipherText/EncTxData are its discovery data (ML-KEM capsule / AES-GCM
// payload, hex). FeeSalt and TokenId let the relayer confirm cmtRelayer is
// addressed to its own fee key, as for /relay/payment_relayer_fee.
type RelayPartialWithdrawRequest struct {
	VaultId    string         `json:"vaultId"    binding:"required"`
	Receipt    ReceiptPayload `json:"receipt"    binding:"required"`
	Recipient  string         `json:"recipient"  binding:"required"`
	Amount     string         `json:"amount"     binding:"required"`
	TokenId    string         `json:"tokenId"    binding:"required"`
	FeeSalt    string         `json:"feeSalt"    binding:"required"`
	CipherText string         `json:"cipherText" binding:"required"`
	EncTxData  string         `json:"encTxData"  binding:"required"`
}

// ── USDr fee (second, independent asset — UsdrFeeCircuit) ─────────────────────

// RelayPaymentUsdrFeeRequest is the body for POST /relay/payment_usdr_fee.
// Settles two independent proofs atomically in one on-chain call
// (EnygmaDvp.paymentWithUsdrFee): a normal payment against VaultId, and a
// UsdrFeeCircuit proof — a second, independent relayer-fee asset with its own
// token/vault/circuit — against UsdrVaultId.
//
// Receipt is a Payment or Payment2in statement; UsdrReceipt is a UsdrFee
// (1-in/2-out) statement whose outputs are the relayer's fee note and the
// sender's change.
//
// UsdrFeeSalt is supplied so the relayer can recompute the USDr fee note's
// commitment; fee and tokenId are read straight from UsdrReceipt's own public
// signal (both public there, unlike the same-asset relayer-fee mechanism).
type RelayPaymentUsdrFeeRequest struct {
	VaultId    string         `json:"vaultId"    binding:"required"`
	Receipt    ReceiptPayload `json:"receipt"    binding:"required"`
	CipherText string         `json:"cipherText" binding:"required"`
	EncTxData  string         `json:"encTxData"  binding:"required"`

	UsdrVaultId    string         `json:"usdrVaultId"    binding:"required"`
	UsdrReceipt    ReceiptPayload `json:"usdrReceipt"    binding:"required"`
	UsdrCipherText string         `json:"usdrCipherText" binding:"required"`
	UsdrEncTxData  string         `json:"usdrEncTxData"  binding:"required"`
	UsdrFeeSalt    string         `json:"usdrFeeSalt"    binding:"required"`
}

// ── swap ─────────────────────────────────────────────────────────────────────

// RelaySwapRequest is the body for POST /relay/swap.
// PaymentReceipt is a DvpInitiator statement, DeliveryReceipt a DvpDestination
// one. Calls EnygmaDvp.swap(paymentReceipt, deliveryReceipt, paymentVaultId, deliveryVaultId).
type RelaySwapRequest struct {
	PaymentReceipt  ReceiptPayload `json:"paymentReceipt"  binding:"required"`
	DeliveryReceipt ReceiptPayload `json:"deliveryReceipt" binding:"required"`
	PaymentVaultId  string         `json:"paymentVaultId"  binding:"required"`
	DeliveryVaultId string         `json:"deliveryVaultId" binding:"required"`
}

// ── exchange ─────────────────────────────────────────────────────────────────

// RelayExchangeRequest is the body for POST /relay/exchange.
// Both receipts are DvpInitiator statements.
// Calls EnygmaDvp.exchange(receipt1, receipt2, vaultId1, vaultId2).
type RelayExchangeRequest struct {
	Receipt1 ReceiptPayload `json:"receipt1" binding:"required"`
	Receipt2 ReceiptPayload `json:"receipt2" binding:"required"`
	VaultId1 string         `json:"vaultId1" binding:"required"`
	VaultId2 string         `json:"vaultId2" binding:"required"`
}

// ── info ─────────────────────────────────────────────────────────────────────

// InfoResponse is returned by GET /relay/info.
type InfoResponse struct {
	RelayerAddr string `json:"relayerAddr"`
	// FeeSpendPubKey is the relayer's BabyJubJub spend public key for
	// relayer-fee notes (empty string if RELAYER_FEE_SPEND_PRIVATE_KEY is not
	// configured) — see POST /relay/payment_relayer_fee and
	// POST /relay/payment_usdr_fee.
	FeeSpendPubKey string `json:"feeSpendPubKey,omitempty"`
}

// ── response ─────────────────────────────────────────────────────────────────

// RelayResponse is the body returned on success for all relay endpoints.
type RelayResponse struct {
	TxHash      string `json:"txHash"`
	BlockNumber uint64 `json:"blockNumber"`
	GasUsed     uint64 `json:"gasUsed"`
}
