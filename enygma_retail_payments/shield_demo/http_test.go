package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestShieldRoutesRejectCrossSitePosts(t *testing.T) {
	routes := (&server{}).routes()
	for _, path := range []string{"/run", "/transfer", "/release"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader("{"))
			req.Header.Set("Origin", "http://elsewhere.example")
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			routes.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", rec.Code)
			}
		})
	}
}

func TestShieldJSONRoutesRejectPlainText(t *testing.T) {
	routes := (&server{}).routes()
	for _, path := range []string{"/run", "/transfer"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader("{"))
			req.Header.Set("Content-Type", "text/plain")
			rec := httptest.NewRecorder()
			routes.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnsupportedMediaType {
				t.Fatalf("status = %d, want 415", rec.Code)
			}
		})
	}
}
