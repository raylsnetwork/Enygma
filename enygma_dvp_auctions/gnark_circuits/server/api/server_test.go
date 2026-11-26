package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"enygma_dvp_auctions/gnark_circuits/server/circuits/auctionBatch"
	"enygma_dvp_auctions/gnark_circuits/server/circuits/auctionBid"
	"enygma_dvp_auctions/gnark_circuits/server/circuits/auctionFinal"
	"enygma_dvp_auctions/gnark_circuits/server/circuits/auctionLock"
	"enygma_dvp_auctions/gnark_circuits/server/circuits/auctionRevert"
	"enygma_dvp_auctions/gnark_circuits/server/circuits/auctionWithdraw"
	"enygma_dvp_auctions/gnark_circuits/server/config"
)

func TestProofRejectsMalformedWitnessNumber(t *testing.T) {
	r := NewServer(&config.Config{})
	for _, tc := range []struct {
		route   string
		request any
	}{
		{"auctionBatch", &auctionBatch.AuctionBatchRequest{}},
		{"auctionBid", &auctionBid.AuctionBidRequest{}},
		{"auctionFinal", &auctionFinal.AuctionFinalRequest{}},
		{"auctionLock", &auctionLock.AuctionLockRequest{}},
		{"auctionRevert", &auctionRevert.AuctionRevertRequest{}},
		{"auctionWithdraw", &auctionWithdraw.AuctionWithdrawRequest{}},
	} {
		t.Run(tc.route, func(t *testing.T) {
			value := reflect.ValueOf(tc.request).Elem()
			fillWitnessNumbers(t, value)
			value.Field(0).SetString("invalid")
			body, err := json.Marshal(tc.request)
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, "/proof/"+tc.route, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest || !bytes.Contains(w.Body.Bytes(), []byte("invalid witness number")) {
				t.Fatalf("got %d %s, want a witness validation error (400)", w.Code, w.Body)
			}
		})
	}
}

// fillWitnessNumbers builds a binding-valid zero witness for the request's shape.
func fillWitnessNumbers(t *testing.T, value reflect.Value) {
	t.Helper()
	switch value.Kind() {
	case reflect.String:
		value.SetString("0")
	case reflect.Array:
		for i := 0; i < value.Len(); i++ {
			fillWitnessNumbers(t, value.Index(i))
		}
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			fillWitnessNumbers(t, value.Field(i))
		}
	default:
		t.Fatalf("unexpected witness request field type %s", value.Type())
	}
}

func TestProofAPIKey(t *testing.T) {
	for _, tc := range []struct {
		configured string
		supplied   string
		want       int
	}{
		{"", "", http.StatusBadRequest},
		{"test-key", "", http.StatusUnauthorized},
		{"test-key", "test-kez", http.StatusUnauthorized},
		{"test-key", "test-key", http.StatusBadRequest},
	} {
		r := NewServer(&config.Config{APIKey: tc.configured})
		req := httptest.NewRequest(http.MethodPost, "/proof/auctionRevert", strings.NewReader("{}"))
		req.Header.Set("X-Gnark-Key", tc.supplied)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Errorf("configured %q with header %q: got %d, want %d", tc.configured, tc.supplied, w.Code, tc.want)
		}
	}
}

func TestProofCapsRequestBody(t *testing.T) {
	r := NewServer(&config.Config{})
	padding := strings.Repeat(" ", 256<<10)
	for _, tc := range []struct {
		name string
		body string
	}{
		{"leading_data", padding + "{}"},
		{"trailing_data", "{}" + padding},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/proof/auctionRevert", strings.NewReader(tc.body))
			req.ContentLength = -1 // The limit also applies without a declared length.
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "request body too large") {
				t.Fatalf("got %d %s, want a body-size error (400)", w.Code, w.Body)
			}
		})
	}
}

func TestConfiguredProofSlotLimit(t *testing.T) {
	t.Setenv("GNARK_MAX_CONCURRENT_PROOFS", "1")
	r := NewServer(&config.Config{})
	body, input := io.Pipe()
	t.Cleanup(func() {
		_ = input.Close()
		_ = body.Close()
	})

	held := make(chan int, 1)
	go func() {
		req := httptest.NewRequest(http.MethodPost, "/proof/auctionRevert", body)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		held <- w.Code
	}()
	read := make(chan error, 1)
	go func() {
		_, err := io.WriteString(input, "{")
		read <- err
	}()
	select {
	case err := <-read:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("first request did not begin reading its body")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/proof/auctionRevert", strings.NewReader("{")).WithContext(ctx))
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("while the only slot is occupied, got %d, want 503", w.Code)
	}

	if err := input.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case code := <-held:
		if code != http.StatusBadRequest {
			t.Errorf("first request returned %d, want 400 for invalid JSON", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("first request did not finish")
	}

	next, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/proof/auctionRevert", strings.NewReader("{")).WithContext(next))
	if w.Code != http.StatusBadRequest {
		t.Errorf("after the slot is released, got %d, want 400 for invalid JSON", w.Code)
	}
}
