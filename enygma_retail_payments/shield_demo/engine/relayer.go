package engine

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"enygma_retail_payments/layout"
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"
)

// relayer is a client for the retail relayer's HTTP API.
type relayer struct {
	baseURL, apiKey string
}

type relayInfo struct {
	RelayerAddr            string `json:"relayerAddr"`
	TagRegistryAddr        string `json:"tagRegistryAddr"`
	TagChannelRegistryAddr string `json:"tagChannelRegistryAddr"`
	FeeSpendPubKey         string `json:"feeSpendPubKey"`
	ChainID                int64  `json:"chainId"`
}

type relayPaymentRequest struct {
	VaultId      string    `json:"vaultId"`
	Proof        [8]string `json:"proof"`
	PublicSignal []string  `json:"publicSignal"`
	CipherText   string    `json:"cipherText"`
	EncTxData    string    `json:"encTxData"`
}

// relayUsdrFeeRequest is POST /relay/payment_usdr_fee's body: the main
// Payment or Payment2in leg plus the UsdrFee leg settled with it.
type relayUsdrFeeRequest struct {
	relayPaymentRequest
	UsdrVaultId      string                    `json:"usdrVaultId"`
	UsdrProof        [8]string                 `json:"usdrProof"`
	UsdrPublicSignal [layout.UsdrFeeLen]string `json:"usdrPublicSignal"`
	UsdrCipherText   string                    `json:"usdrCipherText"`
	UsdrEncTxData    string                    `json:"usdrEncTxData"`
	// UsdrFeeSalt opens the relayer's fee note, so it can spend it.
	UsdrFeeSalt string `json:"usdrFeeSalt"`
}

type relayTxResponse struct {
	TxHash      string `json:"txHash"`
	BlockNumber uint64 `json:"blockNumber"`
	GasUsed     uint64 `json:"gasUsed"`
	Error       string `json:"error,omitempty"`
}

func (r *relayer) info(ctx context.Context) (*relayInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+"/relay/info", nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET /relay/info: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET /relay/info: relayer %d", resp.StatusCode)
	}
	var info relayInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode /relay/info: %w", err)
	}
	return &info, nil
}

// payment submits a Payment or Payment2in proof to POST /relay/payment and
// returns once the relayer reports the transaction mined.
func (r *relayer) payment(ctx context.Context, res *rpcore.PaymentResult) (*relayTxResponse, error) {
	req, err := paymentRequest(res)
	if err != nil {
		return nil, err
	}
	var out relayTxResponse
	if err := r.post(ctx, "/relay/payment", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// paymentUsdrFee submits a Payment or Payment2in proof and the UsdrFee proof
// paying for it
// to POST /relay/payment_usdr_fee, which settles both in one transaction,
// and returns once the relayer reports it mined. The USDr leg publishes no
// ciphertext: its output 0 is the relayer's own note, opened by the salt.
func (r *relayer) paymentUsdrFee(ctx context.Context, res *rpcore.PaymentResult, fee *usdrFeeLeg) (*relayTxResponse, error) {
	main, err := paymentRequest(res)
	if err != nil {
		return nil, err
	}
	req := relayUsdrFeeRequest{
		relayPaymentRequest: main,
		UsdrVaultId:         usdrVaultId.String(),
		UsdrProof:           fee.proof,
		UsdrPublicSignal:    fee.signals,
		UsdrCipherText:      "0x",
		UsdrEncTxData:       "0x",
		UsdrFeeSalt:         fee.feeSalt.String(),
	}
	var out relayTxResponse
	if err := r.post(ctx, "/relay/payment_usdr_fee", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// paymentRequest is the relay request body for a main-token Payment or
// Payment2in proof.
func paymentRequest(res *rpcore.PaymentResult) (relayPaymentRequest, error) {
	stmt := res.ContractStatement()
	if len(stmt) != layout.PaymentLen && len(stmt) != layout.Payment2inLen {
		return relayPaymentRequest{}, fmt.Errorf("payment statement has %d signals, want %d or %d", len(stmt), layout.PaymentLen, layout.Payment2inLen)
	}
	req := relayPaymentRequest{
		VaultId:      erc20VaultId.String(),
		PublicSignal: make([]string, len(stmt)),
		CipherText:   "0x" + hex.EncodeToString(res.CipherText),
		EncTxData:    "0x" + hex.EncodeToString(res.EncTxData),
	}
	copy(req.Proof[:], res.Proof)
	for i, v := range stmt {
		req.PublicSignal[i] = v.String()
	}
	return req, nil
}

func (r *relayer) post(ctx context.Context, path string, body, result any) error {
	return postJSON(ctx, r.baseURL+path, map[string]string{"Authorization": "Bearer " + r.apiKey}, body, result)
}

// postJSON POSTs body to url as JSON with headers and decodes the response
// into result; a non-200 response fails with its "error" field, or its body.
func postJSON(ctx context.Context, url string, headers map[string]string, body, result any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("POST %s: %w", url, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(raw, &e) != nil || e.Error == "" {
			e.Error = string(raw)
		}
		return fmt.Errorf("POST %s: %d: %s", url, resp.StatusCode, e.Error)
	}
	return json.Unmarshal(raw, result)
}
