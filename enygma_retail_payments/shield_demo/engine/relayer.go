package engine

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"enygma_retail_payments/layout"
	tags "github.com/raylsnetwork/enygma_retail_payments/private_tags/src"
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"
)

// relayer is a client for the retail relayer's HTTP API.
type relayer struct {
	baseURL, apiKey string
}

type relayInfo struct {
	RelayerAddr            string `json:"relayerAddr"`
	TagChannelRegistryAddr string `json:"tagChannelRegistryAddr"`
	FeeSpendPubKey         string `json:"feeSpendPubKey"`
	ChainID                int64  `json:"chainId"`
}

// relayProof is a main-token Payment or Payment2in proof as every payment
// route takes it.
type relayProof struct {
	VaultId      string    `json:"vaultId"`
	Proof        [8]string `json:"proof"`
	PublicSignal []string  `json:"publicSignal"`
}

// relayPaymentRequest is POST /relay/payment's body: the proof with its
// payee's ML-KEM capsule and sealed note.
type relayPaymentRequest struct {
	relayProof
	CipherText string `json:"cipherText"`
	EncTxData  string `json:"encTxData"`
}

// relayChannelPaymentRequest is POST /relay/channel_payment's body: the
// proof with the block its tag is for, the tag and the sealed note, all
// three bound by the proof, in place of a capsule.
type relayChannelPaymentRequest struct {
	relayProof
	BlockNumber uint64 `json:"blockNumber"`
	Tag         string `json:"tag"`
	Ctxt        string `json:"ctxt"`
}

// relayUsdrFeeLeg is the UsdrFee leg settled with a payment.
type relayUsdrFeeLeg struct {
	UsdrVaultId      string                    `json:"usdrVaultId"`
	UsdrProof        [8]string                 `json:"usdrProof"`
	UsdrPublicSignal [layout.UsdrFeeLen]string `json:"usdrPublicSignal"`
	UsdrCipherText   string                    `json:"usdrCipherText"`
	UsdrEncTxData    string                    `json:"usdrEncTxData"`
	// UsdrFeeSalt opens the relayer's fee note, so it can spend it.
	UsdrFeeSalt string `json:"usdrFeeSalt"`
}

// relayUsdrFeeRequest is POST /relay/payment_usdr_fee's body: the main
// Payment or Payment2in leg plus the UsdrFee leg settled with it.
type relayUsdrFeeRequest struct {
	relayPaymentRequest
	relayUsdrFeeLeg
}

// relayChannelUsdrFeeRequest is POST /relay/channel_payment_usdr_fee's
// body: a channel payment plus the UsdrFee leg settled with it.
type relayChannelUsdrFeeRequest struct {
	relayChannelPaymentRequest
	relayUsdrFeeLeg
}

type relayTxResponse struct {
	TxHash      string `json:"txHash"`
	BlockNumber uint64 `json:"blockNumber"`
	GasUsed     uint64 `json:"gasUsed"`
	Error       string `json:"error,omitempty"`
}

// relayChannelRequest is POST /relay/channel's body: the ML-KEM capsule
// (c1), the encrypted channel payload (c2) and the privacy bitmap.
type relayChannelRequest struct {
	C1     string `json:"c1"`
	C2     string `json:"c2"`
	Bitmap string `json:"bitmap"`
}

