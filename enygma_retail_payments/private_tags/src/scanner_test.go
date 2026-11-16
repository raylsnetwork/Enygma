package tags

import (
	"bytes"
	"crypto/mlkem"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// A node that reports two channel records but fails every getChannel call,
// as a dropped connection would: the scan returns the error and its cursor
// stays put, rather than moving past records it never read.
func TestScanChannelsKeepsCursorOnRPCError(t *testing.T) {
	getChannelCount := crypto.Keccak256([]byte("getChannelCount()"))[:4]
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Params []struct {
				Data  hexutil.Bytes `json:"data"`
				Input hexutil.Bytes `json:"input"`
			} `json:"params"`
		}
		json.NewDecoder(r.Body).Decode(&req) //nolint:errcheck
		resp := map[string]any{"jsonrpc": "2.0", "id": req.ID}
		call := append(req.Params[0].Input, req.Params[0].Data...)
		if bytes.HasPrefix(call, getChannelCount) {
			resp["result"] = hexutil.Bytes(common.LeftPadBytes([]byte{2}, 32))
		} else {
			resp["error"] = map[string]any{"code": -32000, "message": "node unavailable"}
		}
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	defer srv.Close()
	client, err := ethclient.Dial(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	dk, err := mlkem.GenerateKey768()
	if err != nil {
		t.Fatal(err)
	}

	_, next, err := ScanChannelsFromCursor(client, common.HexToAddress("0xc4"), dk, &ScanCursor{}, 10)
	if err == nil {
		t.Fatal("scan succeeded with every channel record unreadable")
	}
	if next.LastChannelIdx != 0 {
		t.Fatalf("cursor moved to channel %d on an RPC error, want 0", next.LastChannelIdx)
	}
}
