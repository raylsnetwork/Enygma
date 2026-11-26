package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"enygma_retail_payments/relayer/config"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

type healthRPC struct {
	err   error
	delay time.Duration
}

func (p healthRPC) BlockNumber(ctx context.Context) (hexutil.Uint64, error) {
	if p.delay > 0 {
		timer := time.NewTimer(p.delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-timer.C:
		}
	}
	return 123, p.err
}

func TestHealthReportsChainReachability(t *testing.T) {
	const upstreamError = "upstream URL contains a secret"
	for _, tc := range []struct {
		name      string
		probe     healthRPC
		reachable bool
	}{
		{name: "reachable", reachable: true},
		{name: "unreachable", probe: healthRPC{err: errors.New(upstreamError)}},
		{name: "stalled", probe: healthRPC{delay: 4 * time.Second}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := rpc.NewServer()
			t.Cleanup(upstream.Stop)
			if err := upstream.RegisterName("eth", tc.probe); err != nil {
				t.Fatal(err)
			}
			client := ethclient.NewClient(rpc.DialInProc(upstream))
			t.Cleanup(client.Close)
			router, err := newRouter(&config.Config{}, &Handler{client: client})
			if err != nil {
				t.Fatal(err)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
			var resp struct {
				Status         string `json:"status"`
				ChainReachable *bool  `json:"chainReachable"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatal(err)
			}
			if w.Code != http.StatusOK || resp.Status != "ok" || resp.ChainReachable == nil ||
				*resp.ChainReachable != tc.reachable || strings.Contains(w.Body.String(), upstreamError) {
				t.Fatalf("got %d %s, want status ok and chainReachable %v without upstream details", w.Code, w.Body, tc.reachable)
			}
		})
	}
}
