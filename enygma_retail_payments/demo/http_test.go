package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRetailRunRejectsCrossSitePosts(t *testing.T) {
	srv := &Server{broker: newBroker()}
	// A request that reaches the flow handler gets 409 without running it.
	srv.runLock.Lock()
	defer srv.runLock.Unlock()
	req := httptest.NewRequest(http.MethodPost, "/run", nil)
	req.Header.Set("Origin", "http://elsewhere.example")
	rec := httptest.NewRecorder()
	srv.handleRun(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestRetailEventsHaveNoWildcardOrigin(t *testing.T) {
	srv := &Server{broker: newBroker()}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rec := httptest.NewRecorder()
	srv.handleEvents(rec, httptest.NewRequestWithContext(ctx, http.MethodGet, "/events", nil))
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got == "*" {
		t.Fatal("events allow any origin")
	}
}