// relayChannelResponse is the opened channel's index and its transaction.
type relayChannelResponse struct {
	ChannelIdx uint64 `json:"channelIdx"`
	relayTxResponse
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

// relayRequest is a transfer's relay request: its path and body. A Payment
// or Payment2in proof alone goes to POST /relay/payment, or, over a tag
// channel (ch not nil), to POST /relay/channel_payment with its tag and
// sealed note; with the UsdrFee proof paying for it (fee not nil), both go
// to POST /relay/payment_usdr_fee, or /relay/channel_payment_usdr_fee,
// which settle them in one transaction. The USDr leg publishes no
// ciphertext: its output 0 is the relayer's own note, opened by the salt.
type relayRequest struct {
	path string
	body any
}

// newRelayRequest is the relay request of the proof in res, over the tag
// channel payment ch if not nil, with fee's.
func newRelayRequest(res *rpcore.PaymentResult, ch *tags.ChannelPayment, fee *usdrFeeLeg) (relayRequest, error) {
	proof, err := paymentProof(res)
	if err != nil {
		return relayRequest{}, err
	}
	var leg relayUsdrFeeLeg
	if fee != nil {
		leg = relayUsdrFeeLeg{
			UsdrVaultId:      usdrVaultId.String(),
			UsdrProof:        fee.proof,
			UsdrPublicSignal: fee.signals,
			UsdrCipherText:   "0x",
			UsdrEncTxData:    "0x",
			UsdrFeeSalt:      fee.feeSalt.String(),
		}
	}
	if ch != nil {
		main := relayChannelPaymentRequest{relayProof: proof, BlockNumber: ch.BlockNumber,
			Tag: "0x" + hex.EncodeToString(ch.Tag[:]), Ctxt: "0x" + hex.EncodeToString(ch.Ctxt)}
		if fee == nil {
			return relayRequest{"/relay/channel_payment", main}, nil
		}
		return relayRequest{"/relay/channel_payment_usdr_fee", relayChannelUsdrFeeRequest{main, leg}}, nil
	}
	main := relayPaymentRequest{relayProof: proof,
		CipherText: "0x" + hex.EncodeToString(res.CipherText), EncTxData: "0x" + hex.EncodeToString(res.EncTxData)}
	if fee == nil {
		return relayRequest{"/relay/payment", main}, nil
	}
	return relayRequest{"/relay/payment_usdr_fee", relayUsdrFeeRequest{main, leg}}, nil
}

// relay submits req and returns once the relayer reports its transaction
// mined.
func (r *relayer) relay(ctx context.Context, req relayRequest) (*relayTxResponse, HopDetail, error) {
	var out relayTxResponse
	ex, err := r.post(ctx, req.path, req.body, &out)
	if err != nil {
		return nil, HopDetail{}, err
	}
	return &out, ex, nil
}

// channel opens a tag channel through POST /relay/channel, so the relayer is
// its on-chain sender, and returns once it is mined.
func (r *relayer) channel(ctx context.Context, c1, c2, bitmap []byte) (*relayChannelResponse, HopDetail, error) {
	req := relayChannelRequest{
		C1:     "0x" + hex.EncodeToString(c1),
		C2:     "0x" + hex.EncodeToString(c2),
		Bitmap: "0x" + hex.EncodeToString(bitmap),
	}
	var out relayChannelResponse
	ex, err := r.post(ctx, "/relay/channel", req, &out)
	if err != nil {
		return nil, HopDetail{}, err
	}
	return &out, ex, nil
}

// paymentProof is the proof part of the relay request body for a
// main-token Payment or Payment2in proof.
func paymentProof(res *rpcore.PaymentResult) (relayProof, error) {
	stmt := res.ContractStatement()
	if len(stmt) != layout.PaymentLen && len(stmt) != layout.Payment2inLen {
		return relayProof{}, fmt.Errorf("payment statement has %d signals, want %d or %d", len(stmt), layout.PaymentLen, layout.Payment2inLen)
	}
	req := relayProof{VaultId: erc20VaultId.String(), PublicSignal: make([]string, len(stmt))}
	copy(req.Proof[:], res.Proof)
	for i, v := range stmt {
		req.PublicSignal[i] = v.String()
	}
	return req, nil
}

// rateLimitWait is how long post waits before retrying a request the
// relayer turned away with 429; rateLimitRetries how often it retries.
const (
	rateLimitWait    = 300 * time.Millisecond
	rateLimitRetries = 20
)

// post sends body to the relayer's path and returns the http hop detail of
// the request that was answered. The relayer rate-limits each client (5
// requests/s), and the run's channel requests can come faster than
// that on an automining chain; a 429 is answered before the relayer does
// anything, so post waits and sends the request again.
func (r *relayer) post(ctx context.Context, path string, body, result any) (HopDetail, error) {
	headers := map[string]string{"Authorization": "Bearer " + r.apiKey}
	rec := &recorder{}
	for retry := 0; ; retry++ {
		err := postJSON(ctx, rec.client(0), r.baseURL+path, headers, body, result)
		var he *httpError
		if !errors.As(err, &he) || he.status != http.StatusTooManyRequests || retry == rateLimitRetries {
			return rec.exchange(), err
		}
		select {
		case <-ctx.Done():
			return HopDetail{}, ctx.Err()
		case <-time.After(rateLimitWait):
		}
	}
}

// httpError is a non-200 response to postJSON.
type httpError struct {
	url    string
	status int
	msg    string
}

func (e *httpError) Error() string {
	return fmt.Sprintf("POST %s: %d: %s", e.url, e.status, e.msg)
}

// postJSON POSTs body to url as JSON with headers through client and
// decodes the response into result; a non-200 response fails with its
// "error" field, or its body.
func postJSON(ctx context.Context, client *http.Client, url string, headers map[string]string, body, result any) error {
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
	resp, err := client.Do(req)
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
		return &httpError{url: url, status: resp.StatusCode, msg: e.Error}
	}
	return json.Unmarshal(raw, result)
}
