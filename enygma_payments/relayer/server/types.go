package server

// ── Transfer ─────────────────────────────────────────────────────────────────

// RelayTransferRequest is the JSON body accepted by POST /relay/transfer.
//
// Used for Enygma-to-Enygma confidential transfers (the enygma circuit),
// plus a second, independent USDr proof that pays the relayer a fixed fee,
// settled atomically in the same on-chain transfer() call. Both PublicSignal
// and UsdrPublicSignal must have exactly 81 elements (the last being the
// Fix L-01 domain separator on both circuits) — Fix L-05: this used to be
// silently zero-padded up to 81 rather than rejected at any other length.
// UsdrPublicSignal additionally carries the fee amount as a public signal,
// checked on-chain against usdrFixedFeeAmount. Both proofs share KIndex
// (the same k=6 anonymity-set participantIds).
type RelayTransferRequest struct {
	Proof        [8]string  `json:"proof"        binding:"required"`
	PublicSignal []string   `json:"publicSignal" binding:"required"`
	Commitments  [][]string `json:"commitments"  binding:"required"`

	UsdrProof        [8]string  `json:"usdrProof"        binding:"required"`
	UsdrPublicSignal []string   `json:"usdrPublicSignal" binding:"required"`
	UsdrCommitments  [][]string `json:"usdrCommitments"  binding:"required"`

	KIndex []int64 `json:"kIndex" binding:"required"` // shared by both proofs
}

// RelayTransferFeeRequest is the JSON body accepted by POST /relay/transfer_fee.
//
// Used for Enygma-to-Enygma confidential transfers with a public relayer fee
// (the enygma_fee circuit). The public signal must have exactly 55 elements
// with the fee at index 50 and the Fix L-01 domain separator at index 54.
type RelayTransferFeeRequest struct {
	Proof        [8]string  `json:"proof"        binding:"required"`
	PublicSignal []string   `json:"publicSignal" binding:"required"`
	Commitments  [][]string `json:"commitments"  binding:"required"`
	KIndex       []int64    `json:"kIndex"       binding:"required"`
}

// ── Shared response ───────────────────────────────────────────────────────────

// RelayResponse is returned by the relay endpoint on success.
type RelayResponse struct {
	TxHash      string `json:"txHash"`
	BlockNumber uint64 `json:"blockNumber"`
	GasUsed     uint64 `json:"gasUsed"`
}

// ── Info ─────────────────────────────────────────────────────────────────────

// InfoResponse is returned by GET /relay/info.
// Clients use this to discover the relayer's address and the contract address
// without needing them pre-configured out of band.
type InfoResponse struct {
	RelayerAddr  string `json:"relayerAddr"`
	ContractAddr string `json:"contractAddr"`
	ChainID      int64  `json:"chainId"`
}
