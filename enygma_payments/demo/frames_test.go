package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDemoPagesDenyFraming(t *testing.T) {
	srv := &Server{}
	for path, handler := range map[string]http.HandlerFunc{"/": srv.handleIndex, "/bank": srv.handleBank} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler(rec, httptest.NewRequest(http.MethodGet, path, nil))
			if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
				t.Errorf("X-Frame-Options = %q, want DENY", got)
			}
			if got := rec.Header().Get("Content-Security-Policy"); got != "frame-ancestors 'none'" {
				t.Errorf("Content-Security-Policy = %q, want frame-ancestors 'none'", got)
			}
		})
	}
}
