package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDvPMutationsRejectCrossSitePosts(t *testing.T) {
	broker := newBroker()
	state := newFlowState(broker)
	srv := &Server{broker: broker, fs: state}
	for path, handler := range map[string]http.HandlerFunc{"/step/unknown": srv.handleStep, "/reset": srv.handleReset} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, path, nil)
			req.Header.Set("Origin", "http://elsewhere.example")
			rec := httptest.NewRecorder()
			handler(rec, req)
			if rec.Code != http.StatusForbidden || srv.fs != state {
				t.Fatalf("status = %d, want 403 with the original flow state", rec.Code)
			}
		})
	}
}

func TestDvPEventsHaveNoWildcardOrigin(t *testing.T) {
	srv := &Server{broker: newBroker()}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rec := httptest.NewRecorder()
	srv.handleEvents(rec, httptest.NewRequestWithContext(ctx, http.MethodGet, "/events", nil))
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got == "*" {
		t.Fatal("events allow any origin")
	}
}
