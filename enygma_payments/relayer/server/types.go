package server

// ── Transfer ─────────────────────────────────────────────────────────────────

// RelayTransferRequest is the JSON body accepted by POST /relay/transfer.
//
// Used for Enygma-to-Enygma confidential transfers (the enygma circuit).
// The public signal array length is variable (up to 50 elements).
type RelayTransferRequest struct {
	Proof        [8]string  `json:"proof"        binding:"required"`
	PublicSignal []string   `json:"publicSignal" binding:"required"`
	Commitments  [][]string `json:"commitments"  binding:"required"`
	KIndex       []int64    `json:"kIndex"       binding:"required"`
}

// RelayTransferFeeRequest is the JSON body accepted by POST /relay/transfer_fee.
//
// Used for Enygma-to-Enygma confidential transfers with a public relayer fee
// (the enygma_fee circuit). The public signal must have exactly 54 elements
// with the fee at index 50.
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
	// MinFee is the minimum value the relayer requires at publicSignal[50]
	// for POST /relay/transfer_fee, as a decimal string. "0" means disabled.
	MinFee string `json:"minFee"`
}
