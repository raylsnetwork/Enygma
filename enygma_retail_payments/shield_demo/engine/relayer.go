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
	ChainID                int64  `json:"chainId"`
}

type relayPaymentRequest struct {
	VaultId      string                    `json:"vaultId"`
	Proof        [8]string                 `json:"proof"`
	PublicSignal [layout.PaymentLen]string `json:"publicSignal"`
	CipherText   string                    `json:"cipherText"`
	EncTxData    string                    `json:"encTxData"`
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

// payment submits a Payment proof to POST /relay/payment and returns once the
// relayer reports the transaction mined.
func (r *relayer) payment(ctx context.Context, res *rpcore.PaymentResult) (*relayTxResponse, error) {
	stmt := res.ContractStatement()
	if len(stmt) != layout.PaymentLen {
		return nil, fmt.Errorf("payment statement has %d signals, want %d", len(stmt), layout.PaymentLen)
	}
	req := relayPaymentRequest{
		VaultId:    "0",
		CipherText: "0x" + hex.EncodeToString(res.CipherText),
		EncTxData:  "0x" + hex.EncodeToString(res.EncTxData),
	}
	copy(req.Proof[:], res.Proof)
	for i, v := range stmt {
		req.PublicSignal[i] = v.String()
	}
	var out relayTxResponse
	if err := r.post(ctx, "/relay/payment", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *relayer) post(ctx context.Context, path string, body, result any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("POST %s: %w", path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &e)
		return fmt.Errorf("POST %s: relayer %d: %s", path, resp.StatusCode, e.Error)
	}
	return json.Unmarshal(raw, result)
}
